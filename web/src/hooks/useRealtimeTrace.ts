import { useEffect, useRef, useCallback } from 'react'
import type { Span } from '../types/index'
import { WebSocketClient } from '../lib/websocket'

/**
 * Hook for real-time trace updates via WebSocket
 * Streams new spans as they arrive in the trace
 * Fires callback when new span arrives for animation
 */
export function useRealtimeTrace(
  traceID: string,
  onNewSpan?: (span: Span) => void,
  onError?: (error: string) => void
) {
  const wsRef = useRef<WebSocketClient | null>(null)
  const isConnectingRef = useRef(false)

  const connect = useCallback(async () => {
    if (isConnectingRef.current) return
    if (wsRef.current) return

    isConnectingRef.current = true

    try {
      const protocol = window.location.protocol === 'https:' ? 'wss:' : 'ws:'
      const wsUrl = `${protocol}//${window.location.host}/api/v1/traces/${traceID}/watch`

      const client = new WebSocketClient(wsUrl)

      // Subscribe to span events
      client.onMessage((data: unknown) => {
        const message = data as { type: string; span?: Span }
        if (message.type === 'span_added') {
          onNewSpan?.(message.span!)
        }
      })

      client.onError((error: string) => {
        onError?.(error)
      })

      await client.connect()
      wsRef.current = client
    } catch (error) {
      const message = error instanceof Error ? error.message : 'Connection failed'
      onError?.(message)
    } finally {
      isConnectingRef.current = false
    }
  }, [traceID, onNewSpan, onError])

  const disconnect = useCallback(() => {
    if (wsRef.current) {
      wsRef.current.disconnect()
      wsRef.current = null
    }
  }, [])

  useEffect(() => {
    connect()
    return () => disconnect()
  }, [connect, disconnect])

  return { connected: wsRef.current !== null, disconnect }
}
