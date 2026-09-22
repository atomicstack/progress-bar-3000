package app

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"progress-bar-3000/internal/config"
	"progress-bar-3000/internal/progress"
)

func TestStartHookRunsOnceBeforeCompletionHook(t *testing.T) {
	path := filepath.Join(t.TempDir(), "log")
	quoted := "'" + strings.ReplaceAll(path, "'", "'\\''") + "'"
	m := NewModel(config.Config{
		OnStart:    "printf start >> " + quoted,
		OnComplete: "printf done >> " + quoted,
		FPS:        60,
	}, progress.State{Total: 1, Value: 1})
	m.runStartHook()
	m.runCompletionHook()
	if err := m.hooks.wait(); err != nil {
		t.Fatal(err)
	}
	if data, err := os.ReadFile(path); err != nil || string(data) != "startdone" {
		t.Fatalf("hook log = %q, error = %v", data, err)
	}
}

func TestBlankStartHookIsSkipped(t *testing.T) {
	m := NewModel(config.Config{OnStart: "  ", FPS: 60}, progress.State{})
	m.runStartHook()
	if m.hooks.tail != nil {
		t.Fatal("blank start hook was queued")
	}
}

func TestStartHookFailureIsReported(t *testing.T) {
	m := NewModel(config.Config{OnStart: "exit 3", FPS: 60}, progress.State{})
	m.runStartHook()
	if err := m.hooks.wait(); err == nil || !strings.Contains(err.Error(), "start hook failed") {
		t.Fatalf("start hook error = %v", err)
	}
}
