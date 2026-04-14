package ingest

import (
	"encoding/json"
	"net/http"

	"github.com/argusxdr/argus/internal/auth"
	"go.uber.org/zap"
)

type createUserRequest struct {
	Email       string `json:"email"`
	Password    string `json:"password"`
	DisplayName string `json:"display_name"`
	Role        string `json:"role"`
}

func (h *QueryHandler) handleListUsers(w http.ResponseWriter, r *http.Request) {
	if !h.authAvailable() {
		jsonError(w, "auth service unavailable", http.StatusServiceUnavailable)
		return
	}

	users, err := h.authService.UserStore.ListUsers(r.Context(), nil)
	if err != nil {
		h.log.Error("failed to list users", zap.Error(err))
		jsonError(w, "internal error", http.StatusInternalServerError)
		return
	}

	// Sanitize: never return password hashes
	type userResponse struct {
		ID          string `json:"id"`
		Email       string `json:"email"`
		DisplayName string `json:"display_name"`
		Role        string `json:"role"`
		Status      string `json:"status"`
	}
	result := make([]userResponse, 0, len(users))
	for _, u := range users {
		result = append(result, userResponse{
			ID:          u.ID.String(),
			Email:       u.Email,
			DisplayName: u.DisplayName,
			Role:        u.Role,
			Status:      u.Status,
		})
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{"users": result, "total": len(result)})
}

func (h *QueryHandler) handleCreateUser(w http.ResponseWriter, r *http.Request) {
	if !h.authAvailable() {
		jsonError(w, "auth service unavailable", http.StatusServiceUnavailable)
		return
	}

	var req createUserRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		jsonError(w, "invalid request body", http.StatusBadRequest)
		return
	}

	if req.Role == "" {
		req.Role = auth.RoleViewer
	}
	if req.Role != auth.RoleAdmin && req.Role != auth.RoleAnalyst && req.Role != auth.RoleViewer {
		jsonError(w, "invalid role; must be admin, analyst, or viewer", http.StatusBadRequest)
		return
	}

	// Only admin can create users — check caller
	caller := auth.GetUserFromContext(r)
	if caller != nil && caller.Role != auth.RoleAdmin {
		jsonError(w, "forbidden", http.StatusForbidden)
		return
	}

	var createdBy *interface{ String() string }
	_ = createdBy

	user, err := h.authService.UserSvc.CreateUser(r.Context(), req.Email, req.DisplayName, req.Password, req.Role, nil)
	if err != nil {
		jsonError(w, err.Error(), http.StatusBadRequest)
		return
	}

	if h.authService.AuditLog != nil {
		h.authService.AuditLog.LogUserCreated(r.Context(), user.ID, user.ID, user.Email)
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(map[string]interface{}{
		"id":           user.ID.String(),
		"email":        user.Email,
		"display_name": user.DisplayName,
		"role":         user.Role,
		"status":       user.Status,
	})
}
