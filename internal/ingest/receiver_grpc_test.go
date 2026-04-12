package ingest

import (
	"context"
	"io"
	"testing"

	"github.com/argusxdr/argus/gen/go/argus/v1"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
	"google.golang.org/grpc/metadata"
	"google.golang.org/protobuf/types/known/timestamppb"
)

func TestNewGRPCReceiver(t *testing.T) {
	queue := NewQueue(100, nil)
	receiver := NewGRPCReceiver(queue, nil, nil)

	assert.NotNil(t, receiver)
	assert.Equal(t, queue, receiver.queue)
}

func TestGRPCReceiver_StreamSignals_EmptyStream(t *testing.T) {
	queue := NewQueue(100, nil)
	receiver := NewGRPCReceiver(queue, nil, zap.NewNop())

	// Create a mock stream that closes immediately (EOF on first recv)
	mockStream := &mockIngestStream{
		signals: nil, // No signals
	}

	err := receiver.StreamSignals(mockStream)
	assert.NoError(t, err)

	// Verify response was sent
	assert.NotNil(t, mockStream.response)
	assert.Equal(t, int32(0), mockStream.response.AcceptedCount)
	assert.Equal(t, int32(0), mockStream.response.RejectedCount)
}

func TestGRPCReceiver_StreamSignals_ValidSignals(t *testing.T) {
	queue := NewQueue(100, nil)
	receiver := NewGRPCReceiver(queue, nil, zap.NewNop())

	// Create test signals
	signals := []*v1.ArgusSignal{
		{
			SignalId: "signal-1",
			TraceId:  "trace-1",
			SpanId:   "span-1",
			Layer:    v1.Layer_L5_OUTPUT_DECODING,
			Category: "output.generation",
			Severity: v1.Severity_INFO,
			Source: &v1.Source{
				AppId:      "app-1",
				AppVersion: "1.0.0",
				Environment: "prod",
			},
			Timestamp: timestamppb.Now(),
		},
		{
			SignalId: "signal-2",
			TraceId:  "trace-1",
			SpanId:   "span-2",
			Layer:    v1.Layer_L7_RAG_RETRIEVAL,
			Category: "retrieval.search",
			Severity: v1.Severity_LOW,
			Source: &v1.Source{
				AppId:      "app-1",
				AppVersion: "1.0.0",
			},
			Timestamp: timestamppb.Now(),
		},
	}

	mockStream := &mockIngestStream{
		signals: signals,
	}

	err := receiver.StreamSignals(mockStream)
	assert.NoError(t, err)

	// Verify all signals were accepted
	assert.NotNil(t, mockStream.response)
	assert.Equal(t, int32(2), mockStream.response.AcceptedCount)
	assert.Equal(t, int32(0), mockStream.response.RejectedCount)
	assert.Empty(t, mockStream.response.Rejections)

	// Verify signals are in the queue
	assert.Equal(t, 2, queue.Depth())
	accepted, dropped := queue.Stats()
	assert.Equal(t, int64(2), accepted)
	assert.Equal(t, int64(0), dropped)
}

func TestGRPCReceiver_StreamSignals_InvalidSignals(t *testing.T) {
	queue := NewQueue(100, nil)
	receiver := NewGRPCReceiver(queue, nil, zap.NewNop())

	// Create signals with missing required fields
	signals := []*v1.ArgusSignal{
		{
			// Missing signal_id
			TraceId: "trace-1",
			SpanId:  "span-1",
		},
		{
			SignalId: "signal-2",
			// Missing trace_id
			SpanId: "span-2",
		},
		{
			SignalId: "signal-3",
			TraceId:  "trace-3",
			SpanId:   "span-3",
			Layer:    v1.Layer_L5_OUTPUT_DECODING,
			Timestamp: timestamppb.Now(),
		},
	}

	mockStream := &mockIngestStream{
		signals: signals,
	}

	err := receiver.StreamSignals(mockStream)
	assert.NoError(t, err)

	// Verify rejection details
	assert.NotNil(t, mockStream.response)
	assert.Equal(t, int32(1), mockStream.response.AcceptedCount)
	assert.Equal(t, int32(2), mockStream.response.RejectedCount)
	assert.Len(t, mockStream.response.Rejections, 2)

	// Verify rejection reasons
	assert.Equal(t, "signal_id is empty", mockStream.response.Rejections[0].Reason)
	assert.Equal(t, "trace_id is empty", mockStream.response.Rejections[1].Reason)
}

func TestGRPCReceiver_StreamSignals_QueueFull(t *testing.T) {
	// Create queue with small capacity
	queueCapacity := 5
	queue := NewQueue(queueCapacity, nil)
	receiver := NewGRPCReceiver(queue, nil, zap.NewNop())

	// Create more signals than queue capacity
	signals := make([]*v1.ArgusSignal, queueCapacity+2)
	for i := 0; i < len(signals); i++ {
		signals[i] = &v1.ArgusSignal{
			SignalId:  string(rune(i)),
			TraceId:   "trace-1",
			SpanId:    "span-1",
			Timestamp: timestamppb.Now(),
		}
	}

	// Pre-fill queue to capacity
	for i := 0; i < queueCapacity; i++ {
		queue.Enqueue(signals[i])
	}

	mockStream := &mockIngestStream{
		signals: signals[queueCapacity:], // Only the overflow signals
	}

	err := receiver.StreamSignals(mockStream)
	assert.Error(t, err)
	// Should return ResourceExhausted status
}

func TestGRPCReceiver_StreamSignals_NilSignal(t *testing.T) {
	queue := NewQueue(100, nil)
	receiver := NewGRPCReceiver(queue, nil, zap.NewNop())

	// Stream with nil signal followed by valid signal
	mockStream := &mockIngestStream{
		signals: []*v1.ArgusSignal{
			nil, // Nil signal
			{
				SignalId:  "signal-1",
				TraceId:   "trace-1",
				SpanId:    "span-1",
				Timestamp: timestamppb.Now(),
			},
		},
	}

	err := receiver.StreamSignals(mockStream)
	assert.NoError(t, err)

	// Nil should be rejected, valid signal accepted
	assert.NotNil(t, mockStream.response)
	assert.Equal(t, int32(1), mockStream.response.AcceptedCount)
	assert.Equal(t, int32(1), mockStream.response.RejectedCount)
}

func TestGRPCReceiver_StreamSignals_IngestedAtSet(t *testing.T) {
	queue := NewQueue(100, nil)
	receiver := NewGRPCReceiver(queue, nil, zap.NewNop())

	// Create signal without ingested_at
	signal := &v1.ArgusSignal{
		SignalId:  "signal-1",
		TraceId:   "trace-1",
		SpanId:    "span-1",
		Timestamp: timestamppb.Now(),
		IngestedAt: nil, // Not set
	}

	mockStream := &mockIngestStream{
		signals: []*v1.ArgusSignal{signal},
	}

	err := receiver.StreamSignals(mockStream)
	assert.NoError(t, err)

	// Dequeue the signal and verify ingested_at was set
	sig, err := queue.Dequeue(context.Background())
	require.NoError(t, err)
	assert.NotNil(t, sig.IngestedAt)
}

// Mock IngestService_StreamSignalsServer for testing
type mockIngestStream struct {
	signals   []*v1.ArgusSignal
	index     int
	response  *v1.IngestResponse
	sendError error
}

func (m *mockIngestStream) Recv() (*v1.ArgusSignal, error) {
	if m.index >= len(m.signals) {
		return nil, io.EOF
	}
	sig := m.signals[m.index]
	m.index++
	return sig, nil
}

func (m *mockIngestStream) SendAndClose(resp *v1.IngestResponse) error {
	m.response = resp
	return m.sendError
}

func (m *mockIngestStream) SetHeader(metadata.MD) error {
	return nil
}

func (m *mockIngestStream) SendHeader(metadata.MD) error {
	return nil
}

func (m *mockIngestStream) SetTrailer(metadata.MD) {
}

func (m *mockIngestStream) Context() context.Context {
	ctx := context.Background()
	ctx = context.WithValue(ctx, appIDKey, "test-app")
	return ctx
}

func (m *mockIngestStream) SendMsg(interface{}) error {
	return nil
}

func (m *mockIngestStream) RecvMsg(interface{}) error {
	return nil
}

// Verify UnimplementedIngestServiceServer embedding
var _ v1.IngestServiceServer = (*GRPCReceiver)(nil)
