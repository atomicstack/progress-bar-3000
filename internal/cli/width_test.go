package cli

import (
	"testing"

	"progress-bar-3000/internal/config"
)

func TestRootAcceptsFullWidth(t *testing.T) {
	called := false
	cmd := NewRootCommand(func(cfg config.Config) error {
		called = true
		if !cfg.WidthFull || cfg.Width != 0 {
			t.Fatalf("full width config = %d, %v", cfg.Width, cfg.WidthFull)
		}
		return nil
	})
	cmd.SetArgs([]string{"--width", "full"})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("full width rejected: %v", err)
	}
	if !called {
		t.Fatal("renderer not called")
	}
}

func TestRootWidthValues(t *testing.T) {
	for _, tc := range []struct {
		value   string
		want    int
		invalid bool
	}{
		{"0", 0, false},
		{"42", 42, false},
		{"-1", 0, true},
		{"wide", 0, true},
		{"1.5", 0, true},
		{"", 0, true},
		{"9999999999999999999999999999999", 0, true},
	} {
		t.Run(tc.value, func(t *testing.T) {
			called := false
			cmd := NewRootCommand(func(cfg config.Config) error {
				called = true
				if cfg.Width != tc.want || cfg.WidthFull {
					t.Fatalf("width config = %d, %v", cfg.Width, cfg.WidthFull)
				}
				return nil
			})
			cmd.SetArgs([]string{"--width", tc.value})
			err := cmd.Execute()
			if (err != nil) != tc.invalid {
				t.Fatalf("width error = %v, want invalid = %v", err, tc.invalid)
			}
			if called == tc.invalid {
				t.Fatalf("renderer called = %v, want %v", called, !tc.invalid)
			}
		})
	}
}
