import { useLayerStore } from '../stores/layer'
import { useCoverageMap } from '../hooks/useCoverageMap'
import { LayerStatusCell } from './LayerStatusCell'

/**
 * CoverageMap Component
 *
 * Displays the health status of all 10 layers (L1-L10) in a grid.
 * Each cell shows:
 * - Layer name and status color (green/yellow/gray/red)
 * - Last signal timestamp
 * - Signal count in last 5 minutes
 * - Error messages if present
 *
 * Updates layer status every 30 seconds via the useCoverageMap hook.
 *
 * Layout: 5 columns × 2 rows to display all 10 layers
 */
export const CoverageMap: React.FC = () => {
  const { statuses } = useLayerStore()
  useCoverageMap() // Initialize polling

  // Sort layers for consistent display order
  const sortedLayers = Object.entries(statuses).sort(([a], [b]) => a.localeCompare(b))

  return (
    <div className="p-4 bg-muted-background rounded-lg border border-border">
      {/* Header */}
      <h2 className="text-lg font-bold text-foreground mb-4">Coverage Map (10 Layers)</h2>

      {/* Grid of layer status cells - responsive: 1 col (mobile) → 2 col (sm) → 3 col (md) → 5 col (lg+) */}
      <div className="grid grid-cols-1 sm:grid-cols-2 md:grid-cols-3 lg:grid-cols-5 gap-4">
        {sortedLayers.map(([layer, status]) => (
          <LayerStatusCell
            key={layer}
            layer={layer as Parameters<typeof LayerStatusCell>[0]['layer']}
            status={status}
          />
        ))}
      </div>

      {/* Footer with refresh note */}
      <div className="mt-4 text-xs text-muted-foreground">
        Status updates every 30 seconds via API polling.
      </div>
    </div>
  )
}
