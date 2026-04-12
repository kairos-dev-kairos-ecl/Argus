package storage

import (
	"context"
	"testing"
	"time"

	"github.com/argusxdr/argus/gen/go/argus/v1"
	"github.com/argusxdr/argus/internal/metrics"
	"github.com/stretchr/testify/assert"
	"go.uber.org/zap"
	"google.golang.org/protobuf/types/known/timestamppb"
)

func TestNewBatchWriter(t *testing.T) {
	mockCH := &ClickHouse{conn: nil}
	reg := &mockPromRegistry{}
	storageMetrics := metrics.NewStorage(reg)

	bw, err := NewBatchWriter(context.Background(), mockCH, 500, 2*time.Second, storageMetrics, zap.NewNop())

	assert.NoError(t, err)
	assert.NotNil(t, bw)
	assert.Equal(t, 500, bw.batchSize)
	assert.Equal(t, 2*time.Second, bw.flushInterval)

	bw.Close()
}

func TestBatchWriter_DefaultValues(t *testing.T) {
	mockCH := &ClickHouse{conn: nil}
	reg := &mockPromRegistry{}
	storageMetrics := metrics.NewStorage(reg)

	// Test with zero batch size (should default to 500)
	bw, err := NewBatchWriter(context.Background(), mockCH, 0, 0, storageMetrics, nil)

	assert.NoError(t, err)
	assert.Equal(t, 500, bw.batchSize)
	assert.Equal(t, 2*time.Second, bw.flushInterval)

	bw.Close()
}

func TestBatchWriter_Write_NilSignal(t *testing.T) {
	mockCH := &ClickHouse{conn: &mockClickHouseConn{}}
	reg := &mockPromRegistry{}
	storageMetrics := metrics.NewStorage(reg)

	bw, _ := NewBatchWriter(context.Background(), mockCH, 500, 2*time.Second, storageMetrics, zap.NewNop())
	defer bw.Close()

	err := bw.Write(context.Background(), nil)
	assert.Error(t, err)
	assert.Equal(t, "signal cannot be nil", err.Error())
}

func TestBatchWriter_Write_Closed(t *testing.T) {
	mockCH := &ClickHouse{conn: &mockClickHouseConn{}}
	reg := &mockPromRegistry{}
	storageMetrics := metrics.NewStorage(reg)

	bw, _ := NewBatchWriter(context.Background(), mockCH, 500, 2*time.Second, storageMetrics, zap.NewNop())
	bw.Close()

	signal := &v1.ArgusSignal{
		SignalId:  "signal-1",
		TraceId:   "trace-1",
		Timestamp: timestamppb.Now(),
	}

	err := bw.Write(context.Background(), signal)
	assert.Error(t, err)
}

func TestBatchWriter_Write_Accumulation(t *testing.T) {
	mockCH := &ClickHouse{conn: &mockClickHouseConn{}}
	reg := &mockPromRegistry{}
	storageMetrics := metrics.NewStorage(reg)

	bw, _ := NewBatchWriter(context.Background(), mockCH, 100, 10*time.Second, storageMetrics, zap.NewNop())
	defer bw.Close()

	// Add signals up to batch size
	for i := 0; i < 50; i++ {
		signal := &v1.ArgusSignal{
			SignalId:  string(rune(i)),
			TraceId:   "trace-1",
			Timestamp: timestamppb.Now(),
		}
		err := bw.Write(context.Background(), signal)
		assert.NoError(t, err)
	}

	// Verify buffer size
	bw.mu.Lock()
	assert.Equal(t, 50, len(bw.buffer))
	bw.mu.Unlock()
}

func TestBatchWriter_Close_FlushesBuffer(t *testing.T) {
	mockCH := &ClickHouse{conn: &mockClickHouseConn{}}
	reg := &mockPromRegistry{}
	storageMetrics := metrics.NewStorage(reg)

	bw, _ := NewBatchWriter(context.Background(), mockCH, 500, 2*time.Second, storageMetrics, zap.NewNop())

	// Add a signal
	signal := &v1.ArgusSignal{
		SignalId:  "signal-1",
		TraceId:   "trace-1",
		Timestamp: timestamppb.Now(),
	}
	bw.Write(context.Background(), signal)

	// Verify buffer has signal
	bw.mu.Lock()
	assert.Equal(t, 1, len(bw.buffer))
	bw.mu.Unlock()

	// Close should flush
	bw.Close()

	// Verify buffer is cleared
	bw.mu.Lock()
	assert.Equal(t, 0, len(bw.buffer))
	bw.mu.Unlock()
}

func TestDrainWorker_BasicFlow(t *testing.T) {
	mockCH := &ClickHouse{conn: &mockClickHouseConn{}}
	reg := &mockPromRegistry{}
	storageMetrics := metrics.NewStorage(reg)
	ingestMetrics := metrics.NewIngest(reg)

	queue := NewQueue(100, ingestMetrics)
	bw, _ := NewBatchWriter(context.Background(), mockCH, 500, 2*time.Second, storageMetrics, zap.NewNop())

	ctx, cancel := context.WithTimeout(context.Background(), 1*time.Second)
	defer cancel()

	// Run drain worker in background
	go DrainWorker(ctx, queue, bw, zap.NewNop())

	// Enqueue some signals
	for i := 0; i < 5; i++ {
		signal := &v1.ArgusSignal{
			SignalId:  string(rune(i)),
			TraceId:   "trace-1",
			Timestamp: timestamppb.Now(),
		}
		queue.Enqueue(signal)
	}

	// Wait for context to cancel
	<-ctx.Done()

	bw.Close()
}

// Mock objects for testing

type mockPromRegistry struct{}

func (m *mockPromRegistry) Register(collector interface{}) error {
	return nil
}

func (m *mockPromRegistry) MustRegister(collectors ...interface{}) {
}

func (m *mockPromRegistry) Unregister(collector interface{}) bool {
	return true
}

type mockClickHouseConn struct{}

func (m *mockClickHouseConn) Ping(ctx context.Context) error {
	return nil
}

func (m *mockClickHouseConn) Exec(ctx context.Context, query string, args ...interface{}) error {
	return nil
}

func (m *mockClickHouseConn) Query(ctx context.Context, query string, args ...interface{}) (interface{}, error) {
	return nil, nil
}

func (m *mockClickHouseConn) QueryRow(ctx context.Context, query string, args ...interface{}) (interface{}, error) {
	return nil, nil
}

func (m *mockClickHouseConn) PrepareBatch(ctx context.Context, query string) (interface{}, error) {
	return &mockBatch{}, nil
}

func (m *mockClickHouseConn) Close() error {
	return nil
}

type mockBatch struct{}

func (m *mockBatch) Append(v ...interface{}) error {
	return nil
}

func (m *mockBatch) AppendMap(map[string]interface{}) error {
	return nil
}

func (m *mockBatch) Send() error {
	return nil
}

func (m *mockBatch) AbortStream() error {
	return nil
}
