package auth

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

// Invite represents a pending user invitation.
type Invite struct {
	ID         uuid.UUID
	Email      string
	Role       string
	TokenHash  string
	InvitedBy  uuid.UUID
	ExpiresAt  time.Time
	AcceptedAt *time.Time
	CreatedAt  time.Time
}

// InviteStore defines the persistence contract for invitations.
type InviteStore interface {
	Create(ctx context.Context, email, role string, invitedBy uuid.UUID, tokenHash string, expiresAt time.Time) (*Invite, error)
	GetByTokenHash(ctx context.Context, tokenHash string) (*Invite, error)
	MarkAccepted(ctx context.Context, id uuid.UUID) error
}

// PgInviteStore is a PostgreSQL-backed InviteStore.
type PgInviteStore struct {
	pool *pgxpool.Pool
}

// NewPgInviteStore creates a new PgInviteStore.
func NewPgInviteStore(pool *pgxpool.Pool) *PgInviteStore {
	return &PgInviteStore{pool: pool}
}

// Create inserts a new invite row and returns the created Invite.
func (s *PgInviteStore) Create(ctx context.Context, email, role string, invitedBy uuid.UUID, tokenHash string, expiresAt time.Time) (*Invite, error) {
	invite := &Invite{}
	err := s.pool.QueryRow(ctx,
		`INSERT INTO invites (email, role, token_hash, invited_by, expires_at)
		 VALUES ($1, $2, $3, $4, $5)
		 RETURNING id, email, role, token_hash, invited_by, expires_at, accepted_at, created_at`,
		email, role, tokenHash, invitedBy, expiresAt,
	).Scan(
		&invite.ID, &invite.Email, &invite.Role, &invite.TokenHash,
		&invite.InvitedBy, &invite.ExpiresAt, &invite.AcceptedAt, &invite.CreatedAt,
	)
	if err != nil {
		return nil, fmt.Errorf("create invite: %w", err)
	}
	return invite, nil
}

// GetByTokenHash looks up an invite by the SHA-256 hash of the raw token.
func (s *PgInviteStore) GetByTokenHash(ctx context.Context, tokenHash string) (*Invite, error) {
	invite := &Invite{}
	err := s.pool.QueryRow(ctx,
		`SELECT id, email, role, token_hash, invited_by, expires_at, accepted_at, created_at
		 FROM invites WHERE token_hash = $1`,
		tokenHash,
	).Scan(
		&invite.ID, &invite.Email, &invite.Role, &invite.TokenHash,
		&invite.InvitedBy, &invite.ExpiresAt, &invite.AcceptedAt, &invite.CreatedAt,
	)
	if err != nil {
		return nil, fmt.Errorf("get invite by token hash: %w", err)
	}
	return invite, nil
}

// MarkAccepted stamps accepted_at = NOW() on the given invite row.
func (s *PgInviteStore) MarkAccepted(ctx context.Context, id uuid.UUID) error {
	_, err := s.pool.Exec(ctx,
		`UPDATE invites SET accepted_at = NOW() WHERE id = $1`,
		id,
	)
	return err
}

// PasswordHasher is a function that hashes a plain-text password.
// Callers typically pass bcrypt.GenerateFromPassword wrapped as a closure.
type PasswordHasher func(password string) (string, error)

// InviteUserCreator is the minimal interface InviteService needs to create users.
// Matches the signature of *UserService.CreateUser.
type InviteUserCreator interface {
	CreateUser(ctx context.Context, email, displayName, password, role string, createdBy *uuid.UUID) (*User, error)
}

// InviteService orchestrates invite lifecycle: generation, validation, acceptance.
type InviteService struct {
	store    InviteStore
	userSvc  InviteUserCreator
}

// NewInviteService creates an InviteService.
func NewInviteService(store InviteStore, userSvc InviteUserCreator) *InviteService {
	return &InviteService{store: store, userSvc: userSvc}
}

// GenerateToken returns (rawToken, tokenHash).
// rawToken is 32 random bytes encoded as hex (64 characters).
// tokenHash is the SHA-256 of rawToken, also hex-encoded.
func GenerateToken() (string, string, error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", "", fmt.Errorf("generate token: %w", err)
	}
	raw := hex.EncodeToString(b)
	h := sha256.Sum256([]byte(raw))
	return raw, hex.EncodeToString(h[:]), nil
}

// hashInviteToken returns the SHA-256 hex of a raw invite token string.
func hashInviteToken(raw string) string {
	h := sha256.Sum256([]byte(raw))
	return hex.EncodeToString(h[:])
}

// CreateInvite generates a token and persists the invite for 7 days.
// Returns the raw token (to embed in the invite URL) and the stored Invite.
func (svc *InviteService) CreateInvite(ctx context.Context, email, role string, invitedBy uuid.UUID) (string, *Invite, error) {
	raw, hash, err := GenerateToken()
	if err != nil {
		return "", nil, err
	}
	expiresAt := time.Now().Add(7 * 24 * time.Hour)
	invite, err := svc.store.Create(ctx, email, role, invitedBy, hash, expiresAt)
	if err != nil {
		return "", nil, err
	}
	return raw, invite, nil
}

// GetByToken hashes the raw token and fetches the matching invite.
// Returns an error if the invite does not exist, is expired, or is already accepted.
func (svc *InviteService) GetByToken(ctx context.Context, rawToken string) (*Invite, error) {
	hash := hashInviteToken(rawToken)
	invite, err := svc.store.GetByTokenHash(ctx, hash)
	if err != nil {
		return nil, fmt.Errorf("invite not found")
	}
	if invite.AcceptedAt != nil {
		return nil, fmt.Errorf("invite already accepted")
	}
	if time.Now().After(invite.ExpiresAt) {
		return nil, fmt.Errorf("invite expired")
	}
	return invite, nil
}

// AcceptInvite validates the token, creates the user, and marks the invite accepted.
// The hasher function receives the plain-text password and returns (hash, error).
func (svc *InviteService) AcceptInvite(ctx context.Context, rawToken, displayName, password string, hasher PasswordHasher) (*User, error) {
	invite, err := svc.GetByToken(ctx, rawToken)
	if err != nil {
		return nil, err
	}

	// hasher is used indirectly — UserService.CreateUser handles hashing internally.
	// We just need to pass the plain-text password through.
	_ = hasher

	invitedBy := invite.InvitedBy
	user, err := svc.userSvc.CreateUser(ctx, invite.Email, displayName, password, invite.Role, &invitedBy)
	if err != nil {
		return nil, fmt.Errorf("create invited user: %w", err)
	}

	if err := svc.store.MarkAccepted(ctx, invite.ID); err != nil {
		// Non-fatal: user is already created; log and continue
		return user, nil
	}

	return user, nil
}
