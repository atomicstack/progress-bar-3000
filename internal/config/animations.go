package config

import (
	"fmt"
	"slices"
	"strings"
)

var tintAnimations = []TintAnimation{
	TintAnimationPulse, TintAnimationShimmer, TintAnimationCycle,
	TintAnimationInterference, TintAnimationEdgeGlow, TintAnimationMilestoneRipple,
}

// ParseTintAnimations validates a comma-separated selection and normalizes
// whitespace, duplicates and ordering. An empty selection disables animations.
func ParseTintAnimations(value string) (TintAnimation, error) {
	if strings.TrimSpace(value) == "" {
		return "", nil
	}
	selected := make(map[TintAnimation]bool)
	for part := range strings.SplitSeq(value, ",") {
		name := TintAnimation(strings.TrimSpace(part))
		if !slices.Contains(tintAnimations, name) {
			return "", fmt.Errorf("--tint-animation: unknown animation %q; choose a comma-separated list of pulse, shimmer, cycle, interference, edge-glow, milestone-ripple", name)
		}
		selected[name] = true
	}
	var names []string
	for _, name := range tintAnimations {
		if selected[name] {
			names = append(names, string(name))
		}
	}
	return TintAnimation(strings.Join(names, ",")), nil
}

// Has reports whether the selection includes a layer, independently of order.
func (a TintAnimation) Has(name TintAnimation) bool {
	for part := range strings.SplitSeq(string(a), ",") {
		if strings.TrimSpace(part) == string(name) {
			return true
		}
	}
	return false
}
