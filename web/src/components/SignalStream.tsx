import { useRef, useMemo } from 'react'
import { useVirtualizer } from '@tanstack/react-virtual'
import { useSignalStore } from '../stores/signal'
import { SignalRow } from './SignalRow'
import type { ArgusSignal } from '../types'

/**
 * SignalStream Component
 *
 * Displays a real-time stream of signals with virtual scrolling.
 * Supports filtering by layer, severity, category, and search text.
 * Renders 1000+ rows without performance degradation using TanStack React Virtual.
 *
 * Virtual scrolling configuration:
 * - ~50px per row
 * - 10-row overscan for smooth scrolling
 * - Dynamic height based on filtered signal count
 */
export const SignalStream: React.FC = () => {
  const { signals, filter } = useSignalStore()
  const containerRef = useRef<HTMLDivElement>(null)

  // Filter signals based on current filter criteria
  const filtered: ArgusSignal[] = useMemo(() => {
    return signals.filter((s) => {
      // Filter by layers (layer is now a number 1-10)
      if (filter.layers && filter.layers.length > 0) {
        // Convert numeric layer to string for comparison
        if (!filter.layers.some(l => l.startsWith(`L${s.layer}`))) return false
      }

      // Filter by minimum severity
      if (filter.severity_min !== undefined) {
        if (s.severity < filter.severity_min) return false
      }

      // Filter by category
      if (filter.category && filter.category !== '') {
        if (s.category !== filter.category) return false
      }

      // Filter by search text in category or message
      if (filter.search && filter.search !== '') {
        const searchLower = filter.search.toLowerCase()
        const messageMatch = s.message?.toLowerCase().includes(searchLower) ?? false
        const categoryMatch = s.category.toLowerCase().includes(searchLower)
        if (!messageMatch && !categoryMatch) {
          return false
        }
      }

      return true
    })
  }, [signals, filter])

  // Setup virtual scroller
  const virtualizer = useVirtualizer({
    count: filtered.length,
    getScrollElement: () => containerRef.current,
    estimateSize: () => 50, // ~50px per row
    overscan: 10 // Render 10 rows outside visible area for smooth scrolling
  })

  return (
    <div
      ref={containerRef}
      className="flex-1 overflow-y-auto border border-border bg-background relative"
    >
      {/* Empty state */}
      {filtered.length === 0 && (
        <div className="flex items-center justify-center h-32 text-slate-400">
          <span>No signals match the current filter.</span>
        </div>
      )}

      {/* Virtual container */}
      {filtered.length > 0 && (
        <div style={{ height: `${virtualizer.getTotalSize()}px`, position: 'relative' }}>
          {virtualizer.getVirtualItems().map((virtualItem) => (
            <SignalRow
              key={filtered[virtualItem.index]?.signal_id || virtualItem.index}
              signal={filtered[virtualItem.index]!}
              style={{
                transform: `translateY(${virtualItem.start}px)`
              }}
            />
          ))}
        </div>
      )}
    </div>
  )
}
