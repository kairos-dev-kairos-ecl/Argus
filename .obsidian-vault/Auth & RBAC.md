# Auth & RBAC

> Two auth systems: API key auth (signal ingest) + JWT auth (dashboard/API).

## Two Auth Contexts

```
Signal ingest (/v1/signals, gRPC)
  └─ API Key (X-Argus-Key header)
       └─ AuthValidator → PostgreSQL api_keys table

Dashboard + REST API (/api/v1/*)
  └─ JWT Bearer token (Authorization: Bearer <token>)
       └─ AuthMiddleware → TokenManager.Verify()
```

---

## API Key Auth

### Flow
File: `internal/ingest/auth.go` → `AuthValidator`

1. Client sends `X-Argus-Key: <key>` header
2. SHA256 hash the key
3. Query: `SELECT app_id, scopes FROM api_keys WHERE key_hash=$1 AND revoked_at IS NULL AND (expires_at IS NULL OR expires_at > now())`
4. Cache hit (in-memory, 5-min TTL) → skip PG query
5. Attach `app_id` to request context

### API Key Management
- `POST /api/v1/apps` → register app, generate key
- `GET /api/v1/apps/{id}/key` → retrieve key
- `POST /api/v1/apps/{id}/key/rotate` → revoke old, issue new

---

## JWT Auth

### Token Manager
File: `internal/auth/jwt.go` → `TokenManager`

- Algorithm: RS256 (RSA 2048-bit)
- Key generation: `keygen.go` → `LoadOrGenerateRSAKey()` — loads from disk or generates on first run
- Access token TTL: 15 minutes
- Refresh token TTL: 7 days

Token claims:
```json
{
  "sub": "<user_id>",
  "email": "<email>",
  "role": "admin|analyst|viewer",
  "permissions": ["signals:read", "alerts:write", ...],
  "jti": "<unique token ID>",
  "iat": 1234567890,
  "exp": 1234568790
}
```

### Auth Middleware
File: `internal/auth/middleware.go` → `AuthMiddleware`

Chi middleware. Applied to all `/api/v1/*` routes.
Skipped for: `/health`, `/metrics`, `/v1/signals`, `/v1/traces`, `/v1/metrics`

1. Extract `Authorization: Bearer <token>` header
2. `TokenManager.Verify(token)` → parse + validate signature + expiry
3. Check `token_revocations` table (handles logout)
4. Attach claims to request context
5. `RequireRole(roles...)` — additional middleware for role-specific endpoints
6. `RequirePermission(perm)` — fine-grained permission check

---

## RBAC

File: `internal/auth/rbac.go`

### Roles

| Role | Description |
|------|-------------|
| `admin` | Full access — users, rules, config, all data |
| `analyst` | Read signals, manage alerts/incidents, read rules |
| `viewer` | Read-only — signals, alerts, dashboard |

### Permissions (PermissionMatrix)

| Permission | admin | analyst | viewer |
|-----------|-------|---------|--------|
| `signals:read` | ✓ | ✓ | ✓ |
| `signals:write` | ✓ | ✓ | — |
| `alerts:read` | ✓ | ✓ | ✓ |
| `alerts:write` | ✓ | ✓ | — |
| `incidents:read` | ✓ | ✓ | ✓ |
| `incidents:write` | ✓ | ✓ | — |
| `rules:read` | ✓ | ✓ | ✓ |
| `rules:write` | ✓ | — | — |
| `users:read` | ✓ | — | — |
| `users:write` | ✓ | — | — |
| `config:read` | ✓ | — | — |
| `config:write` | ✓ | — | — |
| `audit:read` | ✓ | — | — |
| `query:execute` | ✓ | ✓ | ✓ |

---

## User Lifecycle

File: `internal/auth/users.go` → `UserService`
File: `internal/auth/store_pg.go` → `PgUserStore`

### PostgreSQL `users` table columns
```
id, email, password_hash (bcrypt cost 12),
role (admin/analyst/viewer),
status (active/suspended/pending_mfa),
mfa_secret (TOTP), mfa_enabled,
failed_logins, locked_until,
created_at, updated_at, last_login_at
```

### First-Run Setup
File: `internal/auth/setup.go`

On first start with empty `users` table:
- API serves `POST /api/v1/auth/setup`
- Creates admin user with provided email + password
- After setup, endpoint is disabled

---

## Session Management

File: `internal/auth/sessions.go` + `store_pg.go`

PostgreSQL `sessions` table:
```
id, user_id, refresh_token_hash, user_agent, ip_address, created_at, expires_at
```

Login flow:
1. `POST /api/v1/auth/login` → validate credentials → issue access_token + refresh_token
2. Refresh token stored as hash in `sessions` table
3. Refresh token set as `HttpOnly` cookie

Silent refresh:
1. `POST /api/v1/auth/refresh` → validate refresh cookie → verify session not revoked → issue new access_token

Logout:
1. `POST /api/v1/auth/logout` → add `jti` to `token_revocations` table + delete session

---

## Audit Log

File: `internal/auth/audit.go` → `AuditLogger`

Every state-changing action logged to PostgreSQL `audit_log`:
```
id, user_id, action, resource_type, resource_id,
before (JSONB), after (JSONB),
ip_address, user_agent, timestamp
```

Exposed via: `GET /api/v1/audit` (admin only)

---

## File Map

| File | Component |
|------|-----------|
| `internal/auth/jwt.go` | TokenManager (RS256 issue/verify) |
| `internal/auth/middleware.go` | Chi middleware (Bearer + RequireRole) |
| `internal/auth/rbac.go` | Role definitions + PermissionMatrix |
| `internal/auth/users.go` | UserService (business logic) |
| `internal/auth/store_pg.go` | PgUserStore, PgSessionStore, PgAuditStore |
| `internal/auth/sessions.go` | SessionManager |
| `internal/auth/setup.go` | First-run admin creation |
| `internal/auth/keygen.go` | RSA key load/generate |
| `internal/auth/audit.go` | AuditLogger |
| `internal/ingest/auth.go` | AuthValidator (API key, ingest path) |
