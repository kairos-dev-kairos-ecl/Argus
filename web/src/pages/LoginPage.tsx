import { useState } from 'react'
import { useNavigate, useLocation } from 'react-router-dom'
import { useAuthStore } from '../stores/auth'

/**
 * LoginPage Component
 * Handles user authentication with email and password.
 *
 * Features:
 * - Email + password form
 * - Real-time validation
 * - Error message display
 * - Auto-redirect to original location after successful login
 * - Support for "remember me" (optional in v1)
 */
export function LoginPage() {
  const navigate = useNavigate()
  useLocation() // Needed for redirect after login
  const { login, error, clearError, loading } = useAuthStore()

  const [email, setEmail] = useState('')
  const [password, setPassword] = useState('')
  const [localError, setLocalError] = useState<string | null>(null)

  const from = sessionStorage.getItem('redirectAfterLogin') || '/'

  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault()
    setLocalError(null)
    clearError()

    // Validation
    if (!email || !password) {
      setLocalError('Email and password are required')
      return
    }

    if (!email.includes('@')) {
      setLocalError('Please enter a valid email address')
      return
    }

    try {
      await login(email, password)
      // Clear saved redirect location
      sessionStorage.removeItem('redirectAfterLogin')
      // Redirect to original location or dashboard
      navigate(from, { replace: true })
    } catch (err) {
      const message = err instanceof Error ? err.message : 'Login failed'
      setLocalError(message)
    }
  }

  const displayError = localError || error

  return (
    <div className="min-h-screen bg-background flex items-center justify-center p-4">
      <div className="w-full max-w-md">
        {/* Header */}
        <div className="text-center mb-8">
          <h1 className="text-3xl font-bold text-foreground mb-2">Argus XDR</h1>
          <p className="text-muted-foreground">Sign in to your account</p>
        </div>

        {/* Login Card */}
        <div className="bg-muted-background rounded-lg border border-border shadow-lg p-6 md:p-8">
          <form onSubmit={handleSubmit} className="space-y-6">
            {/* Error Alert */}
            {displayError && (
              <div className="bg-status-error/10 border border-status-error/30 rounded-lg p-4">
                <p className="text-status-error text-sm">{displayError}</p>
              </div>
            )}

            {/* Email Field */}
            <div>
              <label
                htmlFor="email"
                className="block text-sm font-medium text-foreground mb-2"
              >
                Email Address
              </label>
              <input
                id="email"
                type="email"
                value={email}
                onChange={(e) => setEmail(e.target.value)}
                placeholder="you@example.com"
                className="w-full px-4 py-2 h-11 rounded-lg bg-background border border-border text-foreground placeholder-muted-foreground focus:outline-none focus:ring-2 focus:ring-primary focus:border-transparent transition-colors duration-200"
                disabled={loading}
                aria-label="Email Address"
              />
            </div>

            {/* Password Field */}
            <div>
              <label
                htmlFor="password"
                className="block text-sm font-medium text-foreground mb-2"
              >
                Password
              </label>
              <input
                id="password"
                type="password"
                value={password}
                onChange={(e) => setPassword(e.target.value)}
                placeholder="••••••••"
                className="w-full px-4 py-2 h-11 rounded-lg bg-background border border-border text-foreground placeholder-muted-foreground focus:outline-none focus:ring-2 focus:ring-primary focus:border-transparent transition-colors duration-200"
                disabled={loading}
                aria-label="Password"
              />
              <p className="text-xs text-muted-foreground mt-1">
                At least 12 characters
              </p>
            </div>

            {/* Submit Button */}
            <button
              type="submit"
              disabled={loading || !email || !password}
              className="w-full h-11 px-4 bg-primary hover:bg-primary/90 disabled:bg-primary/50 text-foreground font-medium rounded-lg transition-colors duration-200"
              aria-label={loading ? 'Signing in' : 'Sign In'}
            >
              {loading ? 'Signing in...' : 'Sign In'}
            </button>
          </form>

          {/* Footer Links */}
          <div className="mt-6 text-center text-sm">
            <a
              href="/forgot-password"
              className="text-muted-foreground hover:text-foreground underline transition-colors duration-200"
            >
              Forgot password?
            </a>
          </div>
        </div>

        {/* Info Text */}
        <p className="text-center text-muted-foreground text-sm mt-6">
          First time? Run the{' '}
          <a href="/setup" className="text-primary hover:text-primary/80 transition-colors duration-200">
            setup wizard
          </a>
        </p>
      </div>
    </div>
  )
}
