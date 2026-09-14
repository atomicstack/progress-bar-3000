package app

import (
	"strings"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"progress-bar-3000/internal/config"
	"progress-bar-3000/internal/input"
	"progress-bar-3000/internal/progress"
	"progress-bar-3000/internal/render"
)

func animationModel() Model {
	m := NewModel(config.Config{Format: "%p", Style: config.StyleGranular, ColorMode: config.ColorModeTrueColor, BackgroundStyle: config.BackgroundStyleSpace, GradientStart: "#ff70d2", GradientEnd: "#00d8ff", Width: 60, FPS: 30, Lerp: 1}, progress.State{Total: 100, Value: 40, DisplayValue: 40, Phases: []string{"build", "test"}})
	m.state.PhaseIndex = 0
	return m
}

func animationUpdate(m Model, msg tea.Msg) Model {
	updated, _ := m.Update(msg)
	return updated.(Model)
}

func TestEventAnimationsReactToActualChanges(t *testing.T) {
	for _, tc := range []struct {
		name, line          string
		ripple, phase, want bool
	}{
		{"advance", "@value 60", true, false, true},
		{"duplicate", "@value 40", true, false, false},
		{"backward", "@value 20", true, false, false},
		{"denominator", "@set-total 80", true, false, false},
		{"phase change", "@phase-name test", false, true, true},
		{"same phase", "@phase-name build", false, true, false},
		{"unknown phase", "@phase-name missing", false, true, false},
		{"label only", "@label waiting", true, true, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			m := animationModel()
			m.cfg.MilestoneRipple = tc.ripple
			m.cfg.PhaseTransition = tc.phase
			at := time.Unix(100, 0)
			evt, err := input.ParseLine(config.InputModeAuto, tc.line)
			if err != nil {
				t.Fatal(err)
			}
			m = animationUpdate(m, eventMsg{Event: evt, Now: at})
			m = animationUpdate(m, frameMsg{Now: at.Add(300 * time.Millisecond)})
			baseline := m
			baseline.cfg.MilestoneRipple = false
			baseline.cfg.PhaseTransition = false
			got, want := m.View(), baseline.View()
			if (got != want) != tc.want {
				t.Fatalf("effect visible=%v, want %v", got != want, tc.want)
			}
			if render.StripANSI(got) != render.StripANSI(want) {
				t.Fatal("animation changed fill geometry")
			}
			m = animationUpdate(m, frameMsg{Now: at.Add(3 * time.Second)})
			baseline = m
			baseline.cfg.MilestoneRipple = false
			baseline.cfg.PhaseTransition = false
			if m.View() != baseline.View() {
				t.Fatal("event animation did not expire")
			}
		})
	}
}

func TestAmbientAnimationsReachRenderer(t *testing.T) {
	for _, name := range []string{"aurora", "comet", "interference", "embers", "liquid", "edge-glow"} {
		t.Run(name, func(t *testing.T) {
			m := animationModel()
			m.cfg.TintAnimation = config.TintAnimation(name)
			m = animationUpdate(m, frameMsg{Now: time.Unix(100, 0)})
			first := m.View()
			m = animationUpdate(m, frameMsg{Now: time.Unix(101, 370000000)})
			second := m.View()
			if first == second {
				t.Fatal("ambient animation did not advance")
			}
			if render.StripANSI(first) != render.StripANSI(second) {
				t.Fatal("ambient animation changed fill geometry")
			}
		})
	}
}

func TestEventAnimationLifecycle(t *testing.T) {
	at := time.Unix(100, 0)
	m := animationModel()
	m.cfg.MilestoneRipple, m.cfg.PhaseTransition = true, true
	apply := func(line string, when time.Time) {
		t.Helper()
		evt, err := input.ParseLine(config.InputModeAuto, line)
		if err != nil {
			t.Fatal(err)
		}
		m = animationUpdate(m, eventMsg{Event: evt, Now: when})
	}
	apply(`{"type":"value","value":60,"phase":"test"}`, at)
	if m.animations.rippleAt != at || m.animations.phaseAt != at {
		t.Fatal("combined update did not trigger both effects")
	}
	firstStrength := m.animations.rippleStrength
	apply(`{"type":"value","value":60,"phase":"test"}`, at.Add(time.Second))
	if m.animations.rippleAt != at || m.animations.phaseAt != at {
		t.Fatal("duplicate update restarted effects")
	}
	apply(`{"type":"value","value":61,"phase":"test"}`, at.Add(2*time.Second))
	if m.animations.rippleAt != at.Add(2*time.Second) || m.animations.rippleStrength >= firstStrength {
		t.Fatal("newest smaller milestone did not replace the ripple with lower intensity")
	}
	apply("@reset", at.Add(3*time.Second))
	if m.animations != (animationEvents{}) {
		t.Fatal("reset retained stale effects")
	}
}

func TestPhaseAnimationTracksOnlyVisibleSubphaseChanges(t *testing.T) {
	at := time.Unix(100, 0)
	m := animationModel()
	m.cfg.PhaseTransition = true
	apply := func(line string, when time.Time) {
		t.Helper()
		evt, err := input.ParseLine(config.InputModeAuto, line)
		if err != nil {
			t.Fatal(err)
		}
		m = animationUpdate(m, eventMsg{Event: evt, Now: when})
	}
	apply(`{"type":"set_subphases","phase":"test","subphases":["unit","integration"]}`, at)
	if !m.animations.phaseAt.IsZero() {
		t.Fatal("future parent plan triggered visible phase wave")
	}
	apply("@set-subphases compile,link", at.Add(time.Second))
	if m.animations.phaseAt != at.Add(time.Second) {
		t.Fatal("new visible child did not trigger wave")
	}
	apply("@subphase-name link", at.Add(2*time.Second))
	if m.animations.phaseAt != at.Add(2*time.Second) {
		t.Fatal("child selection did not trigger wave")
	}
	apply("@subphase-name missing", at.Add(3*time.Second))
	apply(`{"type":"subphase","phase":"test","name":"integration"}`, at.Add(4*time.Second))
	if m.animations.phaseAt != at.Add(2*time.Second) {
		t.Fatal("ignored or hidden child restarted wave")
	}
	apply("@set-phases build,test", at.Add(5*time.Second))
	if m.animations != (animationEvents{}) {
		t.Fatal("new plan retained old animation state")
	}
}

func TestBatchAnimationsMatchRawEventsAndDoNotDelayCompletion(t *testing.T) {
	at := time.Unix(100, 0)
	m := animationModel()
	m.cfg.MilestoneRipple, m.cfg.PhaseTransition = true, true
	m.cfg.OnComplete = "printf done"
	var output strings.Builder
	m.hooks.output = &output
	var events []input.Event
	for _, line := range []string{"@value 60", "@phase-name test", "@value 100"} {
		evt, err := input.ParseLine(config.InputModeAuto, line)
		if err != nil {
			t.Fatal(err)
		}
		events = append(events, evt)
	}
	ack := false
	m = animationUpdate(m, batchMsg{Events: events, Now: at, Reply: func(err error) error {
		if err != nil {
			t.Fatal(err)
		}
		if m.hooks.tail != nil {
			t.Fatal("hook started before acknowledgement")
		}
		ack = true
		return nil
	}})
	if !ack || m.animations.rippleAt != at || m.animations.phaseAt != at {
		t.Fatal("batch effects or acknowledgement missing")
	}
	if err := m.hooks.wait(); err != nil {
		t.Fatal(err)
	}
	if output.String() != "done" {
		t.Fatal("completion was deferred until animation finished")
	}
	if m.state.Value != 100 {
		t.Fatal("animation changed final progress")
	}
}

func TestBatchDoesNotAnimateInvisiblePhaseTransitions(t *testing.T) {
	m := animationModel()
	m.cfg.PhaseTransition = true
	at := time.Unix(100, 0)
	// value mapping briefly selects test; the batch explicitly restores build
	// before any frame can render it. the visible phase never changed.
	events := []input.Event{{Kind: input.KindValue, Value: 45}, {Kind: input.KindPhase, PhaseName: "build"}}
	m = animationUpdate(m, batchMsg{Events: events, Now: at})
	if !m.animations.phaseAt.IsZero() {
		t.Fatal("batch animated a phase the user never saw")
	}
}
