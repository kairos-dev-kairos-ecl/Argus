import React, { useState } from 'react'
import { useQuery } from '@tanstack/react-query'
import axios from 'axios'

interface App {
  id: string
  name: string
  description: string
  environment: 'development' | 'staging' | 'production'
  api_key: string
  connector_mode: 'sdk-direct' | 'api-proxy' | 'local-model'
  status: 'active' | 'inactive' | 'error'
  signal_rate: number // signals per minute
  last_signal_at: string
  created_at: string
}

/**
 * AppsPage Component
 * Manage monitored applications:
 * - Register new apps
 * - View API keys
 * - Check connection status
 * - Configure connector modes
 */
export const AppsPage: React.FC = () => {
  const [showCreateModal, setShowCreateModal] = useState(false)
  const [showApiKeyModal, setShowApiKeyModal] = useState<string | null>(null)
  const [selectedApp, setSelectedApp] = useState<App | null>(null)

  // Fetch apps
  const { data: apps = [], isLoading } = useQuery({
    queryKey: ['apps'],
    queryFn: async () => {
      const res = await axios.get('/api/v1/apps')
      return res.data.apps || []
    },
    staleTime: 30000,
  })

  const getStatusColor = (status: string): string => {
    switch (status) {
      case 'active':
        return 'text-status-success'
      case 'inactive':
        return 'text-muted-foreground'
      case 'error':
        return 'text-status-error'
      default:
        return 'text-status-warning'
    }
  }

  const getConnectorLabel = (mode: string): string => {
    switch (mode) {
      case 'sdk-direct':
        return 'SDK Direct'
      case 'api-proxy':
        return 'API Proxy'
      case 'local-model':
        return 'Local Model'
      default:
        return mode
    }
  }

  const getEnvironmentColor = (env: string): string => {
    switch (env) {
      case 'production':
        return 'bg-destructive/20 text-destructive'
      case 'staging':
        return 'bg-status-warning/20 text-status-warning'
      case 'development':
        return 'bg-status-info/20 text-status-info'
      default:
        return 'bg-border text-muted-foreground'
    }
  }

  return (
    <div className="space-y-6">
      {/* Header */}
      <div className="flex items-center justify-between">
        <div>
          <h1 className="text-3xl font-bold text-foreground">Applications</h1>
          <p className="text-muted-foreground mt-1">Register and manage monitored applications</p>
        </div>
        <button
          onClick={() => setShowCreateModal(true)}
          className="px-4 py-2 bg-primary text-white rounded-md hover:opacity-90 transition-opacity font-medium"
        >
          + New App
        </button>
      </div>

      {/* Apps Table */}
      <div className="bg-muted-background border border-border rounded-lg overflow-hidden">
        {isLoading ? (
          <div className="p-8 text-center text-muted-foreground">
            Loading applications...
          </div>
        ) : apps.length === 0 ? (
          <div className="p-8 text-center text-muted-foreground">
            <p className="mb-4">No applications registered yet</p>
            <button
              onClick={() => setShowCreateModal(true)}
              className="px-4 py-2 bg-border text-foreground rounded-md hover:bg-border/80 transition-colors"
            >
              Create First App
            </button>
          </div>
        ) : (
          <table className="w-full">
            <thead className="bg-background border-b border-border">
              <tr>
                <th className="px-6 py-3 text-left text-sm font-semibold text-foreground">Name</th>
                <th className="px-6 py-3 text-left text-sm font-semibold text-foreground">Environment</th>
                <th className="px-6 py-3 text-left text-sm font-semibold text-foreground">Connector</th>
                <th className="px-6 py-3 text-left text-sm font-semibold text-foreground">Status</th>
                <th className="px-6 py-3 text-left text-sm font-semibold text-foreground">Signal Rate</th>
                <th className="px-6 py-3 text-left text-sm font-semibold text-foreground">Last Signal</th>
                <th className="px-6 py-3 text-right text-sm font-semibold text-foreground">Actions</th>
              </tr>
            </thead>
            <tbody className="divide-y divide-border">
              {apps.map((app: App) => (
                <tr key={app.id} className="hover:bg-border/30 transition-colors">
                  <td className="px-6 py-4">
                    <div className="font-medium text-foreground">{app.name}</div>
                    <div className="text-xs text-muted-foreground">{app.description}</div>
                  </td>
                  <td className="px-6 py-4">
                    <span className={`px-2 py-1 text-xs font-medium rounded ${getEnvironmentColor(app.environment)}`}>
                      {app.environment.toUpperCase()}
                    </span>
                  </td>
                  <td className="px-6 py-4 text-sm text-muted-foreground">
                    {getConnectorLabel(app.connector_mode)}
                  </td>
                  <td className="px-6 py-4">
                    <div className="flex items-center gap-2">
                      <div className={`w-2 h-2 rounded-full ${
                        app.status === 'active' ? 'bg-status-success' :
                        app.status === 'inactive' ? 'bg-muted-foreground' :
                        'bg-status-error'
                      }`} />
                      <span className={`text-sm font-medium ${getStatusColor(app.status)}`}>
                        {app.status.charAt(0).toUpperCase() + app.status.slice(1)}
                      </span>
                    </div>
                  </td>
                  <td className="px-6 py-4 text-sm text-muted-foreground">
                    {app.signal_rate} signals/min
                  </td>
                  <td className="px-6 py-4 text-sm text-muted-foreground">
                    {new Date(app.last_signal_at).toLocaleDateString()}
                  </td>
                  <td className="px-6 py-4 text-right">
                    <div className="flex items-center justify-end gap-2">
                      <button
                        onClick={() => setShowApiKeyModal(app.id)}
                        className="px-3 py-1 text-xs bg-border text-foreground rounded hover:bg-border/80 transition-colors"
                      >
                        API Key
                      </button>
                      <button
                        onClick={() => setSelectedApp(app)}
                        className="px-3 py-1 text-xs bg-border text-foreground rounded hover:bg-border/80 transition-colors"
                      >
                        Configure
                      </button>
                    </div>
                  </td>
                </tr>
              ))}
            </tbody>
          </table>
        )}
      </div>

      {/* Create App Modal */}
      {showCreateModal && (
        <CreateAppModal onClose={() => setShowCreateModal(false)} />
      )}

      {/* API Key Modal */}
      {showApiKeyModal && (
        <ApiKeyModal
          appId={showApiKeyModal}
          onClose={() => setShowApiKeyModal(null)}
        />
      )}

      {/* Configure App Modal */}
      {selectedApp && (
        <ConfigureAppModal
          app={selectedApp}
          onClose={() => setSelectedApp(null)}
        />
      )}
    </div>
  )
}

const CreateAppModal: React.FC<{ onClose: () => void }> = ({ onClose }) => {
  const [formData, setFormData] = useState({
    name: '',
    description: '',
    environment: 'development' as const,
    connector_mode: 'sdk-direct' as const,
  })

  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault()
    try {
      await axios.post('/api/v1/apps', formData)
      onClose()
    } catch (error) {
      console.error('Failed to create app:', error)
    }
  }

  return (
    <div className="fixed inset-0 bg-black/50 z-50 flex items-center justify-center p-4">
      <div className="bg-muted-background border border-border rounded-lg p-6 max-w-md w-full">
        <h2 className="text-xl font-bold text-foreground mb-4">Create New App</h2>
        <form onSubmit={handleSubmit} className="space-y-4">
          <div>
            <label className="block text-sm font-medium text-foreground mb-2">
              App Name
            </label>
            <input
              type="text"
              value={formData.name}
              onChange={e => setFormData({ ...formData, name: e.target.value })}
              className="w-full px-3 py-2 bg-background border border-border rounded text-foreground placeholder-muted-foreground focus:outline-none focus:ring-2 focus:ring-primary"
              placeholder="My LLM Application"
              required
            />
          </div>

          <div>
            <label className="block text-sm font-medium text-foreground mb-2">
              Description
            </label>
            <input
              type="text"
              value={formData.description}
              onChange={e => setFormData({ ...formData, description: e.target.value })}
              className="w-full px-3 py-2 bg-background border border-border rounded text-foreground placeholder-muted-foreground focus:outline-none focus:ring-2 focus:ring-primary"
              placeholder="Brief description"
            />
          </div>

          <div>
            <label className="block text-sm font-medium text-foreground mb-2">
              Environment
            </label>
            <select
              value={formData.environment}
              onChange={e => setFormData({ ...formData, environment: e.target.value as any })}
              className="w-full px-3 py-2 bg-background border border-border rounded text-foreground focus:outline-none focus:ring-2 focus:ring-primary"
            >
              <option value="development">Development</option>
              <option value="staging">Staging</option>
              <option value="production">Production</option>
            </select>
          </div>

          <div>
            <label className="block text-sm font-medium text-foreground mb-2">
              Connector Mode
            </label>
            <select
              value={formData.connector_mode}
              onChange={e => setFormData({ ...formData, connector_mode: e.target.value as any })}
              className="w-full px-3 py-2 bg-background border border-border rounded text-foreground focus:outline-none focus:ring-2 focus:ring-primary"
            >
              <option value="sdk-direct">SDK Direct</option>
              <option value="api-proxy">API Proxy</option>
              <option value="local-model">Local Model</option>
            </select>
          </div>

          <div className="flex gap-3 pt-4">
            <button
              type="button"
              onClick={onClose}
              className="flex-1 px-4 py-2 bg-border text-foreground rounded hover:bg-border/80 transition-colors font-medium"
            >
              Cancel
            </button>
            <button
              type="submit"
              className="flex-1 px-4 py-2 bg-primary text-white rounded hover:opacity-90 transition-opacity font-medium"
            >
              Create
            </button>
          </div>
        </form>
      </div>
    </div>
  )
}

const ApiKeyModal: React.FC<{ appId: string; onClose: () => void }> = ({ appId, onClose }) => {
  const [apiKey, setApiKey] = useState('')

  React.useEffect(() => {
    // Fetch API key
    axios.get(`/api/v1/apps/${appId}/api-key`).then(res => {
      setApiKey(res.data.api_key)
    }).catch(err => {
      console.error('Failed to fetch API key:', err)
    })
  }, [appId])

  const copyToClipboard = () => {
    navigator.clipboard.writeText(apiKey)
  }

  return (
    <div className="fixed inset-0 bg-black/50 z-50 flex items-center justify-center p-4">
      <div className="bg-muted-background border border-border rounded-lg p-6 max-w-md w-full">
        <h2 className="text-xl font-bold text-foreground mb-4">API Key</h2>
        <div className="bg-background border border-border rounded p-3 mb-4 flex items-center justify-between gap-2">
          <code className="text-xs text-muted-foreground overflow-auto flex-1">
            {apiKey || 'Loading...'}
          </code>
          <button
            onClick={copyToClipboard}
            className="px-2 py-1 bg-border text-foreground rounded text-xs hover:bg-border/80 transition-colors flex-shrink-0"
          >
            Copy
          </button>
        </div>
        <p className="text-xs text-muted-foreground mb-4">
          Store this key securely. It won't be shown again.
        </p>
        <button
          onClick={onClose}
          className="w-full px-4 py-2 bg-primary text-white rounded hover:opacity-90 transition-opacity font-medium"
        >
          Done
        </button>
      </div>
    </div>
  )
}

const ConfigureAppModal: React.FC<{ app: App; onClose: () => void }> = ({ app, onClose }) => {
  return (
    <div className="fixed inset-0 bg-black/50 z-50 flex items-center justify-center p-4">
      <div className="bg-muted-background border border-border rounded-lg p-6 max-w-md w-full">
        <h2 className="text-xl font-bold text-foreground mb-4">Configure {app.name}</h2>
        <p className="text-muted-foreground mb-4">
          Go to LLM Connectors page to configure this app's connector settings.
        </p>
        <button
          onClick={onClose}
          className="w-full px-4 py-2 bg-primary text-white rounded hover:opacity-90 transition-opacity font-medium"
        >
          Close
        </button>
      </div>
    </div>
  )
}

export default AppsPage
