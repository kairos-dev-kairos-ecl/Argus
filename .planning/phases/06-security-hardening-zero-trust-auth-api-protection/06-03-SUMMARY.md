---
phase: 06-security-hardening-zero-trust-auth-api-protection
plan: 03
subsystem: Authentication & API Protection
tags:
  - api-keys
  - machine-identity
  - security
  - zero-trust
dependency_graph:
  requires: [06-01]
  provides: [API key schema, crypto primitives, REST CRUD]
  affects: [06-05 (Wave 3 — API key middleware for /v1/signals ingest)]
tech_stack:
  added:
    - github.com/mr-tron/base58 (v1.2.0) — base58 encoding for high-entropy keys
  patterns:
    - PgAPIKeyStore following UserStore interface pattern
    - Package-level ValidateAPIKey function (not method receiver)
    - Non-blocking best-effort last_used_at updates
key_files:
  created:
    - migrations/009_api_keys.up.sql — api_keys table schema
    - migrations/009_api_keys.down.sql — migration rollback
    - internal/auth/apikeys.go — GenerateAPIKey, HashAPIKey, ValidateAPIKey
    - internal/auth/apikeys_store.go — PgAPIKeyStore implementation
    - internal/auth/apikeys_test.go — unit tests (8 behaviors, table-driven)
    - internal/ingest/handler_apikeys.go — REST handlers
  modified:
    - go.mod — added github.com/mr-tron/base58
    - internal/ingest/handler_auth.go — added APIKeyStore to AuthService
    - internal/ingest/receiver_query.go — added /api-keys routes under /api/v1
decisions:
  - APIKeyStore injected via AuthService (consistent with UserStore pattern)
  - ValidateAPIKey is package-level function, not method (Phase 6 Wave 3 middleware calls auth.ValidateAPIKey(ctx, store, rawKey))
  - TouchLastUsed uses non-blocking goroutine submission (_ = store.TouchLastUsed) to prevent validation latency impact
  - Key hash uses sha256 without salt (32-byte random key provides sufficient entropy)
  - POST /api/v1/api-keys restricted to admin/analyst roles (SDKs created by operators, not viewers)
metrics:
  duration: ~15 minutes (execution start to completion)
  completed_date: 2026-04-24
  tasks_completed: 3/3
  files_created: 8
  files_modified: 3
  test_coverage: 8 unit tests covering generation, hashing, validation paths

# Phase 6 Plan 3: API Key Schema & CRUD

## Objective

Introduce first-class API keys for machine identity (SDK → ingest). Separate from user JWTs. Hashed at rest, shown plaintext exactly once at creation.

## Summary

Wave 2 of Phase 6 (Security Hardening) completes the API key subsystem: database schema (migration 009), cryptographic primitives, and REST CRUD operations. This plan delivers everything SDKs need to authenticate with the ingest layer (to be wired in Phase 6 Wave 3, plan 06-05).

## What Was Built

### Task 1: Migration 009 — api_keys Table

**Status:** COMPLETE

Created migration 009 (following migration 008_core_tables) with:
- api_keys table: id (UUID PK), user_id (FK to users), app_id (optional FK to apps)
- Columns: name, key_prefix (VARCHAR 12), key_hash (VARCHAR 64 UNIQUE), scopes (TEXT[]), last_used_at, expires_at, revoked_at, created_at
- Indexes: idx_api_keys_hash (for O(1) lookups), idx_api_keys_user (for list by user)
- Down migration: DROP TABLE IF EXISTS api_keys

Key decisions:
- key_hash is UNIQUE (only one API key per hash, enforced at DB level)
- key_prefix is VARCHAR 12 (sufficient for UI display, e.g. "argus_sk_ab")
- scopes is TEXT[] (array of permission strings, e.g. ["signals:write"])
- All timestamp columns TIMESTAMP NOT NULL DEFAULT now() or NULL (not TIMESTAMPTZ, matching existing schema)

### Task 2: GenerateAPIKey / HashAPIKey / ValidateAPIKey + ApiKeyStore

**Status:** COMPLETE

Implemented core API key functions and store:

#### Crypto Primitives (internal/auth/apikeys.go)

1. **GenerateAPIKey()** (string, string, string, error)
   - Returns: (fullKey, prefix12, hash64, error)
   - fullKey: "argus_sk_" + base58(32 random bytes) — shown to user once
   - prefix12: first 12 chars of fullKey for UI display
   - hash64: sha256(fullKey) as lowercase hex, 64 chars

2. **HashAPIKey(key string)** string
   - Deterministic sha256 hex (no salt — 32-byte random key provides sufficient entropy)
   - Always lowercase, always 64 chars

3. **ValidateAPIKey(ctx, store, rawKey)** (*APIKey, error)
   - Package-level function (not method receiver)
   - Hashes raw key internally, looks up via store.GetByHash
   - Returns ErrAPIKeyNotFound if not found
   - Returns ErrAPIKeyRevoked if revoked_at is set
   - Returns ErrAPIKeyExpired if expires_at is in the past
   - Non-blocking: calls store.TouchLastUsed (best-effort, ignores errors)
   - Matches Phase 6 Wave 3 middleware contract

#### ApiKeyStore Interface & PgAPIKeyStore Implementation (internal/auth/apikeys_store.go)

Implements 6 operations:
- **Create(ctx, apiKey)** — INSERT with all fields
- **GetByHash(ctx, hash)** — SELECT WHERE key_hash = $1 (O(1) via index)
- **GetByID(ctx, id)** — SELECT WHERE id = $1 (lookup for ownership check)
- **ListByUser(ctx, userID)** — SELECT WHERE user_id = $1 ORDER BY created_at DESC
- **Revoke(ctx, id)** — UPDATE SET revoked_at = now()
- **TouchLastUsed(ctx, id, t)** — UPDATE SET last_used_at = $2 (non-fatal if errors)

Pattern: Follows PgUserStore style (pgxpool, scannable interface, null-safe scanning)

#### Unit Tests (internal/auth/apikeys_test.go)

8 test cases covering all behaviors:
1. GenerateAPIKey returns key with argus_sk_ prefix and length ≥ 40
2. HashAPIKey is deterministic sha256 hex, 64 chars lowercase
3. Two GenerateAPIKey calls produce different keys (32 bytes random)
4. ValidateAPIKey on valid, non-revoked, non-expired key → returns APIKey + updates last_used_at
5. ValidateAPIKey on revoked key → ErrAPIKeyRevoked
6. ValidateAPIKey on expired key → ErrAPIKeyExpired
7. ValidateAPIKey on unknown hash → ErrAPIKeyNotFound
8. Successful ValidateAPIKey updates last_used_at (verified via mock store)

Table-driven test suite verifies all validation paths. Mock ApiKeyStore (in-memory map) allows unit tests without database.

### Task 3: REST Handlers + Routing

**Status:** COMPLETE

Created handlers (internal/ingest/handler_apikeys.go) and routes (internal/ingest/receiver_query.go):

#### Handlers on QueryHandler

1. **handleCreateAPIKey** — POST /api/v1/api-keys
   - Request: { name, app_id (optional), scopes (optional), expires_at (optional) }
   - Requires: admin or analyst role (requireRole checked via middleware)
   - Returns 201: { id, key (full value shown once), prefix, name, created_at }
   - Key value never appears anywhere else (security-first)

2. **handleListAPIKeys** — GET /api/v1/api-keys
   - Query: ?user_id=X (admin only, to view other user's keys)
   - Returns 200: array of { id, name, prefix, scopes, last_used_at, expires_at, revoked_at, created_at }
   - Never includes key_hash or full key in response
   - Caller sees only own keys unless admin queries another user

3. **handleRevokeAPIKey** — DELETE /api/v1/api-keys/{id}
   - Ownership check: key.UserID == caller OR caller is admin
   - Returns 403 if non-owner non-admin attempts revoke
   - Returns 204 on success (key.revoked_at set)
   - After revocation, ValidateAPIKey will return ErrAPIKeyRevoked

#### Routes (under /api/v1 in receiver_query.go)

```go
r.Route("/api-keys", func(r chi.Router) {
    // Create: admin or analyst
    r.With(auth.RequireRole(auth.RoleAdmin, auth.RoleAnalyst)).
        Post("/", h.handleCreateAPIKey)
    // List: any authenticated user (sees own)
    r.Get("/", h.handleListAPIKeys)
    // Delete: own key OR admin — enforced inside handler
    r.Delete("/{id}", h.handleRevokeAPIKey)
})
```

#### Integration with AuthService

- Added `APIKeyStore auth.ApiKeyStore` field to ingest.AuthService struct
- QueryHandler checks `h.authService.APIKeyStore != nil` before operating (graceful degradation)
- All handlers use `auth.UserIDFromContext(r.Context())` to extract caller ID

### Architecture Decisions

1. **Separate from User JWTs:** API keys are machine identities, stored in api_keys table, validated separately from JWT refresh pipeline
2. **Hashing Strategy:** sha256(key) without salt — 32-byte random key is high-entropy enough
3. **Key Exposure:** Full key returned exactly once at creation (POST response); never in list/get
4. **Prefix for UI:** First 12 chars (e.g. "argus_sk_ab") shown in UI, allowing users to identify keys
5. **Revocation vs Deletion:** Soft revocation (revoked_at timestamp) preserves audit trail, not hard delete
6. **Expiry Enforcement:** checked at validation time (not cleanup job), allows gradual transition
7. **Last Used Tracking:** Non-blocking async update (_ = store.TouchLastUsed) enables audit without latency impact

## Verification

All tasks pass:

```bash
go build ./...                    # Clean build
go test ./internal/auth/ -run TestAPIKey -count=1 -v   # 8 tests pass
```

Must-haves checklist:
- [x] POST /api/v1/api-keys returns new key value exactly once
- [x] GET /api/v1/api-keys returns caller's keys only, never key_hash or value
- [x] DELETE /api/v1/api-keys/:id by non-owner non-admin returns 403
- [x] Revoked key fails ValidateAPIKey
- [x] Expired key fails ValidateAPIKey
- [x] Valid call to ValidateAPIKey updates last_used_at
- [x] Migration 009 named correctly (not 008 — 008_core_tables exists)
- [x] GenerateAPIKey, HashAPIKey, ValidateAPIKey all present
- [x] ApiKeyStore interface with all 6 operations
- [x] Handlers follow security-first patterns (owner checks, role checks, no key leaks)

## Deviations from Plan

None — plan executed exactly as written.

## Known Stubs

None. All deliverables are complete and wired.

## Self-Check: PASSED

All required files exist and contain expected functionality:
- ✓ migrations/009_api_keys.up.sql
- ✓ migrations/009_api_keys.down.sql
- ✓ internal/auth/apikeys.go
- ✓ internal/auth/apikeys_store.go
- ✓ internal/auth/apikeys_test.go
- ✓ internal/ingest/handler_apikeys.go
- ✓ internal/ingest/handler_auth.go (modified)
- ✓ internal/ingest/receiver_query.go (modified)
- ✓ go.mod (base58 added)

All commits present:
- ✓ 5c364cb — migration(06-03): create api_keys table with indexes
- ✓ b26674f — feat(06-03): implement API key crypto and store
- ✓ b84178a — feat(06-03): add API key REST handlers and routes

## Next Steps (Phase 6 Wave 3)

Plan 06-05 (Wave 3) will:
1. Create middleware to extract API key from Authorization header (Bearer {key})
2. Call ValidateAPIKey to look up the key
3. Wire API key identity into /v1/signals ingest pipeline
4. Allow SDKs to post signals without JWT (using API key credential)
