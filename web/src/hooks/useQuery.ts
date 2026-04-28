import { useState, useCallback } from 'react'
import type { QueryResult } from '../types/index'
import { ApiError } from '../services/types'
import * as api from '../services/api'

/**
 * Hook for managing SQL query execution against ClickHouse.
 * Integrates with the API service layer for proper authentication and error handling.
 *
 * Usage:
 * const { sql, setSql, result, loading, error, execute, reset } = useQuery()
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

      // Call API service executeQuery function
      const queryResult = await api.executeQuery({
        query: sql.trim(),
      })

      setResult(queryResult)
    } catch (err: unknown) {
      let errorMessage = 'Query execution failed'

      if (err instanceof ApiError) {
        if (err.status === 400) {
          errorMessage = `Query error: ${err.message}`
        } else if (err.status === 401) {
          errorMessage = 'Session expired - please login again'
        } else if (err.status === 403) {
          errorMessage = 'Permission denied - cannot execute query'
        } else if (err.status === 408) {
          errorMessage = 'Query timeout - execution took too long'
        } else if (err.status === 500) {
          errorMessage = 'Server error while executing query'
        } else {
          errorMessage = err.message || 'Query execution failed'
        }
      } else if (err instanceof Error) {
        errorMessage = err.message
      }

      setError(errorMessage)
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
