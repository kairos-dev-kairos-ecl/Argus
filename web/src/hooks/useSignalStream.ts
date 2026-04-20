import { useEffect, useRef, useCallback, useState } from 'react'
import { createWebSocketClient, WebSocketClient } from '../lib/websocket'
import { getSignals } from '../services/api'
import { useSignalStore } from '../stores/signal'
import { useAuthStore } from '../stores/auth'
import type { ArgusSignal, LayerStatus, Layer } from '../types'
import { LAYERS } from '../types'

/**
 * useSignalStream Hook
 *
 * Manages WebSocket connection to backend signal stream and fetches
 * initial signals and layer status on mount.
 *
 * Returns:
 * - signals: ArgusSignal[] from store
 * - isConnected: boolean indicating WebSocket connection status
 * - error: string | null containing last error message
 * - filter: current signal filter
 * - setFilter: function to update filter
 * - refetchSignals: function to manually refresh signal list
 * - layerStatus: LayerStatus[] computed from signals
 */
export function useSignalStream() {
  const store = useSignalStore()
  const { token } = useAuthStore()
  const wsRef = useRef<WebSocketClient | null>(null)
  const unsubscribeRef = useRef<{ message: () => void; error: () => void }>({ message: () => {}, error: () => {} })
  const [wsConnected, setWsConnected] = useState(false)

  /**
   * Compute layer status from recent signals
   */
  const layerStatus: LayerStatus[] = LAYERS.map((layer: Layer, index: number) => {
    const layerNum = index + 1

    // Count signals for this layer from last 5 minutes
    const now = Date.now()
    const fiveMinutesAgo = now - 5 * 60 * 1000
    const recentSignals = store.signals.filter((s) => {
      const signalTime = typeof s.timestamp === 'string'
        ? new Date(s.timestamp).getTime()
        : s.timestamp.seconds * 1000 + ((s.timestamp as any).nanos ?? 0) / 1000000
      return s.layer === layerNum && signalTime > fiveMinutesAgo
    })

    // Determine status based on signal count and severity
    let status: 'green' | 'yellow' | 'gray' | 'red'
    if (recentSignals.length === 0) {
      status = 'gray'
    } else if (recentSignals.some((s) => s.severity >= 4)) {
      status = 'red'
    } else if (recentSignals.some((s) => s.severity >= 3)) {
      status = 'yellow'
    } else {
      status = 'green'
    }

    // Get last signal timestamp
    const lastSignal = recentSignals[0]
    const lastSignalTime = lastSignal
      ? typeof lastSignal.timestamp === 'string'
        ? lastSignal.timestamp
        : new Date(lastSignal.timestamp.seconds * 1000 + ((lastSignal.timestamp as any).nanos ?? 0) / 1000000).toISOString()
      : null

    return {
      layer,
      status,
      signal_count_5min: recentSignals.length,
      last_signal_time: lastSignalTime
    }
  })

  /**
   * Fetch initial signals on mount
   */
  const refetchSignals = useCallback(async () => {
    try {
      store.setLoading(true)
      const response = await getSignals({ limit: 100, app_id: 'test' })

      if (response && Array.isArray(response.signals)) {
        store.clearSignals()
        response.signals.forEach((signal) => {
          store.addSignal(signal)
        })
      }
      store.setError(null)
    } catch (error) {
      const message = error instanceof Error ? error.message : 'Failed to fetch signals'
      store.setError(message)
    } finally {
      store.setLoading(false)
    }
  }, [store])

  /**
   * Initialize WebSocket connection on mount
   */
  useEffect(() => {
    // Fetch initial signals
    refetchSignals()

    // Setup WebSocket connection
    const setupWebSocket = async () => {
      if (!token) {
        store.setError('Not authenticated - WebSocket requires valid token')
        return
      }

      try {
        // Create WebSocket client
        const protocol = location.protocol === 'https:' ? 'wss' : 'ws'
        const wsUrl = `${protocol}://${location.host}/ws/signals`
        const client = createWebSocketClient(wsUrl)
        wsRef.current = client

        // Setup message handler for incoming signals
        const unsubscribeMessage = client.onMessage((data: unknown) => {
          try {
            const signal = data as ArgusSignal
            // Validate required fields
            if (signal.signal_id && signal.trace_id && signal.layer && signal.timestamp) {
              store.addSignal(signal)
            }
          } catch (err) {
            console.error('Error processing signal:', err)
          }
        })

        // Setup error handler
        const unsubscribeError = client.onError((error: string) => {
          console.error('WebSocket error:', error)
          setWsConnected(false)
          if (error.includes('max retries')) {
            store.setError('Signal stream disconnected - attempting to reconnect...')
          } else {
            store.setError(error)
          }
        })

        // Store unsubscribe functions
        unsubscribeRef.current = { message: unsubscribeMessage, error: unsubscribeError }

        // Connect to WebSocket
        try {
          await client.connect()
          setWsConnected(true)
          store.setSubscribed(true)
          store.setError(null)
        } catch (connectError) {
          const message = connectError instanceof Error ? connectError.message : 'Failed to connect'
          store.setError(`WebSocket connection failed: ${message}`)
          setWsConnected(false)
        }
      } catch (error) {
        const message = error instanceof Error ? error.message : 'WebSocket setup failed'
        store.setError(message)
        setWsConnected(false)
      }
    }

    setupWebSocket()

    // Cleanup on unmount
    return () => {
      if (wsRef.current) {
        wsRef.current.disconnect()
        wsRef.current = null
      }
      unsubscribeRef.current.message()
      unsubscribeRef.current.error()
      store.setSubscribed(false)
    }
  }, [token, store, refetchSignals])

  return {
    signals: store.signals,
    isConnected: wsConnected,
    error: store.error,
    filter: store.filter,
    setFilter: store.updateFilter,
    refetchSignals,
    layerStatus
  }
}
