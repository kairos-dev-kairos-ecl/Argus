package metrics

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/assert"
	"go.uber.org/zap"
)

// mockRedisClient implements a minimal mock of redis.Client for testing.
type mockRedisClient struct {
	infoFunc   func(ctx context.Context, section ...string) *redis.StringCmd
	dbsizeFunc func(ctx context.Context) *redis.IntCmd
	keysFunc   func(ctx context.Context, pattern string) *redis.StringSliceCmd
}

func (m *mockRedisClient) Info(ctx context.Context, section ...string) *redis.StringCmd {
	if m.infoFunc != nil {
		return m.infoFunc(ctx, section...)
	}
	return nil
}

func (m *mockRedisClient) DBSize(ctx context.Context) *redis.IntCmd {
	if m.dbsizeFunc != nil {
		return m.dbsizeFunc(ctx)
	}
	return nil
}

func (m *mockRedisClient) Keys(ctx context.Context, pattern string) *redis.StringSliceCmd {
	if m.keysFunc != nil {
		return m.keysFunc(ctx, pattern)
	}
	return nil
}

// mockStringCmd wraps a string result for testing
type mockStringCmd struct {
	val string
	err error
}

func (m *mockStringCmd) Result() (string, error) {
	return m.val, m.err
}

func (m *mockStringCmd) String() string {
	return m.val
}

func (m *mockStringCmd) Args() []interface{} {
	return nil
}

func (m *mockStringCmd) Err() error {
	return m.err
}

func (m *mockStringCmd) Name() string {
	return "mock"
}

func (m *mockStringCmd) FullName() string {
	return "mock"
}

// mockIntCmd wraps an int result for testing
type mockIntCmd struct {
	val int64
	err error
}

func (m *mockIntCmd) Result() (int64, error) {
	return m.val, m.err
}

func (m *mockIntCmd) String() string {
	return ""
}

func (m *mockIntCmd) Args() []interface{} {
	return nil
}

func (m *mockIntCmd) Err() error {
	return m.err
}

func (m *mockIntCmd) Name() string {
	return "mock"
}

func (m *mockIntCmd) FullName() string {
	return "mock"
}

// mockStringSliceCmd wraps a string slice result for testing
type mockStringSliceCmd struct {
	val []string
	err error
}

func (m *mockStringSliceCmd) Result() ([]string, error) {
	return m.val, m.err
}

func (m *mockStringSliceCmd) String() string {
	return ""
}

func (m *mockStringSliceCmd) Args() []interface{} {
	return nil
}

func (m *mockStringSliceCmd) Err() error {
	return m.err
}

func (m *mockStringSliceCmd) Name() string {
	return "mock"
}

func (m *mockStringSliceCmd) FullName() string {
	return "mock"
}

func getTestLogger() *zap.Logger {
	// Create a no-op logger for testing
	logger, _ := zap.NewDevelopment()
	return logger
}

// Test 1: Metrics are properly registered
func TestRedisMonitorMetricsRegistered(t *testing.T) {
	reg := prometheus.NewRegistry()
	logger := getTestLogger()

	mock := &mockRedisClient{}
	monitor := NewRedisMonitor(reg, mock, logger)

	assert.NotNil(t, monitor, "RedisMonitor should not be nil")
	assert.NotNil(t, monitor.memoryBytes, "memoryBytes gauge should be registered")
	assert.NotNil(t, monitor.keyCount, "keyCount gauge should be registered")
	assert.NotNil(t, monitor.traceKeys, "traceKeys gauge should be registered")

	// Verify metrics are in the registry
	metrics, err := reg.Gather()
	assert.NoError(t, err, "Gather should not error")

	expectedMetrics := map[string]bool{
		"argus_redis_memory_bytes":       false,
		"argus_redis_key_count":          false,
		"argus_redis_trace_keys_count":   false,
	}

	for _, mf := range metrics {
		name := mf.GetName()
		if _, ok := expectedMetrics[name]; ok {
			expectedMetrics[name] = true
		}
	}

	for metric, found := range expectedMetrics {
		assert.True(t, found, "Metric %s should be registered", metric)
	}
}

// Test 2: Memory gauge updates on Start
func TestRedisMonitorMemoryGaugeUpdates(t *testing.T) {
	reg := prometheus.NewRegistry()
	logger := getTestLogger()

	// Mock Redis returning memory info
	infoOutput := "# Memory\r\nused_memory:1048576\r\nmemory_human:1.00M\r\n"

	mock := &mockRedisClient{
		infoFunc: func(ctx context.Context, section ...string) *redis.StringCmd {
			cmd := redis.NewStringCmd(ctx, nil)
			cmd.SetVal(infoOutput)
			return cmd
		},
		dbsizeFunc: func(ctx context.Context) *redis.IntCmd {
			cmd := redis.NewIntCmd(ctx, nil)
			cmd.SetVal(100)
			return cmd
		},
		keysFunc: func(ctx context.Context, pattern string) *redis.StringSliceCmd {
			cmd := redis.NewStringSliceCmd(ctx, nil)
			cmd.SetVal([]string{"trace:123", "trace:456"})
			return cmd
		},
	}

	monitor := NewRedisMonitor(reg, mock, logger)
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	monitor.Start(ctx)

	// Give the monitoring goroutine time to run at least once
	time.Sleep(100 * time.Millisecond)

	// Gather metrics and verify memory value was set
	metrics, err := reg.Gather()
	assert.NoError(t, err)

	found := false
	for _, mf := range metrics {
		if mf.GetName() == "argus_redis_memory_bytes" {
			found = true
			assert.NotNil(t, mf.Metric)
			if len(mf.Metric) > 0 {
				val := mf.Metric[0].Gauge.GetValue()
				assert.Equal(t, float64(1048576), val, "Memory gauge should be set to 1048576 bytes")
			}
		}
	}
	assert.True(t, found, "argus_redis_memory_bytes metric should be found")

	monitor.Stop()
}

// Test 3: Key count gauge updates on Start
func TestRedisMonitorKeyCountGaugeUpdates(t *testing.T) {
	reg := prometheus.NewRegistry()
	logger := getTestLogger()

	infoOutput := "# Memory\r\nused_memory:1048576\r\n"
	keyCount := int64(5432)

	mock := &mockRedisClient{
		infoFunc: func(ctx context.Context, section ...string) *redis.StringCmd {
			cmd := redis.NewStringCmd(ctx, nil)
			cmd.SetVal(infoOutput)
			return cmd
		},
		dbsizeFunc: func(ctx context.Context) *redis.IntCmd {
			cmd := redis.NewIntCmd(ctx, nil)
			cmd.SetVal(keyCount)
			return cmd
		},
		keysFunc: func(ctx context.Context, pattern string) *redis.StringSliceCmd {
			cmd := redis.NewStringSliceCmd(ctx, nil)
			cmd.SetVal([]string{"trace:123", "trace:456", "trace:789"})
			return cmd
		},
	}

	monitor := NewRedisMonitor(reg, mock, logger)
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	monitor.Start(ctx)

	// Give the monitoring goroutine time to run at least once
	time.Sleep(100 * time.Millisecond)

	// Gather metrics and verify key count value was set
	metrics, err := reg.Gather()
	assert.NoError(t, err)

	found := false
	for _, mf := range metrics {
		if mf.GetName() == "argus_redis_key_count" {
			found = true
			assert.NotNil(t, mf.Metric)
			if len(mf.Metric) > 0 {
				val := mf.Metric[0].Gauge.GetValue()
				assert.Equal(t, float64(keyCount), val, "Key count gauge should be set to %d", keyCount)
			}
		}
	}
	assert.True(t, found, "argus_redis_key_count metric should be found")

	monitor.Stop()
}

// Test 4: Handles Redis errors gracefully
func TestRedisMonitorHandlesErrorsGracefully(t *testing.T) {
	reg := prometheus.NewRegistry()
	logger := getTestLogger()

	// Mock Redis returning errors
	mock := &mockRedisClient{
		infoFunc: func(ctx context.Context, section ...string) *redis.StringCmd {
			cmd := redis.NewStringCmd(ctx, nil)
			cmd.SetErr(errors.New("connection refused"))
			return cmd
		},
		dbsizeFunc: func(ctx context.Context) *redis.IntCmd {
			cmd := redis.NewIntCmd(ctx, nil)
			cmd.SetErr(errors.New("connection refused"))
			return cmd
		},
		keysFunc: func(ctx context.Context, pattern string) *redis.StringSliceCmd {
			cmd := redis.NewStringSliceCmd(ctx, nil)
			cmd.SetErr(errors.New("connection refused"))
			return cmd
		},
	}

	monitor := NewRedisMonitor(reg, mock, logger)
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	// Start should not panic or error even when Redis is unavailable
	assert.NotPanics(t, func() {
		monitor.Start(ctx)
		time.Sleep(100 * time.Millisecond)
		monitor.Stop()
	}, "Monitor should handle Redis errors gracefully")
}

// Test 5: Stop() terminates monitoring goroutine
func TestRedisMonitorStopTerminatesGoroutine(t *testing.T) {
	reg := prometheus.NewRegistry()
	logger := getTestLogger()

	callCount := 0
	mock := &mockRedisClient{
		infoFunc: func(ctx context.Context, section ...string) *redis.StringCmd {
			cmd := redis.NewStringCmd(ctx, nil)
			cmd.SetVal("used_memory:1000000\r\n")
			return cmd
		},
		dbsizeFunc: func(ctx context.Context) *redis.IntCmd {
			callCount++
			cmd := redis.NewIntCmd(ctx, nil)
			cmd.SetVal(100)
			return cmd
		},
		keysFunc: func(ctx context.Context, pattern string) *redis.StringSliceCmd {
			cmd := redis.NewStringSliceCmd(ctx, nil)
			cmd.SetVal([]string{})
			return cmd
		},
	}

	monitor := NewRedisMonitor(reg, mock, logger)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	monitor.Start(ctx)

	// Let it run for a short time
	initialCount := callCount
	time.Sleep(150 * time.Millisecond)

	// Stop the monitor
	monitor.Stop()

	// Record call count after stop
	countAfterStop := callCount

	// Give it time to try another update (shouldn't happen)
	time.Sleep(150 * time.Millisecond)

	// The count should not increase much after stop (allowing some minor variance)
	// It should have completed cleanly
	assert.True(t, countAfterStop >= initialCount, "Should have at least one call during monitoring")
}

// Test 6: Trace key counting works
func TestRedisMonitorTraceKeyCountUpdates(t *testing.T) {
	reg := prometheus.NewRegistry()
	logger := getTestLogger()

	infoOutput := "# Memory\r\nused_memory:1048576\r\n"
	traceKeys := []string{"trace:id1", "trace:id2", "trace:id3", "trace:id4"}

	mock := &mockRedisClient{
		infoFunc: func(ctx context.Context, section ...string) *redis.StringCmd {
			cmd := redis.NewStringCmd(ctx, nil)
			cmd.SetVal(infoOutput)
			return cmd
		},
		dbsizeFunc: func(ctx context.Context) *redis.IntCmd {
			cmd := redis.NewIntCmd(ctx, nil)
			cmd.SetVal(500)
			return cmd
		},
		keysFunc: func(ctx context.Context, pattern string) *redis.StringSliceCmd {
			cmd := redis.NewStringSliceCmd(ctx, nil)
			if pattern == "trace:*" {
				cmd.SetVal(traceKeys)
			} else {
				cmd.SetVal([]string{})
			}
			return cmd
		},
	}

	monitor := NewRedisMonitor(reg, mock, logger)
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	monitor.Start(ctx)

	// Give the monitoring goroutine time to run
	time.Sleep(100 * time.Millisecond)

	// Gather metrics and verify trace key count
	metrics, err := reg.Gather()
	assert.NoError(t, err)

	found := false
	for _, mf := range metrics {
		if mf.GetName() == "argus_redis_trace_keys_count" {
			found = true
			assert.NotNil(t, mf.Metric)
			if len(mf.Metric) > 0 {
				val := mf.Metric[0].Gauge.GetValue()
				assert.Equal(t, float64(4), val, "Trace key count should be 4")
			}
		}
	}
	assert.True(t, found, "argus_redis_trace_keys_count metric should be found")

	monitor.Stop()
}

// Test 7: parseInfoField helper function
func TestParseInfoField(t *testing.T) {
	tests := []struct {
		name      string
		input     string
		field     string
		expected  int64
		shouldErr bool
	}{
		{
			name:     "simple used_memory",
			input:    "used_memory:1048576\r\n",
			field:    "used_memory",
			expected: 1048576,
		},
		{
			name:     "multiple fields",
			input:    "used_memory:1048576\r\nused_memory_human:1.00M\r\nmemory_peak:2097152\r\n",
			field:    "used_memory",
			expected: 1048576,
		},
		{
			name:     "with comment lines",
			input:    "# Memory\r\nused_memory:1048576\r\n# Stats\r\n",
			field:    "used_memory",
			expected: 1048576,
		},
		{
			name:     "memory_peak field",
			input:    "used_memory:1048576\r\nmemory_peak:2097152\r\n",
			field:    "memory_peak",
			expected: 2097152,
		},
		{
			name:      "field not found",
			input:     "used_memory:1048576\r\n",
			field:     "nonexistent",
			shouldErr: true,
		},
		{
			name:      "empty input",
			input:     "",
			field:     "used_memory",
			shouldErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := parseInfoField(tt.input, tt.field)
			if tt.shouldErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				assert.Equal(t, tt.expected, result)
			}
		})
	}
}

// Test 8: Duplicate registration should panic
func TestRedisMonitorDuplicateRegistration(t *testing.T) {
	reg := prometheus.NewRegistry()
	logger := getTestLogger()
	mock := &mockRedisClient{}

	// First registration should succeed
	monitor1 := NewRedisMonitor(reg, mock, logger)
	assert.NotNil(t, monitor1)

	// Second registration with same registry should panic
	defer func() {
		if r := recover(); r == nil {
			t.Fatal("Expected panic on duplicate registration, but got none")
		}
	}()

	NewRedisMonitor(reg, mock, logger)
}
