import React, { useState } from 'react'
import { GeneralSettings } from './SettingsPages/GeneralSettings'
import { NotificationsSettings } from './SettingsPages/NotificationsSettings'
import { RetentionSettings } from './SettingsPages/RetentionSettings'
import { SDKHealthSettings } from './SettingsPages/SDKHealthSettings'

type SettingsTab = 'general' | 'notifications' | 'retention' | 'sdk-health'

/**
 * Settings Page
 * Tabbed interface for all system settings
 */
export const SettingsPage: React.FC = () => {
  const [activeTab, setActiveTab] = useState<SettingsTab>('general')

  const tabs: Array<{ id: SettingsTab; label: string; icon: string }> = [
    { id: 'general', label: 'General', icon: '⚙️' },
    { id: 'notifications', label: 'Notifications', icon: '🔔' },
    { id: 'retention', label: 'Retention', icon: '📦' },
    { id: 'sdk-health', label: 'SDK Health', icon: '💚' },
  ]

  return (
    <div className="space-y-6">
      {/* Header */}
      <div>
        <h1 className="text-3xl font-bold text-foreground">Settings</h1>
        <p className="text-muted-foreground mt-1">Manage instance configuration and integrations</p>
      </div>

      {/* Tab Navigation */}
      <div className="flex gap-2 border-b border-border overflow-x-auto">
        {tabs.map(tab => (
          <button
            key={tab.id}
            onClick={() => setActiveTab(tab.id)}
            className={`px-4 py-3 font-medium text-sm transition-colors border-b-2 ${
              activeTab === tab.id
                ? 'border-primary text-primary'
                : 'border-transparent text-muted-foreground hover:text-foreground'
            }`}
          >
            <span className="mr-2">{tab.icon}</span>
            {tab.label}
          </button>
        ))}
      </div>

      {/* Tab Content */}
      <div className="py-6">
        {activeTab === 'general' && <GeneralSettings />}
        {activeTab === 'notifications' && <NotificationsSettings />}
        {activeTab === 'retention' && <RetentionSettings />}
        {activeTab === 'sdk-health' && <SDKHealthSettings />}
      </div>
    </div>
  )
}

export default SettingsPage
