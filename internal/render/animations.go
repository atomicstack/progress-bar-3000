package render

import "math"

// Event animation durations are measured in seconds.
const (
	MilestoneRippleDuration = 1.4
	PhaseTransitionDuration = 1.2
)

// ambientColor only runs for filled glyphs. The track keeps its original dim
// gradient, so moving light cannot imply progress that has not happened.
func ambientColor(opts Options, index int, base RGB) RGB {
	if opts.Width <= 0 || opts.Percent <= 0 {
		return base
	}
	x := (float64(index) + 0.5) / float64(opts.Width)
	t := opts.AnimationTime
	filled := clampUnit(opts.Percent)
	switch opts.Animation {
	case "aurora":
		// Overlapping broad curtains travel at different speeds. Their hue is
		// anchored to the configured gradient, with saturated cool excursions.
		a := math.Sin(2 * math.Pi * (1.15*x - 0.085*t))
		b := math.Sin(2 * math.Pi * (2.1*x + 0.047*t))
		h, s, _ := RGBToHSL(gradientColor(opts, opts.Width-1))
		curtain := HSLToRGB(h+78*a+27*b, math.Max(s, 0.8), 0.48+0.09*b)
		return lerpRGB(base, curtain, 0.72+0.12*a)
	case "comet":
		// The head enters and exits beyond the filled span; its tail always
		// trails behind it. A soft front avoids an abrupt wrap at the edge.
		u := x / filled
		position := -0.25 + 1.6*unitCycle(t/3.2)
		distance := position - u
		head := math.Exp(-math.Pow(distance/0.045, 2))
		tail := 0.0
		if distance >= 0 {
			tail = math.Exp(-distance / 0.16)
		}
		return BlendTowardWhite(base, clampUnit(0.9*head+0.48*tail))
	case "interference":
		a := 0.5 + 0.5*math.Sin(2*math.Pi*(2.3*x-0.19*t))
		b := 0.5 + 0.5*math.Sin(2*math.Pi*(2.7*x+0.16*t))
		waves := Scale(base, 0.76+0.24*(a+b)/2)
		return BlendTowardWhite(waves, 0.55*math.Pow(a*b, 3))
	case "embers":
		// Per-cell phases and rates create stable scattered sparks without
		// random state, while the edge envelope concentrates the activity.
		seed := unitCycle(math.Sin(float64(index+1)*127.1) * 43758.5453)
		rate := 0.22 + 0.31*seed
		spark := math.Pow(0.5+0.5*math.Sin(2*math.Pi*(t*rate+seed)), 18)
		edge := 0.22 + 0.78*math.Exp(-(filled-x)/0.14)
		warm := lerpRGB(base, RGB{R: 255, G: 91, B: 18}, 0.24)
		return lerpRGB(warm, RGB{R: 255, G: 224, B: 135}, clampUnit(spark*edge))
	case "liquid":
		// A slowly moving field bends another field, creating smooth eddies
		// rather than evenly spaced bands or discrete sparks.
		warp := math.Sin(2 * math.Pi * (1.2*x + 0.055*t))
		eddy := math.Sin(2*math.Pi*(2*x-0.09*t) + 1.6*warp)
		h, s, l := RGBToHSL(base)
		fluid := HSLToRGB(h+38*eddy, math.Max(0.65, s), 0.45+0.1*warp)
		return lerpRGB(base, fluid, 0.65+0.15*math.Sin(t*0.4+x*5+l))
	case "edge-glow":
		breath := 0.5 + 0.5*math.Sin(2*math.Pi*t/2.8)
		radius := math.Max(2/float64(opts.Width), 0.09+0.035*breath)
		halo := softBand(x-filled, radius)
		return BlendTowardWhite(base, halo*(0.3+0.65*breath))
	default:
		return base
	}
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
	phaseActive := opts.PhaseTransition && opts.PhaseTransitionAge >= 0 &&
		opts.PhaseTransitionAge < PhaseTransitionDuration
	if phaseActive {
		age := opts.PhaseTransitionAge / PhaseTransitionDuration
		position := -0.18 + 1.36*age
		intensity := softBand(x-position, 0.18) * math.Sin(math.Pi*age)
		h, s, _ := RGBToHSL(base)
		wave := HSLToRGB(h+65, math.Max(s, 0.72), 0.68)
		base = lerpRGB(base, wave, 0.85*intensity)
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

func unitCycle(value float64) float64 {
	return value - math.Floor(value)
}
