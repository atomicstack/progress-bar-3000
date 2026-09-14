package cli

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"time"

	"progress-bar-3000/internal/config"
	"progress-bar-3000/internal/input"

	"github.com/spf13/cobra"
)

type tmuxHandles struct {
	Pane   string `json:"pane"`
	Socket string `json:"socket"`
	PID    int    `json:"pid"`
	Window string `json:"window"`
}

type tmuxDependencies struct {
	getenv     func(string) string
	executable func() (string, error)
	run        func(context.Context, ...string) (string, error)
	send       func(context.Context, string, []string) error
}

type tmuxStartOptions struct {
	phases    string
	total     int
	width     string
	autoClose bool
	output    string
	timeout   time.Duration
}

func newTmuxStartCommand() *cobra.Command {
	return newTmuxStartCommandWith(tmuxDependencies{
		getenv:     os.Getenv,
		executable: os.Executable,
		run:        runTmux,
		send:       input.Send,
	})
}

func newTmuxStartCommandWith(deps tmuxDependencies) *cobra.Command {
	options := tmuxStartOptions{}
	cmd := &cobra.Command{
		Use: "tmux-start", Short: "start an initialized two-row tmux progress pane",
		Args: cobra.NoArgs, SilenceUsage: true, SilenceErrors: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			if deps.getenv("TMUX") == "" || deps.getenv("TMUX_PANE") == "" {
				return fmt.Errorf("tmux-start requires TMUX and TMUX_PANE; run inside tmux")
			}
			phases, err := options.validate(cmd.Flags().Changed("total"))
			if err != nil {
				return err
			}
			ctx, stop := signal.NotifyContext(cmd.Context(), os.Interrupt, syscall.SIGTERM)
			defer stop()
			ctx, cancel := context.WithTimeout(ctx, options.timeout)
			defer cancel()
			return startTmux(ctx, deps, options, phases, cmd.OutOrStdout())
		},
	}
	cmd.Flags().StringVar(&options.phases, "phases", "", "required comma-separated phase names")
	cmd.Flags().IntVar(&options.total, "total", 0, "total steps (defaults to the number of phases)")
	cmd.Flags().StringVar(&options.width, "width", "full", "bar width: full, 0 (90% automatic), or positive columns")
	cmd.Flags().BoolVar(&options.autoClose, "auto-close", true, "remove the socket directory and close the owned pane on completion")
	cmd.Flags().StringVar(&options.output, "output", "json", "handle output: json or shell")
	cmd.Flags().DurationVar(&options.timeout, "timeout", 10*time.Second, "maximum startup duration")
	return cmd
}

func (options *tmuxStartOptions) validate(totalSet bool) ([]string, error) {
	if strings.TrimSpace(options.phases) == "" {
		return nil, fmt.Errorf("--phases is required")
	}
	if strings.ContainsAny(options.phases, "\r\n\x00") {
		return nil, fmt.Errorf("--phases must contain single-line names")
	}
	phases := strings.Split(options.phases, ",")
	for i := range phases {
		phases[i] = strings.TrimSpace(phases[i])
		if phases[i] == "" {
			return nil, fmt.Errorf("--phases must not contain empty names")
		}
	}
	if !totalSet {
		options.total = len(phases)
	}
	if options.total <= 0 {
		return nil, fmt.Errorf("--total must be positive")
	}
	if _, _, err := config.ParseWidth(options.width); err != nil {
		return nil, err
	}
	if options.output != "json" && options.output != "shell" {
		return nil, fmt.Errorf("--output must be json or shell")
	}
	if options.timeout <= 0 {
		return nil, fmt.Errorf("--timeout must be positive")
	}
	return phases, nil
}

func startTmux(
	ctx context.Context,
	deps tmuxDependencies,
	options tmuxStartOptions,
	phases []string,
	output io.Writer,
) (err error) {
	ownPane := deps.getenv("TMUX_PANE")
	if !validTmuxID(ownPane, '%') {
		return fmt.Errorf("TMUX_PANE must be an exact pane id such as %%42")
	}
	window, err := deps.run(ctx, "display-message", "-p", "-t", ownPane, "#{window_id}")
	if err != nil {
		return fmt.Errorf("resolve tmux parent window: %w", err)
	}
	window = strings.TrimSpace(window)
	if !validTmuxID(window, '@') {
		return fmt.Errorf("tmux returned an invalid parent window id %q", window)
	}
	executable, err := deps.executable()
	if err != nil {
		return fmt.Errorf("resolve renderer executable: %w", err)
	}
	executable, err = filepath.Abs(executable)
	if err != nil {
		return fmt.Errorf("resolve absolute renderer executable: %w", err)
	}
	directory, err := os.MkdirTemp("/tmp", "pb-")
	if err != nil {
		return fmt.Errorf("create private socket directory: %w", err)
	}
	handles := tmuxHandles{Socket: filepath.Join(directory, "p.sock")}
	renderer := "exec " + shellCommand(append([]string{executable}, []string{
		"--socket-path", handles.Socket,
		"--total", strconv.Itoa(options.total),
		"--width", options.width,
		"--style", "gradient-granular",
		"--tint-animation=cycle",
		"--format", "%p %{percent}",
		"--detail-format", "%{phases}",
	}...))
	defer func() {
		if err == nil {
			return
		}
		// Rollback must still run after a startup timeout or interrupt.
		cleanupCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), 2*time.Second)
		defer cancel()
		if handles.Pane == "" {
			recovered, recoveryErr := recoverTmuxPane(cleanupCtx, deps, window, ownPane, renderer)
			if recoveryErr != nil {
				err = errors.Join(err, fmt.Errorf("recover owned tmux pane: %w", recoveryErr))
			}
			handles.Pane = recovered.Pane
			handles.PID = recovered.PID
		}
		if handles.Pane != "" {
			if _, cleanupErr := deps.run(cleanupCtx, "kill-pane", "-t", handles.Pane); cleanupErr != nil {
				err = errors.Join(err, fmt.Errorf("clean up owned tmux pane %s: %w", handles.Pane, cleanupErr))
			}
		}
		if cleanupErr := os.RemoveAll(directory); cleanupErr != nil {
			err = errors.Join(err, fmt.Errorf("clean up private socket directory: %w", cleanupErr))
		}
	}()
	// Keep the pane target even when the user changes the active window meanwhile.
	split, splitErr := deps.run(ctx,
		"split-window", "-d", "-v", "-l", "2", "-t", ownPane,
		"-P", "-F", "#{pane_id}\t#{pane_pid}\t#{window_id}", renderer,
	)
	if splitErr != nil {
		return fmt.Errorf("create tmux progress pane: %w", splitErr)
	}
	fields := strings.Fields(split)
	if len(fields) != 3 || !validTmuxID(fields[0], '%') || fields[0] == ownPane {
		return fmt.Errorf("tmux returned invalid progress pane handles %q", split)
	}
	handles.PID, err = strconv.Atoi(fields[1])
	if err != nil || handles.PID <= 0 {
		return fmt.Errorf("tmux returned invalid pane pid %q", fields[1])
	}
	handles.Pane = fields[0]
	handles.Window = fields[2]
	if handles.Window != window {
		return fmt.Errorf("progress pane window %q differs from parent window %q", handles.Window, window)
	}
	if _, err := deps.run(ctx, "set-option", "-p", "-t", handles.Pane, "pane-border-format", ""); err != nil {
		return fmt.Errorf("set progress pane border: %w", err)
	}
	if err := waitForSocket(ctx, handles.Socket); err != nil {
		return err
	}
	events := []string{
		"@set-phases " + strings.Join(phases, ","),
		"@set-total " + strconv.Itoa(options.total),
		"@value 0",
		"@phase-name " + phases[0],
	}
	if options.autoClose {
		hook := "rm -f -- " + shellQuote(handles.Socket) + "; rmdir -- " + shellQuote(directory) +
			"; tmux kill-pane -t " + shellQuote(handles.Pane)
		events = append(events, "@on-complete "+hook)
	}
	if err := deps.send(ctx, handles.Socket, events); err != nil {
		return fmt.Errorf("initialize progress pane: %w", err)
	}
	if err := waitForTmuxFrame(ctx, deps, handles.Pane); err != nil {
		return err
	}
	if options.output == "shell" {
		_, err = fmt.Fprintf(output, "PB_PANE=%s\nPB_SOCK=%s\nPB_PID=%s\nPB_WINDOW=%s\n",
			shellQuote(handles.Pane), shellQuote(handles.Socket), shellQuote(strconv.Itoa(handles.PID)), shellQuote(handles.Window),
		)
		return err
	}
	return json.NewEncoder(output).Encode(handles)
}

// A split may reach the server even if cancellation discards its reply. The exact
// command embeds a unique private socket path, so it identifies our pane without
// trusting a truncated pane id or inspecting unrelated process names.
func recoverTmuxPane(
	ctx context.Context,
	deps tmuxDependencies,
	window, parent, renderer string,
) (tmuxHandles, error) {
	panes, err := deps.run(ctx, "list-panes", "-t", window, "-F",
		"#{pane_id}\t#{pane_pid}\t#{pane_start_command}",
	)
	if err != nil {
		return tmuxHandles{}, err
	}
	for line := range strings.SplitSeq(panes, "\n") {
		fields := strings.SplitN(line, "\t", 3)
		if len(fields) != 3 || fields[2] != renderer {
			continue
		}
		if !validTmuxID(fields[0], '%') || fields[0] == parent {
			continue
		}
		pid, err := strconv.Atoi(fields[1])
		if err != nil || pid <= 0 {
			continue
		}
		return tmuxHandles{Pane: fields[0], PID: pid, Window: window}, nil
	}
	return tmuxHandles{}, nil
}

func waitForSocket(ctx context.Context, path string) error {
	for {
		info, err := os.Stat(path)
		if err == nil && info.Mode()&os.ModeSocket != 0 {
			return nil
		}
		if err != nil && !os.IsNotExist(err) {
			return fmt.Errorf("inspect renderer socket: %w", err)
		}
		if err := waitTmuxPoll(ctx); err != nil {
			return fmt.Errorf("wait for renderer socket: %w", err)
		}
	}
}

func waitForTmuxFrame(ctx context.Context, deps tmuxDependencies, pane string) error {
	height, err := deps.run(ctx, "display-message", "-p", "-t", pane, "#{pane_height}")
	if err != nil {
		return fmt.Errorf("inspect progress pane height: %w", err)
	}
	if strings.TrimSpace(height) != "2" {
		return fmt.Errorf("progress pane must be exactly 2 rows; got %q", strings.TrimSpace(height))
	}
	for {
		frame, err := deps.run(ctx, "capture-pane", "-p", "-t", pane)
		if err != nil {
			return fmt.Errorf("capture initial progress frame: %w", err)
		}
		rows := strings.Split(strings.TrimSuffix(frame, "\n"), "\n")
		if len(rows) == 2 && strings.Contains(rows[0], "%") && strings.TrimSpace(rows[1]) != "" {
			return nil
		}
		if err := waitTmuxPoll(ctx); err != nil {
			return fmt.Errorf("wait for initial bar and phase detail: %w", err)
		}
	}
}

func waitTmuxPoll(ctx context.Context) error {
	timer := time.NewTimer(20 * time.Millisecond)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-timer.C:
		return nil
	}
}

func runTmux(ctx context.Context, args ...string) (string, error) {
	cmd := exec.CommandContext(ctx, "tmux", args...)
	var stderr strings.Builder
	cmd.Stderr = &stderr
	output, err := cmd.Output()
	if err != nil {
		if detail := strings.TrimSpace(stderr.String()); detail != "" {
			err = fmt.Errorf("%w: %s", err, detail)
		}
	}
	return string(output), err
}

func validTmuxID(value string, prefix byte) bool {
	if len(value) < 2 || value[0] != prefix {
		return false
	}
	for _, char := range value[1:] {
		if char < '0' || char > '9' {
			return false
		}
	}
	return true
}

func shellQuote(value string) string {
	return "'" + strings.ReplaceAll(value, "'", "'\"'\"'") + "'"
}

func shellCommand(args []string) string {
	quoted := make([]string, len(args))
	for i, arg := range args {
		quoted[i] = shellQuote(arg)
	}
	return strings.Join(quoted, " ")
}
