# progress-bar-3000

A renderer-only Go CLI for progress bars in shell scripts.

This repo also ships as a Claude Code plugin: `.claude-plugin/plugin.json`
declares the plugin, and `skills/progress-bar-3000/SKILL.md` contains the skill
that teaches agents how to drive the tool. Build the binary with `make build`
before invoking the skill; the skill resolves the binary via
`$CLAUDE_PLUGIN_ROOT/progress-bar-3000` when installed as a plugin, or
`./progress-bar-3000` in a repo checkout.

## Features

- Bubble Tea rendering in compact or detail mode
- hybrid input protocol: plain lines, control commands, and JSON lines
- `pv`-style format strings
- gradient bars with truecolor, 256-color, and 16-color output
- configurable background tracks and custom background rune
- lerp smoothing and optional tint animation
- startup phase files plus runtime reset/rebase commands
- Unix socket input mode with reconnect support

## Examples

### One line equals one completed step

```bash
printf 'build\ntest\npackage\n' | go run . --total 3
```

### Numeric value mode

```bash
printf '1\n2\n3\n' | go run . --total 3 --input-mode value
```

### Control protocol

```bash
{
  printf '@set-total 3\n'
  printf '@phase-name build\n'
  printf '@tick\n'
  printf '@phase-name test\n'
  printf '@tick\n'
  printf '@phase-name package\n'
  printf '@tick\n'
} | go run . --style gradient-granular --bg-style shade-light --fps 30
```

### JSON protocol

```bash
{
  printf '{"type":"reset","total":3,"phases":["build","test","package"]}\n'
  printf '{"type":"tick","amount":1}\n'
  printf '{"type":"phase","name":"test"}\n'
  printf '{"type":"tick","amount":1}\n'
  printf '{"type":"phase","name":"package"}\n'
  printf '{"type":"tick","amount":1}\n'
} | go run . --input-mode json --detail
```

### Phase file

```bash
go run . --phase-file testdata/phase-files/phases.txt --total 3
```

### Socket mode

Start the renderer in one terminal:

```bash
go run . --socket-path /tmp/progress-bar-3000.sock --total 3
```

Send progress updates from another process:

```bash
{
  printf '@tick\n'
  printf '@phase-name test\n'
  printf '@tick\n'
  printf '@phase-name package\n'
  printf '@tick\n'
} | nc -U /tmp/progress-bar-3000.sock
```
