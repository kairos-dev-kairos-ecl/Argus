package auth

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"github.com/google/uuid"
	"golang.org/x/crypto/bcrypt"
)

// User represents a user in the system
type User struct {
	ID                uuid.UUID
	Email             string
	DisplayName       string
	Role              string
	Status            string
	PasswordHash      string
	PasswordChangedAt *time.Time
	LastLoginAt       *time.Time
	LastLoginIP       *string
	FailedLoginCount  int
	LockedUntil       *time.Time
	CreatedBy         *uuid.UUID
	InvitedAt         *time.Time
	MFASecret         *string
	ExternalProvider  *string
	ExternalID        *string
	CreatedAt         time.Time
	UpdatedAt         time.Time
}

// UserStore defines the interface for user persistence
type UserStore interface {
	// CreateUser creates a new user
	CreateUser(ctx context.Context, user *User) error
	// GetUserByID retrieves a user by ID
	GetUserByID(ctx context.Context, id uuid.UUID) (*User, error)
	// GetUserByEmail retrieves a user by email
	GetUserByEmail(ctx context.Context, email string) (*User, error)
	// ListUsers retrieves all users with optional filtering
	ListUsers(ctx context.Context, filters map[string]interface{}) ([]*User, error)
	// UpdateUser updates an existing user
	UpdateUser(ctx context.Context, user *User) error
	// DeleteUser marks a user as deleted (soft delete)
	DeleteUser(ctx context.Context, id uuid.UUID) error
	// UpdateLoginAttempt records a login attempt
	UpdateLoginAttempt(ctx context.Context, userID uuid.UUID, success bool, ip string) error
	// UnlockUser removes account lockout
	UnlockUser(ctx context.Context, userID uuid.UUID) error
}

// UserService handles user business logic
type UserService struct {
	store          UserStore
	logger         interface{} // zap.Logger
	bcryptCost     int
	maxFailedLogins int
	lockoutDuration time.Duration
}

// NewUserService creates a new user service
func NewUserService(store UserStore) *UserService {
	return &UserService{
		store:           store,
		bcryptCost:      12,
		maxFailedLogins: 5,
		lockoutDuration: 15 * time.Minute,
	}
}

// CreateUser creates a new user with hashed password
func (us *UserService) CreateUser(ctx context.Context, email, displayName, password, role string, createdByID *uuid.UUID) (*User, error) {
	// Validate inputs
	if email == "" || password == "" {
		return nil, fmt.Errorf("email and password required")
	}

	// Hash password with bcrypt
	hash, err := bcrypt.GenerateFromPassword([]byte(password), us.bcryptCost)
	if err != nil {
		return nil, fmt.Errorf("failed to hash password: %w", err)
	}

	now := time.Now()
	user := &User{
		ID:           uuid.New(),
		Email:        email,
		DisplayName:  displayName,
		PasswordHash: string(hash),
		Role:         role,
		Status:       "active",
		CreatedBy:    createdByID,
		CreatedAt:    now,
		UpdatedAt:    now,
	}

	// Check if user already exists
	existing, err := us.store.GetUserByEmail(ctx, email)
	if err != nil && err != sql.ErrNoRows {
		return nil, fmt.Errorf("failed to check existing user: %w", err)
	}
	if existing != nil {
		return nil, fmt.Errorf("user with email %s already exists", email)
	}

	if err := us.store.CreateUser(ctx, user); err != nil {
		return nil, fmt.Errorf("failed to create user: %w", err)
	}

	return user, nil
}

// VerifyPassword checks if the provided password matches the user's hash
func (us *UserService) VerifyPassword(user *User, password string) error {
	return bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(password))
}

// ResetPassword hashes and updates a user's password
func (us *UserService) ResetPassword(ctx context.Context, userID uuid.UUID, newPassword string) error {
	// Hash new password
	hash, err := bcrypt.GenerateFromPassword([]byte(newPassword), us.bcryptCost)
	if err != nil {
		return fmt.Errorf("failed to hash password: %w", err)
	}

	user, err := us.store.GetUserByID(ctx, userID)
	if err != nil {
		return fmt.Errorf("user not found: %w", err)
	}

	now := time.Now()
	user.PasswordHash = string(hash)
	user.PasswordChangedAt = &now
	user.FailedLoginCount = 0 // Reset failed login count
	user.UpdatedAt = now

	if err := us.store.UpdateUser(ctx, user); err != nil {
		return fmt.Errorf("failed to update user: %w", err)
	}

	return nil
}

// AuthenticateUser verifies credentials and handles account lockout
func (us *UserService) AuthenticateUser(ctx context.Context, email, password, ipAddress string) (*User, error) {
	// Get user by email
	user, err := us.store.GetUserByEmail(ctx, email)
	if err != nil {
		if err == sql.ErrNoRows {
			// Record failed attempt
			return nil, fmt.Errorf("invalid credentials")
		}
		return nil, fmt.Errorf("authentication error: %w", err)
	}

	// Check if user is locked
	if user.LockedUntil != nil && user.LockedUntil.After(time.Now()) {
		return nil, fmt.Errorf("account locked until %v", user.LockedUntil)
	}

	// Check account status
	if user.Status != "active" {
		return nil, fmt.Errorf("account is %s", user.Status)
	}

	// Verify password
	if err := us.VerifyPassword(user, password); err != nil {
		// Record failed login attempt
		us.store.UpdateLoginAttempt(ctx, user.ID, false, ipAddress)

		// Increment failed login count
		user.FailedLoginCount++
		if user.FailedLoginCount >= us.maxFailedLogins {
			// Lock account
			lockUntil := time.Now().Add(us.lockoutDuration)
			user.LockedUntil = &lockUntil
		}
		us.store.UpdateUser(ctx, user)

		return nil, fmt.Errorf("invalid credentials")
	}

	// Successful login - reset failed count and update last login
	now := time.Now()
	user.FailedLoginCount = 0
	user.LastLoginAt = &now
	user.LastLoginIP = &ipAddress
	user.UpdatedAt = now

	if err := us.store.UpdateUser(ctx, user); err != nil {
		return nil, fmt.Errorf("failed to update last login: %w", err)
	}

	// Record successful login
	us.store.UpdateLoginAttempt(ctx, user.ID, true, ipAddress)

	return user, nil
}

// SuspendUser marks a user as suspended
func (us *UserService) SuspendUser(ctx context.Context, userID uuid.UUID) error {
	user, err := us.store.GetUserByID(ctx, userID)
	if err != nil {
		return fmt.Errorf("user not found: %w", err)
	}

	user.Status = "suspended"
	user.UpdatedAt = time.Now()

	if err := us.store.UpdateUser(ctx, user); err != nil {
		return fmt.Errorf("failed to suspend user: %w", err)
	}

	return nil
}

// UpdateUserRole updates a user's role
func (us *UserService) UpdateUserRole(ctx context.Context, userID uuid.UUID, newRole string) error {
	user, err := us.store.GetUserByID(ctx, userID)
	if err != nil {
		return fmt.Errorf("user not found: %w", err)
	}

	user.Role = newRole
	user.UpdatedAt = time.Now()

	if err := us.store.UpdateUser(ctx, user); err != nil {
		return fmt.Errorf("failed to update user: %w", err)
	}

	return nil
}
