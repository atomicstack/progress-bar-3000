package render

import (
	"math"
	"strings"
	"testing"
)

func TestParseHexColor(t *testing.T) {
	got, err := ParseHexColor("#0087ff")
	if err != nil {
		t.Fatalf("ParseHexColor() error = %v", err)
	}
	if got != (RGB{R: 0x00, G: 0x87, B: 0xff}) {
		t.Fatalf("ParseHexColor() = %#v", got)
	}
}

func TestRenderBarUsesConfiguredBackgroundRune(t *testing.T) {
	got := StripANSI(RenderBar(Options{
		Width:           8,
		Percent:         0.5,
		Style:           StylePlain,
		BackgroundStyle: BackgroundCustom,
		BackgroundRune:  '░',
		Profile:         ProfileNone,
	}))

	if got != "====░░░░" {
		t.Fatalf("RenderBar() = %q, want %q", got, "====░░░░")
	}
}

func TestRenderBarGranularUsesPartialBlocks(t *testing.T) {
	got := StripANSI(RenderBar(Options{
		Width:           4,
		Percent:         0.375,
		Style:           StyleGranular,
		BackgroundStyle: BackgroundSpace,
		Profile:         ProfileNone,
	}))

	if !strings.HasPrefix(got, "█▌") {
		t.Fatalf("RenderBar() = %q, want prefix %q", got, "█▌")
	}
}

func TestRenderBarPartialCellCarriesBackgroundColor(t *testing.T) {
	got := RenderBar(Options{
		Width:           4,
		Percent:         0.375,
		Style:           StyleGradientGranular,
		BackgroundStyle: BackgroundCustom,
		BackgroundRune:  '░',
		Profile:         ProfileTrueColor,
		GradientStart:   RGB{R: 0xff, G: 0xff, B: 0xff},
		GradientEnd:     RGB{R: 0x00, G: 0x87, B: 0xff},
	})

	if !strings.Contains(got, "\x1b[48;2;") {
		t.Fatalf("RenderBar() = %q, want truecolor background escape for partial cell", got)
	}
}

func TestDetectProfileHonorsExplicitModes(t *testing.T) {
	if got := DetectProfile("truecolor", Profile16); got != ProfileTrueColor {
		t.Fatalf("DetectProfile(truecolor) = %q", got)
	}
	if got := DetectProfile("256", Profile16); got != Profile256 {
		t.Fatalf("DetectProfile(256) = %q", got)
	}
	if got := DetectProfile("auto", Profile16); got != Profile16 {
		t.Fatalf("DetectProfile(auto) = %q", got)
	}
}

func TestRenderBarASCIIForcesPlainGlyphs(t *testing.T) {
	tests := []struct {
		name string
		opts Options
		want string
	}{
		{
			name: "granular style with shaded background",
			opts: Options{
				Width:           4,
				Percent:         0.375,
				Style:           StyleGradientGranular,
				BackgroundStyle: BackgroundShadeLight,
				Profile:         ProfileNone,
				ASCII:           true,
			},
			want: "=...",
		},
		{
			name: "custom non-ascii background rune",
			opts: Options{
				Width:           4,
				Percent:         0.5,
				Style:           StyleBlock,
				BackgroundStyle: BackgroundCustom,
				BackgroundRune:  '░',
				Profile:         ProfileNone,
				ASCII:           true,
			},
			want: "==..",
		},
		{
			name: "custom ascii background rune is kept",
			opts: Options{
				Width:           4,
				Percent:         0.5,
				Style:           StyleShaded,
				BackgroundStyle: BackgroundCustom,
				BackgroundRune:  '#',
				Profile:         ProfileNone,
				ASCII:           true,
			},
			want: "==##",
		},
		{
			name: "space background stays a space",
			opts: Options{
				Width:           4,
				Percent:         0.5,
				Style:           StyleGradientBlock,
				BackgroundStyle: BackgroundSpace,
				Profile:         ProfileNone,
				ASCII:           true,
			},
			want: "==  ",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := StripANSI(RenderBar(tc.opts))
			if got != tc.want {
				t.Fatalf("RenderBar() = %q, want %q", got, tc.want)
			}
		})
	}
}

func TestRenderBarASCIIKeepsColour(t *testing.T) {
	got := RenderBar(Options{
		Width:           4,
		Percent:         0.5,
		Style:           StyleGradientBlock,
		BackgroundStyle: BackgroundShadeLight,
		Profile:         ProfileTrueColor,
		GradientStart:   RGB{R: 0xff, G: 0xff, B: 0xff},
		GradientEnd:     RGB{R: 0x00, G: 0x87, B: 0xff},
		ASCII:           true,
	})

	if !strings.Contains(got, "\x1b[38;2;") {
		t.Fatalf("RenderBar() = %q, want truecolor escapes to survive --ascii", got)
	}
	if StripANSI(got) != "==.." {
		t.Fatalf("RenderBar() stripped = %q, want %q", StripANSI(got), "==..")
	}
}

func TestRGBToHSLKnownValues(t *testing.T) {
	tests := []struct {
		name    string
		in      RGB
		h, s, l float64
	}{
		{name: "red", in: RGB{R: 255}, h: 0, s: 1, l: 0.5},
		{name: "green", in: RGB{G: 255}, h: 120, s: 1, l: 0.5},
		{name: "blue", in: RGB{B: 255}, h: 240, s: 1, l: 0.5},
		{name: "white", in: RGB{R: 255, G: 255, B: 255}, h: 0, s: 0, l: 1},
		{name: "black", in: RGB{}, h: 0, s: 0, l: 0},
		{name: "grey", in: RGB{R: 128, G: 128, B: 128}, h: 0, s: 0, l: 128.0 / 255},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			h, s, l := RGBToHSL(tt.in)
			if math.Abs(h-tt.h) > 0.01 || math.Abs(s-tt.s) > 0.01 || math.Abs(l-tt.l) > 0.01 {
				t.Fatalf("RGBToHSL(%v) = (%v, %v, %v), want (%v, %v, %v)", tt.in, h, s, l, tt.h, tt.s, tt.l)
			}
		})
	}
}

func TestHSLRoundTrip(t *testing.T) {
	colors := []RGB{
		{R: 0x00, G: 0x87, B: 0xff},
		{R: 0xff, G: 0x5f, B: 0x00},
		{R: 0x12, G: 0x34, B: 0x56},
		{R: 0xab, G: 0xcd, B: 0xef},
		{R: 0x80, G: 0x80, B: 0x80},
		{R: 0xff, G: 0xff, B: 0xff},
		{R: 0x00, G: 0x00, B: 0x00},
		{R: 0x01, G: 0xfe, B: 0x7f},
	}
	for _, c := range colors {
		h, s, l := RGBToHSL(c)
		got := HSLToRGB(h, s, l)
		if absDiff(got.R, c.R) > 1 || absDiff(got.G, c.G) > 1 || absDiff(got.B, c.B) > 1 {
			t.Fatalf("HSLToRGB(RGBToHSL(%v)) = %v, want within 1 of original", c, got)
		}
	}
}

func TestHSLToRGBClampsInputs(t *testing.T) {
	got := HSLToRGB(0, 2, 0.5)
	if got != (RGB{R: 255}) {
		t.Fatalf("HSLToRGB(0, 2, 0.5) = %v, want pure red (saturation clamped to 1)", got)
	}
	got = HSLToRGB(0, -1, 0.5)
	if got.R != got.G || got.G != got.B {
		t.Fatalf("HSLToRGB(0, -1, 0.5) = %v, want grey (saturation clamped to 0)", got)
	}
}

func absDiff(a, b uint8) int {
	if a > b {
		return int(a - b)
	}
	return int(b - a)
}
