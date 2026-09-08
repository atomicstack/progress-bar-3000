package progress

import (
	"math"
	"time"

	"progress-bar-3000/internal/input"
)

const rateSampleLimit = 8

type State struct {
	Total        int
	Value        float64
	DisplayValue float64
	Phases       []string
	PhaseIndex   int
	// PreviousPhaseIndex and PhaseChangedAt describe the most recent phase
	// transition so renderers can crossfade between the two phases.
	PreviousPhaseIndex int
	PhaseChangedAt     time.Time
	Label              string
	Meta               map[string]string
	// StartedAt anchors the %{timer} token: the moment the state was
	// bootstrapped or last reset. A zero value renders as "0s".
	StartedAt time.Time

	updatedAt time.Time
	samples   []sample
}

type sample struct {
	at    time.Time
	value float64
}

func (s *State) Apply(evt input.Event, now time.Time) {
	switch evt.Kind {
	case input.KindTick, input.KindIncrement:
		delta := evt.Amount
		if evt.Kind == input.KindTick && delta == 0 {
			delta = 1
		}
		s.setValue(s.Value+delta, now)
		// A plain input line ticks and carries its text as the label; an
		// unlabelled tick must not wipe the label a previous one set.
		if evt.Label != "" {
			s.Label = evt.Label
		}
	case input.KindValue:
		s.setValue(evt.Value, now)
	case input.KindSetTotal:
		s.Total = evt.Total
	case input.KindPhase:
		s.applyPhase(evt, now)
	case input.KindLabel:
		s.Label = evt.Label
	case input.KindMeta:
		s.mergeMeta(evt.Meta)
	case input.KindReset:
		s.reset(evt, now)
	}

	s.updatedAt = now
}

// EffectiveTotal is the denominator every progress calculation shares: the
// explicit Total when one is set, otherwise the phase count, otherwise 0.
func (s *State) EffectiveTotal() int {
	if s.Total > 0 {
		return s.Total
	}
	if len(s.Phases) > 0 {
		return len(s.Phases)
	}
	return 0
}

func (s *State) Percent() float64 {
	total := s.EffectiveTotal()
	if total <= 0 {
		return 0
	}
	return (s.Value / float64(total)) * 100
}

func (s *State) CurrentPhase() string {
	if len(s.Phases) == 0 {
		return ""
	}

	index := s.PhaseIndex
	if index < 0 || index >= len(s.Phases) {
		index = phaseIndexForValue(s.Value, len(s.Phases))
	}
	if index < 0 || index >= len(s.Phases) {
		return ""
	}
	return s.Phases[index]
}

func (s *State) RatePerSecond() float64 {
	if len(s.samples) < 2 {
		return 0
	}

	// Use the oldest retained sample and the newest retained sample; the window is bounded
	// so stale history cannot keep influencing rate/ETA after a reset or long run.
	first := s.samples[0]
	last := s.samples[len(s.samples)-1]
	elapsed := last.at.Sub(first.at).Seconds()
	if elapsed <= 0 {
		return 0
	}

	return (last.value - first.value) / elapsed
}

func (s *State) ETA() time.Duration {
	rate := s.RatePerSecond()
	if rate <= 0 {
		return 0
	}

	remaining := float64(s.EffectiveTotal()) - s.Value
	if remaining <= 0 {
		return 0
	}

	return time.Duration((remaining / rate) * float64(time.Second))
}

func (s *State) setValue(value float64, now time.Time) {
	s.Value = value
	s.DisplayValue = value
	s.setPhaseIndex(phaseIndexForValue(value, len(s.Phases)), now)
	s.recordSample(value, now)
}

func (s *State) applyPhase(evt input.Event, now time.Time) {
	if len(s.Phases) == 0 {
		s.setPhaseIndex(0, now)
		return
	}

	// Explicit phase names win when present; otherwise fall back to the provided index.
	if evt.PhaseName != "" {
		for i, phase := range s.Phases {
			if phase == evt.PhaseName {
				s.setPhaseIndex(i, now)
				return
			}
		}
	}

	if evt.PhaseIndex >= 0 && evt.PhaseIndex < len(s.Phases) {
		s.setPhaseIndex(evt.PhaseIndex, now)
	}
}

// setPhaseIndex is the single place PhaseIndex changes, so every transition
// (name, index, value-driven or reset) records where it came from and when.
// A no-op assignment leaves the previous transition untouched so an
// in-progress crossfade is not restarted.
func (s *State) setPhaseIndex(index int, now time.Time) {
	if index == s.PhaseIndex {
		return
	}
	s.PreviousPhaseIndex = s.PhaseIndex
	s.PhaseIndex = index
	s.PhaseChangedAt = now
}

func (s *State) mergeMeta(meta map[string]string) {
	if len(meta) == 0 {
		return
	}
	if s.Meta == nil {
		s.Meta = make(map[string]string, len(meta))
	}
	for k, v := range meta {
		s.Meta[k] = v
	}
}

func (s *State) reset(evt input.Event, now time.Time) {
	s.Value = 0
	s.DisplayValue = 0
	s.Label = ""
	s.Meta = nil
	s.samples = nil
	s.StartedAt = now

	if evt.Total > 0 {
		s.Total = evt.Total
	}
	// A nil phase list means "keep the current plan"; an explicit empty slice clears it.
	if evt.Phases != nil {
		s.Phases = append([]string(nil), evt.Phases...)
	}
	s.setPhaseIndex(phaseIndexForValue(s.Value, len(s.Phases)), now)

	s.updatedAt = now
}

func (s *State) recordSample(value float64, now time.Time) {
	s.samples = append(s.samples, sample{at: now, value: value})
	if len(s.samples) > rateSampleLimit {
		s.samples = append([]sample(nil), s.samples[len(s.samples)-rateSampleLimit:]...)
	}
}

func phaseIndexForValue(value float64, phaseCount int) int {
	if phaseCount <= 0 {
		return 0
	}

	// Value 1 maps to phase 0, value 2 maps to phase 1, and so on; clamp to the
	// available range so the final phase stays selected once the value runs past it.
	index := int(math.Floor(value))
	if index > 0 {
		index--
	}
	if index < 0 {
		index = 0
	}
	if index >= phaseCount {
		index = phaseCount - 1
	}
	return index
}
