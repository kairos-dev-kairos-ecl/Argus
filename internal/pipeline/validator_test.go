package pipeline

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
	"google.golang.org/protobuf/types/known/timestamppb"

	v1 "github.com/argusxdr/argus/gen/go/argus/v1"
)

func TestSchemaValidator_ValidSignal(t *testing.T) {
	ctx := context.Background()
	logger := zap.NewNop()
	validator, err := NewSchemaValidator(logger)
	require.NoError(t, err)

	sig := &v1.ArgusSignal{
		SignalId: "test-signal-001",
		TraceId:  "trace-001",
		SpanId:   "span-001",
		Source: &v1.Source{
			AppId:      "test-app",
			AppVersion: "1.0.0",
			SdkVersion: "0.1.0",
			Environment: "test",
			InstanceId: "instance-1",
		},
		Layer:     v1.Layer_L7_RAG_RETRIEVAL,
		Category:  "retrieval.search",
		Severity:  v1.Severity_MEDIUM,
		Timestamp: timestamppb.Now(),
		IngestedAt: timestamppb.Now(),
		DataClassification: v1.DataClassification_INTERNAL,
	}

	result, err := validator.Process(ctx, sig)
	assert.NoError(t, err)
	assert.NotNil(t, result)
	assert.Equal(t, sig, result)
}

func TestSchemaValidator_MissingSignalId(t *testing.T) {
	ctx := context.Background()
	logger := zap.NewNop()
	validator, err := NewSchemaValidator(logger)
	require.NoError(t, err)

	sig := &v1.ArgusSignal{
		SignalId: "",  // Missing required field
		TraceId:  "trace-001",
		SpanId:   "span-001",
		Source: &v1.Source{
			AppId:       "test-app",
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

	result, err := validator.Process(ctx, sig)
	assert.Error(t, err)
	assert.Nil(t, result)
	assert.Contains(t, err.Error(), "signal_id")
}

func TestSchemaValidator_MissingTraceId(t *testing.T) {
	ctx := context.Background()
	logger := zap.NewNop()
	validator, err := NewSchemaValidator(logger)
	require.NoError(t, err)

	sig := &v1.ArgusSignal{
		SignalId: "test-signal-001",
		TraceId:  "",  // Missing required field
		SpanId:   "span-001",
		Source: &v1.Source{
			AppId:       "test-app",
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

	result, err := validator.Process(ctx, sig)
	assert.Error(t, err)
	assert.Nil(t, result)
	assert.Contains(t, err.Error(), "trace_id")
}

func TestSchemaValidator_MissingSource(t *testing.T) {
	ctx := context.Background()
	logger := zap.NewNop()
	validator, err := NewSchemaValidator(logger)
	require.NoError(t, err)

	sig := &v1.ArgusSignal{
		SignalId:   "test-signal-001",
		TraceId:    "trace-001",
		SpanId:     "span-001",
		Source:     nil,  // Missing required field
		Layer:      v1.Layer_L7_RAG_RETRIEVAL,
		Category:   "retrieval.search",
		Severity:   v1.Severity_MEDIUM,
		Timestamp:  timestamppb.Now(),
		IngestedAt: timestamppb.Now(),
	}

	result, err := validator.Process(ctx, sig)
	assert.Error(t, err)
	assert.Nil(t, result)
	assert.Contains(t, err.Error(), "source")
}

func TestSchemaValidator_InvalidSourceFields(t *testing.T) {
	ctx := context.Background()
	logger := zap.NewNop()
	validator, err := NewSchemaValidator(logger)
	require.NoError(t, err)

	sig := &v1.ArgusSignal{
		SignalId: "test-signal-001",
		TraceId:  "trace-001",
		SpanId:   "span-001",
		Source: &v1.Source{
			AppId:      "",  // Missing required field
			AppVersion: "1.0.0",
			SdkVersion: "0.1.0",
			Environment: "test",
			InstanceId: "instance-1",
		},
		Layer:      v1.Layer_L7_RAG_RETRIEVAL,
		Category:   "retrieval.search",
		Severity:   v1.Severity_MEDIUM,
		Timestamp:  timestamppb.Now(),
		IngestedAt: timestamppb.Now(),
	}

	result, err := validator.Process(ctx, sig)
	assert.Error(t, err)
	assert.Nil(t, result)
	assert.Contains(t, err.Error(), "source.app_id")
}

func TestSchemaValidator_InvalidLayerEnum(t *testing.T) {
	ctx := context.Background()
	logger := zap.NewNop()
	validator, err := NewSchemaValidator(logger)
	require.NoError(t, err)

	sig := &v1.ArgusSignal{
		SignalId: "test-signal-001",
		TraceId:  "trace-001",
		SpanId:   "span-001",
		Source: &v1.Source{
			AppId:       "test-app",
			AppVersion:  "1.0.0",
			SdkVersion:  "0.1.0",
			Environment: "test",
			InstanceId:  "instance-1",
		},
		Layer:      v1.Layer_LAYER_UNSPECIFIED,  // Invalid enum value
		Category:   "retrieval.search",
		Severity:   v1.Severity_MEDIUM,
		Timestamp:  timestamppb.Now(),
		IngestedAt: timestamppb.Now(),
	}

	result, err := validator.Process(ctx, sig)
	assert.Error(t, err)
	assert.Nil(t, result)
	assert.Contains(t, err.Error(), "layer")
}

func TestSchemaValidator_InvalidSeverityEnum(t *testing.T) {
	ctx := context.Background()
	logger := zap.NewNop()
	validator, err := NewSchemaValidator(logger)
	require.NoError(t, err)

	sig := &v1.ArgusSignal{
		SignalId: "test-signal-001",
		TraceId:  "trace-001",
		SpanId:   "span-001",
		Source: &v1.Source{
			AppId:       "test-app",
			AppVersion:  "1.0.0",
			SdkVersion:  "0.1.0",
			Environment: "test",
			InstanceId:  "instance-1",
		},
		Layer:      v1.Layer_L7_RAG_RETRIEVAL,
		Category:   "retrieval.search",
		Severity:   v1.Severity_SEVERITY_UNSPECIFIED,  // Invalid enum value
		Timestamp:  timestamppb.Now(),
		IngestedAt: timestamppb.Now(),
	}

	result, err := validator.Process(ctx, sig)
	assert.Error(t, err)
	assert.Nil(t, result)
	assert.Contains(t, err.Error(), "severity")
}

func TestSchemaValidator_MissingTimestamp(t *testing.T) {
	ctx := context.Background()
	logger := zap.NewNop()
	validator, err := NewSchemaValidator(logger)
	require.NoError(t, err)

	sig := &v1.ArgusSignal{
		SignalId: "test-signal-001",
		TraceId:  "trace-001",
		SpanId:   "span-001",
		Source: &v1.Source{
			AppId:       "test-app",
			AppVersion:  "1.0.0",
			SdkVersion:  "0.1.0",
			Environment: "test",
			InstanceId:  "instance-1",
		},
		Layer:      v1.Layer_L7_RAG_RETRIEVAL,
		Category:   "retrieval.search",
		Severity:   v1.Severity_MEDIUM,
		Timestamp:  nil,  // Missing required field
		IngestedAt: timestamppb.Now(),
	}

	result, err := validator.Process(ctx, sig)
	assert.Error(t, err)
	assert.Nil(t, result)
	assert.Contains(t, err.Error(), "timestamp")
}

func TestSchemaValidator_MissingIngestedAt(t *testing.T) {
	ctx := context.Background()
	logger := zap.NewNop()
	validator, err := NewSchemaValidator(logger)
	require.NoError(t, err)

	sig := &v1.ArgusSignal{
		SignalId: "test-signal-001",
		TraceId:  "trace-001",
		SpanId:   "span-001",
		Source: &v1.Source{
			AppId:       "test-app",
			AppVersion:  "1.0.0",
			SdkVersion:  "0.1.0",
			Environment: "test",
			InstanceId:  "instance-1",
		},
		Layer:      v1.Layer_L7_RAG_RETRIEVAL,
		Category:   "retrieval.search",
		Severity:   v1.Severity_MEDIUM,
		Timestamp:  timestamppb.Now(),
		IngestedAt: nil,  // Missing required field
	}

	result, err := validator.Process(ctx, sig)
	assert.Error(t, err)
	assert.Nil(t, result)
	assert.Contains(t, err.Error(), "ingested_at")
}

func TestSchemaValidator_MissingCategory(t *testing.T) {
	ctx := context.Background()
	logger := zap.NewNop()
	validator, err := NewSchemaValidator(logger)
	require.NoError(t, err)

	sig := &v1.ArgusSignal{
		SignalId: "test-signal-001",
		TraceId:  "trace-001",
		SpanId:   "span-001",
		Source: &v1.Source{
			AppId:       "test-app",
			AppVersion:  "1.0.0",
			SdkVersion:  "0.1.0",
			Environment: "test",
			InstanceId:  "instance-1",
		},
		Layer:      v1.Layer_L7_RAG_RETRIEVAL,
		Category:   "",  // Missing required field
		Severity:   v1.Severity_MEDIUM,
		Timestamp:  timestamppb.Now(),
		IngestedAt: timestamppb.Now(),
	}

	result, err := validator.Process(ctx, sig)
	assert.Error(t, err)
	assert.Nil(t, result)
	assert.Contains(t, err.Error(), "category")
}

func TestSchemaValidator_ValidEnrichment(t *testing.T) {
	ctx := context.Background()
	logger := zap.NewNop()
	validator, err := NewSchemaValidator(logger)
	require.NoError(t, err)

	deviation := float32(1.5)
	riskScore := float32(0.75)

	sig := &v1.ArgusSignal{
		SignalId: "test-signal-001",
		TraceId:  "trace-001",
		SpanId:   "span-001",
		Source: &v1.Source{
			AppId:       "test-app",
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
		Enrichment: &v1.Enrichment{
			BaselineDeviation: &deviation,
			RiskScore:         &riskScore,
		},
	}

	result, err := validator.Process(ctx, sig)
	assert.NoError(t, err)
	assert.NotNil(t, result)
}

func TestSchemaValidator_InvalidBaselineDeviation(t *testing.T) {
	ctx := context.Background()
	logger := zap.NewNop()
	validator, err := NewSchemaValidator(logger)
	require.NoError(t, err)

	deviation := float32(15.0)  // Out of range [-10, 10]

	sig := &v1.ArgusSignal{
		SignalId: "test-signal-001",
		TraceId:  "trace-001",
		SpanId:   "span-001",
		Source: &v1.Source{
			AppId:       "test-app",
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
		Enrichment: &v1.Enrichment{
			BaselineDeviation: &deviation,
		},
	}

	result, err := validator.Process(ctx, sig)
	assert.Error(t, err)
	assert.Nil(t, result)
	assert.Contains(t, err.Error(), "baseline_deviation")
}

func TestSchemaValidator_InvalidRiskScore(t *testing.T) {
	ctx := context.Background()
	logger := zap.NewNop()
	validator, err := NewSchemaValidator(logger)
	require.NoError(t, err)

	riskScore := float32(1.5)  // Out of range [0, 1]

	sig := &v1.ArgusSignal{
		SignalId: "test-signal-001",
		TraceId:  "trace-001",
		SpanId:   "span-001",
		Source: &v1.Source{
			AppId:       "test-app",
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
		Enrichment: &v1.Enrichment{
			RiskScore: &riskScore,
		},
	}

	result, err := validator.Process(ctx, sig)
	assert.Error(t, err)
	assert.Nil(t, result)
	assert.Contains(t, err.Error(), "risk_score")
}

func TestSchemaValidator_NilSignal(t *testing.T) {
	ctx := context.Background()
	logger := zap.NewNop()
	validator, err := NewSchemaValidator(logger)
	require.NoError(t, err)

	result, err := validator.Process(ctx, nil)
	assert.NoError(t, err)
	assert.Nil(t, result)
}

func TestSchemaValidator_MultipleErrors(t *testing.T) {
	ctx := context.Background()
	logger := zap.NewNop()
	validator, err := NewSchemaValidator(logger)
	require.NoError(t, err)

	sig := &v1.ArgusSignal{
		SignalId: "",  // Missing
		TraceId:  "",  // Missing
		Category: "",  // Missing
		// Missing Source, Layer, Severity, Timestamp, IngestedAt
	}

	result, err := validator.Process(ctx, sig)
	assert.Error(t, err)
	assert.Nil(t, result)
	// Check that error message contains multiple field names
	errStr := err.Error()
	assert.Contains(t, errStr, "signal_id")
	assert.Contains(t, errStr, "trace_id")
	assert.Contains(t, errStr, "category")
}

func TestSchemaValidator_Name(t *testing.T) {
	logger := zap.NewNop()
	validator, err := NewSchemaValidator(logger)
	require.NoError(t, err)

	assert.Equal(t, "SchemaValidator", validator.Name())
}

func TestSchemaValidator_AllDataClassifications(t *testing.T) {
	ctx := context.Background()
	logger := zap.NewNop()
	validator, err := NewSchemaValidator(logger)
	require.NoError(t, err)

	testCases := []v1.DataClassification{
		v1.DataClassification_CLASSIFICATION_UNSPECIFIED,
		v1.DataClassification_PUBLIC,
		v1.DataClassification_INTERNAL,
		v1.DataClassification_CONFIDENTIAL,
		v1.DataClassification_RESTRICTED,
	}

	for _, dc := range testCases {
		sig := &v1.ArgusSignal{
			SignalId:            "test-signal-001",
			TraceId:             "trace-001",
			SpanId:              "span-001",
			Source: &v1.Source{
				AppId:       "test-app",
				AppVersion:  "1.0.0",
				SdkVersion:  "0.1.0",
				Environment: "test",
				InstanceId:  "instance-1",
			},
			Layer:               v1.Layer_L7_RAG_RETRIEVAL,
			Category:            "retrieval.search",
			Severity:            v1.Severity_MEDIUM,
			Timestamp:           timestamppb.Now(),
			IngestedAt:          timestamppb.Now(),
			DataClassification:  dc,
		}

		result, err := validator.Process(ctx, sig)
		assert.NoError(t, err, "DataClassification %v should be valid", dc)
		assert.NotNil(t, result)
	}
}
