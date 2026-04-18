import { useState, useCallback } from 'react'
import axios from 'axios'
import { apiClient } from '../lib/axios-client'
import type { QueryResult } from '../types/index'

/**
 * Hook for managing SQL query execution against ClickHouse.
 * Handles query state, loading, error, and result caching.
 */
export function useQuery() {
  const [sql, setSql] = useState('')
  const [result, setResult] = useState<QueryResult | null>(null)
  const [loading, setLoading] = useState(false)
  const [error, setError] = useState<string | null>(null)

  const execute = useCallback(async () => {
    if (!sql.trim()) {
      setError('SQL query cannot be empty')
      return
    }

    try {
      setLoading(true)
      setError(null)

      // POST /api/v1/query with SQL and limit
      const response = await apiClient.post(
        '/query',
        {
          sql: sql.trim(),
          limit: 10000
        },
        {
          timeout: 30000 // 30 second timeout
        }
      )

      setResult(response.data as QueryResult)
    } catch (err: unknown) {
      if (axios.isAxiosError(err)) {
        if (err.response?.status === 400) {
          const errorMsg = (err.response.data as any)?.error || 'Invalid query'
          setError(`Query error: ${errorMsg}`)
        } else if (err.response?.status === 500) {
          setError('Server error while executing query')
        } else if (err.code === 'ECONNABORTED') {
          setError('Query timeout (>30s)')
        } else {
          setError(`Request failed: ${err.message}`)
        }
      } else {
        setError(`Execution failed: ${err instanceof Error ? err.message : 'Unknown error'}`)
      }
      setResult(null)
    } finally {
      setLoading(false)
    }
  }, [sql])

  const reset = useCallback(() => {
    setSql('')
    setResult(null)
    setError(null)
  }, [])

  return { sql, setSql, result, loading, error, execute, reset }
}
