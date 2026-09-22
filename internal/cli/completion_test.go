package cli

import (
	"testing"

	"progress-bar-3000/internal/config"
)

func TestCompletionHookFlag(t *testing.T) {
	called := false
	cmd := NewRootCommand(func(cfg config.Config) error {
		if cfg.OnComplete != `tmux kill-pane -t '%42'` {
			t.Fatalf("hook command changed: %q", cfg.OnComplete)
		}
		called = true
		return nil
	})
	cmd.SetArgs([]string{"--on-complete", `tmux kill-pane -t '%42'`})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("execute with completion hook: %v", err)
	}
	if !called {
		t.Fatal("renderer was not called")
	}
}

func TestStartHookFlag(t *testing.T) {
	called := false
	cmd := NewRootCommand(func(cfg config.Config) error {
		if cfg.OnStart != `tmux set-option -p -t "$TMUX_PANE" pane-border-format ""` {
			t.Fatalf("start hook command changed: %q", cfg.OnStart)
		}
		called = true
		return nil
	})
	cmd.SetArgs([]string{"--on-start", `tmux set-option -p -t "$TMUX_PANE" pane-border-format ""`})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("execute with start hook: %v", err)
	}
	if !called {
		t.Fatal("renderer was not called")
	}
}
