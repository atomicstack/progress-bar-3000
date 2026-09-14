package app

import (
	"errors"
	"fmt"
	"io"
	"os/exec"
	"strings"
	"sync"
)

// completionHooks owns shell processes independently of Bubble Tea commands so
// a final input event followed immediately by EOF cannot discard the hook.
// start is called only by the model's event loop; wait is called after it stops.
type completionHooks struct {
	output  io.Writer
	pending sync.WaitGroup
	tail    <-chan struct{}
	err     error
}

func (h *completionHooks) start(command string) {
	previous := h.tail
	done := make(chan struct{})
	h.tail = done
	h.pending.Add(1)
	go func() {
		defer h.pending.Done()
		defer close(done)
		if previous != nil {
			<-previous
		}
		// security: the explicitly configured hook is shell code. Never expand
		// labels, metadata, or format tokens into it. Stdin stays disconnected.
		cmd := exec.Command("/bin/sh", "-c", command)
		cmd.Stdout = h.output
		cmd.Stderr = h.output
		if err := cmd.Run(); err != nil {
			h.err = errors.Join(h.err, fmt.Errorf("completion hook failed: %w", err))
		}
	}()
}

func (h *completionHooks) wait() error {
	h.pending.Wait()
	return h.err
}

func (m *Model) runCompletionHook() {
	if command := m.takeCompletionCommand(); command != "" {
		m.hooks.start(command)
	}
}

func (m *Model) takeCompletionCommand() string {
	total := m.state.EffectiveTotal()
	if m.completionFired || strings.TrimSpace(m.cfg.OnComplete) == "" || total <= 0 || !(m.state.Value >= float64(total)) {
		return ""
	}
	m.completionFired = true
	return m.cfg.OnComplete
}
