import { useAuthStore } from '../stores/auth'

interface PermissionGateProps {
  permission?: string
  role?: 'admin' | 'analyst' | 'viewer'
  children: React.ReactNode
  fallback?: React.ReactNode
  className?: string
}

/**
 * PermissionGate Component
 * Conditionally renders content based on user permissions or role.
 *
 * Features:
 * - Permission-based access (fine-grained)
 * - Role-based access (coarse-grained)
 * - Customizable fallback content
 * - Never hides navigation (always show in UI, disable if needed)
 *
 * Usage:
 * <PermissionGate permission="rules:create">
 *   <CreateRuleButton />
 * </PermissionGate>
 */
export function PermissionGate({
  permission,
  role,
  children,
  fallback = null,
  className = '',
}: PermissionGateProps) {
  const { hasPermission, canAccess } = useAuthStore()

  // Check permission if specified
  if (permission && !hasPermission(permission)) {
    return fallback ? (
      <div className={className}>{fallback}</div>
    ) : (
      <div
        className={`p-3 rounded-lg bg-amber-900/20 border border-amber-800/50 text-amber-400/70 text-sm ${className}`}
      >
        Insufficient permissions
      </div>
    )
  }

  // Check role if specified
  if (role && !canAccess(role)) {
    return fallback ? (
      <div className={className}>{fallback}</div>
    ) : (
      <div
        className={`p-3 rounded-lg bg-amber-900/20 border border-amber-800/50 text-amber-400/70 text-sm ${className}`}
      >
        This action requires {role} or higher privileges
      </div>
    )
  }

  // All checks passed
  return <div className={className}>{children}</div>
}

/**
 * Variant for inline buttons/controls
 * Shows disabled state instead of placeholder
 */
export function PermissionGateButton({
  permission,
  role,
  children,
  disabled = false,
  title,
  ...props
}: React.ButtonHTMLAttributes<HTMLButtonElement> & {
  permission?: string
  role?: 'admin' | 'analyst' | 'viewer'
}) {
  const { hasPermission, canAccess } = useAuthStore()

  let disabledReason = ''
  let isDenied = false

  if (permission && !hasPermission(permission)) {
    isDenied = true
    disabledReason = 'Insufficient permissions'
  }

  if (role && !canAccess(role)) {
    isDenied = true
    disabledReason = `Requires ${role} or higher privileges`
  }

  return (
    <button
      {...props}
      disabled={disabled || isDenied}
      title={isDenied ? disabledReason : title}
      className={`${props.className || ''} ${
        isDenied ? 'opacity-50 cursor-not-allowed' : ''
      }`}
    >
      {children}
    </button>
  )
}
