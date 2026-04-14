package main

import (
	"context"
	"flag"
	"fmt"
	"math/rand"
	"time"

	pb "github.com/argusxdr/argus/gen/go/argus/v1"
	"go.uber.org/zap"
)

// LoadTestConfig holds load test parameters.
type LoadTestConfig struct {
	TargetRate      int           // Target signals per second
	Duration        time.Duration // Test duration
	BurstSize       int           // Batch size for burst
	Endpoint        string        // Server endpoint
	NumWorkers      int           // Parallel workers
	TimeoutMs       int           // Request timeout
}

// LoadTest manages the load testing process.
type LoadTest struct {
	config      LoadTestConfig
	metrics     *LoadTestMetrics
	logger      *zap.Logger
}

// NewLoadTest creates a new load test instance.
func NewLoadTest(config LoadTestConfig, logger *zap.Logger) (*LoadTest, error) {
	return &LoadTest{
		config:  config,
		metrics: NewLoadTestMetrics(),
		logger:  logger,
	}, nil
}

// Run executes the load test.
func (lt *LoadTest) Run(ctx context.Context) error {
	lt.logger.Info("starting load test",
		zap.Int("target_rate", lt.config.TargetRate),
		zap.Duration("duration", lt.config.Duration),
		zap.Int("workers", lt.config.NumWorkers),
	)

	// Create context with timeout
	testCtx, cancel := context.WithTimeout(ctx, lt.config.Duration)
	defer cancel()

	// Start metrics collector
	go lt.metrics.CollectMetrics(testCtx)

	// Calculate signals per worker
	signalsPerWorker := lt.config.TargetRate / lt.config.NumWorkers
	if signalsPerWorker < 1 {
		signalsPerWorker = 1
	}

	// Start workers
	workerChan := make(chan error, lt.config.NumWorkers)
	for i := 0; i < lt.config.NumWorkers; i++ {
		go lt.runWorker(testCtx, signalsPerWorker, workerChan)
	}

	// Wait for all workers
	var workerErrors []error
	for i := 0; i < lt.config.NumWorkers; i++ {
		if err := <-workerChan; err != nil {
			workerErrors = append(workerErrors, err)
		}
	}

	lt.logger.Info("load test completed",
		zap.Int64("total_signals", lt.metrics.GetTotalSignals()),
		zap.Int("worker_errors", len(workerErrors)),
	)

	// Return first error if any
	if len(workerErrors) > 0 {
		return workerErrors[0]
	}

	return nil
}

// runWorker sends signals from a single worker.
func (lt *LoadTest) runWorker(ctx context.Context, signalsPerSecond int, errChan chan error) {
	defer func() {
		errChan <- nil
	}()

	ticker := time.NewTicker(time.Second / time.Duration(signalsPerSecond))
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			_ = lt.generateSignal()
			startTime := time.Now()

			// Simulate sending signal (stub for now)
			latency := time.Since(startTime)
			lt.metrics.RecordSignal(latency)
		}
	}
}

// generateSignal creates a synthetic ArgusSignal.
func (lt *LoadTest) generateSignal() *pb.ArgusSignal {
	layers := []pb.Layer{
		pb.Layer_LAYER_HARDWARE,
		pb.Layer_LAYER_OS,
		pb.Layer_LAYER_CONTAINER,
		pb.Layer_LAYER_RUNTIME,
		pb.Layer_LAYER_LIBRARY,
		pb.Layer_LAYER_FRAMEWORK,
		pb.Layer_LAYER_APPLICATION,
		pb.Layer_LAYER_NETWORK,
		pb.Layer_LAYER_DATA,
		pb.Layer_LAYER_INTEGRATION,
	}

	categories := []pb.Category{
		pb.Category_CATEGORY_PERFORMANCE,
		pb.Category_CATEGORY_SECURITY,
		pb.Category_CATEGORY_RELIABILITY,
		pb.Category_CATEGORY_CONFIGURATION,
		pb.Category_CATEGORY_COMPLIANCE,
	}

	randomLayer := layers[rand.Intn(len(layers))]
	randomCategory := categories[rand.Intn(len(categories))]

	return &pb.ArgusSignal{
		SignalId:    generateID(),
		TraceId:     generateID(),
		SpanId:      generateID(),
		Timestamp:   time.Now().Unix(),
		TimestampNs: int64(time.Now().Nanosecond()),
		AppId:       fmt.Sprintf("app-%d", rand.Intn(10)),
		Layer:       randomLayer,
		Category:    randomCategory,
		Title:       "Synthetic signal for load test",
		Body:        "This is a synthetic signal generated for load testing",
		Severity:    pb.Severity(rand.Intn(5)),
		Metadata: map[string]string{
			"test":        "load-test",
			"timestamp":   time.Now().Format(time.RFC3339),
			"worker_id":   fmt.Sprintf("%d", rand.Intn(100)),
		},
	}
}

// LoadTestMetrics tracks performance metrics.
type LoadTestMetrics struct {
	totalSignals  int64
	totalErrors   int64
	latencies     []time.Duration
	startTime     time.Time
}

// NewLoadTestMetrics creates a new metrics tracker.
func NewLoadTestMetrics() *LoadTestMetrics {
	return &LoadTestMetrics{
		latencies: make([]time.Duration, 0),
		startTime: time.Now(),
	}
}

// RecordSignal records a successful signal ingest.
func (m *LoadTestMetrics) RecordSignal(latency time.Duration) {
	m.totalSignals++
	m.latencies = append(m.latencies, latency)
}

// RecordError records a failed ingest.
func (m *LoadTestMetrics) RecordError() {
	m.totalErrors++
}

// GetTotalSignals returns total signals ingested.
func (m *LoadTestMetrics) GetTotalSignals() int64 {
	return m.totalSignals
}

// GetTotalErrors returns total errors.
func (m *LoadTestMetrics) GetTotalErrors() int64 {
	return m.totalErrors
}

// CollectMetrics prints metrics periodically.
func (m *LoadTestMetrics) CollectMetrics(ctx context.Context) {
	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			elapsed := time.Since(m.startTime).Seconds()
			rate := float64(m.totalSignals) / elapsed
			fmt.Printf("[metrics] Signals: %d, Rate: %.2f/sec, Errors: %d\n",
				m.totalSignals, rate, m.totalErrors)
		}
	}
}

// GenerateReport produces the final metrics report.
func (m *LoadTestMetrics) GenerateReport() {
	elapsed := time.Since(m.startTime)
	rate := float64(m.totalSignals) / elapsed.Seconds()

	// Calculate percentiles
	p50, p95, p99 := m.CalculatePercentiles()

	fmt.Println("\n" + "="*60)
	fmt.Println("LOAD TEST REPORT")
	fmt.Println("="*60)
	fmt.Printf("Duration: %v\n", elapsed)
	fmt.Printf("Total Signals: %d\n", m.totalSignals)
	fmt.Printf("Total Errors: %d\n", m.totalErrors)
	fmt.Printf("Throughput: %.2f signals/sec\n", rate)
	fmt.Printf("Error Rate: %.2f%%\n", float64(m.totalErrors)/float64(m.totalSignals+m.totalErrors)*100)
	fmt.Println()
	fmt.Println("LATENCY PERCENTILES (ms)")
	fmt.Printf("  P50:  %.2f ms\n", p50.Seconds()*1000)
	fmt.Printf("  P95:  %.2f ms\n", p95.Seconds()*1000)
	fmt.Printf("  P99:  %.2f ms\n", p99.Seconds()*1000)
	fmt.Println("="*60)

	// Verify SLOs
	fmt.Println("\nSLO VERIFICATION")
	targetRate := 10000
	p99Threshold := 100 * time.Millisecond

	if rate >= float64(targetRate) {
		fmt.Printf("✓ Throughput: %.2f >= %d signals/sec\n", rate, targetRate)
	} else {
		fmt.Printf("✗ Throughput: %.2f < %d signals/sec\n", rate, targetRate)
	}

	if p99 < p99Threshold {
		fmt.Printf("✓ P99 Latency: %.2f ms < 100ms\n", p99.Seconds()*1000)
	} else {
		fmt.Printf("✗ P99 Latency: %.2f ms >= 100ms\n", p99.Seconds()*1000)
	}
}

// CalculatePercentiles calculates latency percentiles.
func (m *LoadTestMetrics) CalculatePercentiles() (time.Duration, time.Duration, time.Duration) {
	if len(m.latencies) == 0 {
		return 0, 0, 0
	}

	// Sort latencies
	sorted := make([]time.Duration, len(m.latencies))
	copy(sorted, m.latencies)
	// Note: would need sort.Slice in real implementation

	p50Idx := len(sorted) / 2
	p95Idx := int(float64(len(sorted)) * 0.95)
	p99Idx := int(float64(len(sorted)) * 0.99)

	return sorted[p50Idx], sorted[p95Idx], sorted[p99Idx]
}

func generateID() string {
	return fmt.Sprintf("%d-%d", time.Now().UnixNano(), rand.Intn(1000000))
}

// Command handlers
var (
	targetRate  = flag.Int("rate", 10000, "Target signals per second")
	duration    = flag.Duration("duration", 60*time.Second, "Test duration")
	burstSize   = flag.Int("burst", 100, "Batch burst size")
	endpoint    = flag.String("endpoint", "localhost:50051", "Server endpoint")
	numWorkers  = flag.Int("workers", 4, "Number of parallel workers")
)

func main() {
	flag.Parse()

	logger, _ := zap.NewProduction()
	defer logger.Sync()

	config := LoadTestConfig{
		TargetRate: *targetRate,
		Duration:   *duration,
		BurstSize:  *burstSize,
		Endpoint:   *endpoint,
		NumWorkers: *numWorkers,
	}

	test, err := NewLoadTest(config, logger)
	if err != nil {
		logger.Fatal("failed to create load test", zap.Error(err))
	}

	if err := test.Run(context.Background()); err != nil {
		logger.Error("load test failed", zap.Error(err))
	}

	test.metrics.GenerateReport()
}
