---
name: progress-bar-3000
description: this skill should be used when driving a multi-step or long-running task on behalf of a user — builds, deploys, batch jobs, iterating over a known list of items — and a live progress bar with phase labels is wanted instead of a wall of log lines. trigger phrases include "show progress", "add a progress bar", "surface progress for this run", "drive the bar". the tool is a renderer only; drive it by emitting events into stdin or a unix socket, never by computing a bar directly.
---

# progress-bar-3000

a cli that draws a gradient progress bar and phase labels in the terminal. feed it events; it draws the bar.

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
3. close the input stream; the bar snaps to 100% and the process exits

## Usage modes

Pick the mode that fits the producer.

### control protocol (recommended for agents)

Structured `@command` lines on stdin. Composable, easy to emit from shell, easy to reason about.

```bash
{
  printf '@set-total 3\n'
  printf '@phase-name build\n'
  run_build   && printf '@tick\n'
  printf '@phase-name test\n'
  run_tests   && printf '@tick\n'
  printf '@phase-name package\n'
  run_package && printf '@tick\n'
} | ./progress-bar-3000 --style gradient-granular --fps 30 --detail
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
| `@reset`             | zero the value, clear label/meta, reset rate samples   |

### json protocol

Same semantics, stricter format — use when the producer is a program, not a shell.

```bash
{
  printf '{"type":"reset","total":3,"phases":["build","test","ship"]}\n'
  printf '{"type":"tick","amount":1}\n'
  printf '{"type":"phase","name":"test"}\n'
  printf '{"type":"tick","amount":1}\n'
} | ./progress-bar-3000 --input-mode json --detail
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
printf '@phase-name build\n@tick\n' | nc -U /tmp/pb3.sock
```

In socket mode the renderer does **not** exit when the producer disconnects — it keeps listening for the next client. Tear it down explicitly (`kill`, `tmux kill-pane`, or close the enclosing shell) when the task is finished.

### running inside tmux

When the agent is already running inside a multiplexer (`[ -n "$TMUX" ]`), the best experience is to put the bar in its own small pane at the top or bottom of the window, driven by socket mode. The agent's own pane stays free for logs and tool output; the bar pane is just a status line.

```bash
# only worth doing inside tmux — fall back to inline rendering otherwise
if [[ -n "${TMUX-}" ]]; then
  sock="$(mktemp -u -t pb3.XXXXXX).sock"

  # size the bar to fill the window. -6 leaves room for " 100%" after the
  # bar — the format below intentionally drops %{phase} because the
  # --detail-format row already prints "phase: <name> [<label>]" on its own
  # line. if you keep %{phase} in the inline format, reserve enough columns
  # for the longest phase name or it gets truncated ("submit" → "sub").
  bar_width=$(($(tmux display-message -p -t "$TMUX_PANE" '#{window_width}') - 6))
  (( bar_width < 10 )) && bar_width=10

  # -d  : don't steal focus — keep the agent's pane active
  # -f  : span the full window width (not just the active pane's width)
  # -b  : above the target; drop -b for a pane at the bottom
  # -v  : vertical split (panes stacked), -l 3 = 3 rows tall
  # -P -F '#{pane_id}' echoes the new pane id so we can kill it later
  # pre-quote the child command with printf '%q' since tmux reparses via sh -c;
  # printf '%q' works in bash and zsh, unlike ${var@Q} (bash 4.4+ only).
  #
  # --detail-format is a custom detail row rendered via the same format-token
  # grammar as --format. it composes phase + label onto one line so you can
  # see both at a glance without burning two detail rows. emit @label events
  # alongside @phase-name / @tick to populate the bracketed half.
  cmd=$(printf '%q ' ./progress-bar-3000 \
    --socket-path "$sock" \
    --total "$n" \
    --width "$bar_width" \
    --style gradient-granular \
    --tint-animation cycle \
    --format '%p %{percent}' \
    --detail-format 'phase: %{phase} [%{label}]')
  pane="$(tmux split-window -d -f -b -v -l 3 -P -F '#{pane_id}' "$cmd")"

  # wait for the renderer to bind the socket before sending events
  until [[ -S "$sock" ]]; do sleep 0.05; done

  # drive the bar from the agent's pane
  printf '@phase-name build\n@tick\n' | nc -U "$sock"
  # ... more events as work progresses ...

  # tear it down when the task is done
  tmux kill-pane -t "$pane" 2>/dev/null
  rm -f "$sock"
fi
```

Key points:

- `-d -f -b -v -l 3` → 3-row full-width pane at the top, agent's pane keeps focus. Drop `-b` for bottom.
- `-P -F '#{pane_id}'` prints the new pane id (e.g. `%42`) — capture it so you can target `tmux kill-pane -t` later.
- size `--width` from `#{window_width}` minus a small reserve for trailing format tokens, not a fixed literal — hardcoding 40 on a 195-column window leaves most of the bar empty.
- **format / detail must not duplicate the phase.** the default `--format` is `'%p %{percent} %{phase}'` and `--detail=phase` (or the `--detail-format` recipe above) prints the phase on its own row. running both at once both duplicates the phase and truncates the inline copy whenever the phase name is longer than your width reserve (long names like `submit` clip to `sub`). pick one home for the phase: drop `%{phase}` from `--format` (the recipe above) and let the detail row own it, or drop the detail row and widen the reserve to fit the longest phase name + a space.
- `--detail-format '<template>'` is repeatable. use it whenever the built-in `--detail=phase/value/label` rows aren't the shape you want (e.g. `phase: %{phase} [%{label}]` to fuse two signals into one row, or `%{value}/%{total} @ %{rate}` for a value+rate row). same token grammar as `--format`. each occurrence adds one row below the keyed `--detail` rows, in the order given. bump `-l 3` → `-l 4` (etc.) if you add more rows than the pane can fit.
- wait for the socket file to appear before the first `nc -U`; the renderer takes a frame or two to bind.
- use a unique socket path per run (`mktemp -u`) so concurrent agent runs don't collide.
- without `$TMUX`, fall back to inline piped mode (pattern A below) rather than trying to split something that isn't there.
- socket mode doesn't self-terminate; always clean up the pane and the socket file when the task finishes.

## Flags worth knowing

| flag                                 | what it does                                                           |
|--------------------------------------|------------------------------------------------------------------------|
| `--total N` / `--current N`          | denominator and starting value                                         |
| `--phase-file path`                  | load phase names before any events arrive                              |
| `--phase name`                       | set the starting phase by name                                         |
| `--style ...`                        | plain / block / granular / shaded / gradient-{block,granular,shaded}   |
| `--bg-style ...`                     | none / space / ascii / shade-light / shade-medium / shade-dark / custom|
| `--bg-char '▓'`                      | background rune when `--bg-style custom`                               |
| `--tint-animation ...`               | pulse / shimmer / cycle — all fps-independent (omit for no animation)  |
| `--gradient-start` / `--gradient-end`| hex colours (`#ffffff` etc.)                                           |
| `--color-mode ...`                   | auto (default) / truecolor / 256 / 16 / none                           |
| `--fps 15 \| 30 \| 60`                | render rate                                                            |
| `--lerp 0.18`                        | display-value smoothing factor (0 < lerp ≤ 1; higher = snappier)       |
| `--width N`                          | bar width in columns (default 20)                                      |
| `--detail[=keys]`                    | extra rows below the bar; comma list of `label`/`phase`/`value`, or `all` (bare `--detail` = `all`) |
| `--detail-format '<template>'`       | extra detail row rendered via the same format-token grammar as `--format`; repeatable, rendered after the keyed `--detail` rows |
| `--format '...'`                     | pv-style template (see tokens below)                                   |
| `--socket-path /abs/path.sock`       | listen on a unix socket instead of stdin                               |

### format-string tokens

Used in `--format` and in `@label` templating. `%%` is a literal `%`.

- `%p` or `%{progress}` — the bar itself
- `%{bar-only}` — same bar, explicit token form
- `%{percent}` — `NN%`
- `%{phase}`, `%{phase-index}`, `%{phase-count}`
- `%{label}`, `%{value}`, `%{total}`
- `%{rate}` — ticks/s, `%e` / `%{eta}` — eta duration, `%t` / `%{timer}` — elapsed
- `%{meta:key}` — the current value of `@meta key=...`
- numeric widths: `%20{bar-only}`

## Patterns for an agent

### pattern A — you are the driver

You are executing a known list of steps yourself. Emit control events alongside each step:

```bash
{
  printf '@set-total %d\n' "${#steps[@]}"
  for step in "${steps[@]}"; do
    printf '@phase-name %s\n' "$step"
    run_step "$step" && printf '@tick\n'
  done
} | ./progress-bar-3000 --detail
```

### pattern B — you are monitoring something you don't control

The work is happening in a subprocess whose output you shouldn't corrupt with bar escape codes. Use socket mode: start the renderer in its own pane, send events from a hook. Inside tmux, prefer the split-pane recipe in [running inside tmux](#running-inside-tmux) over a plain background process.

```bash
./progress-bar-3000 --socket-path "$sock" --phase-file phases.txt &
# later, from wherever, when a phase finishes:
printf '@phase-name %s\n@tick\n' "$next" | nc -U "$sock"
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
