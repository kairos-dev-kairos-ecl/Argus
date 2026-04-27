package ingest

import (
	"encoding/json"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/argusxdr/argus/internal/auth"
)

// auditEntryDTO is the JSON response shape the frontend AuditEntry interface expects.
// Backend AuditLogEntry has PascalCase fields with no json tags; this DTO provides
// snake_case serialization and fills in derived fields (hash, resource_type, etc.).
type auditEntryDTO struct {
	ID           string                 `json:"id"`
	Timestamp    string                 `json:"timestamp"`
	Action       string                 `json:"action"`
	ActorID      *string                `json:"actor_id"`
	ActorEmail   *string                `json:"actor_email"`
	ResourceType string                 `json:"resource_type"`
	ResourceID   string                 `json:"resource_id"`
	IPAddress    string                 `json:"ip_address"`
	Hash         string                 `json:"hash"`
	Metadata     map[string]interface{} `json:"metadata"`
}

// toAuditDTO converts an internal AuditLogEntry to the frontend DTO shape.
func toAuditDTO(e *auth.AuditLogEntry) auditEntryDTO {
	// actor_id
	var actorID *string
	if e.UserID != nil {
		s := e.UserID.String()
		actorID = &s
	}

	// resource_type / resource_id: Resource field contains e.g. "user", "session"
	// If the Detail map has a resource_id key, use it; otherwise leave empty.
	resourceType := e.Resource
	resourceID := ""
	if e.Detail != nil {
		if v, ok := e.Detail["resource_id"]; ok {
			if s, ok := v.(string); ok {
				resourceID = s
			}
		}
	}

	// hash: use first 12 chars of entry ID (UUID without dashes)
	rawID := strings.ReplaceAll(e.ID.String(), "-", "")
	hash := rawID
	if len(rawID) > 12 {
		hash = rawID[:12]
	}

	// metadata: surface Detail map (never nil for JSON)
	metadata := e.Detail
	if metadata == nil {
		metadata = map[string]interface{}{}
	}

	return auditEntryDTO{
		ID:           e.ID.String(),
		Timestamp:    e.Timestamp.UTC().Format(time.RFC3339),
		Action:       e.Action,
		ActorID:      actorID,
		ActorEmail:   nil, // not stored in audit_log without a join
		ResourceType: resourceType,
		ResourceID:   resourceID,
		IPAddress:    e.IPAddress,
		Hash:         hash,
		Metadata:     metadata,
	}
}

func (h *QueryHandler) handleListAuditLog(w http.ResponseWriter, r *http.Request) {
	if !h.authAvailable() || h.authService.AuditLog == nil {
		jsonError(w, "audit log unavailable", http.StatusServiceUnavailable)
		return
	}

	limit := 50
	offset := 0
	if s := r.URL.Query().Get("limit"); s != "" {
		if n, err := strconv.Atoi(s); err == nil && n > 0 && n <= 1000 {
			limit = n
		}
	}
	if s := r.URL.Query().Get("offset"); s != "" {
		if n, err := strconv.Atoi(s); err == nil && n >= 0 {
			offset = n
		}
	}

	entries, err := h.authService.AuditLog.GetEntries(r.Context(), nil, limit, offset)
	if err != nil {
		jsonError(w, "internal error", http.StatusInternalServerError)
		return
	}

	// Convert to frontend-compatible DTOs
	dtos := make([]auditEntryDTO, 0, len(entries))
	for _, e := range entries {
		dtos = append(dtos, toAuditDTO(e))
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"entries": dtos,
		"limit":   limit,
		"offset":  offset,
	})
}
