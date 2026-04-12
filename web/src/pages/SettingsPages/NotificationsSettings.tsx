import React, { useState } from 'react'
import axios from 'axios'

interface NotificationConfig {
  slack_enabled: boolean
  slack_webhook_url: string
  smtp_enabled: boolean
  smtp_host: string
  smtp_port: number
  smtp_from: string
  pagerduty_enabled: boolean
  pagerduty_api_key: string
}

/**
 * Notifications Settings Page
 */
export const NotificationsSettings: React.FC = () => {
  const [config, setConfig] = useState<NotificationConfig>({
    slack_enabled: false,
    slack_webhook_url: '',
    smtp_enabled: false,
    smtp_host: '',
    smtp_port: 587,
    smtp_from: '',
    pagerduty_enabled: false,
    pagerduty_api_key: '',
  })
  const [testingService, setTestingService] = useState<string | null>(null)
  const [testResult, setTestResult] = useState<{ success: boolean; message: string } | null>(null)
  const [isSaving, setIsSaving] = useState(false)
  const [saved, setSaved] = useState(false)

  const handleTest = async (service: string) => {
    setTestingService(service)
    try {
      await axios.post(`/api/v1/settings/notifications/test/${service}`, config)
      setTestResult({ success: true, message: `${service} notification sent successfully` })
    } catch (error: any) {
      setTestResult({
        success: false,
        message: error.response?.data?.error || `Failed to send ${service} notification`,
      })
    } finally {
      setTestingService(null)
    }
  }

  const handleSave = async () => {
    setIsSaving(true)
    try {
      await axios.post('/api/v1/settings/notifications', config)
      setSaved(true)
      setTimeout(() => setSaved(false), 3000)
    } catch (error) {
      console.error('Failed to save settings:', error)
    } finally {
      setIsSaving(false)
    }
  }

  return (
    <div className="space-y-6">
      <div>
        <h2 className="text-2xl font-bold text-foreground">Notification Settings</h2>
        <p className="text-muted-foreground mt-1">Configure notification channels for alerts</p>
      </div>

      {/* Slack */}
      <div className="p-6 bg-muted-background border border-border rounded-lg space-y-4">
        <div className="flex items-center justify-between">
          <h3 className="text-lg font-semibold text-foreground">Slack</h3>
          <label className="flex items-center gap-2 cursor-pointer">
            <input
              type="checkbox"
              checked={config.slack_enabled}
              onChange={e => setConfig({ ...config, slack_enabled: e.target.checked })}
            />
            <span className="text-sm text-muted-foreground">Enabled</span>
          </label>
        </div>
        {config.slack_enabled && (
          <>
            <input
              type="url"
              value={config.slack_webhook_url}
              onChange={e => setConfig({ ...config, slack_webhook_url: e.target.value })}
              placeholder="https://hooks.slack.com/services/..."
              className="w-full px-3 py-2 bg-background border border-border rounded text-foreground placeholder-muted-foreground focus:outline-none focus:ring-2 focus:ring-primary text-sm"
            />
            <button
              onClick={() => handleTest('slack')}
              disabled={testingService === 'slack'}
              className="px-3 py-1 text-sm bg-border text-foreground rounded hover:bg-border/80 disabled:opacity-50 transition-colors"
            >
              {testingService === 'slack' ? 'Testing...' : 'Test Slack'}
            </button>
          </>
        )}
      </div>

      {/* SMTP */}
      <div className="p-6 bg-muted-background border border-border rounded-lg space-y-4">
        <div className="flex items-center justify-between">
          <h3 className="text-lg font-semibold text-foreground">SMTP (Email)</h3>
          <label className="flex items-center gap-2 cursor-pointer">
            <input
              type="checkbox"
              checked={config.smtp_enabled}
              onChange={e => setConfig({ ...config, smtp_enabled: e.target.checked })}
            />
            <span className="text-sm text-muted-foreground">Enabled</span>
          </label>
        </div>
        {config.smtp_enabled && (
          <>
            <div className="grid grid-cols-2 gap-4">
              <input
                type="text"
                value={config.smtp_host}
                onChange={e => setConfig({ ...config, smtp_host: e.target.value })}
                placeholder="smtp.gmail.com"
                className="px-3 py-2 bg-background border border-border rounded text-foreground placeholder-muted-foreground focus:outline-none focus:ring-2 focus:ring-primary text-sm"
              />
              <input
                type="number"
                value={config.smtp_port}
                onChange={e => setConfig({ ...config, smtp_port: parseInt(e.target.value) })}
                placeholder="587"
                className="px-3 py-2 bg-background border border-border rounded text-foreground focus:outline-none focus:ring-2 focus:ring-primary text-sm"
              />
            </div>
            <input
              type="email"
              value={config.smtp_from}
              onChange={e => setConfig({ ...config, smtp_from: e.target.value })}
              placeholder="noreply@example.com"
              className="w-full px-3 py-2 bg-background border border-border rounded text-foreground placeholder-muted-foreground focus:outline-none focus:ring-2 focus:ring-primary text-sm"
            />
            <button
              onClick={() => handleTest('smtp')}
              disabled={testingService === 'smtp'}
              className="px-3 py-1 text-sm bg-border text-foreground rounded hover:bg-border/80 disabled:opacity-50 transition-colors"
            >
              {testingService === 'smtp' ? 'Testing...' : 'Test Email'}
            </button>
          </>
        )}
      </div>

      {/* PagerDuty */}
      <div className="p-6 bg-muted-background border border-border rounded-lg space-y-4">
        <div className="flex items-center justify-between">
          <h3 className="text-lg font-semibold text-foreground">PagerDuty</h3>
          <label className="flex items-center gap-2 cursor-pointer">
            <input
              type="checkbox"
              checked={config.pagerduty_enabled}
              onChange={e => setConfig({ ...config, pagerduty_enabled: e.target.checked })}
            />
            <span className="text-sm text-muted-foreground">Enabled</span>
          </label>
        </div>
        {config.pagerduty_enabled && (
          <>
            <input
              type="password"
              value={config.pagerduty_api_key}
              onChange={e => setConfig({ ...config, pagerduty_api_key: e.target.value })}
              placeholder="Your PagerDuty API Key"
              className="w-full px-3 py-2 bg-background border border-border rounded text-foreground placeholder-muted-foreground focus:outline-none focus:ring-2 focus:ring-primary text-sm"
            />
            <button
              onClick={() => handleTest('pagerduty')}
              disabled={testingService === 'pagerduty'}
              className="px-3 py-1 text-sm bg-border text-foreground rounded hover:bg-border/80 disabled:opacity-50 transition-colors"
            >
              {testingService === 'pagerduty' ? 'Testing...' : 'Test PagerDuty'}
            </button>
          </>
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

      {saved && (
        <div className="p-4 rounded-lg bg-status-success/10 border border-status-success text-status-success">
          Settings saved successfully
        </div>
      )}

      <button
        onClick={handleSave}
        disabled={isSaving}
        className="px-4 py-2 bg-primary text-white rounded hover:opacity-90 disabled:opacity-50 transition-opacity font-medium"
      >
        {isSaving ? 'Saving...' : 'Save Settings'}
      </button>
    </div>
  )
}
