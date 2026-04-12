import { useEffect, useRef, useState } from 'react'
import { createWebSocketClient, WebSocketClient } from '../lib/websocket'
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
    // TODO: WebSocket streaming endpoint not yet implemented on backend
    // For now, use REST polling or query-based loading
    // When backend implements /v1/signals/stream, uncomment code below:

    /*
    // Determine WebSocket URL based on current location
    const protocol = location.protocol === 'https:' ? 'wss' : 'ws'
    const host = location.host
    const wsUrl = `${protocol}://${host}/v1/signals/stream`

    // Create WebSocket client
    const ws = createWebSocketClient(wsUrl)

    // Handle incoming signals
    const unsubscribeMessage = ws.onMessage((data) => {
      try {
        const signal = data as ArgusSignal
        // Ensure signal.timestamp is a date string (already should be from backend)
        if (signal && typeof signal === 'object' && 'signal_id' in signal) {
          store.addSignal(signal)
        }
      } catch (err) {
        console.error('Failed to process signal:', err)
      }
    })

    // Handle connection errors
    const unsubscribeError = ws.onError((err) => {
      console.error('WebSocket error:', err)
      setError(err)
      setIsConnected(false)
    })

    // Connect to WebSocket
    ws.connect()
      .then(() => {
        setIsConnected(true)
        setError(null)
        store.setSubscribed(true)
      })
      .catch((err) => {
        const errMsg = err instanceof Error ? err.message : String(err)
        console.error('WebSocket connection failed:', errMsg)
        setError(errMsg)
        setIsConnected(false)
      })

    wsRef.current = ws

    // Cleanup on unmount
    return () => {
      unsubscribeMessage()
      unsubscribeError()
      ws.disconnect()
      store.setSubscribed(false)
    }
    */

    // Mark as subscribed even without WebSocket for now
    store.setSubscribed(true)
    setIsConnected(false)
    setError('WebSocket streaming not yet implemented on backend')

    return () => {
      store.setSubscribed(false)
    }
  }, [])

  return { isConnected, error }
}
