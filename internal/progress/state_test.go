package progress

import (
	"math"
	"testing"
	"time"

	"progress-bar-3000/internal/input"
)

func TestStateApplyTickDerivesPercentAndPhase(t *testing.T) {
	t.Parallel()

	state := State{
		Total:  3,
		Phases: []string{"build", "test", "package"},
	}

	state.Apply(input.Event{Kind: input.KindTick, Amount: 1}, time.Unix(100, 0))

	if state.Value != 1 {
		t.Fatalf("Value = %v, want 1", state.Value)
	}

	if diff := math.Abs(state.Percent() - 33.333333333333336); diff > 0.01 {
		t.Fatalf("Percent() = %v, want about 33.33", state.Percent())
	}

	if got := state.CurrentPhase(); got != "build" {
		t.Fatalf("CurrentPhase() = %q, want %q", got, "build")
	}
}

func TestStateApplyIncrementUsesAmount(t *testing.T) {
	t.Parallel()

	state := State{Total: 4}

	state.Apply(input.Event{Kind: input.KindIncrement, Amount: 2}, time.Unix(100, 0))

	if state.Value != 2 {
		t.Fatalf("Value = %v, want 2", state.Value)
	}

	if state.DisplayValue != 2 {
		t.Fatalf("DisplayValue = %v, want 2", state.DisplayValue)
	}

	if diff := math.Abs(state.Percent() - 50); diff > 0.01 {
		t.Fatalf("Percent() = %v, want about 50", state.Percent())
	}
}

func TestStateApplyValueReplacesValueAndDisplayValue(t *testing.T) {
	t.Parallel()

	state := State{Total: 10}

	state.Apply(input.Event{Kind: input.KindTick, Amount: 1}, time.Unix(100, 0))
	state.Apply(input.Event{Kind: input.KindValue, Value: 4}, time.Unix(101, 0))

	if state.Value != 4 {
		t.Fatalf("Value = %v, want 4", state.Value)
	}

	if state.DisplayValue != 4 {
		t.Fatalf("DisplayValue = %v, want 4", state.DisplayValue)
	}
}

func TestStateApplyPhaseUsesNameOrIndex(t *testing.T) {
	t.Parallel()

	t.Run("name wins", func(t *testing.T) {
		t.Parallel()

		state := State{Phases: []string{"build", "test", "package"}}
		state.Apply(input.Event{Kind: input.KindPhase, PhaseIndex: 0, PhaseName: "test"}, time.Unix(100, 0))

		if state.PhaseIndex != 1 {
			t.Fatalf("PhaseIndex = %d, want 1", state.PhaseIndex)
		}
		if got := state.CurrentPhase(); got != "test" {
			t.Fatalf("CurrentPhase() = %q, want %q", got, "test")
		}
	})

	t.Run("index fallback", func(t *testing.T) {
		t.Parallel()

		state := State{Phases: []string{"build", "test", "package"}}
		state.Apply(input.Event{Kind: input.KindPhase, PhaseIndex: 2}, time.Unix(100, 0))

		if state.PhaseIndex != 2 {
			t.Fatalf("PhaseIndex = %d, want 2", state.PhaseIndex)
		}
		if got := state.CurrentPhase(); got != "package" {
			t.Fatalf("CurrentPhase() = %q, want %q", got, "package")
		}
	})
}

func TestStateResetReplacesPlanAndResetsRateWindow(t *testing.T) {
	t.Parallel()

	state := State{
		Total:  3,
		Phases: []string{"build", "test", "package"},
	}

	state.Apply(input.Event{Kind: input.KindTick, Amount: 1}, time.Unix(100, 0))
	state.Apply(input.Event{
		Kind:   input.KindReset,
		Total:  5,
		Phases: []string{"fetch", "compile"},
	}, time.Unix(101, 0))

	if state.Total != 5 {
		t.Fatalf("Total = %d, want 5", state.Total)
	}

	if len(state.Phases) != 2 || state.Phases[0] != "fetch" || state.Phases[1] != "compile" {
		t.Fatalf("Phases = %#v, want %#v", state.Phases, []string{"fetch", "compile"})
	}

	if state.Value != 0 {
		t.Fatalf("Value = %v, want 0", state.Value)
	}

	if state.DisplayValue != 0 {
		t.Fatalf("DisplayValue = %v, want 0", state.DisplayValue)
	}

	if rate := state.RatePerSecond(); rate != 0 {
		t.Fatalf("RatePerSecond() = %v, want 0", rate)
	}
}

func TestStateResetWithNilPhasesKeepsExistingPlan(t *testing.T) {
	t.Parallel()

	state := State{
		Total:  3,
		Phases: []string{"build", "test", "package"},
	}

	state.Apply(input.Event{Kind: input.KindReset, Total: 5}, time.Unix(101, 0))

	if state.Total != 5 {
		t.Fatalf("Total = %d, want 5", state.Total)
	}

	if len(state.Phases) != 3 || state.Phases[0] != "build" || state.Phases[1] != "test" || state.Phases[2] != "package" {
		t.Fatalf("Phases = %#v, want existing plan to remain", state.Phases)
	}
}

func TestStateRatePerSecondUsesBoundedSampleWindow(t *testing.T) {
	t.Parallel()

	state := State{Total: 80}

	for i := 0; i < 8; i++ {
		state.Apply(input.Event{Kind: input.KindValue, Value: 0}, time.Unix(int64(i), 0))
	}
	state.Apply(input.Event{Kind: input.KindValue, Value: 80}, time.Unix(8, 0))

	if len(state.samples) != 8 {
		t.Fatalf("len(samples) = %d, want 8", len(state.samples))
	}

	if got := state.samples[0].at; !got.Equal(time.Unix(1, 0)) {
		t.Fatalf("oldest retained sample = %v, want %v", got, time.Unix(1, 0))
	}

	if got := state.samples[len(state.samples)-1].at; !got.Equal(time.Unix(8, 0)) {
		t.Fatalf("newest retained sample = %v, want %v", got, time.Unix(8, 0))
	}

	if diff := math.Abs(state.RatePerSecond() - (80.0 / 7.0)); diff > 0.01 {
		t.Fatalf("RatePerSecond() = %v, want about %v", state.RatePerSecond(), 80.0/7.0)
	}
}

func TestStateETAUsesFallbackTotalFromPhases(t *testing.T) {
	t.Parallel()

	state := State{
		Phases: []string{"build", "test", "package"},
	}

	state.Apply(input.Event{Kind: input.KindValue, Value: 1}, time.Unix(100, 0))
	state.Apply(input.Event{Kind: input.KindValue, Value: 2}, time.Unix(101, 0))

	if got := state.ETA(); got != time.Second {
		t.Fatalf("ETA() = %v, want 1s", got)
	}
}

func TestStateApplyTickAndIncrementSetLabelWhenPresent(t *testing.T) {
	t.Parallel()

	state := State{Total: 4, Label: "initial"}

	state.Apply(input.Event{Kind: input.KindTick, Amount: 1, Label: "compiling"}, time.Unix(100, 0))
	if state.Label != "compiling" {
		t.Fatalf("Label = %q after tick, want %q", state.Label, "compiling")
	}

	state.Apply(input.Event{Kind: input.KindTick, Amount: 1}, time.Unix(101, 0))
	if state.Label != "compiling" {
		t.Fatalf("Label = %q after unlabelled tick, want existing label kept", state.Label)
	}

	state.Apply(input.Event{Kind: input.KindIncrement, Amount: 1, Label: "linking"}, time.Unix(102, 0))
	if state.Label != "linking" {
		t.Fatalf("Label = %q after increment, want %q", state.Label, "linking")
	}

	state.Apply(input.Event{Kind: input.KindIncrement, Amount: 1}, time.Unix(103, 0))
	if state.Label != "linking" {
		t.Fatalf("Label = %q after unlabelled increment, want existing label kept", state.Label)
	}
}

func TestStateEffectiveTotalFallsBackToPhaseCount(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name  string
		state State
		want  int
	}{
		{name: "explicit total wins", state: State{Total: 7, Phases: []string{"a", "b"}}, want: 7},
		{name: "phase count fallback", state: State{Phases: []string{"a", "b", "c"}}, want: 3},
		{name: "nothing known", state: State{}, want: 0},
	}

	for _, tc := range tests {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			if got := tc.state.EffectiveTotal(); got != tc.want {
				t.Fatalf("EffectiveTotal() = %d, want %d", got, tc.want)
			}
		})
	}
}

func TestStateResetRestartsTimer(t *testing.T) {
	t.Parallel()

	state := State{Total: 3, StartedAt: time.Unix(100, 0)}

	state.Apply(input.Event{Kind: input.KindReset}, time.Unix(250, 0))

	if !state.StartedAt.Equal(time.Unix(250, 0)) {
		t.Fatalf("StartedAt = %v after reset, want %v", state.StartedAt, time.Unix(250, 0))
	}
}
