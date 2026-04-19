package metrics

import "github.com/prometheus/client_golang/prometheus"

// Detection holds all Prometheus metrics for the detection subsystem.
// Implements D-18 requirements.
type Detection struct {
	// argus_detection_latency_seconds{tier} Histogram
	Latency *prometheus.HistogramVec

	// argus_detection_queue_depth Gauge
	QueueDepth prometheus.Gauge

	// argus_rule_evaluations_total{tier} Counter
	Evaluations *prometheus.CounterVec

	// argus_alert_created_total{severity,rule_id} Counter
	AlertsCreated *prometheus.CounterVec

	// argus_detection_dropped_total{severity} Counter
	Dropped *prometheus.CounterVec

	// kairos_timeout_total Counter
	KairosTimeout prometheus.Counter
}

// NewDetection registers and returns Detection metrics.
// Panics if registration fails (metrics must be unique per registry).
func NewDetection(reg prometheus.Registerer) *Detection {
	latency := prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "argus_detection_latency_seconds",
			Help:    "Detection evaluation latency in seconds",
			Buckets: []float64{.005, .01, .025, .05, .1, .25, .5, 1.0},
		},
		[]string{"tier"},
	)

	queueDepth := prometheus.NewGauge(prometheus.GaugeOpts{
		Name: "argus_detection_queue_depth",
		Help: "Current number of signals in the detection queue",
	})

	evaluations := prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "argus_rule_evaluations_total",
			Help: "Total number of rule evaluations by tier",
		},
		[]string{"tier"},
	)

	alertsCreated := prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "argus_alert_created_total",
			Help: "Total number of alerts created by severity and rule_id",
		},
		[]string{"severity", "rule_id"},
	)

	dropped := prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "argus_detection_dropped_total",
			Help: "Total number of detection signals dropped by severity",
		},
		[]string{"severity"},
	)

	kairosTimeout := prometheus.NewCounter(prometheus.CounterOpts{
		Name: "kairos_timeout_total",
		Help: "Total number of Kairos policy evaluation timeouts",
	})

	reg.MustRegister(latency)
	reg.MustRegister(queueDepth)
	reg.MustRegister(evaluations)
	reg.MustRegister(alertsCreated)
	reg.MustRegister(dropped)
	reg.MustRegister(kairosTimeout)

	return &Detection{
		Latency:       latency,
		QueueDepth:    queueDepth,
		Evaluations:   evaluations,
		AlertsCreated: alertsCreated,
		Dropped:       dropped,
		KairosTimeout: kairosTimeout,
	}
}
