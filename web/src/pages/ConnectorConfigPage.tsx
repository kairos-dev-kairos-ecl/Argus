import React, { useState } from 'react'
import { useParams } from 'react-router-dom'
import axios from 'axios'

type ConnectorMode = 'sdk-direct' | 'api-proxy' | 'local-model'

interface ConnectorConfig {
  app_id: string
  mode: ConnectorMode
  sdk_direct?: Record<string, any>
  api_proxy?: {
    upstream_url: string
    auth_passthrough: boolean
    signal_depth: 'headers' | 'headers-metadata' | 'full'
    latency_budget_ms: number
  }
  local_model?: {
    server_url: string
    model_name: string
    signal_depth: 'basic' | 'full'
  }
}

/**
 * ConnectorConfigPage Component
 * Configure LLM connector modes:
 * 1. SDK Direct - embedded SDK (no config)
 * 2. API Proxy - transparent HTTP forwarding
 * 3. Local Model - Ollama/vLLM integration
 */
export const ConnectorConfigPage: React.FC = () => {
  const { appId } = useParams<{ appId: string }>()
  const [mode, setMode] = useState<ConnectorMode>('sdk-direct')
  const [config, setConfig] = useState<ConnectorConfig>({
    app_id: appId || '',
    mode: 'sdk-direct',
    api_proxy: {
      upstream_url: '',
      auth_passthrough: true,
      signal_depth: 'headers-metadata',
      latency_budget_ms: 100,
    },
    local_model: {
      server_url: '',
      model_name: '',
      signal_depth: 'full',
    },
  })
  const [models, setModels] = useState<string[]>([])
  const [testResult, setTestResult] = useState<{ success: boolean; message: string } | null>(null)
  const [isSaving, setIsSaving] = useState(false)

  const handleModeChange = (newMode: ConnectorMode) => {
    setMode(newMode)
    setConfig({ ...config, mode: newMode })
  }

  const handleTestConnection = async () => {
    try {
      const endpoint =
        mode === 'api-proxy' ? '/api/v1/connectors/test-proxy' : '/api/v1/connectors/test-local-model'
      const res = await axios.post(endpoint, config)
      setTestResult({ success: true, message: res.data.message })
    } catch (error: any) {
      setTestResult({
        success: false,
        message: error.response?.data?.error || 'Connection test failed',
      })
    }
  }

  const handleDiscoverModels = async () => {
    if (!config.local_model?.server_url) {
      setTestResult({ success: false, message: 'Please enter server URL first' })
      return
    }
    try {
      const res = await axios.get('/api/v1/connectors/models', {
        params: { server_url: config.local_model.server_url },
      })
      setModels(res.data.models || [])
    } catch (error: any) {
      setTestResult({
        success: false,
        message: error.response?.data?.error || 'Failed to discover models',
      })
    }
  }

  const handleSave = async () => {
    setIsSaving(true)
    try {
      await axios.post(`/api/v1/apps/${appId}/connector-config`, config)
      setTestResult({ success: true, message: 'Configuration saved successfully' })
    } catch (error: any) {
      setTestResult({
        success: false,
        message: error.response?.data?.error || 'Failed to save configuration',
      })
    } finally {
      setIsSaving(false)
    }
  }

  return (
    <div className="max-w-4xl space-y-6">
      {/* Header */}
      <div>
        <h1 className="text-3xl font-bold text-foreground">LLM Connector Configuration</h1>
        <p className="text-muted-foreground mt-1">
          Choose how signals are captured from LLM calls
        </p>
      </div>

      {/* Mode Selection */}
      <div className="space-y-4">
        <h2 className="text-lg font-semibold text-foreground">Connector Mode</h2>
        <div className="space-y-3">
          {/* SDK Direct */}
          <label className="flex items-start gap-4 p-4 border border-border rounded-lg cursor-pointer hover:bg-border/30 transition-colors">
            <input
              type="radio"
              name="mode"
              value="sdk-direct"
              checked={mode === 'sdk-direct'}
              onChange={e => handleModeChange(e.target.value as ConnectorMode)}
              className="mt-1"
            />
            <div className="flex-1">
              <div className="font-medium text-foreground">SDK Direct</div>
              <p className="text-sm text-muted-foreground mt-1">
                Application embeds Argus SDK. Signals sent directly to Argus API.
              </p>
              <div className="mt-3 text-xs bg-background px-3 py-2 rounded border border-border text-muted-foreground font-mono">
                pip install argus-sdk
              </div>
            </div>
          </label>

          {/* API Proxy */}
          <label className="flex items-start gap-4 p-4 border border-border rounded-lg cursor-pointer hover:bg-border/30 transition-colors">
            <input
              type="radio"
              name="mode"
              value="api-proxy"
              checked={mode === 'api-proxy'}
              onChange={e => handleModeChange(e.target.value as ConnectorMode)}
              className="mt-1"
            />
            <div className="flex-1">
              <div className="font-medium text-foreground">API Proxy</div>
              <p className="text-sm text-muted-foreground mt-1">
                Route LLM API calls through Argus proxy for signal extraction without SDK.
              </p>
            </div>
          </label>

          {/* Local Model */}
          <label className="flex items-start gap-4 p-4 border border-border rounded-lg cursor-pointer hover:bg-border/30 transition-colors">
            <input
              type="radio"
              name="mode"
              value="local-model"
              checked={mode === 'local-model'}
              onChange={e => handleModeChange(e.target.value as ConnectorMode)}
              className="mt-1"
            />
            <div className="flex-1">
              <div className="font-medium text-foreground">Local Model</div>
              <p className="text-sm text-muted-foreground mt-1">
                Monitor local model servers (Ollama, vLLM, llama.cpp, TGI).
              </p>
            </div>
          </label>
        </div>
      </div>

      {/* Mode-Specific Configuration */}
      <div className="space-y-4 p-6 bg-muted-background border border-border rounded-lg">
        {mode === 'sdk-direct' && (
          <div>
            <p className="text-muted-foreground">
              No additional configuration needed. See Apps page for SDK setup instructions.
            </p>
          </div>
        )}

        {mode === 'api-proxy' && config.api_proxy && (
          <div className="space-y-4">
            <div>
              <label className="block text-sm font-medium text-foreground mb-2">
                Upstream API URL
              </label>
              <input
                type="url"
                value={config.api_proxy.upstream_url}
                onChange={e =>
                  setConfig({
                    ...config,
                    api_proxy: { ...config.api_proxy!, upstream_url: e.target.value },
                  })
                }
                placeholder="https://api.openai.com/v1"
                className="w-full px-3 py-2 bg-background border border-border rounded text-foreground placeholder-muted-foreground focus:outline-none focus:ring-2 focus:ring-primary"
              />
              <p className="text-xs text-muted-foreground mt-2">
                Examples: https://api.openai.com/v1, https://api.anthropic.com/v1
              </p>
            </div>

            <div>
              <label className="flex items-center gap-3 text-sm font-medium text-foreground cursor-pointer">
                <input
                  type="checkbox"
                  checked={config.api_proxy.auth_passthrough}
                  onChange={e =>
                    setConfig({
                      ...config,
                      api_proxy: { ...config.api_proxy!, auth_passthrough: e.target.checked },
                    })
                  }
                />
                Pass through Authorization header
              </label>
              <p className="text-xs text-muted-foreground mt-2 ml-7">
                Forward client's Authorization header to upstream API
              </p>
            </div>

            <div>
              <label className="block text-sm font-medium text-foreground mb-2">
                Signal Depth
              </label>
              <select
                value={config.api_proxy.signal_depth}
                onChange={e =>
                  setConfig({
                    ...config,
                    api_proxy: { ...config.api_proxy!, signal_depth: e.target.value as any },
                  })
                }
                className="w-full px-3 py-2 bg-background border border-border rounded text-foreground focus:outline-none focus:ring-2 focus:ring-primary"
              >
                <option value="headers">Headers Only</option>
                <option value="headers-metadata">Headers + Metadata</option>
                <option value="full">Full (Response Body)</option>
              </select>
              <p className="text-xs text-muted-foreground mt-2">
                How much response data to capture for signal extraction
              </p>
            </div>

            <div>
              <label className="block text-sm font-medium text-foreground mb-2">
                Latency Budget (ms)
              </label>
              <input
                type="number"
                min="1"
                max="10000"
                value={config.api_proxy.latency_budget_ms}
                onChange={e =>
                  setConfig({
                    ...config,
                    api_proxy: { ...config.api_proxy!, latency_budget_ms: parseInt(e.target.value) },
                  })
                }
                className="w-full px-3 py-2 bg-background border border-border rounded text-foreground focus:outline-none focus:ring-2 focus:ring-primary"
              />
              <p className="text-xs text-muted-foreground mt-2">
                Maximum acceptable additional latency from proxy
              </p>
            </div>
          </div>
        )}

        {mode === 'local-model' && config.local_model && (
          <div className="space-y-4">
            <div>
              <label className="block text-sm font-medium text-foreground mb-2">
                Model Server URL
              </label>
              <div className="flex gap-2">
                <input
                  type="url"
                  value={config.local_model.server_url}
                  onChange={e =>
                    setConfig({
                      ...config,
                      local_model: { ...config.local_model!, server_url: e.target.value },
                    })
                  }
                  placeholder="http://localhost:11434"
                  className="flex-1 px-3 py-2 bg-background border border-border rounded text-foreground placeholder-muted-foreground focus:outline-none focus:ring-2 focus:ring-primary"
                />
                <button
                  onClick={handleDiscoverModels}
                  className="px-4 py-2 bg-border text-foreground rounded hover:bg-border/80 transition-colors text-sm font-medium"
                >
                  Discover
                </button>
              </div>
            </div>

            {models.length > 0 && (
              <div>
                <label className="block text-sm font-medium text-foreground mb-2">
                  Model Name
                </label>
                <select
                  value={config.local_model.model_name}
                  onChange={e =>
                    setConfig({
                      ...config,
                      local_model: { ...config.local_model!, model_name: e.target.value },
                    })
                  }
                  className="w-full px-3 py-2 bg-background border border-border rounded text-foreground focus:outline-none focus:ring-2 focus:ring-primary"
                >
                  <option value="">Select a model...</option>
                  {models.map(model => (
                    <option key={model} value={model}>
                      {model}
                    </option>
                  ))}
                </select>
              </div>
            )}

            <div>
              <label className="block text-sm font-medium text-foreground mb-2">
                Signal Depth
              </label>
              <select
                value={config.local_model.signal_depth}
                onChange={e =>
                  setConfig({
                    ...config,
                    local_model: { ...config.local_model!, signal_depth: e.target.value as any },
                  })
                }
                className="w-full px-3 py-2 bg-background border border-border rounded text-foreground focus:outline-none focus:ring-2 focus:ring-primary"
              >
                <option value="basic">Basic (token counts)</option>
                <option value="full">Full (logprobs, sampling params)</option>
              </select>
            </div>
          </div>
        )}
      </div>

      {/* Test Result */}
      {testResult && (
        <div
          className={`p-4 rounded-lg border ${
            testResult.success
              ? 'bg-status-success/10 border-status-success text-status-success'
              : 'bg-status-error/10 border-status-error text-status-error'
          }`}
        >
          {testResult.message}
        </div>
      )}

      {/* Actions */}
      <div className="flex gap-3">
        {mode !== 'sdk-direct' && (
          <button
            onClick={handleTestConnection}
            className="px-4 py-2 bg-border text-foreground rounded hover:bg-border/80 transition-colors font-medium"
          >
            Test Connection
          </button>
        )}
        <button
          onClick={handleSave}
          disabled={isSaving}
          className="px-4 py-2 bg-primary text-white rounded hover:opacity-90 disabled:opacity-50 transition-opacity font-medium"
        >
          {isSaving ? 'Saving...' : 'Save Configuration'}
        </button>
      </div>
    </div>
  )
}

export default ConnectorConfigPage
