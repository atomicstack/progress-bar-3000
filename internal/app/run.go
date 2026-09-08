package app

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"strings"
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
	final, ok := finalModel.(Model)
	if ok && final.err != nil {
		return final.err
	}
	if ok && cfg.ClearOnExit {
		eraseRenderedBlock(out, final.View())
	}
	return nil
}

// CSI control sequences. The byte 0x1b is ESC; "[" introduces a CSI.
const (
	csi              = "\x1b["
	cursorUpFmt      = csi + "%dA" // CSI n A — move cursor up n rows
	eraseScreenBelow = csi + "J"   // CSI J  — erase from cursor to end of screen
)

// eraseRenderedBlock wipes the bar (and any detail lines) from the screen
// after the Bubble Tea program has finished. View() ends with a trailing
// newline, so the renderer's shutdown EraseEntireLine has already cleared
// the bottom empty row and the cursor is parked at column 0 of that row.
// We move up over the visible rows and erase from there to the end of the
// screen.
func eraseRenderedBlock(out io.Writer, finalView string) {
	rowsAbove := strings.Count(finalView, "\n")
	if rowsAbove <= 0 {
		return
	}
	fmt.Fprintf(out, cursorUpFmt+eraseScreenBelow, rowsAbove)
}

func bootstrapState(cfg config.Config) (progress.State, error) {
	state := progress.State{
		Total:        cfg.Total,
		Value:        float64(cfg.Current),
		DisplayValue: float64(cfg.Current),
		StartedAt:    now(),
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
	// If stdin is an interactive TTY, Bubble Tea is already reading it for
	// keystrokes (including ctrl-c). A bufio.Scanner on the same fd would
	// race with the key reader and eat the keystrokes, so skip it.
	if f, ok := stdin.(*os.File); ok && term.IsTerminal(int(f.Fd())) {
		return input.NewNullSource(), nil
	}
	return input.NewReaderSource(stdin), nil
}

func autoProfile() render.Profile {
	switch termenv.EnvColorProfile() {
	case termenv.TrueColor:
		return render.ProfileTrueColor
	case termenv.ANSI256:
		return render.Profile256
	case termenv.ANSI:
		return render.Profile16
	case termenv.Ascii:
		return render.ProfileNone
	default:
		return render.ProfileTrueColor
	}
}
