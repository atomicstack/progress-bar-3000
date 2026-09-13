package config

import (
	"fmt"
	"path/filepath"
	"strings"

	fmtx "progress-bar-3000/internal/format"
)

type InputMode string

const (
	InputModeAuto    InputMode = "auto"
	InputModeLines   InputMode = "lines"
	InputModeValue   InputMode = "value"
	InputModeJSON    InputMode = "json"
	InputModeControl InputMode = "control"
)

type Style string

const (
	StylePlain            Style = "plain"
	StyleBlock            Style = "block"
	StyleGranular         Style = "granular"
	StyleShaded           Style = "shaded"
	StyleGradientBlock    Style = "gradient-block"
	StyleGradientGranular Style = "gradient-granular"
	StyleGradientShaded   Style = "gradient-shaded"
)

type BackgroundStyle string

const (
	BackgroundStyleNone        BackgroundStyle = "none"
	BackgroundStyleSpace       BackgroundStyle = "space"
	BackgroundStyleASCII       BackgroundStyle = "ascii"
	BackgroundStyleShadeLight  BackgroundStyle = "shade-light"
	BackgroundStyleShadeMedium BackgroundStyle = "shade-medium"
	BackgroundStyleShadeDark   BackgroundStyle = "shade-dark"
	BackgroundStyleCustom      BackgroundStyle = "custom"
)

type ColorMode string

const (
	ColorModeAuto      ColorMode = "auto"
	ColorModeTrueColor ColorMode = "truecolor"
	ColorMode256       ColorMode = "256"
	ColorMode16        ColorMode = "16"
	ColorModeNone      ColorMode = "none"
)

type TintAnimation string

const (
	TintAnimationPulse   TintAnimation = "pulse"
	TintAnimationShimmer TintAnimation = "shimmer"
	TintAnimationCycle   TintAnimation = "cycle"
)

// DetailKey identifies a single detail row that can be rendered below the bar.
type DetailKey string

const (
	DetailLabel DetailKey = "label"
	DetailPhase DetailKey = "phase"
	DetailValue DetailKey = "value"
)

// DetailAll is a flag-only shorthand that expands to every DetailKey.
const DetailAll = "all"

// DetailRenderOrder is the canonical top-to-bottom order in which detail
// rows appear when multiple keys are selected.
var DetailRenderOrder = []DetailKey{DetailPhase, DetailValue, DetailLabel}

// ParseDetail parses the raw --detail flag value (a comma-separated list of
// DetailKey tokens, optionally including the "all" shorthand) into the set
// of keys to render, returned in DetailRenderOrder. An empty string returns
// nil (no detail rows). Unknown tokens produce an error.
func ParseDetail(s string) ([]DetailKey, error) {
	if s == "" {
		return nil, nil
	}
	known := map[string]DetailKey{
		string(DetailLabel): DetailLabel,
		string(DetailPhase): DetailPhase,
		string(DetailValue): DetailValue,
	}
	selected := map[DetailKey]bool{}
	for raw := range strings.SplitSeq(s, ",") {
		tok := strings.TrimSpace(raw)
		if tok == "" {
			continue
		}
		if tok == DetailAll {
			for _, k := range DetailRenderOrder {
				selected[k] = true
			}
			continue
		}
		key, ok := known[tok]
		if !ok {
			return nil, fmt.Errorf("--detail: unknown value %q (want label, phase, value, or all)", tok)
		}
		selected[key] = true
	}
	out := make([]DetailKey, 0, len(selected))
	for _, k := range DetailRenderOrder {
		if selected[k] {
			out = append(out, k)
		}
	}
	return out, nil
}

type Config struct {
	Total           int
	Current         int
	InputMode       InputMode
	Format          string
	Style           Style
	BackgroundStyle BackgroundStyle
	BackgroundRune  rune
	ColorMode       ColorMode
	GradientStart   string
	GradientEnd     string
	PhaseFile       string
	Phase           string
	SocketPath      string
	OnComplete      string
	Width           int
	Detail          string
	DetailFormats   []string
	ClearOnExit     bool
	FPS             int
	Lerp            float64
	TintAnimation   TintAnimation
	ASCII           bool
}

func (c Config) Validate() error {
	switch c.InputMode {
	case InputModeAuto, InputModeLines, InputModeValue, InputModeJSON, InputModeControl:
	default:
		return fmt.Errorf("--input-mode must be one of auto, lines, value, json, control")
	}

	switch c.Style {
	case StylePlain, StyleBlock, StyleGranular, StyleShaded, StyleGradientBlock, StyleGradientGranular, StyleGradientShaded:
	default:
		return fmt.Errorf("--style must be one of plain, block, granular, shaded, gradient-block, gradient-granular, gradient-shaded")
	}

	switch c.BackgroundStyle {
	case BackgroundStyleNone, BackgroundStyleSpace, BackgroundStyleASCII, BackgroundStyleShadeLight, BackgroundStyleShadeMedium, BackgroundStyleShadeDark, BackgroundStyleCustom:
	default:
		return fmt.Errorf("--bg-style must be one of none, space, ascii, shade-light, shade-medium, shade-dark, custom")
	}

	switch c.ColorMode {
	case ColorModeAuto, ColorModeTrueColor, ColorMode256, ColorMode16, ColorModeNone:
	default:
		return fmt.Errorf("--color-mode must be one of auto, truecolor, 256, 16, none")
	}

	switch c.TintAnimation {
	case "", TintAnimationPulse, TintAnimationShimmer, TintAnimationCycle:
	default:
		return fmt.Errorf("--tint-animation must be one of pulse, shimmer, or cycle")
	}

	if _, err := ParseDetail(c.Detail); err != nil {
		return err
	}

	for i, tpl := range c.DetailFormats {
		if _, err := fmtx.Parse(tpl); err != nil {
			return fmt.Errorf("--detail-format[%d] %q: %w", i, tpl, err)
		}
	}

	switch c.FPS {
	case 15, 30, 60:
	default:
		return fmt.Errorf("--fps must be one of 15, 30, or 60")
	}

	if c.Lerp <= 0 || c.Lerp > 1 {
		return fmt.Errorf("--lerp must be greater than 0 and less than or equal to 1")
	}

	if c.Width < 0 {
		return fmt.Errorf("--width must be greater than or equal to 0")
	}

	if c.BackgroundStyle == BackgroundStyleCustom && c.BackgroundRune == 0 {
		return fmt.Errorf("--bg-char is required when --bg-style=custom")
	}

	if c.SocketPath != "" && !filepath.IsAbs(c.SocketPath) {
		return fmt.Errorf("--socket-path must be an absolute path")
	}

	return nil
}
