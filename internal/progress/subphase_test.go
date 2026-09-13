package progress

import (
	"testing"
	"time"

	"progress-bar-3000/internal/input"
)

func TestCombinedSubphaseUpdateRecordsOnlyFinalParentTransition(t *testing.T) {
	s := State{Total: 100, Phases: []string{"build", "test", "ship"}}
	s.Apply(input.Event{Kind: input.KindSetSubphases, Subphases: []string{"compile", "link"}}, time.Unix(1, 0))
	s.Apply(input.Event{Kind: input.KindValue, Value: 42, ParentPhase: "build", SubphaseName: "link"}, time.Unix(2, 0))
	if !s.PhaseChangedAt.IsZero() || s.PreviousPhaseIndex != 0 {
		t.Fatalf("same-parent update restarted the phase fade: previous=%d, at=%v", s.PreviousPhaseIndex, s.PhaseChangedAt)
	}
	s.Apply(input.Event{Kind: input.KindValue, Value: 43, ParentPhase: "test"}, time.Unix(3, 0))
	if s.PhaseIndex != 1 || s.PreviousPhaseIndex != 0 {
		t.Fatalf("combined update recorded an intermediate parent: current=%d previous=%d", s.PhaseIndex, s.PreviousPhaseIndex)
	}
}

func TestSubphaseStateOwnsPlansAndKeepsProgressSamples(t *testing.T) {
	plan := []string{"compile", "link"}
	s := State{}
	s.Apply(input.Event{Kind: input.KindReset, Phases: []string{"build", "test"}, PhaseSubphases: map[int][]string{0: plan}}, time.Unix(0, 0))
	plan[0] = "changed"
	if s.CurrentSubphase() != "compile" {
		t.Fatal("state retained a mutable reset plan")
	}
	s.Apply(input.Event{Kind: input.KindValue, Value: 0.25}, time.Unix(1, 0))
	s.Apply(input.Event{Kind: input.KindValue, Value: 0.5}, time.Unix(2, 0))
	rate, eta := s.RatePerSecond(), s.ETA()
	s.Apply(input.Event{Kind: input.KindSetSubphases, Subphases: plan}, time.Unix(9, 0))
	plan[0] = "changed again"
	if s.CurrentSubphase() != "changed" {
		t.Fatal("state retained a mutable runtime plan")
	}
	s.Apply(input.Event{Kind: input.KindSubphase, SubphaseIndex: 1}, time.Unix(10, 0))
	if s.Value != 0.5 || s.DisplayValue != 0.5 || s.EffectiveTotal() != 2 || s.RatePerSecond() != rate || s.ETA() != eta {
		t.Fatal("child update changed parent progress accounting")
	}
}
