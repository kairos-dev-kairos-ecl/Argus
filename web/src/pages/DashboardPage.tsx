import { useSignalStream } from '../hooks/useSignalStream'
import { SignalStream } from '../components/SignalStream'
import { SignalStreamFilter } from '../components/SignalStreamFilter'
import { CoverageMap } from '../components/CoverageMap'

/**
 * DashboardPage Component
 *
 * Main dashboard view combining:
 * 1. Connection status banner (shows WebSocket state)
 * 2. Coverage Map (L1-L10 layer status grid)
 * 3. Signal Stream Filter (layer, severity, search)
 * 4. Signal Stream (virtual-scrolling live signal feed)
 *
 * Layout:
 * - Top: CoverageMap (responsive grid)
 * - Middle: SignalStreamFilter (filter controls)
 * - Bottom: SignalStream (flex-1 to fill remaining height)
 */
export const DashboardPage: React.FC = () => {
  const { isConnected, error } = useSignalStream()

  return (
    <div className="flex flex-col h-full gap-4 p-4 bg-background">
      {/* Connection status banner */}
      {!isConnected && (
        <div className="bg-status-error text-foreground px-4 py-2 rounded-lg border border-status-error">
          <div className="font-semibold">Disconnected from signal stream</div>
          <div className="text-sm opacity-90">Attempting to reconnect...</div>
          {error && <div className="text-xs opacity-75 mt-1">{error}</div>}
        </div>
      )}

      {isConnected && (
        <div className="bg-status-success text-foreground px-4 py-2 rounded-lg border border-status-success">
          <div className="font-semibold">Connected to signal stream</div>
          <div className="text-sm opacity-90">Real-time updates active</div>
        </div>
      )}

      {/* Top: Coverage Map */}
      <CoverageMap />

      {/* Bottom: Signal Stream with Filter */}
      <div className="flex flex-col flex-1 min-h-0">
        {/* Filter controls */}
        <SignalStreamFilter />

        {/* Virtual-scrolling signal list */}
        <SignalStream />
      </div>
    </div>
  )
}
