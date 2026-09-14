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
	// Animation selects a colour effect; AnimationTime is elapsed wall-clock seconds.
	Animation     string
	AnimationTime float64
	// RippleOrigin is the progress fraction at the event, not the current fill.
	RippleAge      float64
	RippleOrigin   float64
	RippleStrength float64
	// PhaseTransitionAge is measured in seconds since the phase changed.
	PhaseTransition    bool
	PhaseTransitionAge float64
	// ASCII forces every glyph to a 7-bit character: fills become '=' with
	// no partial cells, and non-ascii background glyphs become '.'. Colour
	// handling is unaffected.
	ASCII bool
}
