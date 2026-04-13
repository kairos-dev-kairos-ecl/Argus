package metrics

import (
	"context"
	"fmt"
	"strconv"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/redis/go-redis/v9"
	"go.uber.org/zap"
)

// RedisClient defines the minimal interface needed for monitoring Redis.
type RedisClient interface {
	Info(ctx context.Context, section ...string) *redis.StringCmd
	DBSize(ctx context.Context) *redis.IntCmd
	Keys(ctx context.Context, pattern string) *redis.StringSliceCmd
}

// RedisMonitor periodically queries Redis INFO and DBSIZE commands
// to populate Prometheus gauges for memory usage and key count.
// This helps operators detect unbounded Redis growth (Critical Pitfall 2).
type RedisMonitor struct {
	client RedisClient
	logger *zap.Logger

	// Metrics
	memoryBytes prometheus.Gauge
	keyCount    prometheus.Gauge
	traceKeys   prometheus.Gauge

	// Control
	cancel context.CancelFunc
	done   chan struct{}

	// Configuration
	interval time.Duration
}

// NewRedisMonitor creates and registers Redis monitoring metrics.
// The returned monitor must be started with Start() to begin periodic monitoring.
//
// Metrics registered:
// - argus_redis_memory_bytes: Redis memory usage in bytes (from INFO memory)
// - argus_redis_key_count: Total number of keys in Redis (from DBSIZE)
// - argus_redis_trace_keys_count: Count of trace:* keys (debugging correlation growth)
func NewRedisMonitor(reg prometheus.Registerer, client RedisClient, logger *zap.Logger) *RedisMonitor {
	memoryBytes := prometheus.NewGauge(
		prometheus.GaugeOpts{
			Name: "argus_redis_memory_bytes",
			Help: "Redis memory usage in bytes (from INFO memory)",
		},
	)

	keyCount := prometheus.NewGauge(
		prometheus.GaugeOpts{
			Name: "argus_redis_key_count",
			Help: "Total number of keys in Redis (from DBSIZE)",
		},
	)

	traceKeys := prometheus.NewGauge(
		prometheus.GaugeOpts{
			Name: "argus_redis_trace_keys_count",
			Help: "Count of trace:* keys used by CorrelationTagger (debugging correlation growth)",
		},
	)

	// Register all metrics
	reg.MustRegister(memoryBytes)
	reg.MustRegister(keyCount)
	reg.MustRegister(traceKeys)

	return &RedisMonitor{
		client:   client,
		logger:   logger,
		memoryBytes: memoryBytes,
		keyCount:    keyCount,
		traceKeys:   traceKeys,
		done:        make(chan struct{}),
		interval:    30 * time.Second, // Monitor every 30 seconds
	}
}

// Start launches a goroutine that periodically queries Redis and updates metrics.
// The monitoring runs every 30 seconds until Stop() is called.
// If Redis is unavailable, errors are logged at debug level and monitoring continues.
func (rm *RedisMonitor) Start(ctx context.Context) {
	// Create a cancellable context for this monitor
	monitorCtx, cancel := context.WithCancel(ctx)
	rm.cancel = cancel

	go func() {
		defer close(rm.done)

		ticker := time.NewTicker(rm.interval)
		defer ticker.Stop()

		// Run once immediately, then on each tick
		rm.updateMetrics(monitorCtx)

		for {
			select {
			case <-monitorCtx.Done():
				rm.logger.Debug("redis monitor context cancelled, stopping")
				return
			case <-ticker.C:
				rm.updateMetrics(monitorCtx)
			}
		}
	}()

	rm.logger.Info("redis monitor started", zap.Duration("interval", rm.interval))
}

// Stop terminates the monitoring goroutine and waits for it to finish.
func (rm *RedisMonitor) Stop() {
	if rm.cancel != nil {
		rm.cancel()
		// Wait for the goroutine to finish
		<-rm.done
		rm.logger.Info("redis monitor stopped")
	}
}

// updateMetrics queries Redis and updates all monitoring gauges.
func (rm *RedisMonitor) updateMetrics(ctx context.Context) {
	// Query INFO memory to get memory usage
	if memoryBytes, err := rm.queryMemoryBytes(ctx); err != nil {
		rm.logger.Debug("failed to query redis memory", zap.Error(err))
	} else {
		rm.memoryBytes.Set(float64(memoryBytes))
		rm.logger.Debug("updated redis_memory_bytes", zap.Int64("bytes", memoryBytes))
	}

	// Query DBSIZE to get total key count
	if keyCount, err := rm.queryKeyCount(ctx); err != nil {
		rm.logger.Debug("failed to query redis key count", zap.Error(err))
	} else {
		rm.keyCount.Set(float64(keyCount))
		rm.logger.Debug("updated redis_key_count", zap.Int64("count", keyCount))
	}

	// Query trace:* keys for debugging correlation growth
	if traceCount, err := rm.queryTraceKeyCount(ctx); err != nil {
		rm.logger.Debug("failed to query redis trace keys", zap.Error(err))
	} else {
		rm.traceKeys.Set(float64(traceCount))
		rm.logger.Debug("updated redis_trace_keys_count", zap.Int64("count", traceCount))
	}
}

// queryMemoryBytes executes INFO memory and extracts used_memory.
// Returns the memory usage in bytes, or an error if the query fails.
func (rm *RedisMonitor) queryMemoryBytes(ctx context.Context) (int64, error) {
	// Create a context with a 5-second timeout for the INFO command
	queryCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	cmd := rm.client.Info(queryCtx, "memory")
	result, err := cmd.Result()
	if err != nil {
		return 0, fmt.Errorf("info memory failed: %w", err)
	}

	// Parse the INFO output to find used_memory
	// INFO output format: key1:value1\r\nkey2:value2\r\n...
	memoryBytes, err := parseInfoField(result, "used_memory")
	if err != nil {
		return 0, fmt.Errorf("failed to parse used_memory from info: %w", err)
	}

	return memoryBytes, nil
}

// queryKeyCount executes DBSIZE and returns the total number of keys.
func (rm *RedisMonitor) queryKeyCount(ctx context.Context) (int64, error) {
	// Create a context with a 5-second timeout for the DBSIZE command
	queryCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	cmd := rm.client.DBSize(queryCtx)
	count, err := cmd.Result()
	if err != nil {
		return 0, fmt.Errorf("dbsize failed: %w", err)
	}

	return count, nil
}

// queryTraceKeyCount counts the number of trace:* keys for debugging correlation growth.
// Uses KEYS pattern matching (not ideal for large key spaces, but acceptable for monitoring).
func (rm *RedisMonitor) queryTraceKeyCount(ctx context.Context) (int64, error) {
	// Create a context with a 5-second timeout for the KEYS command
	queryCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	cmd := rm.client.Keys(queryCtx, "trace:*")
	keys, err := cmd.Result()
	if err != nil {
		return 0, fmt.Errorf("keys trace:* failed: %w", err)
	}

	return int64(len(keys)), nil
}

// parseInfoField extracts a numeric field value from Redis INFO output.
// INFO output format: key1:value1\r\nkey2:value2\r\n...
func parseInfoField(info string, fieldName string) (int64, error) {
	// Parse line by line
	lines := len(info)
	start := 0

	for start < lines {
		// Find end of line
		end := start
		for end < lines && info[end] != '\n' {
			end++
		}

		line := info[start:end]

		// Remove trailing \r if present
		if len(line) > 0 && line[len(line)-1] == '\r' {
			line = line[:len(line)-1]
		}

		// Skip empty lines and comments
		if len(line) == 0 || line[0] == '#' {
			start = end + 1
			continue
		}

		// Split on colon
		colonIdx := -1
		for i := 0; i < len(line); i++ {
			if line[i] == ':' {
				colonIdx = i
				break
			}
		}

		if colonIdx == -1 {
			start = end + 1
			continue
		}

		key := line[:colonIdx]
		value := line[colonIdx+1:]

		if key == fieldName {
			// Parse the value as int64
			return strconv.ParseInt(value, 10, 64)
		}

		start = end + 1
	}

	return 0, fmt.Errorf("field %s not found in info output", fieldName)
}
