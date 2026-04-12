import { useState } from 'react'
import { apiClient } from '../lib/axios-client'
import { useAuthStore } from '../stores/auth'

/**
 * ConfigPage Component
 * Configuration editor for YAML rules, alert routing, retention, notifications
 * (v1 stub - detailed implementation in Phase 6.7)
 */
export function ConfigPage() {
  const { canAccess } = useAuthStore()
  const [tab, setTab] = useState<'rules' | 'routing' | 'retention' | 'notifications'>('rules')
  const [loading, setLoading] = useState(false)
  const [error, setError] = useState<string | null>(null)
  const [success, setSuccess] = useState<string | null>(null)

  // Rules tab state
  const [rulesYaml, setRulesYaml] = useState('')

  // Retention tab state
  const [retention, setRetention] = useState({
    signals_days: 30,
    incidents_days: 90,
    audit_days: 365,
  })

  // Notifications tab state
  const [notifications, setNotifications] = useState({
    slack_enabled: false,
    slack_webhook: '',
    smtp_enabled: false,
    smtp_host: '',
    smtp_port: 587,
  })

  const handleValidateRules = async () => {
    setLoading(true)
    setError(null)
    try {
      await apiClient.post('/rules/validate', {
        yaml_content: rulesYaml,
      })
      setSuccess('Rules validation passed!')
    } catch (err) {
      const message = err instanceof Error ? err.message : 'Validation failed'
      setError(message)
    } finally {
      setLoading(false)
    }
  }

  const handleApplyRules = async () => {
    setLoading(true)
    setError(null)
    try {
      await apiClient.post('/rules/apply', {
        yaml_content: rulesYaml,
      })
      setSuccess('Rules applied successfully!')
    } catch (err) {
      const message = err instanceof Error ? err.message : 'Failed to apply rules'
      setError(message)
    } finally {
      setLoading(false)
    }
  }

  const handleSaveRetention = async () => {
    setLoading(true)
    setError(null)
    try {
      await apiClient.put('/config/retention', retention)
      setSuccess('Retention settings saved!')
    } catch (err) {
      const message = err instanceof Error ? err.message : 'Failed to save settings'
      setError(message)
    } finally {
      setLoading(false)
    }
  }

  const handleSaveNotifications = async () => {
    setLoading(true)
    setError(null)
    try {
      await apiClient.put('/config/notifications', notifications)
      setSuccess('Notification settings saved!')
    } catch (err) {
      const message = err instanceof Error ? err.message : 'Failed to save settings'
      setError(message)
    } finally {
      setLoading(false)
    }
  }

  const handleTestSlack = async () => {
    setLoading(true)
    try {
      await apiClient.post('/config/notifications/test-slack', {
        webhook_url: notifications.slack_webhook,
      })
      setSuccess('Slack webhook test successful!')
    } catch (err) {
      const message = err instanceof Error ? err.message : 'Slack test failed'
      setError(message)
    } finally {
      setLoading(false)
    }
  }

  if (!canAccess('admin')) {
    return (
      <div className="min-h-screen bg-background flex items-center justify-center">
        <div className="text-center">
          <h1 className="text-2xl font-bold text-status-error mb-2">Access Denied</h1>
          <p className="text-muted-foreground">Only admins can access configuration</p>
        </div>
      </div>
    )
  }

  return (
    <div className="min-h-screen bg-background">
      {/* Header */}
      <div className="bg-muted-background border-b border-border p-6">
        <h1 className="text-3xl font-bold text-foreground">Configuration</h1>
        <p className="text-muted-foreground mt-1">Manage system rules, routing, retention, and notifications</p>
      </div>

      {/* Content */}
      <div className="max-w-7xl mx-auto p-6">
        {/* Messages */}
        {error && (
          <div className="bg-status-error/10 border border-status-error/30 rounded-lg p-4 mb-6" role="alert">
            <p className="text-status-error">{error}</p>
          </div>
        )}
        {success && (
          <div className="bg-status-success/10 border border-status-success/30 rounded-lg p-4 mb-6" role="status">
            <p className="text-status-success">{success}</p>
          </div>
        )}

        {/* Tabs */}
        <div className="flex gap-4 mb-6 border-b border-border">
          {['rules', 'routing', 'retention', 'notifications'].map((t) => (
            <button
              key={t}
              onClick={() => setTab(t as any)}
              className={`px-4 py-2 border-b-2 transition-colors capitalize h-10 ${
                tab === t
                  ? 'border-primary text-foreground font-medium'
                  : 'border-transparent text-muted-foreground hover:text-foreground'
              }`}
              aria-selected={tab === t}
            >
              {t}
            </button>
          ))}
        </div>

        {/* Rules Tab */}
        {tab === 'rules' && (
          <div className="bg-muted-background rounded-lg border border-border p-6">
            <h2 className="text-xl font-bold text-foreground mb-4">Detection Rules</h2>
            <div className="mb-4">
              <label htmlFor="rules-yaml" className="block text-sm font-medium text-foreground mb-2">
                YAML Content
              </label>
              <textarea
                id="rules-yaml"
                value={rulesYaml}
                onChange={(e) => setRulesYaml(e.target.value)}
                placeholder="rules:
  - name: PromptInjection
    pattern: '.*ignore.*'
    severity: high"
                className="w-full h-64 px-4 py-2 rounded-lg bg-background border border-border text-foreground font-mono text-sm focus:outline-none focus:ring-2 focus:ring-primary transition-colors"
                disabled={loading}
                aria-label="YAML rules content"
              />
            </div>
            <div className="flex gap-4">
              <button
                onClick={handleValidateRules}
                disabled={loading}
                className="h-10 px-4 bg-primary hover:bg-primary/90 disabled:bg-primary/50 text-foreground rounded-lg transition-colors duration-200 font-medium"
                aria-label="Validate rules"
              >
                {loading ? 'Validating...' : 'Validate'}
              </button>
              <button
                onClick={handleApplyRules}
                disabled={loading}
                className="h-10 px-4 bg-status-success hover:bg-status-success/90 disabled:bg-status-success/50 text-foreground rounded-lg transition-colors duration-200 font-medium"
                aria-label="Apply rules"
              >
                {loading ? 'Applying...' : 'Apply'}
              </button>
            </div>
          </div>
        )}

        {/* Retention Tab */}
        {tab === 'retention' && (
          <div className="bg-muted-background rounded-lg border border-border p-6">
            <h2 className="text-xl font-bold text-foreground mb-4">Data Retention</h2>
            <div className="grid grid-cols-1 md:grid-cols-3 gap-6 mb-6">
              <div>
                <label htmlFor="signals-days" className="block text-sm font-medium text-foreground mb-2">
                  Signals Retention (days)
                </label>
                <input
                  id="signals-days"
                  type="number"
                  min="1"
                  max="3650"
                  value={retention.signals_days}
                  onChange={(e) =>
                    setRetention({
                      ...retention,
                      signals_days: parseInt(e.target.value),
                    })
                  }
                  className="w-full h-10 px-4 py-2 rounded-lg bg-background border border-border text-foreground focus:outline-none focus:ring-2 focus:ring-primary transition-colors"
                  disabled={loading}
                />
                <p className="text-xs text-muted-foreground mt-1">
                  {retention.signals_days} days = ~{Math.ceil((retention.signals_days / 30) * 100)}% of year
                </p>
              </div>
              <div>
                <label htmlFor="incidents-days" className="block text-sm font-medium text-foreground mb-2">
                  Incidents Retention (days)
                </label>
                <input
                  id="incidents-days"
                  type="number"
                  min="1"
                  max="3650"
                  value={retention.incidents_days}
                  onChange={(e) =>
                    setRetention({
                      ...retention,
                      incidents_days: parseInt(e.target.value),
                    })
                  }
                  className="w-full h-10 px-4 py-2 rounded-lg bg-background border border-border text-foreground focus:outline-none focus:ring-2 focus:ring-primary transition-colors"
                  disabled={loading}
                />
              </div>
              <div>
                <label htmlFor="audit-days" className="block text-sm font-medium text-foreground mb-2">
                  Audit Log Retention (days)
                </label>
                <input
                  id="audit-days"
                  type="number"
                  min="1"
                  max="3650"
                  value={retention.audit_days}
                  onChange={(e) =>
                    setRetention({
                      ...retention,
                      audit_days: parseInt(e.target.value),
                    })
                  }
                  className="w-full h-10 px-4 py-2 rounded-lg bg-background border border-border text-foreground focus:outline-none focus:ring-2 focus:ring-primary transition-colors"
                  disabled={loading}
                />
              </div>
            </div>
            <button
              onClick={handleSaveRetention}
              disabled={loading}
              className="h-10 px-4 bg-primary hover:bg-primary/90 disabled:bg-primary/50 text-foreground rounded-lg transition-colors duration-200 font-medium"
              aria-label="Save retention settings"
            >
              {loading ? 'Saving...' : 'Save'}
            </button>
          </div>
        )}

        {/* Notifications Tab */}
        {tab === 'notifications' && (
          <div className="bg-muted-background rounded-lg border border-border p-6 space-y-6">
            <h2 className="text-xl font-bold text-foreground">Notification Channels</h2>

            {/* Slack */}
            <div className="border-t border-border pt-6">
              <div className="flex items-center gap-4 mb-4">
                <h3 className="text-lg font-semibold text-foreground">Slack</h3>
                <label className="flex items-center h-6">
                  <input
                    type="checkbox"
                    checked={notifications.slack_enabled}
                    onChange={(e) =>
                      setNotifications({
                        ...notifications,
                        slack_enabled: e.target.checked,
                      })
                    }
                    className="mr-2 w-4 h-4 cursor-pointer"
                    disabled={loading}
                    aria-label="Enable Slack notifications"
                  />
                  <span className="text-muted-foreground">Enabled</span>
                </label>
              </div>
              {notifications.slack_enabled && (
                <div className="space-y-4">
                  <div>
                    <label htmlFor="slack-webhook" className="block text-sm font-medium text-foreground mb-2">
                      Webhook URL
                    </label>
                    <input
                      id="slack-webhook"
                      type="text"
                      value={notifications.slack_webhook}
                      onChange={(e) =>
                        setNotifications({
                          ...notifications,
                          slack_webhook: e.target.value,
                        })
                      }
                      placeholder="https://hooks.slack.com/..."
                      className="w-full h-10 px-4 py-2 rounded-lg bg-background border border-border text-foreground focus:outline-none focus:ring-2 focus:ring-primary transition-colors"
                      disabled={loading}
                    />
                  </div>
                  <button
                    onClick={handleTestSlack}
                    disabled={loading || !notifications.slack_webhook}
                    className="h-10 px-4 bg-primary/20 hover:bg-primary/30 disabled:bg-border disabled:text-muted-foreground text-foreground rounded-lg transition-colors duration-200 font-medium"
                    aria-label="Test Slack connection"
                  >
                    Test Connection
                  </button>
                </div>
              )}
            </div>

            {/* SMTP */}
            <div className="border-t border-border pt-6">
              <div className="flex items-center gap-4 mb-4">
                <h3 className="text-lg font-semibold text-foreground">Email (SMTP)</h3>
                <label className="flex items-center h-6">
                  <input
                    type="checkbox"
                    checked={notifications.smtp_enabled}
                    onChange={(e) =>
                      setNotifications({
                        ...notifications,
                        smtp_enabled: e.target.checked,
                      })
                    }
                    className="mr-2 w-4 h-4 cursor-pointer"
                    disabled={loading}
                    aria-label="Enable SMTP notifications"
                  />
                  <span className="text-muted-foreground">Enabled</span>
                </label>
              </div>
              {notifications.smtp_enabled && (
                <div className="grid grid-cols-1 md:grid-cols-2 gap-4">
                  <div>
                    <label htmlFor="smtp-host" className="block text-sm font-medium text-foreground mb-2">
                      Host
                    </label>
                    <input
                      id="smtp-host"
                      type="text"
                      value={notifications.smtp_host}
                      onChange={(e) =>
                        setNotifications({
                          ...notifications,
                          smtp_host: e.target.value,
                        })
                      }
                      placeholder="smtp.example.com"
                      className="w-full h-10 px-4 py-2 rounded-lg bg-background border border-border text-foreground focus:outline-none focus:ring-2 focus:ring-primary transition-colors"
                      disabled={loading}
                    />
                  </div>
                  <div>
                    <label htmlFor="smtp-port" className="block text-sm font-medium text-foreground mb-2">
                      Port
                    </label>
                    <input
                      id="smtp-port"
                      type="number"
                      value={notifications.smtp_port}
                      onChange={(e) =>
                        setNotifications({
                          ...notifications,
                          smtp_port: parseInt(e.target.value),
                        })
                      }
                      className="w-full h-10 px-4 py-2 rounded-lg bg-background border border-border text-foreground focus:outline-none focus:ring-2 focus:ring-primary transition-colors"
                      disabled={loading}
                    />
                  </div>
                </div>
              )}
            </div>

            <button
              onClick={handleSaveNotifications}
              disabled={loading}
              className="h-10 px-4 bg-primary hover:bg-primary/90 disabled:bg-primary/50 text-foreground rounded-lg transition-colors duration-200 font-medium"
              aria-label="Save notification settings"
            >
              {loading ? 'Saving...' : 'Save'}
            </button>
          </div>
        )}

        {/* Routing Tab */}
        {tab === 'routing' && (
          <div className="bg-muted-background rounded-lg border border-border p-6">
            <h2 className="text-xl font-bold text-foreground mb-4">Alert Routing</h2>
            <p className="text-muted-foreground mb-4">
              Alert routing configuration coming in Phase 6.7 (detailed implementation)
            </p>
            <div className="space-y-2 text-sm text-muted-foreground">
              <p>- Route rules by severity level</p>
              <p>- Configure escalation workflows</p>
              <p>- Filter by detection category</p>
            </div>
          </div>
        )}
      </div>
    </div>
  )
}
