import { useEffect, useMemo } from 'react'
import { BrowserRouter as Router, Routes, Route, Navigate } from 'react-router-dom'
import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import { MainLayout } from './layouts/MainLayout'
import { DashboardPage } from './pages/DashboardPage'
import { TracePage } from './pages/TracePage'
import { QueryPage } from './pages/QueryPage'
import AppsPage from './pages/AppsPage'
import ConnectorConfigPage from './pages/ConnectorConfigPage'
import SettingsPage from './pages/SettingsPage'
import IncidentsPage from './pages/IncidentsPage'
import AlertsPage from './pages/AlertsPage'
import RulesPage from './pages/RulesPage'
import { useLayerStore } from './stores/layer'
import './styles/globals.css'

/**
 * Root App Component
 * Sets up routing with React Router and initializes stores.
 * Routes include:
 * - / → DashboardPage (main dashboard)
 * - /dashboard → DashboardPage
 * - /trace/:traceId → Trace view
 * - /query → Query interface
 * - /apps → Apps management
 * - /connectors → LLM connector config
 * - /settings → System settings (tabbed)
 * - /incidents → Incidents dashboard
 * - /alerts → Alerts stream
 * - /rules → Detection rules
 */
function App() {
  const { initializeStatuses } = useLayerStore()
  const queryClient = useMemo(() => new QueryClient(), [])

  useEffect(() => {
    // Initialize layer statuses on app mount
    initializeStatuses()

    // TODO: Initialize WebSocket connection
    // TODO: Check auth token from localStorage and restore session
  }, [])

  return (
    <QueryClientProvider client={queryClient}>
      <Router>
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

          {/* Admin */}
          <Route path="/settings" element={<SettingsPage />} />

          {/* Catch-all redirect to dashboard */}
          <Route path="*" element={<Navigate to="/" replace />} />
        </Routes>
      </MainLayout>
    </Router>
    </QueryClientProvider>
  )
}

export default App
