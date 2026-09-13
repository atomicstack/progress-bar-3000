---
name: progress-bar-3000
description: this skill should be used when driving a multi-step or long-running task on behalf of a user — builds, deploys, batch jobs, iterating over a known list of items — and a live progress bar with phase labels is wanted instead of a wall of log lines. trigger phrases include "show progress", "add a progress bar", "surface progress for this run", "drive the bar". drive the renderer by emitting events into stdin or a unix socket; register a completion hook to remove its own pane automatically.
---

# progress-bar-3000

a cli that draws a gradient progress bar and phase labels in the terminal. feed it events; it draws the bar.

## mandatory agent checklist

1. resolve your own window from `tmux display-message -p -t "$TMUX_PANE" '#{window_id}'`. every split must use `-t "$TMUX_PANE"`; never use the currently active window. abort pane creation if this lookup fails. verify the returned pane's window id matches, and save its exact pane id, pid and socket path.
2. initialize phases with `@set-phases a,b,c` before `@phase-name a`. `--detail=phase` only displays a name; it does not create phases. `@set-phases` resets progress, so send total/value again when changing a running phase list.
3. actually send events after creating the bar. use the verified direct socket sender below on macos. a successful producer exit is not proof that the renderer received anything.
4. before the final completion event, capture the exact pane with `tmux capture-pane -p -t "$pane"` after initialization and each incomplete milestone. once a teardown hook is registered, the final event closes the pane, so verify its absence instead of capturing it. allow a short bounded redraw delay, then require a nonempty phase and the expected progress. if the display disagrees, fix delivery before reporting success; never create extra panes to repair one stalled bar.
5. advance on completed work and update phase at transitions, including tests/review. during long phases, use meaningful fractional steps or visible subphases; never manufacture progress on a timer. explain genuine waits instead of leaving an unexplained stale bar. 100% means the declared task/batch really finished.
6. use `--style gradient-granular --tint-animation=cycle --format '%p %{percent}'` plus exactly one phase row: `--detail-format '%{phases}'` when the plan is known up front (send `@set-phases` first), else `--detail=phase`. that is bar + one row, so the bottom pane is exactly two rows (`-l 2`); add one row per extra detail row. display the phase only once. reuse an existing owned pane when appropriate. register an exact-pane completion hook so successful completion removes only the pane you created.

### verified socket sender

short-lived `nc -U` / `nc -w` producers have returned successfully without updating the renderer in the agent environment. do not assume a universal netcat bug or blame permissions without testing. this direct sender has been verified by inspecting the displayed pane:

```bash
pb_send() {
  /usr/bin/python3 -c '
import socket, sys
with socket.socket(socket.AF_UNIX, socket.SOCK_STREAM) as client:
    client.settimeout(2)
    client.connect(sys.argv[1])
    client.sendall(sys.stdin.buffer.read())
    client.shutdown(socket.SHUT_WR)
' "$1"
}
printf '@set-phases build,test,review\n@set-total 3\n@value 0\n@phase-name build\n' | pb_send "$sock"
# later, after the build actually passes:
printf '@value 1\n@phase-name test\n' | pb_send "$sock"
tmux capture-pane -p -t "$pane"
```

use `python -c`, not a heredoc, because stdin carries the event stream. on another platform use its available python 3 binary. if the sandbox denies tmux/socket access, request a narrowly scoped escalation; do not repeatedly retry unchanged commands or infer success from an empty tool result. this sender changes no files and does not touch the device being worked on.

## automatic completion cleanup

register this after the phase initialization and before doing work, using the pane id returned by your own split:

```sh
printf '@on-complete tmux kill-pane -t %s\n' "$pane" | pb_send "$sock"
```

`--on-complete 'command'` sets the same hook at startup. the socket form is convenient because the pane id is only known after creation. json clients send `{"type":"on_complete","command":"..."}`. `@on-complete` without an argument clears it.

hooks run once when the actual value is at least a positive effective total, independent of rounding and animation. setting a hook after completion fires it immediately if no hook has fired. a reset (including `@set-phases`) keeps the hook and re-arms it. changing the hook after it has fired does not re-arm it. never send the final value until all task work and verification have passed.

commands run in `/bin/sh -c` with inherited environment/cwd, no stdin, and output on stderr; they do not interpolate progress tokens. hooks execute serially in the background; orderly exit, including ctrl-c, waits for them and reports failures with a nonzero exit. keep cleanup finite: there is no hook timeout. socket files use mode `0600`; keep their directory private and accept only trusted event producers, since they can register shell commands.

pane teardown can terminate its subprocesses, so put other cleanup before `tmux kill-pane` if combining commands. otherwise keep responsibility for temporary directories and socket files with the caller. on task failure below 100%, remove only your recorded pane explicitly; never fake 100% just to trigger cleanup.

## locating the binary

snippets below invoke `./progress-bar-3000`. pick the right path for the install:

- **repo checkout:** run `make build` at the repo root; the binary lands beside the `Makefile` as `./progress-bar-3000`. run snippets with cwd at the repo root, or substitute the absolute path.
- **claude code plugin install:** the binary lives at `$CLAUDE_PLUGIN_ROOT/progress-bar-3000` once `make build` has been run inside the plugin directory. substitute `$CLAUDE_PLUGIN_ROOT/progress-bar-3000` wherever snippets show `./progress-bar-3000`. if the binary is missing (check before first use), run `make -C "$CLAUDE_PLUGIN_ROOT" build` first.

## When to reach for this

- you know the total step count up front (or can set it with `@set-total`)
- the task takes long enough that a user would otherwise stare at silence
- you have natural "phase" boundaries (build / test / deploy, item 1..N, stage names)

Skip it for: tasks under a few seconds, tasks with unknown unbounded work, non-interactive runs where no human is watching (stdout must be a tty — the renderer refuses otherwise).

## The shape of every run

1. start the renderer with `--total N` and optionally `--phase-file` or `--detail`
2. emit one event per tick of progress (plus phase / label changes)
3. register the completion hook while progress is incomplete. in socket mode, send the final value only after successful verification. stdin mode exits at eof and may snap to 100% even when its producer failed: do not use stdin mode for fallible agent workflows. the stdin examples below demonstrate event syntax, not a verified build pipeline.

## Usage modes

Pick the mode that fits the producer.

### control protocol (recommended for agents)

Structured `@command` lines on stdin. Composable, easy to emit from shell, easy to reason about.

```bash
{
  printf '@set-phases build,test,package\n'
  printf '@set-total 3\n'
  printf '@phase-name build\n'
  printf '@tick\n' # illustration: emit only after verified build success
  printf '@phase-name test\n'
  printf '@tick\n' # illustration: emit only after verified test success
  printf '@phase-name package\n'
  printf '@tick\n' # illustration: emit only after verified package success
} | ./progress-bar-3000 --style gradient-granular --tint-animation cycle --fps 30 --format '%p %{percent}' --detail=phase
```

Supported commands:

| command              | effect                                                 |
|----------------------|--------------------------------------------------------|
| `@tick [amount]`     | advance by `amount` (default 1)                        |
| `@inc <amount>`      | same as tick but amount is required                    |
| `@value <n>`         | set absolute progress value                            |
| `@set-total <n>`     | change the denominator                                 |
| `@phase <index>`     | jump to phase index (0-based)                          |
| `@phase-name <name>` | jump to phase by name (must exist in the plan)         |
| `@label <text>`      | free-form label shown in the template                  |
| `@meta key=value`    | bind `%{meta:key}` in the format string                |
| `@set-phases a,b,c`  | replace the phase plan                                 |
| `@reset`             | zero the value, clear label/meta, reset rate samples; re-arm the hook |
| `@on-complete <command>` | set the completion shell command; omit it to clear |

### sub-phase plans and combined updates

use one level of children when a parent has finer steps. only the current parent's selected child appears in the phase strip: `build [compile] › test › package`. `%{phase}` and `--detail=phase` include the same suffix; `%{subphase}` is the bare child name. this fits the existing two-row pane: keep `--format '%p %{percent}' --detail-format '%{phases}'`.

startup plans can use a json phase file:

```json
["fetch", {"name":"build","subphases":["compile","link"]}, "test", "package"]
```

pass it with `--phase-file plan.json`; `--phase build` optionally selects the starting parent. json `reset` accepts this same mixed array in `phases`. plain-text files and flat string arrays still work.

when children become known during work, define them without resetting progress:

```sh
printf '@phase-name build\n@set-subphases compile,link\n' | pb_send "$sock"
# after moving to the linking task, without advancing progress:
printf '@subphase-name link\n' | pb_send "$sock"
# or update actual progress and both labels atomically:
printf '%s\n' '{"type":"value","value":42,"phase":"build","subphase":"link"}' | pb_send "$sock"
```

`phase` and `subphase` are optional on json `value` and `tick` events (the latter accepts fractional `amount`). the explicit parent overrides the normal value-to-phase mapping for that event. omit it to target the parent selected by the new value. use json `phase` with `name` or `index` plus `subphase` to select both labels without a value change. standalone `@subphase 1` (zero-based) or `@subphase-name link` never advances progress. sub-phase selection is optional on every progress update.

configure or select a future parent's child without changing the current parent:

```json
{"type":"set_subphases","phase":"test","subphases":["unit","integration"]}
{"type":"subphase","phase":"test","name":"integration"}
```

omit `phase` to target the current parent. json `subphase` also accepts zero-based `index`; `name` wins when both are present. invalid parent or child selections are ignored. child names must be nonempty strings; use json for names containing commas.

- the first child is selected by default; switching parents remembers each selection.
- replacing a child list selects its first entry. `@set-subphases` with no argument or json `"subphases":[]` clears the list. neither operation resets progress, affects rate samples, changes the total, or re-arms a completion hook.
- `@reset` preserves child plans but resets all selections to their first child. replacing the parent plan (`@set-phases` or json reset with `phases`) discards old child plans; structured json can supply replacements.
- progress remains explicitly controlled by value/tick events. reaching the last child does not mean the parent or run completed. keep using verified milestones and send the final value only after all work succeeds.

### json protocol

Same semantics, stricter format — use when the producer is a program, not a shell.

```bash
{
  printf '{"type":"reset","total":3,"phases":["build","test","ship"]}\n'
  printf '{"type":"tick","amount":1}\n'
  printf '{"type":"phase","name":"test"}\n'
  printf '{"type":"tick","amount":1}\n'
} | ./progress-bar-3000 --input-mode json --style gradient-granular --tint-animation cycle --format '%p %{percent}' --detail=phase
```

### plain line mode

One line of input = one completed step. The line text becomes the label.

```bash
printf '%s\n' build test ship | ./progress-bar-3000 --total 3
```

### value mode

Each line is an absolute numeric value (0..total).

```bash
printf '%s\n' 1 2 3 | ./progress-bar-3000 --total 3 --input-mode value
```

### phase file bootstrap

Pre-load the phase plan from a file so you don't need a `@reset`:

```bash
./progress-bar-3000 --phase-file testdata/phase-files/phases.txt --total 3
```

### socket mode

The renderer listens on a unix socket instead of stdin. Run it in one pane; send events from many short-lived clients in another. This is the right mode when the producer isn't the renderer's parent process — e.g. the agent is driving a subprocess pipeline and wants the bar in a separate pane.

```bash
./progress-bar-3000 --socket-path /tmp/pb3.sock --total 5 &
# ...then, from any process:
printf '@set-phases build,test,review\n@set-total 5\n@phase-name build\n' | pb_send /tmp/pb3.sock
```

In socket mode the renderer does **not** exit when the producer disconnects — it keeps listening for the next client. register a completion hook to remove its owned tmux pane automatically, or stop it explicitly when finished.

### running inside tmux

When the agent is already running inside a multiplexer (`[ -n "$TMUX" ]`), the best experience is to put the bar in its own small pane at the top or bottom of the window, driven by socket mode. The agent's own pane stays free for logs and tool output; the bar pane is just a status line.

```bash
# without tmux, use socket mode in an existing tty for fallible work;
# if no suitable tty exists, report that the live display is unavailable.
if [[ -n "${TMUX-}" && -n "${TMUX_PANE-}" ]]; then
  own_window=$(tmux display-message -p -t "$TMUX_PANE" '#{window_id}') || exit 1
  test -n "$own_window" || exit 1
  sock_dir=$(mktemp -d "${TMPDIR:-/tmp}/pb3.XXXXXX") || exit 1
  sock="$sock_dir/progress.sock"
  n=3

  # size the bar to fill the window. -6 leaves room for " 100%" after the
  # bar — the format below intentionally drops %{phase} because the
  # %{phases} row already shows the plan with the current phase highlighted.
  # if you keep %{phase} in the inline format, reserve enough columns
  # for the longest phase name or it gets truncated ("submit" → "sub").
  bar_width=$(($(tmux display-message -p -t "$TMUX_PANE" '#{window_width}') - 6))
  (( bar_width < 10 )) && bar_width=10

  # -d  : don't steal focus — keep the agent's pane active
  # -f  : span the full window width (not just the active pane's width)
  # -t  : mandatory own-pane target; never follow the user's active window
  # omit -b: put the full-width pane at the bottom
  # -v  : vertical split (panes stacked), -l 2 = 2 rows tall (bar + phases row)
  # -P -F '#{pane_id}' echoes the new pane id so we can kill it later
  # pre-quote the child command with printf '%q' since tmux reparses via sh -c;
  # printf '%q' works in bash and zsh, unlike ${var@Q} (bash 4.4+ only).
  #
  # resolve the executable before splitting; the child may inherit another cwd.
  # this is the installed checkout here; adapt using "locating the binary".
  progress_binary="${CLAUDE_PLUGIN_ROOT:-$PWD}/progress-bar-3000"
  test -x "$progress_binary" || exit 1
  cmd=$(printf '%q ' "$progress_binary" \
    --socket-path "$sock" \
    --total "$n" \
    --width "$bar_width" \
    --style gradient-granular \
    --tint-animation cycle \
    --format '%p %{percent}' \
    --detail-format '%{phases}')
  pane="$(tmux split-window -t "$TMUX_PANE" -d -f -v -l 2 -P -F '#{pane_id}' "$cmd")" || exit 1
  tmux set-option -p -t "$pane" pane-border-format ""
  actual_window=$(tmux display-message -p -t "$pane" '#{window_id}') || exit 1
  test "$actual_window" = "$own_window" || exit 1
  pane_pid=$(tmux display-message -p -t "$pane" '#{pane_pid}') || exit 1
  printf 'pane=%s window=%s pid=%s socket=%s\n' "$pane" "$own_window" "$pane_pid" "$sock"

  # wait at most two seconds for the renderer to bind its socket
  for attempt in {1..40}; do
    [[ -S "$sock" ]] && break
    sleep 0.05
  done
  test -S "$sock" || exit 1

  # use pb_send from the verified socket sender above; initialize before work
  printf '@set-phases build,test,review\n@set-total 3\n@phase-name build\n' | pb_send "$sock"
  printf '@on-complete tmux kill-pane -t %s\n' "$pane" | pb_send "$sock"
  sleep 0.1
  tmux capture-pane -p -t "$pane"
  # ... more events as work progresses ...

  # stop here. leave the pane running; this setup block must not tear it down.
  # after successful verification, the final @value 3 closes this pane.
  # verify its absence; clean up any remaining socket file and directory.
fi
```


after the entire task and its verification succeed, complete the bar and verify automatic removal with a bounded wait:

```sh
printf '@value %s\n' "$n" | pb_send "$sock"
for attempt in {1..40}; do
  tmux list-panes -a -F '#{pane_id}' | grep -Fxq -- "$pane" || break
  sleep 0.05
done
if tmux list-panes -a -F '#{pane_id}' | grep -Fxq -- "$pane"; then
  printf 'completion hook did not remove the progress pane\n' >&2
  exit 1
fi
rm -f -- "$sock"
rmdir -- "$sock_dir"
```

Key points:

- `-t "$TMUX_PANE" -d -f -v -l 2` → two-row full-width bottom pane (bar + phases row) in the agent's own window, without stealing focus. never omit the target.
- `-P -F '#{pane_id}'` prints the new pane id (e.g. `%42`) — capture it so you can target `tmux kill-pane -t` later.
- size `--width` from `#{window_width}` minus a small reserve for trailing format tokens, not a fixed literal — hardcoding 40 on a 195-column window leaves most of the bar empty.
- **format / detail must not duplicate the phase.** the default `--format` is `'%p %{percent} %{phase}'` and `--detail=phase` prints the phase on its own row. running both at once both duplicates the phase and truncates the inline copy whenever the phase name is longer than your width reserve (long names like `submit` clip to `sub`). pick one home for the phase: drop `%{phase}` from `--format` (the recipe above) and let the detail row own it, or drop the detail row and widen the reserve to fit the longest phase name + a space.
- `--detail-format '<template>'` is repeatable. use it whenever the built-in `--detail=phase/value/label` rows aren't the shape you want (e.g. `phase: %{phase} [%{label}]` to fuse two signals into one row, or `%{value}/%{total} @ %{rate}` for a value+rate row). same token grammar as `--format`. each occurrence adds one row below the keyed `--detail` rows, in the order given. bump `-l 2` → `-l 3` (etc.) if you add more rows than the pane can fit.
- wait for the socket file with a bounded timeout before the first send; then verify rendered phase/progress, not just socket existence.
- use a unique socket path in a directory from `mktemp -d` so concurrent agent runs don't collide. keep it short: macos rejects unix socket paths over about 104 bytes with `bind: invalid argument`, so use `mktemp -d -t pb3` under `$TMPDIR`, never a long scratchpad or project path.
- frames occupy exactly the rows they print (bar + one detail row = 2 rows), so `-l 2` is enough for the default recipe. if the pane is shorter than the frame, bubble tea drops rows from the top and the bar vanishes first, so add one `-l` row per extra detail row.
- without `$TMUX`, do not split. for fallible agent work use socket mode in an existing tty, or explain that no live display is available. inline piped mode is only for syntax demonstrations or infallible streams because eof can imply completion.
- register the completion hook while progress is incomplete, targeting only the captured pane id. the final success event then closes that pane automatically. on failure or cancellation below 100%, clean up only that recorded pane explicitly. never target a pane by name, position, or `-a`. the caller still owns temporary-directory/socket cleanup.

## Flags worth knowing

| flag                                 | what it does                                                           |
|--------------------------------------|------------------------------------------------------------------------|
| `--total N` / `--current N`          | denominator and starting value                                         |
| `--phase-file path`                  | load phase names and optional json sub-phase plans before events arrive                              |
| `--phase name`                       | set the starting phase by name                                         |
| `--style ...`                        | plain / block / granular / shaded / gradient-{block,granular,shaded}   |
| `--bg-style ...`                     | none / space / ascii / shade-light / shade-medium / shade-dark / custom|
| `--bg-char '▓'`                      | background rune when `--bg-style custom`                               |
| `--tint-animation ...`               | pulse / shimmer / cycle — all fps-independent (omit for no animation)  |
| `--gradient-start` / `--gradient-end`| hex colours (`#ffffff` etc.)                                           |
| `--color-mode ...`                   | auto (default) / truecolor / 256 / 16 / none                           |
| `--ascii`                            | force `=` fill and `.` track glyphs regardless of style (colour kept)  |
| `--fps 15 \| 30 \| 60`                | render rate                                                            |
| `--lerp 0.18`                        | display-value smoothing factor (0 < lerp ≤ 1; higher = snappier)       |
| `--width N`                          | fixed bar width in columns; omitted or 0 uses 90% of terminal columns and follows resizing |
| `--detail[=keys]`                    | extra rows below the bar; comma list of `label`/`phase`/`value`, or `all` (bare `--detail` = `all`) |
| `--detail-format '<template>'`       | extra detail row rendered via the same format-token grammar as `--format`; repeatable, rendered after the keyed `--detail` rows |
| `--format '...'`                     | pv-style template (see tokens below)                                   |
| `--socket-path /abs/path.sock`       | listen on a unix socket instead of stdin                               |
| `--on-complete command` | run a shell command once at actual 100%; resets re-arm it |

### format-string tokens

Used in `--format` and in `@label` templating. `%%` is a literal `%`.

- `%p` or `%{progress}` — the bar itself
- `%{bar-only}` — same bar, explicit token form
- `%{percent}` — `NN%`
- `%{phase}`, `%{phase-index}`, `%{phase-count}`
- `%{label}`, `%{value}`, `%{total}`
- `%{rate}` — ticks/s, `%e` / `%{eta}` — eta duration, `%t` / `%{timer}` — elapsed
- `%{phases}` — the whole phase plan on one line with only the current phase highlighted (bold, gradient-end colour, gentle saturation pulse); other phases dim. slides to keep the current phase visible when the plan is wider than the budget (width prefix, else terminal width), with `…` at clipped edges. empty when no plan is set, so always `@set-phases` first. put it on its own `--detail-format '%{phases}'` row; the default two-row recipe already includes this detail row.
- `%{meta:key}` — the current value of `@meta key=...`
- numeric widths: `%20{bar-only}`

## Patterns for an agent

### pattern A — you are the driver

You are executing a known list of steps yourself. Emit control events alongside each step:

```bash
{
  printf '@set-phases %s\n' "$(IFS=,; echo "${steps[*]}")"
  printf '@set-total %d\n' "${#steps[@]}"
  for step in "${steps[@]}"; do
    printf '@phase-name %s\n' "$step"
    printf '@tick\n' # illustration: emit only after this step has passed
  done
} | ./progress-bar-3000 --style gradient-granular --tint-animation cycle --format '%p %{percent}' --detail=phase
```

### pattern B — you are monitoring something you don't control

The work is happening in a subprocess whose output you shouldn't corrupt with bar escape codes. Use socket mode: start the renderer in its own pane, send events from a hook. Inside tmux, prefer the split-pane recipe in [running inside tmux](#running-inside-tmux) over a plain background process.

```bash
./progress-bar-3000 --socket-path "$sock" --phase-file phases.txt &
# later, from wherever, when a phase finishes:
printf '@phase-name %s\n@tick\n' "$next" | pb_send "$sock"
```

### pattern C — streaming numeric progress

You know a quantity (bytes processed, rows scanned) and can emit the running total. Use value mode so you don't have to compute deltas:

```bash
some_job --emit-progress | ./progress-bar-3000 --total 1000000 --input-mode value
```

### pattern D — embedding in a broader pv-shaped workflow

The existing convention in this environment uses `pv` in a secondary single-line pane for progress. `progress-bar-3000` drops into the same slot with richer phase info. Prefer it when you have named phases; stay with `pv` when the only signal is bytes over a pipe.

## Gotchas

- stdout must be a tty; the renderer errors out otherwise.
- when stdin is an interactive tty, the renderer hands stdin to bubble tea for keystrokes and does not read events from it — use socket mode if you want to feed events interactively.
- `ctrl-c` quits cleanly.
- `--socket-path` must be absolute.
- in control-protocol mode you can send any command the parser recognises even if `--input-mode` is something else — the parser auto-detects `{` and `@` prefixes.
- `@tick` accepts fractional amounts; fine-grained progress is fine.
- the format string uses `%%` for a literal `%`; bare `%x` where `x` isn't a known alias is an error.
