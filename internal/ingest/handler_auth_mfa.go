package ingest

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"io"
	"net/http"
	"time"

	"github.com/argusxdr/argus/internal/auth"
	"github.com/argusxdr/argus/internal/secrets"
	"go.uber.org/zap"
)

// ---- request/response types ----

type mfaEnrollRequest struct {
	// Empty body — uses authenticated user's email from context
}

type mfaEnrollResponse struct {
	QRCode    string `json:"qr_code"`
	SecretB32 string `json:"secret_base32"`
}

type mfaVerifyRequest struct {
	SecretB32 string `json:"secret_base32"`
	Code      string `json:"code"`
}

type mfaVerifyResponse struct {
	BackupCodes []string `json:"backup_codes"`
	Message     string   `json:"message"`
}

type mfaDisableRequest struct {
	Code string `json:"code"`
}

type mfaDisableResponse struct {
	Message string `json:"message"`
}

type mfaChallengeRequest struct {
	MFAToken   string `json:"mfa_token"`
	Code       string `json:"code,omitempty"`
	BackupCode string `json:"backup_code,omitempty"`
}

type mfaChallengeResponse struct {
	AccessToken string `json:"access_token"`
	ExpiresIn   int    `json:"expires_in"`
	TokenType   string `json:"token_type"`
}

// ---- handlers ----

// handleMFAEnroll generates a new TOTP secret and QR code for enrollment.
// Requires active JWT (user ID via auth.UserIDFromContext).
// Does NOT persist the secret yet — the user must call /verify to confirm.
func (h *QueryHandler) handleMFAEnroll(w http.ResponseWriter, r *http.Request) {
	if !h.authAvailable() {
		jsonError(w, "auth service unavailable", http.StatusServiceUnavailable)
		return
	}

	userID, ok := auth.UserIDFromContext(r.Context())
	if !ok {
		jsonError(w, "user not in context", http.StatusUnauthorized)
		return
	}

	// Get user to retrieve email
	user, err := h.authService.UserStore.GetUserByID(r.Context(), userID)
	if err != nil || user == nil {
		jsonError(w, "user not found", http.StatusNotFound)
		return
	}

	// Generate TOTP secret
	secret, err := auth.GenerateTOTPSecret(user.Email, "ArgusXDR")
	if err != nil {
		h.log.Error("failed to generate TOTP secret", zap.Error(err))
		jsonError(w, "failed to generate TOTP secret", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(mfaEnrollResponse{
		QRCode:    secret.URL(),
		SecretB32: secret.Secret(),
	})
}

// handleMFAVerify validates the TOTP code and persists the encrypted secret.
// Requires active JWT. Body contains {secret_base32, code}.
// On success, generates 8 backup codes and returns them (plaintext, one-time only).
func (h *QueryHandler) handleMFAVerify(w http.ResponseWriter, r *http.Request) {
	if !h.authAvailable() {
		jsonError(w, "auth service unavailable", http.StatusServiceUnavailable)
		return
	}

	userID, ok := auth.UserIDFromContext(r.Context())
	if !ok {
		jsonError(w, "user not in context", http.StatusUnauthorized)
		return
	}

	var req mfaVerifyRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		jsonError(w, "invalid request body", http.StatusBadRequest)
		return
	}

	if req.SecretB32 == "" || req.Code == "" {
		jsonError(w, "secret_base32 and code required", http.StatusBadRequest)
		return
	}

	// Validate the TOTP code
	if !auth.ValidateTOTPCode(req.SecretB32, req.Code) {
		jsonError(w, "invalid TOTP code", http.StatusUnauthorized)
		return
	}

	// Fetch KEK for encryption
	key, ok := secrets.GetSecret(secrets.KeyMFAEncryption)
	if !ok {
		h.log.Error("mfa encryption key not configured")
		jsonError(w, `{"error":"mfa encryption key not configured"}`, http.StatusInternalServerError)
		return
	}

	// Decode base64 KEK
	kek, err := base64.StdEncoding.DecodeString(key)
	if err != nil || len(kek) != 32 {
		h.log.Error("mfa encryption key invalid", zap.Error(err))
		jsonError(w, `{"error":"mfa encryption key invalid"}`, http.StatusInternalServerError)
		return
	}

	// Encrypt the secret using AES-256-GCM
	block, err := aes.NewCipher(kek)
	if err != nil {
		h.log.Error("failed to create cipher", zap.Error(err))
		jsonError(w, "internal error", http.StatusInternalServerError)
		return
	}

	gcm, err := cipher.NewGCM(block)
	if err != nil {
		h.log.Error("failed to create GCM", zap.Error(err))
		jsonError(w, "internal error", http.StatusInternalServerError)
		return
	}

	// Generate nonce
	nonce := make([]byte, gcm.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		h.log.Error("failed to generate nonce", zap.Error(err))
		jsonError(w, "internal error", http.StatusInternalServerError)
		return
	}

	// Encrypt the secret
	ciphertext := gcm.Seal(nonce, nonce, []byte(req.SecretB32), nil)
	encryptedSecret := base64.StdEncoding.EncodeToString(ciphertext)

	// Generate 8 backup codes
	backupCodes, err := auth.GenerateBackupCodes(8)
	if err != nil {
		h.log.Error("failed to generate backup codes", zap.Error(err))
		jsonError(w, "failed to generate backup codes", http.StatusInternalServerError)
		return
	}

	// Update user to set mfa_enabled=true and mfa_secret_encrypted
	user, err := h.authService.UserStore.GetUserByID(r.Context(), userID)
	if err != nil || user == nil {
		jsonError(w, "user not found", http.StatusNotFound)
		return
	}

	user.MFAEnabled = true
	user.MFASecretEncrypted = &encryptedSecret
	if err := h.authService.UserStore.UpdateUser(r.Context(), user); err != nil {
		h.log.Error("failed to update user with MFA secret", zap.Error(err))
		jsonError(w, "failed to persist MFA secret", http.StatusInternalServerError)
		return
	}

	// Insert backup codes into user_backup_codes table
	if h.pool != nil {
		for _, code := range backupCodes {
			hash := auth.HashBackupCode(code)
			_, err := h.pool.Exec(r.Context(), `
				INSERT INTO user_backup_codes (user_id, code_hash, created_at)
				VALUES ($1, $2, $3)
			`, userID, hash, time.Now())
			if err != nil {
				h.log.Warn("failed to insert backup code", zap.Error(err))
			}
		}
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(mfaVerifyResponse{
		BackupCodes: backupCodes,
		Message:     "MFA enabled. Backup codes above should be saved in a secure location.",
	})
}

// handleMFADisable disables MFA for the user.
// Requires active JWT. Body contains {code} (TOTP or backup code).
// On success, clears mfa_secret_encrypted, sets mfa_enabled=false, and deletes unused backup codes.
func (h *QueryHandler) handleMFADisable(w http.ResponseWriter, r *http.Request) {
	if !h.authAvailable() {
		jsonError(w, "auth service unavailable", http.StatusServiceUnavailable)
		return
	}

	userID, ok := auth.UserIDFromContext(r.Context())
	if !ok {
		jsonError(w, "user not in context", http.StatusUnauthorized)
		return
	}

	var req mfaDisableRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		jsonError(w, "invalid request body", http.StatusBadRequest)
		return
	}

	if req.Code == "" {
		jsonError(w, "code required", http.StatusBadRequest)
		return
	}

	// Get user
	user, err := h.authService.UserStore.GetUserByID(r.Context(), userID)
	if err != nil || user == nil {
		jsonError(w, "user not found", http.StatusNotFound)
		return
	}

	if !user.MFAEnabled || user.MFASecretEncrypted == nil {
		jsonError(w, "MFA not enabled for this user", http.StatusBadRequest)
		return
	}

	// Fetch KEK for decryption
	key, ok := secrets.GetSecret(secrets.KeyMFAEncryption)
	if !ok {
		h.log.Error("mfa encryption key not configured")
		jsonError(w, `{"error":"mfa encryption key not configured"}`, http.StatusInternalServerError)
		return
	}

	// Decode base64 KEK
	kek, err := base64.StdEncoding.DecodeString(key)
	if err != nil || len(kek) != 32 {
		h.log.Error("mfa encryption key invalid", zap.Error(err))
		jsonError(w, `{"error":"mfa encryption key invalid"}`, http.StatusInternalServerError)
		return
	}

	// Decrypt the stored secret
	block, err := aes.NewCipher(kek)
	if err != nil {
		h.log.Error("failed to create cipher", zap.Error(err))
		jsonError(w, "internal error", http.StatusInternalServerError)
		return
	}

	gcm, err := cipher.NewGCM(block)
	if err != nil {
		h.log.Error("failed to create GCM", zap.Error(err))
		jsonError(w, "internal error", http.StatusInternalServerError)
		return
	}

	ciphertext, err := base64.StdEncoding.DecodeString(*user.MFASecretEncrypted)
	if err != nil {
		h.log.Error("failed to decode encrypted secret", zap.Error(err))
		jsonError(w, "internal error", http.StatusInternalServerError)
		return
	}

	if len(ciphertext) < gcm.NonceSize() {
		h.log.Error("encrypted secret too short")
		jsonError(w, "internal error", http.StatusInternalServerError)
		return
	}

	nonce, ciphertextOnly := ciphertext[:gcm.NonceSize()], ciphertext[gcm.NonceSize():]
	plaintext, err := gcm.Open(nil, nonce, ciphertextOnly, nil)
	if err != nil {
		h.log.Error("failed to decrypt secret", zap.Error(err))
		jsonError(w, "internal error", http.StatusInternalServerError)
		return
	}

	secretB32 := string(plaintext)

	// Validate TOTP code
	codeValid := auth.ValidateTOTPCode(secretB32, req.Code)

	// If not valid as TOTP, check backup codes
	if !codeValid && h.pool != nil {
		row := h.pool.QueryRow(r.Context(), `
			SELECT code_hash FROM user_backup_codes
			WHERE user_id = $1 AND used_at IS NULL LIMIT 1
		`, userID)

		var hash string
		err := row.Scan(&hash)
		if err == nil && auth.VerifyBackupCode(req.Code, hash) {
			codeValid = true
			// Mark the backup code as used
			_, _ = h.pool.Exec(r.Context(), `
				UPDATE user_backup_codes SET used_at = $1
				WHERE user_id = $2 AND code_hash = $3
			`, time.Now(), userID, hash)
		}
	}

	if !codeValid {
		jsonError(w, "invalid TOTP code or backup code", http.StatusUnauthorized)
		return
	}

	// Clear MFA settings
	user.MFAEnabled = false
	user.MFASecretEncrypted = nil
	if err := h.authService.UserStore.UpdateUser(r.Context(), user); err != nil {
		h.log.Error("failed to update user", zap.Error(err))
		jsonError(w, "failed to disable MFA", http.StatusInternalServerError)
		return
	}

	// Delete all backup codes for this user
	if h.pool != nil {
		_, _ = h.pool.Exec(r.Context(), `DELETE FROM user_backup_codes WHERE user_id = $1`, userID)
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(mfaDisableResponse{
		Message: "MFA disabled successfully",
	})
}

// handleMFAChallenge exchanges an mfa_token and valid TOTP/backup code for a full access token.
// PUBLIC endpoint — no JWT required. Body contains {mfa_token, code?, backup_code?}.
// On success, returns full access token + refresh token + creates session (mirrors handleLogin response).
func (h *QueryHandler) handleMFAChallenge(w http.ResponseWriter, r *http.Request) {
	if !h.authAvailable() {
		jsonError(w, "auth service unavailable", http.StatusServiceUnavailable)
		return
	}

	var req mfaChallengeRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		jsonError(w, "invalid request body", http.StatusBadRequest)
		return
	}

	if req.MFAToken == "" {
		jsonError(w, "mfa_token required", http.StatusBadRequest)
		return
	}

	if req.Code == "" && req.BackupCode == "" {
		jsonError(w, "code or backup_code required", http.StatusBadRequest)
		return
	}

	// Verify MFA token
	userID, err := h.authService.TokenMgr.VerifyMFAToken(req.MFAToken)
	if err != nil {
		jsonError(w, "invalid or expired MFA token", http.StatusUnauthorized)
		return
	}

	// Get user
	user, err := h.authService.UserStore.GetUserByID(r.Context(), userID)
	if err != nil || user == nil {
		jsonError(w, "user not found", http.StatusNotFound)
		return
	}

	if !user.MFAEnabled || user.MFASecretEncrypted == nil {
		jsonError(w, "MFA not enabled for this user", http.StatusBadRequest)
		return
	}

	// Fetch KEK for decryption
	key, ok := secrets.GetSecret(secrets.KeyMFAEncryption)
	if !ok {
		h.log.Error("mfa encryption key not configured")
		jsonError(w, `{"error":"mfa encryption key not configured"}`, http.StatusInternalServerError)
		return
	}

	// Decode base64 KEK
	kek, err := base64.StdEncoding.DecodeString(key)
	if err != nil || len(kek) != 32 {
		h.log.Error("mfa encryption key invalid", zap.Error(err))
		jsonError(w, `{"error":"mfa encryption key invalid"}`, http.StatusInternalServerError)
		return
	}

	// Decrypt the stored secret
	block, err := aes.NewCipher(kek)
	if err != nil {
		h.log.Error("failed to create cipher", zap.Error(err))
		jsonError(w, "internal error", http.StatusInternalServerError)
		return
	}

	gcm, err := cipher.NewGCM(block)
	if err != nil {
		h.log.Error("failed to create GCM", zap.Error(err))
		jsonError(w, "internal error", http.StatusInternalServerError)
		return
	}

	ciphertext, err := base64.StdEncoding.DecodeString(*user.MFASecretEncrypted)
	if err != nil {
		h.log.Error("failed to decode encrypted secret", zap.Error(err))
		jsonError(w, "internal error", http.StatusInternalServerError)
		return
	}

	if len(ciphertext) < gcm.NonceSize() {
		h.log.Error("encrypted secret too short")
		jsonError(w, "internal error", http.StatusInternalServerError)
		return
	}

	nonce, ciphertextOnly := ciphertext[:gcm.NonceSize()], ciphertext[gcm.NonceSize():]
	plaintext, err := gcm.Open(nil, nonce, ciphertextOnly, nil)
	if err != nil {
		h.log.Error("failed to decrypt secret", zap.Error(err))
		jsonError(w, "internal error", http.StatusInternalServerError)
		return
	}

	secretB32 := string(plaintext)

	// Validate code
	codeValid := false

	// Try TOTP code if provided
	if req.Code != "" {
		codeValid = auth.ValidateTOTPCode(secretB32, req.Code)
	}

	// Try backup code if provided (and TOTP didn't work)
	if !codeValid && req.BackupCode != "" && h.pool != nil {
		row := h.pool.QueryRow(r.Context(), `
			SELECT code_hash FROM user_backup_codes
			WHERE user_id = $1 AND used_at IS NULL
		`, userID)

		var hash string
		err := row.Scan(&hash)
		if err == nil && auth.VerifyBackupCode(req.BackupCode, hash) {
			codeValid = true
			// Mark the backup code as used
			_, _ = h.pool.Exec(r.Context(), `
				UPDATE user_backup_codes SET used_at = $1
				WHERE user_id = $2 AND code_hash = $3
			`, time.Now(), userID, hash)
		}
	}

	if !codeValid {
		jsonError(w, "invalid TOTP code or backup code", http.StatusUnauthorized)
		return
	}

	// Issue full access token + refresh token (mirror handleLogin)
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
