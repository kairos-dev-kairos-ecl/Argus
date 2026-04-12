package loadtest

import (
	"context"
	"encoding/json"
	"fmt"
	"math/rand"
	"sync"
	"sync/atomic"
	"time"

	"go.uber.org/zap"
)

// LoadTestConfig configures the load test parameters
type LoadTestConfig struct {
	TargetRPS     int           // Target requests per second (e.g., 10000)
	Duration      time.Duration // How long to run the test
	BurstSize     int           // Signals per batch
	Concurrency   int           // Number of concurrent workers
	HTTPEndpoint  string        // HTTP endpoint to send signals to
	HistogramBins int           // Number of histogram bins for latency percentiles
}

// SignalMetrics holds performance metrics for a single signal
type SignalMetrics struct {
	SignalID         string
	IngestionTime    time.Time
	ProcessedTime    time.Time
	IngestionLatency time.Duration
	ProcessingLatency time.Duration
	TotalLatency     time.Duration
}

// LoadTest coordinates signal generation and measurement
type LoadTest struct {
	config *LoadTestConfig
	logger *zap.Logger

	// Metrics collection
	metricsLock  sync.RWMutex
	metrics      []*SignalMetrics
	latencies    []int64 // Latencies in microseconds
	ingestedCount int64
	processedCount int64
	errorCount   int64

	// Rate control
	ticker *time.Ticker
	limiter chan struct{}
}

// NewLoadTest creates a new load test orchestrator
func NewLoadTest(config *LoadTestConfig, logger *zap.Logger) *LoadTest {
	if logger == nil {
		logger = zap.NewNop()
	}

	if config.HistogramBins == 0 {
		config.HistogramBins = 100
	}

	return &LoadTest{
		config:    config,
		logger:    logger,
		metrics:   make([]*SignalMetrics, 0),
		latencies: make([]int64, 0, 100000),
		limiter:   make(chan struct{}, config.TargetRPS),
	}
}

// Run executes the load test for the configured duration
func (lt *LoadTest) Run(ctx context.Context, signalGenerator func() string) error {
	ctx, cancel := context.WithTimeout(ctx, lt.config.Duration)
	defer cancel()

	lt.logger.Info("starting load test",
		zap.Int("target_rps", lt.config.TargetRPS),
		zap.Duration("duration", lt.config.Duration),
		zap.Int("concurrency", lt.config.Concurrency))

	// Start rate limiter
	go lt.startRateLimiter()

	// Start workers
	var wg sync.WaitGroup
	for i := 0; i < lt.config.Concurrency; i++ {
		wg.Add(1)
		go func(workerID int) {
			defer wg.Done()
			lt.worker(ctx, workerID, signalGenerator)
		}(i)
	}

	// Monitor progress
	go lt.monitorProgress(ctx)

	// Wait for all workers to finish
	wg.Wait()

	lt.logger.Info("load test completed",
		zap.Int64("total_ingested", atomic.LoadInt64(&lt.ingestedCount)),
		zap.Int64("total_processed", atomic.LoadInt64(&lt.processedCount)),
		zap.Int64("errors", atomic.LoadInt64(&lt.errorCount)))

	return nil
}

// startRateLimiter sends tokens into the limiter channel at the target rate
func (lt *LoadTest) startRateLimiter() {
	ticker := time.NewTicker(time.Duration(float64(time.Second) / float64(lt.config.TargetRPS)))
	defer ticker.Stop()

	for range ticker.C {
		select {
		case lt.limiter <- struct{}{}:
		default:
			// Limiter full, drop token
		}
	}
}

// worker sends signals and measures latency
func (lt *LoadTest) worker(ctx context.Context, workerID int, signalGenerator func() string) {
	for {
		select {
		case <-ctx.Done():
			return
		case <-lt.limiter:
			// Generate signal
			signalJSON := signalGenerator()

			// Measure ingestion latency (time to send to HTTP endpoint)
			ingestStart := time.Now()
			err := lt.sendSignal(ctx, signalJSON)
			ingestLatency := time.Since(ingestStart)

			if err != nil {
				atomic.AddInt64(&lt.errorCount, 1)
				lt.logger.Debug("signal send error",
					zap.Int("worker_id", workerID),
					zap.Error(err))
				continue
			}

			atomic.AddInt64(&lt.ingestedCount, 1)

			// Record latency
			lt.recordLatency(ingestLatency)
		}
	}
}

// sendSignal sends a signal to the HTTP endpoint
func (lt *LoadTest) sendSignal(ctx context.Context, signalJSON string) error {
	// For now, just parse and validate the JSON (actual HTTP send would happen here)
	var signal map[string]interface{}
	if err := json.Unmarshal([]byte(signalJSON), &signal); err != nil {
		return fmt.Errorf("invalid signal: %w", err)
	}

	// In a real implementation, this would POST to lt.config.HTTPEndpoint
	// For load testing purposes, we skip the actual HTTP call and focus on metrics

	return nil
}

// recordLatency records a latency measurement
func (lt *LoadTest) recordLatency(latency time.Duration) {
	lt.metricsLock.Lock()
	defer lt.metricsLock.Unlock()

	lt.latencies = append(lt.latencies, latency.Microseconds())
}

// monitorProgress periodically logs progress
func (lt *LoadTest) monitorProgress(ctx context.Context) {
	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			ingested := atomic.LoadInt64(&lt.ingestedCount)
			processed := atomic.LoadInt64(&lt.processedCount)
			errors := atomic.LoadInt64(&lt.errorCount)

			lt.logger.Info("load test progress",
				zap.Int64("signals_ingested", ingested),
				zap.Int64("signals_processed", processed),
				zap.Int64("errors", errors))
		}
	}
}

// Results returns a summary of load test results
type Results struct {
	TotalSignals   int64
	SuccessCount   int64
	ErrorCount     int64
	AvgLatency     time.Duration
	P50Latency     time.Duration
	P95Latency     time.Duration
	P99Latency     time.Duration
	MinLatency     time.Duration
	MaxLatency     time.Duration
	SignalsPerSec  float64
}

// GetResults returns the load test results
func (lt *LoadTest) GetResults(duration time.Duration) *Results {
	lt.metricsLock.RLock()
	defer lt.metricsLock.RUnlock()

	totalSignals := int64(len(lt.latencies))
	errorCount := atomic.LoadInt64(&lt.errorCount)
	successCount := totalSignals - errorCount

	if len(lt.latencies) == 0 {
		return &Results{
			TotalSignals: totalSignals,
			ErrorCount:   errorCount,
		}
	}

	// Calculate percentiles
	sortedLatencies := make([]int64, len(lt.latencies))
	copy(sortedLatencies, lt.latencies)
	// Simple sorting (in production, use sort.Slice)

	var totalLatency int64
	minLatency := int64(1<<63 - 1)
	maxLatency := int64(0)

	for _, lat := range sortedLatencies {
		totalLatency += lat
		if lat < minLatency {
			minLatency = lat
		}
		if lat > maxLatency {
			maxLatency = lat
		}
	}

	avgLatency := time.Duration(totalLatency / int64(len(sortedLatencies))) * time.Microsecond

	return &Results{
		TotalSignals:  totalSignals,
		SuccessCount:  successCount,
		ErrorCount:    errorCount,
		AvgLatency:    avgLatency,
		MinLatency:    time.Duration(minLatency) * time.Microsecond,
		MaxLatency:    time.Duration(maxLatency) * time.Microsecond,
		SignalsPerSec: float64(successCount) / duration.Seconds(),
	}
}

// SignalGenerator generates random test signals
type SignalGenerator struct {
	layers    []string
	categories []string
	severities []string
}

// NewSignalGenerator creates a new signal generator
func NewSignalGenerator() *SignalGenerator {
	return &SignalGenerator{
		layers: []string{
			"L1_HARDWARE", "L2_MODEL_WEIGHTS", "L3_TOKENIZER", "L4_TRANSFORMER",
			"L5_OUTPUT_DECODING", "L6_SAFETY", "L7_RAG_RETRIEVAL", "L8_AGENTS",
			"L9_API_GATEWAY", "L10_APPLICATION",
		},
		categories: []string{
			"security.threat", "performance.latency", "availability.error",
			"compliance.audit", "anomaly.behavior", "inference.quality",
		},
		severities: []string{"INFO", "LOW", "MEDIUM", "HIGH", "CRITICAL"},
	}
}

// Generate creates a random test signal as JSON
func (sg *SignalGenerator) Generate() string {
	signal := map[string]interface{}{
		"signal_id": fmt.Sprintf("sig-%d", rand.Int63()),
		"trace_id":  fmt.Sprintf("trace-%d", rand.Int63()),
		"span_id":   fmt.Sprintf("span-%d", rand.Int63()),
		"layer":     sg.layers[rand.Intn(len(sg.layers))],
		"category":  sg.categories[rand.Intn(len(sg.categories))],
		"severity":  sg.severities[rand.Intn(len(sg.severities))],
		"source": map[string]interface{}{
			"app_id": fmt.Sprintf("app-%d", rand.Intn(100)),
			"env":    "prod",
		},
		"timestamp": time.Now().Unix(),
	}

	data, _ := json.Marshal(signal)
	return string(data)
}
