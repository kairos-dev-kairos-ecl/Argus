package pipeline

import (
	"context"
	"fmt"

	"github.com/prometheus/client_golang/prometheus"
	"go.uber.org/zap"

	v1 "github.com/argusxdr/argus/gen/go/argus/v1"
)

// Enricher is a Processor that enriches signals with external data (GeoIP, threat intel, risk scoring).
// It is non-fatal: if enrichment data is unavailable, the signal continues through the pipeline.
// Future enrichment sources (ThreatIntel, RiskScore) can be added here.
type Enricher struct {
	geoip   *GeoIPEnricher
	logger  *zap.Logger
	metrics *EnricherMetrics
}

// EnricherMetrics tracks enrichment statistics
type EnricherMetrics struct {
	geoipEnriched prometheus.Counter
	geoipErrors   prometheus.Counter
	signalsSkipped prometheus.Counter
}

// NewEnricher creates a new Enricher processor.
// geoip may be nil (gracefully skipped if so).
func NewEnricher(geoip *GeoIPEnricher, logger *zap.Logger) *Enricher {
	if logger == nil {
		logger = zap.L()
	}

	metrics := &EnricherMetrics{
		geoipEnriched: prometheus.NewCounter(prometheus.CounterOpts{
			Name: "argus_pipeline_enricher_geoip_enriched_total",
			Help: "Total number of signals enriched with GeoIP data",
		}),
		geoipErrors: prometheus.NewCounter(prometheus.CounterOpts{
			Name: "argus_pipeline_enricher_geoip_errors_total",
			Help: "Total number of GeoIP enrichment errors (non-fatal)",
		}),
		signalsSkipped: prometheus.NewCounter(prometheus.CounterOpts{
			Name: "argus_pipeline_enricher_signals_skipped_total",
			Help: "Total number of signals skipped by enricher (e.g., no enrichment available)",
		}),
	}

	return &Enricher{
		geoip:   geoip,
		logger:  logger,
		metrics: metrics,
	}
}

// Process applies enrichment to a signal.
// Initializes sig.Enrichment if nil, then calls each enrichment helper.
// All enrichment errors are non-fatal; the signal is returned unchanged if enrichment fails.
// Returns (sig, nil) always (non-fatal error handling).
func (e *Enricher) Process(ctx context.Context, sig *v1.ArgusSignal) (*v1.ArgusSignal, error) {
	if sig == nil {
		return nil, nil
	}

	// Initialize Enrichment message if nil
	if sig.Enrichment == nil {
		sig.Enrichment = &v1.Enrichment{}
	}

	// Call GeoIP enrichment if available
	if e.geoip != nil {
		if err := e.enrichGeoIP(ctx, sig); err != nil {
			e.metrics.geoipErrors.Inc()
			e.logger.Debug("geoip enrichment failed",
				zap.String("signal_id", sig.SignalId),
				zap.Error(err),
			)
			// Continue — non-fatal error
		}
	}

	// Future: Call enrichThreatIntel (stub for now)
	// if err := e.enrichThreatIntel(ctx, sig) { ... }

	// Future: Call enrichRiskScore (stub for now)
	// if err := e.enrichRiskScore(ctx, sig) { ... }

	return sig, nil
}

// Name returns the processor's name for logging and metrics.
func (e *Enricher) Name() string {
	return "Enricher"
}

// enrichGeoIP extracts an IP from the signal and performs geolocation lookup.
// IP extraction is a placeholder for now; Phase 1 schema may not yet have IP fields in contexts.
// Returns nil on error (non-fatal — logging only).
func (e *Enricher) enrichGeoIP(ctx context.Context, sig *v1.ArgusSignal) error {
	// Extract IP from signal context (placeholder for now)
	// In a future phase, once IP fields are added to layer contexts, extract from there.
	ip := e.extractIP(sig)
	if ip == "" {
		// No IP available — return nil (non-fatal)
		return nil
	}

	// Perform GeoIP lookup
	geodata, err := e.geoip.LookupIP(ctx, ip)
	if err != nil {
		// LookupIP returns non-fatal errors; just log
		e.logger.Debug("geoip lookup error",
			zap.String("ip", ip),
			zap.Error(err),
		)
		return err
	}

	// If geodata was found, populate Enrichment
	if geodata != nil {
		if sig.Enrichment == nil {
			sig.Enrichment = &v1.Enrichment{}
		}
		sig.Enrichment.Geo = geodata
		e.metrics.geoipEnriched.Inc()
		e.logger.Debug("geoip enrichment succeeded",
			zap.String("signal_id", sig.SignalId),
			zap.String("ip", ip),
		)
	}

	return nil
}

// extractIP attempts to extract an IP address from the signal.
// This is a placeholder implementation; IP fields should be added to layer contexts in Phase 1.
// For now, this returns empty string (no IP available).
func (e *Enricher) extractIP(sig *v1.ArgusSignal) string {
	// Placeholder: attempt to extract from known locations
	// TODO: Once layer contexts have IP fields (e.g., context.ip), extract from there

	// For now, return empty — signals don't yet have IP fields
	return ""
}

// RegisterMetrics registers enricher metrics with Prometheus.
func (e *Enricher) RegisterMetrics(reg prometheus.Registerer) error {
	if err := reg.Register(e.metrics.geoipEnriched); err != nil {
		return fmt.Errorf("failed to register geoip enriched metric: %w", err)
	}
	if err := reg.Register(e.metrics.geoipErrors); err != nil {
		return fmt.Errorf("failed to register geoip errors metric: %w", err)
	}
	if err := reg.Register(e.metrics.signalsSkipped); err != nil {
		return fmt.Errorf("failed to register signals skipped metric: %w", err)
	}
	return nil
}
