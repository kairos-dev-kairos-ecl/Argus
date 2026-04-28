import { LayerStatusCell } from './LayerStatusCell'
import type { LayerStatus } from '../types'

interface CoverageMapProps {
  layerStatus: LayerStatus[]
}

/**
 * CoverageMap Component
 *
 * Displays the health status of all 10 layers (L1-L10) in a grid.
 * Receives layerStatus as a prop from DashboardPage (computed by useSignalStream)
 * rather than reading from the layerStore — avoids the update loop caused by
 * syncing derived data back into a store during render.
 */
export const CoverageMap: React.FC<CoverageMapProps> = ({ layerStatus }) => {
  // Sort layers for consistent display order
  const sortedLayers = [...layerStatus].sort((a, b) => a.layer.localeCompare(b.layer))

  return (
    <div className="p-4 bg-muted-background rounded-lg border border-border">
      {/* Header */}
      <h2 className="text-lg font-bold text-foreground mb-4">Coverage Map (10 Layers)</h2>

      {/* Grid of layer status cells - responsive: 1 col (mobile) → 2 col (sm) → 3 col (md) → 5 col (lg+) */}
      <div className="grid grid-cols-1 sm:grid-cols-2 md:grid-cols-3 lg:grid-cols-5 gap-4">
        {sortedLayers.map((status) => (
          <LayerStatusCell
            key={status.layer}
            layer={status.layer}
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
