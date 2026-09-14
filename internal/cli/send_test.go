package cli

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"progress-bar-3000/internal/config"
	"progress-bar-3000/internal/input"
)

func TestSendMissingSocketDoesNotStartRenderer(t *testing.T) {
	called := false
	cmd := NewRootCommand(func(_ config.Config) error { called = true; return nil })
	cmd.SetArgs([]string{"send", "--socket-path", "/tmp/pb3-no-such-renderer.sock", "@value 1"})
	err := cmd.Execute()
	if err == nil || !strings.Contains(err.Error(), "connect to renderer") || called {
		t.Fatalf("send error=%v, renderer invoked=%v", err, called)
	}
}

func TestSendRejectsNonJSONBeforeConnecting(t *testing.T) {
	cmd := NewRootCommand(func(config.Config) error { return nil })
	cmd.SetArgs([]string{"send", "--socket-path", "/tmp/pb3-missing.sock", "--json", "@tick"})
	if err := cmd.Execute(); err == nil || !strings.Contains(err.Error(), "json") {
		t.Fatalf("expected json validation: %v", err)
	}
}

func TestSendReadsStdinAndAcknowledges(t *testing.T) {
	dir, err := os.MkdirTemp("/tmp", "pb3-cli-")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(dir)
	path := filepath.Join(dir, "s.sock")
	source, err := input.NewUnixSocketSource(path)
	if err != nil {
		t.Fatal(err)
	}
	defer source.Close()
	go source.Run(t.Context(), func(string) error { return nil })
	cmd := NewRootCommand(func(config.Config) error { return nil })
	cmd.SetIn(strings.NewReader("@value 1\n@phase-name test\n"))
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetArgs([]string{"send", "--socket-path", path})
	if err := cmd.Execute(); err != nil {
		t.Fatal(err)
	}
	if out.Len() != 0 {
		t.Fatalf("unexpected sender output: %q", out.String())
	}
}
