import { useEffect, useMemo, useState } from 'react'
import { BrowserRouter as Router, Routes, Route, Navigate } from 'react-router-dom'
import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import { MainLayout } from './layouts/MainLayout'
import { ProtectedRoute } from './components/ProtectedRoute'
import { LoginPage } from './pages/LoginPage'
import { SetupWizard } from './pages/SetupWizard'
import { DashboardPage } from './pages/DashboardPage'
import { SignalTopologyPage } from './pages/SignalTopologyPage'
import { TracePage } from './pages/TracePage'
import { QueryPage } from './pages/QueryPage'
import AppsPage from './pages/AppsPage'
import ConnectorConfigPage from './pages/ConnectorConfigPage'
import SettingsPage from './pages/SettingsPage'
import IncidentsPage from './pages/IncidentsPage'
import AlertsPage from './pages/AlertsPage'
import RulesPage from './pages/RulesPage'
import { UsersPage } from './pages/UsersPage'
import { AuditLogPage } from './pages/AuditLogPage'
import { KairosPage } from './pages/KairosPage'
import { ProfilePage } from './pages/ProfilePage'
import { ConfigPage } from './pages/ConfigPage'
import { useLayerStore } from './stores/layer'
import { useAuthStore } from './stores/auth'
import './styles/globals.css'

/**
 * Root App Component
 * Sets up routing with React Router and initializes stores.
 *
 * Route structure:
 *   Public (no auth required):
 *     /login      → LoginPage
 *     /setup      → SetupWizard
 *
 *   Protected (require authentication via ProtectedRoute):
 *     /           → DashboardPage
 *     /dashboard  → DashboardPage
 *     /trace/:id  → TracePage
 *     /query      → QueryPage
 *     /incidents  → IncidentsPage
 *     /alerts     → AlertsPage
 *     /apps       → AppsPage
 *     /connectors → ConnectorConfigPage
 *     /rules      → RulesPage
 *     /settings   → SettingsPage
 *     /config     → ConfigPage
 *     /profile    → ProfilePage
 *     /users      → UsersPage (admin only)
 *     /audit      → AuditLogPage (admin only)
 *     *           → redirect to /
 */
function App() {
  const { initializeStatuses } = useLayerStore()
  const { refreshToken } = useAuthStore()
  const queryClient = useMemo(() => new QueryClient(), [])
  const [isInitialized, setIsInitialized] = useState(false)

  useEffect(() => {
    const initialize = async () => {
      initializeStatuses()

      // Attempt silent token refresh on app load to restore session from
      // existing HttpOnly refresh cookie. Failure is expected when not logged in.
      try {
        await refreshToken()
      } catch (error) {
        // Not authenticated — ProtectedRoute will redirect to /login
      } finally {
        // Mark initialization complete - allow routes to render
        setIsInitialized(true)
      }
    }

    initialize()
  }, [initializeStatuses, refreshToken])

  // Show loading screen while initializing
  if (!isInitialized) {
    return (
      <div className="flex items-center justify-center h-screen bg-background">
        <div className="text-center">
          <div className="animate-spin rounded-full h-12 w-12 border-b-2 border-blue-500 mx-auto mb-4" />
          <p className="text-muted-foreground">Initializing application...</p>
        </div>
      </div>
    )
  }

  return (
    <QueryClientProvider client={queryClient}>
      <Router>
        <Routes>
          {/* ── Public routes (no layout, no auth guard) ── */}
          <Route path="/login" element={<LoginPage />} />
          <Route path="/setup" element={<SetupWizard />} />

          {/* ── Protected routes (auth guard + app shell) ── */}
          <Route
            path="/*"
            element={
              <ProtectedRoute>
                <MainLayout>
                  <Routes>
                    {/* Dashboard */}
                    <Route path="/" element={<DashboardPage />} />
                    <Route path="/dashboard" element={<DashboardPage />} />
                    <Route path="/topology" element={<SignalTopologyPage />} />

                    {/* Investigation */}
                    <Route path="/trace/:traceId" element={<TracePage />} />
                    <Route path="/trace" element={<TracePage />} />
                    <Route path="/query" element={<QueryPage />} />

                    {/* Kairos */}
                    <Route path="/kairos" element={<KairosPage />} />

                    {/* Operations */}
                    <Route path="/incidents" element={<IncidentsPage />} />
                    <Route path="/alerts" element={<AlertsPage />} />

                    {/* Configuration */}
                    <Route path="/apps" element={<AppsPage />} />
                    <Route path="/connectors" element={<ConnectorConfigPage />} />
                    <Route path="/connectors/:appId" element={<ConnectorConfigPage />} />
                    <Route path="/rules" element={<RulesPage />} />
                    <Route path="/config" element={<ConfigPage />} />
                    <Route path="/settings" element={<SettingsPage />} />

                    {/* Account */}
                    <Route path="/profile" element={<ProfilePage />} />

                    {/* Admin */}
                    <Route
                      path="/users"
                      element={
                        <ProtectedRoute requiredRole="admin">
                          <UsersPage />
                        </ProtectedRoute>
                      }
                    />
                    <Route
                      path="/audit"
                      element={
                        <ProtectedRoute requiredRole="admin">
                          <AuditLogPage />
                        </ProtectedRoute>
                      }
                    />

                    {/* Catch-all → dashboard */}
                    <Route path="*" element={<Navigate to="/" replace />} />
                  </Routes>
                </MainLayout>
              </ProtectedRoute>
            }
          />
        </Routes>
      </Router>
    </QueryClientProvider>
  )
}

export default App
