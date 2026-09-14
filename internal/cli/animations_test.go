package cli

import (
	"testing"

	"progress-bar-3000/internal/config"
)

func TestRootAcceptsAmbientAnimations(t *testing.T) {
	for _, name := range []string{"interference", "edge-glow"} {
		t.Run(name, func(t *testing.T) {
			called := false
			cmd := NewRootCommand(func(cfg config.Config) error {
				called = true
				if string(cfg.TintAnimation) != name {
					t.Fatalf("animation=%q", cfg.TintAnimation)
				}
				return nil
			})
			cmd.SetArgs([]string{"--tint-animation", name, "--style", "granular"})
			if err := cmd.Execute(); err != nil {
				t.Fatal(err)
			}
			if !called {
				t.Fatal("renderer not called")
			}
		})
	}
}
