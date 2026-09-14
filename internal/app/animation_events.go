package app

import (
	"math"
	"time"

	"progress-bar-3000/internal/config"
	"progress-bar-3000/internal/input"
	"progress-bar-3000/internal/render"
)

// animationEvents keeps only the newest event of each kind, bounding work and
// memory regardless of producer frequency. redraws never restart an effect.
type animationEvents struct {
	rippleAt       time.Time
	rippleOrigin   float64
	rippleStrength float64
	phaseAt        time.Time
}

func (m *Model) recordAnimationEvent(evt input.Event, at time.Time, previousValue float64, previousPhase, previousChild string, previousIndex int) {
	if evt.Kind == input.KindReset {
		m.animations = animationEvents{}
		return
	}
	if m.cfg.MilestoneRipple && (evt.Kind == input.KindValue || evt.Kind == input.KindTick || evt.Kind == input.KindIncrement) {
		total := m.state.EffectiveTotal()
		before, after := clampPercent(previousValue, total), clampPercent(m.state.Value, total)
		if total > 0 && after > before {
			m.animations.rippleAt = at
			m.animations.rippleOrigin = after
			m.animations.rippleStrength = math.Min(1, 0.3+2*(after-before))
		}
	}
	if m.cfg.PhaseTransition && previousPhase != "" && m.state.CurrentPhase() != "" &&
		(previousIndex != m.state.PhaseIndex || previousPhase != m.state.CurrentPhase() || previousChild != m.state.CurrentSubphase()) {
		m.animations.phaseAt = at
	}
}

func (a animationEvents) apply(opts *render.Options, cfg config.Config, at time.Time) {
	if cfg.MilestoneRipple && !a.rippleAt.IsZero() {
		age := at.Sub(a.rippleAt).Seconds()
		if age >= 0 && age < render.MilestoneRippleDuration {
			opts.RippleAge = age
			opts.RippleOrigin = a.rippleOrigin
			opts.RippleStrength = a.rippleStrength
		}
	}
	if cfg.PhaseTransition && !a.phaseAt.IsZero() {
		age := at.Sub(a.phaseAt).Seconds()
		if age >= 0 && age < render.PhaseTransitionDuration {
			opts.PhaseTransition = true
			opts.PhaseTransitionAge = age
		}
	}
}
