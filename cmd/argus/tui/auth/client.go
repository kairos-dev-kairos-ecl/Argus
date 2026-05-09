package auth

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/cookiejar"
	"time"
)

// Client handles authentication requests against the Argus backend.
// It operates against:
//   - POST /api/v1/auth/login
//   - POST /api/v1/auth/mfa/verify
//   - POST /api/v1/auth/refresh
//   - POST /api/v1/auth/logout
type Client struct {
	baseURL    string
	httpClient *http.Client
}

// NewClient creates an auth client targeting the given base URL.
// A cookie jar is required so the csrf_token cookie set by GET /api/v1/auth/csrf-token
// is automatically carried on subsequent POST requests (double-submit CSRF pattern).
func NewClient(baseURL string) *Client {
	jar, _ := cookiejar.New(nil) // error only on nil Options — always nil here
	return &Client{
		baseURL: baseURL,
		httpClient: &http.Client{
			Timeout: 10 * time.Second,
			Jar:     jar,
		},
	}
}

// loginRequest is the JSON body sent to POST /api/v1/auth/login.
type loginRequest struct {
	Email    string `json:"email"`
	Password string `json:"password"` // #nosec G101 — this is the field name, not a credential
}

// loginResponse is the JSON body returned by POST /api/v1/auth/login.
type loginResponse struct {
	AccessToken  string `json:"access_token"`
	RefreshToken string `json:"refresh_token"`
	MFARequired  bool   `json:"mfa_required"`
	MFAToken     string `json:"mfa_token"`
	User         struct {
		Email string `json:"email"`
		Role  string `json:"role"`
	} `json:"user"`
}

// mfaVerifyRequest is the JSON body sent to POST /api/v1/auth/mfa/verify.
type mfaVerifyRequest struct {
	MFAToken string `json:"mfa_token"`
	Code     string `json:"code"`
}

// refreshRequest is the JSON body sent to POST /api/v1/auth/refresh.
type refreshRequest struct {
	RefreshToken string `json:"refresh_token"`
}

// fetchCSRF performs GET /api/v1/auth/csrf-token to prime the CSRF cookie and returns
// the token value from the X-CSRF-Token response header.
// The server sets the csrf_token cookie (captured by the jar) and echoes the value in
// the header. Both must be sent on subsequent mutating requests.
func (c *Client) fetchCSRF(ctx context.Context) (string, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.baseURL+"/api/v1/auth/csrf-token", nil)
	if err != nil {
		return "", fmt.Errorf("auth: build csrf-token request: %w", err)
	}
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("auth: csrf-token request: %w", err)
	}
	defer resp.Body.Close()
	token := resp.Header.Get("X-CSRF-Token")
	if token == "" {
		return "", fmt.Errorf("auth: csrf-token response missing X-CSRF-Token header")
	}
	return token, nil
}

// Login authenticates with email and password.
// On success: returns a populated *AuthState with mfaRequired=false.
// On MFA requirement: returns a partial *AuthState (RefreshToken holds the mfa_token
// scratch value), mfaRequired=true, and no access token.
func (c *Client) Login(ctx context.Context, email, password string) (*AuthState, bool, error) {
	// Step 1: fetch CSRF token (also sets csrf_token cookie in the jar).
	csrfToken, err := c.fetchCSRF(ctx)
	if err != nil {
		return nil, false, fmt.Errorf("auth: prefetch csrf token: %w", err)
	}

	body, err := json.Marshal(loginRequest{Email: email, Password: password})
	if err != nil {
		return nil, false, fmt.Errorf("auth: marshal login request: %w", err)
	}

	// Step 2: POST with X-CSRF-Token header; cookie jar carries the csrf_token cookie.
	resp, err := c.postWithCSRF(ctx, "/api/v1/auth/login", bytes.NewReader(body), csrfToken)
	if err != nil {
		return nil, false, fmt.Errorf("auth: login request: %w", err)
	}
	defer resp.Body.Close()

	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, false, fmt.Errorf("auth: read login response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return nil, false, fmt.Errorf("auth: login failed (status %d): %s", resp.StatusCode, data)
	}

	var loginResp loginResponse
	if err := json.Unmarshal(data, &loginResp); err != nil {
		return nil, false, fmt.Errorf("auth: decode login response: %w", err)
	}

	state := NewAuthState()

	if loginResp.MFARequired {
		// Store the mfa_token in RefreshToken field as scratch space.
		// It will be replaced with the real refresh token after VerifyMFA.
		state.Set("", loginResp.MFAToken, time.Time{}, "", "")
		return state, true, nil
	}

	state.Set(
		loginResp.AccessToken,
		loginResp.RefreshToken,
		time.Now().Add(15*time.Minute), // default expiry; real expiry would come from JWT claims
		loginResp.User.Email,
		loginResp.User.Role,
	)
	return state, false, nil
}

// VerifyMFA completes MFA verification.
// mfaToken is the scratch value stored in state.RefreshToken() after a Login
// that returned mfaRequired=true.
func (c *Client) VerifyMFA(ctx context.Context, mfaToken, code string) (*AuthState, error) {
	body, err := json.Marshal(mfaVerifyRequest{MFAToken: mfaToken, Code: code})
	if err != nil {
		return nil, fmt.Errorf("auth: marshal mfa verify: %w", err)
	}

	resp, err := c.post(ctx, "/api/v1/auth/mfa/verify", bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("auth: mfa verify request: %w", err)
	}
	defer resp.Body.Close()

	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("auth: read mfa verify response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("auth: mfa verify failed (status %d): %s", resp.StatusCode, data)
	}

	var loginResp loginResponse
	if err := json.Unmarshal(data, &loginResp); err != nil {
		return nil, fmt.Errorf("auth: decode mfa verify response: %w", err)
	}

	state := NewAuthState()
	state.Set(
		loginResp.AccessToken,
		loginResp.RefreshToken,
		time.Now().Add(15*time.Minute),
		loginResp.User.Email,
		loginResp.User.Role,
	)
	return state, nil
}

// RefreshTokens posts the current refresh token and mutates state in place.
// If the refresh fails, state is NOT cleared here — the caller (APIClient) is
// responsible for calling state.ClearOnLogout() when a refresh failure means
// the session is definitively over.
func (c *Client) RefreshTokens(ctx context.Context, state *AuthState) error {
	body, err := json.Marshal(refreshRequest{RefreshToken: state.RefreshToken()})
	if err != nil {
		return fmt.Errorf("auth: marshal refresh: %w", err)
	}

	resp, err := c.post(ctx, "/api/v1/auth/refresh", bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("auth: refresh request: %w", err)
	}
	defer resp.Body.Close()

	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("auth: read refresh response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("auth: refresh failed (status %d)", resp.StatusCode)
	}

	var loginResp loginResponse
	if err := json.Unmarshal(data, &loginResp); err != nil {
		return fmt.Errorf("auth: decode refresh response: %w", err)
	}

	state.Set(
		loginResp.AccessToken,
		loginResp.RefreshToken,
		time.Now().Add(15*time.Minute),
		loginResp.User.Email,
		loginResp.User.Role,
	)
	return nil
}

// Logout posts to /api/v1/auth/logout and calls state.ClearOnLogout() regardless
// of the HTTP outcome. This ensures tokens are always cleared on logout attempts.
func (c *Client) Logout(ctx context.Context, state *AuthState) error {
	// Always clear state, even if the HTTP call fails.
	defer state.ClearOnLogout()

	resp, err := c.post(ctx, "/api/v1/auth/logout", http.NoBody)
	if err != nil {
		return fmt.Errorf("auth: logout request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusNoContent {
		return fmt.Errorf("auth: logout failed (status %d)", resp.StatusCode)
	}
	return nil
}

// post sends a POST request to path with the given body.
func (c *Client) post(ctx context.Context, path string, body io.Reader) (*http.Response, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+path, body)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	return c.httpClient.Do(req)
}

// postWithCSRF sends a POST request with the X-CSRF-Token header set.
// Use for any mutating auth endpoint that requires the double-submit CSRF pattern.
// The csrf_token cookie is carried automatically by the client's cookie jar.
func (c *Client) postWithCSRF(ctx context.Context, path string, body io.Reader, csrfToken string) (*http.Response, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+path, body)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-CSRF-Token", csrfToken)
	return c.httpClient.Do(req)
}
