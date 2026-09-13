package input

import (
	"strings"
	"testing"

	"progress-bar-3000/internal/config"
)

func TestParseCompletionHook(t *testing.T) {
	for _, line := range []string{
		`@on-complete tmux kill-pane -t '%42'`,
		`@on-complete`,
		`{"type":"on_complete","command":"tmux kill-pane -t '%42'"}`,
		`{"type":"on_complete","command":""}`,
	} {
		t.Run(line, func(t *testing.T) {
			event, err := ParseLine(config.InputModeAuto, line)
			if err != nil {
				t.Fatalf("parse completion hook: %v", err)
			}
			if string(event.Kind) != "on_complete" {
				t.Fatalf("event kind = %q, want on_complete", event.Kind)
			}
			want := ""
			if strings.Contains(line, "tmux") {
				want = `tmux kill-pane -t '%42'`
			}
			if event.Command != want {
				t.Fatalf("hook command = %q, want %q", event.Command, want)
			}
		})
	}
}
