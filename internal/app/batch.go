package app

import (
	"context"
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"progress-bar-3000/internal/config"
	"progress-bar-3000/internal/input"
)

type batchMsg struct {
	Events []input.Event
	Reply  func(error) error
	Done   chan struct{}
	Now    time.Time
}

func applyRequest(ctx context.Context, mode config.InputMode, request input.Request, send func(tea.Msg)) error {
	events := make([]input.Event, 0, len(request.Lines))
	for _, line := range request.Lines {
		evt, err := input.ParseLine(mode, line)
		if err != nil {
			return err
		}
		if evt.Kind != "" {
			events = append(events, evt)
		}
	}
	if request.Reply == nil {
		for _, evt := range events {
			send(eventMsg{Event: evt, Now: now()})
		}
		return nil
	}
	done := make(chan struct{})
	send(batchMsg{Events: events, Reply: request.Reply, Done: done, Now: now()})
	select {
	case <-done:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

func (m *Model) applyEvent(evt input.Event, at time.Time) {
	if evt.Kind == input.KindOnComplete {
		m.cfg.OnComplete = evt.Command
		return
	}
	if evt.Kind == input.KindReset {
		m.completionFired = false
	}
	previousDisplay := m.state.DisplayValue
	m.state.Apply(evt, at)
	if affectsProgressValue(evt.Kind) && evt.Kind != input.KindReset {
		m.state.DisplayValue = previousDisplay
	}
}
