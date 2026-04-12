package auth

import (
	"crypto/rsa"
	"fmt"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
)

// Claims represents JWT claims for Argus access tokens
type Claims struct {
	UserID      uuid.UUID   `json:"sub"`
	Email       string      `json:"email"`
	DisplayName string      `json:"name"`
	Role        string      `json:"role"`
	Permissions []string    `json:"permissions"`
	TokenID     string      `json:"jti"`
	jwt.RegisteredClaims
}

// TokenConfig holds JWT signing/verification configuration
type TokenConfig struct {
	PrivateKey       *rsa.PrivateKey
	PublicKey        *rsa.PublicKey
	AccessTokenTTL   time.Duration
	RefreshTokenTTL  time.Duration
	Issuer           string
	Audience         []string
}

// TokenManager handles JWT signing, verification, and refresh logic
type TokenManager struct {
	config TokenConfig
}

// NewTokenManager creates a new token manager
func NewTokenManager(cfg TokenConfig) *TokenManager {
	if cfg.AccessTokenTTL == 0 {
		cfg.AccessTokenTTL = 15 * time.Minute
	}
	if cfg.RefreshTokenTTL == 0 {
		cfg.RefreshTokenTTL = 7 * 24 * time.Hour
	}
	if cfg.Issuer == "" {
		cfg.Issuer = "argus-xdr"
	}
	return &TokenManager{config: cfg}
}

// IssueAccessToken creates a signed JWT access token
func (tm *TokenManager) IssueAccessToken(userID uuid.UUID, email, displayName, role string, permissions []string) (string, error) {
	now := time.Now()
	expiresAt := now.Add(tm.config.AccessTokenTTL)

	claims := Claims{
		UserID:      userID,
		Email:       email,
		DisplayName: displayName,
		Role:        role,
		Permissions: permissions,
		TokenID:     uuid.New().String(),
		RegisteredClaims: jwt.RegisteredClaims{
			Issuer:    tm.config.Issuer,
			Audience:  tm.config.Audience,
			Subject:   userID.String(),
			IssuedAt:  jwt.NewNumericDate(now),
			ExpiresAt: jwt.NewNumericDate(expiresAt),
			NotBefore: jwt.NewNumericDate(now),
		},
	}

	token := jwt.NewWithClaims(jwt.SigningMethodRS256, claims)
	return token.SignedString(tm.config.PrivateKey)
}

// IssueRefreshToken creates a refresh token (just a random string, content not important)
// The hash is stored in the database; the token string is returned
func (tm *TokenManager) IssueRefreshToken() string {
	return uuid.New().String()
}

// VerifyAccessToken verifies and parses a JWT access token
func (tm *TokenManager) VerifyAccessToken(tokenString string) (*Claims, error) {
	claims := &Claims{}
	token, err := jwt.ParseWithClaims(tokenString, claims, func(token *jwt.Token) (interface{}, error) {
		if _, ok := token.Method.(*jwt.SigningMethodRSA); !ok {
			return nil, fmt.Errorf("unexpected signing method: %v", token.Header["alg"])
		}
		return tm.config.PublicKey, nil
	})

	if err != nil {
		return nil, fmt.Errorf("token parsing failed: %w", err)
	}

	if !token.Valid {
		return nil, fmt.Errorf("invalid token")
	}

	// Verify claims
	if err := claims.Valid(); err != nil {
		return nil, fmt.Errorf("claims validation failed: %w", err)
	}

	return claims, nil
}

// VerifyTokenSignature verifies only the signature without checking expiry
// Used for refresh token validation (we allow expired tokens if refresh is valid)
func (tm *TokenManager) VerifyTokenSignature(tokenString string) (*Claims, error) {
	claims := &Claims{}
	token, err := jwt.ParseWithClaims(tokenString, claims, func(token *jwt.Token) (interface{}, error) {
		if _, ok := token.Method.(*jwt.SigningMethodRSA); !ok {
			return nil, fmt.Errorf("unexpected signing method: %v", token.Header["alg"])
		}
		return tm.config.PublicKey, nil
	}, jwt.WithoutClaimsValidation())

	if err != nil {
		return nil, fmt.Errorf("token parsing failed: %w", err)
	}

	if !token.Valid {
		return nil, fmt.Errorf("invalid token signature")
	}

	return claims, nil
}

// IsTokenExpired checks if a token's expiry claim has passed
func (tm *TokenManager) IsTokenExpired(claims *Claims) bool {
	if claims.ExpiresAt == nil {
		return false
	}
	return claims.ExpiresAt.Before(time.Now())
}

// GetTokenTTL calculates remaining TTL for a token
func (tm *TokenManager) GetTokenTTL(claims *Claims) time.Duration {
	if claims.ExpiresAt == nil {
		return 0
	}
	ttl := claims.ExpiresAt.Time.Sub(time.Now())
	if ttl < 0 {
		return 0
	}
	return ttl
}
