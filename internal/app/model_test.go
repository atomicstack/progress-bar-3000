package app

import (
	"strings"
	"testing"
	"time"

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
