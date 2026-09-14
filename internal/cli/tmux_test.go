package cli

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"regexp"
	"slices"
	"strings"
	"testing"

	"progress-bar-3000/internal/config"
)

func TestTmuxStartMissingEnvironment(t *testing.T) {
	t.Setenv("TMUX", "")
	t.Setenv("TMUX_PANE", "")
	cmd := NewRootCommand(func(config.Config) error { t.Fatal("renderer invoked"); return nil })
	cmd.SetArgs([]string{"tmux-start", "--phases", "build,verify"})
	err := cmd.Execute()
	if err == nil || !strings.Contains(err.Error(), "tmux-start requires TMUX and TMUX_PANE") {
		t.Fatalf("got %v, want missing tmux environment error", err)
	}
}

func TestTmuxStartBootstrap(t *testing.T) {
	f := newFakeTmux(t)
	var output bytes.Buffer
	cmd := newTmuxStartCommandWith(f.dependencies())
	cmd.SetOut(&output)
	cmd.SetArgs([]string{"--phases", "build,verify", "--total", "9"})
	if err := cmd.Execute(); err != nil {
		t.Fatal(err)
	}
	var got tmuxHandles
	if err := json.Unmarshal(output.Bytes(), &got); err != nil {
		t.Fatal(err)
	}
	if got.Pane != "%42" || got.Window != "@7" || got.PID != 1234 {
		t.Fatalf("unexpected handles: %+v", got)
	}
	if filepath.Dir(filepath.Dir(got.Socket)) != "/tmp" {
		t.Fatalf("socket not in short private /tmp dir: %s", got.Socket)
	}
	info, err := os.Stat(filepath.Dir(got.Socket))
	if err != nil || info.Mode().Perm() != 0700 {
		t.Fatalf("private directory: %v %v", info, err)
	}
	want := []string{"@set-phases build,verify", "@set-total 9", "@value 0", "@phase-name build"}
	if len(f.events) != 5 || !reflect.DeepEqual(f.events[:4], want) {
		t.Fatalf("events = %#v", f.events)
	}
	for _, token := range []string{"rm -f", "rmdir", "kill-pane", "'%42'", shellQuote(got.Socket)} {
		if !strings.Contains(f.events[4], token) {
			t.Errorf("hook missing %q: %s", token, f.events[4])
		}
	}
	split := f.command("split-window")
	if !reflect.DeepEqual(split[:9], []string{"split-window", "-d", "-v", "-l", "2", "-t", "%11", "-P", "-F"}) {
		t.Fatalf("split args = %#v", split)
	}
	renderer := split[len(split)-1]
	for _, token := range []string{"exec '/opt/test bin/pb'", "'--width' 'full'", "'--detail-format' '%{phases}'", "'--format' '%p %{percent}'"} {
		if !strings.Contains(renderer, token) {
			t.Errorf("renderer missing %q: %s", token, renderer)
		}
	}
	if !reflect.DeepEqual(f.command("set-option"), []string{"set-option", "-p", "-t", "%42", "pane-border-format", ""}) {
		t.Fatalf("wrong border args: %#v", f.command("set-option"))
	}
	if f.command("kill-pane") != nil {
		t.Fatal("successful start killed pane")
	}
}

func TestTmuxStartFailureRollsBackOwnedResources(t *testing.T) {
	for _, stage := range []string{"wrong window", "missing socket", "send", "wrong height", "missing detail", "output"} {
		t.Run(stage, func(t *testing.T) {
			f := newFakeTmux(t)
			f.failure = stage
			cmd := newTmuxStartCommandWith(f.dependencies())
			cmd.SetOut(io.Discard)
			if stage == "output" {
				cmd.SetOut(failingWriter{})
			}
			cmd.SetArgs([]string{"--phases", "build,verify", "--timeout", "100ms"})
			if err := cmd.Execute(); err == nil {
				t.Fatal("expected startup failure")
			}
			if !reflect.DeepEqual(f.command("kill-pane"), []string{"kill-pane", "-t", "%42"}) {
				t.Fatalf("rollback did not target owned pane: %#v", f.commands)
			}
			if _, err := os.Stat(filepath.Dir(f.socket)); !os.IsNotExist(err) {
				t.Fatalf("private directory leaked: %v", err)
			}
		})
	}
}

func TestTmuxStartValidationBeforeSplitting(t *testing.T) {
	for _, args := range [][]string{
		{}, {"--phases", ""}, {"--phases", "build,,verify"}, {"--phases", "build\n@value 1"},
		{"--phases", "build", "--total", "0"}, {"--phases", "build", "--total", "-1"},
		{"--phases", "build", "--width", "wide"}, {"--phases", "build", "--output", "yaml"},
		{"--phases", "build", "--timeout", "0s"},
	} {
		t.Run(strings.Join(args, " "), func(t *testing.T) {
			f := newFakeTmux(t)
			cmd := newTmuxStartCommandWith(f.dependencies())
			cmd.SetArgs(args)
			if err := cmd.Execute(); err == nil {
				t.Fatal("expected validation error")
			}
			if len(f.commands) != 0 {
				t.Fatalf("tmux invoked before validation: %#v", f.commands)
			}
		})
	}
}

func TestTmuxStartShellOutputAndNoAutoClose(t *testing.T) {
	f := newFakeTmux(t)
	var output bytes.Buffer
	cmd := newTmuxStartCommandWith(f.dependencies())
	cmd.SetOut(&output)
	cmd.SetArgs([]string{"--phases", "build,verify", "--output", "shell", "--auto-close=false"})
	if err := cmd.Execute(); err != nil {
		t.Fatal(err)
	}
	if len(f.events) != 4 {
		t.Fatalf("unexpected hook: %#v", f.events)
	}
	for _, line := range []string{"PB_PANE='%42'", "PB_PID='1234'", "PB_WINDOW='@7'", "PB_SOCK=" + shellQuote(f.socket)} {
		if !strings.Contains(output.String(), line+"\n") {
			t.Errorf("output missing %q: %s", line, output.String())
		}
	}
	if f.events[1] != "@set-total 2" {
		t.Fatalf("default total = %s", f.events[1])
	}
}

func TestShellQuoteRoundTrip(t *testing.T) {
	for _, value := range []string{"", "simple", "a'b", "$(touch never); `echo unsafe`\n spaced ", "a\"b"} {
		out, err := exec.Command("/bin/sh", "-c", "printf '%s' "+shellQuote(value)).Output()
		if err != nil || string(out) != value {
			t.Fatalf("quote %q: got %q, %v", value, out, err)
		}
	}
}

type failingWriter struct{}

func (failingWriter) Write([]byte) (int, error) { return 0, errors.New("output failed") }

type fakeTmux struct {
	t        *testing.T
	commands [][]string
	events   []string
	socket   string
	failure  string
}

func newFakeTmux(t *testing.T) *fakeTmux {
	t.Helper()
	return &fakeTmux{t: t}
}

func (f *fakeTmux) command(name string) []string {
	for _, cmd := range f.commands {
		if cmd[0] == name {
			return cmd
		}
	}
	return nil
}

func (f *fakeTmux) dependencies() tmuxDependencies {
	return tmuxDependencies{
		getenv: func(key string) string {
			if key == "TMUX_PANE" {
				return "%11"
			}
			return "/tmp/tmux/session"
		},
		executable: func() (string, error) { return "/opt/test bin/pb", nil },
		send: func(ctx context.Context, path string, lines []string) error {
			f.events = append(f.events, lines...)
			if f.failure == "send" {
				return errors.New("ack failed")
			}
			return nil
		},
		run: func(ctx context.Context, args ...string) (string, error) {
			f.commands = append(f.commands, append([]string(nil), args...))
			switch args[0] {
			case "display-message":
				if slices.Contains(args, "#{pane_height}") {
					if f.failure == "wrong height" {
						return "3", nil
					}
					return "2", nil
				}
				return "@7", nil
			case "split-window":
				matches := regexp.MustCompile(`'--socket-path' '([^']+)'`).FindStringSubmatch(args[len(args)-1])
				if len(matches) != 2 {
					f.t.Fatalf("missing socket argument: %#v", args)
				}
				f.socket = matches[1]
				f.t.Cleanup(func() { _ = os.RemoveAll(filepath.Dir(f.socket)) })
				if f.failure != "missing socket" {
					listener, err := net.Listen("unix", f.socket)
					if err != nil {
						f.t.Fatal(err)
					}
					f.t.Cleanup(func() { _ = listener.Close() })
				}
				if f.failure == "wrong window" {
					return "%42\t1234\t@8\n", nil
				}
				return "%42\t1234\t@7\n", nil
			case "capture-pane":
				if f.failure == "missing detail" {
					return "0%\n\n", nil
				}
				return "bar 0%\nbuild • verify\n", nil
			}
			return "", nil
		},
	}
}

func TestTmuxStartRecoversPaneWhenSplitReplyIsLost(t *testing.T) {
	for _, reply := range []string{"", "%", "%4", "%42\t1234\t@7\n"} {
		t.Run("reply_"+reply, func(t *testing.T) {
			f := newFakeTmux(t)
			deps := f.dependencies()
			run := deps.run
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			renderer := ""
			deps.run = func(ctx context.Context, args ...string) (string, error) {
				if args[0] == "list-panes" {
					f.commands = append(f.commands, args)
					if ctx.Err() != nil {
						t.Fatal("recovery inherited canceled context")
					}
					return "%11\t11\t" + renderer + "\n" +
						"%20\t20\tunrelated user command\n" +
						"bad-id\t22\t" + renderer + "\n" +
						"%23\t0\t" + renderer + "\n" +
						"%24\tinvalid\t" + renderer + "\n" +
						"%42\t1234\t" + renderer + "\n", nil
				}
				output, err := run(ctx, args...)
				if args[0] == "split-window" {
					renderer = args[len(args)-1]
					cancel()
					return reply, context.Canceled
				}
				return output, err
			}
			cmd := newTmuxStartCommandWith(deps)
			cmd.SetContext(ctx)
			cmd.SetArgs([]string{"--phases", "build,verify"})
			if err := cmd.Execute(); !errors.Is(err, context.Canceled) {
				t.Fatalf("got %v, want split cancellation", err)
			}
			if !reflect.DeepEqual(f.command("list-panes"), []string{"list-panes", "-t", "@7", "-F", "#{pane_id}\t#{pane_pid}\t#{pane_start_command}"}) {
				t.Fatalf("recovery did not inspect the original window: %#v", f.command("list-panes"))
			}
			kills := 0
			for _, command := range f.commands {
				if command[0] != "kill-pane" {
					continue
				}
				kills++
				if !reflect.DeepEqual(command, []string{"kill-pane", "-t", "%42"}) {
					t.Fatalf("killed unrelated pane: %#v", command)
				}
			}
			if kills != 1 {
				t.Fatalf("killed %d panes; want exactly one owned pane", kills)
			}
			if _, err := os.Stat(filepath.Dir(f.socket)); !os.IsNotExist(err) {
				t.Fatalf("private socket directory leaked: %v", err)
			}
		})
	}
}
