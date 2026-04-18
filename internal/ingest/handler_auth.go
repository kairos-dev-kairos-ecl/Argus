package ingest

import (
	"encoding/json"
	"net/http"
	"time"

	"github.com/argusxdr/argus/internal/auth"
	"github.com/google/uuid"
	"go.uber.org/zap"
)

// AuthService is a convenience facade holding all auth subsystem components.
// Injected into QueryHandler via SetAuthService.
type AuthService struct {
	UserSvc      *auth.UserService    // AuthenticateUser, CreateUser
	UserStore    auth.UserStore        // ListUsers, GetUserByID
	SessionMgr   *auth.SessionManager
	TokenMgr     *auth.TokenManager
	AuditLog     *auth.AuditLogger    // audit logging
	SessionStore *auth.PgSessionStore
}

// ---- request/response types ----

type loginRequest struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}

type loginResponse struct {
	AccessToken string `json:"access_token"`
	ExpiresIn   int    `json:"expires_in"` // seconds
	TokenType   string `json:"token_type"`
}

type setupRequest struct {
	Email        string `json:"email"`
	Password     string `json:"password"`
	DisplayName  string `json:"display_name"`
	InstanceName string `json:"instance_name"`
	AppName      string `json:"app_name"`
}

// ---- handlers ----

func (h *QueryHandler) handleSetup(w http.ResponseWriter, r *http.Request) {
	if !h.authAvailable() {
		jsonError(w, "auth service unavailable", http.StatusServiceUnavailable)
		return
	}

	var req setupRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		jsonError(w, "invalid request body", http.StatusBadRequest)
		return
	}

	sm := auth.NewSetupManager(h.authService.UserSvc, h.authService.UserStore)
	required, err := sm.IsSetupRequired(r.Context())
	if err != nil {
		h.log.Error("setup check failed", zap.Error(err))
		jsonError(w, "internal error", http.StatusInternalServerError)
		return
	}
	if !required {
		jsonError(w, "setup already completed", http.StatusConflict)
		return
	}

	setupResp, err := sm.PerformSetup(r.Context(), &auth.SetupRequest{
		Email:        req.Email,
		Password:     req.Password,
		DisplayName:  req.DisplayName,
		InstanceName: req.InstanceName,
		AppName:      req.AppName,
	})
	if err != nil {
		jsonError(w, err.Error(), http.StatusBadRequest)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(setupResp)
}

func (h *QueryHandler) handleLogin(w http.ResponseWriter, r *http.Request) {
	if !h.authAvailable() {
		jsonError(w, "auth service unavailable", http.StatusServiceUnavailable)
		return
	}

	var req loginRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		jsonError(w, "invalid request body", http.StatusBadRequest)
		return
	}
	if req.Email == "" || req.Password == "" {
		jsonError(w, "email and password required", http.StatusBadRequest)
		return
	}

	user, err := h.authService.UserSvc.AuthenticateUser(r.Context(), req.Email, req.Password, getIP(r))
	if err != nil {
		jsonError(w, "invalid credentials", http.StatusUnauthorized)
		return
	}

	perms := auth.NewPermissionChecker().GetPermissionsForRole(user.Role)
	accessToken, err := h.authService.TokenMgr.IssueAccessToken(
		user.ID, user.Email, user.DisplayName, user.Role, perms,
	)
	if err != nil {
		h.log.Error("failed to issue access token", zap.Error(err))
		jsonError(w, "internal error", http.StatusInternalServerError)
		return
	}

	refreshToken, sessionID, err := h.authService.SessionMgr.CreateSession(
		r.Context(), user.ID, r.UserAgent(), getIP(r),
	)
	if err != nil {
		h.log.Warn("failed to create session", zap.Error(err))
		// Non-fatal: still return access token
	} else if h.authService.SessionStore != nil {
		now := time.Now()
		sess := &auth.Session{
			ID:               sessionID,
			UserID:           user.ID.String(),
			RefreshTokenHash: h.authService.SessionMgr.HashToken(refreshToken),
			UserAgent:        r.UserAgent(),
			IPAddress:        getIP(r),
			CreatedAt:        now.Unix(),
			ExpiresAt:        now.Add(7 * 24 * time.Hour).Unix(),
			LastUsedAt:       now.Unix(),
		}
		if err := h.authService.SessionStore.CreateSession(r.Context(), sess); err != nil {
			h.log.Warn("failed to persist session", zap.Error(err))
		}

		http.SetCookie(w, &http.Cookie{
			Name:     "refresh_token",
			Value:    refreshToken,
			Path:     "/",
			HttpOnly: true,
			Secure:   r.TLS != nil,
			SameSite: http.SameSiteLaxMode,
			MaxAge:   int((7 * 24 * time.Hour).Seconds()),
		})
	}

	if h.authService.AuditLog != nil {
		h.authService.AuditLog.LogLogin(r.Context(), user.ID, true, getIP(r))
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(loginResponse{
		AccessToken: accessToken,
		ExpiresIn:   int((15 * time.Minute).Seconds()),
		TokenType:   "Bearer",
	})
}

func (h *QueryHandler) handleRefreshToken(w http.ResponseWriter, r *http.Request) {
	if !h.authAvailable() {
		jsonError(w, "auth service unavailable", http.StatusServiceUnavailable)
		return
	}

	cookie, err := r.Cookie("refresh_token")
	if err != nil || cookie.Value == "" {
		jsonError(w, "missing refresh token", http.StatusUnauthorized)
		return
	}

	oldHash := h.authService.SessionMgr.HashToken(cookie.Value)
	sess, err := h.authService.SessionStore.GetSessionByHash(r.Context(), oldHash)
	if err != nil || sess == nil {
		jsonError(w, "invalid refresh token", http.StatusUnauthorized)
		return
	}
	if sess.RevokedAt != nil || sess.ExpiresAt < time.Now().Unix() {
		jsonError(w, "refresh token expired or revoked", http.StatusUnauthorized)
		return
	}

	// Rotate: generate new token, update DB
	newToken, newHash, err := h.authService.SessionMgr.RotateRefreshToken(r.Context(), oldHash)
	if err != nil {
		jsonError(w, "could not rotate refresh token", http.StatusInternalServerError)
		return
	}
	if err := h.authService.SessionStore.UpdateSessionHash(r.Context(), sess.ID, newHash); err != nil {
		h.log.Warn("failed to persist rotated session hash", zap.Error(err))
	}

	// Fetch user to re-issue access token
	userID, parseErr := uuid.Parse(sess.UserID)
	if parseErr != nil {
		jsonError(w, "invalid session", http.StatusInternalServerError)
		return
	}
	user, err := h.authService.UserStore.GetUserByID(r.Context(), userID)
	if err != nil || user == nil {
		jsonError(w, "user not found", http.StatusUnauthorized)
		return
	}

	perms := auth.NewPermissionChecker().GetPermissionsForRole(user.Role)
	accessToken, err := h.authService.TokenMgr.IssueAccessToken(
		user.ID, user.Email, user.DisplayName, user.Role, perms,
	)
	if err != nil {
		jsonError(w, "internal error", http.StatusInternalServerError)
		return
	}

	http.SetCookie(w, &http.Cookie{
		Name:     "refresh_token",
		Value:    newToken,
		Path:     "/",
		HttpOnly: true,
		Secure:   r.TLS != nil,
		SameSite: http.SameSiteLaxMode,
		MaxAge:   int((7 * 24 * time.Hour).Seconds()),
	})

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(loginResponse{
		AccessToken: accessToken,
		ExpiresIn:   int((15 * time.Minute).Seconds()),
		TokenType:   "Bearer",
	})
}

func (h *QueryHandler) handleLogout(w http.ResponseWriter, r *http.Request) {
	// Logout is always 200 (idempotent — can't reveal whether session existed)
	if h.authAvailable() {
		if cookie, err := r.Cookie("refresh_token"); err == nil && cookie.Value != "" {
			hash := h.authService.SessionMgr.HashToken(cookie.Value)
			if sess, err := h.authService.SessionStore.GetSessionByHash(r.Context(), hash); err == nil && sess != nil {
				h.authService.SessionStore.RevokeSession(r.Context(), sess.ID)
			}
		}
		// Revoke access token if present
		if authHeader := r.Header.Get("Authorization"); len(authHeader) > 7 {
			tokenStr := authHeader[7:]
			if claims, err := h.authService.TokenMgr.VerifyTokenSignature(tokenStr); err == nil {
				expiry := time.Now().Add(15 * time.Minute) // max access token TTL
				if claims.ExpiresAt != nil {
					expiry = claims.ExpiresAt.Time
				}
				h.authService.SessionStore.RevokeTokenHash(r.Context(), hashTokenStr(tokenStr), expiry)
			}
		}
	}

	http.SetCookie(w, &http.Cookie{
		Name:   "refresh_token",
		Value:  "",
		Path:   "/",
		MaxAge: -1,
	})
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]string{"status": "logged out"})
}

// ---- helpers ----

func getIP(r *http.Request) string {
	var ip string
	if xff := r.Header.Get("X-Forwarded-For"); xff != "" {
		ip = xff
	} else if xri := r.Header.Get("X-Real-IP"); xri != "" {
		ip = xri
	} else {
		ip = r.RemoteAddr
	}

	// Safeguard against empty IP
	if len(ip) == 0 {
		return ""
	}

	// Strip port from IP:port format (required for PostgreSQL INET type)
	// Format is "IP:port", we only want the IP
	if idx := lastIndexOf(ip, ':'); idx > 0 {
		// Check if it's IPv6 (has multiple colons)
		if countChar(ip, ':') > 1 {
			// IPv6 address in brackets like [::1]:port, strip port
			if ip[0] == '[' && idx > 1 {
				return ip[1 : idx-1]
			}
			// IPv6 without brackets, return as-is
			return ip
		}
		// IPv4 address, strip port
		return ip[:idx]
	}
	return ip
}

// Helper functions for IP parsing
func lastIndexOf(s string, c byte) int {
	for i := len(s) - 1; i >= 0; i-- {
		if s[i] == c {
			return i
		}
	}
	return -1
}

func countChar(s string, c byte) int {
	count := 0
	for i := 0; i < len(s); i++ {
		if s[i] == c {
			count++
		}
	}
	return count
}

func hashTokenStr(token string) string {
	return auth.HashToken(token)
}
