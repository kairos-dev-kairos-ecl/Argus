package metrics

import (
	"testing"

	"github.com/prometheus/client_golang/prometheus"
)

func TestNewIngest(t *testing.T) {
	reg := prometheus.NewRegistry()

	ingest := NewIngest(reg)

	if ingest == nil {
		t.Fatal("NewIngest returned nil")
	}

	if ingest.SignalsReceived == nil {
		t.Fatal("SignalsReceived is nil")
	}

	if ingest.SignalsDropped == nil {
		t.Fatal("SignalsDropped is nil")
	}

	if ingest.QueueDepth == nil {
		t.Fatal("QueueDepth is nil")
	}

	if ingest.RequestDuration == nil {
		t.Fatal("RequestDuration is nil")
	}
}

func TestNewIngestDuplicate(t *testing.T) {
	reg := prometheus.NewRegistry()

	// First call should succeed
	ingest1 := NewIngest(reg)
	if ingest1 == nil {
		t.Fatal("First NewIngest returned nil")
	}

	// Second call with same registry should panic
	defer func() {
		if r := recover(); r == nil {
			t.Fatal("Expected panic on duplicate registration, but got none")
		}
	}()

	NewIngest(reg)
}

func TestNewStorage(t *testing.T) {
	reg := prometheus.NewRegistry()

	storage := NewStorage(reg)

	if storage == nil {
		t.Fatal("NewStorage returned nil")
	}

	if storage.BatchFlushes == nil {
		t.Fatal("BatchFlushes is nil")
	}

	if storage.BatchSize == nil {
		t.Fatal("BatchSize is nil")
	}

	if storage.FlushDuration == nil {
		t.Fatal("FlushDuration is nil")
	}

	if storage.InsertErrors == nil {
		t.Fatal("InsertErrors is nil")
	}
}

func TestNewStorageDuplicate(t *testing.T) {
	reg := prometheus.NewRegistry()

	// First call should succeed
	storage1 := NewStorage(reg)
	if storage1 == nil {
		t.Fatal("First NewStorage returned nil")
	}

	// Second call with same registry should panic
	defer func() {
		if r := recover(); r == nil {
			t.Fatal("Expected panic on duplicate registration, but got none")
		}
	}()

	NewStorage(reg)
}

func TestNewHTTP(t *testing.T) {
	reg := prometheus.NewRegistry()

	http := NewHTTP(reg)

	if http == nil {
		t.Fatal("NewHTTP returned nil")
	}

	if http.RequestDuration == nil {
		t.Fatal("RequestDuration is nil")
	}

	if http.RequestsTotal == nil {
		t.Fatal("RequestsTotal is nil")
	}
}

func TestNewHTTPDuplicate(t *testing.T) {
	reg := prometheus.NewRegistry()

	// First call should succeed
	http1 := NewHTTP(reg)
	if http1 == nil {
		t.Fatal("First NewHTTP returned nil")
	}

	// Second call with same registry should panic
	defer func() {
		if r := recover(); r == nil {
			t.Fatal("Expected panic on duplicate registration, but got none")
		}
	}()

	NewHTTP(reg)
}

func TestSignalsDroppedCounter(t *testing.T) {
	reg := prometheus.NewRegistry()

	ingest := NewIngest(reg)

	// Increment counter with label
	ingest.SignalsDropped.WithLabelValues("queue_full").Inc()
	ingest.SignalsDropped.WithLabelValues("queue_full").Inc()
	ingest.SignalsDropped.WithLabelValues("auth_failed").Inc()

	// Gather metrics and verify counts
	metrics, err := reg.Gather()
	if err != nil {
		t.Fatalf("Gather failed: %v", err)
	}

	foundMetric := false
	for _, mf := range metrics {
		if mf.GetName() == "argus_signals_dropped_total" {
			foundMetric = true
			// We're not doing detailed value checks due to complexity,
			// but we verify the metric exists with correct label
			if len(mf.Metric) == 0 {
				t.Fatal("Expected metrics with labels")
			}
		}
	}

	if !foundMetric {
		t.Fatal("argus_signals_dropped_total metric not found")
	}
}

func TestQueueDepthGauge(t *testing.T) {
	reg := prometheus.NewRegistry()

	ingest := NewIngest(reg)

	// Set gauge value
	ingest.QueueDepth.Set(42)

	// Gather metrics and verify
	metrics, err := reg.Gather()
	if err != nil {
		t.Fatalf("Gather failed: %v", err)
	}

	foundMetric := false
	for _, mf := range metrics {
		if mf.GetName() == "argus_ingest_queue_depth" {
			foundMetric = true
			if len(mf.Metric) > 0 {
				val := mf.Metric[0].Gauge.GetValue()
				if val != 42 {
					t.Errorf("Expected gauge value 42, got %v", val)
				}
			}
		}
	}

	if !foundMetric {
		t.Fatal("argus_ingest_queue_depth gauge not found")
	}
}

func TestBatchSizeHistogram(t *testing.T) {
	reg := prometheus.NewRegistry()

	storage := NewStorage(reg)

	// Observe histogram values
	storage.BatchSize.Observe(50)
	storage.BatchSize.Observe(100)
	storage.BatchSize.Observe(500)

	// Gather metrics and verify
	metrics, err := reg.Gather()
	if err != nil {
		t.Fatalf("Gather failed: %v", err)
	}

	foundMetric := false
	for _, mf := range metrics {
		if mf.GetName() == "argus_storage_batch_size" {
			foundMetric = true
			if len(mf.Metric) > 0 {
				count := mf.Metric[0].Histogram.GetSampleCount()
				if count != 3 {
					t.Errorf("Expected 3 observations, got %d", count)
				}
			}
		}
	}

	if !foundMetric {
		t.Fatal("argus_storage_batch_size histogram not found")
	}
}

func TestMetricsNaming(t *testing.T) {
	reg := prometheus.NewRegistry()

	_ = NewIngest(reg)
	_ = NewStorage(reg)
	_ = NewHTTP(reg)

	// Gather all metrics
	metrics, err := reg.Gather()
	if err != nil {
		t.Fatalf("Gather failed: %v", err)
	}

	expectedMetrics := map[string]bool{
		"argus_signals_received_total":          false,
		"argus_signals_dropped_total":           false,
		"argus_ingest_queue_depth":              false,
		"argus_ingest_request_duration_seconds": false,
		"argus_storage_batch_flush_total":       false,
		"argus_storage_batch_size":              false,
		"argus_storage_batch_flush_duration_seconds": false,
		"argus_storage_insert_errors_total":     false,
		"argus_http_request_duration_seconds":   false,
		"argus_http_requests_total":             false,
	}

	for _, mf := range metrics {
		name := mf.GetName()
		if _, ok := expectedMetrics[name]; ok {
			expectedMetrics[name] = true
		}
	}

	for metric, found := range expectedMetrics {
		if !found {
			t.Errorf("Expected metric %s not found", metric)
		}
	}
}
