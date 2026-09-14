package app

import (
	"progress-bar-3000/internal/config"
	"progress-bar-3000/internal/input"
	"progress-bar-3000/internal/render"
	"testing"
	"time"
)

func TestAnimationListComposesWithFixedOrder(t *testing.T) {
	view := func(selection string) string {
		m := animationModel()
		m.cfg.TintAnimation = config.TintAnimation(selection)
		at := time.Unix(100, 0)
		m = animationUpdate(m, eventMsg{Event: input.Event{Kind: input.KindValue, Value: 60}, Now: at})
		m = animationUpdate(m, frameMsg{Now: at.Add(350 * time.Millisecond)})
		return m.View()
	}
	all := view("interference,edge-glow,milestone-ripple")
	if all != view("milestone-ripple,edge-glow,interference,edge-glow") {
		t.Fatal("order or duplicates changed rendering")
	}
	for _, selection := range []string{"", "interference,edge-glow", "interference,milestone-ripple", "edge-glow,milestone-ripple"} {
		other := view(selection)
		if all == other {
			t.Fatalf("missing visible layer when comparing with %q", selection)
		}
		if render.StripANSI(all) != render.StripANSI(other) {
			t.Fatal("composition changed fill geometry")
		}
	}
}

func TestLegacyAnimationListComposes(t *testing.T) {
	cfg := config.Config{TintAnimation: "pulse,shimmer,cycle"}
	pulse, shimmer, shift := animationState(cfg, 1.0)
	if pulse <= 0 || shimmer <= 0 || shift <= 0 {
		t.Fatal("legacy layers did not compose")
	}
}
