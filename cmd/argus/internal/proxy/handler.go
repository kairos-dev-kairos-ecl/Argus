package proxy

import (
	"encoding/json"
	"io"
	"net/http"

	"go.uber.org/zap"
)

// Handler handles HTTP requests for proxy operations
type Handler struct {
	service *Service
	logger  *zap.Logger
}

// NewHandler creates a new proxy handler
func NewHandler(service *Service, logger *zap.Logger) *Handler {
	return &Handler{
		service: service,
		logger:  logger,
	}
}

// HandleProxy is the main proxy handler - forwards requests and extracts signals
func (h *Handler) HandleProxy(w http.ResponseWriter, r *http.Request) {
	var config ProxyConfig
	if err := json.NewDecoder(r.Body).Decode(&config); err != nil {
		h.logger.Error("invalid request body", zap.Error(err))
		http.Error(w, "invalid request body", http.StatusBadRequest)
		return
	}

	// Forward the request
	resp, signals, err := h.service.ProxyRequest(r.Context(), config, r)
	if err != nil {
		h.logger.Error("proxy request failed", zap.Error(err))
		http.Error(w, err.Error(), http.StatusBadGateway)
		return
	}
	defer resp.Body.Close()

	// Write response status and headers
	w.WriteHeader(resp.StatusCode)
	for key, values := range resp.Header {
		for _, value := range values {
			w.Header().Add(key, value)
		}
	}

	// Write response body
	if _, err := io.Copy(w, resp.Body); err != nil {
		h.logger.Error("failed to write response", zap.Error(err))
	}

	// Log signals for ingestion
	h.logger.Info("proxy request completed with signals",
		zap.String("method", r.Method),
		zap.String("path", r.RequestURI),
		zap.Any("signals", signals),
	)
}

// HandleTestConnection tests connectivity to upstream API
func (h *Handler) HandleTestConnection(w http.ResponseWriter, r *http.Request) {
	var req struct {
		UpstreamURL string `json:"upstream_url"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid request body", http.StatusBadRequest)
		return
	}

	if err := h.service.TestConnection(r.Context(), req.UpstreamURL); err != nil {
		h.logger.Error("connection test failed", zap.Error(err))
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

// HandleValidateConfig validates a proxy configuration
func (h *Handler) HandleValidateConfig(w http.ResponseWriter, r *http.Request) {
	var config ProxyConfig
	if err := json.NewDecoder(r.Body).Decode(&config); err != nil {
		http.Error(w, "invalid request body", http.StatusBadRequest)
		return
	}

	// Basic validation
	if config.UpstreamURL == "" {
		http.Error(w, "upstream_url is required", http.StatusBadRequest)
		return
	}

	if config.SignalDepth == "" {
		config.SignalDepth = "headers-metadata"
	}

	if config.LatencyBudgetMs == 0 {
		config.LatencyBudgetMs = 100
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"valid":  true,
		"config": config,
	})
}
