package cli

import (
	"bytes"
	"strings"
	"testing"

	"progress-bar-3000/internal/config"
)

func TestNewRootCommandRejectsInvalidFPS(t *testing.T) {
	var stdout bytes.Buffer
	var stderr bytes.Buffer

	cmd := NewRootCommand(func(config.Config) error { return nil })
	cmd.SetOut(&stdout)
	cmd.SetErr(&stderr)
	cmd.SetArgs([]string{"--fps", "45"})

	err := cmd.Execute()
	if err == nil {
		t.Fatal("expected an error for invalid fps")
	}

	if !strings.Contains(err.Error(), "--fps must be one of 15, 30, or 60") {
		t.Fatalf("expected fps validation error, got %v", err)
	}

	if stdout.Len() != 0 {
		t.Fatalf("expected no stdout output, got %q", stdout.String())
	}

	if stderr.Len() != 0 {
		t.Fatalf("expected no stderr output, got %q", stderr.String())
	}
}

func TestNewRootCommandRejectsInvalidStyle(t *testing.T) {
	var stdout bytes.Buffer
	var stderr bytes.Buffer

	cmd := NewRootCommand(func(config.Config) error { return nil })
	cmd.SetOut(&stdout)
	cmd.SetErr(&stderr)
	cmd.SetArgs([]string{"--style", "banana"})

	err := cmd.Execute()
	if err == nil {
		t.Fatal("expected an error for invalid style")
	}

	if !strings.Contains(err.Error(), "--style must be one of") {
		t.Fatalf("expected style validation error, got %v", err)
	}

	if stdout.Len() != 0 {
		t.Fatalf("expected no stdout output, got %q", stdout.String())
	}

	if stderr.Len() != 0 {
		t.Fatalf("expected no stderr output, got %q", stderr.String())
	}
}

func TestNewRootCommandRejectsInvalidBackgroundRune(t *testing.T) {
	var stdout bytes.Buffer
	var stderr bytes.Buffer

	cmd := NewRootCommand(func(config.Config) error { return nil })
	cmd.SetOut(&stdout)
	cmd.SetErr(&stderr)
	cmd.SetArgs([]string{"--bg-char", "ab"})

	err := cmd.Execute()
	if err == nil {
		t.Fatal("expected an error for invalid bg-char")
	}

	if !strings.Contains(err.Error(), "--bg-char must be exactly one rune") {
		t.Fatalf("expected bg-char validation error, got %v", err)
	}

	if stdout.Len() != 0 {
		t.Fatalf("expected no stdout output, got %q", stdout.String())
	}

	if stderr.Len() != 0 {
		t.Fatalf("expected no stderr output, got %q", stderr.String())
	}
}

func TestNewRootCommandRejectsRelativeSocketPath(t *testing.T) {
	var stdout bytes.Buffer
	var stderr bytes.Buffer

	cmd := NewRootCommand(func(config.Config) error { return nil })
	cmd.SetOut(&stdout)
	cmd.SetErr(&stderr)
	cmd.SetArgs([]string{"--socket-path", "tmp/progress.sock"})

	err := cmd.Execute()
	if err == nil {
		t.Fatal("expected an error for relative socket path")
	}

	if !strings.Contains(err.Error(), "--socket-path must be an absolute path") {
		t.Fatalf("expected socket path validation error, got %v", err)
	}

	if stdout.Len() != 0 {
		t.Fatalf("expected no stdout output, got %q", stdout.String())
	}

	if stderr.Len() != 0 {
		t.Fatalf("expected no stderr output, got %q", stderr.String())
	}
}

func TestNewRootCommandParsesSocketPath(t *testing.T) {
	var got config.Config
	var stdout bytes.Buffer
	var stderr bytes.Buffer

	cmd := NewRootCommand(func(cfg config.Config) error {
		got = cfg
		return nil
	})
	cmd.SetOut(&stdout)
	cmd.SetErr(&stderr)
	cmd.SetArgs([]string{"--socket-path", "/tmp/progress.sock"})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if got.SocketPath != "/tmp/progress.sock" {
		t.Fatalf("expected socket path /tmp/progress.sock, got %q", got.SocketPath)
	}

	if stdout.Len() != 0 {
		t.Fatalf("expected no stdout output, got %q", stdout.String())
	}

	if stderr.Len() != 0 {
		t.Fatalf("expected no stderr output, got %q", stderr.String())
	}
}

func TestNewRootCommandParsesRepeatedDetailFormat(t *testing.T) {
	var got config.Config
	var stdout bytes.Buffer
	var stderr bytes.Buffer

	cmd := NewRootCommand(func(cfg config.Config) error {
		got = cfg
		return nil
	})
	cmd.SetOut(&stdout)
	cmd.SetErr(&stderr)
	cmd.SetArgs([]string{
		"--detail-format", "phase: %{phase} [%{label}]",
		"--detail-format", "value: %{value}/%{total}",
	})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	want := []string{
		"phase: %{phase} [%{label}]",
		"value: %{value}/%{total}",
	}
	if len(got.DetailFormats) != len(want) {
		t.Fatalf("expected %d detail formats, got %d (%v)", len(want), len(got.DetailFormats), got.DetailFormats)
	}
	for i, w := range want {
		if got.DetailFormats[i] != w {
			t.Fatalf("detail format %d = %q, want %q", i, got.DetailFormats[i], w)
		}
	}
}

func TestNewRootCommandRejectsInvalidDetailFormat(t *testing.T) {
	var stdout bytes.Buffer
	var stderr bytes.Buffer

	cmd := NewRootCommand(func(config.Config) error { return nil })
	cmd.SetOut(&stdout)
	cmd.SetErr(&stderr)
	cmd.SetArgs([]string{"--detail-format", "phase: %"})

	err := cmd.Execute()
	if err == nil {
		t.Fatal("expected an error for unterminated template token")
	}
	if !strings.Contains(err.Error(), "--detail-format") {
		t.Fatalf("expected error to mention --detail-format, got %v", err)
	}
}

func TestNewRootCommandParsesRendererFlags(t *testing.T) {
	var got config.Config
	var stdout bytes.Buffer
	var stderr bytes.Buffer

	cmd := NewRootCommand(func(cfg config.Config) error {
		got = cfg
		return nil
	})
	cmd.SetOut(&stdout)
	cmd.SetErr(&stderr)
	cmd.SetArgs([]string{
		"--total", "12",
		"--style", "gradient-granular",
		"--bg-style", "shade-light",
		"--gradient-start", "#ffffff",
		"--gradient-end", "#0087ff",
		"--fps", "30",
		"--detail",
	})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if got.Total != 12 {
		t.Fatalf("expected total 12, got %d", got.Total)
	}

	if got.Style != config.StyleGradientGranular {
		t.Fatalf("expected style gradient-granular, got %q", got.Style)
	}

	if got.BackgroundStyle != config.BackgroundStyleShadeLight {
		t.Fatalf("expected background style shade-light, got %q", got.BackgroundStyle)
	}

	if got.GradientStart != "#ffffff" {
		t.Fatalf("expected gradient start #ffffff, got %q", got.GradientStart)
	}

	if got.GradientEnd != "#0087ff" {
		t.Fatalf("expected gradient end #0087ff, got %q", got.GradientEnd)
	}

	if got.FPS != 30 {
		t.Fatalf("expected fps 30, got %d", got.FPS)
	}

	if got.Detail != config.DetailAll {
		t.Fatalf("expected bare --detail to default to %q, got %q", config.DetailAll, got.Detail)
	}

	if stdout.Len() != 0 {
		t.Fatalf("expected no stdout output, got %q", stdout.String())
	}

	if stderr.Len() != 0 {
		t.Fatalf("expected no stderr output, got %q", stderr.String())
	}
}
