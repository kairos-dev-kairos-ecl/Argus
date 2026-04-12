import type { Layer, LayerStatus } from '../types'

interface LayerStatusCellProps {
  layer: Layer
  status: LayerStatus
}

/**
 * LayerStatusCell Component
 *
 * Displays a single layer's status in the coverage map.
 * Shows:
 * - Layer name
 * - Status color (green/yellow/gray/red)
 * - Last signal time (relative, e.g., "2 minutes ago")
 * - Signal count in the last 5 minutes
 * - Error message (if status is red)
 *
 * @param layer - Layer identifier (L1-L10)
 * @param status - Layer status object with color, timing, and counts
 */
export const LayerStatusCell: React.FC<LayerStatusCellProps> = ({ layer, status }) => {
  const getStatusColor = (statusStr: string): string => {
    switch (statusStr) {
      case 'green':
        return 'bg-status-success hover:bg-status-success/90'
      case 'yellow':
        return 'bg-status-warning hover:bg-status-warning/90'
      case 'red':
        return 'bg-status-error hover:bg-status-error/90'
      case 'gray':
      default:
        return 'bg-muted-foreground hover:bg-muted-foreground/80'
    }
  }

  const formatTimeAgo = (timestamp: string | null): string => {
    if (!timestamp) return 'No signals'

    try {
      const date = new Date(timestamp)
      const now = new Date()
      const seconds = Math.floor((now.getTime() - date.getTime()) / 1000)

      if (seconds < 60) return `${seconds}s ago`
      if (seconds < 3600) return `${Math.floor(seconds / 60)}m ago`
      if (seconds < 86400) return `${Math.floor(seconds / 3600)}h ago`
      return `${Math.floor(seconds / 86400)}d ago`
    } catch {
      return 'Invalid date'
    }
  }

  return (
    <div
      className={`${getStatusColor(status.status)} p-4 rounded-lg text-foreground transition-colors cursor-pointer`}
      role="button"
      tabIndex={0}
      aria-label={`${layer} status: ${status.status}`}
    >
      {/* Layer name */}
      <div className="font-bold text-sm text-foreground">{layer}</div>

      {/* Last signal time */}
      <div className="text-xs text-foreground/80 mt-2 font-mono">
        {formatTimeAgo(status.last_signal_time)}
      </div>

      {/* Signal count in last 5 minutes */}
      <div className="text-xs text-foreground/80">
        {status.signal_count_5min} signals (5m)
      </div>

      {/* Error message if present */}
      {status.error_message && (
        <div className="text-xs text-foreground/70 mt-2 break-words">
          {status.error_message}
        </div>
      )}
    </div>
  )
}
