import { useEffect, useState } from 'react'
import { apiClient } from '../lib/axios-client'
import { useAuthStore } from '../stores/auth'

interface AuditEntry {
  id: string
  user_id: string | null
  action: string
  resource: string
  detail: Record<string, any>
  ip_address: string
  user_agent: string
  timestamp: string
}

/**
 * AuditLogPage Component
 * Allows admins and analysts to view the immutable audit trail
 */
export function AuditLogPage() {
  const { isAdmin, isAnalyst } = useAuthStore()
  const [entries, setEntries] = useState<AuditEntry[]>([])
  const [loading, setLoading] = useState(true)
  const [error, setError] = useState<string | null>(null)
  const [page, setPage] = useState(0)
  const [totalPages, setTotalPages] = useState(0)
  const [filters, setFilters] = useState({
    action: '',
    user_id: '',
    startDate: '',
    endDate: '',
  })

  const pageSize = 50

  useEffect(() => {
    loadAuditLog()
  }, [page, filters])

  const loadAuditLog = async () => {
    setLoading(true)
    setError(null)
    try {
      const params = new URLSearchParams()
      params.set('limit', pageSize.toString())
      params.set('offset', (page * pageSize).toString())
      if (filters.action) params.set('action', filters.action)
      if (filters.user_id) params.set('user_id', filters.user_id)
      if (filters.startDate) params.set('from', filters.startDate)
      if (filters.endDate) params.set('to', filters.endDate)

      const response = await apiClient.get(`/audit?${params.toString()}`)
      setEntries(response.data.entries || [])
      setTotalPages(Math.ceil((response.data.total || 0) / pageSize))
    } catch (err) {
      const message = err instanceof Error ? err.message : 'Failed to load audit log'
      setError(message)
    } finally {
      setLoading(false)
    }
  }

  const handleExport = async () => {
    try {
      const response = await apiClient.get('/audit/export', {
        params: filters,
        responseType: 'blob',
      })
      const url = window.URL.createObjectURL(new Blob([response.data]))
      const link = document.createElement('a')
      link.href = url
      link.setAttribute('download', `audit-log-${new Date().toISOString()}.csv`)
      document.body.appendChild(link)
      link.click()
      link.parentElement?.removeChild(link)
    } catch (err) {
      const message = err instanceof Error ? err.message : 'Failed to export'
      setError(message)
    }
  }

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

  return (
    <div className="min-h-screen bg-background">
      {/* Header */}
      <div className="bg-muted-background border-b border-border p-6">
        <div className="flex items-center justify-between">
          <div>
            <h1 className="text-3xl font-bold text-foreground">Audit Log</h1>
            <p className="text-muted-foreground mt-1">Immutable record of all actions</p>
          </div>
          <button
            onClick={handleExport}
            className="px-4 py-2 bg-primary hover:bg-primary/90 text-foreground rounded-lg"
          >
            Export CSV
          </button>
        </div>
      </div>

      {/* Content */}
      <div className="max-w-7xl mx-auto p-6">
        {/* Error */}
        {error && (
          <div className="bg-red-900/20 border border-red-900/50 rounded-lg p-4 mb-6">
            <p className="text-status-error">{error}</p>
          </div>
        )}

        {/* Filters */}
        <div className="bg-muted-background rounded-lg border border-border p-6 mb-6">
          <h2 className="text-lg font-bold text-foreground mb-4">Filters</h2>
          <div className="grid grid-cols-4 gap-4">
            <div>
              <label className="block text-sm font-medium text-foreground/90 mb-2">
                Action
              </label>
              <select
                value={filters.action}
                onChange={(e) =>
                  setFilters({ ...filters, action: e.target.value })
                }
                className="w-full px-4 py-2 rounded-lg bg-background border border-border text-foreground"
              >
                <option value="">All Actions</option>
                <option value="login">Login</option>
                <option value="logout">Logout</option>
                <option value="user_create">User Create</option>
                <option value="user_update">User Update</option>
                <option value="user_delete">User Delete</option>
                <option value="rule_create">Rule Create</option>
                <option value="rule_update">Rule Update</option>
                <option value="rule_delete">Rule Delete</option>
                <option value="alert_acknowledge">Alert Acknowledge</option>
              </select>
            </div>
            <div>
              <label className="block text-sm font-medium text-foreground/90 mb-2">
                Start Date
              </label>
              <input
                type="date"
                value={filters.startDate}
                onChange={(e) =>
                  setFilters({ ...filters, startDate: e.target.value })
                }
                className="w-full px-4 py-2 rounded-lg bg-background border border-border text-foreground"
              />
            </div>
            <div>
              <label className="block text-sm font-medium text-foreground/90 mb-2">
                End Date
              </label>
              <input
                type="date"
                value={filters.endDate}
                onChange={(e) =>
                  setFilters({ ...filters, endDate: e.target.value })
                }
                className="w-full px-4 py-2 rounded-lg bg-background border border-border text-foreground"
              />
            </div>
            <div className="flex items-end">
              <button
                onClick={() => setPage(0)}
                className="w-full py-2 px-4 bg-slate-700 hover:bg-slate-600 text-foreground rounded-lg"
              >
                Apply
              </button>
            </div>
          </div>
        </div>

        {/* Entries Table */}
        {loading ? (
          <p className="text-muted-foreground">Loading audit log...</p>
        ) : entries.length === 0 ? (
          <p className="text-muted-foreground">No audit entries found</p>
        ) : (
          <>
            <div className="bg-muted-background rounded-lg border border-border overflow-hidden">
              <table className="w-full">
                <thead className="bg-background border-b border-border">
                  <tr>
                    <th className="px-6 py-3 text-left text-sm font-semibold text-foreground/90">
                      Timestamp
                    </th>
                    <th className="px-6 py-3 text-left text-sm font-semibold text-foreground/90">
                      User
                    </th>
                    <th className="px-6 py-3 text-left text-sm font-semibold text-foreground/90">
                      Action
                    </th>
                    <th className="px-6 py-3 text-left text-sm font-semibold text-foreground/90">
                      Resource
                    </th>
                    <th className="px-6 py-3 text-left text-sm font-semibold text-foreground/90">
                      IP Address
                    </th>
                  </tr>
                </thead>
                <tbody className="divide-y divide-border">
                  {entries.map((entry) => (
                    <tr key={entry.id} className="hover:bg-background/50">
                      <td className="px-6 py-4 text-foreground/90 text-sm">
                        {new Date(entry.timestamp).toLocaleString()}
                      </td>
                      <td className="px-6 py-4 text-foreground text-sm">
                        {entry.user_id ? entry.user_id.substring(0, 8) : 'System'}
                      </td>
                      <td className="px-6 py-4 text-foreground text-sm font-medium">
                        {entry.action}
                      </td>
                      <td className="px-6 py-4 text-foreground/90 text-sm">
                        {entry.resource}
                      </td>
                      <td className="px-6 py-4 text-muted-foreground text-sm">
                        {entry.ip_address || '-'}
                      </td>
                    </tr>
                  ))}
                </tbody>
              </table>
            </div>

            {/* Pagination */}
            {totalPages > 1 && (
              <div className="flex items-center justify-center gap-4 mt-6">
                <button
                  onClick={() => setPage(Math.max(0, page - 1))}
                  disabled={page === 0}
                  className="px-4 py-2 bg-slate-700 hover:bg-slate-600 text-foreground rounded-lg disabled:opacity-50"
                >
                  Previous
                </button>
                <span className="text-muted-foreground">
                  Page {page + 1} of {totalPages}
                </span>
                <button
                  onClick={() => setPage(Math.min(totalPages - 1, page + 1))}
                  disabled={page === totalPages - 1}
                  className="px-4 py-2 bg-slate-700 hover:bg-slate-600 text-foreground rounded-lg disabled:opacity-50"
                >
                  Next
                </button>
              </div>
            )}
          </>
        )}
      </div>
    </div>
  )
}
