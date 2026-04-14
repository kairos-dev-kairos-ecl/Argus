package kairos

import (
	"context"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"time"

	"go.uber.org/zap"
)

// MockKairosServer is a test server that simulates Kairos responses.
type MockKairosServer struct {
	server     *http.Server
	port       int
	responses  map[string]*PolicyResponse
	CallCount  int
	LastRequest *PolicyRequest
}

// NewMockKairosServer creates a new mock Kairos server on a random port.
func NewMockKairosServer() (*MockKairosServer, error) {
	m := &MockKairosServer{
		responses: make(map[string]*PolicyResponse),
	}

	// Find a free port
	listener, err := net.Listen("tcp", ":0")
	if err != nil {
		return nil, err
	}
	m.port = listener.Addr().(*net.TCPAddr).Port
	listener.Close()

	// Setup routes
	mux := http.NewServeMux()
	mux.HandleFunc("/policy/evaluate", m.handleEvaluate)
	mux.HandleFunc("/health", m.handleHealth)

	m.server = &http.Server{
		Addr:    fmt.Sprintf("localhost:%d", m.port),
		Handler: mux,
	}

	// Start server in background
	go m.server.ListenAndServe()

	// Give server time to start
	time.Sleep(10 * time.Millisecond)

	return m, nil
}

// GetEndpoint returns the server's base URL (without path).
// Client methods append their own paths (e.g. /health, /policy/evaluate).
func (m *MockKairosServer) GetEndpoint() string {
	return fmt.Sprintf("http://localhost:%d", m.port)
}

// SetResponse sets a response for a given rule ID.
func (m *MockKairosServer) SetResponse(ruleID string, resp *PolicyResponse) {
	m.responses[ruleID] = resp
}

// handleEvaluate handles evaluation requests.
func (m *MockKairosServer) handleEvaluate(w http.ResponseWriter, r *http.Request) {
	m.CallCount++

	var req PolicyRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]string{"error": err.Error()})
		return
	}

	m.LastRequest = &req

	// Return configured response or default
	resp := m.responses[req.RuleID]
	if resp == nil {
		// Default response: allow with high confidence
		resp = &PolicyResponse{
			Decision:            "allow",
			Confidence:          0.9,
			Reasoning:           "Default mock response",
			RecommendedAction:   "suppress",
			KairosPolicyVersion: "1.0",
			ProcessingTimeMs:    5,
		}
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(resp)
}

// handleHealth handles health check requests.
func (m *MockKairosServer) handleHealth(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"status": "healthy"})
}

// Close stops the mock server.
func (m *MockKairosServer) Close() error {
	ctx, cancel := context.WithTimeout(context.Background(), 1*time.Second)
	defer cancel()
	return m.server.Shutdown(ctx)
}

// NewTestEvaluator creates an evaluator with mock dependencies for testing.
func NewTestEvaluator(logger *zap.Logger) (*Evaluator, *MockKairosServer, error) {
	// Create mock server
	mockServer, err := NewMockKairosServer()
	if err != nil {
		return nil, nil, err
	}

	// Create client pointing to mock server
	client := NewClient(mockServer.GetEndpoint(), 5*time.Second, logger)

	// Create policy registry with default policy
	registry := NewPolicyRegistry(logger)
	policy := NewPolicy("test-policy", "1.0", "Test Policy", true, logger)
	rule := Rule{
		ID:         "test-rule-1",
		Name:       "Test Rule 1",
		Condition:  "layer == 'L7'",
		Action:     "allow",
		Priority:   1,
		Confidence: 0.9,
	}
	policy.AddRule(rule)
	registry.RegisterPolicy(policy)

	// Create evaluator
	evaluator := NewEvaluator(client, registry, "test-policy", true, true, logger)

	return evaluator, mockServer, nil
}

// TestClient tests the Kairos HTTP client.
func TestClient(t interface {
	Error(args ...interface{})
	Errorf(format string, args ...interface{})
}, logger *zap.Logger) {
	mockServer, err := NewMockKairosServer()
	if err != nil {
		t.Error("failed to create mock server:", err)
		return
	}
	defer mockServer.Close()

	client := NewClient(mockServer.GetEndpoint(), 5*time.Second, logger)

	// Test successful evaluation
	req := &PolicyRequest{
		SignalID:   "test-signal-1",
		TraceID:    "test-trace-1",
		Layer:      "L7",
		Category:   "api-call",
		RuleID:     "test-rule-1",
		RuleName:   "Test Rule",
		Confidence: 0.8,
		Data:       map[string]interface{}{"key": "value"},
		Timestamp:  time.Now().Unix(),
	}

	resp, err := client.EvaluatePolicy(context.Background(), req)
	if err != nil {
		t.Error("evaluation failed:", err)
		return
	}

	if resp.Decision != "allow" {
		t.Errorf("expected decision 'allow', got '%s'", resp.Decision)
	}

	if mockServer.CallCount != 1 {
		t.Errorf("expected 1 call, got %d", mockServer.CallCount)
	}

	// Test timeout
	ctx, cancel := context.WithTimeout(context.Background(), 1*time.Millisecond)
	defer cancel()

	_, err = client.EvaluatePolicy(ctx, req)
	if err == nil {
		t.Error("expected timeout error")
	}
}

// TestEvaluator tests the policy evaluator.
func TestEvaluator(t interface {
	Error(args ...interface{})
	Errorf(format string, args ...interface{})
}, logger *zap.Logger) {
	evaluator, mockServer, err := NewTestEvaluator(logger)
	if err != nil {
		t.Error("failed to create test evaluator:", err)
		return
	}
	defer mockServer.Close()

	// Test evaluation
	result := evaluator.EvaluateDetection(
		context.Background(),
		"test-signal-1",
		"test-trace-1",
		"L7",
		"api-call",
		"test-rule-1",
		"Test Rule",
		0.8,
		map[string]interface{}{"key": "value"},
	)

	if result.Decision != "allow" {
		t.Errorf("expected decision 'allow', got '%s'", result.Decision)
	}

	if result.Confidence != 0.9 {
		t.Errorf("expected confidence 0.9, got %f", result.Confidence)
	}

	// Test with evaluator disabled
	evaluator.SetEnabled(false)
	result = evaluator.EvaluateDetection(
		context.Background(),
		"test-signal-2",
		"test-trace-2",
		"L7",
		"api-call",
		"test-rule-1",
		"Test Rule",
		0.8,
		map[string]interface{}{"key": "value"},
	)

	if result.Decision != "allow" {
		t.Errorf("expected allow when disabled, got '%s'", result.Decision)
	}
}
