package render

type Style string

type BackgroundStyle string

type Profile string

const (
	StylePlain            Style = "plain"
	StyleBlock            Style = "block"
	StyleGranular         Style = "granular"
	StyleShaded           Style = "shaded"
	StyleGradientBlock    Style = "gradient-block"
	StyleGradientGranular Style = "gradient-granular"
	StyleGradientShaded   Style = "gradient-shaded"
)

const (
	BackgroundNone        BackgroundStyle = "none"
	BackgroundSpace       BackgroundStyle = "space"
	BackgroundASCII       BackgroundStyle = "ascii"
	BackgroundShadeLight  BackgroundStyle = "shade-light"
	BackgroundShadeMedium BackgroundStyle = "shade-medium"
	BackgroundShadeDark   BackgroundStyle = "shade-dark"
	BackgroundCustom      BackgroundStyle = "custom"
)

const (
	ProfileTrueColor Profile = "truecolor"
	Profile256       Profile = "256"
	Profile16        Profile = "16"
	ProfileNone      Profile = "none"
)

type Options struct {
	Width           int
	Percent         float64
	Style           Style
	BackgroundStyle BackgroundStyle
	BackgroundRune  rune
	Profile         Profile
	GradientStart   RGB
	GradientEnd     RGB
	Pulse           float64
	ShimmerPhase    float64
	GradientShift   float64
}
