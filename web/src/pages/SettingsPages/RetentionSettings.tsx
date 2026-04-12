import React, { useState } from 'react'
import axios from 'axios'

type Severity = 'critical' | 'high' | 'medium' | 'low' | 'info'
type Layer = 'L1' | 'L2' | 'L3' | 'L4' | 'L5' | 'L6' | 'L7' | 'L8' | 'L9' | 'L10'

interface RetentionPolicy {
  [key: string]: number // layer_severity -> days
}

/**
 * Retention Settings Page
 * Grid-based retention policy for Layer × Severity
 */
export const RetentionSettings: React.FC = () => {
  const layers: Layer[] = ['L1', 'L2', 'L3', 'L4', 'L5', 'L6', 'L7', 'L8', 'L9', 'L10']
  const severities: Severity[] = ['critical', 'high', 'medium', 'low', 'info']

  const [policy, setPolicy] = useState<RetentionPolicy>({
    'L1_critical': 90,
    'L1_high': 60,
    'L1_medium': 30,
    'L1_low': 14,
    'L1_info': 7,
    // ... more defaults
  })
  const estimatedStorage = '2.5 TB / month'
  const [isSaving, setIsSaving] = useState(false)
  const [saved, setSaved] = useState(false)

  const handleRetentionChange = (layer: Layer, severity: Severity, days: number) => {
    const key = `${layer}_${severity}`
    setPolicy({ ...policy, [key]: days })
  }

  const handleSave = async () => {
    setIsSaving(true)
    try {
      await axios.post('/api/v1/settings/retention', policy)
      setSaved(true)
      setTimeout(() => setSaved(false), 3000)
    } catch (error) {
      console.error('Failed to save settings:', error)
    } finally {
      setIsSaving(false)
    }
  }

  const getLayerColor = (layer: Layer): string => {
    const colors: Record<Layer, string> = {
      L1: 'bg-layer-l1/10 text-layer-l1',
      L2: 'bg-layer-l2/10 text-layer-l2',
      L3: 'bg-layer-l3/10 text-layer-l3',
      L4: 'bg-layer-l4/10 text-layer-l4',
      L5: 'bg-layer-l5/10 text-layer-l5',
      L6: 'bg-layer-l6/10 text-layer-l6',
      L7: 'bg-layer-l7/10 text-layer-l7',
      L8: 'bg-layer-l8/10 text-layer-l8',
      L9: 'bg-layer-l9/10 text-layer-l9',
      L10: 'bg-layer-l10/10 text-layer-l10',
    }
    return colors[layer]
  }

  return (
    <div className="space-y-6">
      <div>
        <h2 className="text-2xl font-bold text-foreground">Retention Policy</h2>
        <p className="text-muted-foreground mt-1">
          Define how long signals are retained per layer and severity
        </p>
      </div>

      {/* Storage Impact */}
      <div className="p-4 bg-border rounded-lg">
        <p className="text-sm text-muted-foreground">
          Estimated monthly storage impact: <span className="font-semibold text-foreground">{estimatedStorage}</span>
        </p>
      </div>

      {/* Retention Grid */}
      <div className="overflow-x-auto">
        <div className="p-6 bg-muted-background border border-border rounded-lg min-w-max">
          <table className="border-collapse">
            <thead>
              <tr>
                <th className="p-3 text-left text-sm font-semibold text-muted-foreground border-b border-border">Layer</th>
                {severities.map(severity => (
                  <th key={severity} className="p-3 text-center text-sm font-semibold text-muted-foreground border-b border-border">
                    {severity.charAt(0).toUpperCase() + severity.slice(1)}
                  </th>
                ))}
              </tr>
            </thead>
            <tbody>
              {layers.map(layer => (
                <tr key={layer}>
                  <td className={`p-3 text-sm font-semibold border-r border-b border-border ${getLayerColor(layer)}`}>
                    {layer}
                  </td>
                  {severities.map(severity => (
                    <td key={`${layer}_${severity}`} className="p-3 border-r border-b border-border">
                      <input
                        type="number"
                        min="1"
                        max="730"
                        value={policy[`${layer}_${severity}`] || 30}
                        onChange={e =>
                          handleRetentionChange(layer, severity, parseInt(e.target.value))
                        }
                        className="w-16 px-2 py-1 bg-background border border-border rounded text-foreground text-center text-sm focus:outline-none focus:ring-2 focus:ring-primary"
                      />
                      <div className="text-xs text-muted-foreground text-center mt-1">days</div>
                    </td>
                  ))}
                </tr>
              ))}
            </tbody>
          </table>
        </div>
      </div>

      {/* Legend */}
      <div className="p-4 bg-border rounded-lg">
        <p className="text-xs text-muted-foreground">
          Retention values in days. Maximum 2 years (730 days). Storage impact calculated based on typical signal volume.
        </p>
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
        {isSaving ? 'Saving...' : 'Save Policy'}
      </button>
    </div>
  )
}
