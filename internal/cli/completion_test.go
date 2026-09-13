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
