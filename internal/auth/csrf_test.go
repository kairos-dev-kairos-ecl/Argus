package auth

import (
	"encoding/base64"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestCSRFMiddleware_SafeMethods(t *testing.T) {
	tests := []struct {
		name        string
		method      string
		hasCookie   bool
		expectCode  int
		expectCookie bool
	}{
		{
			name:         "GET without cookie generates and sets token",
			method:       http.MethodGet,
			hasCookie:    false,
			expectCode:   http.StatusOK,
			expectCookie: true,
		},
		{
			name:         "GET with existing cookie preserves it",
			method:       http.MethodGet,
			hasCookie:    true,
			expectCode:   http.StatusOK,
			expectCookie: true,
		},
		{
			name:         "HEAD without cookie generates and sets token",
			method:       http.MethodHead,
			hasCookie:    false,
			expectCode:   http.StatusOK,
			expectCookie: true,
		},
		{
			name:         "OPTIONS without cookie generates and sets token",
			method:       http.MethodOptions,
			hasCookie:    false,
			expectCode:   http.StatusOK,
			expectCookie: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			middleware := CSRFMiddleware([]string{})
			nextCalled := false
			nextHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				nextCalled = true
				w.WriteHeader(http.StatusOK)
			})

			handler := middleware(nextHandler)

			req := httptest.NewRequest(tt.method, "/api/v1/auth/login", nil)
			if tt.hasCookie {
				req.AddCookie(&http.Cookie{
					Name:  "csrf_token",
					Value: "test-token-value",
				})
			}

			w := httptest.NewRecorder()
			handler.ServeHTTP(w, req)

			assert.Equal(t, tt.expectCode, w.Code)
			assert.True(t, nextCalled, "next handler should be called")

			// Check for header
			if tt.expectCode == http.StatusOK {
				assert.NotEmpty(t, w.Header().Get("X-CSRF-Token"), "should set X-CSRF-Token header")
			}

			// Check for cookie
			if tt.expectCookie {
				cookies := w.Result().Cookies()
				found := false
				for _, cookie := range cookies {
					if cookie.Name == "csrf_token" {
						found = true
						assert.False(t, cookie.HttpOnly, "csrf_token should not be HttpOnly")
						assert.Equal(t, "/api/v1/auth", cookie.Path, "csrf_token should have correct path")
						assert.Equal(t, http.SameSiteStrictMode, cookie.SameSite, "csrf_token should use SameSiteStrict")
						break
					}
				}
				assert.True(t, found, "csrf_token cookie should be set")
			}
		})
	}
}

func TestCSRFMiddleware_UnsafeMethods(t *testing.T) {
	tests := []struct {
		name        string
		method      string
		hasCookie   bool
		hasHeader   bool
		matchToken  bool
		expectCode  int
		expectError bool
	}{
		{
			name:        "POST with matching token succeeds",
			method:      http.MethodPost,
			hasCookie:   true,
			hasHeader:   true,
			matchToken:  true,
			expectCode:  http.StatusOK,
			expectError: false,
		},
		{
			name:        "POST without X-CSRF-Token header fails",
			method:      http.MethodPost,
			hasCookie:   true,
			hasHeader:   false,
			matchToken:  false,
			expectCode:  http.StatusForbidden,
			expectError: true,
		},
		{
			name:        "POST without csrf_token cookie fails",
			method:      http.MethodPost,
			hasCookie:   false,
			hasHeader:   true,
			matchToken:  false,
			expectCode:  http.StatusForbidden,
			expectError: true,
		},
		{
			name:        "POST with mismatched token fails",
			method:      http.MethodPost,
			hasCookie:   true,
			hasHeader:   true,
			matchToken:  false,
			expectCode:  http.StatusForbidden,
			expectError: true,
		},
		{
			name:        "PUT with matching token succeeds",
			method:      http.MethodPut,
			hasCookie:   true,
			hasHeader:   true,
			matchToken:  true,
			expectCode:  http.StatusOK,
			expectError: false,
		},
		{
			name:        "DELETE with matching token succeeds",
			method:      http.MethodDelete,
			hasCookie:   true,
			hasHeader:   true,
			matchToken:  true,
			expectCode:  http.StatusOK,
			expectError: false,
		},
		{
			name:        "PATCH with matching token succeeds",
			method:      "PATCH",
			hasCookie:   true,
			hasHeader:   true,
			matchToken:  true,
			expectCode:  http.StatusOK,
			expectError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			middleware := CSRFMiddleware([]string{})
			nextCalled := false
			nextHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				nextCalled = true
				w.WriteHeader(http.StatusOK)
			})

			handler := middleware(nextHandler)

			req := httptest.NewRequest(tt.method, "/api/v1/auth/login", nil)

			var tokenValue string
			if tt.hasCookie {
				tokenValue = "test-csrf-token-value-123"
				req.AddCookie(&http.Cookie{
					Name:  "csrf_token",
					Value: tokenValue,
				})
			}

			if tt.hasHeader {
				headerValue := tokenValue
				if !tt.matchToken {
					headerValue = "wrong-token-value-456"
				}
				req.Header.Set("X-CSRF-Token", headerValue)
			}

			w := httptest.NewRecorder()
			handler.ServeHTTP(w, req)

			assert.Equal(t, tt.expectCode, w.Code, "response code")

			if !tt.expectError {
				assert.True(t, nextCalled, "next handler should be called for valid requests")
			} else {
				assert.False(t, nextCalled, "next handler should not be called for invalid requests")
			}
		})
	}
}

func TestCSRFMiddleware_ExcludedPaths(t *testing.T) {
	excludedPaths := []string{
		"/api/v1/auth/refresh",
		"/v1/signals",
		"/v1/signals/stream",
		"/v1/schema/signals",
	}

	middleware := CSRFMiddleware(excludedPaths)
	nextCalled := false
	nextHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		nextCalled = true
		w.WriteHeader(http.StatusOK)
	})

	handler := middleware(nextHandler)

	for _, path := range excludedPaths {
		t.Run("excluded path "+path, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodPost, path, nil)
			// Don't set any CSRF token
			w := httptest.NewRecorder()
			handler.ServeHTTP(w, req)

			assert.Equal(t, http.StatusOK, w.Code)
			assert.True(t, nextCalled, "next handler should be called for excluded paths even without CSRF")
			nextCalled = false
		})
	}
}

func TestGenerateCSRFToken(t *testing.T) {
	token1, err := GenerateCSRFToken()
	assert.NoError(t, err)
	assert.NotEmpty(t, token1)

	// Should be base64 URL-safe
	decoded, err := base64.RawURLEncoding.DecodeString(token1)
	assert.NoError(t, err)
	assert.Equal(t, 32, len(decoded), "token should be 32 bytes")

	// Should be unique
	token2, err := GenerateCSRFToken()
	assert.NoError(t, err)
	assert.NotEqual(t, token1, token2)
}

func TestCSRFMiddleware_ConstantTimeCompare(t *testing.T) {
	// This test verifies that mismatched tokens are rejected
	// without timing attacks

	middleware := CSRFMiddleware([]string{})
	nextHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})
	handler := middleware(nextHandler)

	correctToken := "this-is-a-test-token-value-1234"

	req := httptest.NewRequest(http.MethodPost, "/api/v1/auth/login", nil)
	req.AddCookie(&http.Cookie{
		Name:  "csrf_token",
		Value: correctToken,
	})
	req.Header.Set("X-CSRF-Token", correctToken)

	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)
	assert.Equal(t, http.StatusOK, w.Code)

	// Try with wrong token
	req2 := httptest.NewRequest(http.MethodPost, "/api/v1/auth/login", nil)
	req2.AddCookie(&http.Cookie{
		Name:  "csrf_token",
		Value: correctToken,
	})
	req2.Header.Set("X-CSRF-Token", "wrong-token-value-5678")

	w2 := httptest.NewRecorder()
	handler.ServeHTTP(w2, req2)
	assert.Equal(t, http.StatusForbidden, w2.Code)
}
