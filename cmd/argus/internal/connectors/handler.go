package connectors

import (
	"encoding/json"
	"net/http"

	"go.uber.org/zap"
)

// ConnectorHandler handles HTTP requests for connector operations
type ConnectorHandler struct {
	logger *zap.Logger
	connectors map[string]*LocalModelConnector
}

// NewConnectorHandler creates a new connector handler
func NewConnectorHandler(logger *zap.Logger) *ConnectorHandler {
	return &ConnectorHandler{
		logger:     logger,
		connectors: make(map[string]*LocalModelConnector),
	}
}

// HandleDiscoverModels lists available models from a local server
func (h *ConnectorHandler) HandleDiscoverModels(w http.ResponseWriter, r *http.Request) {
	var req struct {
		ServerURL string `json:"server_url"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid request body", http.StatusBadRequest)
		return
	}

	if req.ServerURL == "" {
		http.Error(w, "server_url is required", http.StatusBadRequest)
		return
	}

	// Create temporary connector for discovery
	connector := NewLocalModelConnector(req.ServerURL, "", h.logger)
	models, err := connector.DiscoverModels(r.Context())
	if err != nil {
		h.logger.Error("discovery failed", zap.Error(err), zap.String("server", req.ServerURL))
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusServiceUnavailable)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"error": err.Error(),
		})
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"models": models,
	})
}

// HandleTestConnection tests connectivity to a local model server
func (h *ConnectorHandler) HandleTestConnection(w http.ResponseWriter, r *http.Request) {
	var req struct {
		ServerURL string `json:"server_url"`
		ModelName string `json:"model_name"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid request body", http.StatusBadRequest)
		return
	}

	if req.ServerURL == "" {
		http.Error(w, "server_url is required", http.StatusBadRequest)
		return
	}

	connector := NewLocalModelConnector(req.ServerURL, req.ModelName, h.logger)
	if err := connector.HealthCheck(r.Context()); err != nil {
		h.logger.Error("health check failed", zap.Error(err), zap.String("server", req.ServerURL))
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusServiceUnavailable)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"error": err.Error(),
		})
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{
		"message": "Connection test passed",
	})
}

// HandleRegisterConnector registers a local model connector
func (h *ConnectorHandler) HandleRegisterConnector(w http.ResponseWriter, r *http.Request) {
	var req struct {
		AppID     string `json:"app_id"`
		ServerURL string `json:"server_url"`
		ModelName string `json:"model_name"`
		SignalDepth string `json:"signal_depth"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid request body", http.StatusBadRequest)
		return
	}

	if req.AppID == "" || req.ServerURL == "" || req.ModelName == "" {
		http.Error(w, "app_id, server_url, and model_name are required", http.StatusBadRequest)
		return
	}

	connector := NewLocalModelConnector(req.ServerURL, req.ModelName, h.logger)

	// Verify model exists
	if err := connector.HealthCheck(r.Context()); err != nil {
		h.logger.Error("connector health check failed", zap.Error(err))
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusServiceUnavailable)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"error": "Unable to connect to model server",
		})
		return
	}

	// Register connector
	h.connectors[req.AppID] = connector

	// Start periodic health checks
	go connector.StartHealthCheckLoop(r.Context(), 60 * 60) // Every hour

	h.logger.Info("connector registered",
		zap.String("app_id", req.AppID),
		zap.String("server", req.ServerURL),
		zap.String("model", req.ModelName),
	)

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{
		"message": "Connector registered successfully",
	})
}

// HandleGetConnectorStatus returns the health status of a connector
func (h *ConnectorHandler) HandleGetConnectorStatus(w http.ResponseWriter, r *http.Request) {
	appID := r.URL.Query().Get("app_id")
	if appID == "" {
		http.Error(w, "app_id query parameter required", http.StatusBadRequest)
		return
	}

	connector, ok := h.connectors[appID]
	if !ok {
		http.Error(w, "connector not found", http.StatusNotFound)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"app_id":  appID,
		"healthy": connector.IsHealthy(),
		"model":   connector.modelName,
		"server":  connector.serverURL,
	})
}
