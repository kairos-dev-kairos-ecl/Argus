---
phase: quick-260509-vvd
plan: "01"
subsystem: frontend-websocket
tags: [websocket, react-strictmode, bug-fix, rest-api]
dependency_graph:
  requires: []
  provides: [clean-ws-lifecycle, correct-signal-history]
  affects: [web/src/hooks/useSignalStream.ts, web/src/services/websocket.ts, web/src/lib/websocket.ts]
tech_stack:
  added: []
  patterns: [pendingReject-pattern, handler-detach-before-close]
key_files:
  modified:
    - web/src/hooks/useSignalStream.ts
    - web/src/services/websocket.ts
    - web/src/lib/websocket.ts
decisions:
  - "Null outer ws and detach handlers before calling close() — prevents stale onclose/onerror from firing after cleanup"
  - "pendingReject field stored on WebSocketClient so disconnect() and onclose can settle the in-flight Promise without a closure capture"
  - "settleResolve/settleReject helpers null pending_reject before calling resolve/reject to prevent double-settlement on concurrent events"
metrics:
  duration: "~10 minutes"
  completed: "2026-05-09"
  tasks_completed: 3
  files_modified: 3
---

# Quick Task 260509-vvd: Fix Frontend WebSocket Live-Test Issues — Summary

**One-liner:** Three surgical frontend fixes — remove `app_id='test'` filter blocking all non-test signals, suppress StrictMode CONNECTING-abort console error via handler-detach-before-close, and add `pendingReject` to `WebSocketClient` so `await connect()` rejects (not hangs) when `disconnect()` races the CONNECTING phase.

---

## Tasks Completed

| # | Task | Commit | Files |
|---|------|--------|-------|
| 1 | Remove hardcoded app_id from REST history fetch | e971850 | web/src/hooks/useSignalStream.ts |
| 2 | Fix createSignalSocket cleanup race in services/websocket.ts | 3f87ae6 | web/src/services/websocket.ts |
| 3 | Track pendingReject in WebSocketClient.connect() to prevent promise leak | 04a52d8 | web/src/lib/websocket.ts |

---

## What Was Fixed

### Task 1 — REST History Fetch: Remove `app_id: 'test'` filter

**File:** `web/src/hooks/useSignalStream.ts` line 101

The `refetchSignals` callback was calling `getSignals({ limit: 100, app_id: 'test' })`. This is a leftover from early scaffolding. In production and E2E testing, signals arrive with `app_id` values like `live-test-app`, `argus-e2e`, etc. Filtering to `'test'` caused the dashboard's initial render to show zero signals even when the database was full of fresh data.

Change: `getSignals({ limit: 100, app_id: 'test' })` → `getSignals({ limit: 100 })`. App-level filtering belongs in the FilterSidebar, not the initial fetch.

### Task 2 — createSignalSocket Cleanup Race (services/websocket.ts)

**File:** `web/src/services/websocket.ts` cleanup closure

Under React 18 StrictMode, a component's useEffect cleanup runs synchronously after first mount (before `ws.onopen` fires), meaning the cleanup runs when `ws.readyState === CONNECTING (0)`. The original `ws?.close()` caused the browser to log "WebSocket is closed before the connection is established" and triggered stale `onclose`/`onerror` handlers (reconnect setTimeout).

Fix: The cleanup closure now:
1. Sets `closed = true` (reconnect guard — still first statement)
2. Captures `ws` into a local variable
3. Nulls the outer `ws` binding
4. Detaches all four event handlers (`onopen`, `onmessage`, `onerror`, `onclose`) on the local reference
5. Calls `local.close()` inside a try/catch

Nulling `ws` before `close()` means any handlers that do fire (race between null assignment and close event) see `ws === null` and `closed === true`, so neither the status callback nor the reconnect setTimeout has any effect.

### Task 3 — WebSocketClient.connect() Promise Leak (lib/websocket.ts)

**File:** `web/src/lib/websocket.ts`

`WebSocketClient.connect()` returned a Promise that never rejected when `disconnect()` was called during the CONNECTING phase. Under React StrictMode's double-mount pattern, `useSignalStream`'s `await client.connect()` was hanging forever per mount cycle, leaking one Promise per double-mount.

Changes:
- Added `private pending_reject: ((err: Error) => void) | null = null` field
- `connect()` now assigns `reject` to `this.pending_reject` immediately after the Promise is created
- `settleResolve()` and `settleReject()` helpers null `pending_reject` before calling the settlement function (prevents double-reject on concurrent `onerror` + `onclose`)
- `disconnect()` eagerly rejects `pending_reject` before `ws.close()` — unblocks `await client.connect()` immediately
- `onclose` rejects `pending_reject` with `'WebSocket: disconnected before open'` when `is_closing === true`
- `onerror` now also calls `settleReject` so connection-refused errors surface to the caller

The existing `try/catch` around `await client.connect()` in `useSignalStream` already handles the rejection gracefully (calls `setError` and continues), so no caller changes were needed.

---

## Verification

- `cd web && npx tsc --noEmit` — passed after each task
- `cd web && npm run build` — production build succeeded (14.89s, 963 modules, no new errors; chunk size warnings are pre-existing)

---

## Deviations from Plan

None — plan executed exactly as written. All three edits were strictly scoped, no new files created, no API signatures changed.

---

## Known Stubs

None introduced by this plan.

---

## Self-Check: PASSED

Files modified:
- `web/src/hooks/useSignalStream.ts` — FOUND, contains `getSignals({ limit: 100 })`
- `web/src/services/websocket.ts` — FOUND, contains handler-detach-before-close pattern
- `web/src/lib/websocket.ts` — FOUND, contains `pending_reject` field

Commits verified:
- e971850 — FOUND
- 3f87ae6 — FOUND
- 04a52d8 — FOUND
