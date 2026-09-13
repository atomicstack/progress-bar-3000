package app

import (
	"bytes"
	"errors"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"progress-bar-3000/internal/config"
	"progress-bar-3000/internal/input"
	"progress-bar-3000/internal/progress"
)

func TestModelRunsCompletionHook(t *testing.T) {
	path := filepath.Join(t.TempDir(), "completed")
	command := "printf done > '" + strings.ReplaceAll(path, "'", "'\\''") + "'"
	m := NewModel(config.Config{OnComplete: command, FPS: 60}, progress.State{Total: 2})
	updated, _ := m.Update(eventMsg{Event: input.Event{Kind: input.KindValue, Value: 2}, Now: time.Now()})
	m = updated.(Model)
	if err := m.hooks.wait(); err != nil {
		t.Fatal(err)
	}
	if data, err := os.ReadFile(path); err != nil || string(data) != "done" {
		t.Fatalf("completion hook result = %q, error = %v", data, err)
	}
}

func TestCompletionHookLifecycle(t *testing.T) {
	for _, tc := range []struct {
		name    string
		initial progress.State
		lines   []string
		want    string
	}{
		{"partial", progress.State{Total: 4}, []string{"@tick", "@value 3.999"}, ""},
		{"once despite repeats", progress.State{Total: 2}, []string{"@value 2", "@tick", "@value 0", "@value 2"}, "x"},
		{"overshoot", progress.State{Total: 2}, []string{"@value 3"}, "x"},
		{"reset re-arms", progress.State{Total: 2}, []string{"@value 2", "@reset", "@set-total 2", "@value 2"}, "xx"},
		{"new plan re-arms", progress.State{Total: 2}, []string{"@value 2", "@set-phases build,test", "@value 2"}, "xx"},
		{"unknown total", progress.State{}, []string{"@value 10"}, ""},
		{"total update completes", progress.State{Total: 10, Value: 5}, []string{"@set-total 5"}, "x"},
		{"phase count fallback", progress.State{Phases: []string{"a", "b"}}, []string{"@value 2"}, "x"},
		{"clear", progress.State{Total: 1}, []string{"@on-complete", "@tick"}, ""},
		{"replace before completion", progress.State{Total: 1}, []string{"@on-complete printf y", "@tick"}, "y"},
		{"replace after completion", progress.State{Total: 1}, []string{"@tick", "@on-complete printf y", "@tick"}, "x"},
		{"late registration", progress.State{Total: 1}, []string{"@on-complete", "@tick", "@on-complete printf y"}, "y"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var output bytes.Buffer
			m := NewModel(config.Config{OnComplete: "printf x", FPS: 60}, tc.initial)
			m.hooks.output = &output
			for _, line := range tc.lines {
				event, err := input.ParseLine(config.InputModeAuto, line)
				if err != nil {
					t.Fatal(err)
				}
				updated, _ := m.Update(eventMsg{Event: event, Now: time.Now()})
				m = updated.(Model)
			}
			updated, _ := m.Update(doneMsg{})
			m = updated.(Model)
			if err := m.hooks.wait(); err != nil {
				t.Fatal(err)
			}
			if got := output.String(); got != tc.want {
				t.Fatalf("hook output = %q, want %q", got, tc.want)
			}
		})
	}
}

func TestInitialCompletionHookAndInterruptedInput(t *testing.T) {
	for _, tc := range []struct {
		name  string
		value float64
		msg   tea.Msg
		want  string
	}{
		{"initial complete frame", 2, frameMsg{Now: time.Now()}, "x"},
		{"initial complete eof", 2, doneMsg{}, "x"},
		{"partial eof", 1, doneMsg{}, ""},
		{"partial error", 1, errMsg{Err: io.ErrUnexpectedEOF}, ""},
		{"partial cancel", 1, tea.KeyMsg{Type: tea.KeyCtrlC}, ""},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var output bytes.Buffer
			m := NewModel(config.Config{OnComplete: "printf x", FPS: 60}, progress.State{Total: 2, Value: tc.value})
			m.hooks.output = &output
			updated, _ := m.Update(tc.msg)
			m = updated.(Model)
			if err := m.hooks.wait(); err != nil {
				t.Fatal(err)
			}
			if output.String() != tc.want {
				t.Fatalf("hook output = %q, want %q", output.String(), tc.want)
			}
		})
	}
}

func TestCompletionHooksExecuteSeriallyAndReportFailure(t *testing.T) {
	var output bytes.Buffer
	hooks := completionHooks{output: &output}
	hooks.start("sleep 0.05; printf first; printf error >&2; exit 7")
	hooks.start("printf second; if read -r line; then exit 9; fi")
	err := hooks.wait()
	var exitErr *exec.ExitError
	if !errors.As(err, &exitErr) || exitErr.ExitCode() != 7 {
		t.Fatalf("hook failure = %v, want exit 7", err)
	}
	if got := output.String(); got != "firsterrorsecond" {
		t.Fatalf("serialized hook output = %q", got)
	}
}
