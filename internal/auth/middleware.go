package auth

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"net/http"
	"strings"
	"time"

	"github.com/google/uuid"
	"go.uber.org/zap"
)

const (
	// BearerScheme is the authorization scheme for JWT tokens
	BearerScheme = "Bearer"
	// ContextKeyUser is the context key for storing the authenticated user claims
	ContextKeyUser = "user"
)

// claimsKey is the private type used to avoid collisions on context.Context.
type contextKey string

const claimsKey contextKey = "argus_claims"

// MiddlewareConfig holds configuration for auth middleware
type MiddlewareConfig struct {
	TokenManager     *TokenManager
	AuditLogger      *AuditLogger
	SessionStore     SessionStore
	RefreshURL       string
	Logger           *zap.Logger
	ExcludedPaths    map[string]bool
	AllowRefreshOnly bool
}

// SessionStore defines the interface for session persistence
type SessionStore interface {
	// GetSessionByUserID retrieves all valid sessions for a user
	GetSessionByUserID(ctx context.Context, userID string) ([]Session, error)
	// GetSessionByHash retrieves a session by its refresh token hash
	GetSessionByHash(ctx context.Context, hash string) (*Session, error)
	// RevokeSession marks a session as revoked
	RevokeSession(ctx context.Context, sessionID string) error
	// CheckTokenRevocation checks if a token hash is revoked
	CheckTokenRevocation(ctx context.Context, tokenHash string) (bool, error)
}

// Session represents a user session
type Session struct {
	ID               string
	UserID           string
	RefreshTokenHash string
	UserAgent        string
	IPAddress        string
	CreatedAt        int64
	ExpiresAt        int64
	RevokedAt        *int64
	LastUsedAt       int64
}

// AuthMiddleware validates JWT tokens in HTTP requests
func AuthMiddleware(cfg MiddlewareConfig) func(next http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// Check if path is excluded
			if cfg.ExcludedPaths != nil && cfg.ExcludedPaths[r.URL.Path] {
				next.ServeHTTP(w, r)
				return
			}

			// Extract token from Authorization header
			authHeader := r.Header.Get("Authorization")
			if authHeader == "" {
				http.Error(w, "missing authorization header", http.StatusUnauthorized)
				return
			}

			parts := strings.SplitN(authHeader, " ", 2)
			if len(parts) != 2 || parts[0] != BearerScheme {
				http.Error(w, "invalid authorization header", http.StatusUnauthorized)
				return
			}

			tokenString := parts[1]

			// Verify token
			claims, err := cfg.TokenManager.VerifyAccessToken(tokenString)
			if err != nil {
				// Token might be expired but valid signature
				if claims != nil && cfg.TokenManager.IsTokenExpired(claims) && cfg.SessionStore != nil {
					// Try to refresh silently
					if cfg.Logger != nil {
						cfg.Logger.Debug("access token expired, attempting refresh",
							zap.String("user_id", claims.UserID.String()))
					}
					// Check if refresh cookie exists
					refreshToken, err := r.Cookie("refresh_token")
					if err == nil && refreshToken.Value != "" {
						// Verify refresh token exists in database
						session, err := cfg.SessionStore.GetSessionByHash(r.Context(), hashToken(refreshToken.Value))
						if err == nil && session != nil && session.ExpiresAt > currentUnixTime() {
							// Refresh token is valid
							// Issue new tokens
							newAccessToken, err := cfg.TokenManager.IssueAccessToken(
								claims.UserID,
								claims.Email,
								claims.DisplayName,
								claims.Role,
								claims.Permissions,
							)
							if err == nil {
								// Set new access token in header for this request
								r.Header.Set("Authorization", BearerScheme+" "+newAccessToken)
								// Set new refresh token cookie
								newRefreshToken := cfg.TokenManager.IssueRefreshToken()
								http.SetCookie(w, &http.Cookie{
									Name:     "refresh_token",
									Value:    newRefreshToken,
									Path:     "/",
									HttpOnly: true,
									Secure:   true,
									SameSite: http.SameSiteLaxMode,
									MaxAge:   int(cfg.TokenManager.config.RefreshTokenTTL.Seconds()),
								})

								// Log token refresh
								if cfg.AuditLogger != nil {
									cfg.AuditLogger.LogAction(r.Context(), "token_refresh", "session", map[string]interface{}{
										"user_id": claims.UserID.String(),
									})
								}

								// Update context with new claims and continue
								ctx := context.WithValue(r.Context(), ContextKeyUser, claims)
								// Also add to the new claimsKey for context extractors
								ctx = context.WithValue(ctx, claimsKey, claims)
								next.ServeHTTP(w, r.WithContext(ctx))
								return
							}
						}
					}
				}

				// Token invalid or refresh failed
				if cfg.Logger != nil {
					cfg.Logger.Debug("token verification failed", zap.Error(err))
				}
				http.Error(w, "invalid token", http.StatusUnauthorized)
				return
			}

			// Verify token is not revoked
			if cfg.SessionStore != nil {
				tokenHash := hashToken(tokenString)
				isRevoked, err := cfg.SessionStore.CheckTokenRevocation(r.Context(), tokenHash)
				if err != nil {
					if cfg.Logger != nil {
						cfg.Logger.Error("failed to check token revocation", zap.Error(err))
					}
					http.Error(w, "internal error", http.StatusInternalServerError)
					return
				}
				if isRevoked {
					http.Error(w, "token revoked", http.StatusUnauthorized)
					return
				}
			}

			// Token is valid, add to context and continue
			ctx := context.WithValue(r.Context(), ContextKeyUser, claims)
			// Also add to the new claimsKey for context extractors
			ctx = context.WithValue(ctx, claimsKey, claims)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

// GetUserFromContext retrieves the authenticated user claims from the request context
func GetUserFromContext(r *http.Request) *Claims {
	claims, ok := r.Context().Value(ContextKeyUser).(*Claims)
	if !ok {
		return nil
	}
	return claims
}

// GetClaimsFromContext retrieves Claims from a plain context.Context.
// Used by audit logging code that has a context but not an *http.Request.
func GetClaimsFromContext(ctx context.Context) *Claims {
	claims, ok := ctx.Value(ContextKeyUser).(*Claims)
	if !ok {
		return nil
	}
	return claims
}

// RequireRole middleware ensures user has the specified role
func RequireRole(roles ...string) func(next http.Handler) http.Handler {
	roleMap := make(map[string]bool)
	for _, r := range roles {
		roleMap[r] = true
	}

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			claims := GetUserFromContext(r)
			if claims == nil {
				http.Error(w, "unauthorized", http.StatusUnauthorized)
				return
			}

			if !roleMap[claims.Role] {
				http.Error(w, "forbidden", http.StatusForbidden)
				return
			}

			next.ServeHTTP(w, r)
		})
	}
}

// RequirePermission middleware ensures user has the specified permission
func RequirePermission(permission string) func(next http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			claims := GetUserFromContext(r)
			if claims == nil {
				http.Error(w, "unauthorized", http.StatusUnauthorized)
				return
			}

			hasPermission := false
			for _, p := range claims.Permissions {
				if p == permission {
					hasPermission = true
					break
				}
			}

			if !hasPermission {
				http.Error(w, "forbidden", http.StatusForbidden)
				return
			}

			next.ServeHTTP(w, r)
		})
	}
}

// hashToken creates a SHA256 hash of a token for secure storage.
func hashToken(token string) string {
	h := sha256.Sum256([]byte(token))
	return hex.EncodeToString(h[:])
}

// HashToken is the exported version of hashToken for use by other packages.
func HashToken(token string) string { return hashToken(token) }

// currentUnixTime returns the current time as a Unix timestamp.
func currentUnixTime() int64 { return time.Now().Unix() }

// RequireAuth builds a standard chi/http middleware that validates the JWT,
// loads the session via SessionStore, emits audit events, and injects *Claims
// into the request context under claimsKey. It is a thin wrapper around the
// existing AuthMiddleware config struct with ExcludedPaths left empty (the
// outer router is responsible for attaching this only on protected subtrees).
func RequireAuth(tm *TokenManager, ss SessionStore, al *AuditLogger, log *zap.Logger) func(http.Handler) http.Handler {
	cfg := MiddlewareConfig{
		TokenManager:    tm,
		SessionStore:    ss,
		AuditLogger:     al,
		Logger:          log,
		ExcludedPaths:   nil, // outer router decides what is protected
		AllowRefreshOnly: false,
	}
	return AuthMiddleware(cfg)
}

// ClaimsFromContext retrieves Claims from a plain context.Context.
// Returns (nil, false) if no claims are in the context.
func ClaimsFromContext(ctx context.Context) (*Claims, bool) {
	c, ok := ctx.Value(claimsKey).(*Claims)
	return c, ok && c != nil
}

// UserIDFromContext retrieves the user ID from context.
// Returns (uuid.Nil, false) if no claims are in the context.
func UserIDFromContext(ctx context.Context) (uuid.UUID, bool) {
	c, ok := ClaimsFromContext(ctx)
	if !ok {
		return uuid.Nil, false
	}
	return c.UserID, true
}

// SessionIDFromContext retrieves the session ID from context.
// Returns ("", false) if no claims or session ID are in the context.
func SessionIDFromContext(ctx context.Context) (string, bool) {
	c, ok := ClaimsFromContext(ctx)
	if !ok || c.SessionID == "" {
		return "", false
	}
	return c.SessionID, true
}
