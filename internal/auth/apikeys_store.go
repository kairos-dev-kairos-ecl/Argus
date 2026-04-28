package auth

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// PgAPIKeyStore implements ApiKeyStore against the api_keys table.
type PgAPIKeyStore struct {
	db *pgxpool.Pool
}

// NewPgAPIKeyStore creates a new PgAPIKeyStore.
func NewPgAPIKeyStore(db *pgxpool.Pool) *PgAPIKeyStore {
	return &PgAPIKeyStore{db: db}
}

func (s *PgAPIKeyStore) Create(ctx context.Context, k *APIKey) error {
	if s.db == nil {
		return fmt.Errorf("database unavailable")
	}
	_, err := s.db.Exec(ctx, `
        INSERT INTO api_keys (id, user_id, app_id, name, key_prefix, key_hash, scopes, created_at)
        VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
    `,
		k.ID, k.UserID, k.AppID, k.Name, k.KeyPrefix, k.KeyHash, k.Scopes, k.CreatedAt,
	)
	return err
}

func (s *PgAPIKeyStore) GetByHash(ctx context.Context, hash string) (*APIKey, error) {
	if s.db == nil {
		return nil, fmt.Errorf("database unavailable")
	}
	row := s.db.QueryRow(ctx, `
        SELECT id, user_id, app_id, name, key_prefix, key_hash, scopes, last_used_at, expires_at, revoked_at, created_at
        FROM api_keys WHERE key_hash = $1
    `, hash)
	return scanAPIKey(row)
}

func (s *PgAPIKeyStore) GetByID(ctx context.Context, id uuid.UUID) (*APIKey, error) {
	if s.db == nil {
		return nil, fmt.Errorf("database unavailable")
	}
	row := s.db.QueryRow(ctx, `
        SELECT id, user_id, app_id, name, key_prefix, key_hash, scopes, last_used_at, expires_at, revoked_at, created_at
        FROM api_keys WHERE id = $1
    `, id)
	return scanAPIKey(row)
}

func (s *PgAPIKeyStore) ListByUser(ctx context.Context, userID uuid.UUID) ([]*APIKey, error) {
	if s.db == nil {
		return nil, fmt.Errorf("database unavailable")
	}
	rows, err := s.db.Query(ctx, `
        SELECT id, user_id, app_id, name, key_prefix, key_hash, scopes, last_used_at, expires_at, revoked_at, created_at
        FROM api_keys WHERE user_id = $1 ORDER BY created_at DESC
    `, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var keys []*APIKey
	for rows.Next() {
		k, err := scanAPIKey(rows)
		if err != nil {
			return nil, err
		}
		keys = append(keys, k)
	}
	return keys, rows.Err()
}

func (s *PgAPIKeyStore) Revoke(ctx context.Context, id uuid.UUID) error {
	if s.db == nil {
		return fmt.Errorf("database unavailable")
	}
	_, err := s.db.Exec(ctx, `
        UPDATE api_keys SET revoked_at = now() WHERE id = $1
    `, id)
	return err
}

func (s *PgAPIKeyStore) TouchLastUsed(ctx context.Context, id uuid.UUID, t time.Time) error {
	if s.db == nil {
		return nil // non-fatal
	}
	_, err := s.db.Exec(ctx, `
        UPDATE api_keys SET last_used_at = $2 WHERE id = $1
    `, id, t)
	return err
}

// scanAPIKey scans a single API key row.
func scanAPIKey(row interface{ Scan(dest ...any) error }) (*APIKey, error) {
	k := &APIKey{}
	err := row.Scan(
		&k.ID, &k.UserID, &k.AppID, &k.Name, &k.KeyPrefix, &k.KeyHash,
		&k.Scopes, &k.LastUsedAt, &k.ExpiresAt, &k.RevokedAt, &k.CreatedAt,
	)
	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, nil // not found → nil, nil
		}
		return nil, err
	}
	return k, nil
}
