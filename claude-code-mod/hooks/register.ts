// drives the go progress-bar-3000 renderer in a tmux pane of its own. the mod
// draws nothing inside claude code: the renderer animates in its pane, so the
// prompt's ui thread does no work per frame.
//
// the first batch starts the pane with `tmux-start` (which also registers the
// renderer's own cleanup hook for completion); every batch after goes over the
// socket with `send`, which exits zero only once the renderer applied it all.

import { atom, read, update } from 'claude-code'
import type { EngineInterface, Register } from 'claude-code'

import type { OwnedPane } from '../types'

const COMMAND = 'progress-bar'
const MAX_EVENTS = 256

const owned = atom({ plugin: 'progress-bar-3000', key: 'pane' } as const, null)

const TOOL_DESCRIPTION = `drives a live progress bar in a two-row tmux pane below the conversation: a full-width gradient bar with the percentage, and one row showing the whole phase plan with the current phase highlighted.

use it for multi-phase tasks expected to take longer than 5-10 seconds. send a batch of events per call; the renderer validates the batch whole before anything applies, so an error changes nothing.

events (one string each):
- "@set-phases a,b,c" must be the first event of the first call: it opens the pane with that plan. sending it later replaces the plan and resets progress.
- "@set-total N" sets the denominator (defaults to the phase count).
- "@value N" sets absolute progress; "@tick [N]" / "@inc N" add completed work.
- "@phase-name NAME" / "@phase INDEX" selects a phase (zero-based index).
- "@set-subphases x,y" / "@subphase-name x" / "@subphase INDEX" manage the current phase's children.
- "@label TEXT", "@meta key=value", "@reset".
- "@close" (last in its batch) closes the pane without completing (use when the task fails or is abandoned).
- json objects are accepted too, e.g. {"type":"value","value":2,"phase":"build","subphase":"link"}; a first call may open with {"type":"reset","total":4,"phases":["fetch",{"name":"build","subphases":["compile","link"]}]}.

rules: advance only on completed work and change phase at real transitions; never manufacture progress on a timer. 100% means the declared work really finished: the pane then closes by itself. if work stops short, send "@close" rather than faking completion.

typical start: ["@set-phases build,test,review", "@phase-name build"]. typical milestone: ["@value 1", "@phase-name test"].`

type Runtime = {
  binary: string
  // one batch at a time: two parallel calls must not both open a pane.
  busy: Promise<void> | null
}

type Batch = {
  // the plan to open a pane with, when the batch starts one.
  phases: string | null
  // what goes over the socket, in order.
  events: string[]
  close: boolean
}

// how a batch opens a pane: its first event's plan, if it has one.
function openingPlan(first: string): { phases: string; forward: boolean } | null {
  const match = /^@set-phases\s+(.+)$/.exec(first.trim())
  if (match !== null) {
    // tmux-start sends this plan itself.
    return { phases: match[1]!.trim(), forward: false }
  }
  if (!first.trim().startsWith('{')) {
    return null
  }
  let event: unknown
  try {
    event = JSON.parse(first)
  } catch {
    return null
  }
  const { type, phases } = (event ?? {}) as { type?: unknown; phases?: unknown }
  if ((type !== 'reset' && type !== 'set_phases') || !Array.isArray(phases) || phases.length === 0) {
    return null
  }
  const names = phases.map(phase => (typeof phase === 'string' ? phase : (phase as { name?: unknown } | null)?.name))
  if (names.some(name => typeof name !== 'string' || name === '' || name.includes(','))) {
    return null
  }
  // the json event still goes over the socket: it carries totals and children.
  return { phases: names.join(','), forward: true }
}

// replacing the renderer's completion hook would orphan its pane, and a
// socket producer's hook runs arbitrary shell, so neither form is accepted.
function isHookEvent(line: string): boolean {
  const trimmed = line.trim()
  if (/^@on-complete\b/.test(trimmed)) {
    return true
  }
  return trimmed.startsWith('{') && /"type"\s*:\s*"on_complete"/.test(trimmed)
}

function parse(lines: readonly string[], open: boolean): Batch {
  if (lines.length === 0) {
    throw new Error('no events were given')
  }
  if (lines.length > MAX_EVENTS) {
    throw new Error(`at most ${MAX_EVENTS} events per batch`)
  }
  const events = lines.map(line => line.trim()).filter(line => line !== '')
  if (events.some(isHookEvent)) {
    throw new Error('completion hooks belong to the mod: it closes the pane itself')
  }
  const closeAt = events.indexOf('@close')
  if (closeAt !== -1 && closeAt !== events.length - 1) {
    throw new Error('@close must be the last event of its batch')
  }
  const close = closeAt !== -1
  const forwarded = close ? events.slice(0, -1) : events
  if (!open || close) {
    return { phases: null, events: forwarded, close }
  }
  const plan = forwarded.length > 0 ? openingPlan(forwarded[0]!) : null
  if (plan === null) {
    throw new Error('no bar is running: open one with "@set-phases a,b,c" as the first event')
  }
  return { phases: plan.phases, events: plan.forward ? forwarded : forwarded.slice(1), close: false }
}

async function lock(rt: Runtime): Promise<() => void> {
  while (rt.busy !== null) {
    await rt.busy
  }
  let release: () => void = () => {}
  rt.busy = new Promise<void>(resolve => {
    release = resolve
  })
  return () => {
    rt.busy = null
    release()
  }
}

// the checkout's own renderer: `make build` puts it in the repo root, the
// directory above this mod's.
function checkoutBinary(root: string): string {
  const pluginDir = root.replace(/\/\.claude-plugin\/?$/, '').replace(/\/$/, '')
  return `${pluginDir.slice(0, pluginDir.lastIndexOf('/'))}/progress-bar-3000`
}

async function binaryPath($: EngineInterface, rt: Runtime): Promise<string> {
  if (rt.binary === '') {
    return checkoutBinary($.plugin.root)
  }
  if (!rt.binary.startsWith('~/')) {
    return rt.binary
  }
  const home = await $.env.get('HOME')
  if (home === undefined || home === '') {
    throw new Error(`cannot expand ${rt.binary}: HOME is unset`)
  }
  return `${home}${rt.binary.slice(1)}`
}

function failure(what: string, result: { exitCode: number; stdout: string; stderr: string }): Error {
  const detail = (result.stderr.trim() || result.stdout.trim() || `exit code ${result.exitCode}`).split('\n').slice(-3).join('; ')
  return new Error(`${what} failed: ${detail}`)
}

// the pane is ours only while it still runs the renderer we started: tmux
// reuses nothing within a server, but the pid check also rules out a server
// restarted since.
async function isAlive($: EngineInterface, pane: OwnedPane): Promise<boolean> {
  const result = await $.process.run(['tmux', 'display-message', '-p', '-t', pane.pane, '#{pane_pid}'])
  return result.exitCode === 0 && result.stdout.trim() === String(pane.pid)
}

// the owned pane, or null once it has closed (on completion, its own hook
// removed it): the stale handles are forgotten then.
async function livePane($: EngineInterface): Promise<OwnedPane | null> {
  const pane = await read($, owned)
  if (pane === null) {
    return null
  }
  if (await isAlive($, pane)) {
    return pane
  }
  await update($, owned, () => null)
  return null
}

function parseHandles(stdout: string): OwnedPane {
  let handles: Partial<OwnedPane>
  try {
    handles = JSON.parse(stdout) as Partial<OwnedPane>
  } catch {
    throw new Error(`tmux-start printed no handles: ${stdout.trim().slice(0, 200)}`)
  }
  const { pane, socket, pid, window } = handles
  if (typeof pane !== 'string' || typeof socket !== 'string' || typeof pid !== 'number' || typeof window !== 'string') {
    throw new Error(`tmux-start printed incomplete handles: ${stdout.trim().slice(0, 200)}`)
  }
  return { pane, socket, pid, window }
}

async function openPane($: EngineInterface, bin: string, phases: string): Promise<OwnedPane> {
  // tmux-start splits the pane claude runs in, whichever window is in view.
  if ((await $.env.get('TMUX')) === undefined || (await $.env.get('TMUX_PANE')) === undefined) {
    throw new Error('claude is not running inside tmux, so there is no pane to split')
  }
  const started = await $.process.run([bin, 'tmux-start', '--phases', phases, '--width', 'full', '--output', 'json'], { timeoutMs: 15_000 })
  if (started.exitCode !== 0) {
    throw failure('tmux-start', started)
  }
  const pane = parseHandles(started.stdout)
  await update($, owned, () => pane)
  // only this pane's border, not its neighbours'.
  await $.process.run(['tmux', 'set-option', '-p', '-t', pane.pane, 'pane-border-format', ''])
  return pane
}

// tmux-start keeps the socket in a private directory of its own.
function socketDirectory(socket: string): string | null {
  const slash = socket.lastIndexOf('/')
  const directory = slash > 0 ? socket.slice(0, slash) : ''
  return /^\/tmp\/pb-[^/]+$/.test(directory) ? directory : null
}

async function closePane($: EngineInterface, pane: OwnedPane): Promise<void> {
  if (await isAlive($, pane)) {
    await $.process.run(['tmux', 'kill-pane', '-t', pane.pane])
  }
  await $.process.run(['rm', '-f', pane.socket])
  const directory = socketDirectory(pane.socket)
  if (directory !== null) {
    await $.process.run(['rmdir', directory])
  }
  await update($, owned, value => (value !== null && value.pane === pane.pane && value.pid === pane.pid ? null : value))
}

async function send($: EngineInterface, bin: string, pane: OwnedPane, events: readonly string[]): Promise<void> {
  if (events.length === 0) {
    return
  }
  const sent = await $.process.run([bin, 'send', '--socket-path', pane.socket, '--', ...events], { timeoutMs: 10_000 })
  if (sent.exitCode !== 0) {
    throw failure('send', sent)
  }
}

async function apply($: EngineInterface, rt: Runtime, lines: readonly string[]): Promise<string> {
  const release = await lock(rt)
  try {
    let pane = await livePane($)
    const batch = parse(lines, pane === null)
    if (batch.close && pane === null) {
      return 'no bar is running'
    }
    const bin = await binaryPath($, rt)
    let opened = false
    if (pane === null) {
      pane = await openPane($, bin, batch.phases!)
      opened = true
    }
    await send($, bin, pane, batch.events)
    if (batch.close) {
      await closePane($, pane)
      return `closed pane ${pane.pane}`
    }
    // an opening @set-phases went through tmux-start rather than the socket.
    const count = lines.filter(line => line.trim() !== '').length
    return `${opened ? `opened pane ${pane.pane}; ` : ''}applied ${count} event${count === 1 ? '' : 's'}`
  } finally {
    release()
  }
}

async function describe($: EngineInterface): Promise<string> {
  const pane = await livePane($)
  return pane === null ? 'no bar is running' : `bar running in pane ${pane.pane} (socket ${pane.socket})`
}

async function closeOwned($: EngineInterface, rt: Runtime): Promise<void> {
  const release = await lock(rt)
  try {
    const pane = await read($, owned)
    if (pane !== null) {
      await closePane($, pane)
    }
  } finally {
    release()
  }
}

export const register: Register = (on, options) => {
  const rt: Runtime = {
    binary: typeof options.binary === 'string' ? options.binary.trim() : '',
    busy: null,
  }

  on('session.start', async ($, e, next) => {
    const started = await next(e)
    await $.tool.register({
      name: 'progress',
      description: TOOL_DESCRIPTION,
      inputSchema: {
        type: 'object',
        properties: {
          events: {
            type: 'array',
            items: { type: 'string' },
            minItems: 1,
            maxItems: MAX_EVENTS,
            description: 'event lines applied in order as one batch',
          },
        },
        required: ['events'],
      },
    })
    await $.command.register({
      name: COMMAND,
      description: 'show or close the progress bar pane, or send it events',
      argumentHint: '[close | <event> ; <event> ...]',
    })
    return started
  })

  // a pane left behind would outlive the work it reported on.
  on('session.end', async ($, e, next) => {
    await closeOwned($, rt)
    return next(e)
  })

  on('tool.call', { tool: 'mcp__progress-bar-3000__progress' }, async ($, e) => {
    const events = (e as unknown as { events?: unknown }).events
    if (!Array.isArray(events) || events.some(line => typeof line !== 'string')) {
      return { deny: 'events must be an array of strings' }
    }
    try {
      return { result: await apply($, rt, events as string[]) }
    } catch (error) {
      return { deny: (error as Error).message }
    }
  })

  on('command.run', { command: COMMAND }, async ($, e) => {
    const args = e.args.trim()
    if (args === '') {
      return { text: await describe($) }
    }
    const lines = args === 'close' ? ['@close'] : args.split(';').map(part => part.trim())
    try {
      return { text: await apply($, rt, lines) }
    } catch (error) {
      return { text: (error as Error).message }
    }
  })
}
