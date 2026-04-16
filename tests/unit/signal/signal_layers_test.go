package signal_test

import (
	"testing"

	v1 "github.com/argusxdr/argus/gen/go/argus/v1"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/proto"
)

// Helper functions for optional fields
func floatPtr(v float32) *float32 { return &v }
func boolPtr(v bool) *bool        { return &v }
func strPtr(v string) *string     { return &v }
func int32Ptr(v int32) *int32     { return &v }
func int64Ptr(v int64) *int64     { return &v }

// TestSignalLayerRoundTrip verifies that each of the 11 context layers
// survives a proto Marshal/Unmarshal round-trip with all field values intact.
func TestSignalLayerRoundTrip(t *testing.T) {
	t.Run("L1_HARDWARE", func(t *testing.T) {
		sig := &v1.ArgusSignal{
			SignalId: "test-l1-001",
			TraceId:  "trace-l1-001",
			Layer:    v1.Layer_L1_HARDWARE,
			Context: &v1.ArgusSignal_ContextL1{
				ContextL1: &v1.ContextL1{
					CpuUsagePct: floatPtr(85.5),
					GpuCount:    int32Ptr(4),
				},
			},
		}

		data, err := proto.Marshal(sig)
		require.NoError(t, err)
		require.NotEmpty(t, data)

		got := &v1.ArgusSignal{}
		err = proto.Unmarshal(data, got)
		require.NoError(t, err)

		ctx := got.GetContextL1()
		require.NotNil(t, ctx, "ContextL1 must survive round-trip")
		assert.InDelta(t, float64(85.5), float64(ctx.GetCpuUsagePct()), 0.001, "CpuUsagePct mismatch")
		assert.Equal(t, int32(4), ctx.GetGpuCount(), "GpuCount mismatch")
	})

	t.Run("L2_MODEL_WEIGHTS", func(t *testing.T) {
		sig := &v1.ArgusSignal{
			SignalId: "test-l2-001",
			TraceId:  "trace-l2-001",
			Layer:    v1.Layer_L2_MODEL_WEIGHTS,
			Context: &v1.ArgusSignal_ContextL2{
				ContextL2: &v1.ContextL2{
					ModelId:        strPtr("llama-3"),
					ParameterCount: int64Ptr(70_000_000_000),
				},
			},
		}

		data, err := proto.Marshal(sig)
		require.NoError(t, err)

		got := &v1.ArgusSignal{}
		err = proto.Unmarshal(data, got)
		require.NoError(t, err)

		ctx := got.GetContextL2()
		require.NotNil(t, ctx, "ContextL2 must survive round-trip")
		assert.Equal(t, "llama-3", ctx.GetModelId(), "ModelId mismatch")
		assert.Equal(t, int64(70_000_000_000), ctx.GetParameterCount(), "ParameterCount mismatch")
	})

	t.Run("L3_TOKENIZER", func(t *testing.T) {
		sig := &v1.ArgusSignal{
			SignalId: "test-l3-001",
			TraceId:  "trace-l3-001",
			Layer:    v1.Layer_L3_TOKENIZER,
			Context: &v1.ArgusSignal_ContextL3{
				ContextL3: &v1.ContextL3{
					InputTokens: int32Ptr(1024),
					TokenizerId: strPtr("cl100k_base"),
				},
			},
		}

		data, err := proto.Marshal(sig)
		require.NoError(t, err)

		got := &v1.ArgusSignal{}
		err = proto.Unmarshal(data, got)
		require.NoError(t, err)

		ctx := got.GetContextL3()
		require.NotNil(t, ctx, "ContextL3 must survive round-trip")
		assert.Equal(t, int32(1024), ctx.GetInputTokens(), "InputTokens mismatch")
		assert.Equal(t, "cl100k_base", ctx.GetTokenizerId(), "TokenizerId mismatch")
	})

	t.Run("L4_TRANSFORMER", func(t *testing.T) {
		sig := &v1.ArgusSignal{
			SignalId: "test-l4-001",
			TraceId:  "trace-l4-001",
			Layer:    v1.Layer_L4_TRANSFORMER,
			Context: &v1.ArgusSignal_ContextL4{
				ContextL4: &v1.ContextL4{
					KvCacheUsedMb: floatPtr(2048.0),
					FlashAttention: boolPtr(true),
				},
			},
		}

		data, err := proto.Marshal(sig)
		require.NoError(t, err)

		got := &v1.ArgusSignal{}
		err = proto.Unmarshal(data, got)
		require.NoError(t, err)

		ctx := got.GetContextL4()
		require.NotNil(t, ctx, "ContextL4 must survive round-trip")
		assert.InDelta(t, float64(2048.0), float64(ctx.GetKvCacheUsedMb()), 0.01, "KvCacheUsedMb mismatch")
		assert.Equal(t, true, ctx.GetFlashAttention(), "FlashAttention mismatch")
	})

	t.Run("L5_OUTPUT_DECODING", func(t *testing.T) {
		sig := &v1.ArgusSignal{
			SignalId: "test-l5-001",
			TraceId:  "trace-l5-001",
			Layer:    v1.Layer_L5_OUTPUT_DECODING,
			Context: &v1.ArgusSignal_ContextL5{
				ContextL5: &v1.ContextL5{
					OutputTokens: 512,
					Temperature:  0.7,
					FinishReason: "stop",
				},
			},
		}

		data, err := proto.Marshal(sig)
		require.NoError(t, err)

		got := &v1.ArgusSignal{}
		err = proto.Unmarshal(data, got)
		require.NoError(t, err)

		ctx := got.GetContextL5()
		require.NotNil(t, ctx, "ContextL5 must survive round-trip")
		assert.Equal(t, int32(512), ctx.GetOutputTokens(), "OutputTokens mismatch")
		assert.InDelta(t, float64(0.7), float64(ctx.GetTemperature()), 0.001, "Temperature mismatch")
		assert.Equal(t, "stop", ctx.GetFinishReason(), "FinishReason mismatch")
	})

	t.Run("L6_SAFETY", func(t *testing.T) {
		sig := &v1.ArgusSignal{
			SignalId: "test-l6-001",
			TraceId:  "trace-l6-001",
			Layer:    v1.Layer_L6_SAFETY,
			Context: &v1.ArgusSignal_ContextL6{
				ContextL6: &v1.ContextL6{
					SafetyScore:       floatPtr(0.95),
					Safe:              boolPtr(true),
					JailbreakDetected: boolPtr(false),
				},
			},
		}

		data, err := proto.Marshal(sig)
		require.NoError(t, err)

		got := &v1.ArgusSignal{}
		err = proto.Unmarshal(data, got)
		require.NoError(t, err)

		ctx := got.GetContextL6()
		require.NotNil(t, ctx, "ContextL6 must survive round-trip")
		assert.InDelta(t, float64(0.95), float64(ctx.GetSafetyScore()), 0.001, "SafetyScore mismatch")
		assert.Equal(t, true, ctx.GetSafe(), "Safe mismatch")
		assert.Equal(t, false, ctx.GetJailbreakDetected(), "JailbreakDetected mismatch")
	})

	t.Run("L7_RAG_RETRIEVAL", func(t *testing.T) {
		sig := &v1.ArgusSignal{
			SignalId: "test-l7-001",
			TraceId:  "trace-l7-001",
			Layer:    v1.Layer_L7_RAG_RETRIEVAL,
			Context: &v1.ArgusSignal_ContextL7{
				ContextL7: &v1.ContextL7{
					Operation:      v1.ContextL7_VECTOR_SEARCH,
					ResultsCount:   10,
					EmbeddingModel: "ada-002",
				},
			},
		}

		data, err := proto.Marshal(sig)
		require.NoError(t, err)

		got := &v1.ArgusSignal{}
		err = proto.Unmarshal(data, got)
		require.NoError(t, err)

		ctx := got.GetContextL7()
		require.NotNil(t, ctx, "ContextL7 must survive round-trip")
		assert.Equal(t, v1.ContextL7_VECTOR_SEARCH, ctx.GetOperation(), "Operation mismatch")
		assert.Equal(t, int32(10), ctx.GetResultsCount(), "ResultsCount mismatch")
		assert.Equal(t, "ada-002", ctx.GetEmbeddingModel(), "EmbeddingModel mismatch")
	})

	t.Run("L8_AGENTS", func(t *testing.T) {
		sig := &v1.ArgusSignal{
			SignalId: "test-l8-001",
			TraceId:  "trace-l8-001",
			Layer:    v1.Layer_L8_AGENTS,
			Context: &v1.ArgusSignal_ContextL8{
				ContextL8: &v1.ContextL8{
					Operation:     v1.ContextL8_TOOL_CALL,
					ToolName:      "web_search",
					ToolLatencyMs: 150.0,
				},
			},
		}

		data, err := proto.Marshal(sig)
		require.NoError(t, err)

		got := &v1.ArgusSignal{}
		err = proto.Unmarshal(data, got)
		require.NoError(t, err)

		ctx := got.GetContextL8()
		require.NotNil(t, ctx, "ContextL8 must survive round-trip")
		assert.Equal(t, v1.ContextL8_TOOL_CALL, ctx.GetOperation(), "Operation mismatch")
		assert.Equal(t, "web_search", ctx.GetToolName(), "ToolName mismatch")
		assert.InDelta(t, float64(150.0), float64(ctx.GetToolLatencyMs()), 0.01, "ToolLatencyMs mismatch")
	})

	t.Run("L9_API_GATEWAY", func(t *testing.T) {
		sig := &v1.ArgusSignal{
			SignalId: "test-l9-001",
			TraceId:  "trace-l9-001",
			Layer:    v1.Layer_L9_API_GATEWAY,
			Context: &v1.ArgusSignal_ContextL9{
				ContextL9: &v1.ContextL9{
					Method:     strPtr("POST"),
					StatusCode: int32Ptr(200),
					LatencyMs:  floatPtr(45.0),
				},
			},
		}

		data, err := proto.Marshal(sig)
		require.NoError(t, err)

		got := &v1.ArgusSignal{}
		err = proto.Unmarshal(data, got)
		require.NoError(t, err)

		ctx := got.GetContextL9()
		require.NotNil(t, ctx, "ContextL9 must survive round-trip")
		assert.Equal(t, "POST", ctx.GetMethod(), "Method mismatch")
		assert.Equal(t, int32(200), ctx.GetStatusCode(), "StatusCode mismatch")
		assert.InDelta(t, float64(45.0), float64(ctx.GetLatencyMs()), 0.01, "LatencyMs mismatch")
	})

	t.Run("L10_APPLICATION", func(t *testing.T) {
		sig := &v1.ArgusSignal{
			SignalId: "test-l10-001",
			TraceId:  "trace-l10-001",
			Layer:    v1.Layer_L10_APPLICATION,
			Context: &v1.ArgusSignal_ContextL10{
				ContextL10: &v1.ContextL10{
					EventType: strPtr("chat_submit"),
					Platform:  strPtr("web"),
				},
			},
		}

		data, err := proto.Marshal(sig)
		require.NoError(t, err)

		got := &v1.ArgusSignal{}
		err = proto.Unmarshal(data, got)
		require.NoError(t, err)

		ctx := got.GetContextL10()
		require.NotNil(t, ctx, "ContextL10 must survive round-trip")
		assert.Equal(t, "chat_submit", ctx.GetEventType(), "EventType mismatch")
		assert.Equal(t, "web", ctx.GetPlatform(), "Platform mismatch")
	})

	t.Run("L_DECISION", func(t *testing.T) {
		sig := &v1.ArgusSignal{
			SignalId: "test-ldecision-001",
			TraceId:  "trace-ldecision-001",
			Layer:    v1.Layer_L_DECISION,
			Context: &v1.ArgusSignal_ContextLDecision{
				ContextLDecision: &v1.ContextLDecision{
					Decision:   v1.ContextLDecision_ALLOW,
					Confidence: 0.99,
					PolicyName: "default",
				},
			},
		}

		data, err := proto.Marshal(sig)
		require.NoError(t, err)

		got := &v1.ArgusSignal{}
		err = proto.Unmarshal(data, got)
		require.NoError(t, err)

		ctx := got.GetContextLDecision()
		require.NotNil(t, ctx, "ContextLDecision must survive round-trip")
		assert.Equal(t, v1.ContextLDecision_ALLOW, ctx.GetDecision(), "Decision mismatch")
		assert.InDelta(t, float64(0.99), float64(ctx.GetConfidence()), 0.001, "Confidence mismatch")
		assert.Equal(t, "default", ctx.GetPolicyName(), "PolicyName mismatch")
	})
}
