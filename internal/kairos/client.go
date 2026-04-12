package kairos

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

// PolicyDecision represents the result of Kairos policy evaluation
type PolicyDecision struct {
	Decision            string  `json:"decision"`             // "allow", "deny", "review"
	Confidence          float64 `json:"confidence"`           // 0.0-1.0
	Reasoning           string  `json:"reasoning"`            // Human-readable explanation
	RecommendedAction   string  `json:"recommended_action"`   // "suppress", "escalate", "investigate"
	PolicyVersion       string  `json:"policy_version"`       // Policy version used for decision
	EvaluationTimeMs    float64 `json:"evaluation_time_ms"`   // How long evaluation took
	PolicyName          string  `json:"policy_name"`          // Which policy was applied
}

// EvaluationRequest is sent to Kairos for policy evaluation
type EvaluationRequest struct {
	TraceID          string                 `json:"trace_id"`
	SignalID         string                 `json:"signal_id"`
	Layer            string                 `json:"layer"`
	Category         string                 `json:"category"`
	Severity         string                 `json:"severity"`
	RiskScore        float64                `json:"risk_score"`
	Context          map[string]interface{} `json:"context"`
	PolicyName       string                 `json:"policy_name"`
}

// Client is the HTTP client for Kairos policy evaluation endpoint
type Client struct {
	endpoint   string
	httpClient *http.Client
	logger     *zap.Logger
	timeout    time.Duration
}

// NewClient creates a new Kairos client
func NewClient(endpoint string, timeout time.Duration, logger *zap.Logger) *Client {
	if logger == nil {
		logger = zap.NewNop()
	}
	if timeout == 0 {
		timeout = 5 * time.Second
	}

	return &Client{
		endpoint: endpoint,
		httpClient: &http.Client{
			Timeout: timeout,
		},
		logger:  logger,
		timeout: timeout,
	}
}

// Evaluate sends a request to Kairos and returns the policy decision
// Returns nil if Kairos is unreachable (fail-open behavior)
func (c *Client) Evaluate(ctx context.Context, req *EvaluationRequest) (*PolicyDecision, error) {
	// Create a context with timeout for the evaluation
	evalCtx, cancel := context.WithTimeout(ctx, c.timeout)
	defer cancel()

	// Marshal request
	body, err := json.Marshal(req)
	if err != nil {
		c.logger.Error("failed to marshal evaluation request", zap.Error(err))
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	// Create HTTP request
	httpReq, err := http.NewRequestWithContext(evalCtx, "POST", c.endpoint, bytes.NewReader(body))
	if err != nil {
		c.logger.Error("failed to create HTTP request", zap.Error(err))
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Accept", "application/json")

	// Execute request with timeout
	start := time.Now()
	resp, err := c.httpClient.Do(httpReq)
	elapsed := time.Since(start)

	// Fail-open: if Kairos is unreachable, log and return nil (not an error)
	if err != nil {
		c.logger.Warn("kairos unreachable (fail-open)",
			zap.String("endpoint", c.endpoint),
			zap.Duration("elapsed", elapsed),
			zap.Error(err))
		return nil, nil // Fail-open: no error, just return nil decision
	}

	defer resp.Body.Close()

	// Read response body
	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		c.logger.Error("failed to read response body", zap.Error(err))
		return nil, fmt.Errorf("failed to read response: %w", err)
	}

	// Check HTTP status
	if resp.StatusCode != http.StatusOK {
		c.logger.Warn("kairos returned error status",
			zap.Int("status_code", resp.StatusCode),
			zap.String("response", string(respBody)))
		// Fail-open: bad Kairos response doesn't block detection
		return nil, nil
	}

	// Unmarshal response
	decision := &PolicyDecision{}
	if err := json.Unmarshal(respBody, decision); err != nil {
		c.logger.Error("failed to unmarshal policy decision", zap.Error(err), zap.String("response", string(respBody)))
		return nil, fmt.Errorf("failed to unmarshal decision: %w", err)
	}

	decision.EvaluationTimeMs = elapsed.Seconds() * 1000

	c.logger.Debug("kairos evaluation succeeded",
		zap.String("decision", decision.Decision),
		zap.Float64("confidence", decision.Confidence),
		zap.Float64("evaluation_time_ms", decision.EvaluationTimeMs))

	return decision, nil
}

// Health checks if Kairos is reachable
func (c *Client) Health(ctx context.Context) error {
	healthCtx, cancel := context.WithTimeout(ctx, c.timeout)
	defer cancel()

	healthURL := fmt.Sprintf("%s/health", c.endpoint)
	req, err := http.NewRequestWithContext(healthCtx, "GET", healthURL, nil)
	if err != nil {
		return fmt.Errorf("failed to create health check request: %w", err)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("kairos health check failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("kairos health check returned status %d", resp.StatusCode)
	}

	return nil
}
