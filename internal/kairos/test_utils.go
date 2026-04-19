package kairos

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"time"
)

// MockServer creates a mock Kairos HTTP server for testing
type MockServer struct {
	server       *httptest.Server
	URL          string
	LastRequest  *EvaluationRequest
	Decision     *PolicyDecision
	ResponseCode int
	ShouldError  bool
	// CallCount tracks number of /evaluate requests received.
	CallCount atomic.Int64
}

// NewMockServer creates a mock Kairos server that returns a fixed decision
func NewMockServer(decision *PolicyDecision) *MockServer {
	if decision == nil {
		decision = &PolicyDecision{
			Decision:          "allow",
			Confidence:        1.0,
			Reasoning:         "Test decision",
			RecommendedAction: "investigate",
			PolicyVersion:     "1.0.0",
		}
	}

	m := &MockServer{
		Decision:     decision,
		ResponseCode: http.StatusOK,
	}

	m.server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/health" {
			w.WriteHeader(http.StatusOK)
			w.Write([]byte(`{"status":"ok"}`))
			return
		}

		if r.URL.Path == "/api/v1/evaluate" || r.URL.Path == "/evaluate" {
			m.CallCount.Add(1)
			// Parse request (simplified for testing)
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(m.ResponseCode)

			// Marshal decision
			respBytes, _ := json.Marshal(m.Decision)
			w.Write(respBytes)
			return
		}

		w.WriteHeader(http.StatusNotFound)
	}))

	m.URL = m.server.URL
	return m
}

// Close shuts down the mock server
func (m *MockServer) Close() {
	if m.server != nil {
		m.server.Close()
	}
}

// TestHelpers provides utility functions for testing

// NewTestEvaluator creates an evaluator with a mock server
func NewTestEvaluator(decision *PolicyDecision) (*Evaluator, *MockServer) {
	mockServer := NewMockServer(decision)

	config := &PolicyConfig{
		Enabled:     true,
		Endpoint:    mockServer.URL + "/evaluate",
		TimeoutMs:   1000,
		FailOpen:    true,
		CacheSize:   100,
		CacheTTLSec: 60,
		Policy:      DefaultPolicy(),
	}

	client := NewClient(config.Endpoint, time.Duration(config.TimeoutMs)*time.Millisecond, nil)
	evaluator := NewEvaluator(client, config, nil)

	return evaluator, mockServer
}

// NewTestEvaluationRequest creates a test evaluation request
func NewTestEvaluationRequest(traceID, signalID string) *EvaluationRequest {
	return &EvaluationRequest{
		TraceID:    traceID,
		SignalID:   signalID,
		Layer:      "L9_API_GATEWAY",
		Category:   "security.threat",
		Severity:   "HIGH",
		RiskScore:  0.75,
		PolicyName: "default",
		Context: map[string]interface{}{
			"user_id": "user-123",
			"action":  "suspicious_login",
		},
	}
}

// NewTestDecision creates a test policy decision
func NewTestDecision(decision, action string, confidence float64) *PolicyDecision {
	return &PolicyDecision{
		Decision:          decision,
		Confidence:        confidence,
		Reasoning:         "Test decision for " + decision,
		RecommendedAction: action,
		PolicyVersion:     "1.0.0",
		PolicyName:        "default",
	}
}
