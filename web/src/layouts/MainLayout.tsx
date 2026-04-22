import React, { useState } from 'react'
import { useAuthStore } from '../stores/auth'
import { useLocation } from 'react-router-dom'
import { CommandPalette } from '../components/CommandPalette'

interface MainLayoutProps {
  children: React.ReactNode
}

/**
 * Main Layout Component
 * Provides the shell for the entire application with:
 * - Fixed header with app title, search (⌘K), notifications, user menu
 * - Collapsible sidebar with navigation sections
 * - Main content area with breadcrumbs
 * - Connection status indicator
 * - Command palette (⌘K / Ctrl+K)
 */
export const MainLayout: React.FC<MainLayoutProps> = ({ children }) => {
  const [sidebarOpen, setSidebarOpen] = useState(true)
  const { user, isAdmin, isAnalyst, isViewer } = useAuthStore()
  const location = useLocation()

  const getRoleColor = (role: string): string => {
    switch (role) {
      case 'admin':
        return 'text-destructive'
      case 'analyst':
        return 'text-status-warning'
      case 'viewer':
        return 'text-status-info'
      default:
        return 'text-muted-foreground'
    }
  }

  const getRoleLabel = (): string => {
    if (isAdmin()) return 'Admin'
    if (isAnalyst()) return 'Analyst'
    if (isViewer()) return 'Viewer'
    return 'Unauthorized'
  }

  // Get breadcrumbs from current path
  const getBreadcrumbs = () => {
    const parts = location.pathname.split('/').filter(Boolean)
    return parts.map((part, idx) => ({
      label: part.charAt(0).toUpperCase() + part.slice(1).replace('-', ' '),
      path: '/' + parts.slice(0, idx + 1).join('/'),
    }))
  }

  return (
    <div className="relative h-screen bg-background text-foreground">
      {/* Header */}
      <header className="fixed top-0 left-0 right-0 h-16 border-b border-border z-40" style={{ backgroundColor: '#1F1F23' }}>
        <div className="flex items-center justify-between h-full px-4 gap-4">
          {/* Left: Menu + Logo */}
          <div className="flex items-center gap-3 min-w-0">
            <button
              onClick={() => setSidebarOpen(!sidebarOpen)}
              className="p-2 hover:bg-border rounded-md text-muted-foreground hover:text-foreground transition-colors flex-shrink-0"
              aria-label="Toggle sidebar"
              title="Toggle sidebar (menu)"
            >
              <svg
                className="w-5 h-5"
                fill="none"
                stroke="currentColor"
                viewBox="0 0 24 24"
              >
                <path
                  strokeLinecap="round"
                  strokeLinejoin="round"
                  strokeWidth={2}
                  d="M4 6h16M4 12h16M4 18h16"
                />
              </svg>
            </button>
            <img src="/logo.png" alt="Argus XDR" className="w-8 h-8 hidden sm:block" />
            <h1 className="text-xl font-bold text-foreground hidden sm:block">Argus XDR</h1>
          </div>

          {/* Center: Command Palette */}
          <div className="flex-1 max-w-2xl">
            <button
              onClick={() => {}}
              className="w-full px-3 py-2 bg-background border border-border rounded-md text-muted-foreground hover:border-primary/50 transition-colors text-left text-sm flex items-center justify-between"
              title="⌘K or Ctrl+K to search"
            >
              <span className="flex items-center gap-2">
                <svg className="w-4 h-4" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                  <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M21 21l-6-6m2-5a7 7 0 11-14 0 7 7 0 0114 0z" />
                </svg>
                Search pages, actions...
              </span>
              <span className="text-xs px-2 py-1 bg-border rounded text-muted-foreground">⌘K</span>
            </button>
          </div>

          {/* Right: Notifications + User Menu */}
          <div className="flex items-center gap-3">
            {/* Notifications */}
            <button
              className="p-2 hover:bg-border rounded-md text-muted-foreground hover:text-foreground transition-colors relative"
              aria-label="Notifications"
              title="Notifications"
            >
              <svg className="w-5 h-5" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M15 17h5l-1.405-1.405A2.032 2.032 0 0118 14.158V11a6.002 6.002 0 00-4-5.659V5a2 2 0 10-4 0v.341C7.67 6.165 6 8.388 6 11v3.159c0 .538-.214 1.055-.595 1.436L4 17h5m6 0v1a3 3 0 11-6 0v-1m6 0H9" />
              </svg>
              <span className="absolute top-1 right-1 w-2 h-2 bg-destructive rounded-full"></span>
            </button>

            {/* User Menu */}
            <div className="flex items-center gap-2 pl-3 border-l border-border">
              {user && (
                <>
                  <div className="hidden sm:block text-right">
                    <div className="text-sm font-medium text-foreground">{user.display_name}</div>
                    <div className={`text-xs ${getRoleColor(user.role)}`}>
                      {getRoleLabel()}
                    </div>
                  </div>
                  <button
                    className="p-2 hover:bg-border rounded-md text-muted-foreground hover:text-foreground transition-colors flex-shrink-0"
                    aria-label="User menu"
                    title={user?.email || 'Profile'}
                  >
                    <svg className="w-5 h-5" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                      <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M16 7a4 4 0 11-8 0 4 4 0 018 0zM12 14a7 7 0 00-7 7h14a7 7 0 00-7-7z" />
                    </svg>
                  </button>
                </>
              )}
            </div>
          </div>
        </div>
      </header>

      {/* Sidebar */}
      <aside
        className={`fixed left-0 top-16 bottom-0 w-64 border-r border-border overflow-y-auto transition-all duration-200 z-30 ${
          sidebarOpen ? 'translate-x-0' : '-translate-x-full'
        }`}
        style={{ backgroundColor: '#1F1F23' }}
      >
        <nav className="p-4 space-y-6">
          {/* Dashboard Section */}
          <div>
            <h2 className="px-3 py-2 text-xs font-semibold text-muted-foreground uppercase tracking-wider">
              Dashboard
            </h2>
            <NavLink href="/dashboard" icon="📊" label="Overview" active={location.pathname === '/dashboard'} />
            <NavLink href="/topology" icon="🌊" label="Signal Topology" active={location.pathname === '/topology'} />
            <NavLink href="/signals" icon="📡" label="Signal Stream" active={location.pathname === '/signals'} />
            <NavLink href="/trace" icon="🔍" label="Trace View" active={location.pathname === '/trace'} />
            <NavLink href="/query" icon="💾" label="Query Interface" active={location.pathname === '/query'} />
          </div>

          {/* Operations Section */}
          <div>
            <h2 className="px-3 py-2 text-xs font-semibold text-muted-foreground uppercase tracking-wider">
              Operations
            </h2>
            <NavLink href="/incidents" icon="⚠️" label="Incidents" active={location.pathname === '/incidents'} />
            <NavLink href="/alerts" icon="🔔" label="Alerts" active={location.pathname === '/alerts'} />
            <NavLink href="/apps" icon="📱" label="Apps" active={location.pathname === '/apps'} />
          </div>

          {/* Configuration Section */}
          {(isAdmin() || isAnalyst()) && (
            <div>
              <h2 className="px-3 py-2 text-xs font-semibold text-muted-foreground uppercase tracking-wider">
                Configuration
              </h2>
              <NavLink href="/rules" icon="📋" label="Detection Rules" active={location.pathname === '/rules'} />
              <NavLink href="/connectors" icon="🔌" label="LLM Connectors" active={location.pathname === '/connectors'} />
            </div>
          )}

          {/* Admin Section */}
          {isAdmin() && (
            <div>
              <h2 className="px-3 py-2 text-xs font-semibold text-muted-foreground uppercase tracking-wider">
                Admin
              </h2>
              <NavLink href="/settings" icon="⚙️" label="Settings" active={location.pathname === '/settings'} />
              <NavLink href="/users" icon="👥" label="Users" active={location.pathname === '/users'} />
            </div>
          )}
        </nav>
      </aside>

      {/* Main content */}
      <main
        className={`absolute top-16 left-0 right-0 bottom-0 flex flex-col ${sidebarOpen ? 'ml-64' : 'ml-0'} overflow-auto transition-all duration-200`}
      >
        {/* Breadcrumbs */}
        {location.pathname !== '/' && (
          <div className="px-6 py-3 border-b border-border bg-background sticky top-0 z-10">
            <nav className="flex items-center gap-2 text-sm">
              <a href="/dashboard" className="text-muted-foreground hover:text-foreground transition-colors">
                Dashboard
              </a>
              {getBreadcrumbs().map((crumb, idx) => (
                <div key={idx} className="flex items-center gap-2">
                  <span className="text-muted-foreground">/</span>
                  <a
                    href={crumb.path}
                    className={idx === getBreadcrumbs().length - 1
                      ? 'text-foreground font-medium'
                      : 'text-muted-foreground hover:text-foreground transition-colors'
                    }
                  >
                    {crumb.label}
                  </a>
                </div>
              ))}
            </nav>
          </div>
        )}

        {/* Page content */}
        <div className="flex-1 px-6 py-6">
          {children}
        </div>
      </main>

      {/* Connection status indicator */}
      <ConnectionStatus />

      {/* Command Palette */}
      <CommandPalette />
    </div>
  )
}

/**
 * Navigation Link Component
 */
interface NavLinkProps {
  href: string
  icon: string
  label: string
  active?: boolean
}

const NavLink: React.FC<NavLinkProps> = ({ href, icon, label, active = false }) => (
  <a
    href={href}
    className={`flex items-center gap-3 px-3 py-2 text-sm rounded transition-colors ${
      active
        ? 'bg-primary/20 text-primary border-l-2 border-primary pl-2'
        : 'text-muted-foreground hover:text-foreground hover:bg-border'
    }`}
  >
    <span className="text-base">{icon}</span>
    <span>{label}</span>
  </a>
)

/**
 * Connection Status Indicator
 * Shows WebSocket connection state in bottom-right corner
 */
const ConnectionStatus: React.FC = () => {
  const [connected] = React.useState(false)

  return (
    <div className="fixed bottom-4 right-4 flex items-center gap-2 px-3 py-2 bg-muted-background rounded border border-border">
      <div
        className={`w-2 h-2 rounded-full ${
          connected ? 'bg-status-success' : 'bg-status-error'
        }`}
      />
      <span className="text-xs text-muted-foreground">
        {connected ? 'Connected' : 'Disconnected'}
      </span>
    </div>
  )
}
