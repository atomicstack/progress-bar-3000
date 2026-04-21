package render

import (
	"math"
	"regexp"
	"strings"
)

var ansiPattern = regexp.MustCompile(`\x1b\[[0-9;]*m`)

func StripANSI(in string) string {
	return ansiPattern.ReplaceAllString(in, "")
}

func RenderBar(opts Options) string {
	if opts.Width <= 0 {
		return ""
	}

	if opts.Percent < 0 {
		opts.Percent = 0
	}
	if opts.Percent > 1 {
		opts.Percent = 1
	}

	exact := float64(opts.Width) * opts.Percent
	whole := int(exact)
	frac := exact - float64(whole)
	partialIndex := int(math.Round(frac * 8))
	if partialIndex > 7 {
		partialIndex = 7
	}

	fillRune, partialRunes := fillGlyphs(opts.Style)
	bgRune := backgroundRune(opts)

	var out strings.Builder
	for i := 0; i < whole && i < opts.Width; i++ {
		out.WriteString(colorizeFilled(opts, i, string(fillRune)))
	}

	if whole < opts.Width && partialIndex > 0 && usesPartial(opts.Style) {
		out.WriteString(colorizeFilled(opts, whole, string(partialRunes[partialIndex-1])))
		whole++
	}

	for i := whole; i < opts.Width; i++ {
		out.WriteString(colorizeBackground(opts, i, bgRune))
	}

	return out.String()
}

func fillGlyphs(style Style) (rune, []rune) {
	switch style {
	case StyleGranular, StyleGradientGranular:
		return '█', []rune{'▏', '▎', '▍', '▌', '▋', '▊', '▉'}
	case StyleShaded, StyleGradientShaded:
		return '█', []rune{'░', '▒', '▓', '▓', '▓', '▓', '▓'}
	case StyleBlock, StyleGradientBlock:
		return '█', nil
	default:
		return '=', nil
	}
}

func usesPartial(style Style) bool {
	switch style {
	case StyleGranular, StyleGradientGranular, StyleShaded, StyleGradientShaded:
		return true
	default:
		return false
	}
}

func backgroundRune(opts Options) rune {
	switch opts.BackgroundStyle {
	case BackgroundNone, BackgroundSpace:
		return ' '
	case BackgroundASCII:
		return '.'
	case BackgroundShadeLight:
		return '░'
	case BackgroundShadeMedium:
		return '▒'
	case BackgroundShadeDark:
		return '▓'
	case BackgroundCustom:
		if opts.BackgroundRune != 0 {
			return opts.BackgroundRune
		}
	}
	return ' '
}

func colorizeFilled(opts Options, index int, cell string) string {
	if opts.Profile == ProfileNone {
		return cell
	}

	fillColor := animatedFillColor(opts, index)
	trackColor := trackColor(fillColor)
	return Background(opts.Profile, trackColor) + Foreground(opts.Profile, fillColor) + cell + Reset()
}

func colorizeBackground(opts Options, index int, bgRune rune) string {
	cell := string(bgRune)
	if opts.BackgroundStyle == BackgroundNone || opts.Profile == ProfileNone {
		return cell
	}

	base := gradientColor(opts, index)
	if boost := shimmerBoost(opts, index); boost > 0 {
		base = BlendTowardWhite(base, boost*0.25)
	}
	bgColor := trackColor(base)
	fgColor := BlendTowardWhite(bgColor, 0.28)
	return Background(opts.Profile, bgColor) + Foreground(opts.Profile, fgColor) + cell + Reset()
}

func animatedFillColor(opts Options, index int) RGB {
	color := gradientColor(opts, index)
	if opts.Pulse > 0 {
		color = BlendTowardWhite(color, opts.Pulse)
	}
	if boost := shimmerBoost(opts, index); boost > 0 {
		color = BlendTowardWhite(color, boost*0.45)
	}
	return color
}

// shimmerBoost returns a 0..1 intensity for a bright band sweeping across the
// bar. The band travels from just off the left edge to just off the right
// edge (rather than wrapping instantly), so it enters and exits smoothly.
// Cosine falloff keeps the band soft instead of a hard triangle.
const shimmerMargin = 4.0

func shimmerBoost(opts Options, index int) float64 {
	if opts.ShimmerPhase < 0 {
		return 0
	}
	travel := float64(opts.Width) + 2*shimmerMargin
	position := -shimmerMargin + opts.ShimmerPhase*travel
	dist := math.Abs(float64(index) - position)
	if dist >= shimmerMargin {
		return 0
	}
	return 0.5 * (1 + math.Cos(math.Pi*dist/shimmerMargin))
}

func gradientColor(opts Options, index int) RGB {
	start := opts.GradientStart
	end := opts.GradientEnd
	if start == (RGB{}) {
		start = RGB{R: 255, G: 255, B: 255}
	}
	if end == (RGB{}) {
		end = RGB{R: 0, G: 135, B: 255}
	}

	width := float64(maxInt(opts.Width-1, 1))
	t := float64(index) / width
	if opts.GradientShift != 0 {
		// The configured gradient is start→end (not cyclic), so rotating a
		// simple offset creates a visible seam where end jumps back to start.
		// Fold the shifted position through a triangle wave (start→end→start)
		// so it tiles seamlessly as the shift sweeps around.
		u := math.Mod(t+opts.GradientShift, 1.0)
		if u < 0 {
			u += 1
		}
		if u > 0.5 {
			u = 1 - u
		}
		t = 2 * u
	}
	return lerpRGB(start, end, t)
}

func trackColor(fill RGB) RGB {
	return Scale(fill, 0.18)
}

func lerpRGB(a, b RGB, t float64) RGB {
	if t < 0 {
		t = 0
	}
	if t > 1 {
		t = 1
	}
	return RGB{
		R: uint8(float64(a.R) + (float64(b.R)-float64(a.R))*t),
		G: uint8(float64(a.G) + (float64(b.G)-float64(a.G))*t),
		B: uint8(float64(a.B) + (float64(b.B)-float64(a.B))*t),
	}
}

func maxInt(a, b int) int {
	if a > b {
		return a
	}
	return b
}
