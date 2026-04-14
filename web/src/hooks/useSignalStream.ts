import { useEffect, useRef, useState } from 'react'
import { WebSocketClient } from '../lib/websocket'
import { useSignalStore } from '../stores/signal'
import type { ArgusSignal } from '../types'

/**
 * useSignalStream Hook
 *
 * Manages WebSocket subscription lifecycle for real-time signal delivery.
 * Automatically connects on mount, disconnects on unmount.
 * Dispatches incoming signals to the Zustand signal store.
 *
 * @returns Object with connection status and error state
 */
export function useSignalStream() {
  const [isConnected, setIsConnected] = useState(false)
  const [error, setError] = useState<string | null>(null)
  const store = useSignalStore()
  const wsRef = useRef<WebSocketClient | null>(null)

  useEffect(() => {
    const protocol = location.protocol === 'https:' ? 'wss' : 'ws'
    const wsUrl = `${protocol}://${location.host}/v1/signals/stream`

    const ws = new WebSocketClient(wsUrl)

    const unsubscribeMessage = ws.onMessage((data) => {
      try {
        const signal = data as ArgusSignal
        if (signal && typeof signal === 'object' && 'signal_id' in signal) {
          store.addSignal(signal)
        }
      } catch (err) {
        console.error('Failed to process signal:', err)
      }
    })

    const unsubscribeError = ws.onError((err) => {
      console.error('WebSocket error:', err)
      setError(err)
      setIsConnected(false)
    })

    ws.connect()
      .then(() => {
        setIsConnected(true)
        setError(null)
        store.setSubscribed(true)
      })
      .catch((err) => {
        const errMsg = err instanceof Error ? err.message : String(err)
        setError(errMsg)
        setIsConnected(false)
      })

    wsRef.current = ws

    return () => {
      unsubscribeMessage()
      unsubscribeError()
      ws.disconnect()
      store.setSubscribed(false)
    }
  }, [])

  return { isConnected, error }
}
