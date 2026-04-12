import { useState, useEffect } from 'react'
import { useAuthStore } from '../stores/auth'
import { apiClient } from '../lib/axios-client'

interface Session {
  id: string
  user_agent: string
  ip_address: string
  created_at: string
  last_used_at: string
  current?: boolean
}

/**
 * ProfilePage Component
 * Allows users to manage their profile, change password, and view active sessions.
 */
export function ProfilePage() {
  const { user, logout: _logout } = useAuthStore()
  const [tab, setTab] = useState<'profile' | 'sessions'>('profile')
  const [loading, setLoading] = useState(false)
  const [error, setError] = useState<string | null>(null)
  const [success, setSuccess] = useState<string | null>(null)

  // Profile form
  const [displayName, setDisplayName] = useState(user?.display_name || '')
  const [timezone, setTimezone] = useState('UTC')

  // Password change
  const [showPasswordForm, setShowPasswordForm] = useState(false)
  const [oldPassword, setOldPassword] = useState('')
  const [newPassword, setNewPassword] = useState('')
  const [confirmPassword, setConfirmPassword] = useState('')

  // Sessions
  const [sessions, setSessions] = useState<Session[]>([])

  // Load sessions on mount
  useEffect(() => {
    if (tab === 'sessions') {
      loadSessions()
    }
  }, [tab])

  const loadSessions = async () => {
    setLoading(true)
    try {
      const response = await apiClient.get('/users/me/sessions')
      setSessions(response.data.sessions || [])
    } catch (err) {
      const message = err instanceof Error ? err.message : 'Failed to load sessions'
      setError(message)
    } finally {
      setLoading(false)
    }
  }

  const handleSaveProfile = async (e: React.FormEvent) => {
    e.preventDefault()
    setLoading(true)
    setError(null)
    setSuccess(null)

    try {
      await apiClient.put('/users/me', {
        display_name: displayName,
        timezone,
      })
      setSuccess('Profile updated successfully')
    } catch (err) {
      const message = err instanceof Error ? err.message : 'Failed to update profile'
      setError(message)
    } finally {
      setLoading(false)
    }
  }

  const handleChangePassword = async (e: React.FormEvent) => {
    e.preventDefault()
    setError(null)
    setSuccess(null)

    if (newPassword !== confirmPassword) {
      setError('Passwords do not match')
      return
    }

    if (newPassword.length < 12) {
      setError('Password must be at least 12 characters')
      return
    }

    setLoading(true)
    try {
      await apiClient.post('/users/me/change-password', {
        old_password: oldPassword,
        new_password: newPassword,
      })
      setSuccess('Password changed successfully')
      setOldPassword('')
      setNewPassword('')
      setConfirmPassword('')
      setShowPasswordForm(false)
    } catch (err) {
      const message = err instanceof Error ? err.message : 'Failed to change password'
      setError(message)
    } finally {
      setLoading(false)
    }
  }

  const handleRevokeSession = async (sessionId: string) => {
    setLoading(true)
    try {
      await apiClient.delete(`/users/me/sessions/${sessionId}`)
      setSessions(sessions.filter((s) => s.id !== sessionId))
      setSuccess('Session revoked')
    } catch (err) {
      const message = err instanceof Error ? err.message : 'Failed to revoke session'
      setError(message)
    } finally {
      setLoading(false)
    }
  }

  return (
    <div className="min-h-screen bg-background">
      {/* Header */}
      <div className="bg-muted-background border-b border-border p-6">
        <h1 className="text-3xl font-bold text-foreground">My Profile</h1>
        <p className="text-muted-foreground mt-1">{user?.email}</p>
      </div>

      {/* Content */}
      <div className="max-w-3xl mx-auto p-6">
        {/* Tabs */}
        <div className="flex gap-4 mb-6 border-b border-border">
          <button
            onClick={() => setTab('profile')}
            className={`px-4 py-2 border-b-2 transition-colors ${
              tab === 'profile'
                ? 'border-blue-500 text-foreground'
                : 'border-transparent text-muted-foreground hover:text-foreground/90'
            }`}
          >
            Profile
          </button>
          <button
            onClick={() => setTab('sessions')}
            className={`px-4 py-2 border-b-2 transition-colors ${
              tab === 'sessions'
                ? 'border-blue-500 text-foreground'
                : 'border-transparent text-muted-foreground hover:text-foreground/90'
            }`}
          >
            Active Sessions
          </button>
        </div>

        {/* Messages */}
        {error && (
          <div className="bg-red-900/20 border border-red-900/50 rounded-lg p-4 mb-6">
            <p className="text-status-error">{error}</p>
          </div>
        )}
        {success && (
          <div className="bg-green-900/20 border border-green-900/50 rounded-lg p-4 mb-6">
            <p className="text-status-success">{success}</p>
          </div>
        )}

        {/* Profile Tab */}
        {tab === 'profile' && (
          <div className="bg-muted-background rounded-lg border border-border p-6 space-y-6">
            {/* User Info */}
            <div>
              <h2 className="text-xl font-bold text-foreground mb-4">User Information</h2>
              <div className="grid grid-cols-2 gap-4">
                <div>
                  <label className="block text-sm text-muted-foreground mb-1">Email</label>
                  <div className="text-foreground">{user?.email}</div>
                </div>
                <div>
                  <label className="block text-sm text-muted-foreground mb-1">Role</label>
                  <div className="text-foreground capitalize">{user?.role}</div>
                </div>
              </div>
            </div>

            {/* Profile Form */}
            <div className="border-t border-border pt-6">
              <h2 className="text-xl font-bold text-foreground mb-4">Update Profile</h2>
              <form onSubmit={handleSaveProfile} className="space-y-4">
                <div>
                  <label className="block text-sm font-medium text-foreground/90 mb-2">
                    Display Name
                  </label>
                  <input
                    type="text"
                    value={displayName}
                    onChange={(e) => setDisplayName(e.target.value)}
                    className="w-full px-4 py-2 rounded-lg bg-background border border-border text-foreground"
                    disabled={loading}
                  />
                </div>
                <div>
                  <label className="block text-sm font-medium text-foreground/90 mb-2">
                    Timezone
                  </label>
                  <input
                    type="text"
                    value={timezone}
                    onChange={(e) => setTimezone(e.target.value)}
                    className="w-full px-4 py-2 rounded-lg bg-background border border-border text-foreground"
                    disabled={loading}
                  />
                </div>
                <button
                  type="submit"
                  disabled={loading}
                  className="px-4 py-2 bg-primary hover:bg-primary/90 text-foreground rounded-lg disabled:opacity-50"
                >
                  {loading ? 'Saving...' : 'Save Changes'}
                </button>
              </form>
            </div>

            {/* Password Change */}
            <div className="border-t border-border pt-6">
              <h2 className="text-xl font-bold text-foreground mb-4">Security</h2>
              {!showPasswordForm ? (
                <button
                  onClick={() => setShowPasswordForm(true)}
                  className="px-4 py-2 bg-slate-700 hover:bg-slate-600 text-foreground rounded-lg"
                >
                  Change Password
                </button>
              ) : (
                <form onSubmit={handleChangePassword} className="space-y-4">
                  <div>
                    <label className="block text-sm font-medium text-foreground/90 mb-2">
                      Current Password
                    </label>
                    <input
                      type="password"
                      value={oldPassword}
                      onChange={(e) => setOldPassword(e.target.value)}
                      className="w-full px-4 py-2 rounded-lg bg-background border border-border text-foreground"
                      disabled={loading}
                    />
                  </div>
                  <div>
                    <label className="block text-sm font-medium text-foreground/90 mb-2">
                      New Password
                    </label>
                    <input
                      type="password"
                      value={newPassword}
                      onChange={(e) => setNewPassword(e.target.value)}
                      className="w-full px-4 py-2 rounded-lg bg-background border border-border text-foreground"
                      disabled={loading}
                    />
                  </div>
                  <div>
                    <label className="block text-sm font-medium text-foreground/90 mb-2">
                      Confirm Password
                    </label>
                    <input
                      type="password"
                      value={confirmPassword}
                      onChange={(e) => setConfirmPassword(e.target.value)}
                      className="w-full px-4 py-2 rounded-lg bg-background border border-border text-foreground"
                      disabled={loading}
                    />
                  </div>
                  <div className="flex gap-4">
                    <button
                      type="submit"
                      disabled={loading}
                      className="px-4 py-2 bg-primary hover:bg-primary/90 text-foreground rounded-lg disabled:opacity-50"
                    >
                      {loading ? 'Updating...' : 'Update Password'}
                    </button>
                    <button
                      type="button"
                      onClick={() => setShowPasswordForm(false)}
                      className="px-4 py-2 bg-slate-700 hover:bg-slate-600 text-foreground rounded-lg"
                    >
                      Cancel
                    </button>
                  </div>
                </form>
              )}
            </div>
          </div>
        )}

        {/* Sessions Tab */}
        {tab === 'sessions' && (
          <div className="bg-muted-background rounded-lg border border-border p-6 space-y-6">
            <h2 className="text-xl font-bold text-foreground">Active Sessions</h2>
            {loading && sessions.length === 0 ? (
              <p className="text-muted-foreground">Loading sessions...</p>
            ) : sessions.length === 0 ? (
              <p className="text-muted-foreground">No active sessions</p>
            ) : (
              <div className="space-y-4">
                {sessions.map((session) => (
                  <div
                    key={session.id}
                    className="bg-background rounded-lg border border-border p-4"
                  >
                    <div className="flex justify-between items-start">
                      <div className="flex-1">
                        <p className="text-foreground font-medium">
                          {session.user_agent.split('/')[0] || 'Unknown Device'}
                          {session.current && (
                            <span className="ml-2 text-xs bg-primary text-foreground px-2 py-1 rounded">
                              This device
                            </span>
                          )}
                        </p>
                        <p className="text-muted-foreground text-sm mt-1">{session.ip_address}</p>
                        <p className="text-muted-foreground/70 text-xs mt-2">
                          Created {new Date(session.created_at).toLocaleDateString()}
                        </p>
                      </div>
                      {!session.current && (
                        <button
                          onClick={() => handleRevokeSession(session.id)}
                          disabled={loading}
                          className="px-3 py-1 bg-red-600 hover:bg-red-700 text-foreground text-sm rounded disabled:opacity-50"
                        >
                          Revoke
                        </button>
                      )}
                    </div>
                  </div>
                ))}
              </div>
            )}
          </div>
        )}
      </div>
    </div>
  )
}
