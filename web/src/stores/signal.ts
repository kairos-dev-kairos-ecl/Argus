import { create } from 'zustand'
import type { ArgusSignal, SignalFilter } from '../types'

interface SignalState {
  signals: ArgusSignal[]
  filter: SignalFilter
  loading: boolean
  error: string | null
  subscribed: boolean
  max_buffer_size: number

  // Actions
  addSignal(signal: ArgusSignal): void
  removeOldestSignal(): void
  updateFilter(filter: Partial<SignalFilter>): void
  setLoading(loading: boolean): void
  setError(error: string | null): void
  setSubscribed(subscribed: boolean): void
  clearSignals(): void
}

/**
 * Signal Stream Store
 * Manages the buffer of incoming signals with max 500-signal capacity.
 * When buffer overflows, oldest signals are dropped.
 */
export const useSignalStore = create<SignalState>((set) => ({
  signals: [],
  filter: {},
  loading: false,
  error: null,
  subscribed: false,
  max_buffer_size: 500,

  addSignal: (signal: ArgusSignal) =>
    set((state) => {
      const updated = [signal, ...state.signals]
      // Drop oldest signals if exceeding max buffer size
      if (updated.length > state.max_buffer_size) {
        return {
          signals: updated.slice(0, state.max_buffer_size)
        }
      }
      return { signals: updated }
    }),

  removeOldestSignal: () =>
    set((state) => ({
      signals: state.signals.slice(0, -1)
    })),

  updateFilter: (filter: Partial<SignalFilter>) =>
    set((state) => ({
      filter: { ...state.filter, ...filter }
    })),

  setLoading: (loading: boolean) => set({ loading }),

  setError: (error: string | null) => set({ error }),

  setSubscribed: (subscribed: boolean) => set({ subscribed }),

  clearSignals: () => set({ signals: [] })
}))
