package render

import (
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
