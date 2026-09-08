package app

import (
	"strings"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"progress-bar-3000/internal/config"
	"progress-bar-3000/internal/input"
	"progress-bar-3000/internal/progress"
)

func TestModelViewCompactIncludesPhaseAndPercent(t *testing.T) {
	cfg := config.Config{
		Format:          "%{bar-only} %{percent} %{phase}",
		Style:           config.StyleGradientGranular,
		BackgroundStyle: config.BackgroundStyleSpace,
		GradientStart:   "#ffffff",
		GradientEnd:     "#0087ff",
		FPS:             60,
		Lerp:            0.2,
	}

	m := NewModel(cfg, progress.State{
		Total:        4,
		Value:        2,
		DisplayValue: 2,
		Phases:       []string{"build", "test", "package", "ship"},
	})

	got := stripANSI(m.View())
	if !strings.Contains(got, "50") {
		t.Fatalf("View() = %q, want percent", got)
	}
	if !strings.Contains(got, "test") {
		t.Fatalf("View() = %q, want phase label", got)
	}
}

func TestDetailFormatRendersTemplateBelowBar(t *testing.T) {
	cfg := config.Config{
		Format:          "%{bar-only}",
		DetailFormats:   []string{"phase: %{phase} [%{label}]"},
		Style:           config.StyleGradientGranular,
		BackgroundStyle: config.BackgroundStyleSpace,
		GradientStart:   "#ffffff",
		GradientEnd:     "#0087ff",
		FPS:             60,
		Lerp:            0.2,
	}

	m := NewModel(cfg, progress.State{
		Total:        4,
		Value:        2,
		DisplayValue: 2,
		Phases:       []string{"build", "test", "package", "ship"},
		Label:        "compiling widget.go",
	})

	got := stripANSI(m.View())
	want := "phase: test [compiling widget.go]"
	if !strings.Contains(got, want) {
		t.Fatalf("View() = %q, want to contain %q", got, want)
	}
}

func TestDetailFormatRendersAfterKeyedDetailRows(t *testing.T) {
	cfg := config.Config{
		Format:          "%{bar-only}",
		Detail:          "phase",
		DetailFormats:   []string{"custom: %{label}"},
		Style:           config.StyleGradientGranular,
		BackgroundStyle: config.BackgroundStyleSpace,
		GradientStart:   "#ffffff",
		GradientEnd:     "#0087ff",
		FPS:             60,
		Lerp:            0.2,
	}

	m := NewModel(cfg, progress.State{
		Total:        2,
		Value:        1,
		DisplayValue: 1,
		Phases:       []string{"build", "ship"},
		Label:        "linking",
	})

	got := stripANSI(m.View())
	phaseIdx := strings.Index(got, "phase: build")
	customIdx := strings.Index(got, "custom: linking")
	if phaseIdx < 0 || customIdx < 0 {
		t.Fatalf("View() = %q, want both 'phase: build' and 'custom: linking'", got)
	}
	if phaseIdx >= customIdx {
		t.Fatalf("View() = %q, want keyed detail row before --detail-format row", got)
	}
}

func TestModelTickLerpsDisplayValueTowardTarget(t *testing.T) {
	cfg := config.Config{Format: "%{percent}", FPS: 60, Lerp: 0.25}
	m := NewModel(cfg, progress.State{Total: 4, Value: 4, DisplayValue: 0})

	next, _ := m.Update(frameMsg{Now: time.Unix(1, 0)})
	updated := next.(Model)
	if updated.state.DisplayValue <= 0 {
		t.Fatalf("DisplayValue = %v, want > 0 after frame", updated.state.DisplayValue)
	}
	if updated.state.DisplayValue >= updated.state.Value {
		t.Fatalf("DisplayValue = %v, want it to lerp instead of snap", updated.state.DisplayValue)
	}
}

func TestModelPulseAnimationChangesView(t *testing.T) {
	cfg := config.Config{
		Format:        "%{bar-only}",
		Style:         config.StyleGradientBlock,
		GradientStart: "#ffffff",
		GradientEnd:   "#0087ff",
		ColorMode:     config.ColorModeTrueColor,
		TintAnimation: config.TintAnimationPulse,
		FPS:           60,
		Lerp:          0.2,
	}

	m := NewModel(cfg, progress.State{Total: 4, Value: 2, DisplayValue: 2})
	before := m.View()
	next, _ := m.Update(frameMsg{Now: time.Unix(1, 0)})
	after := next.(Model).View()
	if before == after {
		t.Fatal("expected pulse animation to change rendered output")
	}
}

func TestModelShimmerAnimationChangesView(t *testing.T) {
	cfg := config.Config{
		Format:        "%{bar-only}",
		Style:         config.StyleGradientBlock,
		GradientStart: "#ffffff",
		GradientEnd:   "#0087ff",
		ColorMode:     config.ColorModeTrueColor,
		TintAnimation: config.TintAnimationShimmer,
		FPS:           60,
		Lerp:          0.2,
	}

	m := NewModel(cfg, progress.State{Total: 4, Value: 2, DisplayValue: 2})
	before := m.View()
	next, _ := m.Update(frameMsg{Now: time.Unix(1, 0)})
	after := next.(Model).View()
	if before == after {
		t.Fatal("expected shimmer animation to change rendered output")
	}
}

func TestModelCycleAnimationChangesViewAcrossFrames(t *testing.T) {
	cfg := config.Config{
		Format:        "%{bar-only}",
		Style:         config.StyleGradientBlock,
		GradientStart: "#ffffff",
		GradientEnd:   "#0087ff",
		ColorMode:     config.ColorModeTrueColor,
		TintAnimation: config.TintAnimationCycle,
		FPS:           60,
		Lerp:          0.2,
	}

	m := NewModel(cfg, progress.State{Total: 4, Value: 2, DisplayValue: 2})
	first := m.View()
	next, _ := m.Update(frameMsg{Now: time.Unix(1, 0)})
	second := next.(Model)
	secondView := second.View()
	next, _ = second.Update(frameMsg{Now: time.Unix(2, 0)})
	thirdView := next.(Model).View()

	if first == secondView || secondView == thirdView || first == thirdView {
		t.Fatalf("expected cycle animation to evolve across frames, got first=%q second=%q third=%q", first, secondView, thirdView)
	}
}

func TestModelApplyEventUpdatesState(t *testing.T) {
	cfg := config.Config{Format: "%{value}/%{total}", FPS: 60, Lerp: 0.2}
	m := NewModel(cfg, progress.State{Total: 3})

	next, _ := m.Update(eventMsg{Event: input.Event{Kind: input.KindTick, Amount: 1}, Now: time.Unix(1, 0)})
	updated := next.(Model)
	if updated.state.Value != 1 {
		t.Fatalf("Value = %v, want 1", updated.state.Value)
	}
}

func TestModelASCIIFlagForcesASCIIGlyphs(t *testing.T) {
	cfg := config.Config{
		Format:          "%{bar-only}",
		Style:           config.StyleGradientGranular,
		BackgroundStyle: config.BackgroundStyleShadeLight,
		ColorMode:       config.ColorModeNone,
		Width:           4,
		ASCII:           true,
		FPS:             60,
		Lerp:            0.2,
	}

	m := NewModel(cfg, progress.State{Total: 8, Value: 3, DisplayValue: 3})

	got := stripANSI(m.View())
	want := "=..."
	if got != want {
		t.Fatalf("View() = %q, want %q", got, want)
	}
}

func TestModelTimerRendersElapsedWholeSeconds(t *testing.T) {
	cfg := config.Config{Format: "%{timer}|%t", FPS: 60, Lerp: 0.2}
	m := NewModel(cfg, progress.State{Total: 4, StartedAt: time.Unix(100, 0)})

	next, _ := m.Update(frameMsg{Now: time.Unix(165, 700_000_000)})
	got := stripANSI(next.(Model).View())
	want := "1m5s|1m5s"
	if got != want {
		t.Fatalf("View() = %q, want %q", got, want)
	}
}

func TestModelTimerRendersZeroWhenStartUnknown(t *testing.T) {
	cfg := config.Config{Format: "%{timer}", FPS: 60, Lerp: 0.2}
	m := NewModel(cfg, progress.State{Total: 4})

	next, _ := m.Update(frameMsg{Now: time.Unix(165, 0)})
	got := stripANSI(next.(Model).View())
	if got != "0s" {
		t.Fatalf("View() = %q, want %q", got, "0s")
	}
}

func TestModelBarAndTotalFallBackToPhaseCount(t *testing.T) {
	cfg := config.Config{
		Format:          "%{bar-only}|%{total}",
		Detail:          "value",
		Style:           config.StylePlain,
		BackgroundStyle: config.BackgroundStyleASCII,
		ColorMode:       config.ColorModeNone,
		Width:           4,
		FPS:             60,
		Lerp:            0.2,
	}

	m := NewModel(cfg, progress.State{
		Total:        0,
		Value:        2,
		DisplayValue: 2,
		Phases:       []string{"build", "test", "package", "ship"},
	})

	got := stripANSI(m.View())
	want := "==..|4\nvalue: 2/4"
	if got != want {
		t.Fatalf("View() = %q, want %q", got, want)
	}
}

func TestModelViewRowsAreNotPaddedWithTrailingNewline(t *testing.T) {
	cfg := config.Config{
		Format:          "%{bar-only} %{percent}",
		Detail:          "phase",
		Style:           config.StylePlain,
		BackgroundStyle: config.BackgroundStyleASCII,
		ColorMode:       config.ColorModeNone,
		Width:           4,
		FPS:             60,
		Lerp:            0.2,
	}

	m := NewModel(cfg, progress.State{
		Total:        4,
		Value:        2,
		DisplayValue: 2,
		Phases:       []string{"build", "test", "package", "ship"},
	})

	got := stripANSI(m.View())
	if strings.HasSuffix(got, "\n") {
		t.Fatalf("View() = %q, must not end with a newline: a padding row pushes the bar off a 2-row pane", got)
	}
	rows := strings.Split(got, "\n")
	if len(rows) != 2 {
		t.Fatalf("View() = %q, want exactly 2 rows (bar + detail), got %d", got, len(rows))
	}
	if rows[0] != "==.. 50%" {
		t.Fatalf("row 0 = %q, want %q", rows[0], "==.. 50%")
	}
	if rows[1] != "phase: test" {
		t.Fatalf("row 1 = %q, want %q", rows[1], "phase: test")
	}
}

func phasesConfig(format string) config.Config {
	return config.Config{
		Format:        format,
		Style:         config.StyleGradientBlock,
		GradientStart: "#ffffff",
		GradientEnd:   "#0087ff",
		ColorMode:     config.ColorModeNone,
		FPS:           60,
		Lerp:          0.5,
	}
}

func firstLine(s string) string {
	return strings.SplitN(s, "\n", 2)[0]
}

func TestModelPhasesTokenRendersStripWithCurrentHighlighted(t *testing.T) {
	m := NewModel(phasesConfig("%{phases}"), progress.State{
		Value:        2,
		DisplayValue: 2,
		Phases:       []string{"build", "test", "ship"},
		PhaseIndex:   1,
	})

	got := strings.TrimRight(firstLine(m.View()), " ")
	if got != "build › [test] › ship" {
		t.Fatalf("View() = %q, want bracketed current phase under profile none", got)
	}
}

func TestModelPhasesTokenRendersEmptyWithoutPlan(t *testing.T) {
	m := NewModel(phasesConfig("<%{phases}>"), progress.State{Total: 4, Value: 1, DisplayValue: 1})

	if got := firstLine(m.View()); got != "<>" {
		t.Fatalf("View() = %q, want empty phases token without a plan", got)
	}
}

func TestModelPhasesTokenEmitsBoldAndColourWhenProfileAllows(t *testing.T) {
	cfg := phasesConfig("%{phases}")
	cfg.ColorMode = config.ColorModeTrueColor
	m := NewModel(cfg, progress.State{
		Phases:     []string{"build", "test", "ship"},
		PhaseIndex: 1,
	})

	got := m.View()
	if !strings.Contains(got, "\x1b[1m") || !strings.Contains(got, "\x1b[38;2;") || !strings.Contains(got, "\x1b[2m") {
		t.Fatalf("View() = %q, want bold, colour and dim escapes", got)
	}
	if strings.Contains(stripANSI(got), "[") {
		t.Fatalf("View() = %q, want no bracket form when colour is available", got)
	}
}

func TestModelPhasesTokenUsesASCIISeparator(t *testing.T) {
	cfg := phasesConfig("%{phases}")
	cfg.ASCII = true
	m := NewModel(cfg, progress.State{Phases: []string{"build", "test"}, PhaseIndex: 0})

	got := strings.TrimRight(firstLine(m.View()), " ")
	if got != "[build] > test" {
		t.Fatalf("View() = %q, want ascii separator", got)
	}
}

func TestModelPhasesTokenFallsBackToEightyColumns(t *testing.T) {
	m := NewModel(phasesConfig("%{phases}"), progress.State{Phases: []string{"build", "test"}, PhaseIndex: 0})

	if got := len([]rune(firstLine(m.View()))); got != 80 {
		t.Fatalf("View() width = %d, want 80 before any WindowSizeMsg", got)
	}
}

func TestModelPhasesTokenUsesWindowSizeWidth(t *testing.T) {
	m := NewModel(phasesConfig("%{phases}"), progress.State{Phases: []string{"build", "test"}, PhaseIndex: 0})

	next, _ := m.Update(tea.WindowSizeMsg{Width: 24, Height: 10})
	got := firstLine(next.(Model).View())
	if len([]rune(got)) != 24 {
		t.Fatalf("View() = %q (width %d), want the 24-column terminal width", got, len([]rune(got)))
	}
}

func TestModelPhasesTokenWidthPrefixWinsOverTerminalWidth(t *testing.T) {
	m := NewModel(phasesConfig("%30{phases}"), progress.State{Phases: []string{"build", "test"}, PhaseIndex: 0})

	next, _ := m.Update(tea.WindowSizeMsg{Width: 120, Height: 10})
	got := firstLine(next.(Model).View())
	if len([]rune(got)) != 30 {
		t.Fatalf("View() = %q (width %d), want the 30-column width prefix", got, len([]rune(got)))
	}
}

func TestModelPhasesTokenWorksInDetailFormat(t *testing.T) {
	cfg := phasesConfig("%{percent}")
	cfg.DetailFormats = []string{"plan: %{phases}"}
	m := NewModel(cfg, progress.State{Phases: []string{"build", "test"}, PhaseIndex: 1})

	got := m.View()
	if !strings.Contains(got, "plan: build › [test]") {
		t.Fatalf("View() = %q, want phases strip in the detail row", got)
	}
}

func TestModelLerpsPhasesOffsetTowardTargetAndSnaps(t *testing.T) {
	phases := []string{"alpha", "bravo", "charlie", "delta", "echo", "foxtrot", "golf", "hotel"}
	m := NewModel(phasesConfig("%20{phases}"), progress.State{Phases: phases, PhaseIndex: 0})

	// Jump to the last phase: the target offset moves far to the right.
	next, _ := m.Update(eventMsg{Event: input.Event{Kind: input.KindPhase, PhaseName: "hotel"}, Now: time.Unix(1, 0)})
	next, _ = next.(Model).Update(frameMsg{Now: time.Unix(1, 0)})
	after := next.(Model)
	if after.phasesTarget <= 0 {
		t.Fatalf("phasesTarget = %d, want > 0 after jumping to the last phase", after.phasesTarget)
	}
	if after.phasesOffset <= 0 {
		t.Fatalf("phasesOffset = %v, want > 0 after one frame", after.phasesOffset)
	}
	if after.phasesOffset >= float64(after.phasesTarget) {
		t.Fatalf("phasesOffset = %v, want it to lerp toward %d instead of snapping", after.phasesOffset, after.phasesTarget)
	}

	// Enough frames and it lands exactly on the target.
	var model tea.Model = after
	for i := range 40 {
		model, _ = model.(Model).Update(frameMsg{Now: time.Unix(int64(2+i), 0)})
	}
	settled := model.(Model)
	if settled.phasesOffset != float64(settled.phasesTarget) {
		t.Fatalf("phasesOffset = %v, want to snap exactly onto %d", settled.phasesOffset, settled.phasesTarget)
	}
	if got := firstLine(settled.View()); !strings.Contains(got, "[hotel]") {
		t.Fatalf("View() = %q, want the current phase in view once settled", got)
	}
}

func TestModelPhasesHighlightPulsesOverTime(t *testing.T) {
	cfg := phasesConfig("%{phases}")
	cfg.ColorMode = config.ColorModeTrueColor
	m := NewModel(cfg, progress.State{Phases: []string{"build", "test"}, PhaseIndex: 0})

	next, _ := m.Update(frameMsg{Now: time.Unix(10, 0)})
	first := next.(Model).View()
	// #0087ff is fully saturated, so only the trough of the sine (1.5s into
	// the 2s cycle) can move the colour.
	next, _ = next.(Model).Update(frameMsg{Now: time.Unix(11, 500*int64(time.Millisecond))})
	second := next.(Model).View()
	if first == second {
		t.Fatal("expected the phases highlight colour to pulse between frames")
	}
	if stripANSI(first) != stripANSI(second) {
		t.Fatalf("expected only colours to change, got %q vs %q", stripANSI(first), stripANSI(second))
	}
}

func TestModelPhasesCrossfadeSettlesAfterThreeHundredMilliseconds(t *testing.T) {
	cfg := phasesConfig("%{phases}")
	cfg.ColorMode = config.ColorModeTrueColor
	m := NewModel(cfg, progress.State{Phases: []string{"build", "test"}, PhaseIndex: 0})

	changed := time.Unix(10, 0)
	next, _ := m.Update(eventMsg{Event: input.Event{Kind: input.KindPhase, PhaseName: "test"}, Now: changed})
	next, _ = next.(Model).Update(frameMsg{Now: changed.Add(50 * time.Millisecond)})
	mid := next.(Model).View()
	// Mid-fade the previous phase is coloured, not dim.
	buildIdx := strings.Index(mid, "build")
	if strings.Contains(mid[:buildIdx], "\x1b[2m") {
		t.Fatalf("mid-fade View() = %q, want previous phase still coloured rather than dim", mid)
	}

	next, _ = next.(Model).Update(frameMsg{Now: changed.Add(400 * time.Millisecond)})
	settled := next.(Model).View()
	buildIdx = strings.Index(settled, "build")
	if !strings.Contains(settled[:buildIdx], "\x1b[2m") {
		t.Fatalf("settled View() = %q, want previous phase dim after 300ms", settled)
	}
}
