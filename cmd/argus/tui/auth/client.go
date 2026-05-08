package auth

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
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
func NewClient(baseURL string) *Client {
	return &Client{
		baseURL: baseURL,
		httpClient: &http.Client{
			Timeout: 10 * time.Second,
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

// Login authenticates with email and password.
// On success: returns a populated *AuthState with mfaRequired=false.
// On MFA requirement: returns a partial *AuthState (RefreshToken holds the mfa_token
// scratch value), mfaRequired=true, and no access token.
func (c *Client) Login(ctx context.Context, email, password string) (*AuthState, bool, error) {
	body, err := json.Marshal(loginRequest{Email: email, Password: password})
	if err != nil {
		return nil, false, fmt.Errorf("auth: marshal login request: %w", err)
	}

	resp, err := c.post(ctx, "/api/v1/auth/login", bytes.NewReader(body))
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
