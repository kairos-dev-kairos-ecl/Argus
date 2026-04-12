import React, { useState } from 'react'
import { useQuery } from '@tanstack/react-query'
import axios from 'axios'

interface Incident {
  id: string
  severity: 'critical' | 'high' | 'medium' | 'low'
  status: 'open' | 'acknowledged' | 'resolved'
  signal_count: number
  created_at: string
  updated_at: string
  title: string
  description: string
}

/**
 * IncidentsPage Component
 * View and manage security incidents
 */
export const IncidentsPage: React.FC = () => {
  const [severityFilter, setSeverityFilter] = useState<string>('all')
  const [statusFilter, setStatusFilter] = useState<string>('all')
  const [selectedIncident, setSelectedIncident] = useState<Incident | null>(null)

  const { data: incidents = [], isLoading } = useQuery({
    queryKey: ['incidents', severityFilter, statusFilter],
    queryFn: async () => {
      const params: any = {}
      if (severityFilter !== 'all') params.severity = severityFilter
      if (statusFilter !== 'all') params.status = statusFilter
      const res = await axios.get('/api/v1/incidents', { params })
      return res.data.incidents || []
    },
    staleTime: 30000,
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
      case 'open':
        return 'text-status-error'
      case 'acknowledged':
        return 'text-status-warning'
      case 'resolved':
        return 'text-status-success'
      default:
        return 'text-muted-foreground'
    }
  }

  return (
    <div className="space-y-6">
      {/* Header */}
      <div>
        <h1 className="text-3xl font-bold text-foreground">Incidents</h1>
        <p className="text-muted-foreground mt-1">Security incidents and detected anomalies</p>
      </div>

      {/* Filters */}
      <div className="flex gap-4">
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
          <option value="open">Open</option>
          <option value="acknowledged">Acknowledged</option>
          <option value="resolved">Resolved</option>
        </select>
      </div>

      {/* Incidents Table */}
      <div className="bg-muted-background border border-border rounded-lg overflow-hidden">
        {isLoading ? (
          <div className="p-8 text-center text-muted-foreground">
            Loading incidents...
          </div>
        ) : incidents.length === 0 ? (
          <div className="p-8 text-center text-muted-foreground">
            No incidents found
          </div>
        ) : (
          <table className="w-full">
            <thead className="bg-background border-b border-border">
              <tr>
                <th className="px-6 py-3 text-left text-sm font-semibold text-foreground">ID</th>
                <th className="px-6 py-3 text-left text-sm font-semibold text-foreground">Title</th>
                <th className="px-6 py-3 text-left text-sm font-semibold text-foreground">Severity</th>
                <th className="px-6 py-3 text-left text-sm font-semibold text-foreground">Status</th>
                <th className="px-6 py-3 text-left text-sm font-semibold text-foreground">Signals</th>
                <th className="px-6 py-3 text-left text-sm font-semibold text-foreground">Created</th>
                <th className="px-6 py-3 text-right text-sm font-semibold text-foreground">Actions</th>
              </tr>
            </thead>
            <tbody className="divide-y divide-border">
              {incidents.map((incident: Incident) => (
                <tr key={incident.id} className="hover:bg-border/30 transition-colors">
                  <td className="px-6 py-4 text-sm font-mono text-muted-foreground">
                    {incident.id.slice(0, 8)}...
                  </td>
                  <td className="px-6 py-4 text-sm text-foreground">
                    {incident.title}
                  </td>
                  <td className="px-6 py-4">
                    <span className={`px-2 py-1 text-xs font-medium rounded ${getSeverityColor(incident.severity)}`}>
                      {incident.severity.toUpperCase()}
                    </span>
                  </td>
                  <td className="px-6 py-4">
                    <span className={`text-sm font-medium ${getStatusColor(incident.status)}`}>
                      {incident.status.charAt(0).toUpperCase() + incident.status.slice(1)}
                    </span>
                  </td>
                  <td className="px-6 py-4 text-sm text-muted-foreground">
                    {incident.signal_count}
                  </td>
                  <td className="px-6 py-4 text-sm text-muted-foreground">
                    {new Date(incident.created_at).toLocaleDateString()}
                  </td>
                  <td className="px-6 py-4 text-right">
                    <button
                      onClick={() => setSelectedIncident(incident)}
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

      {/* Incident Detail Modal */}
      {selectedIncident && (
        <IncidentDetailModal
          incident={selectedIncident}
          onClose={() => setSelectedIncident(null)}
        />
      )}
    </div>
  )
}

const IncidentDetailModal: React.FC<{ incident: Incident; onClose: () => void }> = ({
  incident,
  onClose,
}) => {
  const [status, setStatus] = useState(incident.status)
  const [note, setNote] = useState('')
  const [isSaving, setIsSaving] = useState(false)

  const handleStatusUpdate = async () => {
    setIsSaving(true)
    try {
      await axios.patch(`/api/v1/incidents/${incident.id}`, {
        status,
        note: note || undefined,
      })
      onClose()
    } catch (error) {
      console.error('Failed to update incident:', error)
    } finally {
      setIsSaving(false)
    }
  }

  return (
    <div className="fixed inset-0 bg-black/50 z-50 flex items-center justify-center p-4">
      <div className="bg-muted-background border border-border rounded-lg p-6 max-w-2xl w-full max-h-96 overflow-y-auto">
        <h2 className="text-2xl font-bold text-foreground mb-4">{incident.title}</h2>

        <div className="space-y-4 mb-6">
          <div className="grid grid-cols-2 gap-4">
            <div>
              <p className="text-xs text-muted-foreground uppercase">Severity</p>
              <p className="text-sm font-medium text-foreground mt-1">{incident.severity.toUpperCase()}</p>
            </div>
            <div>
              <p className="text-xs text-muted-foreground uppercase">Status</p>
              <p className="text-sm font-medium text-foreground mt-1">{incident.status}</p>
            </div>
            <div>
              <p className="text-xs text-muted-foreground uppercase">Signal Count</p>
              <p className="text-sm font-medium text-foreground mt-1">{incident.signal_count}</p>
            </div>
            <div>
              <p className="text-xs text-muted-foreground uppercase">Created</p>
              <p className="text-sm font-medium text-foreground mt-1">
                {new Date(incident.created_at).toLocaleString()}
              </p>
            </div>
          </div>

          <div>
            <p className="text-xs text-muted-foreground uppercase mb-1">Description</p>
            <p className="text-sm text-muted-foreground">{incident.description}</p>
          </div>

          {/* Status Update */}
          <div>
            <label className="block text-sm font-medium text-foreground mb-2">
              Update Status
            </label>
            <select
              value={status}
              onChange={e => setStatus(e.target.value as any)}
              className="w-full px-3 py-2 bg-background border border-border rounded text-foreground focus:outline-none focus:ring-2 focus:ring-primary text-sm"
            >
              <option value="open">Open</option>
              <option value="acknowledged">Acknowledged</option>
              <option value="resolved">Resolved</option>
            </select>
          </div>

          {/* Add Note */}
          <div>
            <label className="block text-sm font-medium text-foreground mb-2">
              Add Note
            </label>
            <textarea
              value={note}
              onChange={e => setNote(e.target.value)}
              placeholder="Add investigation notes..."
              className="w-full px-3 py-2 bg-background border border-border rounded text-foreground placeholder-muted-foreground focus:outline-none focus:ring-2 focus:ring-primary text-sm"
              rows={3}
            />
          </div>
        </div>

        <div className="flex gap-3">
          <button
            onClick={onClose}
            className="flex-1 px-4 py-2 bg-border text-foreground rounded hover:bg-border/80 transition-colors font-medium"
          >
            Close
          </button>
          <button
            onClick={handleStatusUpdate}
            disabled={isSaving}
            className="flex-1 px-4 py-2 bg-primary text-white rounded hover:opacity-90 disabled:opacity-50 transition-opacity font-medium"
          >
            {isSaving ? 'Updating...' : 'Update'}
          </button>
        </div>
      </div>
    </div>
  )
}

export default IncidentsPage
