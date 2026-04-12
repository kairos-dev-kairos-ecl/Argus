package pipeline_test

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
	"google.golang.org/protobuf/types/known/timestamppb"

	"github.com/argusxdr/argus/internal/pipeline"
	v1 "github.com/argusxdr/argus/gen/go/argus/v1"
)

// TestNormalizer_ProviderNameLowercase_OpenAI tests provider name "OpenAI" → "openai"
func TestNormalizer_ProviderNameLowercase_OpenAI(t *testing.T) {
	ctx := context.Background()
	logger := zap.NewNop()
	normalizer := pipeline.NewNormalizer(logger)

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
		Layer:       v1.Layer_L4_TRANSFORMER,
		Category:    "generation",
		Severity:    v1.Severity_INFO,
		Timestamp:   timestamppb.Now(),
		IngestedAt:  timestamppb.Now(),
		Provider: &v1.Provider{
			Name:  "OpenAI",
			Model: "gpt-4",
		},
	}

	result, err := normalizer.Process(ctx, sig)
	require.NoError(t, err)
	require.NotNil(t, result)
	assert.Equal(t, "openai", result.Provider.Name)
	// Verify input is not mutated
	assert.Equal(t, "OpenAI", sig.Provider.Name)
}

// TestNormalizer_ProviderNameLowercase_Anthropic tests provider name "ANTHROPIC" → "anthropic"
func TestNormalizer_ProviderNameLowercase_Anthropic(t *testing.T) {
	ctx := context.Background()
	logger := zap.NewNop()
	normalizer := pipeline.NewNormalizer(logger)

	sig := &v1.ArgusSignal{
		SignalId: "test-signal-002",
		TraceId:  "trace-002",
		SpanId:   "span-002",
		Source: &v1.Source{
			AppId:       "test-app",
			AppVersion:  "1.0.0",
			SdkVersion:  "0.1.0",
			Environment: "test",
			InstanceId:  "instance-1",
		},
		Layer:       v1.Layer_L5_OUTPUT_DECODING,
		Category:    "decoding",
		Severity:    v1.Severity_LOW,
		Timestamp:   timestamppb.Now(),
		IngestedAt:  timestamppb.Now(),
		Provider: &v1.Provider{
			Name:  "ANTHROPIC",
			Model: "claude-3",
		},
	}

	result, err := normalizer.Process(ctx, sig)
	require.NoError(t, err)
	require.NotNil(t, result)
	assert.Equal(t, "anthropic", result.Provider.Name)
	// Verify input is not mutated
	assert.Equal(t, "ANTHROPIC", sig.Provider.Name)
}

// TestNormalizer_TimestampNormalizationToUTC tests that timestamps are converted to UTC
func TestNormalizer_TimestampNormalizationToUTC(t *testing.T) {
	ctx := context.Background()
	logger := zap.NewNop()
	normalizer := pipeline.NewNormalizer(logger)

	// Create a timestamp in a specific timezone (PST = UTC-8)
	pst, err := time.LoadLocation("America/Los_Angeles")
	require.NoError(t, err)
	localTime := time.Date(2024, 4, 9, 12, 0, 0, 0, pst)
	expectedUTC := localTime.UTC()

	sig := &v1.ArgusSignal{
		SignalId: "test-signal-003",
		TraceId:  "trace-003",
		SpanId:   "span-003",
		Source: &v1.Source{
			AppId:       "test-app",
			AppVersion:  "1.0.0",
			SdkVersion:  "0.1.0",
			Environment: "test",
			InstanceId:  "instance-1",
		},
		Layer:      v1.Layer_L7_RAG_RETRIEVAL,
		Category:   "retrieval",
		Severity:   v1.Severity_MEDIUM,
		Timestamp:  timestamppb.New(localTime),
		IngestedAt: timestamppb.Now(),
	}

	result, err := normalizer.Process(ctx, sig)
	require.NoError(t, err)
	require.NotNil(t, result)
	assert.Equal(t, expectedUTC, result.Timestamp.AsTime())
	// Verify input is not mutated
	assert.Equal(t, localTime, sig.Timestamp.AsTime())
}

// TestNormalizer_DataClassificationValidation tests valid data_classification values
func TestNormalizer_DataClassificationValidation(t *testing.T) {
	testCases := []struct {
		name               string
		classification     v1.DataClassification
		shouldPass         bool
		expectedErrorMsg   string
	}{
		{
			name:           "CLASSIFICATION_UNSPECIFIED",
			classification: v1.DataClassification_CLASSIFICATION_UNSPECIFIED,
			shouldPass:     true,
		},
		{
			name:           "PUBLIC",
			classification: v1.DataClassification_PUBLIC,
			shouldPass:     true,
		},
		{
			name:           "INTERNAL",
			classification: v1.DataClassification_INTERNAL,
			shouldPass:     true,
		},
		{
			name:           "CONFIDENTIAL",
			classification: v1.DataClassification_CONFIDENTIAL,
			shouldPass:     true,
		},
		{
			name:           "RESTRICTED",
			classification: v1.DataClassification_RESTRICTED,
			shouldPass:     true,
		},
		{
			name:             "Invalid enum value",
			classification:   v1.DataClassification(99),
			shouldPass:       false,
			expectedErrorMsg: "invalid data_classification enum value",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			ctx := context.Background()
			logger := zap.NewNop()
			normalizer := pipeline.NewNormalizer(logger)

			sig := &v1.ArgusSignal{
				SignalId:            "test-signal-004",
				TraceId:             "trace-004",
				SpanId:              "span-004",
				Source: &v1.Source{
					AppId:       "test-app",
					AppVersion:  "1.0.0",
					SdkVersion:  "0.1.0",
					Environment: "test",
					InstanceId:  "instance-1",
				},
				Layer:               v1.Layer_L8_AGENTS,
				Category:            "agent-call",
				Severity:            v1.Severity_HIGH,
				Timestamp:           timestamppb.Now(),
				IngestedAt:          timestamppb.Now(),
				DataClassification: tc.classification,
			}

			result, err := normalizer.Process(ctx, sig)
			if tc.shouldPass {
				assert.NoError(t, err)
				assert.NotNil(t, result)
				assert.Equal(t, tc.classification, result.DataClassification)
			} else {
				assert.Error(t, err)
				assert.Nil(t, result)
				assert.Contains(t, err.Error(), tc.expectedErrorMsg)
			}
		})
	}
}

// TestNormalizer_LayerEnumValidation tests that layer enum values are validated
func TestNormalizer_LayerEnumValidation(t *testing.T) {
	testCases := []struct {
		name              string
		layer             v1.Layer
		shouldPass        bool
		expectedErrorMsg  string
	}{
		{
			name:       "L1_HARDWARE",
			layer:      v1.Layer_L1_HARDWARE,
			shouldPass: true,
		},
		{
			name:       "L2_MODEL_WEIGHTS",
			layer:      v1.Layer_L2_MODEL_WEIGHTS,
			shouldPass: true,
		},
		{
			name:       "L3_TOKENIZER",
			layer:      v1.Layer_L3_TOKENIZER,
			shouldPass: true,
		},
		{
			name:       "L4_TRANSFORMER",
			layer:      v1.Layer_L4_TRANSFORMER,
			shouldPass: true,
		},
		{
			name:       "L5_OUTPUT_DECODING",
			layer:      v1.Layer_L5_OUTPUT_DECODING,
			shouldPass: true,
		},
		{
			name:       "L6_SAFETY",
			layer:      v1.Layer_L6_SAFETY,
			shouldPass: true,
		},
		{
			name:       "L7_RAG_RETRIEVAL",
			layer:      v1.Layer_L7_RAG_RETRIEVAL,
			shouldPass: true,
		},
		{
			name:       "L8_AGENTS",
			layer:      v1.Layer_L8_AGENTS,
			shouldPass: true,
		},
		{
			name:       "L9_API_GATEWAY",
			layer:      v1.Layer_L9_API_GATEWAY,
			shouldPass: true,
		},
		{
			name:       "L10_APPLICATION",
			layer:      v1.Layer_L10_APPLICATION,
			shouldPass: true,
		},
		{
			name:              "LAYER_UNSPECIFIED",
			layer:             v1.Layer_LAYER_UNSPECIFIED,
			shouldPass:        false,
			expectedErrorMsg:  "layer must not be unspecified",
		},
		{
			name:              "Invalid enum value",
			layer:             v1.Layer(99),
			shouldPass:        false,
			expectedErrorMsg:  "invalid layer enum value",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			ctx := context.Background()
			logger := zap.NewNop()
			normalizer := pipeline.NewNormalizer(logger)

			sig := &v1.ArgusSignal{
				SignalId: "test-signal-005",
				TraceId:  "trace-005",
				SpanId:   "span-005",
				Source: &v1.Source{
					AppId:       "test-app",
					AppVersion:  "1.0.0",
					SdkVersion:  "0.1.0",
					Environment: "test",
					InstanceId:  "instance-1",
				},
				Layer:      tc.layer,
				Category:   "test",
				Severity:   v1.Severity_INFO,
				Timestamp:  timestamppb.Now(),
				IngestedAt: timestamppb.Now(),
			}

			result, err := normalizer.Process(ctx, sig)
			if tc.shouldPass {
				assert.NoError(t, err, "test case %s should pass", tc.name)
				assert.NotNil(t, result)
				assert.Equal(t, tc.layer, result.Layer)
			} else {
				assert.Error(t, err, "test case %s should fail", tc.name)
				assert.Nil(t, result)
				assert.Contains(t, err.Error(), tc.expectedErrorMsg)
			}
		})
	}
}

// TestNormalizer_NilSignalHandling tests that nil signals are handled gracefully
func TestNormalizer_NilSignalHandling(t *testing.T) {
	ctx := context.Background()
	logger := zap.NewNop()
	normalizer := pipeline.NewNormalizer(logger)

	result, err := normalizer.Process(ctx, nil)
	assert.NoError(t, err)
	assert.Nil(t, result)
}

// TestNormalizer_MissingOptionalFieldsStillPass tests that signals with missing optional fields pass
func TestNormalizer_MissingOptionalFieldsStillPass(t *testing.T) {
	ctx := context.Background()
	logger := zap.NewNop()
	normalizer := pipeline.NewNormalizer(logger)

	sig := &v1.ArgusSignal{
		SignalId: "test-signal-006",
		TraceId:  "trace-006",
		SpanId:   "span-006",
		Source: &v1.Source{
			AppId:       "test-app",
			AppVersion:  "1.0.0",
			SdkVersion:  "0.1.0",
			Environment: "test",
			InstanceId:  "instance-1",
		},
		Layer:      v1.Layer_L10_APPLICATION,
		Category:   "test",
		Severity:   v1.Severity_CRITICAL,
		Timestamp:  timestamppb.Now(),
		IngestedAt: timestamppb.Now(),
		// Missing optional fields: Provider, Enrichment, DataClassification, etc.
	}

	result, err := normalizer.Process(ctx, sig)
	require.NoError(t, err)
	require.NotNil(t, result)
	assert.Equal(t, "test-signal-006", result.SignalId)
	assert.Nil(t, result.Provider)
	assert.Nil(t, result.Enrichment)
}

// TestNormalizer_ProcessorInterfaceCompliance tests that Normalizer implements Processor interface
func TestNormalizer_ProcessorInterfaceCompliance(t *testing.T) {
	logger := zap.NewNop()
	normalizer := pipeline.NewNormalizer(logger)

	// Test that normalizer implements the Processor interface
	var _ pipeline.Processor = normalizer

	// Test Name() method
	assert.Equal(t, "Normalizer", normalizer.Name())

	// Test Process method signature
	ctx := context.Background()
	sig := &v1.ArgusSignal{
		SignalId: "test-signal-007",
		TraceId:  "trace-007",
		SpanId:   "span-007",
		Source: &v1.Source{
			AppId:       "test-app",
			AppVersion:  "1.0.0",
			SdkVersion:  "0.1.0",
			Environment: "test",
			InstanceId:  "instance-1",
		},
		Layer:      v1.Layer_L9_API_GATEWAY,
		Category:   "gateway",
		Severity:   v1.Severity_LOW,
		Timestamp:  timestamppb.Now(),
		IngestedAt: timestamppb.Now(),
	}

	result, err := normalizer.Process(ctx, sig)
	assert.NotNil(t, result)
	assert.NoError(t, err)
}

// TestNormalizer_NonMutatingBehavior tests that the input signal is not mutated
func TestNormalizer_NonMutatingBehavior(t *testing.T) {
	ctx := context.Background()
	logger := zap.NewNop()
	normalizer := pipeline.NewNormalizer(logger)

	originalProvider := "MixedCaseProvider"
	sig := &v1.ArgusSignal{
		SignalId: "test-signal-008",
		TraceId:  "trace-008",
		SpanId:   "span-008",
		Source: &v1.Source{
			AppId:       "test-app",
			AppVersion:  "1.0.0",
			SdkVersion:  "0.1.0",
			Environment: "test",
			InstanceId:  "instance-1",
		},
		Layer:      v1.Layer_L6_SAFETY,
		Category:   "safety",
		Severity:   v1.Severity_MEDIUM,
		Timestamp:  timestamppb.Now(),
		IngestedAt: timestamppb.Now(),
		Provider: &v1.Provider{
			Name:  originalProvider,
			Model: "test-model",
		},
	}

	// Store original provider name
	originalSignalProvider := sig.Provider.Name

	result, err := normalizer.Process(ctx, sig)
	require.NoError(t, err)
	require.NotNil(t, result)

	// Verify result has lowercased provider name
	assert.Equal(t, "mixedcaseprovider", result.Provider.Name)

	// Verify original signal is not mutated
	assert.Equal(t, originalSignalProvider, sig.Provider.Name)
	assert.NotEqual(t, "mixedcaseprovider", sig.Provider.Name)
}
