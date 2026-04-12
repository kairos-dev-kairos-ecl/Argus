package proxy

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"go.uber.org/zap"
)

// ProxyConfig represents the configuration for the API proxy
type ProxyConfig struct {
	UpstreamURL    string // e.g., https://api.openai.com/v1
	AuthPassthrough bool   // Pass through Authorization header
	SignalDepth     string // "headers", "headers-metadata", "full"
	LatencyBudgetMs int    // Maximum acceptable latency overhead
}

// Service handles transparent HTTP forwarding and signal extraction
type Service struct {
	client     *http.Client
	logger     *zap.Logger
	timeout    time.Duration
}

// NewService creates a new proxy service
func NewService(logger *zap.Logger, timeoutMs int) *Service {
	return &Service{
		client: &http.Client{
			Timeout: time.Duration(timeoutMs) * time.Millisecond,
		},
		logger:  logger,
		timeout: time.Duration(timeoutMs) * time.Millisecond,
	}
}

// ProxyRequest forwards an HTTP request to the upstream API and extracts signals
func (s *Service) ProxyRequest(
	ctx context.Context,
	config ProxyConfig,
	req *http.Request,
) (*http.Response, map[string]interface{}, error) {
	start := time.Now()

	// Clone the request for forwarding
	proxyReq, err := http.NewRequestWithContext(ctx, req.Method, config.UpstreamURL+req.RequestURI, req.Body)
	if err != nil {
		s.logger.Error("failed to create proxy request", zap.Error(err))
		return nil, nil, fmt.Errorf("failed to create proxy request: %w", err)
	}

	// Copy headers from original request
	proxyReq.Header = req.Header.Clone()

	// Handle auth passthrough
	if !config.AuthPassthrough && req.Header.Get("Authorization") != "" {
		proxyReq.Header.Del("Authorization")
	}

	// Forward the request
	resp, err := s.client.Do(proxyReq)
	if err != nil {
		s.logger.Error("proxy request failed", zap.Error(err), zap.String("upstream", config.UpstreamURL))
		return nil, nil, fmt.Errorf("proxy request failed: %w", err)
	}

	// Extract signals from response
	signals := s.extractSignals(config, resp)

	// Log latency
	latency := time.Since(start).Milliseconds()
	if latency > int64(config.LatencyBudgetMs) {
		s.logger.Warn("proxy latency exceeded budget",
			zap.Int64("latency_ms", latency),
			zap.Int("budget_ms", config.LatencyBudgetMs),
		)
	}

	s.logger.Debug("proxy request completed",
		zap.String("upstream", config.UpstreamURL),
		zap.Int("status", resp.StatusCode),
		zap.Int64("latency_ms", latency),
	)

	return resp, signals, nil
}

// extractSignals parses the response to extract LLM signal data
func (s *Service) extractSignals(config ProxyConfig, resp *http.Response) map[string]interface{} {
	signals := make(map[string]interface{})

	// Always extract HTTP metadata
	signals["status_code"] = resp.StatusCode
	signals["headers"] = resp.Header.Clone()

	// Extract from response body based on signal depth
	if config.SignalDepth == "headers" {
		return signals
	}

	// Read and parse response body
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		s.logger.Warn("failed to read response body", zap.Error(err))
		return signals
	}

	// Restore body for consumer
	resp.Body = io.NopCloser(bytes.NewReader(body))

	var respData map[string]interface{}
	if err := json.Unmarshal(body, &respData); err != nil {
		// Not JSON, skip parsing
		return signals
	}

	// Extract token counts (OpenAI style)
	if usage, ok := respData["usage"].(map[string]interface{}); ok {
		signals["tokens_input"] = usage["prompt_tokens"]
		signals["tokens_output"] = usage["completion_tokens"]
		signals["tokens_total"] = usage["total_tokens"]
	}

	// Extract model info
	if model, ok := respData["model"].(string); ok {
		signals["model"] = model
	}

	// Extract cost estimation
	if cost, ok := respData["cost"].(float64); ok {
		signals["cost"] = cost
	}

	// Full response body if requested
	if config.SignalDepth == "full" {
		signals["response_body"] = respData
	}

	return signals
}

// TestConnection validates that the upstream API is reachable
func (s *Service) TestConnection(ctx context.Context, upstreamURL string) error {
	req, err := http.NewRequestWithContext(ctx, "GET", upstreamURL+"/models", nil)
	if err != nil {
		return fmt.Errorf("failed to create test request: %w", err)
	}

	resp, err := s.client.Do(req)
	if err != nil {
		return fmt.Errorf("connection test failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 500 {
		return fmt.Errorf("upstream server error (status %d)", resp.StatusCode)
	}

	if resp.StatusCode == 401 || resp.StatusCode == 403 {
		return fmt.Errorf("authentication failed (status %d)", resp.StatusCode)
	}

	s.logger.Info("proxy connection test passed", zap.String("upstream", upstreamURL))
	return nil
}

// ExtractOpenAISignals extracts tokens from OpenAI API response
func ExtractOpenAISignals(resp map[string]interface{}) map[string]interface{} {
	signals := make(map[string]interface{})

	if usage, ok := resp["usage"].(map[string]interface{}); ok {
		if promptTokens, ok := usage["prompt_tokens"].(float64); ok {
			signals["tokens_input"] = int(promptTokens)
		}
		if completionTokens, ok := usage["completion_tokens"].(float64); ok {
			signals["tokens_output"] = int(completionTokens)
		}
		if totalTokens, ok := usage["total_tokens"].(float64); ok {
			signals["tokens_total"] = int(totalTokens)
		}
	}

	if model, ok := resp["model"].(string); ok {
		signals["model"] = model
	}

	return signals
}

// ExtractAnthropicSignals extracts tokens from Anthropic API response
func ExtractAnthropicSignals(resp map[string]interface{}) map[string]interface{} {
	signals := make(map[string]interface{})

	if usage, ok := resp["usage"].(map[string]interface{}); ok {
		if inputTokens, ok := usage["input_tokens"].(float64); ok {
			signals["tokens_input"] = int(inputTokens)
		}
		if outputTokens, ok := usage["output_tokens"].(float64); ok {
			signals["tokens_output"] = int(outputTokens)
		}
	}

	if model, ok := resp["model"].(string); ok {
		signals["model"] = model
	}

	return signals
}
