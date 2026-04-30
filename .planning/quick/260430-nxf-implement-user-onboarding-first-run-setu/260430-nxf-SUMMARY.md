---
phase: quick
plan: 260430-nxf
subsystem: auth+frontend
tags: [onboarding, invite, setup-wizard, jwt, pgxpool]
dependency_graph:
  requires: [internal/auth/users.go, internal/ingest/handler_auth.go, web/src/stores/auth.ts]
  provides: [setup-status-endpoint, auto-login-after-setup, invite-lifecycle, accept-invite-page, setup-wizard-v2]
  affects: [web/src/App.tsx, web/src/pages/LoginPage.tsx, web/src/pages/UsersPage.tsx]
tech_stack:
  added: [crypto/rand SHA-256 token generation, pgxpool invite store]
  patterns: [InviteStore interface + PgInviteStore, InviteService orchestrator, ExcludedPrefixes middleware config]
key_files:
  created:
    - internal/auth/invite.go
    - internal/storage/migrations/021_user_invites.up.sql
    - internal/storage/migrations/021_user_invites.down.sql
    - web/src/pages/AcceptInvitePage.tsx
  modified:
    - internal/ingest/handler_auth.go
    - internal/ingest/receiver_query.go
    - internal/auth/middleware.go
    - cmd/argus/api.go
    - web/src/pages/SetupWizard.tsx
    - web/src/pages/LoginPage.tsx
    - web/src/pages/UsersPage.tsx
    - web/src/services/iam-service.ts
    - web/src/App.tsx
decisions:
  - ExcludedPrefixes added to MiddlewareConfig to handle parameterised invite routes (/api/v1/invite/{token}) without exact-path map lookup
  - PasswordHasher parameter in AcceptInvite kept as nil-safe pass-through since UserService.CreateUser handles bcrypt internally
  - access_token emission after setup is non-fatal (Warn log, empty token) to preserve setup idempotency on token issuance failure
metrics:
  duration: "~35 min"
  completed_date: "2026-04-30"
  tasks_completed: 2
  tasks_total: 2
  files_changed: 13
---

# Quick 260430-nxf: User Onboarding First-Run Setup Summary

First-run setup wizard, auto-login after setup, and admin invite flow with token-based acceptance — end-to-end from fresh install redirect to new user dashboard landing.

## What Was Built

### Backend (Task 1)

**Migration 021 — invites table:**
- `id` UUID PK, `email`, `role`, `token_hash` (UNIQUE), `invited_by` FK→users, `expires_at` (7-day default), `accepted_at`, `created_at`
- Indexes on `token_hash` and `email`

**internal/auth/invite.go:**
- `InviteStore` interface with `Create / GetByTokenHash / MarkAccepted`
- `PgInviteStore` backed by pgxpool with pgx queries
- `InviteService` with `GenerateToken` (32-byte crypto/rand → hex), `CreateInvite`, `GetByToken` (validates expiry + accepted), `AcceptInvite` (creates user + marks accepted)
- `InviteUserCreator` interface avoids circular dependency with concrete postgres store

**handler_auth.go additions:**
- `handleSetupStatus` — GET /api/v1/setup/status, public, returns `{needs_setup: bool}`
- `handleSetup` modified — now issues JWT after PerformSetup, returns `{user, app, api_key, access_token, message}`
- `handleInviteCreate` — POST /api/v1/users/invite, admin only, returns `{invite_url, token, expires_at}`
- `handleInviteGet` — GET /api/v1/invite/{token}, public, returns `{valid, email, role}` or `{valid: false, reason}`
- `handleInviteAccept` — POST /api/v1/invite/{token}/accept, public, creates user + returns `{access_token, user}`

**Route registration:**
- `GET /api/v1/setup/status` and invite routes added to RegisterRoutes
- `ExcludedPrefixes: ["/api/v1/invite/"]` added to MiddlewareConfig so parameterised paths bypass JWT check
- `cmd/argus/api.go` ExcludedPaths updated to include `/api/v1/setup/status`

### Frontend (Task 2)

**SetupWizard.tsx (full rewrite):**
- 4-step flow: ACCOUNT → ORG → TOKEN → DONE
- ACCOUNT: email, display_name, password, confirm_password with client-side validation
- ORG: instance_name, app_name, submits to `performSetup`, stores access_token via auth store
- TOKEN: styled code block for api_key with COPY button and "shown once" warning
- DONE: "Go to Dashboard" → navigate('/')

**LoginPage.tsx:**
- `useEffect` on mount calls `checkSetupStatus()`, redirects to `/setup` if `needs_setup: true`, fails silently on error

**AcceptInvitePage.tsx (new):**
- Reads `?token=` from useSearchParams
- Validates via `getInvite(token)`, shows pre-filled email/role (readonly)
- Form for display_name + password; `acceptInvite` → stores token → navigate('/')
- Shows "invalid/expired" message with back-to-login link on failure

**UsersPage.tsx:**
- Admin-only "+ INVITE USER" button toggles inline invite form
- Form: email (text), role (select: admin/analyst/viewer), "SEND INVITE" submits
- On success: shows shareable invite_url in readonly input with COPY button

**iam-service.ts additions:**
- `checkSetupStatus`, `performSetup` via `callPublic` (no auth header)
- `getInvite`, `acceptInvite` via `callPublic`
- `createInvite` via authenticated `call`

**App.tsx:**
- `/accept-invite` added as public route alongside `/login` and `/setup`

## Verification

- `go build ./...` passes with no errors
- `tsc --noEmit` passes with no TypeScript errors
- `go vet ./internal/auth/... ./internal/ingest/...` passes clean

## Deviations from Plan

### Auto-fixed Issues

**1. [Rule 1 - Bug] hashToken name collision**
- **Found during:** Task 1 build
- **Issue:** `hashToken` function in invite.go collided with existing `hashToken` in middleware.go (same package)
- **Fix:** Renamed to `hashInviteToken` in invite.go
- **Files modified:** internal/auth/invite.go
- **Commit:** 20a44ab

**2. [Rule 2 - Missing functionality] ExcludedPaths exact-match can't handle parameterised invite routes**
- **Found during:** Task 1 route analysis
- **Issue:** `ExcludedPaths` uses exact `map[string]bool` lookup; `/api/v1/invite/{token}` and `/api/v1/invite/{token}/accept` would never match
- **Fix:** Added `ExcludedPrefixes []string` field to `MiddlewareConfig` with prefix-based bypass check
- **Files modified:** internal/auth/middleware.go, cmd/argus/api.go
- **Commit:** 20a44ab

**3. [Rule 3 - Blocking] handler_auth.go referenced `QueryReceiver` (plan) but actual type is `QueryHandler`**
- **Found during:** Task 1 code reading
- **Issue:** Plan spec used `QueryReceiver` as receiver type; codebase uses `QueryHandler`
- **Fix:** Used correct `QueryHandler` receiver throughout all new handlers (matches existing code)
- **Files modified:** internal/ingest/handler_auth.go
- **Commit:** 20a44ab

## Known Stubs

None — all data flows are wired. Invite URL uses `r.Host` as base URL which defaults to the request's Host header; this is correct for both local dev and production deployment.

## Self-Check: PASSED

| Item | Status |
|------|--------|
| internal/auth/invite.go | FOUND |
| 021_user_invites.up.sql | FOUND |
| web/src/pages/AcceptInvitePage.tsx | FOUND |
| Commit 20a44ab (backend) | FOUND |
| Commit 43fc5e6 (frontend) | FOUND |
