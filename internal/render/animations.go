package render

import "math"

// MilestoneRippleDuration is measured in seconds.
const MilestoneRippleDuration = 1.4

// ambientColor only runs for filled glyphs. The track keeps its original dim
// gradient, so moving light cannot imply progress that has not happened.
func ambientColor(opts Options, index int, base RGB) RGB {
	if opts.Width <= 0 || opts.Percent <= 0 {
		return base
	}
	x := (float64(index) + 0.5) / float64(opts.Width)
	t := opts.AnimationTime
	if opts.Interference {
		a := 0.5 + 0.5*math.Sin(2*math.Pi*(2.3*x-0.19*t))
		b := 0.5 + 0.5*math.Sin(2*math.Pi*(2.7*x+0.16*t))
		waves := Scale(base, 0.76+0.24*(a+b)/2)
		return BlendTowardWhite(waves, 0.55*math.Pow(a*b, 3))
	}
	return base
}

func edgeGlowColor(opts Options, index int, base RGB) RGB {
	if opts.Width <= 0 || opts.Percent <= 0 || !opts.EdgeGlow {
		return base
	}
	x := (float64(index) + 0.5) / float64(opts.Width)
	t := opts.AnimationTime
	filled := clampUnit(opts.Percent)
	breath := 0.5 + 0.5*math.Sin(2*math.Pi*t/2.8)
	radius := math.Max(2/float64(opts.Width), 0.09+0.035*breath)
	halo := softBand(x-filled, radius)
	return BlendTowardWhite(base, halo*(0.3+0.65*breath))
}

func eventColor(opts Options, index int, base RGB) RGB {
	if opts.Width <= 0 || opts.Percent <= 0 {
		return base
	}
	x := (float64(index) + 0.5) / float64(opts.Width)
	rippleActive := opts.RippleStrength > 0 && opts.RippleAge >= 0 && opts.RippleAge < MilestoneRippleDuration
	if rippleActive {
		age := opts.RippleAge / MilestoneRippleDuration
		position := clampUnit(opts.RippleOrigin) * (1 - age)
		radius := math.Max(1.5/float64(opts.Width), 0.075)
		intensity := softBand(x-position, radius) * (1 - age) * clampUnit(opts.RippleStrength)
		base = BlendTowardWhite(base, 0.95*intensity)
	}
	return base
}

// softBand has finite support, leaving cells outside the effect byte-identical.
func softBand(distance, radius float64) float64 {
	if math.Abs(distance) >= radius {
		return 0
	}
	return 0.5 * (1 + math.Cos(math.Pi*distance/radius))
}
