import { useEffect } from 'react'
import { useLayerStore } from '../stores/layer'
import type { Layer, LayerStatus } from '../types'

/**
 * useCoverageMap Hook
 *
 * Polls layer status every 30 seconds from the backend API.
 * Updates the Zustand layer store with fresh status for each layer.
 *
 * @returns Object with fetchLayerStatus function (can be called manually if needed)
 */
export function useCoverageMap() {
  const store = useLayerStore()

  const fetchLayerStatus = async () => {
    try {
      const response = await fetch('/api/v1/layers/status')
      if (!response.ok) {
        throw new Error(`API error: ${response.statusText}`)
      }
      const data = await response.json() as Record<string, LayerStatus>

      // Update store with each layer's status
      Object.entries(data).forEach(([layer, status]) => {
        store.updateStatus(layer as Layer, status)
      })
    } catch (error) {
      console.error('Failed to fetch layer status:', error)
    }
  }

  useEffect(() => {
    // TODO: Layer status endpoint not yet implemented on backend
    // When backend implements /api/v1/layers/status, enable polling:

    /*
    // Initial load on component mount
    fetchLayerStatus()

    // Poll every 30 seconds
    const interval = setInterval(() => {
      fetchLayerStatus()
    }, 30000)

    return () => {
      clearInterval(interval)
    }
    */

    // For now, just initialize with default statuses
    // This prevents 404 errors while waiting for backend implementation
  }, [])

  return { fetchLayerStatus }
}
