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
	bgColor := trackColor(base)
	fgColor := BlendTowardWhite(bgColor, 0.28)
	return Background(opts.Profile, bgColor) + Foreground(opts.Profile, fgColor) + cell + Reset()
}

func animatedFillColor(opts Options, index int) RGB {
	color := gradientColor(opts, index)
	if opts.Pulse > 0 {
		color = BlendTowardWhite(color, opts.Pulse)
	}
	if opts.ShimmerPhase >= 0 {
		position := opts.ShimmerPhase * float64(maxInt(opts.Width-1, 1))
		dist := math.Abs(float64(index) - position)
		if dist < 3 {
			color = BlendTowardWhite(color, (1-(dist/3))*0.45)
		}
	}
	return color
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

	return lerp(start, end, shiftedIndex(index, maxInt(opts.Width-1, 1), opts.GradientShift), maxInt(opts.Width-1, 1))
}

func trackColor(fill RGB) RGB {
	return Scale(fill, 0.18)
}

func lerp(a, b RGB, index, width int) RGB {
	t := float64(index) / float64(width)
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

func shiftedIndex(index, width int, shift float64) int {
	if shift == 0 || width <= 0 {
		return index
	}
	normalized := math.Mod((float64(index)/float64(width))+shift, 1.0)
	if normalized < 0 {
		normalized += 1
	}
	return int(math.Round(normalized * float64(width)))
}
