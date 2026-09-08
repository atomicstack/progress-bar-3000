package input

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"progress-bar-3000/internal/config"
)

func TestParseLineHybridProtocol(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		mode config.InputMode
		line string
		want Event
	}{
		{
			name: "plain line in auto mode",
			mode: config.InputModeAuto,
			line: "  build  ",
			want: Event{Kind: KindTick, Amount: 1, Label: "build"},
		},
		{
			name: "numeric line in value mode",
			mode: config.InputModeValue,
			line: " 7 ",
			want: Event{Kind: KindValue, Value: 7},
		},
		{
			name: "control line set total",
			mode: config.InputModeLines,
			line: "@set-total 12",
			want: Event{Kind: KindSetTotal, Total: 12},
		},
		{
			name: "json reset line",
			mode: config.InputModeLines,
			line: `{"type":"reset","total":3}`,
			want: Event{Kind: KindReset, Total: 3},
		},
	}

	for _, tc := range tests {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			got, err := ParseLine(tc.mode, tc.line)
			if err != nil {
				t.Fatalf("ParseLine() error = %v", err)
			}
			if got.Kind != tc.want.Kind || got.Amount != tc.want.Amount || got.Value != tc.want.Value || got.Total != tc.want.Total || got.PhaseIndex != tc.want.PhaseIndex || got.PhaseName != tc.want.PhaseName || got.Label != tc.want.Label {
				t.Fatalf("ParseLine() = %#v, want %#v", got, tc.want)
			}
			if len(got.Meta) != len(tc.want.Meta) {
				t.Fatalf("ParseLine() meta = %#v, want %#v", got.Meta, tc.want.Meta)
			}
			for k, v := range tc.want.Meta {
				if got.Meta[k] != v {
					t.Fatalf("ParseLine() meta[%q] = %q, want %q", k, got.Meta[k], v)
				}
			}
			if len(got.Phases) != len(tc.want.Phases) {
				t.Fatalf("ParseLine() phases = %#v, want %#v", got.Phases, tc.want.Phases)
			}
			for i := range tc.want.Phases {
				if got.Phases[i] != tc.want.Phases[i] {
					t.Fatalf("ParseLine()[%d] = %q, want %q", i, got.Phases[i], tc.want.Phases[i])
				}
			}
		})
	}
}

func TestLoadPhaseFileSupportsTextAndJSON(t *testing.T) {
	t.Parallel()

	wd, err := os.Getwd()
	if err != nil {
		t.Fatalf("Getwd() error = %v", err)
	}

	tests := []struct {
		name string
		path string
		want []string
	}{
		{
			name: "text",
			path: filepath.Join(wd, "..", "..", "testdata", "phase-files", "phases.txt"),
			want: []string{"build", "test", "package"},
		},
		{
			name: "json",
			path: filepath.Join(wd, "..", "..", "testdata", "phase-files", "phases.json"),
			want: []string{"build", "test", "package"},
		},
	}

	for _, tc := range tests {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			got, err := LoadPhaseFile(tc.path)
			if err != nil {
				t.Fatalf("LoadPhaseFile() error = %v", err)
			}
			if len(got) != len(tc.want) {
				t.Fatalf("LoadPhaseFile() len = %d, want %d: %#v", len(got), len(tc.want), got)
			}
			for i := range got {
				if got[i] != tc.want[i] {
					t.Fatalf("LoadPhaseFile()[%d] = %q, want %q", i, got[i], tc.want[i])
				}
			}
		})
	}
}

func TestParseLineFailures(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		mode config.InputMode
		line string
		want string
	}{
		{
			name: "malformed json",
			mode: config.InputModeLines,
			line: `{"type":"tick",`,
			want: "parse json event",
		},
		{
			name: "unknown json field",
			mode: config.InputModeLines,
			line: `{"type":"tick","bogus":1}`,
			want: "unknown field",
		},
		{
			name: "invalid numeric line in value mode",
			mode: config.InputModeValue,
			line: "not-a-number",
			want: "parse value line",
		},
		{
			name: "unknown control command",
			mode: config.InputModeLines,
			line: "@nope 1",
			want: "unknown control command",
		},
		{
			name: "json phase event with phases array",
			mode: config.InputModeLines,
			line: `{"type":"phase","name":"test","phases":["build","test"]}`,
			want: `json phase event does not accept "phases"; use a reset event`,
		},
	}

	for _, tc := range tests {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			_, err := ParseLine(tc.mode, tc.line)
			if err == nil {
				t.Fatal("ParseLine() error = nil, want error")
			}
			if !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("ParseLine() error = %v, want substring %q", err, tc.want)
			}
		})
	}
}
