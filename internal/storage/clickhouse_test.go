package storage

import (
	"context"
	"reflect"
	"testing"
	"time"

	"github.com/ClickHouse/clickhouse-go/v2/lib/driver"
	v1 "github.com/argusxdr/argus/gen/go/argus/v1"
	"github.com/argusxdr/argus/internal/metrics"
	"github.com/prometheus/client_golang/prometheus"
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

	// Use a simple channel-backed mock to avoid importing ingest (import cycle)
	q := &mockDequeuer{ch: make(chan *v1.ArgusSignal, 10)}
	bw, _ := NewBatchWriter(context.Background(), mockCH, 500, 2*time.Second, storageMetrics, zap.NewNop())

	ctx, cancel := context.WithTimeout(context.Background(), 1*time.Second)
	defer cancel()

	go DrainWorker(ctx, q, bw, zap.NewNop())

	for i := 0; i < 5; i++ {
		q.ch <- &v1.ArgusSignal{
			SignalId:  string(rune(i)),
			TraceId:   "trace-1",
			Timestamp: timestamppb.Now(),
		}
	}

	<-ctx.Done()
	bw.Close()
}

// ── Mock: prometheus.Registerer ─────────────────────────────────────────────

type mockPromRegistry struct{}

func (m *mockPromRegistry) Register(c prometheus.Collector) error   { return nil }
func (m *mockPromRegistry) MustRegister(cs ...prometheus.Collector) {}
func (m *mockPromRegistry) Unregister(c prometheus.Collector) bool  { return true }

// ── Mock: driver.Conn ────────────────────────────────────────────────────────

type mockClickHouseConn struct{}

func (m *mockClickHouseConn) Contributors() []string                    { return nil }
func (m *mockClickHouseConn) ServerVersion() (*driver.ServerVersion, error) { return nil, nil }
func (m *mockClickHouseConn) Select(ctx context.Context, dest any, query string, args ...any) error {
	return nil
}
func (m *mockClickHouseConn) Query(ctx context.Context, query string, args ...any) (driver.Rows, error) {
	return &mockRows{}, nil
}
func (m *mockClickHouseConn) QueryRow(ctx context.Context, query string, args ...any) driver.Row {
	return &mockRow{}
}
func (m *mockClickHouseConn) PrepareBatch(ctx context.Context, query string, opts ...driver.PrepareBatchOption) (driver.Batch, error) {
	return &mockBatch{}, nil
}
func (m *mockClickHouseConn) Exec(ctx context.Context, query string, args ...any) error { return nil }
func (m *mockClickHouseConn) AsyncInsert(ctx context.Context, query string, wait bool, args ...any) error {
	return nil
}
func (m *mockClickHouseConn) Ping(ctx context.Context) error { return nil }
func (m *mockClickHouseConn) Stats() driver.Stats            { return driver.Stats{} }
func (m *mockClickHouseConn) Close() error                   { return nil }

// ── Mock: driver.Batch ───────────────────────────────────────────────────────

type mockBatch struct{}

func (m *mockBatch) Abort() error                         { return nil }
func (m *mockBatch) Append(v ...any) error                { return nil }
func (m *mockBatch) AppendStruct(v any) error             { return nil }
func (m *mockBatch) Column(i int) driver.BatchColumn      { return &mockBatchColumn{} }
func (m *mockBatch) Flush() error                         { return nil }
func (m *mockBatch) Send() error                          { return nil }
func (m *mockBatch) IsSent() bool                         { return false }
func (m *mockBatch) Rows() int                            { return 0 }

// ── Mock: driver.BatchColumn ─────────────────────────────────────────────────

type mockBatchColumn struct{}

func (m *mockBatchColumn) Append(v any) error    { return nil }
func (m *mockBatchColumn) AppendRow(v any) error { return nil }

// ── Mock: driver.Rows ────────────────────────────────────────────────────────

type mockRows struct{}

func (m *mockRows) Next() bool                              { return false }
func (m *mockRows) Scan(dest ...any) error                  { return nil }
func (m *mockRows) ScanStruct(dest any) error               { return nil }
func (m *mockRows) ColumnTypes() []driver.ColumnType        { return nil }
func (m *mockRows) Totals(dest ...any) error                { return nil }
func (m *mockRows) Columns() []string                       { return nil }
func (m *mockRows) Close() error                            { return nil }
func (m *mockRows) Err() error                              { return nil }

// ── Mock: driver.Row ─────────────────────────────────────────────────────────

type mockRow struct{}

func (m *mockRow) Err() error               { return nil }
func (m *mockRow) Scan(dest ...any) error   { return nil }
func (m *mockRow) ScanStruct(dest any) error { return nil }

// ── Mock: driver.ColumnType ──────────────────────────────────────────────────

type mockColumnType struct{}

func (m *mockColumnType) Name() string             { return "" }
func (m *mockColumnType) Nullable() bool           { return false }
func (m *mockColumnType) ScanType() reflect.Type   { return nil }
func (m *mockColumnType) DatabaseTypeName() string { return "" }

// ── Mock: Dequeuer ───────────────────────────────────────────────────────────
// Avoids importing internal/ingest (which imports internal/storage → cycle).

type mockDequeuer struct {
	ch chan *v1.ArgusSignal
}

func (m *mockDequeuer) Dequeue(ctx context.Context) (*v1.ArgusSignal, error) {
	select {
	case sig := <-m.ch:
		return sig, nil
	case <-ctx.Done():
		return nil, ctx.Err()
	}
}
