package auth

import (
	"context"
	"encoding/json"
	"net/http"
)

// APIKeyContextKey is the context key for storing the authenticated API key
const APIKeyContextKey contextKey = "argus_api_key"

// APIKeyMiddleware creates a chi/http middleware that validates X-Argus-API-Key headers.
// It calls the package-level ValidateAPIKey function to check the key's validity,
// revocation status, and expiration. If requiredScope is non-empty, it also checks that
// the key has the required scope. On success, the key is injected into the request context.
func APIKeyMiddleware(store ApiKeyStore, requiredScope string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// Read X-Argus-API-Key header
			rawKey := r.Header.Get("X-Argus-API-Key")
			if rawKey == "" {
				respondWithError(w, http.StatusUnauthorized, "missing api key")
				return
			}

			// Validate the key using the package-level function
			// This function hashes the key, checks the store, verifies revocation/expiration,
			// and touches last_used_at (best-effort).
			key, err := ValidateAPIKey(r.Context(), store, rawKey)
			if err != nil {
				if err == ErrAPIKeyNotFound || err == ErrAPIKeyRevoked || err == ErrAPIKeyExpired {
					respondWithError(w, http.StatusUnauthorized, "invalid api key")
					return
				}
				// Unexpected error (e.g., store failure)
				respondWithError(w, http.StatusInternalServerError, "authentication error")
				return
			}

			// Check scope if required
			if requiredScope != "" {
				hasScope := false
				for _, scope := range key.Scopes {
					if scope == requiredScope {
						hasScope = true
						break
					}
				}
				if !hasScope {
					respondWithError(w, http.StatusForbidden, "insufficient scope")
					return
				}
			}

			// Inject key into context and continue
			ctx := context.WithValue(r.Context(), APIKeyContextKey, key)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

// APIKeyFromContext retrieves the authenticated API key from a request context.
// Returns (nil, false) if no key is in the context.
func APIKeyFromContext(ctx context.Context) (*APIKey, bool) {
	key, ok := ctx.Value(APIKeyContextKey).(*APIKey)
	return key, ok && key != nil
}

// respondWithError writes a JSON error response
func respondWithError(w http.ResponseWriter, statusCode int, message string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(statusCode)
	errBody := map[string]string{"error": message}
	json.NewEncoder(w).Encode(errBody)
}
