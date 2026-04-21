package app

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/muesli/termenv"
	"golang.org/x/term"

	"progress-bar-3000/internal/config"
	"progress-bar-3000/internal/input"
	"progress-bar-3000/internal/progress"
	"progress-bar-3000/internal/render"
)

var now = time.Now

func Run(cfg config.Config, in io.Reader, out, _ io.Writer) error {
	if !term.IsTerminal(int(os.Stdout.Fd())) {
		return fmt.Errorf("stdout must be a TTY for progress rendering")
	}

	initial, err := bootstrapState(cfg)
	if err != nil {
		return err
	}

	cfg.ColorMode = config.ColorMode(render.DetectProfile(string(cfg.ColorMode), autoProfile()))

	program := tea.NewProgram(NewModel(cfg, initial), tea.WithOutput(out))

	source, err := newSource(cfg, in)
	if err != nil {
		return err
	}
	defer source.Close()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	go func() {
		err := source.Run(ctx, func(line string) error {
			evt, err := input.ParseLine(cfg.InputMode, line)
			if err != nil {
				return err
			}
			if evt.Kind == "" {
				return nil
			}
			program.Send(eventMsg{Event: evt, Now: now()})
			return nil
		})
		if err != nil && !errors.Is(err, context.Canceled) {
			program.Send(errMsg{Err: err})
			return
		}
		if cfg.SocketPath == "" {
			program.Send(doneMsg{})
		}
	}()

	finalModel, err := program.Run()
	cancel()
	if err != nil {
		return err
	}
	if final, ok := finalModel.(Model); ok && final.err != nil {
		return final.err
	}
	return nil
}

func bootstrapState(cfg config.Config) (progress.State, error) {
	state := progress.State{
		Total:        cfg.Total,
		Value:        float64(cfg.Current),
		DisplayValue: float64(cfg.Current),
	}

	if cfg.PhaseFile != "" {
		phases, err := input.LoadPhaseFile(cfg.PhaseFile)
		if err != nil {
			return progress.State{}, err
		}
		state.Phases = phases
		if state.Total == 0 {
			state.Total = len(phases)
		}
	}

	if cfg.Phase != "" {
		state.Apply(input.Event{Kind: input.KindPhase, PhaseName: cfg.Phase}, now())
	}

	return state, nil
}

func newSource(cfg config.Config, stdin io.Reader) (input.Source, error) {
	if cfg.SocketPath != "" {
		return input.NewUnixSocketSource(cfg.SocketPath)
	}
	return input.NewReaderSource(stdin), nil
}

func autoProfile() render.Profile {
	switch termenv.EnvColorProfile() {
	case termenv.ANSI256:
		return render.Profile256
	case termenv.Ascii:
		return render.Profile16
	default:
		return render.ProfileTrueColor
	}
}
