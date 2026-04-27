---
plan: 05-04
status: complete
completed: 2026-04-27
wave: 2
---

## Summary

Rebuilt DashboardPage (Screen 4). ECharts Sankey diagram (L1→L10, signal flow proportional to volume) replaces dense signal log. FilterSidebar, TimeScrubber, and AnomalyHorizonGrid wired. WebSocket /ws/signals with exponential backoff (1s→2s→4s→8s→30s). Node click syncs selectedLayer across Sankey + anomaly panel + filter sidebar. Skeleton mode when no signals.

## Key Files

### Created
- `web/src/components/dashboard/TimeScrubber.tsx` — 5M/15M/1H/24H time range buttons
- `web/src/components/dashboard/FilterSidebar.tsx` — layer/severity/category chip filters
- `web/src/services/websocket.ts` — createSignalSocket with exponential backoff

### Modified
- `web/src/pages/DashboardPage.tsx` — Full rewrite with ECharts Sankey as primary viz

## Self-Check: PASSED

All tasks complete. TypeScript clean. ECharts Sankey renders. WebSocket connected.
