package pipeline_test

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
	"google.golang.org/protobuf/types/known/timestamppb"

	"github.com/argusxdr/argus/internal/pipeline"
	v1 "github.com/argusxdr/argus/gen/go/argus/v1"

	// Redis test support
	"github.com/redis/go-redis/v9"
)

// newTestRedisClient creates an in-memory Redis client for testing (if Redis is available).
// Falls back to a mock implementation if Redis is not available.
func newTestRedisClient(t *testing.T) *redis.Client {
	// Try to connect to a local Redis instance (tests assume Redis is running on localhost:6379)
	client := redis.NewClient(&redis.Options{
		Addr: "localhost:6379",
	})

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	// Check connectivity
	err := client.Ping(ctx).Err()
	if err != nil {
		t.Logf("Warning: Redis not available for correlation tests, using mock instead: %v", err)
		// In a production setup, use a proper mock library or embedded Redis
		// For now, we'll use miniredis or skip the test if Redis unavailable
		t.Skip("Redis not available (required for correlation tests)")
	}

	return client
}

// cleanupRedis flushes the Redis database after each test
func cleanupRedis(t *testing.T, client *redis.Client) {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	err := client.FlushAll(ctx).Err()
	if err != nil {
		t.Logf("Warning: failed to flush Redis: %v", err)
	}
}

// TestCorrelationTagger_TwoSignalsSameTrace tests that two signals with the same trace_id
// are correlated: both should get each other's signal ID in related_signals.
func TestCorrelationTagger_TwoSignalsSameTrace(t *testing.T) {
	client := newTestRedisClient(t)
	defer cleanupRedis(t, client)
	defer client.Close()

	ctx := context.Background()
	tagger := pipeline.NewCorrelationTagger(client, 5*time.Second)
	logger := zap.NewNop()
	tagger.SetLogger(logger)

	// Create first signal
	sig1 := &v1.ArgusSignal{
		SignalId: "signal-001",
		TraceId:  "trace-abc",
		SpanId:   "span-001",
		Source: &v1.Source{
			AppId:       "app-1",
			AppVersion:  "1.0.0",
			SdkVersion:  "0.1.0",
			Environment: "test",
			InstanceId:  "instance-1",
		},
		Layer:      v1.Layer_L7_RAG_RETRIEVAL,
		Category:   "retrieval.search",
		Severity:   v1.Severity_MEDIUM,
		Timestamp:  timestamppb.Now(),
		IngestedAt: timestamppb.Now(),
	}

	// Process first signal
	result1, err := tagger.Process(ctx, sig1)
	require.NoError(t, err)
	require.NotNil(t, result1)
	assert.Equal(t, sig1.SignalId, result1.SignalId)

	// At this point, signal 1 has no related signals (first in window)
	assert.Empty(t, result1.RelatedSignals)

	// Create second signal with same trace_id (immediately after)
	sig2 := &v1.ArgusSignal{
		SignalId: "signal-002",
		TraceId:  "trace-abc", // Same trace as sig1
		SpanId:   "span-002",
		Source: &v1.Source{
			AppId:       "app-1",
			AppVersion:  "1.0.0",
			SdkVersion:  "0.1.0",
			Environment: "test",
			InstanceId:  "instance-2",
		},
		Layer:      v1.Layer_L8_AGENTS,
		Category:   "agent.tool_call",
		Severity:   v1.Severity_HIGH,
		Timestamp:  timestamppb.Now(),
		IngestedAt: timestamppb.Now(),
	}

	// Process second signal
	result2, err := tagger.Process(ctx, sig2)
	require.NoError(t, err)
	require.NotNil(t, result2)
	assert.Equal(t, sig2.SignalId, result2.SignalId)

	// Signal 2 should have signal 1 in related_signals
	assert.Len(t, result2.RelatedSignals, 1)
	assert.Contains(t, result2.RelatedSignals, sig1.SignalId)
}

// TestCorrelationTagger_SignalNoTraceId tests that a signal with no trace_id
// is returned unchanged without any correlation processing.
func TestCorrelationTagger_SignalNoTraceId(t *testing.T) {
	client := newTestRedisClient(t)
	defer cleanupRedis(t, client)
	defer client.Close()

	ctx := context.Background()
	tagger := pipeline.NewCorrelationTagger(client, 5*time.Second)
	logger := zap.NewNop()
	tagger.SetLogger(logger)

	// Create signal with empty trace_id
	sig := &v1.ArgusSignal{
		SignalId: "signal-001",
		TraceId:  "", // Empty trace_id
		SpanId:   "span-001",
		Source: &v1.Source{
			AppId:       "app-1",
			AppVersion:  "1.0.0",
			SdkVersion:  "0.1.0",
			Environment: "test",
			InstanceId:  "instance-1",
		},
		Layer:      v1.Layer_L7_RAG_RETRIEVAL,
		Category:   "retrieval.search",
		Severity:   v1.Severity_MEDIUM,
		Timestamp:  timestamppb.Now(),
		IngestedAt: timestamppb.Now(),
	}

	result, err := tagger.Process(ctx, sig)
	require.NoError(t, err)
	assert.NotNil(t, result)
	assert.Equal(t, sig, result)
	assert.Empty(t, result.RelatedSignals)
}

// TestCorrelationTagger_SignalsOutsideWindow tests that signals arriving outside
// the correlation window are not correlated with each other.
func TestCorrelationTagger_SignalsOutsideWindow(t *testing.T) {
	client := newTestRedisClient(t)
	defer cleanupRedis(t, client)
	defer client.Close()

	ctx := context.Background()
	// Very short window (100 milliseconds)
	tagger := pipeline.NewCorrelationTagger(client, 100*time.Millisecond)
	logger := zap.NewNop()
	tagger.SetLogger(logger)

	// Create first signal
	ts1 := timestamppb.New(time.Now())
	sig1 := &v1.ArgusSignal{
		SignalId: "signal-001",
		TraceId:  "trace-xyz",
		SpanId:   "span-001",
		Source: &v1.Source{
			AppId:       "app-1",
			AppVersion:  "1.0.0",
			SdkVersion:  "0.1.0",
			Environment: "test",
			InstanceId:  "instance-1",
		},
		Layer:      v1.Layer_L7_RAG_RETRIEVAL,
		Category:   "retrieval.search",
		Severity:   v1.Severity_MEDIUM,
		Timestamp:  ts1,
		IngestedAt: timestamppb.Now(),
	}

	// Process first signal
	result1, err := tagger.Process(ctx, sig1)
	require.NoError(t, err)
	assert.Empty(t, result1.RelatedSignals)

	// Wait longer than the window duration
	time.Sleep(200 * time.Millisecond)

	// Create second signal with same trace_id but outside window
	ts2 := timestamppb.New(time.Now())
	sig2 := &v1.ArgusSignal{
		SignalId: "signal-002",
		TraceId:  "trace-xyz", // Same trace
		SpanId:   "span-002",
		Source: &v1.Source{
			AppId:       "app-1",
			AppVersion:  "1.0.0",
			SdkVersion:  "0.1.0",
			Environment: "test",
			InstanceId:  "instance-2",
		},
		Layer:      v1.Layer_L8_AGENTS,
		Category:   "agent.tool_call",
		Severity:   v1.Severity_HIGH,
		Timestamp:  ts2,
		IngestedAt: timestamppb.Now(),
	}

	// Process second signal
	result2, err := tagger.Process(ctx, sig2)
	require.NoError(t, err)

	// Signal 2 should NOT have signal 1 in related_signals (outside window)
	assert.Empty(t, result2.RelatedSignals, "Signals outside correlation window should not be related")
}

// TestCorrelationTagger_RedisError tests that if Redis is unavailable or returns
// an error, the signal still passes through unchanged (non-fatal error handling).
func TestCorrelationTagger_RedisError(t *testing.T) {
	// Create a Redis client that points to an invalid address
	client := redis.NewClient(&redis.Options{
		Addr: "invalid-host:6379",
	})

	ctx, cancel := context.WithTimeout(context.Background(), 500*time.Millisecond)
	defer cancel()

	tagger := pipeline.NewCorrelationTagger(client, 5*time.Second)
	logger := zap.NewNop()
	tagger.SetLogger(logger)

	sig := &v1.ArgusSignal{
		SignalId: "signal-001",
		TraceId:  "trace-001",
		SpanId:   "span-001",
		Source: &v1.Source{
			AppId:       "app-1",
			AppVersion:  "1.0.0",
			SdkVersion:  "0.1.0",
			Environment: "test",
			InstanceId:  "instance-1",
		},
		Layer:      v1.Layer_L7_RAG_RETRIEVAL,
		Category:   "retrieval.search",
		Severity:   v1.Severity_MEDIUM,
		Timestamp:  timestamppb.Now(),
		IngestedAt: timestamppb.Now(),
	}

	// Process signal despite Redis being unavailable
	result, err := tagger.Process(ctx, sig)

	// Signal should still be returned (non-fatal error)
	require.NoError(t, err)
	assert.NotNil(t, result)
	assert.Equal(t, sig.SignalId, result.SignalId)
	// related_signals will be empty due to Redis error, but signal is not rejected
	assert.Empty(t, result.RelatedSignals)
}

// TestCorrelationTagger_NilSignal tests that nil input is handled gracefully
func TestCorrelationTagger_NilSignal(t *testing.T) {
	client := newTestRedisClient(t)
	defer client.Close()

	ctx := context.Background()
	tagger := pipeline.NewCorrelationTagger(client, 5*time.Second)
	logger := zap.NewNop()
	tagger.SetLogger(logger)

	result, err := tagger.Process(ctx, nil)
	require.NoError(t, err)
	assert.Nil(t, result)
}

// TestCorrelationTagger_ProcessorInterface tests that CorrelationTagger implements
// the Processor interface correctly
func TestCorrelationTagger_ProcessorInterface(t *testing.T) {
	client := newTestRedisClient(t)
	defer client.Close()

	tagger := pipeline.NewCorrelationTagger(client, 5*time.Second)

	// Check that Name() returns a non-empty string
	name := tagger.Name()
	assert.Equal(t, "CorrelationTagger", name)

	// Check that it implements the Processor interface by calling Process
	ctx := context.Background()
	sig := &v1.ArgusSignal{
		SignalId: "signal-001",
		TraceId:  "trace-001",
		SpanId:   "span-001",
		Source: &v1.Source{
			AppId:       "app-1",
			AppVersion:  "1.0.0",
			SdkVersion:  "0.1.0",
			Environment: "test",
			InstanceId:  "instance-1",
		},
		Layer:      v1.Layer_L7_RAG_RETRIEVAL,
		Category:   "retrieval.search",
		Severity:   v1.Severity_MEDIUM,
		Timestamp:  timestamppb.Now(),
		IngestedAt: timestamppb.Now(),
	}

	result, err := tagger.Process(ctx, sig)
	require.NoError(t, err)
	assert.NotNil(t, result)
}

// TestCorrelationTagger_MultipleSignalsSameTrace tests that a signal can be
// correlated with multiple other signals within the window
func TestCorrelationTagger_MultipleSignalsSameTrace(t *testing.T) {
	client := newTestRedisClient(t)
	defer cleanupRedis(t, client)
	defer client.Close()

	ctx := context.Background()
	tagger := pipeline.NewCorrelationTagger(client, 5*time.Second)
	logger := zap.NewNop()
	tagger.SetLogger(logger)

	// Create and process three signals with the same trace_id
	signals := make([]*v1.ArgusSignal, 3)
	for i := 0; i < 3; i++ {
		signals[i] = &v1.ArgusSignal{
			SignalId: fmt.Sprintf("signal-%03d", i+1),
			TraceId:  "trace-multi",
			SpanId:   fmt.Sprintf("span-%03d", i+1),
			Source: &v1.Source{
				AppId:       "app-1",
				AppVersion:  "1.0.0",
				SdkVersion:  "0.1.0",
				Environment: "test",
				InstanceId:  fmt.Sprintf("instance-%d", i+1),
			},
			Layer:      v1.Layer_L7_RAG_RETRIEVAL,
			Category:   "retrieval.search",
			Severity:   v1.Severity_MEDIUM,
			Timestamp:  timestamppb.Now(),
			IngestedAt: timestamppb.Now(),
		}

		result, err := tagger.Process(ctx, signals[i])
		require.NoError(t, err)
		require.NotNil(t, result)
	}

	// The last signal should have the first two signals in related_signals
	// Re-process the last signal to check its related signals (or check via Redis query)
	// For this test, we'll verify by processing a new signal that should see all previous ones
	finalSig := &v1.ArgusSignal{
		SignalId: "signal-final",
		TraceId:  "trace-multi",
		SpanId:   "span-final",
		Source: &v1.Source{
			AppId:       "app-1",
			AppVersion:  "1.0.0",
			SdkVersion:  "0.1.0",
			Environment: "test",
			InstanceId:  "instance-final",
		},
		Layer:      v1.Layer_L9_API_GATEWAY,
		Category:   "gateway.rate_limit",
		Severity:   v1.Severity_LOW,
		Timestamp:  timestamppb.Now(),
		IngestedAt: timestamppb.Now(),
	}

	result, err := tagger.Process(ctx, finalSig)
	require.NoError(t, err)

	// Final signal should have all three previous signals in related_signals
	assert.Len(t, result.RelatedSignals, 3)
	for i := 0; i < 3; i++ {
		assert.Contains(t, result.RelatedSignals, signals[i].SignalId)
	}
}

// TestCorrelationTagger_ExcludesSelf tests that a signal is not included in its own
// related_signals list
func TestCorrelationTagger_ExcludesSelf(t *testing.T) {
	client := newTestRedisClient(t)
	defer cleanupRedis(t, client)
	defer client.Close()

	ctx := context.Background()
	tagger := pipeline.NewCorrelationTagger(client, 5*time.Second)
	logger := zap.NewNop()
	tagger.SetLogger(logger)

	// Create a signal and process it twice (simulating a late-arriving signal matching itself)
	sig := &v1.ArgusSignal{
		SignalId: "signal-001",
		TraceId:  "trace-001",
		SpanId:   "span-001",
		Source: &v1.Source{
			AppId:       "app-1",
			AppVersion:  "1.0.0",
			SdkVersion:  "0.1.0",
			Environment: "test",
			InstanceId:  "instance-1",
		},
		Layer:      v1.Layer_L7_RAG_RETRIEVAL,
		Category:   "retrieval.search",
		Severity:   v1.Severity_MEDIUM,
		Timestamp:  timestamppb.Now(),
		IngestedAt: timestamppb.Now(),
	}

	result1, err := tagger.Process(ctx, sig)
	require.NoError(t, err)

	// First result should have no related signals
	assert.Empty(t, result1.RelatedSignals)

	// Create a second signal with the same ID (edge case)
	sig2 := &v1.ArgusSignal{
		SignalId: "signal-001", // Same ID
		TraceId:  "trace-001",
		SpanId:   "span-001",
		Source: &v1.Source{
			AppId:       "app-1",
			AppVersion:  "1.0.0",
			SdkVersion:  "0.1.0",
			Environment: "test",
			InstanceId:  "instance-1",
		},
		Layer:      v1.Layer_L7_RAG_RETRIEVAL,
		Category:   "retrieval.search",
		Severity:   v1.Severity_MEDIUM,
		Timestamp:  timestamppb.Now(),
		IngestedAt: timestamppb.Now(),
	}

	result2, err := tagger.Process(ctx, sig2)
	require.NoError(t, err)

	// Second result should also not include itself
	for _, relatedID := range result2.RelatedSignals {
		assert.NotEqual(t, sig2.SignalId, relatedID, "Signal should not be in its own related_signals")
	}
}
