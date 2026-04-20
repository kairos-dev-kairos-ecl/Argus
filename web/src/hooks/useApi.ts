/**
 * React Hooks for API Calls
 *
 * Generic useApi hook for handling async operations with loading/error states.
 * Specific data-fetching hooks for common API operations.
 *
 * Architecture:
 * - useApi: Generic hook for any Promise-returning function
 * - Specific hooks (useSignalsQuery, etc.): Convenience wrappers
 * - Error handling: Never throws, returns error state
 * - Dependencies: Array of values that trigger refetch
 */

import { useState, useEffect, useCallback } from 'react'
import type {
  ArgusSignal,
  LayerStatus,
  Trace,
  Detection,
  QueryResult,
} from '../types'
import { ApiError } from '../services/types'
import * as api from '../services/api'

// ============================================================================
// GENERIC useApi HOOK
// ============================================================================

export interface UseApiState<T> {
  data: T | null
  loading: boolean
  error: ApiError | null
}

export interface UseApiResult<T> extends UseApiState<T> {
  refetch: () => Promise<void>
}

/**
 * Generic hook for async API calls with loading and error states.
 *
 * Usage:
 * const { data, loading, error, refetch } = useApi(
 *   () => getSignals({ limit: 10 }),
 *   [apiStore] // dependencies
 * )
 *
 * @param fn - Async function returning T
 * @param deps - Dependencies array (refetch when any dep changes)
 * @returns State object with data, loading, error, and refetch function
 */
export function useApi<T>(
  fn: () => Promise<T>,
  deps: unknown[] = []
): UseApiResult<T> {
  const [state, setState] = useState<UseApiState<T>>({
    data: null,
    loading: true,
    error: null,
  })

  const refetch = useCallback(async () => {
    setState((prev) => ({ ...prev, loading: true, error: null }))
    try {
      const result = await fn()
      setState({
        data: result,
        loading: false,
        error: null,
      })
    } catch (err) {
      const error =
        err instanceof ApiError
          ? err
          : new ApiError(500, err, 'Unexpected error')
      setState({
        data: null,
        loading: false,
        error,
      })
    }
  }, [fn])

  useEffect(() => {
    refetch()
  }, deps)

  return { ...state, refetch }
}

// ============================================================================
// SIGNALS QUERY HOOK
// ============================================================================

/**
 * Hook for fetching signals with filters.
 *
 * @param limit - Page size
 * @param cursor - Pagination cursor for next page
 * @returns Signals array with loading/error states
 */
export function useSignalsQuery(
  limit: number = 100,
  cursor?: string
): UseApiResult<ArgusSignal[]> {
  const result = useApi(
    () =>
      api
        .getSignals({
          limit,
          cursor,
        })
        .then((res) => res.signals),
    [limit, cursor]
  )

  return result
}

// ============================================================================
// LAYER STATUS HOOK
// ============================================================================

/**
 * Hook for fetching layer status across all 10 layers.
 *
 * @returns Array of LayerStatus objects with health indicators
 */
export function useLayerStatusQuery(): UseApiResult<LayerStatus[]> {
  const result = useApi(
    () =>
      api.getLayerStatus().then((res) => res.layers),
    []
  )

  return result
}

// ============================================================================
// TRACE QUERY HOOK
// ============================================================================

/**
 * Hook for fetching trace data with spans and detections.
 *
 * @param traceId - Trace identifier
 * @returns Trace object with spans and detections
 */
export function useTraceQuery(traceId?: string): UseApiResult<Trace> {
  const result = useApi(
    async () => {
      if (!traceId) {
        throw new ApiError(400, null, 'Trace ID is required')
      }
      return api.getTrace(traceId)
    },
    [traceId]
  )

  return result
}

// ============================================================================
// ALERTS QUERY HOOK
// ============================================================================

/**
 * Hook for fetching alerts (detections) with pagination.
 *
 * @param limit - Page size
 * @param offset - Pagination offset
 * @returns Array of Detection objects
 */
export function useAlertsQuery(
  limit: number = 50,
  offset: number = 0
): UseApiResult<Detection[]> {
  const result = useApi(
    () =>
      api
        .getAlerts(limit, offset)
        .then((res) => res.alerts),
    [limit, offset]
  )

  return result
}

// ============================================================================
// QUERY EXECUTION HOOK
// ============================================================================

/**
 * Hook for executing ClickHouse queries with results.
 *
 * Unlike other hooks, this one doesn't fetch on mount.
 * Call the execute() function directly when user submits query.
 *
 * @returns Query results object with execute() function
 */
export function useQueryExecution(): UseApiResult<QueryResult> & {
  execute: (query: string) => Promise<void>
} {
  const [state, setState] = useState<UseApiState<QueryResult>>({
    data: null,
    loading: false,
    error: null,
  })

  const execute = useCallback(async (query: string) => {
    setState({ data: null, loading: true, error: null })
    try {
      const result = await api.executeQuery({ query })
      setState({
        data: result,
        loading: false,
        error: null,
      })
    } catch (err) {
      const error =
        err instanceof ApiError
          ? err
          : new ApiError(500, err, 'Query execution failed')
      setState({
        data: null,
        loading: false,
        error,
      })
    }
  }, [])

  return { ...state, refetch: () => Promise.resolve(), execute }
}

// ============================================================================
// AGGREGATIONS HOOK
// ============================================================================

/**
 * Hook for fetching aggregated signal counts.
 *
 * @param type - Aggregation type ('by-layer' or 'by-model')
 * @returns Object mapping labels to counts
 */
export function useAggregationsQuery(
  type: 'by-layer' | 'by-model' = 'by-layer'
): UseApiResult<Record<string, number>> {
  const result = useApi(
    () => api.getAggregations(type),
    [type]
  )

  return result
}

// ============================================================================
// HEALTH CHECK HOOK
// ============================================================================

/**
 * Hook for checking backend connectivity.
 * Useful for showing connection status banner.
 *
 * @returns Boolean indicating if backend is healthy
 */
export function useHealthCheck(): UseApiResult<boolean> {
  const result = useApi(() => api.healthCheck(), [])

  return result
}
