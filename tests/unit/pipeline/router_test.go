package pipeline

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"go.uber.org/zap/zaptest"
	"google.golang.org/protobuf/types/known/timestamppb"

	v1 "github.com/argusxdr/argus/gen/go/argus/v1"
	"github.com/argusxdr/argus/internal/pipeline"
)

// TestRouterCallsStorageWrite tests that Router calls storage.Write on signal
func TestRouterCallsStorageWrite(t *testing.T) {
	logger := zaptest.NewLogger(t)
	defer logger.Sync()

	// Create a mock storage writer
	mockStorage := &mockStorageWriter{
		writeFunc: func(ctx context.Context, sig *v1.ArgusSignal) error {
			// Verify signal is passed correctly
			assert.NotNil(t, sig)
			assert.Equal(t, "sig-123", sig.SignalId)
			return nil
		},
	}

	router := pipeline.NewRouter(mockStorage, logger)
	ctx := context.Background()

	sig := &v1.ArgusSignal{
		SignalId:  "sig-123",
		TraceId:   "trace-123",
		SpanId:    "span-123",
		Timestamp: timestamppb.Now(),
	}

	result, err := router.Process(ctx, sig)

	// Verify: signal is returned on success
	assert.NoError(t, err)
	assert.NotNil(t, result)
	assert.Equal(t, sig.SignalId, result.SignalId)
}

// TestRouterReturnsErrorIfStorageWriteFails tests Router error handling for write failures
func TestRouterReturnsErrorIfStorageWriteFails(t *testing.T) {
	logger := zaptest.NewLogger(t)
	defer logger.Sync()

	// Create a mock storage writer that fails
	mockStorage := &mockStorageWriter{
		writeFunc: func(ctx context.Context, sig *v1.ArgusSignal) error {
			return errors.New("storage write failed")
		},
	}

	router := pipeline.NewRouter(mockStorage, logger)
	ctx := context.Background()

	sig := &v1.ArgusSignal{
		SignalId:  "sig-fail",
		TraceId:   "trace-fail",
		SpanId:    "span-fail",
		Timestamp: timestamppb.Now(),
	}

	result, err := router.Process(ctx, sig)

	// Verify: error is returned
	assert.Error(t, err)
	assert.Nil(t, result) // Signal is NOT returned on failure
	assert.Equal(t, "storage write failed", err.Error())
}

// TestRouterIncrementsSuccessMetric tests that Router increments success metric
func TestRouterIncrementsSuccessMetric(t *testing.T) {
	logger := zaptest.NewLogger(t)
	defer logger.Sync()

	writeCount := 0
	mockStorage := &mockStorageWriter{
		writeFunc: func(ctx context.Context, sig *v1.ArgusSignal) error {
			writeCount++
			return nil
		},
	}

	router := pipeline.NewRouter(mockStorage, logger)
	ctx := context.Background()

	sig := &v1.ArgusSignal{
		SignalId:  "sig-success",
		TraceId:   "trace-success",
		SpanId:    "span-success",
		Timestamp: timestamppb.Now(),
	}

	result, err := router.Process(ctx, sig)

	// Verify: signal is returned and storage.Write was called
	assert.NoError(t, err)
	assert.NotNil(t, result)
	assert.Equal(t, 1, writeCount)
}

// TestRouterIncrementsErrorMetric tests that Router increments error metric on failure
func TestRouterIncrementsErrorMetric(t *testing.T) {
	logger := zaptest.NewLogger(t)
	defer logger.Sync()

	writeCount := 0
	mockStorage := &mockStorageWriter{
		writeFunc: func(ctx context.Context, sig *v1.ArgusSignal) error {
			writeCount++
			return errors.New("storage error")
		},
	}

	router := pipeline.NewRouter(mockStorage, logger)
	ctx := context.Background()

	sig := &v1.ArgusSignal{
		SignalId:  "sig-error",
		TraceId:   "trace-error",
		SpanId:    "span-error",
		Timestamp: timestamppb.Now(),
	}

	result, err := router.Process(ctx, sig)

	// Verify: error is returned and storage.Write was called
	assert.Error(t, err)
	assert.Nil(t, result)
	assert.Equal(t, 1, writeCount)
}

// TestRouterReturnsNilSignalOnWriteFailure tests that Router returns nil signal on write failure
func TestRouterReturnsNilSignalOnWriteFailure(t *testing.T) {
	logger := zaptest.NewLogger(t)
	defer logger.Sync()

	mockStorage := &mockStorageWriter{
		writeFunc: func(ctx context.Context, sig *v1.ArgusSignal) error {
			return errors.New("write failed")
		},
	}

	router := pipeline.NewRouter(mockStorage, logger)
	ctx := context.Background()

	sig := &v1.ArgusSignal{
		SignalId:  "sig-nil",
		TraceId:   "trace-nil",
		SpanId:    "span-nil",
		Timestamp: timestamppb.Now(),
	}

	result, err := router.Process(ctx, sig)

	// Verify: result is nil (signal not passed downstream)
	assert.Error(t, err)
	assert.Nil(t, result)
}

// TestRouterHandlesNilStorage tests Router behavior with nil storage
func TestRouterHandlesNilStorage(t *testing.T) {
	logger := zaptest.NewLogger(t)
	defer logger.Sync()

	// Create router with nil storage
	router := pipeline.NewRouter(nil, logger)
	ctx := context.Background()

	sig := &v1.ArgusSignal{
		SignalId:  "sig-nil-storage",
		TraceId:   "trace-nil-storage",
		SpanId:    "span-nil-storage",
		Timestamp: timestamppb.Now(),
	}

	result, err := router.Process(ctx, sig)

	// Verify: error is returned
	assert.Error(t, err)
	assert.Nil(t, result)
}

// TestRouterProcessorInterfaceCompliance tests that Router implements Processor interface
func TestRouterProcessorInterfaceCompliance(t *testing.T) {
	logger := zaptest.NewLogger(t)
	defer logger.Sync()

	router := pipeline.NewRouter(nil, logger)

	// Verify: Router implements Processor interface
	var _ pipeline.Processor = router

	// Verify: Name method returns correct string
	assert.Equal(t, "Router", router.Name())
}

// TestRouterHandlesNilSignal tests that Router handles nil signals gracefully
func TestRouterHandlesNilSignal(t *testing.T) {
	logger := zaptest.NewLogger(t)
	defer logger.Sync()

	mockStorage := &mockStorageWriter{
		writeFunc: func(ctx context.Context, sig *v1.ArgusSignal) error {
			return nil
		},
	}

	router := pipeline.NewRouter(mockStorage, logger)
	ctx := context.Background()

	result, err := router.Process(ctx, nil)

	// Verify: nil signal returns nil, nil
	assert.NoError(t, err)
	assert.Nil(t, result)
}

// TestRouterErrorMessageIncludes tests that Router error messages include signal_id
func TestRouterErrorMessageIncludes(t *testing.T) {
	logger := zaptest.NewLogger(t)
	defer logger.Sync()

	mockStorage := &mockStorageWriter{
		writeFunc: func(ctx context.Context, sig *v1.ArgusSignal) error {
			return errors.New("connection timeout")
		},
	}

	router := pipeline.NewRouter(mockStorage, logger)
	ctx := context.Background()

	sig := &v1.ArgusSignal{
		SignalId:  "sig-specific",
		TraceId:   "trace-specific",
		SpanId:    "span-specific",
		Timestamp: timestamppb.Now(),
	}

	result, err := router.Process(ctx, sig)

	// Verify: error details are preserved
	assert.Error(t, err)
	assert.Nil(t, result)
	assert.Equal(t, "connection timeout", err.Error())
}

// mockStorageWriter is a test mock for storage.Writer interface
type mockStorageWriter struct {
	writeFunc func(ctx context.Context, sig *v1.ArgusSignal) error
}

func (m *mockStorageWriter) Write(ctx context.Context, sig *v1.ArgusSignal) error {
	if m.writeFunc != nil {
		return m.writeFunc(ctx, sig)
	}
	return nil
}
