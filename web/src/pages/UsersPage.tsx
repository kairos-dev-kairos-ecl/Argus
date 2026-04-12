import { useEffect, useState } from 'react'
import { apiClient } from '../lib/axios-client'
import { useAuthStore } from '../stores/auth'
import type { User } from '../types'

/**
 * UsersPage Component
 * Allows admins to manage users: create, edit, suspend/activate, reset password
 */
export function UsersPage() {
  const { isAdmin } = useAuthStore()
  const [users, setUsers] = useState<User[]>([])
  const [loading, setLoading] = useState(true)
  const [error, setError] = useState<string | null>(null)
  const [showCreateForm, setShowCreateForm] = useState(false)
  const [searchTerm, setSearchTerm] = useState('')

  // Create user form
  const [newUser, setNewUser] = useState({
    email: '',
    display_name: '',
    role: 'analyst' as 'admin' | 'analyst' | 'viewer',
  })

  useEffect(() => {
    loadUsers()
  }, [])

  const loadUsers = async () => {
    setLoading(true)
    setError(null)
    try {
      const response = await apiClient.get('/users')
      setUsers(response.data.users || [])
    } catch (err) {
      const message = err instanceof Error ? err.message : 'Failed to load users'
      setError(message)
    } finally {
      setLoading(false)
    }
  }

  const handleCreateUser = async (e: React.FormEvent) => {
    e.preventDefault()
    if (!newUser.email || !newUser.display_name) {
      setError('Email and display name are required')
      return
    }

    setLoading(true)
    try {
      const response = await apiClient.post('/users', {
        email: newUser.email,
        display_name: newUser.display_name,
        role: newUser.role,
      })
      setUsers([...users, response.data.user])
      setNewUser({ email: '', display_name: '', role: 'analyst' })
      setShowCreateForm(false)
    } catch (err) {
      const message = err instanceof Error ? err.message : 'Failed to create user'
      setError(message)
    } finally {
      setLoading(false)
    }
  }

  const handleUpdateRole = async (userId: string, newRole: string) => {
    setLoading(true)
    try {
      await apiClient.put(`/users/${userId}`, { role: newRole })
      setUsers(
        users.map((u) =>
          u.id === userId ? { ...u, role: newRole as any } : u
        )
      )
    } catch (err) {
      const message = err instanceof Error ? err.message : 'Failed to update role'
      setError(message)
    } finally {
      setLoading(false)
    }
  }

  const handleSuspendUser = async (userId: string, shouldSuspend: boolean) => {
    setLoading(true)
    try {
      await apiClient.put(`/users/${userId}`, {
        status: shouldSuspend ? 'suspended' : 'active',
      })
      setUsers(
        users.map((u) =>
          u.id === userId
            ? {
                ...u,
                status: shouldSuspend ? 'suspended' : 'active',
              }
            : u
        )
      )
    } catch (err) {
      const message = err instanceof Error ? err.message : 'Failed to update status'
      setError(message)
    } finally {
      setLoading(false)
    }
  }

  const handleResetPassword = async (userId: string, userEmail: string) => {
    if (!confirm(`Send password reset email to ${userEmail}?`)) return

    setLoading(true)
    try {
      await apiClient.post(`/users/${userId}/reset-password`)
      alert('Password reset email sent')
    } catch (err) {
      const message = err instanceof Error ? err.message : 'Failed to reset password'
      setError(message)
    } finally {
      setLoading(false)
    }
  }

  if (!isAdmin()) {
    return (
      <div className="min-h-screen bg-background flex items-center justify-center">
        <div className="text-center">
          <h1 className="text-2xl font-bold text-status-error mb-2">Access Denied</h1>
          <p className="text-muted-foreground">Only admins can access this page</p>
        </div>
      </div>
    )
  }

  const filteredUsers = users.filter(
    (u) =>
      u.email.toLowerCase().includes(searchTerm.toLowerCase()) ||
      u.display_name.toLowerCase().includes(searchTerm.toLowerCase())
  )

  return (
    <div className="min-h-screen bg-background">
      {/* Header */}
      <div className="bg-muted-background border-b border-border p-6">
        <div className="flex items-center justify-between">
          <div>
            <h1 className="text-3xl font-bold text-foreground">Users</h1>
            <p className="text-muted-foreground mt-1">{users.length} total users</p>
          </div>
          <button
            onClick={() => setShowCreateForm(!showCreateForm)}
            className="h-10 px-4 bg-primary hover:bg-primary/90 text-foreground rounded-lg transition-colors duration-200 font-medium"
            aria-label={showCreateForm ? 'Cancel creating user' : 'Create new user'}
          >
            {showCreateForm ? 'Cancel' : 'Create User'}
          </button>
        </div>
      </div>

      {/* Content */}
      <div className="max-w-7xl mx-auto p-6">
        {/* Error */}
        {error && (
          <div className="bg-status-error/10 border border-status-error/30 rounded-lg p-4 mb-6" role="alert">
            <p className="text-status-error">{error}</p>
          </div>
        )}

        {/* Create Form */}
        {showCreateForm && (
          <div className="bg-muted-background rounded-lg border border-border p-6 mb-6">
            <h2 className="text-xl font-bold text-foreground mb-4">Create New User</h2>
            <form onSubmit={handleCreateUser} className="grid grid-cols-1 md:grid-cols-3 gap-4">
              <div>
                <label htmlFor="email" className="block text-sm font-medium text-foreground mb-2">
                  Email
                </label>
                <input
                  id="email"
                  type="email"
                  value={newUser.email}
                  onChange={(e) =>
                    setNewUser({ ...newUser, email: e.target.value })
                  }
                  className="w-full h-10 px-4 py-2 rounded-lg bg-background border border-border text-foreground focus:outline-none focus:ring-2 focus:ring-primary transition-colors"
                  disabled={loading}
                />
              </div>
              <div>
                <label htmlFor="display-name" className="block text-sm font-medium text-foreground mb-2">
                  Display Name
                </label>
                <input
                  id="display-name"
                  type="text"
                  value={newUser.display_name}
                  onChange={(e) =>
                    setNewUser({ ...newUser, display_name: e.target.value })
                  }
                  className="w-full h-10 px-4 py-2 rounded-lg bg-background border border-border text-foreground focus:outline-none focus:ring-2 focus:ring-primary transition-colors"
                  disabled={loading}
                />
              </div>
              <div>
                <label htmlFor="role" className="block text-sm font-medium text-foreground mb-2">
                  Role
                </label>
                <select
                  id="role"
                  value={newUser.role}
                  onChange={(e) =>
                    setNewUser({
                      ...newUser,
                      role: e.target.value as any,
                    })
                  }
                  className="w-full h-10 px-4 py-2 rounded-lg bg-background border border-border text-foreground focus:outline-none focus:ring-2 focus:ring-primary transition-colors"
                  disabled={loading}
                >
                  <option value="admin">Admin</option>
                  <option value="analyst">Analyst</option>
                  <option value="viewer">Viewer</option>
                </select>
              </div>
              <button
                type="submit"
                disabled={loading}
                className="md:col-span-3 h-10 px-4 bg-primary hover:bg-primary/90 disabled:bg-primary/50 text-foreground rounded-lg transition-colors duration-200 font-medium"
              >
                {loading ? 'Creating...' : 'Create User'}
              </button>
            </form>
          </div>
        )}

        {/* Search */}
        <div className="mb-6">
          <input
            type="text"
            placeholder="Search users by email or name..."
            value={searchTerm}
            onChange={(e) => setSearchTerm(e.target.value)}
            className="w-full h-10 px-4 py-2 rounded-lg bg-muted-background border border-border text-foreground placeholder-muted-foreground focus:outline-none focus:ring-2 focus:ring-primary transition-colors"
            aria-label="Search users"
          />
        </div>

        {/* Users Table */}
        {loading && filteredUsers.length === 0 ? (
          <p className="text-muted-foreground">Loading users...</p>
        ) : filteredUsers.length === 0 ? (
          <p className="text-muted-foreground">No users found</p>
        ) : (
          <div className="bg-muted-background rounded-lg border border-border overflow-hidden">
            <table className="w-full">
              <thead className="bg-background border-b border-border">
                <tr>
                  <th className="px-6 py-3 text-left text-sm font-semibold text-foreground">
                    Email
                  </th>
                  <th className="px-6 py-3 text-left text-sm font-semibold text-foreground">
                    Name
                  </th>
                  <th className="px-6 py-3 text-left text-sm font-semibold text-foreground">
                    Role
                  </th>
                  <th className="px-6 py-3 text-left text-sm font-semibold text-foreground">
                    Status
                  </th>
                  <th className="px-6 py-3 text-left text-sm font-semibold text-foreground">
                    Last Login
                  </th>
                  <th className="px-6 py-3 text-left text-sm font-semibold text-foreground">
                    Actions
                  </th>
                </tr>
              </thead>
              <tbody className="divide-y divide-border">
                {filteredUsers.map((user) => (
                  <tr key={user.id} className="hover:bg-background/50 transition-colors">
                    <td className="px-6 py-4 text-foreground">{user.email}</td>
                    <td className="px-6 py-4 text-foreground/80">{user.display_name}</td>
                    <td className="px-6 py-4">
                      <select
                        value={user.role}
                        onChange={(e) => handleUpdateRole(user.id, e.target.value)}
                        className="h-8 px-2 py-1 rounded bg-background border border-border text-foreground text-sm focus:outline-none focus:ring-2 focus:ring-primary transition-colors"
                        disabled={loading}
                      >
                        <option value="admin">Admin</option>
                        <option value="analyst">Analyst</option>
                        <option value="viewer">Viewer</option>
                      </select>
                    </td>
                    <td className="px-6 py-4">
                      <span
                        className={`px-2 py-1 rounded text-xs font-medium ${
                          user.status === 'active'
                            ? 'bg-status-success/20 text-status-success'
                            : 'bg-status-error/20 text-status-error'
                        }`}
                      >
                        {user.status}
                      </span>
                    </td>
                    <td className="px-6 py-4 text-muted-foreground text-sm">
                      {user.last_login_at
                        ? new Date(user.last_login_at).toLocaleDateString()
                        : 'Never'}
                    </td>
                    <td className="px-6 py-4 space-x-2 flex flex-wrap">
                      <button
                        onClick={() =>
                          handleResetPassword(user.id, user.email)
                        }
                        className="h-8 px-2 text-xs bg-primary hover:bg-primary/90 text-foreground rounded transition-colors duration-200"
                        disabled={loading}
                      >
                        Reset PW
                      </button>
                      <button
                        onClick={() =>
                          handleSuspendUser(
                            user.id,
                            user.status === 'active'
                          )
                        }
                        className={`h-8 px-2 text-xs rounded text-foreground transition-colors duration-200 ${
                          user.status === 'active'
                            ? 'bg-status-error hover:bg-status-error/90'
                            : 'bg-status-success hover:bg-status-success/90'
                        }`}
                        disabled={loading}
                      >
                        {user.status === 'active' ? 'Suspend' : 'Activate'}
                      </button>
                    </td>
                  </tr>
                ))}
              </tbody>
            </table>
          </div>
        )}
      </div>
    </div>
  )
}
