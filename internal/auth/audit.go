package auth

import (
	"context"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"strings"
	"time"

	"github.com/google/uuid"
)

// AuditAction constants
const (
	ActionLogin            = "login"
	ActionLogout           = "logout"
	ActionTokenRefresh     = "token_refresh"
	ActionUserCreate       = "user_create"
	ActionUserUpdate       = "user_update"
	ActionUserDelete       = "user_delete"
	ActionPasswordReset    = "password_reset"
	ActionRuleCreate       = "rule_create"
	ActionRuleUpdate       = "rule_update"
	ActionRuleDelete       = "rule_delete"
	ActionConfigUpdate     = "config_update"
	ActionAlertAcknowledge = "alert_acknowledge"
	ActionPermissionDenied  = "permission_denied"
)

// AuditLogEntry represents a single audit log entry
type AuditLogEntry struct {
	ID        uuid.UUID
	UserID    *uuid.UUID
	Action    string
	Resource  string
	Detail    map[string]interface{}
	IPAddress string
	UserAgent string
	Timestamp time.Time
}

// AuditStore defines the interface for audit log persistence
type AuditStore interface {
	// LogEntry persists an audit log entry
	LogEntry(ctx context.Context, entry *AuditLogEntry) error
	// GetEntries retrieves audit log entries with filtering
	GetEntries(ctx context.Context, filters map[string]interface{}, limit, offset int) ([]*AuditLogEntry, error)
	// GetEntriesByUser retrieves all entries for a specific user
	GetEntriesByUser(ctx context.Context, userID uuid.UUID, limit, offset int) ([]*AuditLogEntry, error)
	// GetEntriesByAction retrieves all entries for a specific action
	GetEntriesByAction(ctx context.Context, action string, limit, offset int) ([]*AuditLogEntry, error)
	// Count returns the total count of audit entries
	Count(ctx context.Context, filters map[string]interface{}) (int64, error)
}

// AuditLogger provides audit logging capabilities
type AuditLogger struct {
	store  AuditStore
	logger interface{} // zap.Logger
}

// NewAuditLogger creates a new audit logger
func NewAuditLogger(store AuditStore) *AuditLogger {
	return &AuditLogger{
		store: store,
	}
}

// LogAction creates and persists an audit log entry
func (al *AuditLogger) LogAction(ctx context.Context, action, resource string, detail map[string]interface{}) error {
	// Extract user ID from context if available
	var userID *uuid.UUID
	if claims := GetClaimsFromContext(ctx); claims != nil {
		userID = &claims.UserID
	}

	// Callers that have an *http.Request pass IP/UserAgent in the detail map.
	var ipAddress, userAgent string

	entry := &AuditLogEntry{
		ID:        uuid.New(),
		UserID:    userID,
		Action:    action,
		Resource:  resource,
		Detail:    detail,
		IPAddress: ipAddress,
		UserAgent: userAgent,
		Timestamp: time.Now(),
	}

	return al.store.LogEntry(ctx, entry)
}

// LogLogin logs a login attempt
func (al *AuditLogger) LogLogin(ctx context.Context, userID uuid.UUID, success bool, ipAddress string) error {
	action := ActionLogin
	if !success {
		action = "login_failed"
	}

	return al.LogAction(ctx, action, "user", map[string]interface{}{
		"user_id": userID.String(),
		"ip":      ipAddress,
		"success": success,
	})
}

// LogLogout logs a logout
func (al *AuditLogger) LogLogout(ctx context.Context, userID uuid.UUID) error {
	return al.LogAction(ctx, ActionLogout, "session", map[string]interface{}{
		"user_id": userID.String(),
	})
}

// LogTokenRefresh logs a token refresh
func (al *AuditLogger) LogTokenRefresh(ctx context.Context, userID uuid.UUID) error {
	return al.LogAction(ctx, ActionTokenRefresh, "session", map[string]interface{}{
		"user_id": userID.String(),
	})
}

// LogUserCreated logs user creation
func (al *AuditLogger) LogUserCreated(ctx context.Context, userID, creatorID uuid.UUID, email string) error {
	return al.LogAction(ctx, ActionUserCreate, "user", map[string]interface{}{
		"user_id": userID.String(),
		"email":   email,
		"created_by": creatorID.String(),
	})
}

// LogUserUpdated logs user updates
func (al *AuditLogger) LogUserUpdated(ctx context.Context, userID uuid.UUID, changes map[string]interface{}) error {
	return al.LogAction(ctx, ActionUserUpdate, "user", map[string]interface{}{
		"user_id": userID.String(),
		"changes": changes,
	})
}

// LogUserDeleted logs user deletion
func (al *AuditLogger) LogUserDeleted(ctx context.Context, userID uuid.UUID) error {
	return al.LogAction(ctx, ActionUserDelete, "user", map[string]interface{}{
		"user_id": userID.String(),
	})
}

// LogPasswordReset logs password reset
func (al *AuditLogger) LogPasswordReset(ctx context.Context, userID uuid.UUID) error {
	return al.LogAction(ctx, ActionPasswordReset, "user", map[string]interface{}{
		"user_id": userID.String(),
	})
}

// LogPermissionDenied logs denied permission access
func (al *AuditLogger) LogPermissionDenied(ctx context.Context, userID uuid.UUID, action, resource string) error {
	return al.LogAction(ctx, ActionPermissionDenied, resource, map[string]interface{}{
		"user_id": userID.String(),
		"action": action,
	})
}

// GetEntries retrieves audit entries with pagination
func (al *AuditLogger) GetEntries(ctx context.Context, filters map[string]interface{}, limit, offset int) ([]*AuditLogEntry, error) {
	if limit == 0 {
		limit = 50
	}
	if limit > 1000 {
		limit = 1000 // Max 1000 entries per request
	}

	return al.store.GetEntries(ctx, filters, limit, offset)
}

// Count returns the total number of audit entries matching filters
func (al *AuditLogger) Count(ctx context.Context, filters map[string]interface{}) (int64, error) {
	return al.store.Count(ctx, filters)
}

// ExportCSV exports audit logs in CSV format
func (al *AuditLogger) ExportCSV(ctx context.Context, filters map[string]interface{}) (string, error) {
	// Get all entries matching filters
	entries, err := al.store.GetEntries(ctx, filters, 10000, 0)
	if err != nil {
		return "", fmt.Errorf("failed to retrieve entries: %w", err)
	}

	// Build CSV
	csv := "ID,UserID,Action,Resource,Timestamp,IPAddress,Detail\n"
	for _, entry := range entries {
		userID := ""
		if entry.UserID != nil {
			userID = entry.UserID.String()
		}

		detailJSON, _ := json.Marshal(entry.Detail)

		csv += fmt.Sprintf("%s,%s,%s,%s,%s,%s,%s\n",
			entry.ID.String(),
			userID,
			entry.Action,
			entry.Resource,
			entry.Timestamp.Format(time.RFC3339),
			entry.IPAddress,
			string(detailJSON),
		)
	}

	return csv, nil
}

// getClientIP extracts the client IP from the request (without port)
func getClientIP(r *http.Request) string {
	// Check X-Forwarded-For header first (for proxies)
	// Format can be "IP" or "IP:port" or "IP, IP, IP" (comma-separated)
	if xff := r.Header.Get("X-Forwarded-For"); xff != "" {
		// Take first IP from list
		if idx := strings.Index(xff, ","); idx != -1 {
			xff = xff[:idx]
		}
		// Strip port if present
		if ip, _, err := net.SplitHostPort(xff); err == nil {
			return ip
		}
		return strings.TrimSpace(xff)
	}

	// Check X-Real-IP header
	if xri := r.Header.Get("X-Real-IP"); xri != "" {
		// Strip port if present
		if ip, _, err := net.SplitHostPort(xri); err == nil {
			return ip
		}
		return strings.TrimSpace(xri)
	}

	// Fall back to RemoteAddr (format: "IP:port")
	ip, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		// If no port, return as-is
		return r.RemoteAddr
	}

	return ip
}
