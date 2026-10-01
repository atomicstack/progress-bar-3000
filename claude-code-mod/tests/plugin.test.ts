import { expect, mock, test } from 'claude-code/testing'
import type { On } from 'claude-code'

const TOOL = 'mcp__progress-bar-3000__progress'
const BIN = '<renderer>'
const HANDLES = { pane: '%7', socket: '/tmp/pb-123/p.sock', pid: 4242, window: '@1' }

type Runs = { command: { run: (input: never) => Promise<{ text?: string }> } }
const runCommand = ($: Runs, args = '') => $.command.run({ command: 'progress-bar', args } as never)

type World = { calls: string[][]; alive: boolean; sendFails: string | null; raw: string[][] }

// stands in for tmux and the renderer binary: records each command, and
// answers as a pane that lives until it is killed or the bar completes.
function fakeHost(on: On, env: Record<string, string> = { HOME: '/home/me', TMUX: '/tmp/tmux-501/default,1,0', TMUX_PANE: '%1' }): World {
  const world: World = { calls: [], alive: false, sendFails: null, raw: [] }
  mock.env(on, env)
  on('process.run', async (_$, e) => {
    const argv = [...e.argv]
    world.raw.push(argv)
    // whichever renderer path the mod resolved, recorded under one name.
    world.calls.push(argv[0]?.endsWith('/progress-bar-3000') === true ? [BIN, ...argv.slice(1)] : argv)
    const result = { exitCode: 0, stdout: '', stderr: '', isStdoutTruncated: false, isStderrTruncated: false }
    const ok = (stdout = '') => ({ value: { ...result, stdout } })
    const isBin = argv[0]?.endsWith('/progress-bar-3000') === true
    if (isBin && argv[1] === 'tmux-start') {
      world.alive = true
      return ok(JSON.stringify(HANDLES) + '\n')
    }
    if (isBin && argv[1] === 'send') {
      if (world.sendFails !== null) {
        return { value: { ...result, exitCode: 1, stderr: world.sendFails } }
      }
      if (argv.includes('@value 3')) {
        world.alive = false
      }
      return ok()
    }
    // a symlinked install: the mod's directory resolves into the checkout.
    if (argv[0] === 'realpath') {
      return ok('/checkout/claude-code-mod\n')
    }
    if (argv[0] === 'tmux' && argv[1] === 'display-message') {
      return ok(world.alive ? `${HANDLES.pid}\n` : '\n')
    }
    if (argv[0] === 'tmux' && argv[1] === 'kill-pane') {
      world.alive = false
    }
    return ok()
  })
  return world
}

const named = (world: World, ...prefix: string[]) =>
  world.calls.filter(argv => prefix.every((part, i) => argv[i] === part))

test('without a binary option the mod runs the checkout binary beside its own directory', async ($, on) => {
  const world = fakeHost(on)
  await $.tool.call({ tool: TOOL, events: ['@set-phases a,b'] })
  // resolved through any symlink, so ~/.claude/skills/<link> finds the checkout.
  expect(named(world, 'realpath').length).toBe(1)
  expect(world.raw.find(argv => argv[1] === 'tmux-start')?.[0]).toBe('/checkout/progress-bar-3000')
})

test('the first batch opens a pane with its plan, then sends the rest over the socket', async ($, on) => {
  const world = fakeHost(on)
  const result = await $.tool.call({ tool: TOOL, events: ['@set-phases build,test,review', '@phase-name build'] })
  expect(result.deny).toBeUndefined()
  expect(String(result.result)).toContain('opened pane %7')
  expect(named(world, BIN, 'tmux-start')).toEqual([[BIN, 'tmux-start', '--phases', 'build,test,review', '--width', 'full', '--output', 'json']])
  expect(named(world, 'tmux', 'set-option')).toEqual([['tmux', 'set-option', '-p', '-t', '%7', 'pane-border-format', '']])
  expect(named(world, BIN, 'send')).toEqual([[BIN, 'send', '--socket-path', '/tmp/pb-123/p.sock', '--', '@phase-name build']])
})

test('later batches reuse the pane', async ($, on) => {
  const world = fakeHost(on)
  await $.tool.call({ tool: TOOL, events: ['@set-phases a,b,c'] })
  const result = await $.tool.call({ tool: TOOL, events: ['@value 1', '@phase-name b'] })
  expect(String(result.result)).toBe('applied 2 events')
  expect(named(world, BIN, 'tmux-start').length).toBe(1)
  expect(named(world, BIN, 'send')).toEqual([[BIN, 'send', '--socket-path', '/tmp/pb-123/p.sock', '--', '@value 1', '@phase-name b']])
})

test('a json reset with a structured plan opens the pane and still goes over the socket', async ($, on) => {
  const world = fakeHost(on)
  const reset = '{"type":"reset","total":4,"phases":["fetch",{"name":"build","subphases":["compile","link"]}]}'
  const result = await $.tool.call({ tool: TOOL, events: [reset] })
  expect(result.deny).toBeUndefined()
  expect(named(world, BIN, 'tmux-start')[0]?.[3]).toBe('fetch,build')
  expect(named(world, BIN, 'send')[0]?.slice(-1)).toEqual([reset])
})

test('without a running bar, a batch must open with a plan', async ($, on) => {
  const world = fakeHost(on)
  const result = await $.tool.call({ tool: TOOL, events: ['@value 1'] })
  expect(String(result.deny)).toContain('@set-phases')
  expect(world.calls.filter(argv => argv[0] === BIN).length).toBe(0)
})

test('once the bar completes and its pane closes itself, the next batch opens a new one', async ($, on) => {
  const world = fakeHost(on)
  await $.tool.call({ tool: TOOL, events: ['@set-phases a,b,c'] })
  await $.tool.call({ tool: TOOL, events: ['@value 3'] })
  expect(String((await $.tool.call({ tool: TOOL, events: ['@value 1'] })).deny)).toContain('no bar is running')
  await $.tool.call({ tool: TOOL, events: ['@set-phases x,y'] })
  expect(named(world, BIN, 'tmux-start').length).toBe(2)
})

test('@close kills only the owned pane and removes its socket directory', async ($, on) => {
  const world = fakeHost(on)
  await $.tool.call({ tool: TOOL, events: ['@set-phases a,b'] })
  const result = await $.tool.call({ tool: TOOL, events: ['@label failed', '@close'] })
  expect(String(result.result)).toBe('closed pane %7')
  expect(named(world, BIN, 'send').at(-1)?.slice(-1)).toEqual(['@label failed'])
  expect(named(world, 'tmux', 'kill-pane')).toEqual([['tmux', 'kill-pane', '-t', '%7']])
  expect(named(world, 'rm')).toEqual([['rm', '-f', '/tmp/pb-123/p.sock']])
  expect(named(world, 'rmdir')).toEqual([['rmdir', '/tmp/pb-123']])
  expect(await runCommand($)).toEqual({ text: 'no bar is running' })
})

test('@close must end its batch', async ($, on) => {
  fakeHost(on)
  await $.tool.call({ tool: TOOL, events: ['@set-phases a,b'] })
  expect(String((await $.tool.call({ tool: TOOL, events: ['@close', '@value 1'] })).deny)).toContain('last event')
})

test('completion hooks are refused: the renderer already owns the pane cleanup', async ($, on) => {
  const world = fakeHost(on)
  await $.tool.call({ tool: TOOL, events: ['@set-phases a,b'] })
  expect((await $.tool.call({ tool: TOOL, events: ['@on-complete rm -rf ~'] })).deny).toBeDefined()
  expect((await $.tool.call({ tool: TOOL, events: ['{"type":"on_complete","command":"true"}'] })).deny).toBeDefined()
  expect(named(world, BIN, 'send').length).toBe(0)
})

test('a rejected batch reports the renderer error', async ($, on) => {
  const world = fakeHost(on)
  await $.tool.call({ tool: TOOL, events: ['@set-phases a,b'] })
  world.sendFails = 'event 1: unknown control command "bogus"'
  expect(String((await $.tool.call({ tool: TOOL, events: ['@bogus'] })).deny)).toContain('unknown control command')
})

test('outside tmux nothing starts', async ($, on) => {
  const world = fakeHost(on, { HOME: '/home/me' })
  expect(String((await $.tool.call({ tool: TOOL, events: ['@set-phases a,b'] })).deny)).toContain('not running inside tmux')
  expect(world.calls.length).toBe(0)
})

test('the binary option takes an absolute path as given', { options: { binary: '/opt/pb/progress-bar-3000' } }, async ($, on) => {
  const world = fakeHost(on)
  await $.tool.call({ tool: TOOL, events: ['@set-phases a,b'] })
  expect(world.raw[0]?.[0]).toBe('/opt/pb/progress-bar-3000')
})

test('the command reports the pane and closes it', async ($, on) => {
  const world = fakeHost(on)
  await runCommand($, '@set-phases a,b ; @phase-name a')
  expect(await runCommand($)).toEqual({ text: 'bar running in pane %7 (socket /tmp/pb-123/p.sock)' })
  expect(await runCommand($, 'close')).toEqual({ text: 'closed pane %7' })
  expect(world.alive).toBe(false)
})
