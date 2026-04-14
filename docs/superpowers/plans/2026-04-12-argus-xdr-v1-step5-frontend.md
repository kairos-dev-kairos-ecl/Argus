# Argus XDR v1.0 — Step 5: React+TS Dashboard

**Date:** 2026-04-12
**Status:** Pending
**Worktree:** `C:/Users/Drupad/ArgusXDR/.worktrees/step2-detection-engine`
**Author:** Plan generated for frontend-only coding agent

---

## Goal

Bring the React+TS SPA to a fully navigable, functionally wired state. Every primary route must load without console errors. WebSocket signal streaming must be active. Authentication must gate all protected pages.

## Gate (Definition of Done)

```
npm run build    # exits 0, zero TypeScript errors
```

Manual browser checklist (all must pass):
- [ ] `/login` loads the login form
- [ ] Unauthenticated visit to `/dashboard` redirects to `/login`
- [ ] Login with valid credentials → JWT stored in memory → redirected to `/dashboard`
- [ ] `/dashboard` shows coverage map + signal stream + two ECharts charts
- [ ] WebSocket connection indicator in bottom-right turns green
- [ ] `/alerts` loads, rows render, "Acknowledge" button visible
- [ ] `/incidents` loads, status update modal works
- [ ] `/rules` loads
- [ ] `/query` loads with SQL editor
- [ ] `/settings` → 5 tabs render including "Routing Rules"
- [ ] Command palette (`Cmd+K` / `Ctrl+K`) opens
- [ ] No uncaught console errors on any of the above pages

---

## Architecture Note (Read This Before Writing Any Code)

This SPA is at `web/` in the worktree. It uses React 19, Vite 8, TypeScript 6, Tailwind CSS v4, and Zustand v5.

**Critical conventions:**
- All HTTP calls go through `web/src/lib/axios-client.ts`. Never use raw `axios` or `fetch` for new API calls.
- All WebSocket usage goes through `web/src/lib/websocket.ts` (`WebSocketClient` class). Never instantiate `new WebSocket()` directly.
- Design tokens live in `web/src/lib/design-tokens.ts`. Tailwind is wired to consume them. Use the token class names (e.g., `text-foreground`, `bg-background`, `border-border`) not raw hex values.
- ECharts is the only charting library. Use `echarts-for-react` (`ReactECharts` component). Plotly and Chart.js are banned.
- State: Zustand only. No new state libraries.
- Zustand stores: `web/src/stores/auth.ts`, `signal.ts`, `layer.ts`, `traceViewStore.ts`.

**File layout:**
```
web/src/
  App.tsx                  ← router, QueryClientProvider
  main.tsx                 ← React entry point
  pages/                   ← one file per route
    SettingsPages/         ← sub-components for settings tabs
  components/              ← shared components
  hooks/                   ← React hooks
  stores/                  ← Zustand stores
  lib/                     ← axios-client, websocket, design-tokens
  layouts/
    MainLayout.tsx         ← header + sidebar + breadcrumbs + CommandPalette
  types/
    index.ts               ← all shared TypeScript interfaces
```

**Backend proxy:** Vite dev server (`web/vite.config.ts`) proxies `/api/*` and `/v1/*` to `http://localhost:8080`. In production, nginx routes the same prefixes. Never hardcode `localhost:8080` in frontend code.

**Auth architecture:** Access token is stored in Zustand memory (`useAuthStore().token`). `web/src/lib/axios-client.ts` injects `Authorization: Bearer <token>` on every request and handles 401 → silent refresh via `POST /api/v1/auth/refresh` (HttpOnly cookie). Do not re-implement this logic.

**Known broken pattern:** `MainLayout.tsx` currently wraps ALL routes including `/login`. This causes the sidebar/header to appear on the login page. Task 3 fixes this by moving `MainLayout` inside `ProtectedRoute` scope.

---

## Task 1: Install Dependencies + shadcn/ui Setup

**Estimated time:** 15 minutes

### What to do

The `node_modules` directory does not exist. The `shadcn-ui` package in `dependencies` is the CLI runner — it does NOT install Radix UI primitives. Those must be installed separately.

**Step 1.1 — Run npm install**

```bash
cd C:/Users/Drupad/ArgusXDR/.worktrees/step2-detection-engine/web
npm install
```

This installs all existing `package.json` dependencies including React 19, Vite 8, Zustand 5, TanStack Query 5, ECharts 6, CodeMirror 6, etc.

**Step 1.2 — Install Radix UI primitives + utility libs**

```bash
cd C:/Users/Drupad/ArgusXDR/.worktrees/step2-detection-engine/web
npm install \
  @radix-ui/react-dialog \
  @radix-ui/react-dropdown-menu \
  @radix-ui/react-popover \
  @radix-ui/react-select \
  @radix-ui/react-separator \
  @radix-ui/react-slot \
  @radix-ui/react-tabs \
  @radix-ui/react-tooltip \
  class-variance-authority \
  clsx \
  tailwind-merge \
  lucide-react
```

Do NOT run `npx shadcn-ui init` — it would overwrite `tailwind.config.js` and break the existing design token wiring.

**Step 1.3 — Create `web/components.json`**

This file tells the shadcn/ui CLI where to place generated components (used for future `npx shadcn add` calls). It does not affect the build.

Create `C:/Users/Drupad/ArgusXDR/.worktrees/step2-detection-engine/web/components.json`:

```json
{
  "$schema": "https://ui.shadcn.com/schema.json",
  "style": "default",
  "rsc": false,
  "tsx": true,
  "tailwind": {
    "config": "tailwind.config.js",
    "css": "src/index.css",
    "baseColor": "slate",
    "cssVariables": false,
    "prefix": ""
  },
  "aliases": {
    "components": "@/components",
    "utils": "@/lib/utils",
    "ui": "@/components/ui",
    "lib": "@/lib",
    "hooks": "@/hooks"
  },
  "iconLibrary": "lucide"
}
```

**Step 1.4 — Create `web/src/lib/utils.ts`**

This is the `cn()` helper used by all shadcn/ui components. It does not exist yet.

Create `C:/Users/Drupad/ArgusXDR/.worktrees/step2-detection-engine/web/src/lib/utils.ts`:

```typescript
import { type ClassValue, clsx } from 'clsx'
import { twMerge } from 'tailwind-merge'

/**
 * Utility to merge Tailwind class names safely.
 * Resolves conflicting classes (e.g., p-4 + p-2 → p-2).
 */
export function cn(...inputs: ClassValue[]) {
  return twMerge(clsx(inputs))
}
```

**Step 1.5 — Create `web/src/components/ui/button.tsx`**

This is the minimal Button primitive needed for internal use. It uses Argus design tokens, not shadcn defaults.

Create `C:/Users/Drupad/ArgusXDR/.worktrees/step2-detection-engine/web/src/components/ui/button.tsx`:

```typescript
import * as React from 'react'
import { Slot } from '@radix-ui/react-slot'
import { cva, type VariantProps } from 'class-variance-authority'
import { cn } from '../../lib/utils'

const buttonVariants = cva(
  'inline-flex items-center justify-center gap-2 whitespace-nowrap rounded text-sm font-medium transition-colors focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-primary focus-visible:ring-offset-2 disabled:pointer-events-none disabled:opacity-50',
  {
    variants: {
      variant: {
        default:
          'bg-primary text-white hover:bg-primary/90',
        destructive:
          'bg-status-error text-white hover:bg-status-error/90',
        outline:
          'border border-border bg-transparent text-foreground hover:bg-border/50',
        secondary:
          'bg-border text-foreground hover:bg-border/80',
        ghost:
          'text-muted-foreground hover:bg-border hover:text-foreground',
        link:
          'text-primary underline-offset-4 hover:underline',
      },
      size: {
        default: 'h-9 px-4 py-2',
        sm: 'h-8 rounded px-3 text-xs',
        lg: 'h-11 rounded px-8',
        icon: 'h-9 w-9',
      },
    },
    defaultVariants: {
      variant: 'default',
      size: 'default',
    },
  }
)

export interface ButtonProps
  extends React.ButtonHTMLAttributes<HTMLButtonElement>,
    VariantProps<typeof buttonVariants> {
  asChild?: boolean
}

const Button = React.forwardRef<HTMLButtonElement, ButtonProps>(
  ({ className, variant, size, asChild = false, ...props }, ref) => {
    const Comp = asChild ? Slot : 'button'
    return (
      <Comp
        className={cn(buttonVariants({ variant, size, className }))}
        ref={ref}
        {...props}
      />
    )
  }
)
Button.displayName = 'Button'

export { Button, buttonVariants }
```

### Verification

```bash
cd C:/Users/Drupad/ArgusXDR/.worktrees/step2-detection-engine/web
npm run build
```

Expected: exits 0. If there are TypeScript errors in existing files (unrelated to Task 1), note them — they will be fixed in subsequent tasks.

### Commit message

```
feat(web): install dependencies + shadcn/ui primitives for Step 5
```

---

## Task 2: Fix API Endpoint Mismatches

**Estimated time:** 20 minutes

Three files contain API calls to endpoints that do not exist on the backend. These cause silent 404s (or 405s) that pollute the console and break functionality.

### 2A — Fix `AlertsPage.tsx`: suppress → acknowledge

**File:** `web/src/pages/AlertsPage.tsx`

The `AlertDetailModal` sub-component calls `POST /api/v1/alerts/:id/suppress`. The backend only has `POST /api/v1/alerts/:id/acknowledge`.

**Change 1:** Replace the API call and rename the handler.

In `AlertDetailModal`, find and replace:

```typescript
// BEFORE (lines ~197-209)
const [isSuppressing, setIsSuppressing] = useState(false)

const handleSuppress = async () => {
  setIsSuppressing(true)
  try {
    await axios.post(`/api/v1/alerts/${alert.id}/suppress`)
    onClose()
  } catch (error) {
    console.error('Failed to suppress alert:', error)
  } finally {
    setIsSuppressing(false)
  }
}
```

Replace with:

```typescript
const [isAcknowledging, setIsAcknowledging] = useState(false)

const handleAcknowledge = async () => {
  setIsAcknowledging(true)
  try {
    // Use axios-client (auto-injects JWT)
    const { apiClient } = await import('../lib/axios-client')
    await apiClient.post(`/api/v1/alerts/${alert.id}/acknowledge`)
    onClose()
  } catch (error) {
    console.error('Failed to acknowledge alert:', error)
  } finally {
    setIsAcknowledging(false)
  }
}
```

**Change 2:** Update the button JSX in `AlertDetailModal`.

Find:
```typescript
{alert.status !== 'suppressed' && (
  <button
    onClick={handleSuppress}
    disabled={isSuppressing}
    className="flex-1 px-4 py-2 bg-status-warning/20 text-status-warning rounded hover:opacity-90 disabled:opacity-50 transition-opacity font-medium"
  >
    {isSuppressing ? 'Suppressing...' : 'Suppress Similar'}
  </button>
)}
```

Replace with:
```typescript
{alert.status !== 'acknowledged' && (
  <button
    onClick={handleAcknowledge}
    disabled={isAcknowledging}
    className="flex-1 px-4 py-2 bg-status-warning/20 text-status-warning rounded hover:opacity-90 disabled:opacity-50 transition-opacity font-medium"
  >
    {isAcknowledging ? 'Acknowledging...' : 'Acknowledge'}
  </button>
)}
```

**Change 3:** Also update the status filter in `AlertsPage` to remove `'suppressed'` option (it's no longer a valid backend status) and update the Alert interface `status` field:

In the `Alert` interface at the top of the file, change:
```typescript
status: 'delivered' | 'acknowledged' | 'suppressed'
```
to:
```typescript
status: 'delivered' | 'acknowledged'
```

In the status filter `<select>`, remove the `<option value="suppressed">Suppressed</option>` line.

**Change 4:** Remove the direct `axios` import from `AlertsPage.tsx` top-level (the `AlertDetailModal` will use `apiClient` instead). Keep `import axios from 'axios'` only if other calls in the file still use it (the `useQuery` calls use axios directly — see note below).

> **Note on the `useQuery` calls:** The `useQuery` functions in `AlertsPage.tsx` use `axios.get(...)` directly. This is existing code — do not change it in this task to avoid scope creep. The existing axios instance does not auto-inject JWT, which is a pre-existing gap. Leave the `useQuery` calls as-is; the Task 3 auth guard will ensure users are authenticated before reaching this page.

### 2B — Fix `IncidentsPage.tsx`: PATCH → conditional POST

**File:** `web/src/pages/IncidentsPage.tsx`

The `handleStatusUpdate` function calls `PATCH /api/v1/incidents/:id`. The backend only has:
- `POST /api/v1/incidents/:id/acknowledge`
- `POST /api/v1/incidents/:id/resolve`

Find (lines ~181-194):
```typescript
const handleStatusUpdate = async () => {
  setIsSaving(true)
  try {
    await axios.patch(`/api/v1/incidents/${incident.id}`, {
      status,
      note: note || undefined,
    })
    onClose()
  } catch (error) {
    console.error('Failed to update incident:', error)
  } finally {
    setIsSaving(false)
  }
}
```

Replace with:
```typescript
const handleStatusUpdate = async () => {
  setIsSaving(true)
  try {
    const { apiClient } = await import('../lib/axios-client')
    if (status === 'acknowledged') {
      await apiClient.post(`/api/v1/incidents/${incident.id}/acknowledge`, {
        note: note || undefined,
      })
    } else if (status === 'resolved') {
      await apiClient.post(`/api/v1/incidents/${incident.id}/resolve`, {
        note: note || undefined,
      })
    }
    // 'open' status change is not supported by backend — no-op
    onClose()
  } catch (error) {
    console.error('Failed to update incident:', error)
  } finally {
    setIsSaving(false)
  }
}
```

Also remove `import axios from 'axios'` from the top of `IncidentsPage.tsx` if it is no longer used elsewhere in the file (the `useQuery` calls use it — keep it if so).

### 2C — Enable layer status polling in `useCoverageMap.ts`

**File:** `web/src/hooks/useCoverageMap.ts`

The hook has the correct endpoint (`/api/v1/layers/status`) but the polling code is commented out with a TODO. The backend now implements this endpoint. Uncomment the polling logic.

Replace the entire `useEffect` body:

```typescript
useEffect(() => {
  // Initial load on component mount
  fetchLayerStatus()

  // Poll every 30 seconds
  const interval = setInterval(() => {
    fetchLayerStatus()
  }, 30000)

  return () => {
    clearInterval(interval)
  }
}, [])
```

The `fetchLayerStatus` function already uses native `fetch('/api/v1/layers/status')` — this is acceptable since it does not require auth headers (it's a health endpoint). Leave it as-is.

### Verification

```bash
cd C:/Users/Drupad/ArgusXDR/.worktrees/step2-detection-engine/web
npm run build
# Should still exit 0
```

In browser (with backend running): open `/alerts` → the "Acknowledge" button appears. Click it → no 404 in Network tab.

### Commit message

```
fix(web): align API calls to backend endpoints (acknowledge, incidents, layers/status)
```

---

## Task 3: Register Missing Routes + Wire ProtectedRoute

**Estimated time:** 25 minutes

### Current state

- `App.tsx` has all routes inside `<MainLayout>` with no auth guard
- `ProtectedRoute.tsx` exists and is correct but is not used
- `/login`, `/setup`, `/users`, `/audit-log`, `/profile` routes are not registered
- `MainLayout` (header + sidebar) renders even on `/login`

### What to build

Restructure `App.tsx` so that:
1. Public routes (`/login`, `/setup`) render WITHOUT `MainLayout`
2. All other routes render inside `<MainLayout>` AND inside `<ProtectedRoute>`
3. An authenticated user visiting `/login` is redirected to `/dashboard`

### New `web/src/App.tsx`

Replace the entire file with:

```typescript
import { useMemo } from 'react'
import {
  BrowserRouter as Router,
  Routes,
  Route,
  Navigate,
} from 'react-router-dom'
import { QueryClient, QueryClientProvider } from '@tanstack/react-query'

// Layouts
import { MainLayout } from './layouts/MainLayout'

// Auth guard
import { ProtectedRoute } from './components/ProtectedRoute'

// Public pages (no MainLayout, no auth required)
import { LoginPage } from './pages/LoginPage'
import SetupWizard from './pages/SetupWizard'

// Protected pages
import { DashboardPage } from './pages/DashboardPage'
import { TracePage } from './pages/TracePage'
import { QueryPage } from './pages/QueryPage'
import AppsPage from './pages/AppsPage'
import ConnectorConfigPage from './pages/ConnectorConfigPage'
import SettingsPage from './pages/SettingsPage'
import IncidentsPage from './pages/IncidentsPage'
import AlertsPage from './pages/AlertsPage'
import RulesPage from './pages/RulesPage'
import UsersPage from './pages/UsersPage'
import AuditLogPage from './pages/AuditLogPage'
import ProfilePage from './pages/ProfilePage'

import './styles/globals.css'

/**
 * AuthRedirect — redirects authenticated users away from /login
 */
import { useAuthStore } from './stores/auth'

function AuthRedirect({ children }: { children: React.ReactNode }) {
  const { is_authenticated } = useAuthStore()
  if (is_authenticated) {
    return <Navigate to="/dashboard" replace />
  }
  return <>{children}</>
}

function App() {
  const queryClient = useMemo(() => new QueryClient(), [])

  return (
    <QueryClientProvider client={queryClient}>
      <Router>
        <Routes>
          {/* ── Public routes (no layout, no auth guard) ── */}
          <Route
            path="/login"
            element={
              <AuthRedirect>
                <LoginPage />
              </AuthRedirect>
            }
          />
          <Route path="/setup" element={<SetupWizard />} />

          {/* ── Protected routes (wrapped in MainLayout + ProtectedRoute) ── */}
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

                    {/* Admin */}
                    <Route path="/settings" element={<SettingsPage />} />
                    <Route
                      path="/users"
                      element={
                        <ProtectedRoute requiredRole="admin">
                          <UsersPage />
                        </ProtectedRoute>
                      }
                    />
                    <Route
                      path="/audit-log"
                      element={
                        <ProtectedRoute requiredRole="admin">
                          <AuditLogPage />
                        </ProtectedRoute>
                      }
                    />

                    {/* Profile */}
                    <Route path="/profile" element={<ProfilePage />} />

                    {/* Catch-all */}
                    <Route path="*" element={<Navigate to="/dashboard" replace />} />
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
```

### Fix import issue: `ProtectedRoute` children typing

The current `ProtectedRoute` at `web/src/components/ProtectedRoute.tsx` uses `React.ReactNode` for `children` — this is correct. No changes needed there.

However, notice that `ProtectedRoute` currently imports `useEffect` and `useState` but the validation is instant (sets `isValidating(false)` immediately). The spinner appears for one render cycle. This is fine for Step 5.

### Handle pages that have `.bak` variants

Several pages have both a `.tsx` and a `.tsx.bak` file (e.g., `AuditLogPage.tsx` and `AuditLogPage.tsx.bak`, `ProfilePage.tsx.bak`). Use the `.tsx` version — `.bak` files are backup copies. TypeScript will only compile `.tsx` files, not `.tsx.bak`.

Check that these files exist and export a default component:
- `web/src/pages/UsersPage.tsx`
- `web/src/pages/AuditLogPage.tsx`
- `web/src/pages/ProfilePage.tsx`
- `web/src/pages/SetupWizard.tsx`

If any of these files are empty or have compilation errors unrelated to Step 5 issues, add a minimal placeholder export at the top:

```typescript
// Placeholder — full implementation in later step
export default function UsersPage() {
  return (
    <div className="space-y-6">
      <h1 className="text-3xl font-bold text-foreground">Users</h1>
      <p className="text-muted-foreground">User management — coming in Step 4.</p>
    </div>
  )
}
```

### Verification

```bash
npm run build   # exits 0
```

In browser:
- Navigate to `http://localhost:5173/dashboard` → redirects to `/login`
- Login → redirected back to `/dashboard`
- `/login` while authenticated → redirects to `/dashboard`
- Header and sidebar do NOT appear on `/login`

### Commit message

```
feat(web): register all routes + wire ProtectedRoute auth guard
```

---

## Task 4: Activate WebSocket Signal Stream

**Estimated time:** 15 minutes

### Current state

`web/src/hooks/useSignalStream.ts` has working WebSocket code but it is wrapped in a `/* comment */` block. The TODO says "endpoint not yet implemented" — but the backend `SignalBroadcaster` IS wired at `/v1/signals/stream` (Step 1/2 backend work).

### What to do

Replace the contents of `web/src/hooks/useSignalStream.ts` with the uncommented version:

```typescript
import { useEffect, useRef, useState } from 'react'
import { createWebSocketClient, WebSocketClient } from '../lib/websocket'
import { useSignalStore } from '../stores/signal'
import type { ArgusSignal } from '../types'

/**
 * useSignalStream Hook
 *
 * Manages WebSocket subscription lifecycle for real-time signal delivery.
 * Automatically connects on mount, disconnects on unmount.
 * Dispatches incoming signals to the Zustand signal store.
 *
 * Backend endpoint: WS /v1/signals/stream (SignalBroadcaster)
 * Protocol: JSON-encoded ArgusSignal messages
 */
export function useSignalStream() {
  const [isConnected, setIsConnected] = useState(false)
  const [error, setError] = useState<string | null>(null)
  const store = useSignalStore()
  const wsRef = useRef<WebSocketClient | null>(null)

  useEffect(() => {
    // Derive WebSocket URL from current page protocol
    // In dev: ws://localhost:5173/v1/signals/stream → Vite proxies to ws://localhost:8080/v1/signals/stream
    // In prod: wss://your-domain/v1/signals/stream
    const protocol = location.protocol === 'https:' ? 'wss' : 'ws'
    const host = location.host
    const wsUrl = `${protocol}://${host}/v1/signals/stream`

    const ws = createWebSocketClient(wsUrl)

    // Handle incoming signals
    const unsubscribeMessage = ws.onMessage((data) => {
      try {
        const signal = data as ArgusSignal
        if (signal && typeof signal === 'object' && 'signal_id' in signal) {
          store.addSignal(signal)
        }
      } catch (err) {
        console.error('[useSignalStream] Failed to process signal:', err)
      }
    })

    // Handle connection errors
    const unsubscribeError = ws.onError((err) => {
      console.error('[useSignalStream] WebSocket error:', err)
      setError(err)
      setIsConnected(false)
    })

    // Connect
    ws.connect()
      .then(() => {
        setIsConnected(true)
        setError(null)
        store.setSubscribed(true)
      })
      .catch((err) => {
        const errMsg = err instanceof Error ? err.message : String(err)
        console.error('[useSignalStream] Connection failed:', errMsg)
        setError(errMsg)
        setIsConnected(false)
        // Store is still subscribed=false — set it true anyway so REST fallback works
        store.setSubscribed(true)
      })

    wsRef.current = ws

    return () => {
      unsubscribeMessage()
      unsubscribeError()
      ws.disconnect()
      store.setSubscribed(false)
    }
  }, [])

  return { isConnected, error }
}
```

### Wire connection status to MainLayout

Currently `MainLayout.tsx` has a `ConnectionStatus` component that hardcodes `const [connected] = React.useState(false)`. It never reads from the signal stream hook.

Update the `ConnectionStatus` component in `web/src/layouts/MainLayout.tsx`:

```typescript
/**
 * Connection Status Indicator
 * Shows WebSocket connection state in bottom-right corner.
 * Reads from Zustand signal store (subscribed flag set by useSignalStream).
 */
const ConnectionStatus: React.FC = () => {
  const { subscribed } = useSignalStore()

  return (
    <div className="fixed bottom-4 right-4 flex items-center gap-2 px-3 py-2 bg-muted-background rounded border border-border">
      <div
        className={`w-2 h-2 rounded-full ${
          subscribed ? 'bg-status-success' : 'bg-status-error'
        }`}
      />
      <span className="text-xs text-muted-foreground">
        {subscribed ? 'Connected' : 'Disconnected'}
      </span>
    </div>
  )
}
```

Add the import at the top of `MainLayout.tsx`:
```typescript
import { useSignalStore } from '../stores/signal'
```

> **Note on Vite WS proxy:** Vite 8 does not automatically proxy WebSocket connections through the `/v1` proxy rule by default. You may need to add `ws: true` to the proxy config. Update `web/vite.config.ts`:

```typescript
import { defineConfig } from 'vite'
import react from '@vitejs/plugin-react'

export default defineConfig({
  plugins: [react()],
  server: {
    port: 5173,
    proxy: {
      '/api': 'http://localhost:8080',
      '/v1': {
        target: 'http://localhost:8080',
        ws: true,        // ← enables WebSocket proxying
        changeOrigin: true,
      },
    },
  },
  build: {
    outDir: 'dist',
    sourcemap: true,
  },
})
```

### Verification

```bash
npm run build   # exits 0
```

In browser (with backend running and sending signals):
- Bottom-right indicator turns green
- `/dashboard` signal stream populates with real-time data

If backend is not running, the indicator stays red — this is correct behavior (WebSocketClient retries automatically via exponential backoff).

### Commit message

```
feat(web): activate WebSocket signal stream + wire connection indicator
```

---

## Task 5: Dashboard ECharts Charts

**Estimated time:** 35 minutes

### Overview

Add two new chart components to the dashboard. Both use `echarts-for-react` (already installed as `echarts-for-react@^3.0.6` in `package.json`). Place them in a 2-column grid row below the signal stream in `DashboardPage.tsx`.

### 5A — Create `SignalVolumeChart.tsx`

Create `C:/Users/Drupad/ArgusXDR/.worktrees/step2-detection-engine/web/src/components/SignalVolumeChart.tsx`:

```typescript
import React, { useMemo } from 'react'
import ReactECharts from 'echarts-for-react'
import { useSignalStore } from '../stores/signal'
import type { EChartsOption } from 'echarts'

/**
 * SignalVolumeChart
 *
 * ECharts line chart showing signals per minute over the last 30 minutes.
 * Data is derived from the Zustand signal store buffer (live signals only).
 * No separate API call needed — uses signals already in memory.
 */
export const SignalVolumeChart: React.FC = () => {
  const { signals } = useSignalStore()

  // Bucket signals into 1-minute bins for the last 30 minutes
  const { labels, counts } = useMemo(() => {
    const now = Date.now()
    const BIN_COUNT = 30
    const BIN_MS = 60_000 // 1 minute

    // Initialize bins
    const bins = Array.from({ length: BIN_COUNT }, (_, i) => ({
      label: new Date(now - (BIN_COUNT - 1 - i) * BIN_MS).toLocaleTimeString(
        [],
        { hour: '2-digit', minute: '2-digit' }
      ),
      count: 0,
    }))

    // Count signals into bins
    for (const signal of signals) {
      const t = new Date(signal.timestamp).getTime()
      const ageMs = now - t
      if (ageMs < 0 || ageMs > BIN_COUNT * BIN_MS) continue
      const binIdx = BIN_COUNT - 1 - Math.floor(ageMs / BIN_MS)
      if (binIdx >= 0 && binIdx < BIN_COUNT) {
        bins[binIdx].count++
      }
    }

    return {
      labels: bins.map((b) => b.label),
      counts: bins.map((b) => b.count),
    }
  }, [signals])

  const option: EChartsOption = {
    backgroundColor: '#0A0A0B',
    grid: { left: 40, right: 16, top: 20, bottom: 40 },
    xAxis: {
      type: 'category',
      data: labels,
      axisLine: { lineStyle: { color: '#2A2A2F' } },
      axisLabel: {
        color: '#A0A0A0',
        fontSize: 10,
        interval: 4,
      },
      splitLine: { show: false },
    },
    yAxis: {
      type: 'value',
      minInterval: 1,
      axisLine: { show: false },
      axisLabel: { color: '#A0A0A0', fontSize: 10 },
      splitLine: { lineStyle: { color: '#2A2A2F', type: 'dashed' } },
    },
    series: [
      {
        type: 'line',
        data: counts,
        smooth: true,
        symbol: 'none',
        lineStyle: { color: '#3B82F6', width: 2 },
        areaStyle: {
          color: {
            type: 'linear',
            x: 0, y: 0, x2: 0, y2: 1,
            colorStops: [
              { offset: 0, color: 'rgba(59,130,246,0.3)' },
              { offset: 1, color: 'rgba(59,130,246,0.0)' },
            ],
          },
        },
      },
    ],
    tooltip: {
      trigger: 'axis',
      backgroundColor: '#1F1F23',
      borderColor: '#2A2A2F',
      textStyle: { color: '#FFFFFF', fontSize: 12 },
      formatter: (params: any) => {
        const p = Array.isArray(params) ? params[0] : params
        return `${p.axisValue}<br/>${p.value} signals`
      },
    },
  }

  return (
    <div className="bg-muted-background border border-border rounded-lg p-4">
      <h3 className="text-sm font-semibold text-foreground mb-3">
        Signal Volume (last 30 min)
      </h3>
      <ReactECharts
        option={option}
        style={{ height: '160px' }}
        theme="dark"
        notMerge
      />
    </div>
  )
}
```

### 5B — Create `SeverityDistributionChart.tsx`

Create `C:/Users/Drupad/ArgusXDR/.worktrees/step2-detection-engine/web/src/components/SeverityDistributionChart.tsx`:

```typescript
import React, { useMemo } from 'react'
import ReactECharts from 'echarts-for-react'
import { useQuery } from '@tanstack/react-query'
import axios from 'axios'
import type { EChartsOption } from 'echarts'

interface Alert {
  id: string
  severity: number | string
}

const SEVERITY_COLORS: Record<number, string> = {
  1: '#22C55E', // success green
  2: '#3B82F6', // info blue
  3: '#EAB308', // warning yellow
  4: '#F97316', // orange
  5: '#EF4444', // error red
}

const SEVERITY_LABELS: Record<number, string> = {
  1: 'Info',
  2: 'Low',
  3: 'Medium',
  4: 'High',
  5: 'Critical',
}

/**
 * SeverityDistributionChart
 *
 * ECharts bar chart showing alert counts grouped by severity level 1–5.
 * Fetches from GET /api/v1/alerts (limit 500, no time filter for overview).
 */
export const SeverityDistributionChart: React.FC = () => {
  const { data: alerts = [] } = useQuery<Alert[]>({
    queryKey: ['alerts-severity-chart'],
    queryFn: async () => {
      const res = await axios.get('/api/v1/alerts', { params: { limit: 500 } })
      return res.data.alerts || []
    },
    staleTime: 60_000,
    refetchInterval: 120_000,
  })

  const counts = useMemo(() => {
    const tally: Record<number, number> = { 1: 0, 2: 0, 3: 0, 4: 0, 5: 0 }
    for (const alert of alerts) {
      const sev = typeof alert.severity === 'string'
        ? parseInt(alert.severity, 10)
        : alert.severity
      if (sev >= 1 && sev <= 5) tally[sev]++
    }
    return tally
  }, [alerts])

  const option: EChartsOption = {
    backgroundColor: '#0A0A0B',
    grid: { left: 40, right: 16, top: 20, bottom: 40 },
    xAxis: {
      type: 'category',
      data: [1, 2, 3, 4, 5].map((s) => SEVERITY_LABELS[s]),
      axisLine: { lineStyle: { color: '#2A2A2F' } },
      axisLabel: { color: '#A0A0A0', fontSize: 11 },
      splitLine: { show: false },
    },
    yAxis: {
      type: 'value',
      minInterval: 1,
      axisLine: { show: false },
      axisLabel: { color: '#A0A0A0', fontSize: 10 },
      splitLine: { lineStyle: { color: '#2A2A2F', type: 'dashed' } },
    },
    series: [
      {
        type: 'bar',
        data: [1, 2, 3, 4, 5].map((s) => ({
          value: counts[s],
          itemStyle: { color: SEVERITY_COLORS[s], borderRadius: [4, 4, 0, 0] },
        })),
        barMaxWidth: 40,
      },
    ],
    tooltip: {
      trigger: 'axis',
      backgroundColor: '#1F1F23',
      borderColor: '#2A2A2F',
      textStyle: { color: '#FFFFFF', fontSize: 12 },
      formatter: (params: any) => {
        const p = Array.isArray(params) ? params[0] : params
        return `${p.name}<br/>${p.value} alerts`
      },
    },
  }

  return (
    <div className="bg-muted-background border border-border rounded-lg p-4">
      <h3 className="text-sm font-semibold text-foreground mb-3">
        Alerts by Severity
      </h3>
      <ReactECharts
        option={option}
        style={{ height: '160px' }}
        theme="dark"
        notMerge
      />
    </div>
  )
}
```

### 5C — Update `DashboardPage.tsx`

Replace `web/src/pages/DashboardPage.tsx` with the version that includes the charts:

```typescript
import React from 'react'
import { useSignalStream } from '../hooks/useSignalStream'
import { SignalStream } from '../components/SignalStream'
import { SignalStreamFilter } from '../components/SignalStreamFilter'
import { CoverageMap } from '../components/CoverageMap'
import { SignalVolumeChart } from '../components/SignalVolumeChart'
import { SeverityDistributionChart } from '../components/SeverityDistributionChart'

/**
 * DashboardPage
 *
 * Layout (top to bottom):
 *   1. Connection status banner
 *   2. Coverage Map (L1-L10 layer grid)
 *   3. Signal stream filter + virtual-scroll feed
 *   4. Charts row: SignalVolumeChart + SeverityDistributionChart
 */
export const DashboardPage: React.FC = () => {
  const { isConnected, error } = useSignalStream()

  return (
    <div className="flex flex-col h-full gap-4 p-4 bg-background">
      {/* Connection status banner */}
      {!isConnected && (
        <div className="bg-status-error/10 text-status-error px-4 py-2 rounded-lg border border-status-error/30">
          <div className="font-semibold text-sm">Signal stream disconnected — attempting reconnect</div>
          {error && <div className="text-xs opacity-75 mt-1 font-mono">{error}</div>}
        </div>
      )}

      {/* Coverage Map */}
      <CoverageMap />

      {/* Signal Stream with Filter */}
      <div className="flex flex-col flex-1 min-h-0">
        <SignalStreamFilter />
        <SignalStream />
      </div>

      {/* Charts row */}
      <div className="grid grid-cols-1 md:grid-cols-2 gap-4">
        <SignalVolumeChart />
        <SeverityDistributionChart />
      </div>
    </div>
  )
}
```

### Notes on ECharts types

`echarts-for-react` v3 ships its own type declarations. The `EChartsOption` type comes from the `echarts` package itself. Import it as:
```typescript
import type { EChartsOption } from 'echarts'
```
If the `echarts` types are not found, use `any` for the option object — the runtime behavior is the same.

### Verification

```bash
npm run build   # exits 0
```

In browser: `/dashboard` now shows two charts below the signal stream. Charts render even with zero data (empty bars / flat line).

### Commit message

```
feat(web): add SignalVolumeChart + SeverityDistributionChart to dashboard
```

---

## Task 6: Settings Routing Rules Tab

**Estimated time:** 15 minutes

### What to do

Add a 5th "Routing Rules" tab to `SettingsPage.tsx`. Since the backend `GET /api/v1/routing-rules` endpoint is pending (Step 4+), render a placeholder card that communicates the schema to operators.

### 6A — Create `RoutingRulesSettings.tsx`

Create `C:/Users/Drupad/ArgusXDR/.worktrees/step2-detection-engine/web/src/pages/SettingsPages/RoutingRulesSettings.tsx`:

```typescript
import React from 'react'

/**
 * RoutingRulesSettings
 *
 * Placeholder for the routing rules configuration tab.
 * Full CRUD UI will be added when POST/GET /api/v1/routing-rules
 * is implemented (Step 4+, migration 008).
 *
 * Shows the schema so operators know what to expect.
 */
export const RoutingRulesSettings: React.FC = () => {
  const schemaExample = `{
  "id": "uuid",
  "channel": "slack | pagerduty | webhook | email | syslog",
  "min_severity": 1-5,
  "app_id_filter": "app-uuid or null (all apps)",
  "layer_filter": "L1_HARDWARE...L10_SEMANTIC or null (all layers)",
  "enabled": true,
  "created_at": "ISO8601"
}`

  return (
    <div className="space-y-6">
      <div>
        <h2 className="text-2xl font-bold text-foreground">Routing Rules</h2>
        <p className="text-muted-foreground mt-1">
          Route alerts to notification channels based on severity, app, or layer.
        </p>
      </div>

      {/* Coming soon card */}
      <div className="p-6 bg-muted-background border border-border rounded-lg">
        <div className="flex items-start gap-4">
          <div className="w-10 h-10 rounded-lg bg-primary/10 flex items-center justify-center flex-shrink-0">
            <svg
              className="w-5 h-5 text-primary"
              fill="none"
              stroke="currentColor"
              viewBox="0 0 24 24"
              aria-hidden="true"
            >
              <path
                strokeLinecap="round"
                strokeLinejoin="round"
                strokeWidth={2}
                d="M9 5H7a2 2 0 00-2 2v12a2 2 0 002 2h10a2 2 0 002-2V7a2 2 0 00-2-2h-2M9 5a2 2 0 002 2h2a2 2 0 002-2M9 5a2 2 0 012-2h2a2 2 0 012 2"
              />
            </svg>
          </div>
          <div>
            <h3 className="text-lg font-semibold text-foreground">
              Configurable in the next release
            </h3>
            <p className="text-muted-foreground text-sm mt-1">
              Routing rule CRUD will be available once{' '}
              <code className="text-primary font-mono text-xs">
                GET /api/v1/routing-rules
              </code>{' '}
              is implemented. Rules are currently managed via database migration
              008.
            </p>
          </div>
        </div>
      </div>

      {/* Schema preview */}
      <div>
        <h3 className="text-sm font-semibold text-foreground mb-2">
          Routing Rule Schema (migration 008)
        </h3>
        <pre className="bg-background border border-border rounded-lg p-4 text-xs font-mono text-muted-foreground overflow-x-auto">
          {schemaExample}
        </pre>
      </div>

      {/* Current channels hint */}
      <div className="p-4 bg-muted-background border border-border rounded-lg">
        <p className="text-sm text-muted-foreground">
          <span className="font-medium text-foreground">Tip:</span> To configure
          notification destinations (Slack, PagerDuty, SMTP), use the{' '}
          <strong className="text-foreground">Notifications</strong> tab.
          Routing rules determine{' '}
          <em>which channel receives which alerts</em> based on severity and
          source filters.
        </p>
      </div>
    </div>
  )
}
```

### 6B — Update `SettingsPage.tsx`

Update `web/src/pages/SettingsPage.tsx` to add the new tab:

Change the `SettingsTab` type:
```typescript
type SettingsTab = 'general' | 'notifications' | 'retention' | 'sdk-health' | 'routing-rules'
```

Add the import:
```typescript
import { RoutingRulesSettings } from './SettingsPages/RoutingRulesSettings'
```

Add to the `tabs` array:
```typescript
{ id: 'routing-rules', label: 'Routing Rules', icon: '🔀' },
```

Add to the tab content section:
```typescript
{activeTab === 'routing-rules' && <RoutingRulesSettings />}
```

### Verification

Navigate to `/settings` → 5 tabs visible → click "Routing Rules" → placeholder card renders with schema.

### Commit message

```
feat(web): add Routing Rules placeholder tab to Settings page
```

---

## Task 7: Replace Emoji Icons + Polish

**Estimated time:** 20 minutes

### Overview

`MainLayout.tsx` uses emoji strings as icons (`'📊'`, `'🔔'`, `'⚙️'`, etc.). These have inconsistent sizing and alignment across OSes, and they block WCAG AA compliance. Replace with `lucide-react` icons (already installed in Task 1).

### 7A — Update `MainLayout.tsx` sidebar navigation

Replace the `NavLink` calls in `MainLayout.tsx` sidebar section. The `NavLink` component currently accepts `icon: string` (emoji). Change it to accept a React node.

**Update `NavLinkProps` interface:**
```typescript
interface NavLinkProps {
  href: string
  icon: React.ReactNode
  label: string
  active?: boolean
}
```

**Update `NavLink` component** (the `<span className="text-base">` becomes `<span className="w-4 h-4 flex-shrink-0">`):
```typescript
const NavLink: React.FC<NavLinkProps> = ({ href, icon, label, active = false }) => (
  <a
    href={href}
    className={`flex items-center gap-3 px-3 py-2 text-sm rounded transition-colors ${
      active
        ? 'bg-primary/20 text-primary border-l-2 border-primary pl-2'
        : 'text-muted-foreground hover:text-foreground hover:bg-border'
    }`}
  >
    <span className="w-4 h-4 flex-shrink-0">{icon}</span>
    <span>{label}</span>
  </a>
)
```

**Add lucide-react imports** at the top of `MainLayout.tsx`:
```typescript
import {
  LayoutDashboard,
  Radio,
  Search,
  Database,
  AlertTriangle,
  Bell,
  AppWindow,
  ShieldCheck,
  Plug,
  Settings,
  Users,
  BookOpen,
} from 'lucide-react'
```

**Replace emoji NavLink calls** in the sidebar section:
```typescript
{/* Dashboard Section */}
<NavLink href="/dashboard" icon={<LayoutDashboard className="w-4 h-4" />} label="Overview" active={location.pathname === '/dashboard' || location.pathname === '/'} />
<NavLink href="/trace" icon={<Search className="w-4 h-4" />} label="Trace View" active={location.pathname.startsWith('/trace')} />
<NavLink href="/query" icon={<Database className="w-4 h-4" />} label="Query Interface" active={location.pathname === '/query'} />

{/* Operations Section */}
<NavLink href="/incidents" icon={<AlertTriangle className="w-4 h-4" />} label="Incidents" active={location.pathname === '/incidents'} />
<NavLink href="/alerts" icon={<Bell className="w-4 h-4" />} label="Alerts" active={location.pathname === '/alerts'} />
<NavLink href="/apps" icon={<AppWindow className="w-4 h-4" />} label="Apps" active={location.pathname === '/apps'} />

{/* Configuration Section */}
<NavLink href="/rules" icon={<ShieldCheck className="w-4 h-4" />} label="Detection Rules" active={location.pathname === '/rules'} />
<NavLink href="/connectors" icon={<Plug className="w-4 h-4" />} label="LLM Connectors" active={location.pathname === '/connectors'} />

{/* Admin Section */}
<NavLink href="/settings" icon={<Settings className="w-4 h-4" />} label="Settings" active={location.pathname === '/settings'} />
<NavLink href="/users" icon={<Users className="w-4 h-4" />} label="Users" active={location.pathname === '/users'} />
```

Also remove the `signals` NavLink — there is no `/signals` route (it was in the original but never wired).

### 7B — Replace emoji in `SettingsPage.tsx` tabs

The `tabs` array in `SettingsPage.tsx` uses emoji icons. Replace with text-only labels (the tab bar is compact; lucide icons in tab buttons create sizing issues at this size):

```typescript
const tabs: Array<{ id: SettingsTab; label: string }> = [
  { id: 'general', label: 'General' },
  { id: 'notifications', label: 'Notifications' },
  { id: 'retention', label: 'Retention' },
  { id: 'sdk-health', label: 'SDK Health' },
  { id: 'routing-rules', label: 'Routing Rules' },
]
```

Update the tab button render to remove `{tab.icon}`:
```typescript
<button key={tab.id} onClick={() => setActiveTab(tab.id)} ...>
  {tab.label}
</button>
```

### 7C — Add loading skeletons

Pages that currently show blank while loading should use `Skeleton.tsx`. Update `AlertsPage.tsx`:

Replace the loading state:
```typescript
// BEFORE
{isLoading ? (
  <div className="p-8 text-center text-muted-foreground">Loading alerts...</div>
) : ...}

// AFTER
{isLoading ? (
  <div className="p-4 space-y-2">
    {Array.from({ length: 5 }).map((_, i) => (
      <div key={i} className="h-12 bg-border/40 rounded animate-pulse" />
    ))}
  </div>
) : ...}
```

Apply the same pattern to `IncidentsPage.tsx` loading state.

### Verification

```bash
npm run build   # exits 0
```

In browser: sidebar icons render as clean vector icons, not emojis. No layout shift.

### Commit message

```
feat(web): replace emoji icons with lucide-react + add loading skeletons
```

---

## Task 8: Disable Unimplemented Settings/Notifications Endpoints

**Estimated time:** 10 minutes

### Problem

`web/src/pages/SettingsPages/NotificationsSettings.tsx` calls:
- `POST /api/v1/settings/notifications` (does not exist on backend)
- `POST /api/v1/settings/notifications/test/:service` (does not exist on backend)

These silently fail with 404 errors in the network console.

### Solution (Option A — disable + toast)

Update `NotificationsSettings.tsx` to show a disabled state with an explanatory message instead of firing broken API calls.

**Change `handleSave`:**
```typescript
const handleSave = async () => {
  // Backend endpoint POST /api/v1/settings/notifications not yet implemented.
  // Show informational message instead of firing a failing request.
  setSaved(false)
  setTestResult({
    success: false,
    message: 'Notification settings persistence is not yet implemented (planned for Step 6). Configuration will reset on page reload.',
  })
}
```

**Change `handleTest`:**
```typescript
const handleTest = async (service: string) => {
  setTestingService(service)
  // Simulate brief delay for UX, then show not-implemented message
  await new Promise((r) => setTimeout(r, 400))
  setTestResult({
    success: false,
    message: `Test notifications via ${service} are not yet implemented. Backend endpoint POST /api/v1/settings/notifications/test/${service} is pending.`,
  })
  setTestingService(null)
}
```

**Update the save button** to show the correct disabled reason:
```typescript
<button
  onClick={handleSave}
  disabled={isSaving}
  title="Notification persistence not yet implemented"
  className="px-4 py-2 bg-border text-muted-foreground rounded cursor-not-allowed opacity-60 font-medium"
>
  Save Settings (not yet available)
</button>
```

Remove the `import axios from 'axios'` line from `NotificationsSettings.tsx` since it is no longer used.

### Verification

Open `/settings` → Notifications tab → toggle Slack on → fill webhook URL → click "Test Slack" → orange message appears explaining endpoint is not implemented. No 404 in Network tab.

### Commit message

```
fix(web): disable unimplemented notification settings API calls (501 pattern)
```

---

## Task 9: Gate Verification

**Estimated time:** 20 minutes (build + manual walkthrough)

### 9A — Build verification

```bash
cd C:/Users/Drupad/ArgusXDR/.worktrees/step2-detection-engine/web
npm install          # ensure clean install
npm run build        # must exit 0, zero TypeScript errors
```

Expected output:
```
✓ built in N.Ns
dist/index.html
dist/assets/index-[hash].js
dist/assets/index-[hash].css
```

If there are TypeScript errors, fix them. Common issues to check:
1. Missing `React` import in files using JSX (React 19 auto-imports JSX transform, but some files may still have explicit `import React from 'react'` — keep them, they do not cause errors)
2. `import type` vs `import` for type-only imports in TypeScript strict mode
3. `.bak` files — TypeScript ignores `.bak` extensions, so they will not cause errors

### 9B — Start dev server

```bash
cd C:/Users/Drupad/ArgusXDR/.worktrees/step2-detection-engine/web
npm run dev
# → Listening at http://localhost:5173
```

### 9C — Manual browser checklist

Open browser devtools (F12) and keep the Console tab open throughout.

| Route | Expected | Pass? |
|-------|----------|-------|
| `http://localhost:5173/dashboard` | Redirects to `/login` | |
| `/login` | Login form renders, no console errors | |
| Login with valid credentials | JWT stored, redirect to `/dashboard` | |
| `/dashboard` | Coverage map, signal stream, two charts | |
| WebSocket indicator | Bottom-right dot turns green | |
| `/alerts` | Table of alerts, "Acknowledge" button visible | |
| Click "Acknowledge" on an alert | No 404 in Network tab | |
| `/incidents` | Table of incidents | |
| Click "View" on incident → set "Resolved" → Update | No 404, modal closes | |
| `/rules` | Rules list renders | |
| `/query` | SQL editor renders, can type a query | |
| `/settings` | 5 tabs: General, Notifications, Retention, SDK Health, Routing Rules | |
| Cmd+K (or Ctrl+K) | Command palette opens | |
| Console | Zero uncaught errors on all above pages | |

### 9D — If backend is not running

The following degraded behaviors are acceptable when backend is offline:
- WebSocket indicator stays red (expected)
- Tables show "No alerts found" / "No incidents found" (expected, 503 from backend)
- Coverage map shows gray cells (expected)

The following must NOT happen even with no backend:
- Build failure
- TypeScript compilation errors
- White screen / uncaught JS exceptions on any route
- Routes that return 404 (no matching React Router route)

---

## Dependency Tree (what each task depends on)

```
Task 1 (npm install + primitives)
    └── Task 2 (API fixes — needs axios-client import)
    └── Task 3 (routing — needs pages to exist)
        └── Task 4 (WebSocket — needs auth guard so unauthenticated users don't connect)
            └── Task 5 (charts — needs DashboardPage structure stable)
    └── Task 6 (settings tab — standalone)
    └── Task 7 (icons — needs lucide-react from Task 1)
    └── Task 8 (disable broken endpoints — standalone)
└── Task 9 (gate — depends on all above)
```

Execute in order: 1 → 2 → 3 → 4 → 5 → 6 → 7 → 8 → 9.

---

## Files Changed Summary

| File | Change Type | Task |
|------|-------------|------|
| `web/package.json` | modified (new deps added via npm install) | 1 |
| `web/components.json` | created | 1 |
| `web/src/lib/utils.ts` | created | 1 |
| `web/src/components/ui/button.tsx` | created | 1 |
| `web/src/pages/AlertsPage.tsx` | modified (suppress→acknowledge, interface) | 2 |
| `web/src/pages/IncidentsPage.tsx` | modified (PATCH→conditional POST) | 2 |
| `web/src/hooks/useCoverageMap.ts` | modified (uncomment polling) | 2 |
| `web/src/App.tsx` | rewritten (route structure, ProtectedRoute, public routes) | 3 |
| `web/vite.config.ts` | modified (ws: true on /v1 proxy) | 4 |
| `web/src/hooks/useSignalStream.ts` | modified (uncomment WS code) | 4 |
| `web/src/layouts/MainLayout.tsx` | modified (ConnectionStatus reads from store) | 4 |
| `web/src/components/SignalVolumeChart.tsx` | created | 5 |
| `web/src/components/SeverityDistributionChart.tsx` | created | 5 |
| `web/src/pages/DashboardPage.tsx` | modified (add charts row) | 5 |
| `web/src/pages/SettingsPages/RoutingRulesSettings.tsx` | created | 6 |
| `web/src/pages/SettingsPage.tsx` | modified (add routing-rules tab) | 6 |
| `web/src/layouts/MainLayout.tsx` | modified (lucide icons, NavLink type) | 7 |
| `web/src/pages/SettingsPage.tsx` | modified (remove emoji from tabs) | 7 |
| `web/src/pages/AlertsPage.tsx` | modified (skeleton placeholder) | 7 |
| `web/src/pages/IncidentsPage.tsx` | modified (skeleton placeholder) | 7 |
| `web/src/pages/SettingsPages/NotificationsSettings.tsx` | modified (disable API calls) | 8 |

---

## Known Limitations (Acceptable for Step 5)

1. **Forgot password flow** — `/forgot-password` link in `LoginPage.tsx` points to a non-existent page. A catch-all redirect sends it to `/dashboard` (protected) which redirects to `/login` (loop avoided since unauthenticated). Full forgot-password flow is a Step 6+ item.

2. **Users/AuditLog pages** — These are registered routes with placeholder content. Full implementation pending Step 4 (auth + RBAC endpoints).

3. **REST fallback for signal stream** — If WebSocket is unavailable, the signal store stays empty. No REST polling fallback is implemented. The connection banner informs the operator.

4. **Notification settings not persisted** — Acknowledged. Operators are informed via the disabled UI.

5. **`axios` vs `apiClient`** — Several `useQuery` calls in `AlertsPage.tsx` and `IncidentsPage.tsx` still use raw `axios` (no JWT injection). This is a pre-existing gap. These endpoints currently do not require auth tokens (Step 4 adds auth middleware). No action required in Step 5.

---

## Appendix: `axios-client.ts` Interface

The axios client at `web/src/lib/axios-client.ts` exports `apiClient`. Use it for all new API calls:

```typescript
import { apiClient } from '../lib/axios-client'

// GET with params
const res = await apiClient.get('/api/v1/alerts', { params: { status: 'delivered' } })

// POST
await apiClient.post('/api/v1/alerts/123/acknowledge')

// POST with body
await apiClient.post('/api/v1/incidents/456/resolve', { note: 'Fixed' })
```

The client:
- Injects `Authorization: Bearer <token>` from Zustand auth store
- On 401: silently calls `POST /api/v1/auth/refresh`, retries original request
- On refresh failure: clears auth state (user must re-login)

---

*End of Step 5 Implementation Plan*
