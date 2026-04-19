# Web Dashboard

> React 19 + Vite + TanStack Query + Zustand + ECharts. Real-time signal viewer + configuration UI.

## Tech Stack

| Layer | Library | Version | Purpose |
|-------|---------|---------|---------|
| Framework | React | 19.x | Component model |
| Build | Vite | 5.x | Fast dev server + production bundler |
| Language | TypeScript | 5.4+ | Type safety |
| Server state | TanStack Query | 5.x | Caching + refetch + background sync |
| Client state | Zustand | 4.5+ | Global state (auth, filters, UI) |
| Routing | React Router | 6.x | Page navigation |
| UI Components | shadcn/ui + Radix | latest | Accessible primitives |
| Styling | Tailwind CSS | 3.4+ | Utility-first CSS |
| Charting | Apache ECharts | 5.5+ | 100K+ data point rendering |
| ECharts wrapper | echarts-for-react | 3.0+ | React bindings |
| SQL Editor | CodeMirror | 6.x | Syntax highlight + autocomplete |
| HTTP | Axios | latest | API calls (TanStack Query wraps this) |
| Dates | Dayjs | latest | Date/time parsing |
| WebSocket | Native browser API | — | Live signal stream |

File: `web/package.json`

---

## Directory Structure

```
web/
├── src/
│   ├── App.tsx                    ← Root router setup
│   ├── main.tsx                   ← Vite entry
│   ├── pages/
│   │   ├── DashboardPage.tsx      ← Home (layer status + recent alerts)
│   │   ├── AlertsPage.tsx         ← Alert list/detail + acknowledge
│   │   ├── IncidentsPage.tsx      ← Incident lifecycle
│   │   ├── RulesPage.tsx          ← Rule CRUD + test
│   │   ├── QueryPage.tsx          ← SQL editor + results
│   │   ├── TracePage.tsx          ← Distributed trace Gantt
│   │   ├── UsersPage.tsx          ← User management
│   │   ├── AppsPage.tsx           ← App + API key management
│   │   ├── AuditLogPage.tsx       ← Audit trail
│   │   ├── LoginPage.tsx          ← Auth form
│   │   ├── SetupWizard.tsx        ← First-run admin creation
│   │   ├── ConfigPage.tsx         ← System config
│   │   ├── ConnectorConfigPage.tsx ← Connector setup
│   │   ├── ProfilePage.tsx        ← User profile
│   │   └── SettingsPage*.tsx      ← Various settings pages
│   ├── components/
│   │   ├── SignalStream.tsx       ← WebSocket live signal table
│   │   ├── TraceTimeline.tsx      ← Span Gantt chart (ECharts)
│   │   ├── QueryEditor.tsx        ← CodeMirror SQL + results
│   │   ├── SeverityBadge.tsx      ← Severity color badge
│   │   ├── LayerStatusCell.tsx    ← Layer name + icon
│   │   ├── SignalRow.tsx          ← Single signal table row
│   │   ├── SpanDetail.tsx         ← Expanded span info
│   │   ├── TokenConfidenceView.tsx ← L5 logprob visualization
│   │   ├── CoverageMap.tsx        ← Layer coverage heatmap
│   │   ├── GroundingView.tsx      ← L7 retrieval grounding
│   │   ├── DetectionAnnotation.tsx ← Overlay detection matches
│   │   ├── ReasoningGraph.tsx     ← Kairos decision reasoning
│   │   ├── IncidentDetail.tsx     ← Incident + correlated alerts
│   │   ├── RuleEditor.tsx         ← YAML rule editor
│   │   ├── ErrorBoundary.tsx      ← React error fallback
│   │   └── ui/                    ← shadcn/ui primitives
│   │       ├── button.tsx
│   │       ├── table.tsx
│   │       ├── dialog.tsx
│   │       ├── form.tsx
│   │       └── ... (20+ radix-based components)
│   ├── stores/
│   │   ├── auth.ts                ← JWT token + user claims + login/logout
│   │   ├── signal.ts              ← Signal stream filter + search state
│   │   ├── layer.ts               ← Layer filter + status cache
│   │   └── traceViewStore.ts      ← Trace detail view state
│   ├── hooks/
│   │   ├── useSignalQuery.ts      ← TanStack Query hook for signals
│   │   ├── useWebSocket.ts        ← WebSocket connection lifecycle
│   │   ├── useAuth.ts             ← Auth state + login/logout
│   │   ├── usePagination.ts       ← Cursor pagination logic
│   │   └── ... (10+ custom hooks)
│   ├── api/
│   │   ├── client.ts              ← Axios instance with interceptors
│   │   ├── queries.ts             ← TanStack Query key factory + hooks
│   │   └── mutations.ts           ← TanStack Query mutations (alert ack, rule update, etc)
│   ├── types/
│   │   ├── signal.ts              ← ArgusSignal TS interface
│   │   ├── alert.ts               ← Alert + Incident types
│   │   ├── rule.ts                ← Detection rule types
│   │   └── ... (10+ type defs)
│   ├── lib/
│   │   ├── design-tokens.ts       ← Color, spacing, typography constants
│   │   ├── format-utils.ts        ← Format severity, layer, timestamp
│   │   ├── trace-builder.ts       ← Build trace tree from signals
│   │   └── ... (utils)
│   └── styles/
│       └── index.css              ← Tailwind + global styles
├── vite.config.ts
├── tsconfig.json
├── package.json
└── README.md
```

---

## Key Pages

### DashboardPage
Real-time overview. Shows:
- Layer status boxes (L1–L10): signal count + sparkline
- Recent alerts (last 10)
- Incident summary
- System health (ClickHouse, PostgreSQL, Redis status)

Pulls from:
- `GET /api/v1/layers/status` (5-sec refetch)
- `GET /api/v1/alerts?limit=10` (10-sec refetch)
- `GET /health` (30-sec refetch)

### SignalStream Component
WebSocket consumer for `GET /v1/signals/stream`:
1. Opens WebSocket on mount
2. Receives live `ArgusSignal` messages (JSON)
3. Renders table with: signal_id, timestamp, layer, category, severity
4. Implements filter: layer, category, severity (Zustand `signal.ts`)
5. Auto-scrolls; optionally pauses for manual inspection

### TracePage
Distributed trace visualization:
1. Query: `GET /api/v1/traces/{traceId}`
2. Build span tree from signals
3. Render Gantt chart (ECharts) with layers as lanes, spans as bars
4. On click: expand `SpanDetail` panel with all signal fields

### QueryPage
SQL editor for analysts:
1. CodeMirror syntax highlight + autocomplete (ClickHouse dialect)
2. `POST /api/v1/query` with SQL body (read-only, 5000 row max)
3. Display results in paginated table
4. Save queries as client-side bookmarks (localStorage)

### AlertsPage
Alert management:
1. List (filterable by status, severity, app_id)
2. Detail view: trace link, signal list, incident link
3. `POST /api/v1/alerts/{id}/acknowledge` button
4. Auto-refetch on acknowledge (TanStack Query invalidation)

---

## Stores (Zustand)

### `auth.ts`
```typescript
{
  user?: { id, email, role, permissions[] }
  token?: string
  refreshToken?: string
  isLoggedIn: boolean
  login(email, password)
  logout()
  setUser(user)
}
```

### `signal.ts`
```typescript
{
  filters: {
    layer?: number
    category?: string
    severity?: number
  }
  setLayerFilter(layer)
  setCategoryFilter(cat)
  setSeverityFilter(sev)
}
```

### `traceViewStore.ts`
```typescript
{
  selectedTraceId?: string
  expandedSpanIds: Set<string>
  setSelectedTrace(traceId)
  toggleSpan(spanId)
}
```

---

## API Integration (TanStack Query)

### Pattern
```typescript
// queries.ts
export const signalQueries = {
  all: () => [{ scope: 'signals' }],
  list: (params) => [...signalQueries.all(), { type: 'list', params }],
  detail: (id) => [...signalQueries.all(), { type: 'detail', id }],
};

// hooks
export function useSignals(params) {
  return useQuery({
    queryKey: signalQueries.list(params),
    queryFn: () => api.get('/v1/signals', { params }),
    refetchInterval: 5000, // 5-sec refetch
    staleTime: 3000,       // consider stale after 3s
  });
}
```

### Mutations
```typescript
// mutations.ts
export function useAcknowledgeAlert() {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: (alertId) => api.post(`/api/v1/alerts/${alertId}/acknowledge`),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['alerts'] });
    },
  });
}
```

---

## Styling

Tailwind CSS + shadcn/ui color palette:
- Primary: blue-600
- Destructive: red-500
- Muted: gray-500
- Success: green-500
- Warning: yellow-500

Layer color scheme (L1–L10):
- L1: red-500
- L2: orange-500
- L3: yellow-500
- L4: green-500
- L5: cyan-500
- L6: blue-500
- L7: indigo-500
- L8: purple-500
- L9: pink-500
- L10: rose-500

---

## File Map

| File | Component |
|------|-----------|
| `web/src/App.tsx` | Router setup |
| `web/src/pages/*.tsx` | Page components |
| `web/src/components/*.tsx` | Reusable UI components |
| `web/src/stores/*.ts` | Zustand stores |
| `web/src/hooks/*.ts` | Custom React hooks |
| `web/src/api/client.ts` | Axios instance |
| `web/src/api/queries.ts` | TanStack Query key factory |
| `web/src/api/mutations.ts` | TanStack Query mutations |
| `web/vite.config.ts` | Vite config |
| `web/package.json` | Dependencies |
