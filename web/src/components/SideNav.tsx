import { NavLink } from 'react-router-dom'
import { useAuthStore } from '../stores/auth'

interface NavItem {
  label: string
  path: string
  adminOnly: boolean
}

const NAV_ITEMS: NavItem[] = [
  { label: 'TELEMETRY', path: '/dashboard', adminOnly: false },
  { label: 'TRACES',    path: '/trace',     adminOnly: false },
  { label: 'HUNTING',   path: '/query',     adminOnly: false },
  { label: 'AUDIT',     path: '/audit',     adminOnly: true  },
  { label: 'INCIDENTS', path: '/incidents', adminOnly: false },
  { label: 'KAIROS',    path: '/kairos',    adminOnly: false },
  { label: 'IAM',       path: '/users',     adminOnly: true  },
  { label: 'SETTINGS',  path: '/settings',  adminOnly: false },
]

export function SideNav() {
  const { user, isAdmin, logout } = useAuthStore()

  const visibleItems = NAV_ITEMS.filter(item => !item.adminOnly || isAdmin())

  return (
    <div
      style={{
        width: '240px',
        minHeight: '100vh',
        display: 'flex',
        flexDirection: 'column',
        background: 'var(--color-surface)',
        borderRight: 'var(--border-stark)',
        fontFamily: 'var(--font-mono)',
        flexShrink: 0,
      }}
    >
      {/* Brand header */}
      <div
        style={{
          padding: '20px 16px',
          borderBottom: 'var(--border-stark)',
          fontFamily: 'var(--font-display)',
          color: 'var(--color-primary)',
          fontSize: '14px',
          letterSpacing: '0.1em',
          fontWeight: 700,
        }}
      >
        ARGUS_XDR
      </div>

      {/* Nav items */}
      <nav style={{ flex: 1, paddingTop: '8px' }}>
        {visibleItems.map(item => (
          <NavLink
            key={item.path}
            to={item.path}
            style={({ isActive }) => ({
              display: 'block',
              padding: '10px 16px',
              fontSize: '12px',
              textTransform: 'uppercase' as const,
              letterSpacing: '0.08em',
              textDecoration: 'none',
              color: isActive ? 'var(--color-primary)' : 'var(--color-text)',
              background: isActive ? 'var(--color-background)' : 'transparent',
              borderLeft: isActive
                ? '2px solid var(--color-primary)'
                : '2px solid transparent',
            })}
          >
            {item.label}
          </NavLink>
        ))}
      </nav>

      {/* Footer */}
      {user && (
        <div
          style={{
            borderTop: 'var(--border-stark)',
            padding: '12px 16px',
            fontSize: '11px',
            color: 'var(--color-muted)',
          }}
        >
          <div style={{ marginBottom: '2px', overflow: 'hidden', textOverflow: 'ellipsis', whiteSpace: 'nowrap' }}>
            {user.email}
          </div>
          <div style={{ marginBottom: '8px' }}>
            {user.role.toUpperCase()}
          </div>
          <button
            onClick={() => logout()}
            style={{
              background: 'transparent',
              border: 'var(--border-stark)',
              color: 'var(--color-text)',
              fontFamily: 'var(--font-mono)',
              fontSize: '10px',
              letterSpacing: '0.08em',
              padding: '4px 8px',
              cursor: 'pointer',
              textTransform: 'uppercase' as const,
            }}
          >
            SIGN OUT
          </button>
        </div>
      )}
    </div>
  )
}
