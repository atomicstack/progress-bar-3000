package app

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"progress-bar-3000/internal/config"
	"progress-bar-3000/internal/input"
)

func TestBootstrapStateUsesPhaseFileAndCurrentValue(t *testing.T) {
	cfg := config.Config{
		Current:   2,
		Phase:     "test",
		PhaseFile: filepath.Join("..", "..", "testdata", "phase-files", "phases.txt"),
	}

	state, err := bootstrapState(cfg)
	if err != nil {
		t.Fatalf("bootstrapState() error = %v", err)
	}
	if state.Total != 3 {
		t.Fatalf("Total = %d, want 3", state.Total)
	}
	if state.Value != 2 {
		t.Fatalf("Value = %v, want 2", state.Value)
	}
	if state.CurrentPhase() != "test" {
		t.Fatalf("CurrentPhase = %q, want test", state.CurrentPhase())
	}
}

func TestNewSourceReturnsReaderSourceByDefault(t *testing.T) {
	source, err := newSource(config.Config{}, strings.NewReader("@tick\n"))
	if err != nil {
		t.Fatalf("newSource() error = %v", err)
	}
	defer source.Close()

	if err := source.Run(context.Background(), func(line string) error {
		if line != "@tick" {
			t.Fatalf("line = %q, want %q", line, "@tick")
		}
		return nil
	}); err != nil {
		t.Fatalf("Run() error = %v", err)
	}
}

func TestNewSourceReturnsUnixSocketSource(t *testing.T) {
	path := filepath.Join(os.TempDir(), fmt.Sprintf("pb3-%d.sock", time.Now().UnixNano()))
	_ = os.Remove(path)
	t.Cleanup(func() { _ = os.Remove(path) })
	source, err := newSource(config.Config{SocketPath: path}, strings.NewReader(""))
	if err != nil {
		t.Fatalf("newSource() error = %v", err)
	}
	if err := source.Close(); err != nil {
		t.Fatalf("Close() error = %v", err)
	}
	if _, err := os.Stat(path); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("socket file should be removed, stat err = %v", err)
	}
}

var _ input.Source = (*stubSource)(nil)

type stubSource struct{}

func (stubSource) Run(context.Context, func(string) error) error { return nil }
func (stubSource) Close() error                                  { return nil }
