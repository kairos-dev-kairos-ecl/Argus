package adapters

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"math"
	"math/rand"
	"net/http"
	"time"

	"github.com/argusxdr/argus/internal/notify"
	"go.uber.org/zap"
)

// WebhookConfig contains the configuration for the Webhook notifier.
type WebhookConfig struct {
	URL     string            // Target URL for webhook POST
	Auth    string            // Auth type: "none", "apikey", "oauth"
	Headers map[string]string // Additional HTTP headers to include
	Timeout time.Duration     // HTTP request timeout (default: 5 seconds)
}

// WebhookNotifier sends notifications to a custom webhook endpoint.
type WebhookNotifier struct {
	config WebhookConfig
	logger *zap.Logger
	client *http.Client
}

// webhookPayload is the JSON payload sent to the webhook.
type webhookPayload struct {
	AlertID     string    `json:"alert_id"`
	RuleID      string    `json:"rule_id"`
	Severity    int       `json:"severity"`
	SignalIDs   []string  `json:"signal_ids"`
	Confidence  string    `json:"confidence"`
	Fingerprint string    `json:"fingerprint"`
	Timestamp   time.Time `json:"timestamp"`
	Title       string    `json:"title"`
	Message     string    `json:"message"`
}

// NewWebhookNotifier creates a new Webhook notifier with the given configuration.
func NewWebhookNotifier(config WebhookConfig, logger *zap.Logger) (*WebhookNotifier, error) {
	if config.URL == "" {
		return nil, fmt.Errorf("webhook URL is required")
	}
	if logger == nil {
		logger = zap.NewNop()
	}

	// Default timeout to 5 seconds if not specified
	if config.Timeout == 0 {
		config.Timeout = 5 * time.Second
	}

	return &WebhookNotifier{
		config: config,
		logger: logger,
		client: &http.Client{
			Timeout: config.Timeout,
		},
	}, nil
}

// Name returns the name of this notifier.
func (w *WebhookNotifier) Name() string {
	return "webhook"
}

// parseSignalIDs splits the comma-separated signal IDs string into a slice.
func parseSignalIDs(signalIDsStr string) []string {
	if signalIDsStr == "" {
		return []string{}
	}
	// Simple split on comma (metadata should have proper comma-separated format)
	parts := bytes.Split([]byte(signalIDsStr), []byte(","))
	result := make([]string, len(parts))
	for i, part := range parts {
		result[i] = string(bytes.TrimSpace(part))
	}
	return result
}

// exponentialBackoff calculates exponential backoff with jitter.
// attempt: 0-based attempt number
// Returns: duration to wait before next retry
func exponentialBackoff(attempt int) time.Duration {
	baseDuration := time.Second
	maxDuration := 30 * time.Second

	// Calculate exponential backoff: 1s, 2s, 4s, 8s, 16s, 30s (capped)
	duration := time.Duration(math.Pow(2, float64(attempt))) * baseDuration
	if duration > maxDuration {
		duration = maxDuration
	}

	// Add jitter: random value between 0 and duration
	jitter := time.Duration(rand.Intn(int(duration)))
	return duration + jitter
}

// Send sends a JSON payload to the webhook URL with retries on 5xx errors.
func (w *WebhookNotifier) Send(ctx context.Context, req *notify.NotificationRequest) (*notify.NotificationResponse, error) {
	if req == nil {
		return nil, fmt.Errorf("notification request cannot be nil")
	}

	// Parse signal IDs from metadata
	signalIDsStr := req.Metadata["signal_ids"]
	var signalIDs []string
	if signalIDsStr != "" {
		// Parse the signal IDs field manually
		for _, id := range parseSignalIDs(signalIDsStr) {
			signalIDs = append(signalIDs, string(id))
		}
	}

	// Construct payload
	payload := webhookPayload{
		AlertID:     req.AlertID.String(),
		RuleID:      req.RuleID.String(),
		Severity:    req.Severity,
		SignalIDs:   signalIDs,
		Confidence:  req.Metadata["confidence"],
		Fingerprint: req.Metadata["fingerprint"],
		Timestamp:   time.Now(),
		Title:       req.Title,
		Message:     req.Message,
	}

	jsonData, err := json.Marshal(payload)
	if err != nil {
		w.logger.Error("failed to marshal webhook payload", zap.Error(err))
		return nil, fmt.Errorf("json marshal error: %w", err)
	}

	// Retry logic with exponential backoff
	const maxRetries = 3
	var lastErr error

	for attempt := 0; attempt <= maxRetries; attempt++ {
		// Check context before each attempt
		select {
		case <-ctx.Done():
			return nil, fmt.Errorf("webhook send cancelled: %w", ctx.Err())
		default:
		}

		resp, err := w.sendOnce(ctx, jsonData)
		if err == nil {
			return resp, nil
		}

		lastErr = err

		// Don't retry on 4xx errors (client errors are not retryable)
		if isClientError(err) {
			w.logger.Warn("webhook client error (not retrying)", zap.Error(err))
			return &notify.NotificationResponse{
				Status:    "failed",
				Error:     err.Error(),
				Timestamp: time.Now().Unix(),
			}, err
		}

		// Only retry on 5xx errors or network errors
		if attempt < maxRetries {
			backoffDuration := exponentialBackoff(attempt)
			w.logger.Warn("webhook send failed, retrying",
				zap.Error(err),
				zap.Int("attempt", attempt+1),
				zap.Duration("backoff", backoffDuration),
			)

			select {
			case <-time.After(backoffDuration):
				// Continue to next retry
			case <-ctx.Done():
				return nil, fmt.Errorf("webhook retry cancelled: %w", ctx.Err())
			}
		}
	}

	w.logger.Error("webhook send failed after retries", zap.Error(lastErr))
	return &notify.NotificationResponse{
		Status:    "failed",
		Error:     lastErr.Error(),
		Timestamp: time.Now().Unix(),
	}, lastErr
}

// sendOnce performs a single webhook POST request.
func (w *WebhookNotifier) sendOnce(ctx context.Context, jsonData []byte) (*notify.NotificationResponse, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, w.config.URL, bytes.NewBuffer(jsonData))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	// Set content type
	req.Header.Set("Content-Type", "application/json")

	// Add custom headers
	for key, value := range w.config.Headers {
		req.Header.Set(key, value)
	}

	// Perform request
	resp, err := w.client.Do(req)
	if err != nil {
		w.logger.Warn("webhook request failed", zap.Error(err), zap.String("url", w.config.URL))
		return nil, fmt.Errorf("webhook request failed: %w", err)
	}
	defer resp.Body.Close()

	// Read response body for logging
	body, _ := io.ReadAll(resp.Body)

	// Check status code
	if resp.StatusCode >= 500 {
		// 5xx errors are retryable
		w.logger.Warn("webhook returned 5xx error",
			zap.Int("status_code", resp.StatusCode),
			zap.String("body", string(body)),
		)
		return nil, fmt.Errorf("webhook server error: %d", resp.StatusCode)
	}

	if resp.StatusCode >= 400 {
		// 4xx errors are not retryable
		w.logger.Error("webhook returned 4xx error",
			zap.Int("status_code", resp.StatusCode),
			zap.String("body", string(body)),
		)
		return nil, fmt.Errorf("webhook client error: %d", resp.StatusCode)
	}

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		// Other non-2xx responses are treated as errors
		w.logger.Error("webhook returned non-2xx status",
			zap.Int("status_code", resp.StatusCode),
			zap.String("body", string(body)),
		)
		return nil, fmt.Errorf("webhook unexpected status: %d", resp.StatusCode)
	}

	w.logger.Info("webhook sent successfully",
		zap.Int("status_code", resp.StatusCode),
		zap.String("url", w.config.URL),
	)

	return &notify.NotificationResponse{
		Status:    "sent",
		MessageID: fmt.Sprintf("webhook-%d", time.Now().Unix()),
		Timestamp: time.Now().Unix(),
	}, nil
}

// isClientError checks if the error is a 4xx client error.
func isClientError(err error) bool {
	if err == nil {
		return false
	}
	errStr := err.Error()
	// Check for 4xx status codes in error message
	return (errStr != "" && (bytes.Contains([]byte(errStr), []byte("400")) ||
		bytes.Contains([]byte(errStr), []byte("401")) ||
		bytes.Contains([]byte(errStr), []byte("403")) ||
		bytes.Contains([]byte(errStr), []byte("404")) ||
		bytes.Contains([]byte(errStr), []byte("4xx"))))
}

// Health checks if the webhook endpoint is accessible.
func (w *WebhookNotifier) Health(ctx context.Context) error {
	// Send a GET request to check if the endpoint is reachable
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, w.config.URL, nil)
	if err != nil {
		w.logger.Error("webhook health check failed", zap.Error(err))
		return fmt.Errorf("webhook health check failed: %w", err)
	}

	resp, err := w.client.Do(req)
	if err != nil {
		w.logger.Error("webhook health check failed", zap.Error(err))
		return fmt.Errorf("webhook health check failed: %w", err)
	}
	defer resp.Body.Close()

	// Accept 2xx, 4xx, 5xx responses (endpoint exists), but fail on connection errors
	return nil
}
