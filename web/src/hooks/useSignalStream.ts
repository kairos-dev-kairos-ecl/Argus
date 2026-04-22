import { useEffect, useRef, useCallback, useState, useMemo } from 'react'
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
 * Key design decisions:
 * - Individual Zustand selectors (not whole store object) to avoid unstable
 *   references that cause useCallback/useEffect to re-run on every state change.
 * - useEffect deps reduced to [token] only — WebSocket reconnects solely when
 *   the auth token changes, not on every signal update.
 * - Zustand actions are referentially stable across renders, so they are safe
 *   in useCallback deps without causing infinite loops.
 */
export function useSignalStream() {
  // State selectors — re-render when these values change
  const signals    = useSignalStore(state => state.signals)
  const error      = useSignalStore(state => state.error)
  const filter     = useSignalStore(state => state.filter)

  // Action selectors — Zustand guarantees these are stable references
  const addSignal      = useSignalStore(state => state.addSignal)
  const setError       = useSignalStore(state => state.setError)
  const setLoading     = useSignalStore(state => state.setLoading)
  const clearSignals   = useSignalStore(state => state.clearSignals)
  const setSubscribed  = useSignalStore(state => state.setSubscribed)
  const updateFilter   = useSignalStore(state => state.updateFilter)

  const { token } = useAuthStore()
  const wsRef = useRef<WebSocketClient | null>(null)
  const unsubscribeRef = useRef<{ message: () => void; error: () => void }>({
    message: () => {},
    error: () => {},
  })
  const [wsConnected, setWsConnected] = useState(false)

  /**
   * Compute layer status from signals in the last 5 minutes.
   * Wrapped in useMemo so the array reference is stable between renders —
   * only produces a new reference when `signals` actually changes.
   * Without this, every render creates a new array, which causes DashboardPage's
   * useEffect([layerStatus]) to fire every render → updateStatus → re-render → loop.
   */
  const layerStatus: LayerStatus[] = useMemo(() => LAYERS.map((layer: Layer, index: number) => {
    const layerNum = index + 1
    const now = Date.now()
    const fiveMinutesAgo = now - 5 * 60 * 1000

    const recentSignals = signals.filter((s) => {
      const signalTime =
        typeof s.timestamp === 'string'
          ? new Date(s.timestamp).getTime()
          : s.timestamp.seconds * 1000 + ((s.timestamp as any).nanos ?? 0) / 1_000_000
      return s.layer === layerNum && signalTime > fiveMinutesAgo
    })

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

    const lastSignal = recentSignals[0]
    const lastSignalTime = lastSignal
      ? typeof lastSignal.timestamp === 'string'
        ? lastSignal.timestamp
        : new Date(
            lastSignal.timestamp.seconds * 1000 +
            ((lastSignal.timestamp as any).nanos ?? 0) / 1_000_000
          ).toISOString()
      : null

    return {
      layer,
      status,
      signal_count_5min: recentSignals.length,
      last_signal_time: lastSignalTime,
    }
  }), [signals])

  /**
   * Fetch initial signals from REST API.
   * Deps are individual stable Zustand actions — created once, never recreated.
   */
  const refetchSignals = useCallback(async () => {
    try {
      setLoading(true)
      const response = await getSignals({ limit: 100, app_id: 'test' })
      if (response && Array.isArray(response.signals)) {
        clearSignals()
        response.signals.forEach((signal) => addSignal(signal))
      }
      setError(null)
    } catch (err) {
      const message = err instanceof Error ? err.message : 'Failed to fetch signals'
      setError(message)
    } finally {
      setLoading(false)
    }
  }, [addSignal, clearSignals, setError, setLoading])

  /**
   * Initialize WebSocket on mount / token change only.
   *
   * Deps: [token] — reconnect only when auth changes.
   * Signal state mutations (addSignal, setError, etc.) deliberately excluded:
   * adding a signal must not tear down and recreate the WebSocket connection.
   */
  useEffect(() => {
    refetchSignals()

    if (!token) {
      setError('Not authenticated — WebSocket requires a valid token')
      return
    }

    const setupWebSocket = async () => {
      try {
        // Relative path — Vite proxies /ws → ws://localhost:8082 in dev.
        // In production Go serves everything on the same origin, so /ws/signals
        // resolves correctly with no CORS or cookie issues.
        const protocol = location.protocol === 'https:' ? 'wss' : 'ws'
        const wsUrl = `${protocol}://${location.host}/ws/signals`

        const client = createWebSocketClient(wsUrl)
        wsRef.current = client

        const unsubMessage = client.onMessage((data: unknown) => {
          try {
            const signal = data as ArgusSignal
            if (signal.signal_id && signal.trace_id && signal.layer && signal.timestamp) {
              addSignal(signal)
            }
          } catch (err) {
            console.error('Error processing signal:', err)
          }
        })

        const unsubError = client.onError((errMsg: string) => {
          console.error('WebSocket error:', errMsg)
          setWsConnected(false)
          if (errMsg.includes('max retries')) {
            setError('Signal stream disconnected — attempting to reconnect...')
          } else {
            setError(errMsg)
          }
        })

        unsubscribeRef.current = { message: unsubMessage, error: unsubError }

        try {
          await client.connect()
          setWsConnected(true)
          setSubscribed(true)
          setError(null)
        } catch (connectErr) {
          const message =
            connectErr instanceof Error ? connectErr.message : 'Failed to connect'
          setError(`WebSocket connection failed: ${message}`)
          setWsConnected(false)
        }
      } catch (err) {
        const message = err instanceof Error ? err.message : 'WebSocket setup failed'
        setError(message)
        setWsConnected(false)
      }
    }

    setupWebSocket()

    return () => {
      if (wsRef.current) {
        wsRef.current.disconnect()
        wsRef.current = null
      }
      unsubscribeRef.current.message()
      unsubscribeRef.current.error()
      setSubscribed(false)
      setWsConnected(false)
    }
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [token]) // Only reconnect when auth token changes

  return {
    signals,
    isConnected: wsConnected,
    error,
    filter,
    setFilter: updateFilter,
    refetchSignals,
    layerStatus,
  }
}
