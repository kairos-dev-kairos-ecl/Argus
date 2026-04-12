import React, { useState } from 'react'
import { useQuery } from '@tanstack/react-query'
import axios from 'axios'

interface Alert {
  id: string
  rule_name: string
  severity: 'critical' | 'high' | 'medium' | 'low'
  timestamp: string
  status: 'delivered' | 'acknowledged' | 'suppressed'
  signal_id: string
  signal_summary: string
  suppression_rules: string[]
}

/**
 * AlertsPage Component
 * View and manage detection alerts
 */
export const AlertsPage: React.FC = () => {
  const [ruleFilter, setRuleFilter] = useState<string>('all')
  const [severityFilter, setSeverityFilter] = useState<string>('all')
  const [statusFilter, setStatusFilter] = useState<string>('all')
  const [selectedAlert, setSelectedAlert] = useState<Alert | null>(null)

  const { data: alerts = [], isLoading } = useQuery({
    queryKey: ['alerts', ruleFilter, severityFilter, statusFilter],
    queryFn: async () => {
      const params: any = {}
      if (ruleFilter !== 'all') params.rule_name = ruleFilter
      if (severityFilter !== 'all') params.severity = severityFilter
      if (statusFilter !== 'all') params.status = statusFilter
      const res = await axios.get('/api/v1/alerts', { params })
      return res.data.alerts || []
    },
    staleTime: 30000,
  })

  const { data: rules = [] } = useQuery({
    queryKey: ['rules-list'],
    queryFn: async () => {
      const res = await axios.get('/api/v1/rules?limit=100')
      return res.data.rules || []
    },
    staleTime: 60000,
  })

  const getSeverityColor = (severity: string): string => {
    switch (severity) {
      case 'critical':
        return 'bg-status-error/20 text-status-error'
      case 'high':
        return 'bg-status-warning/20 text-status-warning'
      case 'medium':
        return 'bg-status-info/20 text-status-info'
      case 'low':
        return 'bg-status-success/20 text-status-success'
      default:
        return 'bg-border text-muted-foreground'
    }
  }

  const getStatusColor = (status: string): string => {
    switch (status) {
      case 'delivered':
        return 'text-status-info'
      case 'acknowledged':
        return 'text-status-warning'
      case 'suppressed':
        return 'text-muted-foreground'
      default:
        return 'text-muted-foreground'
    }
  }

  return (
    <div className="space-y-6">
      {/* Header */}
      <div>
        <h1 className="text-3xl font-bold text-foreground">Alerts</h1>
        <p className="text-muted-foreground mt-1">Detection alerts from active rules</p>
      </div>

      {/* Filters */}
      <div className="flex gap-4 flex-wrap">
        <select
          value={ruleFilter}
          onChange={e => setRuleFilter(e.target.value)}
          className="px-3 py-2 bg-muted-background border border-border rounded text-foreground text-sm focus:outline-none focus:ring-2 focus:ring-primary"
        >
          <option value="all">All Rules</option>
          {rules.map((rule: any) => (
            <option key={rule.id} value={rule.name}>
              {rule.name}
            </option>
          ))}
        </select>

        <select
          value={severityFilter}
          onChange={e => setSeverityFilter(e.target.value)}
          className="px-3 py-2 bg-muted-background border border-border rounded text-foreground text-sm focus:outline-none focus:ring-2 focus:ring-primary"
        >
          <option value="all">All Severities</option>
          <option value="critical">Critical</option>
          <option value="high">High</option>
          <option value="medium">Medium</option>
          <option value="low">Low</option>
        </select>

        <select
          value={statusFilter}
          onChange={e => setStatusFilter(e.target.value)}
          className="px-3 py-2 bg-muted-background border border-border rounded text-foreground text-sm focus:outline-none focus:ring-2 focus:ring-primary"
        >
          <option value="all">All Statuses</option>
          <option value="delivered">Delivered</option>
          <option value="acknowledged">Acknowledged</option>
          <option value="suppressed">Suppressed</option>
        </select>
      </div>

      {/* Alerts Table */}
      <div className="bg-muted-background border border-border rounded-lg overflow-hidden">
        {isLoading ? (
          <div className="p-8 text-center text-muted-foreground">
            Loading alerts...
          </div>
        ) : alerts.length === 0 ? (
          <div className="p-8 text-center text-muted-foreground">
            No alerts found
          </div>
        ) : (
          <table className="w-full">
            <thead className="bg-background border-b border-border">
              <tr>
                <th className="px-6 py-3 text-left text-sm font-semibold text-foreground">Rule</th>
                <th className="px-6 py-3 text-left text-sm font-semibold text-foreground">Severity</th>
                <th className="px-6 py-3 text-left text-sm font-semibold text-foreground">Status</th>
                <th className="px-6 py-3 text-left text-sm font-semibold text-foreground">Time</th>
                <th className="px-6 py-3 text-left text-sm font-semibold text-foreground">Summary</th>
                <th className="px-6 py-3 text-right text-sm font-semibold text-foreground">Actions</th>
              </tr>
            </thead>
            <tbody className="divide-y divide-border">
              {alerts.map((alert: Alert) => (
                <tr key={alert.id} className="hover:bg-border/30 transition-colors">
                  <td className="px-6 py-4 text-sm font-medium text-foreground">
                    {alert.rule_name}
                  </td>
                  <td className="px-6 py-4">
                    <span className={`px-2 py-1 text-xs font-medium rounded ${getSeverityColor(alert.severity)}`}>
                      {alert.severity.toUpperCase()}
                    </span>
                  </td>
                  <td className="px-6 py-4">
                    <span className={`text-sm font-medium ${getStatusColor(alert.status)}`}>
                      {alert.status.charAt(0).toUpperCase() + alert.status.slice(1)}
                    </span>
                  </td>
                  <td className="px-6 py-4 text-sm text-muted-foreground">
                    {new Date(alert.timestamp).toLocaleTimeString()}
                  </td>
                  <td className="px-6 py-4 text-sm text-muted-foreground truncate max-w-xs">
                    {alert.signal_summary}
                  </td>
                  <td className="px-6 py-4 text-right">
                    <button
                      onClick={() => setSelectedAlert(alert)}
                      className="px-3 py-1 text-xs bg-border text-foreground rounded hover:bg-border/80 transition-colors"
                    >
                      View
                    </button>
                  </td>
                </tr>
              ))}
            </tbody>
          </table>
        )}
      </div>

      {/* Alert Detail Modal */}
      {selectedAlert && (
        <AlertDetailModal
          alert={selectedAlert}
          onClose={() => setSelectedAlert(null)}
        />
      )}
    </div>
  )
}

const AlertDetailModal: React.FC<{ alert: Alert; onClose: () => void }> = ({
  alert,
  onClose,
}) => {
  const [isSuppressing, setIsSuppressing] = useState(false)

  const handleSuppress = async () => {
    setIsSuppressing(true)
    try {
      await axios.post(`/api/v1/alerts/${alert.id}/suppress`)
      onClose()
    } catch (error) {
      console.error('Failed to suppress alert:', error)
    } finally {
      setIsSuppressing(false)
    }
  }

  return (
    <div className="fixed inset-0 bg-black/50 z-50 flex items-center justify-center p-4">
      <div className="bg-muted-background border border-border rounded-lg p-6 max-w-2xl w-full max-h-96 overflow-y-auto">
        <h2 className="text-2xl font-bold text-foreground mb-4">Alert Details</h2>

        <div className="space-y-4 mb-6">
          <div className="grid grid-cols-2 gap-4">
            <div>
              <p className="text-xs text-muted-foreground uppercase">Rule</p>
              <p className="text-sm font-medium text-foreground mt-1">{alert.rule_name}</p>
            </div>
            <div>
              <p className="text-xs text-muted-foreground uppercase">Severity</p>
              <p className="text-sm font-medium text-foreground mt-1">{alert.severity.toUpperCase()}</p>
            </div>
            <div>
              <p className="text-xs text-muted-foreground uppercase">Status</p>
              <p className="text-sm font-medium text-foreground mt-1">{alert.status}</p>
            </div>
            <div>
              <p className="text-xs text-muted-foreground uppercase">Time</p>
              <p className="text-sm font-medium text-foreground mt-1">
                {new Date(alert.timestamp).toLocaleString()}
              </p>
            </div>
          </div>

          <div>
            <p className="text-xs text-muted-foreground uppercase mb-1">Signal Summary</p>
            <p className="text-sm text-muted-foreground">{alert.signal_summary}</p>
          </div>

          {alert.suppression_rules.length > 0 && (
            <div>
              <p className="text-xs text-muted-foreground uppercase mb-2">Applied Suppression Rules</p>
              <div className="space-y-1">
                {alert.suppression_rules.map((rule, idx) => (
                  <div key={idx} className="text-xs bg-background px-3 py-2 rounded border border-border text-muted-foreground">
                    {rule}
                  </div>
                ))}
              </div>
            </div>
          )}
        </div>

        <div className="flex gap-3">
          <button
            onClick={onClose}
            className="flex-1 px-4 py-2 bg-border text-foreground rounded hover:bg-border/80 transition-colors font-medium"
          >
            Close
          </button>
          {alert.status !== 'suppressed' && (
            <button
              onClick={handleSuppress}
              disabled={isSuppressing}
              className="flex-1 px-4 py-2 bg-status-warning/20 text-status-warning rounded hover:opacity-90 disabled:opacity-50 transition-opacity font-medium"
            >
              {isSuppressing ? 'Suppressing...' : 'Suppress Similar'}
            </button>
          )}
        </div>
      </div>
    </div>
  )
}

export default AlertsPage
