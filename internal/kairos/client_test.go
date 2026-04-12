package kairos

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"go.uber.org/zap"
)

func TestClientEvaluate(t *testing.T) {
	t.Run("successful evaluation", func(t *testing.T) {
		mockServer := NewMockServer(NewTestDecision("allow", "investigate", 0.9))
		defer mockServer.Close()

		client := NewClient(mockServer.URL+"/evaluate", 5*time.Second, zap.NewNop())

		req := NewTestEvaluationRequest("trace-123", "signal-456")
		decision, err := client.Evaluate(context.Background(), req)

		assert.NoError(t, err)
		assert.NotNil(t, decision)
		assert.Equal(t, "allow", decision.Decision)
		assert.Greater(t, decision.EvaluationTimeMs, 0.0)
	})
}

func TestClientFailOpen(t *testing.T) {
	// Test that unreachable Kairos returns nil (fail-open), not error
	client := NewClient("http://localhost:19999/nonexistent", 100*time.Millisecond, zap.NewNop())
	req := NewTestEvaluationRequest("trace-123", "signal-456")

	decision, err := client.Evaluate(context.Background(), req)

	// Should fail open: nil decision, no error
	assert.Nil(t, decision)
	assert.Nil(t, err) // Fail-open means no error
}

func TestClientHealth(t *testing.T) {
	mockServer := NewMockServer(nil)
	defer mockServer.Close()

	client := NewClient(mockServer.URL, 5*time.Second, zap.NewNop())

	err := client.Health(context.Background())
	assert.NoError(t, err)
}

func TestClientHealthFail(t *testing.T) {
	// Unreachable endpoint
	client := NewClient("http://localhost:19999/nonexistent", 100*time.Millisecond, zap.NewNop())

	err := client.Health(context.Background())
	assert.Error(t, err)
}

func TestClientTimeout(t *testing.T) {
	// Create a slow server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(500 * time.Millisecond)
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	// Client with short timeout
	client := NewClient(server.URL, 100*time.Millisecond, zap.NewNop())
	req := NewTestEvaluationRequest("trace-123", "signal-456")

	decision, err := client.Evaluate(context.Background(), req)

	// Should fail open (timeout is fail-open)
	assert.Nil(t, decision)
	assert.Nil(t, err) // Fail-open
}

func TestClientBadResponse(t *testing.T) {
	// Server that returns malformed JSON
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("not valid json"))
	}))
	defer server.Close()

	client := NewClient(server.URL, 5*time.Second, zap.NewNop())
	req := NewTestEvaluationRequest("trace-123", "signal-456")

	decision, err := client.Evaluate(context.Background(), req)

	// Should return error on malformed response
	assert.Nil(t, decision)
	assert.Error(t, err)
}

func TestClientServerError(t *testing.T) {
	// Server returns 500
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte(`{"error":"server error"}`))
	}))
	defer server.Close()

	client := NewClient(server.URL, 5*time.Second, zap.NewNop())
	req := NewTestEvaluationRequest("trace-123", "signal-456")

	decision, err := client.Evaluate(context.Background(), req)

	// Should fail open (server error)
	assert.Nil(t, decision)
	assert.Nil(t, err) // Fail-open
}
