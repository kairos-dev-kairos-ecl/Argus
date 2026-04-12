package ingest

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/argusxdr/argus/internal/storage"
	"go.uber.org/zap"
	"golang.org/x/crypto/bcrypt"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
)

// contextKey is used to inject values into request contexts without clashing with built-ins.
type contextKey string

const appIDKey contextKey = "appID"

// cachedKey holds the cached API key validation result.
type cachedKey struct {
	appID     string
	expiresAt time.Time
	revokedAt *time.Time // nil = not revoked
}

// AuthValidator validates API keys against PostgreSQL and caches results.
type AuthValidator struct {
	db         *storage.Postgres
	cache      *sync.Map // key: string → cachedKey
	log        *zap.Logger
	ttl        time.Duration // cache TTL, default 5 minutes
	testMode   bool          // if true, use test-only validation
	testAppIDs map[string]string // map: key → appID for test mode
}

// NewAuthValidator creates an AuthValidator with a Postgres-backed cache.
func NewAuthValidator(db *storage.Postgres, log *zap.Logger) *AuthValidator {
	if log == nil {
		log = zap.NewNop()
	}
	return &AuthValidator{
		db:         db,
		cache:      &sync.Map{},
		log:        log,
		ttl:        5 * time.Minute,
		testMode:   false,
		testAppIDs: make(map[string]string),
	}
}

// NewTestAuthValidator creates an AuthValidator in test mode (no database required).
func NewTestAuthValidator(log *zap.Logger, testAppIDs map[string]string) *AuthValidator {
	if log == nil {
		log = zap.NewNop()
	}
	return &AuthValidator{
		cache:      &sync.Map{},
		log:        log,
		ttl:        5 * time.Minute,
		testMode:   true,
		testAppIDs: testAppIDs,
	}
}

// ValidateKey checks the raw API key string (Bearer token).
// Returns (appID, nil) on success, ("", error) on failure.
// Lookup order: 1) in-memory cache (check revoked_at), 2) PostgreSQL api_keys table.
// Never stores the raw key; compares bcrypt hash via bcrypt.CompareHashAndPassword.
// Returns error if key is revoked (revoked_at IS NOT NULL) or expired (expires_at < NOW()).
func (a *AuthValidator) ValidateKey(ctx context.Context, rawKey string) (string, error) {
	if rawKey == "" {
		return "", errors.New("api key cannot be empty")
	}

	// Check cache first
	if cached, ok := a.cache.Load(rawKey); ok {
		c := cached.(cachedKey)
		// Check if expired
		if c.expiresAt.Before(time.Now()) {
			a.cache.Delete(rawKey)
			return "", errors.New("api key expired")
		}
		// Check if revoked
		if c.revokedAt != nil && c.revokedAt.Before(time.Now()) {
			a.cache.Delete(rawKey)
			return "", errors.New("api key revoked")
		}
		// Still valid
		return c.appID, nil
	}

	// Cache miss — query PostgreSQL (or test mode)
	if a.testMode {
		// Test mode: direct lookup in test map
		if appID, ok := a.testAppIDs[rawKey]; ok {
			// Cache the result
			expireTime := time.Now().Add(a.ttl)
			a.cache.Store(rawKey, cachedKey{
				appID:     appID,
				expiresAt: expireTime,
				revokedAt: nil,
			})
			return appID, nil
		}
		return "", errors.New("invalid api key")
	}

	a.log.Debug("cache miss, validating key against postgresql")

	// NOTE: In production, the database schema should include a separate
	// fast-lookup hash (e.g., SHA256 prefix or full) that enables efficient
	// key lookups. The bcrypt hash is then verified on the returned row.
	// For this MVP, we use a simple pattern that trades some efficiency for security:
	// store key_hash as bcrypt, then scan and compare. In Phase 3, optimize with
	// key_lookup_hash alongside key_hash.

	var appID string
	var expiresAt *time.Time
	var revokedAt *time.Time

	// Query all active keys and compare hashes (expensive, but secure)
	// TODO: Optimize in Phase 3 with key_lookup_hash column
	rows, err := a.db.Pool.Query(ctx, `
		SELECT app_id, key_hash, expires_at, revoked_at
		FROM api_keys
		WHERE revoked_at IS NULL
		ORDER BY created_at DESC
		LIMIT 100
	`)

	if err != nil {
		return "", fmt.Errorf("database error: %w", err)
	}
	defer rows.Close()

	found := false
	for rows.Next() {
		var hash string
		var checkAppID string
		var checkExpiresAt *time.Time
		var checkRevokedAt *time.Time

		if err := rows.Scan(&checkAppID, &hash, &checkExpiresAt, &checkRevokedAt); err != nil {
			a.log.Debug("row scan error", zap.Error(err))
			continue
		}

		// Compare bcrypt hash
		if err := bcrypt.CompareHashAndPassword([]byte(hash), []byte(rawKey)); err == nil {
			// Found matching key
			appID = checkAppID
			expiresAt = checkExpiresAt
			revokedAt = checkRevokedAt
			found = true
			break
		}
	}

	if err := rows.Err(); err != nil {
		return "", fmt.Errorf("database error: %w", err)
	}

	if !found {
		return "", errors.New("invalid api key")
	}

	// Check if expired
	now := time.Now()
	if expiresAt != nil && expiresAt.Before(now) {
		return "", errors.New("api key expired")
	}

	// Check if revoked
	if revokedAt != nil && revokedAt.Before(now) {
		return "", errors.New("api key revoked")
	}

	// Cache the result
	expireTime := time.Now().Add(a.ttl)
	a.cache.Store(rawKey, cachedKey{
		appID:     appID,
		expiresAt: expireTime,
		revokedAt: revokedAt,
	})

	return appID, nil
}

// extractBearerToken extracts the bearer token from an Authorization header.
// Expected format: "Bearer {key}"
func extractBearerToken(authHeader string) string {
	parts := strings.SplitN(authHeader, " ", 2)
	if len(parts) != 2 || parts[0] != "Bearer" {
		return ""
	}
	return parts[1]
}

// GRPCStreamInterceptor returns a grpc.StreamServerInterceptor that validates API keys.
// Extracts key from grpc metadata Authorization header, value "Bearer {key}".
// Returns codes.Unauthenticated if missing or invalid.
// Injects appID into the stream context.
func (a *AuthValidator) GRPCStreamInterceptor() grpc.StreamServerInterceptor {
	return func(
		srv interface{},
		ss grpc.ServerStream,
		info *grpc.StreamServerInfo,
		handler grpc.StreamHandler,
	) error {
		// Extract metadata from stream context
		md, ok := metadata.FromIncomingContext(ss.Context())
		if !ok {
			return status.Error(codes.Unauthenticated, "missing metadata")
		}

		// Get Authorization header
		authHeaders := md.Get("authorization")
		if len(authHeaders) == 0 {
			return status.Error(codes.Unauthenticated, "missing authorization header")
		}

		token := extractBearerToken(authHeaders[0])
		if token == "" {
			return status.Error(codes.Unauthenticated, "invalid authorization header format")
		}

		// Validate the key
		appID, err := a.ValidateKey(ss.Context(), token)
		if err != nil {
			a.log.Warn("auth validation failed", zap.Error(err))
			return status.Error(codes.Unauthenticated, err.Error())
		}

		// Inject appID into context
		ctx := context.WithValue(ss.Context(), appIDKey, appID)
		wrappedStream := &wrappedServerStream{ServerStream: ss, ctx: ctx}

		// Call handler with injected context
		return handler(srv, wrappedStream)
	}
}

// wrappedServerStream wraps grpc.ServerStream to override Context().
type wrappedServerStream struct {
	grpc.ServerStream
	ctx context.Context
}

func (w *wrappedServerStream) Context() context.Context {
	return w.ctx
}

// HTTPMiddleware returns a chi-compatible middleware that validates API keys.
// Extracts key from Authorization header: "Bearer {key}".
// Returns 401 JSON {"error": "unauthenticated"} if invalid.
// Injects appID into request context.
func (a *AuthValidator) HTTPMiddleware() func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			authHeader := r.Header.Get("Authorization")
			if authHeader == "" {
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusUnauthorized)
				fmt.Fprintf(w, `{"error":"unauthenticated"}`)
				return
			}

			token := extractBearerToken(authHeader)
			if token == "" {
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusUnauthorized)
				fmt.Fprintf(w, `{"error":"unauthenticated"}`)
				return
			}

			appID, err := a.ValidateKey(r.Context(), token)
			if err != nil {
				a.log.Warn("auth validation failed", zap.Error(err))
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusUnauthorized)
				fmt.Fprintf(w, `{"error":"unauthenticated"}`)
				return
			}

			// Inject appID into context
			ctx := context.WithValue(r.Context(), appIDKey, appID)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

// AppIDFromContext retrieves the appID from a context (injected by auth interceptor/middleware).
func AppIDFromContext(ctx context.Context) (string, bool) {
	appID, ok := ctx.Value(appIDKey).(string)
	return appID, ok
}
