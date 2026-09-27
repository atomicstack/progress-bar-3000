package app

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"strings"
	"time"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/colorprofile"
	"golang.org/x/term"

	"progress-bar-3000/internal/config"
	"progress-bar-3000/internal/input"
	"progress-bar-3000/internal/progress"
	"progress-bar-3000/internal/render"
)

var now = time.Now

func Run(cfg config.Config, in io.Reader, out, stderr io.Writer) (runErr error) {
	if !term.IsTerminal(int(os.Stdout.Fd())) {
		return fmt.Errorf("stdout must be a TTY for progress rendering")
	}

	initial, err := bootstrapState(cfg)
	if err != nil {
		return err
	}

	cfg.ColorMode = config.ColorMode(render.DetectProfile(string(cfg.ColorMode), autoProfile()))

	model := NewModel(cfg, initial)
	if stderr != nil {
		model.hooks.output = stderr
	}
	// Pin the renderer to the resolved profile. Bubble Tea downsamples every
	// frame to its own detected profile, which would override an explicit
	// --color-mode above what the environment advertises.
	program := tea.NewProgram(model, tea.WithOutput(withEraseRows(out)), tea.WithColorProfile(teaProfile(render.Profile(cfg.ColorMode))))

	source, err := newSource(cfg, in)
	if err != nil {
		return err
	}
	defer func() {
		// Close the input before waiting for hooks; no more work can be queued
		// after the program stops, and hooks must survive a producer's EOF.
		_ = source.Close()
		runErr = errors.Join(runErr, model.hooks.wait())
	}()
	// The source is listening before the start hook runs, so a hook may send
	// to the socket. Hooks are shared with the program's copy of the model.
	model.runStartHook()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	go func() {
		var err error
		if requests, ok := source.(input.RequestSource); ok {
			err = requests.RunRequests(ctx, func(request input.Request) error { return applyRequest(ctx, cfg.InputMode, request, program.Send) })
		} else {
			err = source.Run(ctx, func(line string) error {
				return applyRequest(ctx, cfg.InputMode, input.Request{Lines: []string{line}}, program.Send)
			})
		}
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
	if ok {
		finalizeRenderedBlock(out, final.View().Content, cfg.ClearOnExit)
	}
	return nil
}

// CSI control sequences. The byte 0x1b is ESC; "[" introduces a CSI.
const (
	csi          = "\x1b["
	cursorUpFmt  = csi + "%dA" // CSI n A — move cursor up n rows
	eraseLineEnd = csi + "K"   // CSI K   — erase from cursor to end of line
	eraseLine    = csi + "2K"  // CSI 2 K — erase the entire line
)

// finalizeRenderedBlock settles the bar (and any detail rows) on screen once
// the Bubble Tea program has finished. The renderer's shutdown moves the
// cursor to column 0 of the last rendered row and erases from there to the
// end of the screen, so that row is gone; the rows above are still visible.
//
// Non-clear: move the cursor up over the surviving rows, then rewrite every
// row with an erase-to-end-of-line and a newline, so the block is restored
// and the shell prompt lands on a fresh line below it.
//
// Clear: erase the current row, then step up through the surviving rows
// erasing each one, so the cursor ends on the block's top row.
//
// Rows are erased individually rather than with a single erase-to-end-of-
// screen because tmux treats CSI J issued from the home position as a full
// clear and scrolls the old rows into the pane's history; in a dedicated
// pane the block starts at the home position, so that would leave a stale
// copy of the bar in scrollback. A one-row or empty view has no rows above,
// so no cursor-up is emitted.
func finalizeRenderedBlock(out io.Writer, finalView string, clear bool) {
	rows := strings.Split(finalView, "\n")
	var b strings.Builder
	if clear {
		b.WriteString(eraseLine)
		for i := 1; i < len(rows); i++ {
			fmt.Fprintf(&b, cursorUpFmt, 1)
			b.WriteString(eraseLine)
		}
		_, _ = io.WriteString(out, b.String())
		return
	}
	if len(rows) > 1 {
		fmt.Fprintf(&b, cursorUpFmt, len(rows)-1)
	}
	for _, row := range rows {
		b.WriteString(row)
		b.WriteString(eraseLineEnd)
		b.WriteByte('\n')
	}
	_, _ = io.WriteString(out, b.String())
}

func bootstrapState(cfg config.Config) (progress.State, error) {
	state := progress.State{
		Total:        cfg.Total,
		Value:        float64(cfg.Current),
		DisplayValue: float64(cfg.Current),
		StartedAt:    now(),
	}

	if cfg.PhaseFile != "" {
		plan, err := input.LoadPhaseFile(cfg.PhaseFile)
		if err != nil {
			return progress.State{}, err
		}
		state.Apply(input.Event{Kind: input.KindReset, Phases: plan.Names, PhaseSubphases: plan.Subphases}, state.StartedAt)
		state.Value = float64(cfg.Current)
		state.DisplayValue = state.Value
		if state.Total == 0 {
			state.Total = len(plan.Names)
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
	switch colorprofile.Detect(os.Stdout, os.Environ()) {
	case colorprofile.TrueColor:
		return render.ProfileTrueColor
	case colorprofile.ANSI256:
		return render.Profile256
	case colorprofile.ANSI:
		return render.Profile16
	case colorprofile.ASCII, colorprofile.NoTTY:
		return render.ProfileNone
	default:
		return render.ProfileTrueColor
	}
}

func teaProfile(profile render.Profile) colorprofile.Profile {
	switch profile {
	case render.Profile256:
		return colorprofile.ANSI256
	case render.Profile16:
		return colorprofile.ANSI
	case render.ProfileNone:
		return colorprofile.ASCII
	default:
		return colorprofile.TrueColor
	}
}
