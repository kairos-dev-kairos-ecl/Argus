/**
 * Hook for fetching and paginating alerts (detection results).
 *
 * Manages:
 * - Alert fetching with pagination (limit/offset)
 * - Loading and error states
 * - Manual refetch capability
 * - Pagination controls (setLimit, setOffset)
 *
 * Usage:
 * const { alerts, total, loading, error, limit, offset, setLimit, setOffset, refetch } = useAlerts()
 */

import { useState, useEffect, useCallback } from 'react'
import type { Detection } from '../types'
import { ApiError } from '../services/types'
import * as api from '../services/api'

export interface UseAlertsState {
  alerts: Detection[]
  total: number
  loading: boolean
  error: ApiError | null
  limit: number
  offset: number
}

export interface UseAlertsResult extends UseAlertsState {
  setLimit: (limit: number) => void
  setOffset: (offset: number) => void
  refetch: () => Promise<void>
}

/**
 * Hook for fetching alerts with pagination management.
 *
 * @param initialLimit - Initial page size (default 50)
 * @returns Alerts data with pagination controls and refetch
 */
export function useAlerts(initialLimit: number = 50): UseAlertsResult {
  const [state, setState] = useState<UseAlertsState>({
    alerts: [],
    total: 0,
    loading: true,
    error: null,
    limit: initialLimit,
    offset: 0,
  })

  /**
   * Fetch alerts with current limit/offset.
   */
  const refetch = useCallback(async () => {
    setState((prev) => ({ ...prev, loading: true, error: null }))
    try {
      const result = await api.getAlerts(state.limit, state.offset)
      setState((prev) => ({
        ...prev,
        alerts: result.alerts,
        total: result.total,
        loading: false,
        error: null,
      }))
    } catch (err) {
      const error =
        err instanceof ApiError
          ? err
          : new ApiError(500, err, 'Failed to fetch alerts')
      setState((prev) => ({
        ...prev,
        alerts: [],
        loading: false,
        error,
      }))
    }
  }, [state.limit, state.offset])

  /**
   * Set page size and reset to first page.
   */
  const setLimit = useCallback((limit: number) => {
    setState((prev) => ({
      ...prev,
      limit,
      offset: 0, // Reset to first page
    }))
  }, [])

  /**
   * Set pagination offset (for next/previous page).
   */
  const setOffset = useCallback((offset: number) => {
    setState((prev) => ({
      ...prev,
      offset,
    }))
  }, [])

  /**
   * Refetch when limit or offset changes.
   */
  useEffect(() => {
    refetch()
  }, [state.limit, state.offset, refetch])

  return {
    ...state,
    setLimit,
    setOffset,
    refetch,
  }
}
