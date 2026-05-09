/**
 * WebSocket client for live signal streaming.
 *
 * Connects to /ws/signals with cookie-based auth (sent automatically by browser).
 * Reconnects on close with exponential backoff: 1s, 2s, 4s, 8s, 30s (capped).
 */

import type { ArgusSignal } from '../types'

// Re-export Signal as alias for plan compatibility
export type Signal = ArgusSignal

export interface SignalSocketHandlers {
  onSignal: (signal: Signal) => void
  onStatus?: (status: 'connecting' | 'open' | 'closed' | 'error') => void
}

/**
 * Creates a WebSocket connection to /ws/signals.
 *
 * @param handlers - Signal and status callbacks
 * @returns Cleanup function — call it to permanently close the socket
 */
export function createSignalSocket(handlers: SignalSocketHandlers): () => void {
  let ws: WebSocket | null = null
  let attempt = 0
  let closed = false
  const delays = [1000, 2000, 4000, 8000, 30000]

  const connect = () => {
    if (closed) return
    handlers.onStatus?.('connecting')
    const proto = window.location.protocol === 'https:' ? 'wss:' : 'ws:'
    ws = new WebSocket(`${proto}//${window.location.host}/ws/signals`)

    ws.onopen = () => {
      attempt = 0
      handlers.onStatus?.('open')
    }

    ws.onmessage = (e) => {
      try {
        handlers.onSignal(JSON.parse(e.data) as Signal)
      } catch {
        // Ignore malformed messages
      }
    }

    ws.onerror = () => {
      handlers.onStatus?.('error')
    }

    ws.onclose = () => {
      handlers.onStatus?.('closed')
      if (closed) return
      const delay = delays[Math.min(attempt, delays.length - 1)]
      attempt++
      setTimeout(connect, delay)
    }
  }

  connect()

  return () => {
    closed = true
    // Capture local reference, null the outer binding BEFORE close().
    // The onclose/onerror handlers reference the outer `ws` variable;
    // by nulling it first we ensure handlers that fire after cleanup
    // (which can happen when close() is called during CONNECTING under
    // React StrictMode) see ws === null and their `handlers.onStatus?.('closed')`
    // / setTimeout(connect, ...) paths effectively no-op (closed === true).
    const local = ws
    ws = null
    // Detach handlers so the browser's CONNECTING-abort error doesn't
    // bubble to onerror / onclose with a misleading state.
    if (local) {
      local.onopen = null
      local.onmessage = null
      local.onerror = null
      local.onclose = null
      try { local.close() } catch { /* ignore — already closing */ }
    }
  }
}
