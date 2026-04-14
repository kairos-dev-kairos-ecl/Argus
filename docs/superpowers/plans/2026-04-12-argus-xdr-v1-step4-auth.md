# Argus XDR v1.0 — Step 4: Auth System

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Fix the non-compiling auth package, implement PostgreSQL-backed UserStore/SessionStore/AuditStore, wire live auth HTTP handlers (login/refresh/logout/setup), add RBAC middleware gating protected routes, and implement live user management handlers — replacing all 501 auth and user stubs.

**Gate (Definition of Done):**
```
go build ./cmd/... ./internal/...    # zero errors
go test ./internal/auth/...          # all tests pass
```

Manual smoke test (after `docker compose up`):
- `POST /api/v1/auth/setup` → 200 with API key (first call only)
- `POST /api/v1/auth/login` → 200 with access token + HttpOnly refresh cookie
- `GET /api/v1/alerts` without token → 401
- `GET /api/v1/alerts` with valid token → 200 or 503 (not 401)
- `POST /api/v1/auth/refresh` with cookie → 200 with new token
- `POST /api/v1/auth/logout` → 200, cookie cleared

**Architecture:** The auth system is already fully designed in `internal/auth/`. It just doesn't compile and has no concrete store implementations or HTTP handlers. This step:
1. Fixes the 6 compilation errors
2. Adds `internal/auth/store_pg.go` — concrete pgx implementations of `UserStore`, `SessionStore`, `AuditStore`
3. Adds `internal/ingest/handler_auth.go` — live auth HTTP handlers
4. Wires auth middleware into `cmd/argus/api.go` so protected routes require a valid JWT

**Tech Stack:** Go, pgx/v5, golang-jwt/jwt/v5, bcrypt, chi — all already in go.mod

---

## File Map

| File | Action | Responsibility |
|------|--------|---------------|
| `internal/auth/jwt.go` | Modify | Remove `claims.Valid()` call (removed in jwt/v5) |
| `internal/auth/audit.go` | Modify | Fix `GetUserFromContext(ctx)` — add context-aware helper |
| `internal/auth/sessions.go` | Modify | Fix `oldestSession = s` (value→pointer) and `*s` dereference |
| `internal/auth/middleware.go` | Modify | Remove unused `"fmt"` import; implement `hashToken`/`currentUnixTime` |
| `internal/auth/setup.go` | Modify | Remove unused `"strings"` import |
| `internal/auth/store_pg.go` | Create | pgx implementations of UserStore, SessionStore, AuditStore |
| `internal/auth/store_pg_test.go` | Create | Unit tests for store logic (no real DB; test helpers only) |
| `internal/ingest/handler_auth.go` | Create | Live handlers: login, refresh, logout, setup, change-password |
| `internal/ingest/handler_users.go` | Create | Live handlers: list users, create user |
| `internal/ingest/handler_audit.go` | Create | Live handler: GET /api/v1/audit |
| `cmd/argus/api.go` | Modify | Wire auth middleware, setup manager, user service into router |

**Do NOT modify:** `internal/auth/rbac.go`, `internal/auth/users.go`, `internal/auth/jwt_test.go`, `internal/auth/rbac_test.go`.

---

### Task 1: Fix Auth Package Compilation

**Files:**
- Modify: `internal/auth/jwt.go` (line 103)
- Modify: `internal/auth/audit.go` (line 74)
- Modify: `internal/auth/sessions.go` (lines 51, 164)
- Modify: `internal/auth/middleware.go` (unused import + TODOs)
- Modify: `internal/auth/setup.go` (unused import)

**Root cause analysis of each error:**

1. **`jwt.go:103` — `claims.Valid undefined`**
   `golang-jwt/jwt/v5` removed the `Valid()` method from `Claims`. `ParseWithClaims` already validates all standard claims (exp, nbf, iss, aud). Remove lines 102–104.

2. **`audit.go:74` — wrong context type for `GetUserFromContext`**
   `GetUserFromContext(r *http.Request)` is defined in `middleware.go` and takes `*http.Request`. `LogAction` has `ctx context.Context`, not a request. Add a new exported helper `GetClaimsFromContext(ctx context.Context) *Claims` in `middleware.go` that reads `ContextKeyUser` directly from the context value. Update `audit.go` to call it.

3. **`sessions.go:51` — `oldestSession = s` (Session value assigned to *Session)**
   Change the loop to track value (not pointer):
   ```go
   var oldestSession Session
   found := false
   for _, s := range existingSessions {
       if s.RevokedAt == nil {
           if !found || s.CreatedAt < oldestSession.CreatedAt {
               oldestSession = s
               found = true
           }
       }
   }
   if found {
       if err := sm.sessionStore.RevokeSession(ctx, oldestSession.ID); err != nil { ... }
   }
   ```

4. **`sessions.go:164` — `*s` where s is already Session value**
   ```go
   activeSessions = append(activeSessions, *s)  // wrong
   activeSessions = append(activeSessions, s)   // correct
   ```

5. **`middleware.go` — unused `"fmt"` import**
   Remove `"fmt"` from imports. Also implement the two TODO stubs:
   ```go
   func hashToken(token string) string {
       h := sha256.Sum256([]byte(token))
       return hex.EncodeToString(h[:])
   }
   func currentUnixTime() int64 { return time.Now().Unix() }
   ```
   Add `"crypto/sha256"`, `"encoding/hex"`, `"time"` to imports.

6. **`setup.go` — unused `"strings"` import**
   Remove `"strings"` from imports.

- [ ] **Step 1.1: Run failing build to confirm errors**

```bash
cd C:/Users/Drupad/ArgusXDR/.worktrees/step2-detection-engine
go build ./internal/auth/... 2>&1
```

Expected: 6 errors listed above.

- [ ] **Step 1.2: Fix `internal/auth/jwt.go`**

Remove lines 102–104 (the `claims.Valid()` block):

```go
// DELETE these three lines:
if err := claims.Valid(); err != nil {
    return nil, fmt.Errorf("claims validation failed: %w", err)
}
```

- [ ] **Step 1.3: Fix `internal/auth/middleware.go`**

Remove `"fmt"` from imports. Add `"crypto/sha256"`, `"encoding/hex"`, `"time"` to imports.

Add new exported helper after `GetUserFromContext`:
```go
// GetClaimsFromContext retrieves Claims from a plain context.Context.
// Used by audit logging code that has context but not a *http.Request.
func GetClaimsFromContext(ctx context.Context) *Claims {
    claims, ok := ctx.Value(ContextKeyUser).(*Claims)
    if !ok {
        return nil
    }
    return claims
}
```

Replace the `hashToken` stub:
```go
func hashToken(token string) string {
    h := sha256.Sum256([]byte(token))
    return hex.EncodeToString(h[:])
}
```

Replace the `currentUnixTime` stub:
```go
func currentUnixTime() int64 { return time.Now().Unix() }
```

- [ ] **Step 1.4: Fix `internal/auth/audit.go`**

Change line 74 from:
```go
if claims := GetUserFromContext(ctx); claims != nil {
```
To:
```go
if claims := GetClaimsFromContext(ctx); claims != nil {
```

Also fix line 80 — `ctx.Value(http.Request{})` is wrong (uses value type as key, and `ctx` not `r`). The context holds `*http.Request` at the handler level, but `LogAction` receives `context.Context` — there is no request in context. Simplify: remove the IP/UserAgent extraction from context entirely (callers that have a request will pass IP/UserAgent via `detail`):

Replace lines 79–83:
```go
// Extract IP and User-Agent if this is an HTTP request
var ipAddress, userAgent string
if req, ok := ctx.Value(http.Request{}).(*http.Request); ok {
    ipAddress = getClientIP(req)
    userAgent = req.UserAgent()
}
```
With:
```go
// Callers that have an *http.Request pass IP/UserAgent in the detail map.
var ipAddress, userAgent string
```

Remove `"net/http"` from audit.go imports (no longer used). Keep `"net"` if `getClientIP` is still there — but `getClientIP` uses `net.SplitHostPort` and is used by HTTP handlers that call `LogLogin` with an explicit IP, so keep it. But `"net/http"` must be removed since it was only used for the context key extraction.

Actually `getClientIP(r *http.Request)` at lines 216–229 still takes `*http.Request` — it's called by handlers not by `LogAction`. Keep `"net/http"` import for that function. Just fix the bad `ctx.Value(http.Request{})` usage.

- [ ] **Step 1.5: Fix `internal/auth/sessions.go`**

Replace lines 47–56 (the concurrent session eviction loop) with value-based tracking:
```go
// If at limit, revoke oldest session
if len(existingSessions) >= sm.maxConcurrentSessions {
    var oldestSession Session
    found := false
    for _, s := range existingSessions {
        if s.RevokedAt == nil {
            if !found || s.CreatedAt < oldestSession.CreatedAt {
                oldestSession = s
                found = true
            }
        }
    }
    if found {
        if err := sm.sessionStore.RevokeSession(ctx, oldestSession.ID); err != nil {
            return "", "", fmt.Errorf("failed to revoke old session: %w", err)
        }
    }
}
```

Fix line 164 — remove the dereference (`*s` → `s`):
```go
activeSessions = append(activeSessions, s)
```

- [ ] **Step 1.6: Fix `internal/auth/setup.go`**

Remove `"strings"` from imports (it is never used in this file).

- [ ] **Step 1.7: Run build to verify compilation fixed**

```bash
go build ./internal/auth/... 2>&1
```

Expected: zero errors.

- [ ] **Step 1.8: Run existing auth tests**

```bash
go test ./internal/auth/... -v 2>&1
```

Expected: all pass (jwt_test.go, rbac_test.go).

- [ ] **Step 1.9: Commit**

```bash
git add internal/auth/jwt.go internal/auth/audit.go internal/auth/sessions.go \
        internal/auth/middleware.go internal/auth/setup.go
git commit -m "fix(auth): repair 6 compilation errors (jwt/v5 Valid removed, context type mismatch, session pointer bugs, unused imports)

- jwt.go: remove claims.Valid() — deleted in golang-jwt/v5 (ParseWithClaims validates)
- middleware.go: remove unused fmt import; implement hashToken (SHA256) and currentUnixTime
- middleware.go: add GetClaimsFromContext(ctx context.Context) for non-HTTP callers
- audit.go: switch GetUserFromContext → GetClaimsFromContext; remove bad ctx.Value(http.Request{})
- sessions.go: fix oldestSession pointer/value mismatch (use value tracking, not *Session)
- sessions.go: fix *s dereference (s is Session value in range, not pointer)
- setup.go: remove unused strings import

Co-Authored-By: Claude Sonnet 4.6 <noreply@anthropic.com>"
```

---

### Task 2: PostgreSQL Store Implementations

**Files:**
- Create: `internal/auth/store_pg.go`
- Create: `internal/auth/store_pg_test.go`

Implements the three interfaces (`UserStore`, `SessionStore`, `AuditStore`) against the migration 007 PostgreSQL schema. No new tables — just SQL against `users`, `sessions`, `audit_log`, `token_revocations`.

Key schema facts from `007_auth.up.sql`:
- `users`: `id UUID, email TEXT UNIQUE, display_name TEXT, role TEXT, password_hash VARCHAR(255), status VARCHAR(50), failed_login_count INT, locked_until TIMESTAMP (nullable), created_at TIMESTAMP, updated_at TIMESTAMP, ...`
- `sessions`: `id UUID, user_id UUID, refresh_token_hash VARCHAR(255), created_at TIMESTAMP, expires_at TIMESTAMP, revoked_at TIMESTAMP (nullable), last_used_at TIMESTAMP`
- `audit_log`: `id UUID, user_id UUID (nullable), action VARCHAR, resource VARCHAR, detail JSONB, ip_address VARCHAR, user_agent TEXT, timestamp TIMESTAMP`
- `token_revocations`: `token_hash VARCHAR PRIMARY KEY, revoked_at TIMESTAMP, expires_at TIMESTAMP`

- [ ] **Step 2.1: Write the failing test**

Create `internal/auth/store_pg_test.go`:

```go
package auth

import (
    "testing"
    "time"

    "github.com/stretchr/testify/assert"
)

// These tests validate struct shapes and helper logic without a real DB.
// Integration tests with a real DB are left for a dedicated test harness.

func TestSession_IsExpired(t *testing.T) {
    past := time.Now().Add(-1 * time.Hour).Unix()
    future := time.Now().Add(1 * time.Hour).Unix()
    revoked := time.Now().Unix()

    active := Session{ID: "s1", ExpiresAt: future, RevokedAt: nil}
    assert.False(t, active.RevokedAt != nil)
    assert.True(t, active.ExpiresAt > time.Now().Unix())

    expired := Session{ID: "s2", ExpiresAt: past, RevokedAt: nil}
    assert.False(t, expired.ExpiresAt > time.Now().Unix())

    revSess := Session{ID: "s3", ExpiresAt: future, RevokedAt: &revoked}
    assert.True(t, revSess.RevokedAt != nil)
}

func TestPgUserStore_nilPool(t *testing.T) {
    store := &PgUserStore{db: nil}
    assert.NotNil(t, store)
}

func TestPgSessionStore_nilPool(t *testing.T) {
    store := &PgSessionStore{db: nil}
    assert.NotNil(t, store)
}

func TestPgAuditStore_nilPool(t *testing.T) {
    store := &PgAuditStore{db: nil}
    assert.NotNil(t, store)
}
```

- [ ] **Step 2.2: Run test to verify it fails**

```bash
go test ./internal/auth/... -run TestPg -v 2>&1
```

Expected: FAIL — `PgUserStore`, `PgSessionStore`, `PgAuditStore` undefined.

- [ ] **Step 2.3: Create `internal/auth/store_pg.go`**

```go
package auth

import (
    "context"
    "encoding/json"
    "fmt"
    "time"

    "github.com/google/uuid"
    "github.com/jackc/pgx/v5"
    "github.com/jackc/pgx/v5/pgxpool"
)

// ---------- PgUserStore ----------

// PgUserStore implements UserStore against the users table (migration 007).
type PgUserStore struct {
    db *pgxpool.Pool
}

// NewPgUserStore creates a new PgUserStore.
func NewPgUserStore(db *pgxpool.Pool) *PgUserStore {
    return &PgUserStore{db: db}
}

func (s *PgUserStore) CreateUser(ctx context.Context, user *User) error {
    if s.db == nil {
        return fmt.Errorf("database unavailable")
    }
    _, err := s.db.Exec(ctx, `
        INSERT INTO users (id, email, display_name, role, password_hash, status,
            password_changed_at, failed_login_count, created_by, created_at, updated_at)
        VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11)
    `,
        user.ID, user.Email, user.DisplayName, user.Role, user.PasswordHash,
        user.Status, user.PasswordChangedAt, user.FailedLoginCount, user.CreatedBy,
        user.CreatedAt, user.UpdatedAt,
    )
    return err
}

func (s *PgUserStore) GetUserByID(ctx context.Context, id uuid.UUID) (*User, error) {
    if s.db == nil {
        return nil, fmt.Errorf("database unavailable")
    }
    row := s.db.QueryRow(ctx, `
        SELECT id, email, display_name, role, password_hash, status,
               password_changed_at, last_login_at, last_login_ip,
               failed_login_count, locked_until, created_by,
               invited_at, mfa_secret, external_provider, external_id,
               created_at, updated_at
        FROM users WHERE id = $1
    `, id)
    return scanUser(row)
}

func (s *PgUserStore) GetUserByEmail(ctx context.Context, email string) (*User, error) {
    if s.db == nil {
        return nil, fmt.Errorf("database unavailable")
    }
    row := s.db.QueryRow(ctx, `
        SELECT id, email, display_name, role, password_hash, status,
               password_changed_at, last_login_at, last_login_ip,
               failed_login_count, locked_until, created_by,
               invited_at, mfa_secret, external_provider, external_id,
               created_at, updated_at
        FROM users WHERE email = $1
    `, email)
    u, err := scanUser(row)
    if err != nil {
        if err == pgx.ErrNoRows {
            return nil, nil // not found → nil, nil (caller checks nil)
        }
        return nil, err
    }
    return u, nil
}

func (s *PgUserStore) ListUsers(ctx context.Context, filters map[string]interface{}) ([]*User, error) {
    if s.db == nil {
        return nil, fmt.Errorf("database unavailable")
    }
    rows, err := s.db.Query(ctx, `
        SELECT id, email, display_name, role, password_hash, status,
               password_changed_at, last_login_at, last_login_ip,
               failed_login_count, locked_until, created_by,
               invited_at, mfa_secret, external_provider, external_id,
               created_at, updated_at
        FROM users ORDER BY created_at DESC LIMIT 1000
    `)
    if err != nil {
        return nil, err
    }
    defer rows.Close()

    var users []*User
    for rows.Next() {
        u, err := scanUser(rows)
        if err != nil {
            return nil, err
        }
        users = append(users, u)
    }
    return users, rows.Err()
}

func (s *PgUserStore) UpdateUser(ctx context.Context, user *User) error {
    if s.db == nil {
        return fmt.Errorf("database unavailable")
    }
    _, err := s.db.Exec(ctx, `
        UPDATE users SET
            display_name = $2, role = $3, password_hash = $4, status = $5,
            password_changed_at = $6, last_login_at = $7, last_login_ip = $8,
            failed_login_count = $9, locked_until = $10, updated_at = $11
        WHERE id = $1
    `,
        user.ID, user.DisplayName, user.Role, user.PasswordHash, user.Status,
        user.PasswordChangedAt, user.LastLoginAt, user.LastLoginIP,
        user.FailedLoginCount, user.LockedUntil, time.Now(),
    )
    return err
}

func (s *PgUserStore) DeleteUser(ctx context.Context, id uuid.UUID) error {
    if s.db == nil {
        return fmt.Errorf("database unavailable")
    }
    _, err := s.db.Exec(ctx, `UPDATE users SET status='suspended', updated_at=now() WHERE id=$1`, id)
    return err
}

func (s *PgUserStore) UpdateLoginAttempt(ctx context.Context, userID uuid.UUID, success bool, ip string) error {
    if s.db == nil {
        return nil // non-fatal
    }
    if success {
        _, err := s.db.Exec(ctx, `
            UPDATE users SET failed_login_count=0, last_login_at=now(), last_login_ip=$2, updated_at=now()
            WHERE id=$1
        `, userID, ip)
        return err
    }
    _, err := s.db.Exec(ctx, `
        UPDATE users SET failed_login_count=failed_login_count+1, updated_at=now()
        WHERE id=$1
    `, userID)
    return err
}

func (s *PgUserStore) UnlockUser(ctx context.Context, userID uuid.UUID) error {
    if s.db == nil {
        return fmt.Errorf("database unavailable")
    }
    _, err := s.db.Exec(ctx, `
        UPDATE users SET locked_until=NULL, failed_login_count=0, updated_at=now()
        WHERE id=$1
    `, userID)
    return err
}

// scanUser scans a single user row (shared by Query and QueryRow).
type scannable interface {
    Scan(dest ...any) error
}

func scanUser(row scannable) (*User, error) {
    u := &User{}
    err := row.Scan(
        &u.ID, &u.Email, &u.DisplayName, &u.Role, &u.PasswordHash, &u.Status,
        &u.PasswordChangedAt, &u.LastLoginAt, &u.LastLoginIP,
        &u.FailedLoginCount, &u.LockedUntil, &u.CreatedBy,
        &u.InvitedAt, &u.MFASecret, &u.ExternalProvider, &u.ExternalID,
        &u.CreatedAt, &u.UpdatedAt,
    )
    if err != nil {
        return nil, err
    }
    return u, nil
}

// ---------- PgSessionStore ----------

// PgSessionStore implements SessionStore against the sessions table (migration 007).
type PgSessionStore struct {
    db *pgxpool.Pool
}

// NewPgSessionStore creates a new PgSessionStore.
func NewPgSessionStore(db *pgxpool.Pool) *PgSessionStore {
    return &PgSessionStore{db: db}
}

func (s *PgSessionStore) GetSessionByUserID(ctx context.Context, userID string) ([]Session, error) {
    if s.db == nil {
        return nil, nil
    }
    rows, err := s.db.Query(ctx, `
        SELECT id, user_id, refresh_token_hash, user_agent, ip_address,
               EXTRACT(EPOCH FROM created_at)::bigint,
               EXTRACT(EPOCH FROM expires_at)::bigint,
               EXTRACT(EPOCH FROM revoked_at)::bigint,
               EXTRACT(EPOCH FROM last_used_at)::bigint
        FROM sessions WHERE user_id=$1 ORDER BY created_at ASC
    `, userID)
    if err != nil {
        return nil, err
    }
    defer rows.Close()

    var sessions []Session
    for rows.Next() {
        sess, err := scanSession(rows)
        if err != nil {
            return nil, err
        }
        sessions = append(sessions, *sess)
    }
    return sessions, rows.Err()
}

func (s *PgSessionStore) GetSessionByHash(ctx context.Context, hash string) (*Session, error) {
    if s.db == nil {
        return nil, fmt.Errorf("database unavailable")
    }
    row := s.db.QueryRow(ctx, `
        SELECT id, user_id, refresh_token_hash, user_agent, ip_address,
               EXTRACT(EPOCH FROM created_at)::bigint,
               EXTRACT(EPOCH FROM expires_at)::bigint,
               EXTRACT(EPOCH FROM revoked_at)::bigint,
               EXTRACT(EPOCH FROM last_used_at)::bigint
        FROM sessions WHERE refresh_token_hash=$1
    `, hash)
    sess, err := scanSession(row)
    if err != nil {
        if err == pgx.ErrNoRows {
            return nil, fmt.Errorf("session not found")
        }
        return nil, err
    }
    return sess, nil
}

func (s *PgSessionStore) RevokeSession(ctx context.Context, sessionID string) error {
    if s.db == nil {
        return fmt.Errorf("database unavailable")
    }
    _, err := s.db.Exec(ctx, `UPDATE sessions SET revoked_at=now() WHERE id=$1`, sessionID)
    return err
}

func (s *PgSessionStore) CheckTokenRevocation(ctx context.Context, tokenHash string) (bool, error) {
    if s.db == nil {
        return false, nil // can't check — allow through
    }
    var count int
    err := s.db.QueryRow(ctx, `
        SELECT COUNT(*) FROM token_revocations
        WHERE token_hash=$1 AND expires_at > now()
    `, tokenHash).Scan(&count)
    if err != nil {
        return false, err
    }
    return count > 0, nil
}

// CreateSession inserts a session into the database.
func (s *PgSessionStore) CreateSession(ctx context.Context, sess *Session) error {
    if s.db == nil {
        return fmt.Errorf("database unavailable")
    }
    _, err := s.db.Exec(ctx, `
        INSERT INTO sessions (id, user_id, refresh_token_hash, user_agent, ip_address,
            created_at, expires_at, last_used_at)
        VALUES ($1,$2,$3,$4,$5,
            to_timestamp($6), to_timestamp($7), to_timestamp($8))
    `,
        sess.ID, sess.UserID, sess.RefreshTokenHash, sess.UserAgent, sess.IPAddress,
        sess.CreatedAt, sess.ExpiresAt, sess.LastUsedAt,
    )
    return err
}

// UpdateSessionHash replaces the refresh token hash (rotation) and updates last_used_at.
func (s *PgSessionStore) UpdateSessionHash(ctx context.Context, sessionID, newHash string) error {
    if s.db == nil {
        return fmt.Errorf("database unavailable")
    }
    _, err := s.db.Exec(ctx, `
        UPDATE sessions SET refresh_token_hash=$2, last_used_at=now() WHERE id=$1
    `, sessionID, newHash)
    return err
}

// RevokeTokenHash adds a token to the revocation list with its expiry.
func (s *PgSessionStore) RevokeTokenHash(ctx context.Context, tokenHash string, expiresAt time.Time) error {
    if s.db == nil {
        return nil
    }
    _, err := s.db.Exec(ctx, `
        INSERT INTO token_revocations (token_hash, revoked_at, expires_at)
        VALUES ($1, now(), $2)
        ON CONFLICT (token_hash) DO NOTHING
    `, tokenHash, expiresAt)
    return err
}

func scanSession(row scannable) (*Session, error) {
    var revokedAt *int64
    sess := &Session{}
    err := row.Scan(
        &sess.ID, &sess.UserID, &sess.RefreshTokenHash, &sess.UserAgent, &sess.IPAddress,
        &sess.CreatedAt, &sess.ExpiresAt, &revokedAt, &sess.LastUsedAt,
    )
    if err != nil {
        return nil, err
    }
    sess.RevokedAt = revokedAt
    return sess, nil
}

// ---------- PgAuditStore ----------

// PgAuditStore implements AuditStore against the audit_log table (migration 007).
type PgAuditStore struct {
    db *pgxpool.Pool
}

// NewPgAuditStore creates a new PgAuditStore.
func NewPgAuditStore(db *pgxpool.Pool) *PgAuditStore {
    return &PgAuditStore{db: db}
}

func (s *PgAuditStore) LogEntry(ctx context.Context, entry *AuditLogEntry) error {
    if s.db == nil {
        return nil // non-fatal: degrade gracefully
    }
    detailJSON, err := json.Marshal(entry.Detail)
    if err != nil {
        detailJSON = []byte("{}")
    }
    _, err = s.db.Exec(ctx, `
        INSERT INTO audit_log (id, user_id, action, resource, detail, ip_address, user_agent, timestamp)
        VALUES ($1,$2,$3,$4,$5,$6,$7,$8)
    `,
        entry.ID, entry.UserID, entry.Action, entry.Resource,
        detailJSON, entry.IPAddress, entry.UserAgent, entry.Timestamp,
    )
    return err
}

func (s *PgAuditStore) GetEntries(ctx context.Context, filters map[string]interface{}, limit, offset int) ([]*AuditLogEntry, error) {
    if s.db == nil {
        return nil, fmt.Errorf("database unavailable")
    }
    rows, err := s.db.Query(ctx, `
        SELECT id, user_id, action, resource, detail, ip_address, user_agent, timestamp
        FROM audit_log ORDER BY timestamp DESC LIMIT $1 OFFSET $2
    `, limit, offset)
    if err != nil {
        return nil, err
    }
    defer rows.Close()
    return scanAuditEntries(rows)
}

func (s *PgAuditStore) GetEntriesByUser(ctx context.Context, userID uuid.UUID, limit, offset int) ([]*AuditLogEntry, error) {
    if s.db == nil {
        return nil, fmt.Errorf("database unavailable")
    }
    rows, err := s.db.Query(ctx, `
        SELECT id, user_id, action, resource, detail, ip_address, user_agent, timestamp
        FROM audit_log WHERE user_id=$1 ORDER BY timestamp DESC LIMIT $2 OFFSET $3
    `, userID, limit, offset)
    if err != nil {
        return nil, err
    }
    defer rows.Close()
    return scanAuditEntries(rows)
}

func (s *PgAuditStore) GetEntriesByAction(ctx context.Context, action string, limit, offset int) ([]*AuditLogEntry, error) {
    if s.db == nil {
        return nil, fmt.Errorf("database unavailable")
    }
    rows, err := s.db.Query(ctx, `
        SELECT id, user_id, action, resource, detail, ip_address, user_agent, timestamp
        FROM audit_log WHERE action=$1 ORDER BY timestamp DESC LIMIT $2 OFFSET $3
    `, action, limit, offset)
    if err != nil {
        return nil, err
    }
    defer rows.Close()
    return scanAuditEntries(rows)
}

func (s *PgAuditStore) Count(ctx context.Context, filters map[string]interface{}) (int64, error) {
    if s.db == nil {
        return 0, fmt.Errorf("database unavailable")
    }
    var count int64
    err := s.db.QueryRow(ctx, `SELECT COUNT(*) FROM audit_log`).Scan(&count)
    return count, err
}

func scanAuditEntries(rows pgx.Rows) ([]*AuditLogEntry, error) {
    var entries []*AuditLogEntry
    for rows.Next() {
        e := &AuditLogEntry{}
        var detailJSON []byte
        err := rows.Scan(
            &e.ID, &e.UserID, &e.Action, &e.Resource,
            &detailJSON, &e.IPAddress, &e.UserAgent, &e.Timestamp,
        )
        if err != nil {
            return nil, err
        }
        if len(detailJSON) > 0 {
            json.Unmarshal(detailJSON, &e.Detail)
        }
        entries = append(entries, e)
    }
    return entries, rows.Err()
}
```

- [ ] **Step 2.4: Run test to verify it passes**

```bash
go test ./internal/auth/... -run TestPg -v 2>&1
```

Expected: PASS

- [ ] **Step 2.5: Confirm full auth package builds and tests pass**

```bash
go build ./internal/auth/...
go test ./internal/auth/... -v 2>&1 | tail -20
```

Expected: zero build errors, all tests pass.

- [ ] **Step 2.6: Commit**

```bash
git add internal/auth/store_pg.go internal/auth/store_pg_test.go
git commit -m "feat(auth): add PgUserStore, PgSessionStore, PgAuditStore backed by migration 007 schema

All three stores implement their respective interfaces using pgx/v5.
Session store extras: CreateSession, UpdateSessionHash, RevokeTokenHash.
Nil-pool guards degrade gracefully (non-fatal) for all read paths.

Co-Authored-By: Claude Sonnet 4.6 <noreply@anthropic.com>"
```

---

### Task 3: RSA Key Generation Helper

**Files:**
- Create: `internal/auth/keygen.go`

The `TokenManager` requires `*rsa.PrivateKey` / `*rsa.PublicKey`. The server needs to generate or load these at startup. Add a simple helper that generates an ephemeral 2048-bit key pair (acceptable for a dev/test build; production operators set `ARGUS_JWT_PRIVATE_KEY_PEM`).

- [ ] **Step 3.1: Create `internal/auth/keygen.go`**

```go
package auth

import (
    "crypto/rand"
    "crypto/rsa"
    "crypto/x509"
    "encoding/pem"
    "fmt"
    "os"
)

// LoadOrGenerateRSAKey loads an RSA private key from the PEM string in ARGUS_JWT_PRIVATE_KEY_PEM
// or generates an ephemeral 2048-bit key if the env var is absent.
// Returns the private key and matching public key.
func LoadOrGenerateRSAKey() (*rsa.PrivateKey, *rsa.PublicKey, error) {
    pemStr := os.Getenv("ARGUS_JWT_PRIVATE_KEY_PEM")
    if pemStr != "" {
        block, _ := pem.Decode([]byte(pemStr))
        if block == nil {
            return nil, nil, fmt.Errorf("ARGUS_JWT_PRIVATE_KEY_PEM: not a valid PEM block")
        }
        priv, err := x509.ParsePKCS8PrivateKey(block.Bytes)
        if err != nil {
            // Try PKCS1
            pk1, err2 := x509.ParsePKCS1PrivateKey(block.Bytes)
            if err2 != nil {
                return nil, nil, fmt.Errorf("failed to parse RSA private key: %w", err)
            }
            return pk1, &pk1.PublicKey, nil
        }
        rsaPriv, ok := priv.(*rsa.PrivateKey)
        if !ok {
            return nil, nil, fmt.Errorf("ARGUS_JWT_PRIVATE_KEY_PEM: not an RSA key")
        }
        return rsaPriv, &rsaPriv.PublicKey, nil
    }

    // Generate ephemeral key (dev/test mode)
    priv, err := rsa.GenerateKey(rand.Reader, 2048)
    if err != nil {
        return nil, nil, fmt.Errorf("failed to generate RSA key: %w", err)
    }
    return priv, &priv.PublicKey, nil
}
```

- [ ] **Step 3.2: Build to verify**

```bash
go build ./internal/auth/...
```

- [ ] **Step 3.3: Commit**

```bash
git add internal/auth/keygen.go
git commit -m "feat(auth): add LoadOrGenerateRSAKey helper for JWT signing keys

Loads from ARGUS_JWT_PRIVATE_KEY_PEM env var or generates an ephemeral
2048-bit RSA key for dev/test. Production operators set the env var.

Co-Authored-By: Claude Sonnet 4.6 <noreply@anthropic.com>"
```

---

### Task 4: Live Auth HTTP Handlers

**Files:**
- Create: `internal/ingest/handler_auth.go`
- The stubs in `handler_stubs.go` for login/refresh/logout/setup already return 501 — the new file overrides these with real implementations

**Wire the handler methods through `QueryHandler`:**
The `QueryHandler` already has `alertRouter *AlertRouter` — add `authService *AuthService` (a facade over the three auth objects). The `handleLogin`, `handleRefreshToken`, `handleLogout`, `handleSetup` methods currently live in `handler_stubs.go`. The new `handler_auth.go` must re-define them in the same `ingest` package. Remove the stub versions from `handler_stubs.go`.

**AuthService** is a convenience wrapper that holds `UserService`, `SessionManager`, `TokenManager`, `AuditLogger` so the handler only needs one dependency.

- [ ] **Step 4.1: Write the failing test**

Create `internal/ingest/handler_auth_test.go`:

```go
package ingest

import (
    "bytes"
    "encoding/json"
    "net/http"
    "net/http/httptest"
    "testing"

    "github.com/stretchr/testify/assert"
    "github.com/stretchr/testify/require"
    "go.uber.org/zap"
)

func TestHandleSetup_NoAuthService_503(t *testing.T) {
    h := NewQueryHandler(nil, nil, zap.NewNop())
    // authService not set → 503
    body := `{"email":"admin@example.com","password":"supersecret1234","display_name":"Admin","instance_name":"test","app_name":"myapp"}`
    req := httptest.NewRequest(http.MethodPost, "/api/v1/auth/setup", bytes.NewBufferString(body))
    req.Header.Set("Content-Type", "application/json")
    w := httptest.NewRecorder()
    h.handleSetup(w, req)
    assert.Equal(t, http.StatusServiceUnavailable, w.Code)
}

func TestHandleLogin_NoAuthService_503(t *testing.T) {
    h := NewQueryHandler(nil, nil, zap.NewNop())
    body := `{"email":"user@example.com","password":"pass"}`
    req := httptest.NewRequest(http.MethodPost, "/api/v1/auth/login", bytes.NewBufferString(body))
    req.Header.Set("Content-Type", "application/json")
    w := httptest.NewRecorder()
    h.handleLogin(w, req)
    assert.Equal(t, http.StatusServiceUnavailable, w.Code)
}

func TestHandleRefreshToken_NoAuthService_503(t *testing.T) {
    h := NewQueryHandler(nil, nil, zap.NewNop())
    req := httptest.NewRequest(http.MethodPost, "/api/v1/auth/refresh", nil)
    w := httptest.NewRecorder()
    h.handleRefreshToken(w, req)
    assert.Equal(t, http.StatusServiceUnavailable, w.Code)
}

func TestHandleLogout_NoAuthService_200(t *testing.T) {
    h := NewQueryHandler(nil, nil, zap.NewNop())
    req := httptest.NewRequest(http.MethodPost, "/api/v1/auth/logout", nil)
    w := httptest.NewRecorder()
    h.handleLogout(w, req)
    // logout with no session is always 200 (idempotent)
    assert.Equal(t, http.StatusOK, w.Code)
}

func TestAuthServiceAvailable_nilCheck(t *testing.T) {
    h := NewQueryHandler(nil, nil, zap.NewNop())
    assert.False(t, h.authAvailable())

    h2 := NewQueryHandler(nil, nil, zap.NewNop())
    h2.SetAuthService(&AuthService{})
    assert.True(t, h2.authAvailable())
}

func TestLoginRequest_Decode(t *testing.T) {
    body := `{"email":"a@b.com","password":"pw"}`
    var req loginRequest
    err := json.NewDecoder(bytes.NewBufferString(body)).Decode(&req)
    require.NoError(t, err)
    assert.Equal(t, "a@b.com", req.Email)
    assert.Equal(t, "pw", req.Password)
}
```

- [ ] **Step 4.2: Run test to verify it fails**

```bash
go test ./internal/ingest/... -run TestHandleLogin -run TestHandleSetup -run TestHandleRefresh -run TestHandleLogout -run TestAuthService -run TestLoginRequest -v 2>&1
```

Expected: FAIL — `AuthService`, `SetAuthService`, `authAvailable`, `loginRequest` undefined.

- [ ] **Step 4.3: Remove auth stubs from `handler_stubs.go`**

Delete the four auth handler stubs from `internal/ingest/handler_stubs.go`:
- `handleLogin`
- `handleRefreshToken`
- `handleLogout`
- `handleSetup`

These will be re-implemented in `handler_auth.go`.

- [ ] **Step 4.4: Add `authService` field to `QueryHandler` in `receiver_query.go`**

In `internal/ingest/receiver_query.go`, add to the `QueryHandler` struct:
```go
authService *AuthService
```

And add the setter and availability check:
```go
// SetAuthService wires the auth subsystem into the query handler.
func (h *QueryHandler) SetAuthService(svc *AuthService) {
    h.authService = svc
}

// authAvailable returns true if the auth service is configured.
func (h *QueryHandler) authAvailable() bool {
    return h.authService != nil
}
```

- [ ] **Step 4.5: Create `internal/ingest/handler_auth.go`**

```go
package ingest

import (
    "encoding/json"
    "net/http"
    "time"

    "github.com/argusxdr/argus/internal/auth"
    "go.uber.org/zap"
)

// AuthService is a convenience facade holding all auth subsystem components.
// Injected into QueryHandler via SetAuthService.
type AuthService struct {
    UserService    *auth.UserService
    SessionMgr     *auth.SessionManager
    TokenMgr       *auth.TokenManager
    AuditLogger    *auth.AuditLogger
    SessionStore   *auth.PgSessionStore
}

// ---- request/response types ----

type loginRequest struct {
    Email    string `json:"email"`
    Password string `json:"password"`
}

type loginResponse struct {
    AccessToken string `json:"access_token"`
    ExpiresIn   int    `json:"expires_in"` // seconds
    TokenType   string `json:"token_type"`
}

type setupRequest struct {
    Email        string `json:"email"`
    Password     string `json:"password"`
    DisplayName  string `json:"display_name"`
    InstanceName string `json:"instance_name"`
    AppName      string `json:"app_name"`
}

// ---- handlers ----

func (h *QueryHandler) handleSetup(w http.ResponseWriter, r *http.Request) {
    if !h.authAvailable() {
        jsonError(w, "auth service unavailable", http.StatusServiceUnavailable)
        return
    }

    var req setupRequest
    if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
        jsonError(w, "invalid request body", http.StatusBadRequest)
        return
    }

    sm := auth.NewSetupManager(h.authService.UserService, h.authService.SessionStore.DB())
    required, err := sm.IsSetupRequired(r.Context())
    if err != nil {
        h.log.Error("setup check failed", zap.Error(err))
        jsonError(w, "internal error", http.StatusInternalServerError)
        return
    }
    if !required {
        jsonError(w, "setup already completed", http.StatusConflict)
        return
    }

    setupResp, err := sm.PerformSetup(r.Context(), &auth.SetupRequest{
        Email:        req.Email,
        Password:     req.Password,
        DisplayName:  req.DisplayName,
        InstanceName: req.InstanceName,
        AppName:      req.AppName,
    })
    if err != nil {
        jsonError(w, err.Error(), http.StatusBadRequest)
        return
    }

    w.Header().Set("Content-Type", "application/json")
    w.WriteHeader(http.StatusOK)
    json.NewEncoder(w).Encode(setupResp)
}

func (h *QueryHandler) handleLogin(w http.ResponseWriter, r *http.Request) {
    if !h.authAvailable() {
        jsonError(w, "auth service unavailable", http.StatusServiceUnavailable)
        return
    }

    var req loginRequest
    if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
        jsonError(w, "invalid request body", http.StatusBadRequest)
        return
    }
    if req.Email == "" || req.Password == "" {
        jsonError(w, "email and password required", http.StatusBadRequest)
        return
    }

    user, err := h.authService.UserService.AuthenticateUser(r.Context(), req.Email, req.Password, getIP(r))
    if err != nil {
        jsonError(w, "invalid credentials", http.StatusUnauthorized)
        return
    }

    perms := auth.NewPermissionChecker().GetPermissionsForRole(user.Role)
    accessToken, err := h.authService.TokenMgr.IssueAccessToken(
        user.ID, user.Email, user.DisplayName, user.Role, perms,
    )
    if err != nil {
        h.log.Error("failed to issue access token", zap.Error(err))
        jsonError(w, "internal error", http.StatusInternalServerError)
        return
    }

    refreshToken, sessionID, err := h.authService.SessionMgr.CreateSession(
        r.Context(), user.ID, r.UserAgent(), getIP(r),
    )
    if err != nil {
        h.log.Warn("failed to create session", zap.Error(err))
        // Non-fatal: still return access token
    } else if h.authService.SessionStore != nil {
        now := time.Now()
        sess := &auth.Session{
            ID:               sessionID,
            UserID:           user.ID.String(),
            RefreshTokenHash: h.authService.SessionMgr.HashToken(refreshToken),
            UserAgent:        r.UserAgent(),
            IPAddress:        getIP(r),
            CreatedAt:        now.Unix(),
            ExpiresAt:        now.Add(7 * 24 * time.Hour).Unix(),
            LastUsedAt:       now.Unix(),
        }
        if err := h.authService.SessionStore.CreateSession(r.Context(), sess); err != nil {
            h.log.Warn("failed to persist session", zap.Error(err))
        }

        http.SetCookie(w, &http.Cookie{
            Name:     "refresh_token",
            Value:    refreshToken,
            Path:     "/",
            HttpOnly: true,
            Secure:   r.TLS != nil,
            SameSite: http.SameSiteLaxMode,
            MaxAge:   int((7 * 24 * time.Hour).Seconds()),
        })
    }

    if h.authService.AuditLogger != nil {
        h.authService.AuditLogger.LogLogin(r.Context(), user.ID, true, getIP(r))
    }

    w.Header().Set("Content-Type", "application/json")
    w.WriteHeader(http.StatusOK)
    json.NewEncoder(w).Encode(loginResponse{
        AccessToken: accessToken,
        ExpiresIn:   int((15 * time.Minute).Seconds()),
        TokenType:   "Bearer",
    })
}

func (h *QueryHandler) handleRefreshToken(w http.ResponseWriter, r *http.Request) {
    if !h.authAvailable() {
        jsonError(w, "auth service unavailable", http.StatusServiceUnavailable)
        return
    }

    cookie, err := r.Cookie("refresh_token")
    if err != nil || cookie.Value == "" {
        jsonError(w, "missing refresh token", http.StatusUnauthorized)
        return
    }

    oldHash := h.authService.SessionMgr.HashToken(cookie.Value)
    sess, err := h.authService.SessionStore.GetSessionByHash(r.Context(), oldHash)
    if err != nil || sess == nil {
        jsonError(w, "invalid refresh token", http.StatusUnauthorized)
        return
    }
    if sess.RevokedAt != nil || sess.ExpiresAt < time.Now().Unix() {
        jsonError(w, "refresh token expired or revoked", http.StatusUnauthorized)
        return
    }

    // Rotate: generate new token, update DB
    newToken, newHash, err := h.authService.SessionMgr.RotateRefreshToken(r.Context(), oldHash)
    if err != nil {
        jsonError(w, "could not rotate refresh token", http.StatusInternalServerError)
        return
    }
    if err := h.authService.SessionStore.UpdateSessionHash(r.Context(), sess.ID, newHash); err != nil {
        h.log.Warn("failed to persist rotated session hash", zap.Error(err))
    }

    // Fetch user to re-issue access token
    userID, err := parseUUID(sess.UserID)
    if err != nil {
        jsonError(w, "invalid session", http.StatusInternalServerError)
        return
    }
    user, err := h.authService.UserService.Store().GetUserByID(r.Context(), userID)
    if err != nil || user == nil {
        jsonError(w, "user not found", http.StatusUnauthorized)
        return
    }

    perms := auth.NewPermissionChecker().GetPermissionsForRole(user.Role)
    accessToken, err := h.authService.TokenMgr.IssueAccessToken(
        user.ID, user.Email, user.DisplayName, user.Role, perms,
    )
    if err != nil {
        jsonError(w, "internal error", http.StatusInternalServerError)
        return
    }

    http.SetCookie(w, &http.Cookie{
        Name:     "refresh_token",
        Value:    newToken,
        Path:     "/",
        HttpOnly: true,
        Secure:   r.TLS != nil,
        SameSite: http.SameSiteLaxMode,
        MaxAge:   int((7 * 24 * time.Hour).Seconds()),
    })

    w.Header().Set("Content-Type", "application/json")
    json.NewEncoder(w).Encode(loginResponse{
        AccessToken: accessToken,
        ExpiresIn:   int((15 * time.Minute).Seconds()),
        TokenType:   "Bearer",
    })
}

func (h *QueryHandler) handleLogout(w http.ResponseWriter, r *http.Request) {
    // Logout is always 200 (idempotent — can't reveal whether session existed)
    if h.authAvailable() {
        if cookie, err := r.Cookie("refresh_token"); err == nil && cookie.Value != "" {
            hash := h.authService.SessionMgr.HashToken(cookie.Value)
            if sess, err := h.authService.SessionStore.GetSessionByHash(r.Context(), hash); err == nil && sess != nil {
                h.authService.SessionStore.RevokeSession(r.Context(), sess.ID)
            }
        }
        // Revoke access token if present
        if authHeader := r.Header.Get("Authorization"); len(authHeader) > 7 {
            tokenStr := authHeader[7:]
            if claims, err := h.authService.TokenMgr.VerifyTokenSignature(tokenStr); err == nil {
                expiry := time.Now().Add(15 * time.Minute) // max access token TTL
                if claims.ExpiresAt != nil {
                    expiry = claims.ExpiresAt.Time
                }
                h.authService.SessionStore.RevokeTokenHash(r.Context(), hashTokenStr(tokenStr), expiry)
            }
        }
    }

    http.SetCookie(w, &http.Cookie{
        Name:   "refresh_token",
        Value:  "",
        Path:   "/",
        MaxAge: -1,
    })
    w.Header().Set("Content-Type", "application/json")
    w.WriteHeader(http.StatusOK)
    json.NewEncoder(w).Encode(map[string]string{"status": "logged out"})
}

// ---- helpers ----

func getIP(r *http.Request) string {
    if xff := r.Header.Get("X-Forwarded-For"); xff != "" {
        return xff
    }
    if xri := r.Header.Get("X-Real-IP"); xri != "" {
        return xri
    }
    return r.RemoteAddr
}

func parseUUID(s string) (interface{ String() string }, error) {
    // Thin wrapper to avoid importing uuid in this file
    // The actual UUID parsing is done by the caller
    _ = s
    return nil, nil
}

func hashTokenStr(token string) string {
    // Re-use the hashToken from auth package via delegation
    return auth.HashTokenPublic(token)
}
```

**Note:** The above draft will need minor adjustments:
- `UserService` needs a `Store()` getter to expose `UserStore` (add to `users.go` or just inject `UserStore` directly into `AuthService`)
- `PgSessionStore` needs a `DB()` method returning `*pgxpool.Pool` for `SetupManager`
- `auth.HashTokenPublic` needs to be exported from middleware.go (rename `hashToken` → `HashToken`)
- `parseUUID` should use `github.com/google/uuid`

Adjust the implementation as needed to fix these when running the build step. The plan intentionally identifies these coupling points so you don't miss them.

Simpler approach — make `AuthService` fully self-contained without the internal coupling:

Revised `AuthService` struct (use `UserStore` directly instead of `UserService.Store()`):
```go
type AuthService struct {
    UserSvc      *auth.UserService
    UserStore    auth.UserStore          // for GetUserByID in refresh
    SessionMgr   *auth.SessionManager
    TokenMgr     *auth.TokenManager
    AuditLog     *auth.AuditLogger
    SessionStore *auth.PgSessionStore
}
```

- [ ] **Step 4.6: Run test to verify it passes**

```bash
go test ./internal/ingest/... -run TestHandleLogin -run TestHandleSetup -run TestHandleRefresh -run TestHandleLogout -run TestAuthService -run TestLoginRequest -v 2>&1
```

Expected: PASS

- [ ] **Step 4.7: Build to confirm no new errors**

```bash
go build ./cmd/... ./internal/...
```

Expected: only pre-existing errors in unrelated packages (resilience, auth integration) — zero new errors.

- [ ] **Step 4.8: Commit**

```bash
git add internal/ingest/handler_auth.go internal/ingest/handler_auth_test.go \
        internal/ingest/receiver_query.go internal/auth/store_pg.go
git commit -m "feat(ingest): live auth handlers — login, refresh, logout, setup

AuthService facade holds UserService, SessionMgr, TokenMgr, AuditLogger.
- handleLogin: bcrypt verify → JWT issue → session create → HttpOnly cookie
- handleRefreshToken: validate cookie → rotate token → re-issue access JWT
- handleLogout: revoke session + access token → clear cookie (always 200)
- handleSetup: first-run only, creates admin user + returns API key
All return 503 when authService not wired (graceful degradation).

Co-Authored-By: Claude Sonnet 4.6 <noreply@anthropic.com>"
```

---

### Task 5: Live User Management Handlers

**Files:**
- Create: `internal/ingest/handler_users.go`
- Remove stubs: `handleListUsers`, `handleCreateUser` from `handler_stubs.go`

- [ ] **Step 5.1: Write the failing test**

Create `internal/ingest/handler_users_test.go`:

```go
package ingest

import (
    "net/http"
    "net/http/httptest"
    "testing"

    "github.com/stretchr/testify/assert"
    "go.uber.org/zap"
)

func TestHandleListUsers_NoAuthService_503(t *testing.T) {
    h := NewQueryHandler(nil, nil, zap.NewNop())
    req := httptest.NewRequest(http.MethodGet, "/api/v1/users", nil)
    w := httptest.NewRecorder()
    h.handleListUsers(w, req)
    assert.Equal(t, http.StatusServiceUnavailable, w.Code)
}

func TestHandleCreateUser_NoAuthService_503(t *testing.T) {
    h := NewQueryHandler(nil, nil, zap.NewNop())
    req := httptest.NewRequest(http.MethodPost, "/api/v1/users", nil)
    w := httptest.NewRecorder()
    h.handleCreateUser(w, req)
    assert.Equal(t, http.StatusServiceUnavailable, w.Code)
}
```

- [ ] **Step 5.2: Remove stubs from `handler_stubs.go`**

Delete `handleListUsers` and `handleCreateUser` from `internal/ingest/handler_stubs.go`.

- [ ] **Step 5.3: Create `internal/ingest/handler_users.go`**

```go
package ingest

import (
    "encoding/json"
    "net/http"

    "github.com/argusxdr/argus/internal/auth"
    "go.uber.org/zap"
)

type createUserRequest struct {
    Email       string `json:"email"`
    Password    string `json:"password"`
    DisplayName string `json:"display_name"`
    Role        string `json:"role"`
}

func (h *QueryHandler) handleListUsers(w http.ResponseWriter, r *http.Request) {
    if !h.authAvailable() {
        jsonError(w, "auth service unavailable", http.StatusServiceUnavailable)
        return
    }

    users, err := h.authService.UserStore.ListUsers(r.Context(), nil)
    if err != nil {
        h.log.Error("failed to list users", zap.Error(err))
        jsonError(w, "internal error", http.StatusInternalServerError)
        return
    }

    // Sanitize: never return password hashes
    type userResponse struct {
        ID          string  `json:"id"`
        Email       string  `json:"email"`
        DisplayName string  `json:"display_name"`
        Role        string  `json:"role"`
        Status      string  `json:"status"`
    }
    result := make([]userResponse, 0, len(users))
    for _, u := range users {
        result = append(result, userResponse{
            ID:          u.ID.String(),
            Email:       u.Email,
            DisplayName: u.DisplayName,
            Role:        u.Role,
            Status:      u.Status,
        })
    }

    w.Header().Set("Content-Type", "application/json")
    json.NewEncoder(w).Encode(map[string]interface{}{"users": result, "total": len(result)})
}

func (h *QueryHandler) handleCreateUser(w http.ResponseWriter, r *http.Request) {
    if !h.authAvailable() {
        jsonError(w, "auth service unavailable", http.StatusServiceUnavailable)
        return
    }

    var req createUserRequest
    if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
        jsonError(w, "invalid request body", http.StatusBadRequest)
        return
    }

    if req.Role == "" {
        req.Role = auth.RoleViewer
    }
    if req.Role != auth.RoleAdmin && req.Role != auth.RoleAnalyst && req.Role != auth.RoleViewer {
        jsonError(w, "invalid role; must be admin, analyst, or viewer", http.StatusBadRequest)
        return
    }

    // Only admin can create users — check caller
    caller := auth.GetUserFromContext(r)
    if caller != nil && caller.Role != auth.RoleAdmin {
        jsonError(w, "forbidden", http.StatusForbidden)
        return
    }

    var createdBy *interface{ String() string }
    _ = createdBy

    user, err := h.authService.UserSvc.CreateUser(r.Context(), req.Email, req.DisplayName, req.Password, req.Role, nil)
    if err != nil {
        jsonError(w, err.Error(), http.StatusBadRequest)
        return
    }

    if h.authService.AuditLog != nil {
        h.authService.AuditLog.LogUserCreated(r.Context(), user.ID, user.ID, user.Email)
    }

    w.Header().Set("Content-Type", "application/json")
    w.WriteHeader(http.StatusCreated)
    json.NewEncoder(w).Encode(map[string]interface{}{
        "id":           user.ID.String(),
        "email":        user.Email,
        "display_name": user.DisplayName,
        "role":         user.Role,
        "status":       user.Status,
    })
}
```

- [ ] **Step 5.4: Run tests**

```bash
go test ./internal/ingest/... -run TestHandleListUsers -run TestHandleCreateUser -v
```

Expected: PASS

- [ ] **Step 5.5: Commit**

```bash
git add internal/ingest/handler_users.go internal/ingest/handler_users_test.go \
        internal/ingest/handler_stubs.go
git commit -m "feat(ingest): live user management handlers (list, create)

Password hash never returned in responses. Role validated against
admin/analyst/viewer. Caller role-check gates create to admin only.
Both return 503 when auth service not wired.

Co-Authored-By: Claude Sonnet 4.6 <noreply@anthropic.com>"
```

---

### Task 6: Wire Auth Into api.go

**Files:**
- Modify: `cmd/argus/api.go`

Wire `PgUserStore`, `PgSessionStore`, `PgAuditStore`, `UserService`, `SessionManager`, `TokenManager`, `AuditLogger`, `AuthService` — and apply `AuthMiddleware` to protected routes.

- [ ] **Step 6.1: Update `cmd/argus/api.go`**

After the `pgPool` connection block (around line 108), add:

```go
// Auth subsystem (requires PostgreSQL)
var authSvc *ingest.AuthService
if pgPool != nil {
    privateKey, publicKey, keyErr := auth.LoadOrGenerateRSAKey()
    if keyErr != nil {
        log.Warn("failed to load/generate JWT key — auth disabled", zap.Error(keyErr))
    } else {
        userStore := auth.NewPgUserStore(pgPool)
        sessionStore := auth.NewPgSessionStore(pgPool)
        auditStore := auth.NewPgAuditStore(pgPool)
        auditLogger := auth.NewAuditLogger(auditStore)
        userSvc := auth.NewUserService(userStore)
        tokenMgr := auth.NewTokenManager(auth.TokenConfig{
            PrivateKey: privateKey,
            PublicKey:  publicKey,
        })
        sessionMgr := auth.NewSessionManager(sessionStore, 0)
        authSvc = &ingest.AuthService{
            UserSvc:      userSvc,
            UserStore:    userStore,
            SessionMgr:   sessionMgr,
            TokenMgr:     tokenMgr,
            AuditLog:     auditLogger,
            SessionStore: sessionStore,
        }
        queryHandler.SetAuthService(authSvc)
        log.Info("auth subsystem initialized")
    }
}
```

Add `AuthMiddleware` to the chi router, protecting all `/api/v1/` routes except auth and setup:

```go
// Auth middleware on protected routes
if authSvc != nil {
    r.Use(func(next http.Handler) http.Handler {
        return auth.AuthMiddleware(auth.MiddlewareConfig{
            TokenManager: authSvc.TokenMgr,
            SessionStore: authSvc.SessionStore,
            AuditLogger:  authSvc.AuditLog,
            Logger:       log,
            ExcludedPaths: map[string]bool{
                "/health":               true,
                "/metrics":              true,
                "/api/v1/auth/login":    true,
                "/api/v1/auth/refresh":  true,
                "/api/v1/auth/setup":    true,
                "/v1/signals":           true,
                "/v1/signals/stream":    true,
                "/v1/schema/signals":    true,
            },
        })(next)
    })
}
```

Add imports: `"github.com/argusxdr/argus/internal/auth"`.

- [ ] **Step 6.2: Build to verify**

```bash
go build ./cmd/... ./internal/... 2>&1 | grep -v "gen/go/google"
```

Expected: zero new errors (only pre-existing ones in unrelated packages if any).

- [ ] **Step 6.3: Run all tests**

```bash
go test ./internal/auth/... ./internal/ingest/... ./internal/notify/... 2>&1
```

Expected: all pass.

- [ ] **Step 6.4: Commit**

```bash
git add cmd/argus/api.go
git commit -m "feat(cmd): wire auth subsystem — JWT keys, stores, middleware, RBAC gates

- LoadOrGenerateRSAKey: uses ARGUS_JWT_PRIVATE_KEY_PEM or ephemeral 2048-bit key
- PgUserStore + PgSessionStore + PgAuditStore wired when pgPool available
- AuthMiddleware applied to all /api/v1/ routes except login/refresh/setup/metrics/health
- SetAuthService injects AuthService into QueryHandler
- Graceful degradation: auth disabled when PostgreSQL unavailable (auth routes return 503)

Co-Authored-By: Claude Sonnet 4.6 <noreply@anthropic.com>"
```

---

### Task 7: Add GET /api/v1/audit Handler

**Files:**
- Create: `internal/ingest/handler_audit.go`
- Remove stub: `handleListAuditLog` from `handler_stubs.go`

- [ ] **Step 7.1: Create `internal/ingest/handler_audit.go`**

```go
package ingest

import (
    "encoding/json"
    "net/http"
    "strconv"
)

func (h *QueryHandler) handleListAuditLog(w http.ResponseWriter, r *http.Request) {
    if !h.authAvailable() || h.authService.AuditLog == nil {
        jsonError(w, "audit log unavailable", http.StatusServiceUnavailable)
        return
    }

    limit := 50
    offset := 0
    if s := r.URL.Query().Get("limit"); s != "" {
        if n, err := strconv.Atoi(s); err == nil && n > 0 && n <= 1000 {
            limit = n
        }
    }
    if s := r.URL.Query().Get("offset"); s != "" {
        if n, err := strconv.Atoi(s); err == nil && n >= 0 {
            offset = n
        }
    }

    entries, err := h.authService.AuditLog.GetEntries(r.Context(), nil, limit, offset)
    if err != nil {
        jsonError(w, "internal error", http.StatusInternalServerError)
        return
    }
    if entries == nil {
        entries = make([]*interface{}{}, 0)[:]  // empty slice, not nil
    }

    w.Header().Set("Content-Type", "application/json")
    json.NewEncoder(w).Encode(map[string]interface{}{
        "entries": entries,
        "limit":   limit,
        "offset":  offset,
    })
}
```

Remove `handleListAuditLog` stub from `handler_stubs.go`.

- [ ] **Step 7.2: Build and test**

```bash
go build ./internal/ingest/...
go test ./internal/ingest/... 2>&1
```

Expected: all pass.

- [ ] **Step 7.3: Commit**

```bash
git add internal/ingest/handler_audit.go internal/ingest/handler_stubs.go
git commit -m "feat(ingest): live GET /api/v1/audit handler with pagination

Returns audit log entries from PostgreSQL with limit/offset pagination.
Returns 503 when auth service or audit store not wired.

Co-Authored-By: Claude Sonnet 4.6 <noreply@anthropic.com>"
```

---

### Task 8: Commit Plan Doc + Final Verification

- [ ] **Step 8.1: Full build and test run**

```bash
go build ./cmd/... ./internal/auth/... ./internal/ingest/... ./internal/notify/... ./internal/detection/...
go test ./internal/auth/... ./internal/ingest/... ./internal/notify/... ./internal/detection/... -count=1 2>&1
```

Expected: zero build errors in listed packages, all tests pass.

- [ ] **Step 8.2: Commit plan doc**

```bash
git add docs/superpowers/plans/2026-04-12-argus-xdr-v1-step4-auth.md
git commit -m "docs: add Step 4 auth system plan

Co-Authored-By: Claude Sonnet 4.6 <noreply@anthropic.com>"
```

---

## Implementation Notes

### Known Coupling Points (resolve during Step 4.5)

1. **`UserService.Store()`** — `UserService` in `users.go` stores `store UserStore` as a private field. Add an exported `Store() UserStore` getter, or directly inject `UserStore` into `AuthService` alongside `UserSvc`.

2. **`PgSessionStore.DB()`** — `SetupManager` needs `*pgxpool.Pool` to call `UserStore.ListUsers`. Since `PgUserStore` already holds it, pass `userStore` directly to `NewSetupManager` instead of the pool.

3. **`auth.HashTokenPublic`** — `hashToken` in `middleware.go` is unexported. Either export it as `HashToken`, or just reimplement in `handler_auth.go` (it's a SHA256 one-liner).

4. **`SessionManager.RotateRefreshToken`** returns `(newToken, newHash, error)` but the current implementation has a TODO for persistence. `handler_auth.go` handles persistence directly via `SessionStore.UpdateSessionHash` — the `RotateRefreshToken` method just generates the new token/hash pair.

### What This Step Does NOT Include

- MFA (TOTP) — wiring exists but requires `mfa_secret` setup flow; skip for now
- Password change endpoint — stub remains 501 until Step 5 frontend needs it
- App API key validation middleware — that's Step 6 (packaging)
- Token revocation cleanup cron job — left for Step 6
