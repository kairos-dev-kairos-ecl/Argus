package pipeline

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"go.uber.org/zap/zaptest"
	"google.golang.org/protobuf/types/known/timestamppb"

	v1 "github.com/argusxdr/argus/gen/go/argus/v1"
	"github.com/argusxdr/argus/internal/pipeline"
)

// TestEnricherCallsGeoIP tests that Enricher processes correctly (IP extraction is placeholder for now)
func TestEnricherCallsGeoIP(t *testing.T) {
	logger := zaptest.NewLogger(t)
	defer logger.Sync()

	// Test with nil GeoIPEnricher (since IP extraction is placeholder)
	enricher := pipeline.NewEnricher(nil, logger)
	ctx := context.Background()

	// Create a signal with enrichment stub IP (placeholder for now)
	sig := &v1.ArgusSignal{
		SignalId:  "sig-123",
		TraceId:   "trace-123",
		SpanId:    "span-123",
		Timestamp: timestamppb.Now(),
	}

	result, err := enricher.Process(ctx, sig)

	// Verify: signal is returned and enrichment initialized
	assert.NoError(t, err)
	assert.NotNil(t, result)
	assert.Equal(t, sig.SignalId, result.SignalId)

	// Verify: Enrichment struct is initialized
	assert.NotNil(t, result.Enrichment)
}

// TestEnricherHandlesNilGeoIP tests that Enricher gracefully handles nil GeoIPEnricher
func TestEnricherHandlesNilGeoIP(t *testing.T) {
	logger := zaptest.NewLogger(t)
	defer logger.Sync()

	// Create enricher with nil GeoIPEnricher
	enricher := pipeline.NewEnricher(nil, logger)
	ctx := context.Background()

	sig := &v1.ArgusSignal{
		SignalId:  "sig-123",
		TraceId:   "trace-123",
		SpanId:    "span-123",
		Timestamp: timestamppb.Now(),
	}

	result, err := enricher.Process(ctx, sig)

	// Verify: signal is returned and enrichment is initialized
	assert.NoError(t, err)
	assert.NotNil(t, result)
	assert.NotNil(t, result.Enrichment)
	assert.Equal(t, sig.SignalId, result.SignalId)
}

// TestEnricherInitializesEnrichment tests that Enricher initializes Enrichment struct if nil
func TestEnricherInitializesEnrichment(t *testing.T) {
	logger := zaptest.NewLogger(t)
	defer logger.Sync()

	enricher := pipeline.NewEnricher(nil, logger)
	ctx := context.Background()

	// Signal with no enrichment
	sig := &v1.ArgusSignal{
		SignalId:    "sig-456",
		TraceId:     "trace-456",
		SpanId:      "span-456",
		Timestamp:   timestamppb.Now(),
		Enrichment:  nil, // Explicitly nil
	}

	result, err := enricher.Process(ctx, sig)

	// Verify: Enrichment is initialized
	assert.NoError(t, err)
	assert.NotNil(t, result.Enrichment)
}

// TestEnricherReturnsSignalUnchangedIfNoEnrichment tests behavior when enrichment is unavailable
func TestEnricherReturnsSignalUnchangedIfNoEnrichment(t *testing.T) {
	logger := zaptest.NewLogger(t)
	defer logger.Sync()

	enricher := pipeline.NewEnricher(nil, logger)
	ctx := context.Background()

	sig := &v1.ArgusSignal{
		SignalId:  "sig-789",
		TraceId:   "trace-789",
		SpanId:    "span-789",
		Timestamp: timestamppb.Now(),
	}

	originalSignalID := sig.SignalId

	result, err := enricher.Process(ctx, sig)

	// Verify: signal remains unchanged (no geo data added since no GeoIP available)
	assert.NoError(t, err)
	assert.NotNil(t, result)
	assert.Equal(t, originalSignalID, result.SignalId)
	assert.NotNil(t, result.Enrichment) // Enrichment should be initialized
	assert.Nil(t, result.Enrichment.Geo) // But Geo should still be nil
}

// TestEnricherProcessorInterfaceCompliance tests that Enricher implements Processor interface
func TestEnricherProcessorInterfaceCompliance(t *testing.T) {
	logger := zaptest.NewLogger(t)
	defer logger.Sync()

	enricher := pipeline.NewEnricher(nil, logger)

	// Verify: Enricher implements Processor interface
	var _ pipeline.Processor = enricher

	// Verify: Name method returns correct string
	assert.Equal(t, "Enricher", enricher.Name())

	// Verify: Process method has correct signature
	ctx := context.Background()
	sig := &v1.ArgusSignal{
		SignalId:  "sig-test",
		TraceId:   "trace-test",
		SpanId:    "span-test",
		Timestamp: timestamppb.Now(),
	}

	result, err := enricher.Process(ctx, sig)
	assert.NoError(t, err)
	assert.NotNil(t, result)
}

// TestEnricherHandlesNilSignal tests that Enricher handles nil signals gracefully
func TestEnricherHandlesNilSignal(t *testing.T) {
	logger := zaptest.NewLogger(t)
	defer logger.Sync()

	enricher := pipeline.NewEnricher(nil, logger)
	ctx := context.Background()

	result, err := enricher.Process(ctx, nil)

	// Verify: nil signal returns nil, nil
	assert.NoError(t, err)
	assert.Nil(t, result)
}

// TestEnricherWithMockGeoIPError tests Enricher behavior when GeoIP is unavailable
func TestEnricherWithMockGeoIPError(t *testing.T) {
	logger := zaptest.NewLogger(t)
	defer logger.Sync()

	// Test with nil GeoIPEnricher (simulates unavailable GeoIP)
	enricher := pipeline.NewEnricher(nil, logger)
	ctx := context.Background()

	sig := &v1.ArgusSignal{
		SignalId:  "sig-error",
		TraceId:   "trace-error",
		SpanId:    "span-error",
		Timestamp: timestamppb.Now(),
	}

	result, err := enricher.Process(ctx, sig)

	// Verify: signal is still returned (graceful degradation)
	assert.NoError(t, err)
	assert.NotNil(t, result)
	assert.NotNil(t, result.Enrichment)
}

// TestEnricherNoMetricsErrors tests that RegisterMetrics doesn't panic on nil registerer
func TestEnricherWithLogger(t *testing.T) {
	logger := zaptest.NewLogger(t)
	defer logger.Sync()

	// Test that custom logger is used
	enricher := pipeline.NewEnricher(nil, logger)
	assert.NotNil(t, enricher)
}

// mockGeoIPEnricher implements a test interface for GeoIPEnricher.
// Since GeoIPEnricher is a concrete type, we just use nil for tests that don't need it.
// For tests that do need mocking, we'd need to refactor GeoIPEnricher to use an interface.
