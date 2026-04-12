import { useState, useEffect } from 'react'
import axios from 'axios'
import type { Trace } from '../types/index'

/**
 * Hook for fetching and managing trace data by trace_id.
 * Handles loading, error states, and span selection.
 */
export function useTrace(traceID: string) {
  const [trace, setTrace] = useState<Trace | null>(null)
  const [loading, setLoading] = useState(true)
  const [error, setError] = useState<string | null>(null)
  const [selectedSpanID, setSelectedSpanID] = useState<string | null>(null)

  useEffect(() => {
    const fetchTrace = async () => {
      try {
        setLoading(true)
        setError(null)
        // GET /api/v1/traces/{trace_id}
        const response = await axios.get(`/api/v1/traces/${traceID}`)
        setTrace(response.data)
      } catch (err) {
        const message = err instanceof Error ? err.message : 'Failed to fetch trace'
        setError(message)
        setTrace(null)
      } finally {
        setLoading(false)
      }
    }

    if (traceID && traceID.trim()) {
      fetchTrace()
    } else {
      setLoading(false)
      setTrace(null)
    }
  }, [traceID])

  return { trace, loading, error, selectedSpanID, setSelectedSpanID }
}
