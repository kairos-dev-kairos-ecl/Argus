import React, { useState } from 'react'
import axios from 'axios'

interface GeneralConfig {
  instance_name: string
  timezone: string
  theme: 'dark' | 'light'
  default_retention_days: number
}

/**
 * General Settings Page
 */
export const GeneralSettings: React.FC = () => {
  const [config, setConfig] = useState<GeneralConfig>({
    instance_name: 'Argus XDR',
    timezone: 'UTC',
    theme: 'dark',
    default_retention_days: 30,
  })
  const [isSaving, setIsSaving] = useState(false)
  const [saved, setSaved] = useState(false)

  const timezones = [
    'UTC',
    'America/New_York',
    'America/Los_Angeles',
    'Europe/London',
    'Europe/Paris',
    'Asia/Tokyo',
    'Asia/Shanghai',
  ]

  const handleSave = async () => {
    setIsSaving(true)
    try {
      await axios.post('/api/v1/settings/general', config)
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
        <h2 className="text-2xl font-bold text-foreground">General Settings</h2>
        <p className="text-muted-foreground mt-1">Configure instance name, timezone, and defaults</p>
      </div>

      <div className="space-y-4 p-6 bg-muted-background border border-border rounded-lg">
        <div>
          <label className="block text-sm font-medium text-foreground mb-2">
            Instance Name
          </label>
          <input
            type="text"
            value={config.instance_name}
            onChange={e => setConfig({ ...config, instance_name: e.target.value })}
            className="w-full px-3 py-2 bg-background border border-border rounded text-foreground placeholder-muted-foreground focus:outline-none focus:ring-2 focus:ring-primary"
            placeholder="My Argus Instance"
          />
        </div>

        <div>
          <label className="block text-sm font-medium text-foreground mb-2">
            Timezone
          </label>
          <select
            value={config.timezone}
            onChange={e => setConfig({ ...config, timezone: e.target.value })}
            className="w-full px-3 py-2 bg-background border border-border rounded text-foreground focus:outline-none focus:ring-2 focus:ring-primary"
          >
            {timezones.map(tz => (
              <option key={tz} value={tz}>
                {tz}
              </option>
            ))}
          </select>
        </div>

        <div>
          <label className="block text-sm font-medium text-foreground mb-2">
            Theme
          </label>
          <select
            value={config.theme}
            onChange={e => setConfig({ ...config, theme: e.target.value as any })}
            className="w-full px-3 py-2 bg-background border border-border rounded text-foreground focus:outline-none focus:ring-2 focus:ring-primary"
          >
            <option value="dark">Dark</option>
            <option value="light">Light</option>
          </select>
        </div>

        <div>
          <label className="block text-sm font-medium text-foreground mb-2">
            Default Retention (days)
          </label>
          <input
            type="number"
            min="1"
            value={config.default_retention_days}
            onChange={e => setConfig({ ...config, default_retention_days: parseInt(e.target.value) })}
            className="w-full px-3 py-2 bg-background border border-border rounded text-foreground focus:outline-none focus:ring-2 focus:ring-primary"
          />
          <p className="text-xs text-muted-foreground mt-2">
            Default number of days to retain signals
          </p>
        </div>
      </div>

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
