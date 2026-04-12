package pipeline

import (
	"context"
	"encoding/json"
	"fmt"
	"net"
	"os"
	"time"

	"github.com/oschwald/geoip2-golang"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/redis/go-redis/v9"
	"go.uber.org/zap"

	v1 "github.com/argusxdr/argus/gen/go/argus/v1"
)

// GeoIPEnricher provides geolocation enrichment for IP addresses using MaxMind GeoLite2 database
// and Redis caching to prevent repeated lookups.
type GeoIPEnricher struct {
	reader  *geoip2.Reader
	redis   *redis.Client
	logger  *zap.Logger
	metrics *GeoIPMetrics
}

// GeoIPMetrics tracks GeoIP enrichment statistics
type GeoIPMetrics struct {
	enriched prometheus.Counter
	errors   prometheus.Counter
	cached   prometheus.Counter
}

// NewGeoIPEnricher creates a new GeoIPEnricher and opens the MaxMind database.
// Returns error if the database file cannot be opened or doesn't exist.
// Checks database freshness on initialization; logs warning if > 30 days old.
func NewGeoIPEnricher(dbPath string, redisClient *redis.Client, logger *zap.Logger) (*GeoIPEnricher, error) {
	if logger == nil {
		logger = zap.L()
	}

	// Open MaxMind database
	reader, err := geoip2.Open(dbPath)
	if err != nil {
		return nil, fmt.Errorf("failed to open GeoIP database at %s: %w", dbPath, err)
	} else if reader == nil {
		return nil, fmt.Errorf("GeoIP database returned nil reader at %s", dbPath)
	}

	// Initialize enricher to check database age
	ge := &GeoIPEnricher{
		reader:  reader,
		redis:   redisClient,
		logger:  logger,
		metrics: nil, // will be set below
	}

	// Check database file modification time for freshness
	// Note: CheckDatabaseAge does not return error for staleness, only for inaccessible files
	if err := ge.CheckDatabaseAge(dbPath); err != nil {
		return nil, err
	}

	// Initialize metrics
	metrics := &GeoIPMetrics{
		enriched: prometheus.NewCounter(prometheus.CounterOpts{
			Name: "argus_pipeline_geoip_enriched_total",
			Help: "Total number of signals enriched with GeoIP data",
		}),
		errors: prometheus.NewCounter(prometheus.CounterOpts{
			Name: "argus_pipeline_geoip_errors_total",
			Help: "Total number of GeoIP lookup errors",
		}),
		cached: prometheus.NewCounter(prometheus.CounterOpts{
			Name: "argus_pipeline_geoip_cached_total",
			Help: "Total number of GeoIP lookups from cache",
		}),
	}

	ge.metrics = metrics
	return ge, nil
}

// LookupIP performs a geolocation lookup for the given IP address.
// Returns:
// - (*v1.GeoData, nil) if lookup succeeds
// - (nil, nil) if IP is private/loopback, invalid format, or lookup fails (non-fatal)
// - (nil, error) only for critical errors
func (ge *GeoIPEnricher) LookupIP(ctx context.Context, ip string) (*v1.GeoData, error) {
	if ip == "" {
		return nil, nil
	}

	// Parse IP address
	parsedIP := net.ParseIP(ip)
	if parsedIP == nil {
		// Invalid IP format — skip enrichment
		ge.logger.Debug("invalid IP format", zap.String("ip", ip))
		return nil, nil
	}

	// Skip private/loopback IPs
	if parsedIP.IsPrivate() || parsedIP.IsLoopback() {
		ge.logger.Debug("skipping private/loopback IP", zap.String("ip", ip))
		return nil, nil
	}

	// Check Redis cache first
	cacheKey := fmt.Sprintf("geoip:%s", ip)
	cachedData, err := ge.redis.Get(ctx, cacheKey).Result()
	if err == nil {
		// Cache hit
		var geodata v1.GeoData
		if err := json.Unmarshal([]byte(cachedData), &geodata); err == nil {
			ge.metrics.cached.Inc()
			ge.logger.Debug("geoip cache hit", zap.String("ip", ip))
			return &geodata, nil
		}
		// If unmarshal fails, fall through to direct lookup
	} else if err != redis.Nil {
		// Redis error — log but continue with direct lookup
		ge.logger.Debug("redis cache lookup error", zap.String("ip", ip), zap.Error(err))
	}

	// Query MaxMind directly
	record, err := ge.reader.City(parsedIP)
	if err != nil {
		// MaxMind lookup failure — log at Debug level, return nil, nil (non-fatal)
		ge.logger.Debug("geoip lookup failed", zap.String("ip", ip), zap.Error(err))
		ge.metrics.errors.Inc()
		return nil, nil
	}

	// Populate GeoData from MaxMind response
	cityName := record.City.Names["en"]
	latitude := float32(record.Location.Latitude)
	longitude := float32(record.Location.Longitude)

	geodata := &v1.GeoData{
		Country:   &record.Country.IsoCode,
		Region:    nil,
		City:      &cityName,
		Latitude:  &latitude,
		Longitude: &longitude,
	}

	// Include region if available
	if len(record.Subdivisions) > 0 {
		regionName := record.Subdivisions[0].Names["en"]
		geodata.Region = &regionName
	}

	// Cache in Redis with 24-hour TTL
	data, _ := json.Marshal(geodata)
	if err := ge.redis.Set(ctx, cacheKey, data, 24*time.Hour).Err(); err != nil {
		ge.logger.Debug("failed to cache geoip data", zap.String("ip", ip), zap.Error(err))
	}

	ge.metrics.enriched.Inc()
	ge.logger.Debug("geoip lookup succeeded", zap.String("ip", ip), zap.String("country", record.Country.IsoCode))
	return geodata, nil
}

// RegisterMetrics registers GeoIP metrics with Prometheus
func (ge *GeoIPEnricher) RegisterMetrics(reg prometheus.Registerer) error {
	if err := reg.Register(ge.metrics.enriched); err != nil {
		return fmt.Errorf("failed to register geoip enriched metric: %w", err)
	}
	if err := reg.Register(ge.metrics.errors); err != nil {
		return fmt.Errorf("failed to register geoip errors metric: %w", err)
	}
	if err := reg.Register(ge.metrics.cached); err != nil {
		return fmt.Errorf("failed to register geoip cached metric: %w", err)
	}
	return nil
}

// CheckDatabaseAge checks the MaxMind database file modification time and logs warnings/errors
// if the database is stale (>25 days old triggers warning, >30 days old triggers error).
// Returns error only if the database file is inaccessible; staleness does not cause errors.
func (ge *GeoIPEnricher) CheckDatabaseAge(dbPath string) error {
	fileInfo, err := os.Stat(dbPath)
	if err != nil {
		ge.logger.Error("failed to stat GeoIP database file",
			zap.String("db_path", dbPath),
			zap.Error(err),
		)
		return fmt.Errorf("failed to stat GeoIP database: %w", err)
	}

	age := time.Since(fileInfo.ModTime())

	if age > 30*24*time.Hour {
		ge.logger.Error("GeoIP database is very stale",
			zap.String("db_path", dbPath),
			zap.Duration("age", age),
			zap.Time("last_modified", fileInfo.ModTime()),
		)
	} else if age > 25*24*time.Hour {
		ge.logger.Warn("GeoIP database is stale",
			zap.String("db_path", dbPath),
			zap.Duration("age", age),
			zap.Time("last_modified", fileInfo.ModTime()),
		)
	} else {
		ge.logger.Debug("GeoIP database is fresh",
			zap.String("db_path", dbPath),
			zap.Duration("age", age),
		)
	}

	return nil
}

// UpdateDatabase performs a hot-reload of the MaxMind database.
// It closes the current reader and opens a new one at the specified path.
// This allows database updates without server downtime.
// Returns error if the update fails; on error, the original reader is preserved.
func (ge *GeoIPEnricher) UpdateDatabase(ctx context.Context, newDBPath string) error {
	// Validate new database path
	if _, err := os.Stat(newDBPath); err != nil {
		return fmt.Errorf("new database file not accessible: %w", err)
	}

	// Open new reader
	newReader, err := geoip2.Open(newDBPath)
	if err != nil {
		return fmt.Errorf("failed to open new GeoIP database at %s: %w", newDBPath, err)
	}

	// Close old reader
	if ge.reader != nil {
		if err := ge.reader.Close(); err != nil {
			ge.logger.Warn("error closing old GeoIP reader during update",
				zap.Error(err),
			)
		}
	}

	// Swap readers (atomic operation)
	ge.reader = newReader

	ge.logger.Info("GeoIP database updated successfully",
		zap.String("new_db_path", newDBPath),
	)

	// Check age of newly updated database
	if err := ge.CheckDatabaseAge(newDBPath); err != nil {
		ge.logger.Warn("failed to check age of updated database",
			zap.Error(err),
		)
	}

	return nil
}

// Close closes the MaxMind database reader
func (ge *GeoIPEnricher) Close() error {
	if ge.reader != nil {
		return ge.reader.Close()
	}
	return nil
}
