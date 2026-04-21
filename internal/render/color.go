package render

import (
	"fmt"
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
