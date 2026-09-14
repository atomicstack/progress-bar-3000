---
name: progress-bar-3000
description: use for multi-step or long-running agent tasks with known milestones, such as builds, deployments and batch jobs, when a live terminal progress bar is useful. start a two-row tmux pane in one call, send acknowledged progress and phase/subphase events, and close the owned pane automatically on completion.
---

# progress-bar-3000

a terminal progress renderer. report completed work; it draws the bar. use socket mode for fallible agent workflows: stdin eof can mark an incomplete producer run as complete.

## one-call start, one-line updates

these helpers require a source build after v0.3.0. in a checkout, run `make build` once if needed and use the absolute path to its `progress-bar-3000` binary. for a source-based plugin install, use `$CLAUDE_PLUGIN_ROOT/progress-bar-3000` after `make -C "$CLAUDE_PLUGIN_ROOT" build`. release installations can use the binary on `PATH` when their version includes these helpers. substitute that binary path for `progress-bar-3000` in every example; no `--help` probing is needed.

inside tmux, start the whole initialized display with one invocation:

```sh
progress-bar-3000 tmux-start --phases build,test,review --total 3 --width full --auto-close
```

success prints one json object with the **actual** `pane`, `socket`, `pid`, and `window` handles. save those handles in your task state; use the returned socket path literally in subsequent calls. shell variables and functions do not persist between agent tool calls. for example, if the returned socket is `/tmp/pb-123456/p.sock`:

```sh
progress-bar-3000 send --socket-path /tmp/pb-123456/p.sock '@value 1' '@phase-name test'
```

substitute your actual returned path; the example path is illustrative. emit this update only after the build succeeds. after tests pass, send value `2` and phase `review`; after review and all declared work succeed, send value `3`. the final update is acknowledged before the automatic cleanup hook runs.

to retain handles across independent shell calls, redirect startup output to a unique task-owned json file in your existing scratch directory. that file may have a long path; the socket itself remains in a short private directory. later, one line can extract its socket:

```sh
progress-bar-3000 send --socket-path "$(jq -r '.socket' /your/task/progress.json)" '@value 1' '@phase-name test'
```

`tmux-start --output shell` instead prints safely quoted `PB_PANE`, `PB_SOCK`, `PB_PID`, and `PB_WINDOW` assignments for callers that explicitly want shell state. use `eval "$(progress-bar-3000 tmux-start --phases build,test,review --output shell)"` only within a shell call that needs those variables; retain the handles separately for later calls.

## agent defaults at a glance

`tmux-start` supplies the renderer settings below automatically. `send` is a separate subcommand; renderer flags do not belong on it.

| renderer flag | agent default | notes |
|---|---|---|
| `--socket-path` | generated `/tmp/pb-…/p.sock` | private directory `0700`, socket `0600` |
| `--total` | verified step count | bootstrap defaults to the phase count |
| `--width` | `full` | fits the complete row, reserving space for percent/text; `0` means a 90% bar |
| `--style` | `gradient-granular` | filled gradient with fractional cells |
| `--tint-animation` | `cycle` | smooth colour cycling |
| `--format` | `'%p %{percent}'` | bar and percent only |
| `--detail-format` | `'%{phases}'` | exactly one detail row; initialize with `@set-phases` first |
| `--on-complete` | set through the socket | bootstrap registers cleanup against its returned pane id |

| helper flag | default | purpose |
|---|---|---|
| `tmux-start --phases a,b,c` | required | comma-separated, nonempty names |
| `tmux-start --total N` | phase count | positive work total |
| `tmux-start --width full` | `full` | full, automatic `0`, or positive columns |
| `tmux-start --auto-close` | `true` | remove its socket directory and pane on completion; `--auto-close=false` disables this |
| `tmux-start --output json` | `json` | json handles or `shell` assignments |
| `tmux-start --timeout 10s` | `10s` | bounded startup and initial-frame verification |
| `send --socket-path PATH` | required | use the actual returned socket |
| `send --json` | `false` | require json objects instead of mixed event lines |
| `send --timeout 3s` | `3s` | deadline for connection, delivery and acknowledgement |

## mandatory agent rules

1. advance only on completed work and change phase at transitions, including tests and review. during long phases, use meaningful fractional steps or visible subphases. never manufacture progress on a timer. explain genuine waits. 100% means the declared task or batch really finished.
2. use `tmux-start` when the plan is known. it sends `@set-phases` before any other event, sets total/value, selects the first phase, registers cleanup, and verifies the initial two-row frame. `@set-phases` resets progress; do not resend it at each milestone.
3. require `send` to exit zero. zero confirms the renderer applied every event in the batch. missing/refused sockets, rejected events, and missing/invalid acknowledgements exit nonzero. after a timeout or lost reply, events may already have applied: do not blindly retry `@tick` or other non-idempotent batches. inspect the saved pane before deciding how to recover.
4. a successful acknowledgement confirms state application, not a rendered pixel or successful cleanup command. bootstrap checks the initial frame; capture the exact saved pane when diagnosing display problems, rather than after every successful update. after final completion, verify the saved pane is absent. do not create extra panes to repair a stalled one.
5. keep the display to a bar plus one phase row, exactly two rows high. use full width in the parent pane's viewport and show phase information only once. when a plan genuinely cannot be known, manually start with `--detail=phase` instead of `--detail-format '%{phases}'`; this displays a name but does not create a plan.
6. clean up every pane you create. bootstrap does so on successful completion and rolls back failed startup. if task work fails below 100%, remove only the recorded owned pane and its private socket directory; never fake completion to trigger cleanup. verify ownership using the saved handles, never kill processes by name.

## running inside tmux

`tmux-start` requires both `TMUX` and the agent's own `TMUX_PANE`. it resolves that pane's window, then uses **the pane itself as the target**. tmux accepts a pane for `-t`: one split invocation both sizes the child and starts its shell command. it does not depend on whichever window the user is currently viewing.

the underlying form is:

```sh
tmux split-window -d -v -l 2 -t "$TMUX_PANE" -P \
  -F '#{pane_id}\t#{pane_pid}\t#{window_id}' 'exec /absolute/path/to/progress-bar-3000 ...'
```

this illustrates the mechanism; use `tmux-start` for the complete runnable bootstrap. it shell-quotes the executable/arguments, verifies the returned window matches, retains the exact pane id and pid, and hides only the new pane's border label using the correct argument order:

```sh
tmux set-option -p -t "$PB_PANE" pane-border-format ""
```

missing tmux context, failed window lookup, startup errors or a missing initial frame fail nonzero. if a split reply is lost, rollback identifies the owned pane by its exact startup command and unique socket path in the original window. no `send-keys` step is needed. if a sandbox denies tmux or socket access, use a narrowly scoped escalation; do not infer success from empty output or retry unchanged commands repeatedly.

## event recipes

`send` accepts one event per argument, or newline-delimited stdin when there are no event arguments. one invocation is a single batch: syntax is validated before any state changes; all events apply before the acknowledgement and before completion hooks start. use the same current binary for sender and renderer: older renderers do not support the acknowledgement envelope.

```sh
progress-bar-3000 send --socket-path "$sock" '@value 1' '@phase-name test'
printf '@value 2\n@phase-name review\n' | progress-bar-3000 send --socket-path "$sock"
progress-bar-3000 send --socket-path "$sock" --json '{"type":"value","value":2,"phase":"review"}'
```

these examples assume `sock` is set in the same shell call; otherwise substitute the saved socket path. limits are 256 events and 1 mib for the encoded request. success is silent. unknown parent/child selections retain the existing event semantics: they are ignored, not reported as delivery errors.

| event | effect |
|---|---|
| `@value N` | set absolute progress; fractional values supported |
| `@tick [N]` / `@inc N` | add completed work; tick defaults to one |
| `@set-total N` | change the denominator |
| `@phase-name NAME` / `@phase INDEX` | choose an existing parent; indices start at zero |
| `@set-phases a,b,c` | replace the plan and reset progress; set total/value again if needed |
| `@label TEXT` | set `%{label}` |
| `@meta key=value` | set `%{meta:key}` |
| `@reset` | reset progress, label/meta and rate samples; retain plans and re-arm the hook |
| `@on-complete COMMAND` | set a shell hook; omit command to clear it |

### subphases and combined updates

one level of children appears after the current parent, such as `build [compile] › test › review`. `%{phase}` includes the child suffix and `%{subphase}` is the bare child name. keep the same two-row layout.

```sh
progress-bar-3000 send --socket-path "$sock" '@phase-name build' '@set-subphases compile,link'
# change labels without changing progress:
progress-bar-3000 send --socket-path "$sock" '@subphase-name link'
# update progress and both labels together when that work completes:
progress-bar-3000 send --socket-path "$sock" --json '{"type":"value","value":1,"phase":"build","subphase":"link"}'
```

`phase` and `subphase` are optional on json `value` and `tick` events. an explicit parent overrides automatic value-to-phase selection for that event; without it the child targets the parent selected by the new value. json `phase` accepts `name` or `index` plus optional `subphase` to change both labels alone.

configure a future parent's children without selecting that parent:

```sh
progress-bar-3000 send --socket-path "$sock" --json '{"type":"set_subphases","phase":"test","subphases":["unit","integration"]}'
```

`@subphase INDEX` and `@subphase-name NAME` select the current parent's child. json `subphase` also accepts optional `phase` and either `name` or `index` (`name` wins). the first child is selected by default and each parent remembers its selection. replacing a child list selects its first entry; an empty list clears it. child changes do not advance progress, change totals, or re-arm hooks.

startup `--phase-file plan.json` accepts mixed names and objects:

```json
["fetch", {"name":"build","subphases":["compile","link"]}, "test"]
```

for a bootstrap-created pane, send a json `reset` with this `phases` array and the desired `total` while progress is still zero. `@reset` retains child plans and returns selections to their first entries. replacing the parent plan discards old children; structured json can provide replacements. reaching the last child never implies completion.

## completion and cleanup

bootstrap's default hook removes its socket file, removes its private directory, then invokes `tmux kill-pane -t` with the exact returned pane id. with `--auto-close=false`, cleanup is the caller's responsibility. if you replace the hook to add other cleanup, retain these actions and put pane removal last: removing the pane may terminate its subprocesses.

```sh
progress-bar-3000 send --socket-path "$sock" "@on-complete tmux kill-pane -t $pane"
```

this manual example only closes the pane; the caller still owns directory cleanup. `--on-complete 'command'` configures the same hook at renderer startup. json uses `{"type":"on_complete","command":"..."}`.

hooks fire once when the actual value is at least a positive effective total, independent of rounding and animation. setting a hook after completion fires it immediately if none has fired. resets, including `@set-phases`, retain and re-arm the hook. replacing an already-fired hook does not re-arm it. a batch containing completion, reset and completion can queue multiple invocations; hooks start in event order after acknowledgement.

commands run serially through `/bin/sh -c`, inherit environment/cwd, receive no stdin, and write output to stderr. normal shutdown waits for hooks and reports failures nonzero; there is no hook timeout, so keep cleanup finite. labels and format tokens are not interpolated into commands. allow only trusted socket producers, because they can register shell commands.

## socket mode without tmux

start the renderer in a tty using a short private directory:

```sh
sock_dir=$(mktemp -d /tmp/pb3.XXXXXX)
printf 'socket: %s/p.sock\n' "$sock_dir"
progress-bar-3000 --socket-path "$sock_dir/p.sock" --total 3 --width full \
  --style gradient-granular --tint-animation=cycle \
  --format '%p %{percent}' --detail-format '%{phases}'
# after the renderer exits normally:
rmdir "$sock_dir"
```

from another tool call, send `@set-phases build,test,review` first, then total/value and the first phase, using the printed socket path. the listener stays alive across client disconnects and completion; stop it explicitly or configure a suitable completion hook.

paths must be absolute and must not already exist. use a unique `0700` directory and `0600` socket. keep the socket in a short directory under `/tmp`, not a deep scratchpad or macos `$TMPDIR`: unix socket paths are byte-limited. usable limits are 103 bytes on macos and 107 on linux (104/108-byte `sun_path` storage including the terminator). overly long paths fail before binding with the actual byte count, platform limit, and a shorter-directory hint. `tmux-start` chooses a short unique path for you.

## appendix: legacy raw clients

prefer the built-in `send` command, including outside tmux. if working with an older renderer that lacks acknowledgements, a raw client can still send newline-delimited events. this compatibility fallback does **not** acknowledge application; a successful exit is insufficient. allow a bounded redraw delay and inspect the exact renderer pane after initialization and each incomplete milestone. after completion with a teardown hook, verify pane absence instead.

```sh
printf '@value 1\n@phase-name test\n' | /usr/bin/python3 -c '
import socket, sys
with socket.socket(socket.AF_UNIX, socket.SOCK_STREAM) as client:
    client.settimeout(2)
    client.connect(sys.argv[1])
    client.sendall(sys.stdin.buffer.read())
    client.shutdown(socket.SHUT_WR)
' /actual/short/socket/path
```

use the platform's python 3 executable if different. stdin carries events, so use `-c`, not a heredoc. short-lived netcat producers have sometimes exited successfully without delivering updates in agent environments; diagnose with observed renderer state, not assumptions about permissions or universal netcat behaviour.
