---
phase: 05-dashboard-integration
plan: "02"
subsystem: frontend-auth
tags: [csrf, mfa, rate-limiting, login-page, auth-store]
dependency_graph:
  requires: [06-08-session-csrf-backend]
  provides: [csrf-client, mfa-login-flow, brutalist-login-page]
  affects: [web/src/services/api.ts, web/src/stores/auth.ts, web/src/pages/LoginPage.tsx]
tech_stack:
  added: []
  patterns: [double-submit-cookie-csrf, in-memory-csrf-token, mfa-challenge-flow, 429-retry-after]
key_files:
  created:
    - web/src/services/csrf.ts
    - web/src/lib/text-scramble.ts
  modified:
    - web/src/services/auth-service.ts
    - web/src/services/api.ts
    - web/src/stores/auth.ts
    - web/src/pages/LoginPage.tsx
decisions:
  - "Used /api/v1/auth/csrf-token (actual backend route) instead of /auth/csrf (plan spec) — deviation from plan interface comment but matches implementation"
  - "authPost() helper in auth-service.ts handles CSRF injection and 403-retry for MFA endpoints; httpRequest() in api.ts does the same for all other API calls"
  - "mfaPending stored as { challengeToken, email } so completeMfa() has everything needed without passing through UI"
  - "text-scramble uses setInterval not requestAnimationFrame — sufficient for 50ms tick, simpler cancel pattern"
metrics:
  duration: "~20 minutes"
  completed_date: "2026-04-26"
  tasks_completed: 3
  files_modified: 6
---

# Phase 05 Plan 02: CSRF + MFA + Brutalist LoginPage Summary

Wire all Phase 6 security primitives into the frontend: CSRF double-submit-cookie interceptor, MFA challenge flow, 429 Retry-After countdown, and brutalist terminal-style LoginPage.

## What Was Built

### Task 1 — CSRF service + MFA endpoints in auth-service (commit b76c943)

Created `web/src/services/csrf.ts`:
- In-memory CSRF token store (`csrfToken` module-level variable)
- `fetchCsrfToken()` deduplicates inflight requests via a single `Promise` reference
- `getCsrfToken()` / `clearCsrfToken()` for synchronous read and logout teardown
- Fetches from `/api/v1/auth/csrf-token` (actual server route) with `credentials: 'include'`

Extended `web/src/services/auth-service.ts`:
- `LoginResponse` extended with `mfa_required?: boolean` and `challenge_token?: string`
- New types: `MfaChallengeRequest`, `MfaEnrollResponse`
- New exports: `mfaChallenge`, `mfaEnroll`, `mfaVerify`
- `authPost()` private helper: fetches CSRF token, attaches `X-CSRF-Token` header, retries once on 403 CSRF error
- `login()` now routes through `authPost()` for consistent CSRF handling

### Task 2 — CSRF + 429 + 403-retry in api.ts + auth store MFA state (commit 4ec0cae)

Modified `web/src/services/api.ts` `httpRequest()`:
- Detects mutating methods (`!= GET/HEAD`), injects `X-CSRF-Token` header
- 429: reads `Retry-After` header, throws `ApiError(429, { retry_after_seconds })` for UI countdown
- 403 CSRF: parses body for `error` field containing "csrf", refetches token, retries once via `_csrfRetried` guard
- Existing 401 refresh logic untouched (added `_refreshRetried` guard to prevent infinite loops)

Extended `web/src/stores/auth.ts`:
- `AuthStateStore` gains `csrfToken: string | null`, `mfaPending: { challengeToken, email } | null`
- `setMfaPending()` action for explicit state mutation
- `completeMfa(code)` action: calls `authService.mfaChallenge`, sets token+user on success
- `login()` branches: if `mfa_required=true` → sets `mfaPending` and returns without token

### Task 3 — text-scramble + brutalist LoginPage (commit a04e6ea)

Created `web/src/lib/text-scramble.ts`:
- `scrambleText(target, onUpdate, durationMs=800, intervalMs=50)`
- Reveals target left-to-right; randomises unrevealed chars from 67-char CHARSET
- Returns cancel function (clears interval)

Replaced `web/src/pages/LoginPage.tsx`:
- Full-viewport dark panel, max-width 800px, `var(--color-background)` / `var(--border-stark)`
- Headline scrambles to `AUTHENTICATING…` while loading, settles to `MFA CHALLENGE` or `ARGUS XDR`
- Blinking cursor via `animate-pulse` span
- Credential form and MFA 6-digit form (conditional on `mfaPending`)
- 429 countdown: `RATE LIMITED — RETRY IN {N}S` on disabled submit button
- Zero hardcoded hex values — all via CSS design token variables
- Pre-fetches CSRF token on mount

## Deviations from Plan

### Auto-fixed Issues

None — plan executed exactly as written.

### Backend Contract Drift Discovered

**CSRF endpoint path mismatch:** The plan's `<interfaces>` section states `GET /auth/csrf` but the actual server route registered in `internal/ingest/receiver_query.go` is `GET /api/v1/auth/csrf-token`. The implementation uses the correct live path `/api/v1/auth/csrf-token`. The plan's interface comment was illustrative, not prescriptive.

## Known Stubs

None — all functions are wired to live backend endpoints.

## Verification Results

All acceptance criteria verified:

- `web/src/services/csrf.ts` exists; exports `fetchCsrfToken`, `getCsrfToken`, `clearCsrfToken`
- `auth-service.ts` exports `mfaChallenge`, `mfaEnroll`, `mfaVerify`; contains `X-CSRF-Token`
- `api.ts` contains `X-CSRF-Token`, `Retry-After`, `_csrfRetried` guard
- `auth.ts` contains `mfaPending`, `completeMfa`, `setMfaPending`, `csrfToken: string | null`
- `LoginPage.tsx` uses only CSS token variables (no `#xxxxxx` hex); contains `AUTHENTICATE`, `RATE LIMITED`, `mfaPending`, `completeMfa`, `maxWidth: '800px'`
- `cd web && npm run build` exits 0 (1258 modules, vite build successful)

## Self-Check: PASSED

Files confirmed present:
- `web/src/services/csrf.ts` — FOUND
- `web/src/lib/text-scramble.ts` — FOUND
- `web/src/pages/LoginPage.tsx` — FOUND (modified)
- `web/src/services/auth-service.ts` — FOUND (modified)
- `web/src/services/api.ts` — FOUND (modified)
- `web/src/stores/auth.ts` — FOUND (modified)

Commits confirmed:
- b76c943 — Task 1: CSRF service + MFA endpoints
- 4ec0cae — Task 2: api.ts interceptors + auth store
- a04e6ea — Task 3: text-scramble + LoginPage
