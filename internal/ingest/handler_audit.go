package ingest

import (
	"encoding/json"
	"net/http"
	"strconv"

	"github.com/argusxdr/argus/internal/auth"
)

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
	if entries == nil {
		entries = []*auth.AuditLogEntry{}
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"entries": entries,
		"limit":   limit,
		"offset":  offset,
	})
}
