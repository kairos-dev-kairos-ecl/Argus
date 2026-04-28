---
plan: 05-03
status: complete
completed: 2026-04-27
wave: 2
---

## Summary

Rebuilt `MainLayout` and `SideNav` brutalist shell. 240px fixed sidebar with 8 uppercase monospace nav items (TELEMETRY, TRACES, HUNTING, AUDIT, INCIDENTS, KAIROS, IAM, SETTINGS). Active route highlighted with `var(--color-primary)` cyan. Admin-filtered items. Added `/kairos` route to `App.tsx`.

## Key Files

### Created
- `web/src/components/SideNav.tsx` — 240px sidebar, 8 nav items, admin filtering
- `web/src/layouts/MainLayout.tsx` — flex two-column shell (SideNav + main content)

### Modified
- `web/src/App.tsx` — Added /kairos route, wired MainLayout

## Self-Check: PASSED

All tasks complete. TypeScript clean. Layout renders correctly.
