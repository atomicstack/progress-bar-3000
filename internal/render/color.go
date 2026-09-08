package render

import (
	"fmt"
	"math"
	"strconv"
	"strings"
)

type RGB struct {
	R uint8
	G uint8
	B uint8
}

func ParseHexColor(in string) (RGB, error) {
	trimmed := strings.TrimPrefix(strings.TrimSpace(in), "#")
	if len(trimmed) != 6 {
		return RGB{}, fmt.Errorf("hex color must be 6 digits")
	}

	value, err := strconv.ParseUint(trimmed, 16, 32)
	if err != nil {
		return RGB{}, err
	}

	return RGB{
		R: uint8(value >> 16),
		G: uint8((value >> 8) & 0xff),
		B: uint8(value & 0xff),
	}, nil
}

func DetectProfile(mode string, auto Profile) Profile {
	switch mode {
	case "truecolor":
		return ProfileTrueColor
	case "256":
		return Profile256
	case "16":
		return Profile16
	case "none":
		return ProfileNone
	default:
		return auto
	}
}

func Foreground(profile Profile, color RGB) string {
	switch profile {
	case ProfileTrueColor:
		return fmt.Sprintf("\x1b[38;2;%d;%d;%dm", color.R, color.G, color.B)
	case Profile256:
		return fmt.Sprintf("\x1b[38;5;%dm", rgbTo256(color))
	case Profile16:
		return fmt.Sprintf("\x1b[%dm", rgbTo16(color))
	default:
		return ""
	}
}

func Background(profile Profile, color RGB) string {
	switch profile {
	case ProfileTrueColor:
		return fmt.Sprintf("\x1b[48;2;%d;%d;%dm", color.R, color.G, color.B)
	case Profile256:
		return fmt.Sprintf("\x1b[48;5;%dm", rgbTo256(color))
	case Profile16:
		return fmt.Sprintf("\x1b[%dm", rgbTo16(color)+10)
	default:
		return ""
	}
}

func Reset() string {
	return "\x1b[0m"
}

// Dim is the SGR "faint" attribute; Bold is SGR "bold". Both are attribute
// escapes rather than colours, so they are independent of the profile
// except under ProfileNone, where the caller is expected to emit nothing.
func Dim() string {
	return "\x1b[2m"
}

func Bold() string {
	return "\x1b[1m"
}

// RGBToHSL converts a colour to hue (degrees, 0-360), saturation and
// lightness (both 0-1).
func RGBToHSL(c RGB) (h, s, l float64) {
	r := float64(c.R) / 255
	g := float64(c.G) / 255
	b := float64(c.B) / 255

	max := math.Max(r, math.Max(g, b))
	min := math.Min(r, math.Min(g, b))
	l = (max + min) / 2

	delta := max - min
	if delta == 0 {
		return 0, 0, l
	}

	if l > 0.5 {
		s = delta / (2 - max - min)
	} else {
		s = delta / (max + min)
	}

	switch max {
	case r:
		h = (g - b) / delta
		if g < b {
			h += 6
		}
	case g:
		h = (b-r)/delta + 2
	default:
		h = (r-g)/delta + 4
	}
	return h * 60, s, l
}

// HSLToRGB is the inverse of RGBToHSL. Saturation and lightness are clamped
// to 0-1 and hue wraps modulo 360.
func HSLToRGB(h, s, l float64) RGB {
	s = clampUnit(s)
	l = clampUnit(l)
	h = math.Mod(h, 360)
	if h < 0 {
		h += 360
	}

	if s == 0 {
		v := uint8(math.Round(l * 255))
		return RGB{R: v, G: v, B: v}
	}

	var q float64
	if l < 0.5 {
		q = l * (1 + s)
	} else {
		q = l + s - l*s
	}
	p := 2*l - q
	hk := h / 360

	return RGB{
		R: uint8(math.Round(hueToChannel(p, q, hk+1.0/3) * 255)),
		G: uint8(math.Round(hueToChannel(p, q, hk) * 255)),
		B: uint8(math.Round(hueToChannel(p, q, hk-1.0/3) * 255)),
	}
}

func hueToChannel(p, q, t float64) float64 {
	if t < 0 {
		t++
	}
	if t > 1 {
		t--
	}
	switch {
	case t < 1.0/6:
		return p + (q-p)*6*t
	case t < 0.5:
		return q
	case t < 2.0/3:
		return p + (q-p)*(2.0/3-t)*6
	default:
		return p
	}
}

func clampUnit(v float64) float64 {
	if v < 0 {
		return 0
	}
	if v > 1 {
		return 1
	}
	return v
}

func BlendTowardWhite(c RGB, factor float64) RGB {
	if factor < 0 {
		factor = 0
	}
	if factor > 1 {
		factor = 1
	}

	return RGB{
		R: uint8(float64(c.R) + (255-float64(c.R))*factor),
		G: uint8(float64(c.G) + (255-float64(c.G))*factor),
		B: uint8(float64(c.B) + (255-float64(c.B))*factor),
	}
}

func Scale(c RGB, factor float64) RGB {
	if factor < 0 {
		factor = 0
	}
	if factor > 1 {
		factor = 1
	}
	return RGB{
		R: uint8(float64(c.R) * factor),
		G: uint8(float64(c.G) * factor),
		B: uint8(float64(c.B) * factor),
	}
}

func rgbTo256(c RGB) int {
	r := int(c.R) * 5 / 255
	g := int(c.G) * 5 / 255
	b := int(c.B) * 5 / 255
	return 16 + 36*r + 6*g + b
}

func rgbTo16(c RGB) int {
	if c.R >= 200 && c.G >= 200 && c.B >= 200 {
		return 97
	}
	if c.B >= c.R && c.B >= c.G {
		return 94
	}
	if c.G >= c.R && c.G >= c.B {
		return 92
	}
	if c.R >= c.G && c.R >= c.B {
		return 91
	}
	return 37
}
