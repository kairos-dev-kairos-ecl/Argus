import { useEffect, useMemo } from 'react'
import { BrowserRouter as Router, Routes, Route, Navigate } from 'react-router-dom'
import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import { MainLayout } from './layouts/MainLayout'
import { ProtectedRoute } from './components/ProtectedRoute'
import { LoginPage } from './pages/LoginPage'
import { SetupWizard } from './pages/SetupWizard'
import { DashboardPage } from './pages/DashboardPage'
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

  useEffect(() => {
    initializeStatuses()

    // Attempt silent token refresh on app load to restore session from
    // existing HttpOnly refresh cookie. Failure is expected when not logged in.
    refreshToken().catch(() => {
      // Not authenticated — ProtectedRoute will redirect to /login
    })
  }, [])

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

                    {/* Investigation */}
                    <Route path="/trace/:traceId" element={<TracePage />} />
                    <Route path="/trace" element={<TracePage />} />
                    <Route path="/query" element={<QueryPage />} />

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
