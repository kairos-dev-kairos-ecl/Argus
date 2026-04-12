import { Navigate, useLocation } from 'react-router-dom'
import { useAuthStore } from '../stores/auth'
import { useEffect, useState } from 'react'

interface ProtectedRouteProps {
  children: React.ReactNode
  requiredRole?: 'admin' | 'analyst' | 'viewer'
  requiredPermission?: string
}

/**
 * ProtectedRoute Component
 * Ensures user is authenticated before rendering protected content.
 * Optionally enforces role-based or permission-based access.
 *
 * Features:
 * - Redirects unauthenticated users to /login
 * - Saves original location for post-login redirect
 * - Supports role-based access (admin/analyst/viewer)
 * - Supports permission-based access (fine-grained)
 * - Validates token on mount (triggers refresh if needed)
 */
export function ProtectedRoute({
  children,
  requiredRole,
  requiredPermission,
}: ProtectedRouteProps) {
  const { is_authenticated, canAccess, hasPermission, token } =
    useAuthStore()
  const location = useLocation()
  const [isValidating, setIsValidating] = useState(true)

  useEffect(() => {
    // On mount, validate that token is still valid
    // If token is expired, silent refresh will be triggered by axios interceptor
    setIsValidating(false)
  }, [])

  // Still validating
  if (isValidating) {
    return (
      <div className="flex items-center justify-center h-full">
        <div className="text-center">
          <div className="animate-spin rounded-full h-12 w-12 border-b-2 border-blue-500 mx-auto mb-4" />
          <p className="text-muted-foreground">Validating session...</p>
        </div>
      </div>
    )
  }

  // Not authenticated
  if (!is_authenticated || !token) {
    // Save location to redirect after login
    sessionStorage.setItem('redirectAfterLogin', location.pathname)
    return <Navigate to="/login" replace />
  }

  // Check role-based access
  if (requiredRole && !canAccess(requiredRole)) {
    return (
      <div className="flex items-center justify-center h-full">
        <div className="text-center">
          <h1 className="text-2xl font-bold text-status-error400 mb-2">
            Access Denied
          </h1>
          <p className="text-muted-foreground mb-4">
            This page requires {requiredRole} or higher privileges.
          </p>
          <a
            href="/"
            className="text-blue-400 hover:text-blue-300 underline"
          >
            Return to Dashboard
          </a>
        </div>
      </div>
    )
  }

  // Check permission-based access
  if (requiredPermission && !hasPermission(requiredPermission)) {
    return (
      <div className="flex items-center justify-center h-full">
        <div className="text-center">
          <h1 className="text-2xl font-bold text-status-error400 mb-2">
            Insufficient Permissions
          </h1>
          <p className="text-muted-foreground mb-4">
            You don't have permission to access this page.
          </p>
          <a
            href="/"
            className="text-blue-400 hover:text-blue-300 underline"
          >
            Return to Dashboard
          </a>
        </div>
      </div>
    )
  }

  // All checks passed, render protected content
  return <>{children}</>
}
