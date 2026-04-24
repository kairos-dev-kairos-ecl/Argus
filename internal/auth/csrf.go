package auth

import (
	"crypto/rand"
	"crypto/subtle"
	"encoding/base64"
	"encoding/json"
	"net/http"
	"strings"
)

// GenerateCSRFToken generates a new CSRF token using 32 bytes of cryptographically
// secure random data, encoded as base64 URL-safe string.
func GenerateCSRFToken() (string, error) {
	token := make([]byte, 32)
	_, err := rand.Read(token)
	if err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(token), nil
}

// CSRFMiddleware returns a middleware that enforces double-submit-cookie CSRF protection.
//
// For safe methods (GET, HEAD, OPTIONS):
//   - If csrf_token cookie is missing, generate a new token and set it
//   - Set X-CSRF-Token response header with the token value
//
// For unsafe methods (POST, PUT, DELETE, PATCH):
//   - Read csrf_token cookie and X-CSRF-Token header
//   - If either is missing or they don't match (constant-time compare), return 403
//   - Otherwise, call next handler
//
// Excluded paths are checked against the request URL path and bypass CSRF checks.
func CSRFMiddleware(excludedPaths []string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// Check if path is excluded from CSRF protection
			for _, excludedPath := range excludedPaths {
				if strings.HasPrefix(r.URL.Path, excludedPath) {
					next.ServeHTTP(w, r)
					return
				}
			}

			// Safe methods (GET, HEAD, OPTIONS) — generate token if needed
			switch r.Method {
			case http.MethodGet, http.MethodHead, http.MethodOptions:
				// Try to get existing token
				var tokenValue string
				cookie, err := r.Cookie("csrf_token")
				if err != nil || cookie.Value == "" {
					// Generate new token
					token, err := GenerateCSRFToken()
					if err != nil {
						http.Error(w, "csrf token generation failed", http.StatusInternalServerError)
						return
					}
					tokenValue = token

					// Set cookie
					http.SetCookie(w, &http.Cookie{
						Name:     "csrf_token",
						Value:    tokenValue,
						Path:     "/api/v1/auth",
						HttpOnly: false, // Must be false so JavaScript can read it
						Secure:   r.TLS != nil,
						SameSite: http.SameSiteStrictMode,
						MaxAge:   12 * 3600, // 12 hours
					})
				} else {
					// Reuse existing token
					tokenValue = cookie.Value
				}

				// Set response header
				w.Header().Set("X-CSRF-Token", tokenValue)
				next.ServeHTTP(w, r)
				return

			// Unsafe methods (POST, PUT, DELETE, PATCH) — validate token
			case http.MethodPost, http.MethodPut, http.MethodDelete, "PATCH":
				// Read csrf_token cookie
				cookie, err := r.Cookie("csrf_token")
				if err != nil || cookie.Value == "" {
					respondCSRFError(w, "missing csrf_token cookie")
					return
				}
				cookieToken := cookie.Value

				// Read X-CSRF-Token header
				headerToken := r.Header.Get("X-CSRF-Token")
				if headerToken == "" {
					respondCSRFError(w, "missing X-CSRF-Token header")
					return
				}

				// Constant-time comparison
				if subtle.ConstantTimeCompare([]byte(cookieToken), []byte(headerToken)) != 1 {
					respondCSRFError(w, "csrf token mismatch")
					return
				}

				next.ServeHTTP(w, r)
				return

			// Other methods pass through
			default:
				next.ServeHTTP(w, r)
			}
		})
	}
}

// respondCSRFError writes a JSON error response for CSRF validation failures
func respondCSRFError(w http.ResponseWriter, message string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusForbidden)
	json.NewEncoder(w).Encode(map[string]string{
		"error": message,
	})
}
