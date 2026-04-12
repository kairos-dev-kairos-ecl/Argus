package pipeline

import (
	"time"
)

// PipelineConfig holds all tunable parameters for the processing pipeline.
// Fields are populated from environment variables and YAML configuration.
type PipelineConfig struct {
	// General
	Enabled                   bool          `mapstructure:"enabled" default:"true"`
	BufferSize                int           `mapstructure:"buffer_size" default:"1000"`
	WorkerCount               int           `mapstructure:"worker_count" default:"0"` // 0 = GOMAXPROCS * 2
	ShutdownTimeout           time.Duration `mapstructure:"shutdown_timeout" default:"30s"`

	// Validation (Task 3.3)
	ValidationEnabled         bool          `mapstructure:"validation_enabled" default:"true"`

	// Normalization (Task 3.3B)
	NormalizationEnabled      bool          `mapstructure:"normalization_enabled" default:"true"`

	// Enrichment (Task 3.6)
	EnrichmentEnabled         bool          `mapstructure:"enrichment_enabled" default:"true"`
	EnrichmentTimeoutPerSignal time.Duration `mapstructure:"enrichment_timeout_per_signal" default:"5s"`

	// Correlation (Task 3.4)
	CorrelationEnabled        bool          `mapstructure:"correlation_enabled" default:"true"`
	CorrelationWindow         time.Duration `mapstructure:"correlation_window" default:"5s"` // Signals within this window are considered related
	CorrelationMaxPerTrace    int           `mapstructure:"correlation_max_per_trace" default:"1000"` // Max signals per trace_id in Redis

	// Baseline Scoring (Task 3.5)
	BaselineEnabled           bool          `mapstructure:"baseline_enabled" default:"true"`
	BaselineEngineAsync       bool          `mapstructure:"baseline_engine_async" default:"true"` // Compute baselines in background loop
	BaselineComputeInterval   time.Duration `mapstructure:"baseline_compute_interval" default:"10m"` // How often to recompute baseline profiles
	BaselineHistoryWindow     time.Duration `mapstructure:"baseline_history_window" default:"24h"` // How far back to query for baseline computation

	// GeoIP (Task 3.5)
	GeoIPEnabled              bool          `mapstructure:"geoip_enabled" default:"false"`
	GeoIPDatabasePath         string        `mapstructure:"geoip_database_path" default:"./data/GeoLite2-City.mmdb"`
	GeoIPCacheSize            int           `mapstructure:"geoip_cache_size" default:"10000"` // LRU cache size for IP→location lookups
	GeoIPCacheTTL             time.Duration `mapstructure:"geoip_cache_ttl" default:"1h"`

	// Routing (Task 3.6B)
	RouterEnabled             bool          `mapstructure:"router_enabled" default:"true"`
	StorageTimeout            time.Duration `mapstructure:"storage_timeout" default:"10s"` // Timeout for writing to ClickHouse/PostgreSQL
}

// NewPipelineConfig returns a PipelineConfig with default values.
func NewPipelineConfig() PipelineConfig {
	return PipelineConfig{
		Enabled:                   true,
		BufferSize:                1000,
		WorkerCount:               0, // Will be set to GOMAXPROCS * 2 at runtime
		ShutdownTimeout:           30 * time.Second,
		ValidationEnabled:         true,
		NormalizationEnabled:      true,
		EnrichmentEnabled:         true,
		EnrichmentTimeoutPerSignal: 5 * time.Second,
		CorrelationEnabled:        true,
		CorrelationWindow:         5 * time.Second,
		CorrelationMaxPerTrace:    1000,
		BaselineEnabled:           true,
		BaselineEngineAsync:       true,
		BaselineComputeInterval:   10 * time.Minute,
		BaselineHistoryWindow:     24 * time.Hour,
		GeoIPEnabled:              false,
		GeoIPDatabasePath:         "./data/GeoLite2-City.mmdb",
		GeoIPCacheSize:            10000,
		GeoIPCacheTTL:             1 * time.Hour,
		RouterEnabled:             true,
		StorageTimeout:            10 * time.Second,
	}
}

// Validate checks that the configuration is reasonable.
func (c *PipelineConfig) Validate() error {
	if c.BufferSize < 1 {
		return ErrInvalidConfig("buffer_size must be >= 1")
	}
	if c.CorrelationWindow < time.Second {
		return ErrInvalidConfig("correlation_window must be >= 1 second")
	}
	if c.BaselineComputeInterval < time.Minute {
		return ErrInvalidConfig("baseline_compute_interval must be >= 1 minute")
	}
	if c.ShutdownTimeout < time.Second {
		return ErrInvalidConfig("shutdown_timeout must be >= 1 second")
	}
	return nil
}

// ConfigError wraps configuration validation errors
type ConfigError struct {
	Message string
}

func (e ConfigError) Error() string {
	return e.Message
}

func ErrInvalidConfig(msg string) error {
	return ConfigError{Message: "invalid pipeline config: " + msg}
}
