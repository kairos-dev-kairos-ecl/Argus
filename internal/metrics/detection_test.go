package metrics

import (
	"testing"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestDetection_AllMetricsRegistered(t *testing.T) {
	reg := prometheus.NewRegistry()
	d := NewDetection(reg)
	require.NotNil(t, d)

	// Trigger observations so metrics appear in Gather output
	d.Latency.WithLabelValues("2").Observe(0.01)
	d.QueueDepth.Set(1)
	d.Evaluations.WithLabelValues("2").Inc()
	d.AlertsCreated.WithLabelValues("high", "rule-1").Inc()
	d.Dropped.WithLabelValues("low").Inc()
	d.KairosTimeout.Inc()

	mfs, err := reg.Gather()
	require.NoError(t, err)

	names := make(map[string]bool)
	for _, mf := range mfs {
		names[mf.GetName()] = true
	}

	expected := []string{
		"argus_detection_latency_seconds",
		"argus_detection_queue_depth",
		"argus_rule_evaluations_total",
		"argus_alert_created_total",
		"argus_detection_dropped_total",
		"kairos_timeout_total",
	}
	for _, name := range expected {
		assert.True(t, names[name], "missing metric: %s", name)
	}
}

func TestDetection_DropLabels(t *testing.T) {
	reg := prometheus.NewRegistry()
	d := NewDetection(reg)
	require.NotNil(t, d)

	d.Dropped.WithLabelValues("low").Inc()
	d.Dropped.WithLabelValues("medium").Inc()
	d.Dropped.WithLabelValues("critical").Inc()

	mfs, err := reg.Gather()
	require.NoError(t, err)

	var dropCount int
	for _, mf := range mfs {
		if mf.GetName() == "argus_detection_dropped_total" {
			dropCount = len(mf.GetMetric())
			break
		}
	}
	assert.Equal(t, 3, dropCount, "expected 3 label values (low, medium, critical)")
}
