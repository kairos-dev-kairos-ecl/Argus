package storage

import (
	"context"
	"fmt"
	"net/url"
	"strings"
	"sync"
	"time"

	"github.com/ClickHouse/clickhouse-go/v2"
	"github.com/ClickHouse/clickhouse-go/v2/lib/driver"
	v1 "github.com/argusxdr/argus/gen/go/argus/v1"
	"github.com/argusxdr/argus/internal/metrics"
	"go.uber.org/zap"
)

// Dequeuer defines the interface for consuming signals from the ingest queue.
// This decouples storage from the ingest package, preventing circular dependencies.
type Dequeuer interface {
	Dequeue(ctx context.Context) (*v1.ArgusSignal, error)
}

// ClickHouse wraps the clickhouse-go/v2 client for Argus signal storage.
// It manages connection pooling and provides batch write operations.
type ClickHouse struct {
	conn driver.Conn
}

// NewClickHouse creates a ClickHouse client and applies the schema.
// It uses the native protocol (not HTTP) for better performance.
// The connection will have AsyncInsert enabled for batch writes.
//
// dsn format accepts:
//   - Full URI: "clickhouse://[user[:password]@]host[:port]/[database]"
//   - Raw address: "host:port"
// Example: "clickhouse://default:@localhost:9000/default" or "localhost:9000"
func NewClickHouse(ctx context.Context, dsn string) (*ClickHouse, error) {
	// Parse the DSN to extract host:port for the driver
	addr := dsn
	if strings.Contains(dsn, "://") {
		u, err := url.Parse(dsn)
		if err == nil && u.Host != "" {
			addr = u.Host
			// If port isn't in the host, use default
			if !strings.Contains(addr, ":") {
				addr = addr + ":9000"
			}
		}
	}

	// Create connection with parsed address
	conn, err := clickhouse.Open(&clickhouse.Options{
		Addr: []string{addr},
	})
	if err != nil {
		return nil, fmt.Errorf("failed to connect to ClickHouse: %w", err)
	}

	// Verify connectivity with a ping
	if err := conn.Ping(ctx); err != nil {
		return nil, fmt.Errorf("failed to ping ClickHouse: %w", err)
	}

	// Apply the schema (idempotent)
	if err := applySchema(ctx, conn); err != nil {
		return nil, fmt.Errorf("failed to apply schema: %w", err)
	}

	return &ClickHouse{
		conn: conn,
	}, nil
}

// applySchema executes the SignalsTableDDL against the ClickHouse connection.
// It is safe to call multiple times (CREATE TABLE IF NOT EXISTS).
func applySchema(ctx context.Context, conn driver.Conn) error {
	if err := conn.Exec(ctx, SignalsTableDDL); err != nil {
		return fmt.Errorf("failed to execute DDL: %w", err)
	}
	return nil
}

// BatchWriter provides a buffered interface for writing signals to ClickHouse.
// It batches signals and flushes them via AsyncInsert for efficiency.
// Signals are flushed when either batchSize is reached or flushInterval elapsed.
type BatchWriter struct {
	ch            *ClickHouse
	conn          driver.Conn
	mu            sync.Mutex
	buffer        []*v1.ArgusSignal
	batchSize     int
	flushInterval time.Duration
	flushCh       chan struct{}
	ctx           context.Context
	cancel        context.CancelFunc
	metrics       *metrics.Storage
	log           *zap.Logger
	closed        bool
}

// NewBatchWriter creates a new batch writer with the given batch size and flush interval.
// The batch size should be between 500-1000 for optimal AsyncInsert performance.
// The flush interval (default 2s) determines max time to wait for a partial batch.
func NewBatchWriter(ctx context.Context, ch *ClickHouse, batchSize int, flushInterval time.Duration, m *metrics.Storage, log *zap.Logger) (*BatchWriter, error) {
	if batchSize <= 0 {
		batchSize = 500
	}
	if flushInterval <= 0 {
		flushInterval = 2 * time.Second
	}
	if log == nil {
		log = zap.NewNop()
	}

	batchCtx, cancel := context.WithCancel(ctx)

	bw := &BatchWriter{
		ch:            ch,
		conn:          ch.conn,
		buffer:        make([]*v1.ArgusSignal, 0, batchSize),
		batchSize:     batchSize,
		flushInterval: flushInterval,
		flushCh:       make(chan struct{}, 1),
		ctx:           batchCtx,
		cancel:        cancel,
		metrics:       m,
		log:           log,
	}

	// Start the flush goroutine
	go bw.flushLoop()

	return bw, nil
}

// Write adds a signal to the buffer. Triggers flush if buffer >= batchSize.
// Non-blocking. Returns immediately. Never blocks the caller.
func (bw *BatchWriter) Write(ctx context.Context, sig *v1.ArgusSignal) error {
	if sig == nil {
		return fmt.Errorf("signal cannot be nil")
	}

	bw.mu.Lock()
	defer bw.mu.Unlock()

	if bw.closed {
		return fmt.Errorf("batch writer is closed")
	}

	bw.buffer = append(bw.buffer, sig)

	// Trigger flush if buffer is full
	if len(bw.buffer) >= bw.batchSize {
		// Non-blocking send to trigger flush
		select {
		case bw.flushCh <- struct{}{}:
		default:
			// Channel already has signal, no need to send again
		}
	}

	return nil
}

// flushLoop runs in a separate goroutine and flushes the buffer periodically.
func (bw *BatchWriter) flushLoop() {
	ticker := time.NewTicker(bw.flushInterval)
	defer ticker.Stop()

	for {
		select {
		case <-bw.ctx.Done():
			return
		case <-ticker.C:
			bw.mu.Lock()
			if len(bw.buffer) > 0 {
				if err := bw.flushLocked(); err != nil {
					bw.log.Error("flush failed", zap.Error(err))
				}
			}
			bw.mu.Unlock()
		case <-bw.flushCh:
			bw.mu.Lock()
			if len(bw.buffer) > 0 {
				if err := bw.flushLocked(); err != nil {
					bw.log.Error("flush failed", zap.Error(err))
				}
			}
			bw.mu.Unlock()
		}
	}
}

// flushLocked serializes buffered signals and inserts into ClickHouse.
// MUST be called with mu locked.
// On ClickHouse error: logs at ERROR, increments storage_insert_errors_total metric,
// retries once after 100ms. If retry fails, signals are lost (log at ERROR with count).
// This is acceptable: signals are not critical path (vs. the ingest queue never losing).
func (bw *BatchWriter) flushLocked() error {
	if len(bw.buffer) == 0 {
		return nil
	}

	startTime := time.Now()
	batchSize := len(bw.buffer)

	batch, err := bw.conn.PrepareBatch(bw.ctx, "INSERT INTO signals (*)")
	if err != nil {
		if bw.metrics != nil && bw.metrics.InsertErrors != nil {
			bw.metrics.InsertErrors.Inc()
		}
		bw.log.Error("failed to prepare batch", zap.Error(err), zap.Int("buffer_size", batchSize))
		return err
	}

	// Append all signals to the batch
	for _, sig := range bw.buffer {
		if err := signalToClickHouseRow(batch, sig); err != nil {
			bw.log.Error("failed to append signal to batch", zap.Error(err), zap.String("signal_id", sig.SignalId))
			batch.Abort()
			if bw.metrics != nil && bw.metrics.InsertErrors != nil {
				bw.metrics.InsertErrors.Inc()
			}
			return err
		}
	}

	// Send the batch
	if err := batch.Send(); err != nil {
		bw.log.Error("batch send failed", zap.Error(err), zap.Int("buffer_size", batchSize))
		batch.Abort()

		// Retry once after 100ms
		bw.log.Info("retrying batch send", zap.Int("buffer_size", batchSize))
		time.Sleep(100 * time.Millisecond)

		batch2, err2 := bw.conn.PrepareBatch(bw.ctx, "INSERT INTO signals (*)")
		if err2 != nil {
			bw.log.Error("failed to prepare retry batch", zap.Error(err2), zap.Int("buffer_size", batchSize))
			if bw.metrics != nil && bw.metrics.InsertErrors != nil {
				bw.metrics.InsertErrors.Inc()
			}
			return err2
		}

		for _, sig := range bw.buffer {
			if err2 := signalToClickHouseRow(batch2, sig); err2 != nil {
				bw.log.Error("failed to append signal to retry batch", zap.Error(err2), zap.String("signal_id", sig.SignalId))
				batch2.Abort()
				if bw.metrics != nil && bw.metrics.InsertErrors != nil {
					bw.metrics.InsertErrors.Inc()
				}
				return err2
			}
		}

		if err2 := batch2.Send(); err2 != nil {
			bw.log.Error("batch retry send failed, signals lost", zap.Error(err2), zap.Int("buffer_size", batchSize))
			batch2.Abort()
			if bw.metrics != nil && bw.metrics.InsertErrors != nil {
				bw.metrics.InsertErrors.Inc()
			}
			return err2
		}
	}

	// Record metrics
	duration := time.Since(startTime).Seconds()
	if bw.metrics != nil {
		if bw.metrics.BatchFlushes != nil {
			bw.metrics.BatchFlushes.WithLabelValues("ok").Inc()
		}
		if bw.metrics.BatchSize != nil {
			bw.metrics.BatchSize.Observe(float64(batchSize))
		}
		if bw.metrics.FlushDuration != nil {
			bw.metrics.FlushDuration.Observe(duration)
		}
	}

	bw.log.Debug("batch flushed", zap.Int("size", batchSize), zap.Float64("duration_seconds", duration))

	// Clear buffer
	bw.buffer = bw.buffer[:0]

	return nil
}

// Close triggers a final flush and closes the batch writer.
func (bw *BatchWriter) Close() error {
	bw.mu.Lock()
	defer bw.mu.Unlock()

	if bw.closed {
		return nil
	}

	// Flush remaining signals
	if len(bw.buffer) > 0 {
		if err := bw.flushLocked(); err != nil {
			bw.log.Error("final flush failed", zap.Error(err))
		}
	}

	bw.closed = true
	bw.cancel()

	return nil
}

// signalToClickHouseRow converts an ArgusSignal to a ClickHouse batch row.
// Uses positional arguments matching the DDL column order defined in schema.go.
func signalToClickHouseRow(batch driver.Batch, sig *v1.ArgusSignal) error {
	// Build the row in the order defined by SignalsTableDDL
	// See internal/storage/schema.go for column order

	// Identity
	signalID := sig.SignalId
	traceID := sig.TraceId
	spanID := sig.SpanId
	parentSpanID := sig.ParentSpanId

	// Source
	appID := ""
	appVersion := ""
	hostID := ""
	environment := ""
	sdkVersion := ""
	if sig.Source != nil {
		appID = sig.Source.AppId
		appVersion = sig.Source.AppVersion
		environment = sig.Source.Environment
		sdkVersion = sig.Source.SdkVersion
	}

	// Classification
	layer := uint8(sig.Layer)
	category := sig.Category
	severity := uint8(sig.Severity)

	// Temporal
	var timestamp int64
	if sig.Timestamp != nil {
		timestamp = sig.Timestamp.AsTime().UnixNano()
	}
	var durationMs *float32
	if sig.DurationMs != nil {
		durationMs = sig.DurationMs
	}
	ingestedAt := time.Now().UnixMilli()

	// Layer contexts (all Nullable, one per signal)
	var ctxL1Cpu, ctxL1Mem, ctxL1Gpu *float32
	var ctxL2ModelID, ctxL2ModelHash, ctxL2Quant *string
	var ctxL3InputTok, ctxL3OutputTok *uint32
	var ctxL3Trunc *uint8
	var ctxL4Entropy, ctxL4KVHit *float32
	var ctxL5MeanLogprob, ctxL5TopLogprob *float32
	var ctxL5FinishReason *string
	var ctxL6SafetyScore *float32
	var ctxL6PolicyViolated, ctxL6Action *string
	var ctxL7QueryText *string
	var ctxL7RetrievedCount *uint32
	var ctxL7TopScore *float32
	var ctxL7CollectionName *string
	var ctxL8ToolName, ctxL8ToolInputHash *string
	var ctxL8AgentStep *uint32
	var ctxL9Method, ctxL9Path *string
	var ctxL9StatusCode *uint16
	var ctxL9LatencyMs *float32
	var ctxL10EventType, ctxL10Component *string

	// Extract layer-specific context based on Layer field
	switch sig.Layer {
	case v1.Layer_L1_HARDWARE:
		if ctx, ok := sig.Context.(*v1.ArgusSignal_ContextL1); ok && ctx.ContextL1 != nil {
			// ContextL1 is a placeholder
		}
	case v1.Layer_L5_OUTPUT_DECODING:
		if ctx, ok := sig.Context.(*v1.ArgusSignal_ContextL5); ok && ctx.ContextL5 != nil {
			c := ctx.ContextL5
			if c.MeanLogprob != nil {
				ctxL5MeanLogprob = c.MeanLogprob
			}
			if c.FinishReason != "" {
				ctxL5FinishReason = &c.FinishReason
			}
		}
	case v1.Layer_L7_RAG_RETRIEVAL:
		if ctx, ok := sig.Context.(*v1.ArgusSignal_ContextL7); ok && ctx.ContextL7 != nil {
			c := ctx.ContextL7
			if c.QueryText != "" {
				ctxL7QueryText = &c.QueryText
			}
			if c.ResultsCount != 0 {
				count := uint32(c.ResultsCount)
				ctxL7RetrievedCount = &count
			}
			if len(c.ResultsScores) > 0 {
				ctxL7TopScore = &c.ResultsScores[0]
			}
			if c.TopChunkSource != "" {
				ctxL7CollectionName = &c.TopChunkSource
			}
		}
	case v1.Layer_L8_AGENTS:
		if ctx, ok := sig.Context.(*v1.ArgusSignal_ContextL8); ok && ctx.ContextL8 != nil {
			c := ctx.ContextL8
			if c.ToolName != "" {
				ctxL8ToolName = &c.ToolName
			}
			if c.StepNumber > 0 {
				step := uint32(c.StepNumber)
				ctxL8AgentStep = &step
			}
		}
	}

	// Relationships
	var incidentID *string
	if sig.IncidentId != nil && *sig.IncidentId != "" {
		incidentID = sig.IncidentId
	}
	var sessionID *string
	if sig.SessionId != nil && *sig.SessionId != "" {
		sessionID = sig.SessionId
	}
	var conversationID *string
	if sig.ConversationId != nil && *sig.ConversationId != "" {
		conversationID = sig.ConversationId
	}
	var userID *string
	if sig.UserId != nil && *sig.UserId != "" {
		userID = sig.UserId
	}

	// Provider
	var providerName, providerModel *string
	if sig.Provider != nil {
		if sig.Provider.Name != "" {
			providerName = &sig.Provider.Name
		}
		if sig.Provider.Model != "" {
			providerModel = &sig.Provider.Model
		}
	}

	// Governance
	dataClassification := uint8(sig.DataClassification)
	retentionPolicy := sig.RetentionPolicy
	if retentionPolicy == "" {
		retentionPolicy = "default"
	}
	piiDetected := uint8(0)
	if sig.PiiDetected {
		piiDetected = 1
	}

	// Version for deduplication
	version := uint32(1)

	// Append to batch in column order
	return batch.Append(
		// Identity
		signalID, traceID, spanID, parentSpanID,
		// Source
		appID, appVersion, hostID, environment, sdkVersion,
		// Classification
		layer, category, severity,
		// Temporal
		timestamp, durationMs, ingestedAt,
		// Layer contexts (L1-L10)
		ctxL1Cpu, ctxL1Mem, ctxL1Gpu, // L1
		ctxL2ModelID, ctxL2ModelHash, ctxL2Quant, // L2
		ctxL3InputTok, ctxL3OutputTok, ctxL3Trunc, // L3
		ctxL4Entropy, ctxL4KVHit, // L4
		ctxL5MeanLogprob, ctxL5TopLogprob, ctxL5FinishReason, // L5
		ctxL6SafetyScore, ctxL6PolicyViolated, ctxL6Action, // L6
		ctxL7QueryText, ctxL7RetrievedCount, ctxL7TopScore, ctxL7CollectionName, // L7
		ctxL8ToolName, ctxL8ToolInputHash, ctxL8AgentStep, // L8
		ctxL9Method, ctxL9Path, ctxL9StatusCode, ctxL9LatencyMs, // L9
		ctxL10EventType, ctxL10Component, // L10
		// Relationships
		sig.RelatedSignals, incidentID, sessionID, conversationID, userID,
		// Provider
		providerName, providerModel,
		// Enrichment
		nil, nil, nil, nil, // placeholder enrichment fields
		// Governance
		dataClassification, retentionPolicy, piiDetected,
		// Version
		version,
	)
}

// Close closes the ClickHouse connection.
// Ping checks ClickHouse connectivity.
func (ch *ClickHouse) Ping(ctx context.Context) error {
	return ch.conn.Ping(ctx)
}

func (ch *ClickHouse) Close() error {
	return ch.conn.Close()
}

// Conn returns the underlying ClickHouse connection for advanced operations.
func (ch *ClickHouse) Conn() driver.Conn {
	return ch.conn
}

// DrainWorker pulls signals from the ingest queue and writes to the batch writer.
// Runs as a fixed goroutine (1 per ClickHouse writer in Phase 2; worker pool in Phase 3).
// Stops when ctx is cancelled.
func DrainWorker(ctx context.Context, queue Dequeuer, writer *BatchWriter, log *zap.Logger) {
	if log == nil {
		log = zap.NewNop()
	}

	log.Info("drain worker started")
	defer log.Info("drain worker stopped")

	for {
		sig, err := queue.Dequeue(ctx)
		if err != nil {
			// Context cancelled
			return
		}

		if sig == nil {
			continue
		}

		// Write signal to batch writer
		if err := writer.Write(ctx, sig); err != nil {
			log.Error("failed to write signal to batch writer", zap.Error(err), zap.String("signal_id", sig.SignalId))
			continue
		}
	}
}
