# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## What this is

A renderer-only Go CLI (`progress-bar-3000`) that draws gradient progress bars and optional detail rows in a terminal. The same repo also ships as a Claude Code plugin — `.claude-plugin/plugin.json` plus `skills/progress-bar-3000/SKILL.md` teach agents how to drive the binary. Changes to the CLI's flag surface or output shape usually need a matching edit in the skill.

## Common commands

The `Makefile` pins `GOCACHE` and `GOMODCACHE` to in-repo directories, so prefer it over raw `go` invocations when you want cache hits.

| make target | command |
|---|---|
| `make build` | `go build -o ./progress-bar-3000 .` (binary lands at repo root, not `./dist`) |
| `make test` | `go test ./...` |
| `make vet` | `go vet ./...` |
| `make smoke` | builds, then runs `scripts/smoke.sh` (visual end-to-end check; requires a TTY) |

Single test / package: `go test ./internal/app/ -run 'TestModelViewCompact' -v`.

## Architecture

Top-down flow on every run:

1. `main.go` → `internal/cli.Execute` builds the cobra command, parses flags into `config.Config`, calls `cfg.Validate()`, and hands the config to `app.Run`.
2. `internal/app.Run` (`run.go`):
   - refuses to start unless stdout is a TTY
   - bootstraps a `progress.State` from `--total`, `--current`, `--phase-file`, etc.
   - resolves the color profile via `render.DetectProfile`
   - constructs the Bubble Tea program from `NewModel(cfg, initial)`
   - spawns a goroutine that pulls lines from an `input.Source`, parses each via `input.ParseLine`, and forwards `eventMsg`s into the program
   - the source is one of: stdin reader, stdin null-source (when stdin is an interactive TTY — Bubble Tea owns those bytes for key input), or a Unix socket listener (`--socket-path`)
3. `internal/app.Model` (`model.go`) implements `tea.Model`. `View()` renders `cfg.Format` via the `fmtx` template, then appends:
   - one fixed row per key in `--detail` (`detailRenderers` map: phase/value/label)
   - one templated row per `--detail-format` occurrence (parsed once in `NewModel`)
   - a trailing newline that absorbs Bubble Tea's shutdown `EraseEntireLine` so multi-line output survives. `--clear-on-exit` then erases the whole block from `Run`.

### Package responsibilities

- `internal/cli` — cobra wiring only. Flag → `config.Config` field mapping; no business logic.
- `internal/config` — `Config` struct + `Validate()`. Owns the `DetailKey` enum, `ParseDetail`, and validation of `--detail-format` templates (parses them through `internal/format` to fail fast on bad syntax).
- `internal/input` — input plumbing. `Source` abstracts stdin vs. socket; `ParseLine` auto-detects between three protocols by prefix: `{` → JSON, `@` → control commands (e.g. `@tick`, `@phase-name`, `@set-phases`), otherwise plain line / numeric value depending on `--input-mode`. Also reads `--phase-file` bootstrap.
- `internal/progress` — pure state: `State` with `Value`, `DisplayValue` (lerped target), `Phases`, `Label`, `Meta`. `Apply(event, now)` is the single mutation point.
- `internal/format` — `pv`-style template parser (`%p`, `%{percent}`, `%{phase}`, `%{meta:key}`, width prefixes like `%20{bar-only}`). The same parser is used by `--format` and by every `--detail-format` template. Has no imports from elsewhere in the project; safe to depend on from anywhere.
- `internal/render` — colour-profile detection and the bar-cell rasterisation (gradient, background style, animation tints). Decoupled from Bubble Tea.

### Key invariants

- **Format vs. detail must not duplicate.** The default `--format` includes `%{phase}`; `--detail=phase` and `--detail-format 'phase: ...'` also print the phase. Pick one home or you get duplication and (with short width reserves) inline truncation. The skill recipe drops `%{phase}` from the inline format.
- **TTY requirement.** `Run` errors out if stdout isn't a TTY. Tests that exercise `View()` directly (see `internal/app/model_test.go`) avoid the TTY guard by constructing the model and calling `View()` without going through `Run`.
- **Source vs. stdin.** When stdin is an interactive TTY, the input source becomes a no-op (`NewNullSource`) because Bubble Tea claims those bytes for key handling. Drive events via `--socket-path` in that case.
- **Socket mode doesn't self-terminate.** The renderer keeps listening across producer disconnects. Callers must `kill` the renderer process (or `tmux kill-pane -t <id>` against a pane *they spawned* for the bar — capture the id from `tmux split-window -P -F '#{pane_id}'` and target only that id; never kill panes by name, position, or `-a`) and unlink the socket file when done.

## Skill / plugin coupling

`skills/progress-bar-3000/SKILL.md` documents the CLI for agents and contains a tmux split-pane recipe. When you change a flag name, default, or output layout, update the skill in the same commit. The skill resolves the binary as `$CLAUDE_PLUGIN_ROOT/progress-bar-3000` when installed as a plugin and `./progress-bar-3000` in a repo checkout — keep both paths working.

## Testing notes

- The full suite is fast (~1s); run `go test ./...` after any change to `internal/app`, `internal/config`, `internal/cli`, or `internal/format`.
- `scripts/smoke.sh` is the manual visual check — it builds the binary, then drives several invocations and pauses between them. Use it when changing rendering, animations, or detail-row layout.
- `model_test.go` uses `stripANSI(m.View())` to assert on textual content while ignoring colour escapes; follow that pattern for new render assertions.
