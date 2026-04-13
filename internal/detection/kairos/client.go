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

// Client represents an HTTP client for Kairos policy evaluation.
type Client struct {
	endpoint string
	client   *http.Client
	timeout  time.Duration
	logger   *zap.Logger
}

// PolicyRequest is the request structure sent to Kairos.
type PolicyRequest struct {
	SignalID   string                 `json:"signal_id"`
	TraceID    string                 `json:"trace_id"`
	Layer      string                 `json:"layer"`
	Category   string                 `json:"category"`
	RuleID     string                 `json:"rule_id"`
	RuleName   string                 `json:"rule_name"`
	Confidence float64                `json:"confidence"`
	Data       map[string]interface{} `json:"data"`
	Timestamp  int64                  `json:"timestamp"`
}

// PolicyResponse is the response structure from Kairos.
type PolicyResponse struct {
	Decision             string  `json:"decision"`              // allow, deny, review
	Confidence           float64 `json:"confidence"`            // 0-1
	Reasoning            string  `json:"reasoning"`             // explanation
	RecommendedAction    string  `json:"recommended_action"`    // suppress, escalate, investigate
	KairosPolicyVersion  string  `json:"kairos_policy_version"`
	PolicyRuleTriggered  string  `json:"policy_rule_triggered,omitempty"`
	ProcessingTimeMs     int64   `json:"processing_time_ms"`
	Error                string  `json:"error,omitempty"`
}

// NewClient creates a new Kairos policy evaluation client.
func NewClient(endpoint string, timeout time.Duration, logger *zap.Logger) *Client {
	return &Client{
		endpoint: endpoint,
		client: &http.Client{
			Timeout: timeout,
		},
		timeout: timeout,
		logger:  logger,
	}
}

// EvaluatePolicy sends a policy evaluation request to Kairos.
// Returns a PolicyResponse or an error.
func (c *Client) EvaluatePolicy(ctx context.Context, req *PolicyRequest) (*PolicyResponse, error) {
	if c.endpoint == "" {
		return nil, fmt.Errorf("kairos endpoint not configured")
	}

	// Create a new context with timeout if not already set
	if _, ok := ctx.Deadline(); !ok {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, c.timeout)
		defer cancel()
	}

	// Marshal request to JSON
	body, err := json.Marshal(req)
	if err != nil {
		c.logger.Error("failed to marshal policy request", zap.Error(err))
		return nil, fmt.Errorf("marshal policy request: %w", err)
	}

	// Create HTTP request
	httpReq, err := http.NewRequestWithContext(ctx, "POST", c.endpoint+"/policy/evaluate", bytes.NewReader(body))
	if err != nil {
		c.logger.Error("failed to create http request", zap.Error(err))
		return nil, fmt.Errorf("create http request: %w", err)
	}

	httpReq.Header.Set("Content-Type", "application/json")

	// Execute request
	startTime := time.Now()
	resp, err := c.client.Do(httpReq)
	latency := time.Since(startTime)

	if err != nil {
		c.logger.Warn("kairos policy evaluation failed",
			zap.String("signal_id", req.SignalID),
			zap.Duration("latency", latency),
			zap.Error(err),
		)
		return nil, fmt.Errorf("kairos request failed: %w", err)
	}
	defer resp.Body.Close()

	// Read response body
	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		c.logger.Error("failed to read kairos response body", zap.Error(err))
		return nil, fmt.Errorf("read response body: %w", err)
	}

	// Check HTTP status
	if resp.StatusCode != http.StatusOK {
		c.logger.Warn("kairos returned non-200 status",
			zap.Int("status_code", resp.StatusCode),
			zap.String("response", string(respBody)),
		)
		return nil, fmt.Errorf("kairos returned status %d", resp.StatusCode)
	}

	// Unmarshal response
	var policyResp PolicyResponse
	if err := json.Unmarshal(respBody, &policyResp); err != nil {
		c.logger.Error("failed to unmarshal kairos response", zap.Error(err))
		return nil, fmt.Errorf("unmarshal policy response: %w", err)
	}

	// Log successful evaluation
	c.logger.Debug("kairos policy evaluation succeeded",
		zap.String("signal_id", req.SignalID),
		zap.String("decision", policyResp.Decision),
		zap.Float64("confidence", policyResp.Confidence),
		zap.Duration("latency", latency),
	)

	return &policyResp, nil
}

// Health checks if the Kairos endpoint is reachable.
func (c *Client) Health(ctx context.Context) error {
	if c.endpoint == "" {
		return fmt.Errorf("kairos endpoint not configured")
	}

	// Create a new context with timeout if not already set
	if _, ok := ctx.Deadline(); !ok {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, c.timeout)
		defer cancel()
	}

	httpReq, err := http.NewRequestWithContext(ctx, "GET", c.endpoint+"/health", nil)
	if err != nil {
		return fmt.Errorf("create health check request: %w", err)
	}

	resp, err := c.client.Do(httpReq)
	if err != nil {
		return fmt.Errorf("health check failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("health check returned status %d", resp.StatusCode)
	}

	return nil
}
