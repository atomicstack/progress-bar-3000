package app

import (
	"bytes"
	"context"
	"errors"
	tea "github.com/charmbracelet/bubbletea"
	"testing"

	"progress-bar-3000/internal/config"
	"progress-bar-3000/internal/input"
	"progress-bar-3000/internal/progress"
)

func TestBatchAcknowledgesAppliedStateBeforeCompletionHooks(t *testing.T) {
	var output bytes.Buffer
	m := NewModel(config.Config{OnComplete: "printf done", Format: "%{value} %{phase}", FPS: 60}, progress.State{Total: 2, Phases: []string{"build", "test"}})
	m.hooks.output = &output
	done := make(chan struct{})
	ack := false
	updated, _ := m.Update(batchMsg{Events: []input.Event{{Kind: input.KindValue, Value: 2}, {Kind: input.KindPhase, PhaseName: "test"}}, Done: done, Reply: func(err error) error {
		if err != nil {
			t.Fatal(err)
		}
		if m.hooks.tail != nil {
			t.Fatal("completion hook started before acknowledgement")
		}
		ack = true
		return nil
	}})
	m = updated.(Model)
	if !ack || m.state.Value != 2 || m.state.CurrentPhase() != "test" {
		t.Fatalf("batch not acknowledged/applied: ack=%v state=%+v", ack, m.state)
	}
	select {
	case <-done:
	default:
		t.Fatal("batch completion not signalled")
	}
	if err := m.hooks.wait(); err != nil {
		t.Fatal(err)
	}
	if output.String() != "done" {
		t.Fatalf("hook output=%q", output.String())
	}
}

func TestBatchParsingIsAtomicForRendererInputMode(t *testing.T) {
	sent := false
	err := applyRequest(context.Background(), config.InputModeValue, input.Request{Lines: []string{"1", "not a value"}, Reply: func(error) error { return nil }}, func(tea.Msg) { sent = true })
	if err == nil || sent {
		t.Fatalf("malformed batch reached model: sent=%v error=%v", sent, err)
	}
}

func TestBatchPreservesResetRearmingWhenReplyFails(t *testing.T) {
	var output bytes.Buffer
	m := NewModel(config.Config{OnComplete: "printf x", FPS: 60}, progress.State{Total: 1})
	m.hooks.output = &output
	updated, _ := m.Update(batchMsg{Events: []input.Event{{Kind: input.KindValue, Value: 1}, {Kind: input.KindReset}, {Kind: input.KindValue, Value: 1}}, Done: make(chan struct{}), Reply: func(error) error { return errors.New("client disconnected") }})
	m = updated.(Model)
	if err := m.hooks.wait(); err != nil {
		t.Fatal(err)
	}
	if output.String() != "xx" {
		t.Fatalf("hook output=%q, want two completed runs", output.String())
	}
}
