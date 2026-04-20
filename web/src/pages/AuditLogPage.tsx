import { useState } from 'react'
import { useAlerts } from '../hooks/useAlerts'
import { useAuthStore } from '../stores/auth'

/**
 * AuditLogPage / Detection Ledger
 *
 * Displays immutable record of all detection alerts triggered by the system.
 * Shows decision metadata: rule, signal, confidence, severity, timestamp.
 * Admins and analysts only.
 */
export function AuditLogPage() {
  const { isAdmin, isAnalyst } = useAuthStore()
  const [expandedAlertId, setExpandedAlertId] = useState<string | null>(null)

  // Fetch alerts with pagination
  const { alerts, total, loading, error, limit, offset, setLimit, setOffset, refetch } = useAlerts(50)

  // Access control
  if (!isAdmin() && !isAnalyst()) {
    return (
      <div className="min-h-screen bg-background flex items-center justify-center">
        <div className="text-center">
          <h1 className="text-2xl font-bold text-status-error mb-2">Access Denied</h1>
          <p className="text-muted-foreground">Only admins and analysts can access this page</p>
        </div>
      </div>
    )
  }

  // Calculate pagination info
  const totalPages = Math.ceil(total / limit)
  const currentPage = Math.floor(offset / limit) + 1
  const pageStart = offset + 1
  const pageEnd = Math.min(offset + limit, total)

  /**
   * Get color class for severity badge.
   * Severity scale: 1=info, 2=warning, 3=high, 4=critical, 5=emergency
   */
  const getSeverityClass = (severity: number): string => {
    switch (severity) {
      case 5:
        return 'bg-red-900/30 text-red-200 border-red-700' // Emergency: dark red
      case 4:
        return 'bg-orange-900/30 text-orange-200 border-orange-700' // Critical: dark orange
      case 3:
        return 'bg-yellow-900/30 text-yellow-200 border-yellow-700' // High: dark yellow
      case 2:
        return 'bg-blue-900/30 text-blue-200 border-blue-700' // Warning: dark blue
      case 1:
      default:
        return 'bg-green-900/30 text-green-200 border-green-700' // Info: dark green
    }
  }

  /**
   * Get severity label.
   */
  const getSeverityLabel = (severity: number): string => {
    switch (severity) {
      case 5:
        return 'Emergency'
      case 4:
        return 'Critical'
      case 3:
        return 'High'
      case 2:
        return 'Warning'
      case 1:
      default:
        return 'Info'
    }
  }

  /**
   * Format confidence as percentage.
   */
  const formatConfidence = (confidence: number): string => {
    return `${Math.round(confidence * 100)}%`
  }

  /**
   * Format timestamp to readable format.
   */
  const formatTime = (timestamp: string): string => {
    try {
      return new Date(timestamp).toLocaleString()
    } catch {
      return timestamp
    }
  }

  /**
   * Truncate ID to first 8 characters for display.
   */
  const truncateId = (id: string): string => {
    return id.substring(0, 8)
  }

  return (
    <div className="min-h-screen bg-background">
      {/* Header */}
      <div className="bg-muted-background border-b border-border p-6">
        <div className="flex items-center justify-between">
          <div>
            <h1 className="text-3xl font-bold text-foreground">Detection Ledger</h1>
            <p className="text-muted-foreground mt-1">Immutable record of all detection alerts</p>
          </div>
          <button
            onClick={() => refetch()}
            disabled={loading}
            className="px-4 py-2 bg-primary hover:bg-primary/90 text-foreground rounded-lg disabled:opacity-50"
          >
            {loading ? 'Loading...' : 'Refresh'}
          </button>
        </div>
      </div>

      {/* Content */}
      <div className="max-w-7xl mx-auto p-6">
        {/* Error State */}
        {error && (
          <div className="bg-red-900/20 border border-red-900/50 rounded-lg p-4 mb-6 flex items-start justify-between">
            <div>
              <p className="text-status-error font-medium">Failed to load alerts</p>
              <p className="text-red-200/80 text-sm mt-1">{error.message}</p>
            </div>
            <button
              onClick={() => refetch()}
              className="px-3 py-1 bg-red-900/40 hover:bg-red-900/60 text-red-200 rounded text-sm"
            >
              Retry
            </button>
          </div>
        )}

        {/* Alerts Table */}
        {loading && alerts.length === 0 ? (
          <div className="flex items-center justify-center py-12">
            <div className="text-center">
              <div className="inline-block animate-spin rounded-full h-8 w-8 border-2 border-primary border-t-transparent mb-4" />
              <p className="text-muted-foreground">Loading alerts...</p>
            </div>
          </div>
        ) : alerts.length === 0 ? (
          <div className="text-center py-12">
            <p className="text-muted-foreground">No alerts found</p>
          </div>
        ) : (
          <>
            {/* Alerts List */}
            <div className="space-y-3 mb-6">
              {alerts.map((alert) => (
                <div
                  key={alert.detection_id}
                  className="bg-muted-background rounded-lg border border-border overflow-hidden hover:border-border/80 transition"
                >
                  {/* Alert Row */}
                  <button
                    onClick={() =>
                      setExpandedAlertId(
                        expandedAlertId === alert.detection_id ? null : alert.detection_id
                      )
                    }
                    className="w-full px-6 py-4 flex items-center justify-between hover:bg-background/50 transition text-left"
                  >
                    {/* Left: ID and Rule */}
                    <div className="flex-1 min-w-0">
                      <div className="flex items-center gap-3">
                        {/* Severity Badge */}
                        <div
                          className={`px-3 py-1 rounded border ${getSeverityClass(
                            alert.severity
                          )} text-sm font-medium whitespace-nowrap`}
                        >
                          {getSeverityLabel(alert.severity)}
                        </div>

                        {/* Detection ID */}
                        <div className="font-mono text-foreground/80 text-sm">{truncateId(alert.detection_id)}</div>

                        {/* Rule Name */}
                        <div className="text-foreground font-medium truncate">{alert.rule_name}</div>
                      </div>

                      {/* Subtitle: Signal and Time */}
                      <div className="text-muted-foreground text-sm mt-2 ml-24">
                        <span className="font-mono">{truncateId(alert.signal_id)}</span>
                        <span className="mx-2">·</span>
                        <span>{formatTime(alert.matched_at)}</span>
                      </div>
                    </div>

                    {/* Right: Confidence and Toggle */}
                    <div className="flex items-center gap-4 ml-4">
                      <div className="text-right">
                        <div className="text-foreground font-medium">{formatConfidence(alert.confidence)}</div>
                        <div className="text-muted-foreground text-xs">confidence</div>
                      </div>

                      {/* Expand Toggle */}
                      <div
                        className={`w-5 h-5 flex items-center justify-center transition transform ${
                          expandedAlertId === alert.detection_id ? 'rotate-180' : ''
                        }`}
                      >
                        <svg
                          className="w-4 h-4 text-muted-foreground"
                          fill="none"
                          stroke="currentColor"
                          viewBox="0 0 24 24"
                        >
                          <path
                            strokeLinecap="round"
                            strokeLinejoin="round"
                            strokeWidth={2}
                            d="M19 14l-7 7m0 0l-7-7m7 7V3"
                          />
                        </svg>
                      </div>
                    </div>
                  </button>

                  {/* Expanded Details */}
                  {expandedAlertId === alert.detection_id && (
                    <div className="border-t border-border bg-background/50 px-6 py-4">
                      <div className="grid grid-cols-2 gap-6">
                        {/* Column 1: IDs and Rule */}
                        <div className="space-y-3">
                          <div>
                            <div className="text-muted-foreground text-xs uppercase tracking-wide">Detection ID</div>
                            <div className="font-mono text-foreground text-sm mt-1">{alert.detection_id}</div>
                          </div>
                          <div>
                            <div className="text-muted-foreground text-xs uppercase tracking-wide">Rule ID</div>
                            <div className="font-mono text-foreground text-sm mt-1">{alert.rule_id}</div>
                          </div>
                          <div>
                            <div className="text-muted-foreground text-xs uppercase tracking-wide">Signal ID</div>
                            <div className="font-mono text-foreground text-sm mt-1">{alert.signal_id}</div>
                          </div>
                        </div>

                        {/* Column 2: Metrics */}
                        <div className="space-y-3">
                          <div>
                            <div className="text-muted-foreground text-xs uppercase tracking-wide">Confidence</div>
                            <div className="text-foreground text-sm mt-1">{formatConfidence(alert.confidence)}</div>
                          </div>
                          <div>
                            <div className="text-muted-foreground text-xs uppercase tracking-wide">Severity</div>
                            <div className="text-foreground text-sm mt-1">{getSeverityLabel(alert.severity)}</div>
                          </div>
                          <div>
                            <div className="text-muted-foreground text-xs uppercase tracking-wide">Matched At</div>
                            <div className="text-foreground text-sm mt-1">{formatTime(alert.matched_at)}</div>
                          </div>
                        </div>
                      </div>

                      {/* Full Alert JSON (optional debug) */}
                      <div className="mt-4 pt-4 border-t border-border/50">
                        <details className="group">
                          <summary className="cursor-pointer text-muted-foreground text-xs uppercase tracking-wide hover:text-foreground transition">
                            Raw Detection Payload
                          </summary>
                          <div className="mt-3 bg-background rounded border border-border p-3 font-mono text-xs text-foreground/70 overflow-x-auto">
                            <pre>{JSON.stringify(alert, null, 2)}</pre>
                          </div>
                        </details>
                      </div>
                    </div>
                  )}
                </div>
              ))}
            </div>

            {/* Pagination Controls */}
            <div className="flex items-center justify-between bg-muted-background rounded-lg border border-border p-4">
              <div className="text-muted-foreground text-sm">
                Showing <span className="font-medium text-foreground">{pageStart}</span> to{' '}
                <span className="font-medium text-foreground">{pageEnd}</span> of{' '}
                <span className="font-medium text-foreground">{total}</span> alerts
              </div>

              <div className="flex items-center gap-4">
                {/* Page Size Selector */}
                <div className="flex items-center gap-2">
                  <label htmlFor="pageSize" className="text-muted-foreground text-sm">
                    Per page:
                  </label>
                  <select
                    id="pageSize"
                    value={limit}
                    onChange={(e) => setLimit(parseInt(e.target.value))}
                    className="px-3 py-1 rounded bg-background border border-border text-foreground text-sm"
                  >
                    <option value={10}>10</option>
                    <option value={25}>25</option>
                    <option value={50}>50</option>
                    <option value={100}>100</option>
                  </select>
                </div>

                {/* Navigation Buttons */}
                <div className="flex items-center gap-2">
                  <button
                    onClick={() => setOffset(Math.max(0, offset - limit))}
                    disabled={offset === 0}
                    className="px-3 py-1 rounded bg-primary hover:bg-primary/90 text-foreground text-sm disabled:opacity-50 disabled:cursor-not-allowed"
                  >
                    Previous
                  </button>

                  <span className="text-muted-foreground text-sm">
                    Page {currentPage} of {totalPages}
                  </span>

                  <button
                    onClick={() => setOffset(offset + limit)}
                    disabled={currentPage >= totalPages}
                    className="px-3 py-1 rounded bg-primary hover:bg-primary/90 text-foreground text-sm disabled:opacity-50 disabled:cursor-not-allowed"
                  >
                    Next
                  </button>
                </div>
              </div>
            </div>
          </>
        )}
      </div>
    </div>
  )
}
