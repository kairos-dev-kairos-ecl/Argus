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

	// Convert empty IP address to NULL for PostgreSQL INET type
	var ipAddress interface{} = sess.IPAddress
	if sess.IPAddress == "" {
		ipAddress = nil
	}

	// Convert empty user agent to NULL
	var userAgent interface{} = sess.UserAgent
	if sess.UserAgent == "" {
		userAgent = nil
	}

	_, err := s.db.Exec(ctx, `
        INSERT INTO sessions (id, user_id, refresh_token_hash, user_agent, ip_address,
            created_at, expires_at, last_used_at)
        VALUES ($1,$2,$3,$4,$5,
            to_timestamp($6), to_timestamp($7), to_timestamp($8))
    `,
		sess.ID, sess.UserID, sess.RefreshTokenHash, userAgent, ipAddress,
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

	// Convert empty IP address to NULL for PostgreSQL INET type
	var ipAddress interface{} = entry.IPAddress
	if entry.IPAddress == "" {
		ipAddress = nil
	}

	// Convert empty user agent to NULL
	var userAgent interface{} = entry.UserAgent
	if entry.UserAgent == "" {
		userAgent = nil
	}

	_, err = s.db.Exec(ctx, `
        INSERT INTO audit_log (id, user_id, action, resource, detail, ip_address, user_agent, timestamp)
        VALUES ($1,$2,$3,$4,$5,$6,$7,$8)
    `,
		entry.ID, entry.UserID, entry.Action, entry.Resource,
		detailJSON, ipAddress, userAgent, entry.Timestamp,
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
