package render

import (
	"strings"
	"testing"

	"github.com/mattn/go-runewidth"
)

func TestSubphaseStripClippingAndHighlight(t *testing.T) {
	for _, ascii := range []bool{false, true} {
		for _, profile := range []Profile{ProfileNone, ProfileTrueColor, Profile256, Profile16} {
			for _, child := range []string{"link", "構築 e\u0301", strings.Repeat("long name ", 12)} {
				for width := 1; width <= 80; width++ {
					opts := plainPhases([]string{"fetch", "build", "test", "ship"}, 1, width)
					opts.ASCII, opts.Profile, opts.Subphase = ascii, profile, child
					_, opts.Offset = RenderPhases(opts)
					got, _ := RenderPhases(opts)
					plain := StripANSI(got)
					if gotWidth := runewidth.StringWidth(plain); gotWidth != width {
						t.Fatalf("width=%d child=%q ascii=%v profile=%s: got %d columns in %q", width, child, ascii, profile, gotWidth, plain)
					}
					if width >= 30 && child == "link" && !strings.Contains(plain, "[link]") {
						t.Fatalf("subphase clipped despite available room: %q", plain)
					}
					if width == 80 && profile == ProfileTrueColor && child == "link" && !strings.Contains(got, Bold()+Foreground(profile, testHighlight)+"build [link]") {
						t.Fatalf("child did not share parent's highlight: %q", got)
					}
				}
			}
		}
	}
}
