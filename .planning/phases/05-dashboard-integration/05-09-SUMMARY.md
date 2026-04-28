---
phase: 05-dashboard-integration
plan: "09"
subsystem: frontend-iam
tags: [iam, mfa, sessions, api-keys, password, settings, phase6-ui]
dependency_graph:
  requires: ["05-01", "05-02", "05-03"]
  provides: ["screen-8-iam-console", "settings-pages", "iam-service"]
  affects: ["UsersPage", "SettingsPages"]
tech_stack:
  added: []
  patterns: ["20/80-sidebar-layout", "one-time-key-visibility", "hibp-breach-indicator", "mfa-enrollment-flow", "brutalist-form-style"]
key_files:
  created:
    - web/src/services/iam-service.ts
    - web/src/components/iam/RolePermissionMatrix.tsx
    - web/src/pages/SettingsPages/MfaEnrollment.tsx
    - web/src/pages/SettingsPages/ActiveSessions.tsx
    - web/src/pages/SettingsPages/ApiKeys.tsx
    - web/src/pages/SettingsPages/PasswordChange.tsx
    - web/src/pages/SettingsPages/index.ts
  modified:
    - web/src/pages/UsersPage.tsx
decisions:
  - "Used var(--color-success) (not --color-status-success) per actual token definition in tokens.css"
  - "RolePermissionMatrix static display only, no editing in this plan"
  - "UsersGrid embedded inline in UsersPage as inner component to keep file self-contained"
  - "mfaVerify called with code string directly matching auth-service export signature"
metrics:
  duration: "~10 minutes"
  completed_date: "2026-04-27"
  tasks: 3
  files: 8
---

# Phase 5 Plan 09: IAM Console (Screen 8) Summary

**One-liner:** 20/80 IAM console shell with MFA enrollment QR flow, active session revocation, one-time API key visibility, HIBP password breach indicator, and L1-L10 role permission matrix.

---

## Tasks Completed

| Task | Name | Commit | Files |
|------|------|--------|-------|
| 1 | iam-service + RolePermissionMatrix | 625e91a | iam-service.ts, RolePermissionMatrix.tsx |
| 2 | 4 settings pages + index.ts | 6d02913 | MfaEnrollment.tsx, ActiveSessions.tsx, ApiKeys.tsx, PasswordChange.tsx, index.ts |
| 3 | UsersPage 20/80 shell + section routing | 0db3755 | UsersPage.tsx |

---

## What Was Built

### iam-service.ts
HTTP service layer for IAM endpoints with CSRF + Authorization header injection:
- `fetchUsers` → `GET /api/v1/users`
- `fetchSessions` → `GET /api/v1/auth/sessions`
- `revokeSession(id)` → `DELETE /api/v1/auth/sessions/{id}`
- `revokeAllOther()` → `DELETE /api/v1/auth/sessions`
- `fetchApiKeys` → `GET /api/v1/apps`
- `createApiKey(name)` → `POST /api/v1/apps`
- `revokeApiKey(id)` → `DELETE /api/v1/apps/{id}`
- `changePassword(current, next)` → `POST /api/v1/auth/password` → `{ changed, hibp_breached? }`

### RolePermissionMatrix.tsx
Static CSS grid table: admin/analyst/viewer rows × L1-L10 columns.
- admin: full access all 10 layers (● dot in primary)
- analyst: access L4-L10 (— dash in muted for L1-L3)
- viewer: access L9-L10 only
- Layer column headers colored per `var(--color-layer-lN)` tokens

### MfaEnrollment.tsx
TOTP enrollment flow: BEGIN ENROLLMENT → `mfaEnroll()` → QR code img + manual key + backup codes list → 6-digit input → `mfaVerify(code)` → MFA ENABLED confirmation. Backup codes displayed with STORE THESE BACKUP CODES warning in var(--color-warning).

### ActiveSessions.tsx
Session list via `fetchSessions()`. 20px row height: DEVICE (180px), IP (140px), CREATED (180px), LAST SEEN (180px), ACTIONS. Current session shows CURRENT chip (no revoke). Other sessions: REVOKE button per row calling `revokeSession(id)`. REVOKE ALL OTHER SESSIONS mass button calling `revokeAllOther()`.

### ApiKeys.tsx
API key CRUD: name input + CREATE button → `createApiKey(name)` → shows returned key ONCE in callout with "THIS KEY WILL NOT BE SHOWN AGAIN" warning + COPY button. List: 20px rows with NAME, CREATED, LAST USED, per-row REVOKE via `revokeApiKey(id)`.

### PasswordChange.tsx
Three password fields: current, new (min 12 chars), confirm. Client-side validation blocks submit until: all fields filled, new ≥ 12 chars, new ≠ current, confirm = new. On `changePassword()` response: HIBP breach warning if `hibp_breached > 0`, PASSWORD UPDATED success otherwise.

### UsersPage.tsx
20/80 sidebar layout via `gridTemplateColumns: '20% 80%'`. Left nav: 6 section labels with active-state cyan left border. Right pane: routes to UsersGrid (inline), RolePermissionMatrix, MfaEnrollment, ActiveSessions, ApiKeys, PasswordChange based on `section` useState.

---

## Endpoint Contract Verification

| Endpoint | Used By | Method |
|----------|---------|--------|
| `POST /api/v1/auth/mfa/enroll` | MfaEnrollment via auth-service.mfaEnroll | POST |
| `POST /api/v1/auth/mfa/verify` | MfaEnrollment via auth-service.mfaVerify | POST |
| `GET /api/v1/auth/sessions` | ActiveSessions via iam-service.fetchSessions | GET |
| `DELETE /api/v1/auth/sessions/{id}` | ActiveSessions via iam-service.revokeSession | DELETE |
| `DELETE /api/v1/auth/sessions` | ActiveSessions via iam-service.revokeAllOther | DELETE |
| `GET /api/v1/apps` | ApiKeys via iam-service.fetchApiKeys | GET |
| `POST /api/v1/apps` | ApiKeys via iam-service.createApiKey | POST |
| `DELETE /api/v1/apps/{id}` | ApiKeys via iam-service.revokeApiKey | DELETE |
| `POST /api/v1/auth/password` | PasswordChange via iam-service.changePassword | POST |
| `GET /api/v1/users` | UsersGrid via iam-service.fetchUsers | GET |

---

## Deviations from Plan

None — plan executed exactly as written. One token name clarification:
- `var(--color-success)` used (not `--color-status-success`) per actual token definitions in `web/src/styles/tokens.css`. The plan referenced a non-existent token name; the correct one from the tokens file was used.

---

## Known Stubs

None. All API calls are wired to real backend endpoints. The permission matrix is intentionally static (display-only per plan spec — no editing in this plan).

---

## Self-Check: PASSED

Files created:
- web/src/services/iam-service.ts: FOUND
- web/src/components/iam/RolePermissionMatrix.tsx: FOUND
- web/src/pages/SettingsPages/MfaEnrollment.tsx: FOUND
- web/src/pages/SettingsPages/ActiveSessions.tsx: FOUND
- web/src/pages/SettingsPages/ApiKeys.tsx: FOUND
- web/src/pages/SettingsPages/PasswordChange.tsx: FOUND
- web/src/pages/SettingsPages/index.ts: FOUND
- web/src/pages/UsersPage.tsx: FOUND (rebuilt)

Commits:
- 625e91a: feat(05-09): add iam-service + RolePermissionMatrix
- 6d02913: feat(05-09): add 4 settings pages (MFA, sessions, API keys, password)
- 0db3755: feat(05-09): rebuild UsersPage as 20/80 IAM console shell

Build: `cd web && npm run build` exits 0.
