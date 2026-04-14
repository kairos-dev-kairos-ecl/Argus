package ingest

import (
	"encoding/json"
	"net/http"
)

// Rules handlers

func (h *QueryHandler) handleListRules(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusNotImplemented)
	json.NewEncoder(w).Encode(ErrorResponse{Error: "not implemented"})
}

func (h *QueryHandler) handleCreateRule(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusNotImplemented)
	json.NewEncoder(w).Encode(ErrorResponse{Error: "not implemented"})
}

func (h *QueryHandler) handleGetRule(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusNotImplemented)
	json.NewEncoder(w).Encode(ErrorResponse{Error: "not implemented"})
}

func (h *QueryHandler) handleUpdateRule(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusNotImplemented)
	json.NewEncoder(w).Encode(ErrorResponse{Error: "not implemented"})
}

func (h *QueryHandler) handleDeleteRule(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusNotImplemented)
	json.NewEncoder(w).Encode(ErrorResponse{Error: "not implemented"})
}

func (h *QueryHandler) handleValidateRule(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusNotImplemented)
	json.NewEncoder(w).Encode(ErrorResponse{Error: "not implemented"})
}

func (h *QueryHandler) handleTestRule(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusNotImplemented)
	json.NewEncoder(w).Encode(ErrorResponse{Error: "not implemented"})
}

// Auth handlers

func (h *QueryHandler) handleLogin(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusNotImplemented)
	json.NewEncoder(w).Encode(ErrorResponse{Error: "not implemented"})
}

func (h *QueryHandler) handleRefreshToken(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusNotImplemented)
	json.NewEncoder(w).Encode(ErrorResponse{Error: "not implemented"})
}

func (h *QueryHandler) handleLogout(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusNotImplemented)
	json.NewEncoder(w).Encode(ErrorResponse{Error: "not implemented"})
}

func (h *QueryHandler) handleSetup(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusNotImplemented)
	json.NewEncoder(w).Encode(ErrorResponse{Error: "not implemented"})
}

// Users handlers

func (h *QueryHandler) handleListUsers(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusNotImplemented)
	json.NewEncoder(w).Encode(ErrorResponse{Error: "not implemented"})
}

func (h *QueryHandler) handleCreateUser(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusNotImplemented)
	json.NewEncoder(w).Encode(ErrorResponse{Error: "not implemented"})
}

// Apps handlers

func (h *QueryHandler) handleListApps(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusNotImplemented)
	json.NewEncoder(w).Encode(ErrorResponse{Error: "not implemented"})
}

func (h *QueryHandler) handleCreateApp(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusNotImplemented)
	json.NewEncoder(w).Encode(ErrorResponse{Error: "not implemented"})
}

func (h *QueryHandler) handleGetAppKey(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusNotImplemented)
	json.NewEncoder(w).Encode(ErrorResponse{Error: "not implemented"})
}

func (h *QueryHandler) handleRotateAppKey(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusNotImplemented)
	json.NewEncoder(w).Encode(ErrorResponse{Error: "not implemented"})
}

// Audit handler

func (h *QueryHandler) handleListAuditLog(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusNotImplemented)
	json.NewEncoder(w).Encode(ErrorResponse{Error: "not implemented"})
}
