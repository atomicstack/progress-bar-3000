package render

import (
	"fmt"
	"progress-bar-3000/internal/config"
	"strings"
	"testing"
	"unicode/utf8"
)

var ambientAnimations = []string{"interference", "edge-glow", "interference,edge-glow"}

func animationOptions() Options {
	return Options{Width: 80, Percent: 0.675, Style: StyleGradientGranular,
		BackgroundStyle: BackgroundShadeLight, Profile: ProfileTrueColor,
		GradientStart: RGB{R: 38, G: 210, B: 170}, GradientEnd: RGB{R: 130, G: 40, B: 240},
		ShimmerPhase: -1}
}

func TestAmbientAnimationsMoveDeterministically(t *testing.T) {
	t.Parallel()
	outputs := make(map[string]string)
	for _, name := range ambientAnimations {
		t.Run(name, func(t *testing.T) {
			opts := animationOptions()
			baseline := RenderBar(opts)
			setAnimation(&opts, name)
			opts.AnimationTime = 0.37
			first := RenderBar(opts)
			opts.AnimationTime = 1.19
			second := RenderBar(opts)
			if first == baseline || first == second {
				t.Fatal("animation must visibly change colour over time")
			}
			opts.AnimationTime = 0.37
			if first != RenderBar(opts) {
				t.Fatal("same time and input must produce identical output")
			}
			for other, output := range outputs {
				if first == output {
					t.Fatalf("animation duplicates %s", other)
				}
			}
			outputs[name] = first
		})
	}
}

func TestAnimationsPreserveFillAndUnfilledTrack(t *testing.T) {
	t.Parallel()
	styles := []Style{StylePlain, StyleBlock, StyleGranular, StyleShaded,
		StyleGradientBlock, StyleGradientGranular, StyleGradientShaded}
	for _, style := range styles {
		for _, width := range []int{0, 1, 2, 17} {
			for _, percent := range []float64{0, 0.031, 0.375, 1} {
				for _, ascii := range []bool{false, true} {
					for _, profile := range []Profile{ProfileNone, ProfileTrueColor, Profile256, Profile16} {
						name := fmt.Sprintf("%s/width_%d/fill_%g/ascii_%t/%s", style, width, percent, ascii, profile)
						t.Run(name, func(t *testing.T) {
							opts := animationOptions()
							opts.Style, opts.Width, opts.Percent, opts.ASCII, opts.Profile = style, width, percent, ascii, profile
							baseline := RenderBar(opts)
							for _, animation := range ambientAnimations {
								setAnimation(&opts, animation)
								opts.AnimationTime = 0.37
								opts.RippleOrigin, opts.RippleStrength, opts.RippleAge = percent, 0.8, 0.2
								got := RenderBar(opts)
								if StripANSI(got) != StripANSI(baseline) || utf8.RuneCountInString(StripANSI(got)) != width {
									t.Fatalf("%s changes fill or width: %q vs %q", animation, StripANSI(got), StripANSI(baseline))
								}
								if profile == ProfileNone && got != baseline {
									t.Fatal("no-colour output changed")
								}
								if percent == 0 && got != baseline {
									t.Fatal("empty bar must not animate")
								}
								// Each coloured cell ends with reset. The unfilled suffix must be byte-identical.
								before, after := strings.Split(baseline, Reset()), strings.Split(got, Reset())
								for i, cell := range before {
									if strings.Contains(StripANSI(cell), ".") || strings.Contains(StripANSI(cell), "░") {
										// Shaded partial cells also use ░, so check only cells after the fill boundary.
										if float64(i) >= float64(width)*percent && after[i] != cell {
											t.Fatalf("%s illuminates unfilled cell %d", animation, i)
										}
									}
								}
							}
						})
					}
				}
			}
		}
	}
}

func TestEventAnimationsComposeAndExpire(t *testing.T) {
	t.Parallel()
	for _, ambient := range append([]string{""}, ambientAnimations...) {
		t.Run(ambient, func(t *testing.T) {
			opts := animationOptions()
			setAnimation(&opts, ambient)
			opts.AnimationTime = 0.37
			baseline := RenderBar(opts)
			opts.RippleOrigin, opts.RippleStrength, opts.RippleAge = opts.Percent, 0.8, 0.3
			ripple := RenderBar(opts)
			if ripple == baseline {
				t.Fatal("active ripple must be visible")
			}
			opts.RippleStrength = 0.2
			if RenderBar(opts) == ripple {
				t.Fatal("ripple strength must affect intensity")
			}
			opts.RippleStrength, opts.RippleAge = 0.8, MilestoneRippleDuration
			if RenderBar(opts) != baseline {
				t.Fatal("expired events must restore exact ambient output")
			}
			opts.RippleAge = -1
			if RenderBar(opts) != baseline {
				t.Fatal("events must not render before their start")
			}
		})
	}
}

func TestMilestoneRippleTravelsBackwardFromOriginalProgress(t *testing.T) {
	t.Parallel()
	opts := animationOptions()
	opts.Width, opts.Percent = 100, 1
	baseline := RenderBar(opts)
	opts.RippleOrigin, opts.RippleStrength = 0.7, 1
	strongest := func(age float64) int {
		opts.RippleAge = age
		before := strings.Split(baseline, Reset())
		after := strings.Split(RenderBar(opts), Reset())
		last := -1
		for i := range before {
			if before[i] != after[i] {
				last = i
			}
		}
		return last
	}
	// Later support must move left, even when progress has advanced beyond the origin.
	early, late := strongest(0.3), strongest(0.8)
	if early < 0 || late < 0 || late >= early {
		t.Fatalf("ripple must travel backward: early=%d late=%d", early, late)
	}
}

func TestAnimationsRetainConfiguredGradientInfluence(t *testing.T) {
	t.Parallel()
	for _, name := range ambientAnimations {
		t.Run(name, func(t *testing.T) {
			opts := animationOptions()
			setAnimation(&opts, name)
			opts.AnimationTime = 0.37
			first := RenderBar(opts)
			opts.GradientStart, opts.GradientEnd = RGB{R: 220, G: 30, B: 20}, RGB{R: 250, G: 150, B: 15}
			if first == RenderBar(opts) {
				t.Fatal("animation discards configured gradient")
			}
		})
	}
}

func setAnimation(opts *Options, name string) {
	selection := config.TintAnimation(name)
	opts.Interference = selection.Has(config.TintAnimationInterference)
	opts.EdgeGlow = selection.Has(config.TintAnimationEdgeGlow)
}
