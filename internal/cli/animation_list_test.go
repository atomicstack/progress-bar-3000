package cli

import (
	"progress-bar-3000/internal/config"
	"strings"
	"testing"
)

func TestRootAnimationLists(t *testing.T) {
	for _, tc := range []struct{ in, want string }{
		{"interference,edge-glow,milestone-ripple", "interference,edge-glow,milestone-ripple"},
		{" milestone-ripple, interference,edge-glow,interference ", "interference,edge-glow,milestone-ripple"},
		{"pulse", "pulse"}, {"shimmer", "shimmer"}, {"cycle", "cycle"},
		{"cycle,pulse,cycle", "pulse,cycle"}, {"milestone-ripple", "milestone-ripple"}, {"", ""},
	} {
		t.Run(tc.in, func(t *testing.T) {
			called := false
			cmd := NewRootCommand(func(c config.Config) error {
				called = true
				if string(c.TintAnimation) != tc.want {
					t.Fatalf("selection=%q want %q", c.TintAnimation, tc.want)
				}
				return nil
			})
			cmd.SetArgs([]string{"--tint-animation", tc.in})
			if err := cmd.Execute(); err != nil {
				t.Fatal(err)
			}
			if !called {
				t.Fatal("renderer not called")
			}
		})
	}
}

func TestRootRejectsInvalidAnimationList(t *testing.T) {
	for _, name := range []string{"aurora", "comet", "liquid", "embers", "phase-transition", "interference,,edge-glow", ",cycle", "pulse,", "interference,no-such-effect"} {
		t.Run(name, func(t *testing.T) {
			cmd := NewRootCommand(func(config.Config) error { t.Fatal("invalid selection reached renderer"); return nil })
			cmd.SetArgs([]string{"--tint-animation", name})
			if err := cmd.Execute(); err == nil || !strings.Contains(err.Error(), "--tint-animation") {
				t.Fatalf("expected clear validation error, got %v", err)
			}
		})
	}
}

func TestUnreleasedEventFlagsAreReplacedByList(t *testing.T) {
	for _, flag := range []string{"--milestone-ripple", "--phase-transition"} {
		cmd := NewRootCommand(func(config.Config) error { t.Fatal("removed flag reached renderer"); return nil })
		cmd.SetArgs([]string{flag})
		if err := cmd.Execute(); err == nil || !strings.Contains(err.Error(), "unknown flag") {
			t.Fatalf("removed flag accepted: %v", err)
		}
	}
}
