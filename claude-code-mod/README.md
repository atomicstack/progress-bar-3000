# claude code mod

a claude code mod that drives the `progress-bar-3000` renderer in a two-row
tmux pane split below claude's own pane. for claude code it replaces the
skill: the model calls one tool instead of composing shell commands, and the
mod looks after the pane from start to finish. other agents, such as codex,
keep using [the skill](../skills/progress-bar-3000/SKILL.md).

claude code itself draws nothing, so the bar animates in its own pane and
costs the prompt's ui thread nothing.

## how it works

- the tool is `mcp__progress-bar-3000__progress`. it takes `{events: string[]}`,
  using the renderer's own event lines (`@value 1`, `@phase-name test`, json
  objects, ...).
- if no bar is running, the batch must open with `@set-phases a,b,c`, or with
  a json `reset` that carries `phases`. the mod then runs
  `progress-bar-3000 tmux-start --phases ... --width full --output json`
  against `$TMUX_PANE`. the pane splits from claude's own pane, whichever
  window is in view. the mod clears the new pane's `pane-border-format` and
  keeps the returned handles in mod state.
- every other batch goes through `progress-bar-3000 send --socket-path ...`.
  that command exits zero only once the renderer has applied the whole batch.
  if the renderer rejects a batch, the model gets the renderer's error.
- at 100%, the hook that `tmux-start` registered removes the socket directory
  and kills the pane. the next batch sees that the pane's pid no longer
  matches, and forgets the pane.
- `@close`, which must be the last event in its batch, kills only the pane the
  mod owns, after checking its pid. it also removes the pane's socket
  directory. the mod does the same at session end.
- the mod refuses `@on-complete` and json `on_complete`, because they would
  replace the cleanup hook.
- `/progress-bar` shows the pane, `/progress-bar close` closes it, and
  `/progress-bar @value 1 ; @phase-name test` sends events.

## config

`binary` is the path to the renderer. leave it empty to use the binary that
`make build` puts in the checkout above this directory. it needs v0.4.0 or
newer, for `tmux-start` and `send`.

## install

the mod is named `progress-bar-3000`, the same as the skill plugin at the repo
root. install one or the other.

```sh
make build
claude --plugin-dir ./claude-code-mod     # one session
ln -s "$PWD/claude-code-mod" ~/.claude/skills/progress-bar-3000   # every session
claude plugin test claude-code-mod        # the tests stand in for tmux and the binary
```
