import React, { useState } from 'react'
import { useQuery } from '@tanstack/react-query'
import axios from 'axios'

interface Rule {
  id: string
  name: string
  category: string
  enabled: boolean
  created_at: string
  version: number
  yaml_content: string
}

/**
 * RulesPage Component
 * Detection rule management with YAML editor
 */
export const RulesPage: React.FC = () => {
  const [showCreateModal, setShowCreateModal] = useState(false)
  const [selectedRule, setSelectedRule] = useState<Rule | null>(null)
  const [editingRule, setEditingRule] = useState<Rule | null>(null)

  const { data: rules = [], isLoading, refetch } = useQuery({
    queryKey: ['rules'],
    queryFn: async () => {
      const res = await axios.get('/api/v1/rules')
      return res.data.rules || []
    },
    staleTime: 30000,
  })

  const handleToggle = async (rule: Rule) => {
    try {
      await axios.patch(`/api/v1/rules/${rule.id}`, {
        enabled: !rule.enabled,
      })
      refetch()
    } catch (error) {
      console.error('Failed to toggle rule:', error)
    }
  }

  const handleDelete = async (ruleId: string) => {
    if (!window.confirm('Delete this rule? This cannot be undone.')) return
    try {
      await axios.delete(`/api/v1/rules/${ruleId}`)
      refetch()
    } catch (error) {
      console.error('Failed to delete rule:', error)
    }
  }

  return (
    <div className="space-y-6">
      {/* Header */}
      <div className="flex items-center justify-between">
        <div>
          <h1 className="text-3xl font-bold text-foreground">Detection Rules</h1>
          <p className="text-muted-foreground mt-1">Create and manage detection rules in YAML</p>
        </div>
        <button
          onClick={() => setShowCreateModal(true)}
          className="px-4 py-2 bg-primary text-white rounded-md hover:opacity-90 transition-opacity font-medium"
        >
          + New Rule
        </button>
      </div>

      {/* Rules Table */}
      <div className="bg-muted-background border border-border rounded-lg overflow-hidden">
        {isLoading ? (
          <div className="p-8 text-center text-muted-foreground">
            Loading rules...
          </div>
        ) : rules.length === 0 ? (
          <div className="p-8 text-center text-muted-foreground">
            <p className="mb-4">No detection rules created yet</p>
            <button
              onClick={() => setShowCreateModal(true)}
              className="px-4 py-2 bg-border text-foreground rounded-md hover:bg-border/80 transition-colors"
            >
              Create First Rule
            </button>
          </div>
        ) : (
          <table className="w-full">
            <thead className="bg-background border-b border-border">
              <tr>
                <th className="px-6 py-3 text-left text-sm font-semibold text-foreground">Name</th>
                <th className="px-6 py-3 text-left text-sm font-semibold text-foreground">Category</th>
                <th className="px-6 py-3 text-left text-sm font-semibold text-foreground">Status</th>
                <th className="px-6 py-3 text-left text-sm font-semibold text-foreground">Version</th>
                <th className="px-6 py-3 text-left text-sm font-semibold text-foreground">Created</th>
                <th className="px-6 py-3 text-right text-sm font-semibold text-foreground">Actions</th>
              </tr>
            </thead>
            <tbody className="divide-y divide-border">
              {rules.map((rule: Rule) => (
                <tr key={rule.id} className="hover:bg-border/30 transition-colors">
                  <td className="px-6 py-4 text-sm font-medium text-foreground">
                    {rule.name}
                  </td>
                  <td className="px-6 py-4 text-sm text-muted-foreground">
                    {rule.category}
                  </td>
                  <td className="px-6 py-4">
                    <label className="flex items-center gap-2 cursor-pointer">
                      <input
                        type="checkbox"
                        checked={rule.enabled}
                        onChange={() => handleToggle(rule)}
                        className="w-4 h-4"
                      />
                      <span className={`text-sm font-medium ${
                        rule.enabled ? 'text-status-success' : 'text-muted-foreground'
                      }`}>
                        {rule.enabled ? 'Enabled' : 'Disabled'}
                      </span>
                    </label>
                  </td>
                  <td className="px-6 py-4 text-sm text-muted-foreground">
                    v{rule.version}
                  </td>
                  <td className="px-6 py-4 text-sm text-muted-foreground">
                    {new Date(rule.created_at).toLocaleDateString()}
                  </td>
                  <td className="px-6 py-4 text-right">
                    <div className="flex items-center justify-end gap-2">
                      <button
                        onClick={() => setEditingRule(rule)}
                        className="px-3 py-1 text-xs bg-border text-foreground rounded hover:bg-border/80 transition-colors"
                      >
                        Edit
                      </button>
                      <button
                        onClick={() => setSelectedRule(rule)}
                        className="px-3 py-1 text-xs bg-border text-foreground rounded hover:bg-border/80 transition-colors"
                      >
                        Test
                      </button>
                      <button
                        onClick={() => handleDelete(rule.id)}
                        className="px-3 py-1 text-xs bg-status-error/20 text-status-error rounded hover:opacity-90 transition-colors"
                      >
                        Delete
                      </button>
                    </div>
                  </td>
                </tr>
              ))}
            </tbody>
          </table>
        )}
      </div>

      {/* Create/Edit Modal */}
      {(showCreateModal || editingRule) && (
        <RuleEditorModal
          rule={editingRule || undefined}
          onClose={() => {
            setShowCreateModal(false)
            setEditingRule(null)
          }}
          onSave={() => {
            setShowCreateModal(false)
            setEditingRule(null)
            refetch()
          }}
        />
      )}

      {/* Test Modal */}
      {selectedRule && (
        <RuleTestModal
          rule={selectedRule}
          onClose={() => setSelectedRule(null)}
        />
      )}
    </div>
  )
}

const RuleEditorModal: React.FC<{
  rule?: Rule
  onClose: () => void
  onSave: () => void
}> = ({ rule, onClose, onSave }) => {
  const [name, setName] = useState(rule?.name || '')
  const [category, setCategory] = useState(rule?.category || 'custom')
  const [yaml, setYaml] = useState(rule?.yaml_content || '')
  const [validationResult, setValidationResult] = useState<{
    valid: boolean
    errors: string[]
  } | null>(null)
  const [isSaving, setIsSaving] = useState(false)

  const handleValidate = async () => {
    try {
      const res = await axios.post('/api/v1/rules/validate', { yaml })
      setValidationResult({
        valid: res.data.valid,
        errors: res.data.errors || [],
      })
    } catch (error: any) {
      setValidationResult({
        valid: false,
        errors: [error.response?.data?.error || 'Validation failed'],
      })
    }
  }

  const handleSave = async () => {
    if (!validationResult?.valid) {
      alert('Please validate the rule first')
      return
    }
    setIsSaving(true)
    try {
      if (rule) {
        await axios.patch(`/api/v1/rules/${rule.id}`, {
          name,
          category,
          yaml_content: yaml,
        })
      } else {
        await axios.post('/api/v1/rules', {
          name,
          category,
          yaml_content: yaml,
        })
      }
      onSave()
    } catch (error) {
      console.error('Failed to save rule:', error)
    } finally {
      setIsSaving(false)
    }
  }

  return (
    <div className="fixed inset-0 bg-black/50 z-50 flex items-center justify-center p-4">
      <div className="bg-muted-background border border-border rounded-lg p-6 max-w-4xl w-full max-h-96 overflow-y-auto">
        <h2 className="text-2xl font-bold text-foreground mb-4">
          {rule ? 'Edit Rule' : 'Create New Rule'}
        </h2>

        <div className="space-y-4">
          <div className="grid grid-cols-2 gap-4">
            <div>
              <label className="block text-sm font-medium text-foreground mb-2">
                Rule Name
              </label>
              <input
                type="text"
                value={name}
                onChange={e => setName(e.target.value)}
                className="w-full px-3 py-2 bg-background border border-border rounded text-foreground focus:outline-none focus:ring-2 focus:ring-primary"
              />
            </div>
            <div>
              <label className="block text-sm font-medium text-foreground mb-2">
                Category
              </label>
              <input
                type="text"
                value={category}
                onChange={e => setCategory(e.target.value)}
                className="w-full px-3 py-2 bg-background border border-border rounded text-foreground focus:outline-none focus:ring-2 focus:ring-primary"
              />
            </div>
          </div>

          <div>
            <label className="block text-sm font-medium text-foreground mb-2">
              YAML Definition
            </label>
            <textarea
              value={yaml}
              onChange={e => setYaml(e.target.value)}
              placeholder="name: my_rule
detection:
  condition: 'any'
  patterns:
    - field: layer
      operator: equals
      value: L1"
              className="w-full px-3 py-2 bg-background border border-border rounded text-foreground placeholder-muted-foreground focus:outline-none focus:ring-2 focus:ring-primary font-mono text-sm"
              rows={6}
            />
          </div>

          {validationResult && (
            <div
              className={`p-4 rounded-lg border ${
                validationResult.valid
                  ? 'bg-status-success/10 border-status-success text-status-success'
                  : 'bg-status-error/10 border-status-error text-status-error'
              }`}
            >
              {validationResult.valid ? (
                <p className="text-sm">✓ YAML is valid</p>
              ) : (
                <>
                  <p className="text-sm font-medium mb-2">Validation errors:</p>
                  <ul className="text-sm space-y-1">
                    {validationResult.errors.map((err, idx) => (
                      <li key={idx}>• {err}</li>
                    ))}
                  </ul>
                </>
              )}
            </div>
          )}
        </div>

        <div className="flex gap-3 mt-6">
          <button
            onClick={onClose}
            className="flex-1 px-4 py-2 bg-border text-foreground rounded hover:bg-border/80 transition-colors font-medium"
          >
            Cancel
          </button>
          <button
            onClick={handleValidate}
            className="px-4 py-2 bg-border text-foreground rounded hover:bg-border/80 transition-colors font-medium"
          >
            Validate
          </button>
          <button
            onClick={handleSave}
            disabled={isSaving || !validationResult?.valid}
            className="flex-1 px-4 py-2 bg-primary text-white rounded hover:opacity-90 disabled:opacity-50 transition-opacity font-medium"
          >
            {isSaving ? 'Saving...' : rule ? 'Update' : 'Create'}
          </button>
        </div>
      </div>
    </div>
  )
}

const RuleTestModal: React.FC<{ rule: Rule; onClose: () => void }> = ({
  rule,
  onClose,
}) => {
  const [testResult, setTestResult] = useState<{ matches: number; sample_signals: any[] } | null>(null)
  const [isTesting, setIsTesting] = useState(false)

  React.useEffect(() => {
    handleDryRun()
  }, [])

  const handleDryRun = async () => {
    setIsTesting(true)
    try {
      const res = await axios.post(`/api/v1/rules/${rule.id}/dry-run`, {
        time_range_hours: 24,
      })
      setTestResult(res.data)
    } catch (error) {
      console.error('Failed to run test:', error)
    } finally {
      setIsTesting(false)
    }
  }

  return (
    <div className="fixed inset-0 bg-black/50 z-50 flex items-center justify-center p-4">
      <div className="bg-muted-background border border-border rounded-lg p-6 max-w-2xl w-full max-h-96 overflow-y-auto">
        <h2 className="text-2xl font-bold text-foreground mb-4">Test Rule: {rule.name}</h2>

        {isTesting ? (
          <p className="text-muted-foreground">Running dry-run test...</p>
        ) : testResult ? (
          <div className="space-y-4">
            <div className="p-4 bg-status-info/10 border border-status-info rounded-lg">
              <p className="text-status-info text-sm font-semibold">
                Matches: {testResult.matches} signal(s) in last 24 hours
              </p>
            </div>

            {testResult.sample_signals.length > 0 && (
              <div>
                <h3 className="text-sm font-medium text-foreground mb-2">Sample Matching Signals</h3>
                {testResult.sample_signals.map((signal, idx) => (
                  <div key={idx} className="text-xs bg-background px-3 py-2 rounded border border-border text-muted-foreground mb-2 font-mono">
                    {JSON.stringify(signal, null, 2).slice(0, 200)}...
                  </div>
                ))}
              </div>
            )}
          </div>
        ) : null}

        <div className="flex gap-3 mt-6">
          <button
            onClick={onClose}
            className="flex-1 px-4 py-2 bg-border text-foreground rounded hover:bg-border/80 transition-colors font-medium"
          >
            Close
          </button>
          <button
            onClick={handleDryRun}
            disabled={isTesting}
            className="px-4 py-2 bg-border text-foreground rounded hover:bg-border/80 disabled:opacity-50 transition-colors font-medium"
          >
            {isTesting ? 'Testing...' : 'Re-run Test'}
          </button>
        </div>
      </div>
    </div>
  )
}

export default RulesPage
