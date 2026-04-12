import React from 'react'
import axios from 'axios'
import { useQuery } from '@tanstack/react-query'

interface SDKAppHealth {
  app_id: string
  app_name: string
  api_key: string
  last_signal_at: string
  signal_rate: number
  errors_24h: number
  latency_p99_ms: number
  health_status: 'healthy' | 'warning' | 'critical'
}

/**
 * SDK Health Settings Page
 * Monitor SDK integrations, rotation, signal rates
 */
export const SDKHealthSettings: React.FC = () => {
  const { data: apps = [], isLoading } = useQuery({
    queryKey: ['sdk-health'],
    queryFn: async () => {
      const res = await axios.get('/api/v1/settings/sdk-health')
      return res.data.apps || []
    },
    staleTime: 60000,
  })

  const getHealthColor = (status: string): string => {
    switch (status) {
      case 'healthy':
        return 'text-status-success'
      case 'warning':
        return 'text-status-warning'
      case 'critical':
        return 'text-status-error'
      default:
        return 'text-muted-foreground'
    }
  }

  const handleRotateKey = async (appId: string) => {
    if (!window.confirm('Rotate API key? Current key will be invalidated.')) return
    try {
      await axios.post(`/api/v1/apps/${appId}/rotate-key`)
      // Refetch apps
      window.location.reload()
    } catch (error) {
      console.error('Failed to rotate key:', error)
    }
  }

  return (
    <div className="space-y-6">
      <div>
        <h2 className="text-2xl font-bold text-foreground">SDK Health</h2>
        <p className="text-muted-foreground mt-1">
          Monitor SDK integration health and manage API keys
        </p>
      </div>

      {/* Health Table */}
      <div className="bg-muted-background border border-border rounded-lg overflow-hidden">
        {isLoading ? (
          <div className="p-8 text-center text-muted-foreground">
            Loading SDK health...
          </div>
        ) : apps.length === 0 ? (
          <div className="p-8 text-center text-muted-foreground">
            No SDK integrations found
          </div>
        ) : (
          <table className="w-full">
            <thead className="bg-background border-b border-border">
              <tr>
                <th className="px-6 py-3 text-left text-sm font-semibold text-foreground">App</th>
                <th className="px-6 py-3 text-left text-sm font-semibold text-foreground">Status</th>
                <th className="px-6 py-3 text-left text-sm font-semibold text-foreground">Signal Rate</th>
                <th className="px-6 py-3 text-left text-sm font-semibold text-foreground">Last Signal</th>
                <th className="px-6 py-3 text-left text-sm font-semibold text-foreground">Errors (24h)</th>
                <th className="px-6 py-3 text-left text-sm font-semibold text-foreground">Latency p99</th>
                <th className="px-6 py-3 text-right text-sm font-semibold text-foreground">Actions</th>
              </tr>
            </thead>
            <tbody className="divide-y divide-border">
              {apps.map((app: SDKAppHealth) => (
                <tr key={app.app_id} className="hover:bg-border/30 transition-colors">
                  <td className="px-6 py-4 text-sm font-medium text-foreground">
                    {app.app_name}
                  </td>
                  <td className="px-6 py-4">
                    <div className="flex items-center gap-2">
                      <div className={`w-2 h-2 rounded-full ${
                        app.health_status === 'healthy' ? 'bg-status-success' :
                        app.health_status === 'warning' ? 'bg-status-warning' :
                        'bg-status-error'
                      }`} />
                      <span className={`text-sm font-medium ${getHealthColor(app.health_status)}`}>
                        {app.health_status.charAt(0).toUpperCase() + app.health_status.slice(1)}
                      </span>
                    </div>
                  </td>
                  <td className="px-6 py-4 text-sm text-muted-foreground">
                    {app.signal_rate} signals/min
                  </td>
                  <td className="px-6 py-4 text-sm text-muted-foreground">
                    {new Date(app.last_signal_at).toLocaleTimeString()}
                  </td>
                  <td className="px-6 py-4 text-sm text-muted-foreground">
                    {app.errors_24h > 0 ? (
                      <span className="text-status-error">{app.errors_24h}</span>
                    ) : (
                      '0'
                    )}
                  </td>
                  <td className="px-6 py-4 text-sm text-muted-foreground">
                    {app.latency_p99_ms}ms
                  </td>
                  <td className="px-6 py-4 text-right">
                    <button
                      onClick={() => handleRotateKey(app.app_id)}
                      className="px-3 py-1 text-xs bg-border text-foreground rounded hover:bg-border/80 transition-colors"
                    >
                      Rotate Key
                    </button>
                  </td>
                </tr>
              ))}
            </tbody>
          </table>
        )}
      </div>

      {/* Health Tips */}
      <div className="p-4 bg-status-info/10 border border-status-info rounded-lg">
        <h3 className="font-semibold text-status-info mb-2">Health Status Guide</h3>
        <ul className="text-xs text-status-info space-y-1">
          <li>• <strong>Healthy:</strong> Regular signal flow, no errors</li>
          <li>• <strong>Warning:</strong> Occasional errors or rate fluctuations</li>
          <li>• <strong>Critical:</strong> No recent signals or persistent errors</li>
        </ul>
      </div>
    </div>
  )
}
