package auth

import (
	"bufio"
	"context"
	"crypto/sha1"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/google/uuid"
)

// SetupRequest represents the first-run setup request payload
type SetupRequest struct {
	Email       string `json:"email"`
	DisplayName string `json:"display_name"`
	Password    string `json:"password"`
	InstanceName string `json:"instance_name"`
	Timezone    string `json:"timezone"`
	AppName     string `json:"app_name"`
}

// SetupResponse represents the setup completion response
type SetupResponse struct {
	User    *User             `json:"user"`
	App     map[string]interface{} `json:"app"`
	APIKey  string            `json:"api_key"`
	Message string            `json:"message"`
}

// SetupManager handles first-run setup
type SetupManager struct {
	userService *UserService
	userStore   UserStore
	appStore    interface{} // Would be AppStore
	logger      interface{} // zap.Logger
}

// NewSetupManager creates a new setup manager
func NewSetupManager(userService *UserService, userStore UserStore) *SetupManager {
	return &SetupManager{
		userService: userService,
		userStore:   userStore,
	}
}

// IsSetupRequired checks if setup is needed (no users exist yet)
func (sm *SetupManager) IsSetupRequired(ctx context.Context) (bool, error) {
	// Try to get any user
	users, err := sm.userStore.ListUsers(ctx, map[string]interface{}{})
	if err != nil {
		return false, fmt.Errorf("failed to check users: %w", err)
	}

	return len(users) == 0, nil
}

// PerformSetup creates the initial admin user and app
func (sm *SetupManager) PerformSetup(ctx context.Context, req *SetupRequest) (*SetupResponse, error) {
	// Verify setup is still required
	required, err := sm.IsSetupRequired(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to check setup status: %w", err)
	}
	if !required {
		return nil, fmt.Errorf("setup already completed")
	}

	// Validate input
	if err := sm.ValidateSetupRequest(req); err != nil {
		return nil, fmt.Errorf("validation failed: %w", err)
	}

	// Check password against haveibeenpwned
	if isBreached, err := sm.CheckPasswordBreach(req.Password); err != nil {
		// Log but don't fail - API might be unavailable
		fmt.Printf("warning: failed to check password breach: %v\n", err)
	} else if isBreached {
		return nil, fmt.Errorf("password found in known breaches")
	}

	// Create admin user
	adminUser, err := sm.userService.CreateUser(ctx, req.Email, req.DisplayName, req.Password, RoleAdmin, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create admin user: %w", err)
	}

	// Generate API key for first app
	apiKey := sm.GenerateAPIKey()
	apiKeyHash := sm.HashAPIKey(apiKey)

	// Create first app (simplified - actual app creation would involve app store)
	app := map[string]interface{}{
		"id":        uuid.New().String(),
		"name":      req.AppName,
		"api_key_hash": apiKeyHash,
		"created_at": time.Now().Format(time.RFC3339),
	}

	// Log setup completion
	// sm.logger.Info("setup completed", zap.String("email", req.Email))

	return &SetupResponse{
		User:   adminUser,
		App:    app,
		APIKey: apiKey,
		Message: "Setup completed. Welcome to Argus XDR!",
	}, nil
}

// ValidateSetupRequest validates the setup request
func (sm *SetupManager) ValidateSetupRequest(req *SetupRequest) error {
	if req == nil {
		return fmt.Errorf("request cannot be nil")
	}

	if req.Email == "" {
		return fmt.Errorf("email is required")
	}

	if req.Password == "" {
		return fmt.Errorf("password is required")
	}

	if len(req.Password) < 12 {
		return fmt.Errorf("password must be at least 12 characters")
	}

	if req.DisplayName == "" {
		return fmt.Errorf("display name is required")
	}

	if req.InstanceName == "" {
		return fmt.Errorf("instance name is required")
	}

	if req.AppName == "" {
		return fmt.Errorf("app name is required")
	}

	return nil
}

// CheckPasswordBreach checks if password is in haveibeenpwned database
// Uses k-anonymity: only sends first 5 chars of SHA-1 hash
func (sm *SetupManager) CheckPasswordBreach(password string) (bool, error) {
	// Calculate SHA-1 hash of password
	sum := sha1.Sum([]byte(password))
	hashStr := strings.ToUpper(hex.EncodeToString(sum[:]))
	prefix := hashStr[:5]
	suffix := hashStr[5:]

	// Query haveibeenpwned API with prefix
	url := fmt.Sprintf("https://api.pwnedpasswords.com/range/%s", prefix)
	client := &http.Client{Timeout: 5 * time.Second}

	resp, err := client.Get(url)
	if err != nil {
		// Network error: fail-open (log warning, but don't block setup)
		return false, nil
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		// Server error: fail-open (log warning, but don't block setup)
		return false, nil
	}

	// Parse response and check if full hash is in results
	// Response format: one line per match, format is "SUFFIX:COUNT\r\n"
	scanner := bufio.NewScanner(resp.Body)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}

		// Split on first colon only
		parts := strings.SplitN(line, ":", 2)
		if len(parts) != 2 {
			continue
		}

		// Check if suffix matches
		if parts[0] == suffix {
			// Parse count and check if it's greater than 0
			count, err := strconv.Atoi(parts[1])
			if err != nil {
				// Invalid count format, skip
				continue
			}
			if count > 0 {
				return true, nil
			}
		}
	}

	// No match found
	return false, nil
}

// GenerateAPIKey creates a new API key
func (sm *SetupManager) GenerateAPIKey() string {
	return uuid.New().String()
}

// HashAPIKey creates a hash of an API key for storage
func (sm *SetupManager) HashAPIKey(apiKey string) string {
	hash := sha256.Sum256([]byte(apiKey))
	return hex.EncodeToString(hash[:])
}

// SetupMiddleware redirects to setup if required
func SetupMiddleware(setupManager *SetupManager) func(next http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// Check if path is exempted
			exemptedPaths := map[string]bool{
				"/": true,
				"/setup": true,
				"/api/v1/setup": true,
				"/api/v1/setup/status": true,
				"/login": true,
			}

			if exemptedPaths[r.URL.Path] {
				next.ServeHTTP(w, r)
				return
			}

			// Check if setup is required
			required, err := setupManager.IsSetupRequired(r.Context())
			if err != nil {
				http.Error(w, "internal error", http.StatusInternalServerError)
				return
			}

			if required {
				// Redirect to setup
				http.Redirect(w, r, "/setup", http.StatusSeeOther)
				return
			}

			next.ServeHTTP(w, r)
		})
	}
}
