package input

import (
	"testing"

	"progress-bar-3000/internal/config"
)

func TestSubphaseProtocolRejectsMalformedEvents(t *testing.T) {
	for _, line := range []string{
		"@subphase", "@subphase nope", "@subphase-name",
		`{"type":"subphase"}`, `{"type":"subphase","index":1.5}`,
		`{"type":"set_subphases"}`, `{"type":"set_subphases","subphases":null}`,
		`{"type":"set_subphases","subphases":["ok",3]}`,
		`{"type":"set_subphases","subphases":[""]}`,
		`{"type":"value","value":1,"subphase":5}`,
		`{"type":"value","value":1,"subphases":["link"]}`,
		`{"type":"phase","phase":"build","subphase":"link"}`,
		`{"type":"label","label":"waiting","subphase":"link"}`,
		`{"type":"subphase","name":"link","subphase":"compile"}`,
		`{"type":"reset","phases":[{"subphases":["a"]}]}`,
		`{"type":"reset","phases":[{"name":"build","subphase":["a"]}]}`,
		`{"type":"reset","phases":[{"name":"build","subphases":[{"name":"nested"}]}]}`,
	} {
		if _, err := ParseLine(config.InputModeAuto, line); err == nil {
			t.Errorf("accepted malformed event: %s", line)
		}
	}
}
