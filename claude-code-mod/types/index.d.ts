/**
 * the handles `progress-bar-3000 tmux-start` printed for the pane this mod
 * owns. plain data, so they survive a hot reload.
 */
export type OwnedPane = {
  pane: string
  socket: string
  pid: number
  window: string
}

declare module 'claude-code' {
  interface PluginState {
    'progress-bar-3000': { pane: OwnedPane | null }
  }
}
