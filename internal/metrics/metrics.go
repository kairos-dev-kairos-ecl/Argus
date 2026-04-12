package metrics

import (
	"github.com/prometheus/client_golang/prometheus"
	"github.com/redis/go-redis/v9"
	"go.uber.org/zap"
)

// Ingest holds all metrics related to signal ingestion.
type Ingest struct {
	// argus_signals_received_total{receiver="grpc"|"http"|"otlp"} Counter
	// Total signals received by each receiver type (before queue)
	SignalsReceived *prometheus.CounterVec

	// argus_signals_dropped_total{reason="queue_full"|"auth_failed"|"invalid"} Counter
	// Signals dropped for any reason — INGEST-06 requirement
	SignalsDropped *prometheus.CounterVec

	// argus_ingest_queue_depth Gauge
	// Current number of signals in the ingest queue — INGEST-05 requirement
	QueueDepth prometheus.Gauge

	// argus_ingest_request_duration_seconds{receiver="grpc"|"http"|"otlp", status="ok"|"error"} Histogram
	// Duration of each ingest RPC/request (from first byte to ack)
	RequestDuration *prometheus.HistogramVec
}

// Storage holds all metrics related to storage writes.
type Storage struct {
	// argus_storage_batch_flush_total{status="ok"|"error"} Counter
	BatchFlushes *prometheus.CounterVec

	// argus_storage_batch_size Histogram (buckets: 1, 50, 100, 250, 500, 750, 1000)
	// Signals per flush — helps tune batch size
	BatchSize prometheus.Histogram

	// argus_storage_batch_flush_duration_seconds Histogram (buckets: .001, .005, .01, .05, .1, .5, 1)
	// Time from flush trigger to ClickHouse ack
	FlushDuration prometheus.Histogram

	// argus_storage_insert_errors_total Counter
	InsertErrors prometheus.Counter
}

// HTTP holds metrics for the query API.
type HTTP struct {
	// argus_http_request_duration_seconds{method, path, status_code} Histogram
	RequestDuration *prometheus.HistogramVec

	// argus_http_requests_total{method, path, status_code} Counter
	RequestsTotal *prometheus.CounterVec
}

// Metrics holds all metric groups for the system.
type Metrics struct {
	Ingest        *Ingest
	Storage       *Storage
	HTTP          *HTTP
	RedisMonitor  *RedisMonitor
}

// Options for metrics initialization.
type MetricsOptions struct {
	// If set, enables Redis monitoring
	RedisClient *redis.Client
	Logger      *zap.Logger
}

// Init initializes all metrics and optionally starts Redis monitoring.
// Returns a Metrics struct containing all registered metric groups.
// If Redis monitoring is enabled, the caller must call Shutdown() to clean up.
func Init(reg prometheus.Registerer, opts *MetricsOptions) *Metrics {
	m := &Metrics{
		Ingest:  NewIngest(reg),
		Storage: NewStorage(reg),
		HTTP:    NewHTTP(reg),
	}

	// Initialize Redis monitoring if enabled
	if opts != nil && opts.RedisClient != nil && opts.Logger != nil {
		m.RedisMonitor = NewRedisMonitor(reg, opts.RedisClient, opts.Logger)
	}

	return m
}

// Shutdown gracefully shuts down the metrics system.
// If RedisMonitor is running, it will be stopped.
func (m *Metrics) Shutdown() {
	if m.RedisMonitor != nil {
		m.RedisMonitor.Stop()
	}
}

// NewIngest registers and returns Ingest metrics.
// Panics if registration fails (metrics must be unique per process).
func NewIngest(reg prometheus.Registerer) *Ingest {
	signalsReceived := prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "argus_signals_received_total",
			Help: "Total signals received by each receiver type",
		},
		[]string{"receiver"},
	)

	signalsDropped := prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "argus_signals_dropped_total",
			Help: "Total signals dropped due to any reason (queue full, auth failed, invalid)",
		},
		[]string{"reason"},
	)

	queueDepth := prometheus.NewGauge(
		prometheus.GaugeOpts{
			Name: "argus_ingest_queue_depth",
			Help: "Current number of signals in the ingest queue",
		},
	)

	requestDuration := prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "argus_ingest_request_duration_seconds",
			Help:    "Duration of each ingest RPC/request from first byte to ack",
			Buckets: prometheus.DefBuckets,
		},
		[]string{"receiver", "status"},
	)

	// Register all metrics
	reg.MustRegister(signalsReceived)
	reg.MustRegister(signalsDropped)
	reg.MustRegister(queueDepth)
	reg.MustRegister(requestDuration)

	return &Ingest{
		SignalsReceived: signalsReceived,
		SignalsDropped:  signalsDropped,
		QueueDepth:      queueDepth,
		RequestDuration: requestDuration,
	}
}

// NewStorage registers and returns Storage metrics.
func NewStorage(reg prometheus.Registerer) *Storage {
	batchFlushes := prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "argus_storage_batch_flush_total",
			Help: "Total number of batch flushes to storage",
		},
		[]string{"status"},
	)

	batchSize := prometheus.NewHistogram(
		prometheus.HistogramOpts{
			Name:    "argus_storage_batch_size",
			Help:    "Number of signals per batch flush",
			Buckets: []float64{1, 50, 100, 250, 500, 750, 1000},
		},
	)

	flushDuration := prometheus.NewHistogram(
		prometheus.HistogramOpts{
			Name:    "argus_storage_batch_flush_duration_seconds",
			Help:    "Time from flush trigger to ClickHouse acknowledgment",
			Buckets: []float64{.001, .005, .01, .05, .1, .5, 1},
		},
	)

	insertErrors := prometheus.NewCounter(
		prometheus.CounterOpts{
			Name: "argus_storage_insert_errors_total",
			Help: "Total number of insert errors to storage",
		},
	)

	// Register all metrics
	reg.MustRegister(batchFlushes)
	reg.MustRegister(batchSize)
	reg.MustRegister(flushDuration)
	reg.MustRegister(insertErrors)

	return &Storage{
		BatchFlushes:  batchFlushes,
		BatchSize:     batchSize,
		FlushDuration: flushDuration,
		InsertErrors:  insertErrors,
	}
}

// NewHTTP registers and returns HTTP metrics.
func NewHTTP(reg prometheus.Registerer) *HTTP {
	requestDuration := prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "argus_http_request_duration_seconds",
			Help:    "Duration of HTTP requests",
			Buckets: prometheus.DefBuckets,
		},
		[]string{"method", "path", "status_code"},
	)

	requestsTotal := prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "argus_http_requests_total",
			Help: "Total number of HTTP requests",
		},
		[]string{"method", "path", "status_code"},
	)

	// Register all metrics
	reg.MustRegister(requestDuration)
	reg.MustRegister(requestsTotal)

	return &HTTP{
		RequestDuration: requestDuration,
		RequestsTotal:   requestsTotal,
	}
}
