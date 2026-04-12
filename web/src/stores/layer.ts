import { create } from 'zustand'
import type { Layer, LayerStatus } from '../types'
import { LAYERS } from '../types'

interface LayerStatusState {
  statuses: Record<Layer, LayerStatus>
  updateStatus(layer: Layer, status: Partial<LayerStatus>): void
  getOverallHealth(): 'green' | 'yellow' | 'red'
  getStatusForLayer(layer: Layer): LayerStatus | undefined
  initializeStatuses(): void
}

/**
 * Layer Status Store
 * Manages the status of all 10 layers (L1-L10) for the coverage map.
 * Tracks signal freshness, count, and overall system health.
 */
const initializeEmptyStatuses = (): Record<Layer, LayerStatus> => {
  const statuses = {} as Record<Layer, LayerStatus>
  LAYERS.forEach((layer) => {
    statuses[layer] = {
      layer,
      status: 'gray',
      last_signal_time: null,
      signal_count_5min: 0
    }
  })
  return statuses
}

export const useLayerStore = create<LayerStatusState>((set, get) => ({
  statuses: initializeEmptyStatuses(),

  updateStatus: (layer: Layer, statusUpdate: Partial<LayerStatus>) =>
    set((state) => {
      const existing = state.statuses[layer]
      return {
        statuses: {
          ...state.statuses,
          [layer]: {
            layer,
            status: existing?.status ?? 'gray',
            last_signal_time: existing?.last_signal_time ?? null,
            signal_count_5min: existing?.signal_count_5min ?? 0,
            ...statusUpdate
          }
        }
      }
    }),

  getOverallHealth: (): 'green' | 'yellow' | 'red' => {
    const { statuses } = get()
    const statusValues = Object.values(statuses)

    if (statusValues.length === 0) return 'gray' as never
    if (statusValues.some((s) => s.status === 'red')) return 'red'
    if (statusValues.some((s) => s.status === 'yellow')) return 'yellow'
    return 'green'
  },

  getStatusForLayer: (layer: Layer) => get().statuses[layer],

  initializeStatuses: () => {
    const statuses: Record<Layer, LayerStatus> = {} as Record<Layer, LayerStatus>
    LAYERS.forEach((layer) => {
      statuses[layer] = {
        layer,
        status: 'gray',
        last_signal_time: null,
        signal_count_5min: 0
      }
    })
    set({ statuses })
  }
}))
