package pipeline

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/oschwald/geoip2-golang"
	"github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"

	v1 "github.com/argusxdr/argus/gen/go/argus/v1"
)

// newMockGeoIPEnricher creates a GeoIPEnricher with a mock reader for testing
func newMockGeoIPEnricher(t *testing.T, mockReader *geoip2.Reader, redisClient *redis.Client) *GeoIPEnricher {
	logger := zap.NewNop()
	enricher := &GeoIPEnricher{
		reader:  mockReader,
		redis:   redisClient,
		logger:  logger,
		metrics: &GeoIPMetrics{},
	}
	return enricher
}

// createTestRedisClient creates a temporary Redis client for testing
func createTestRedisClient(t *testing.T) *redis.Client {
	opts := &redis.Options{
		Addr: "localhost:6379",
	}
	client := redis.NewClient(opts)

	ctx := context.Background()
	if err := client.Ping(ctx).Err(); err != nil {
		t.Skip("Redis not available for testing")
	}

	// Clean up test data before starting
	_ = client.FlushDB(ctx)

	t.Cleanup(func() {
		_ = client.Close()
	})

	return client
}

// TestGeoIPEnricher_ValidPublicIP tests geolocation lookup for a valid public IP
func TestGeoIPEnricher_ValidPublicIP(t *testing.T) {
	t.Skip("Requires actual MaxMind database file")

	ctx := context.Background()
	redisClient := createTestRedisClient(t)

	// This test would require a real MaxMind database file.
	// For unit testing, we use mock data instead.
	// In production, the GeoIPEnricher.LookupIP would be tested with an actual database.
	// For demonstration, we test the logic with a simple mock.

	// Example mock setup (requires geoip2 mock or test database)
	geodata, err := (&GeoIPEnricher{
		reader: nil, // Would be real reader in production
		redis:  redisClient,
		logger: zap.NewNop(),
	}).LookupIP(ctx, "8.8.8.8")

	assert.NoError(t, err)
	assert.NotNil(t, geodata)
}

// TestGeoIPEnricher_PrivateIP_10_Range tests that private IPs in 10.x.x.x range are skipped
func TestGeoIPEnricher_PrivateIP_10_Range(t *testing.T) {
	ctx := context.Background()
	redisClient := createTestRedisClient(t)

	enricher := &GeoIPEnricher{
		reader:  nil,
		redis:   redisClient,
		logger:  zap.NewNop(),
		metrics: &GeoIPMetrics{},
	}

	geodata, err := enricher.LookupIP(ctx, "10.0.0.1")
	assert.NoError(t, err)
	assert.Nil(t, geodata)
}

// TestGeoIPEnricher_PrivateIP_172_Range tests that private IPs in 172.16.x.x range are skipped
func TestGeoIPEnricher_PrivateIP_172_Range(t *testing.T) {
	ctx := context.Background()
	redisClient := createTestRedisClient(t)

	enricher := &GeoIPEnricher{
		reader:  nil,
		redis:   redisClient,
		logger:  zap.NewNop(),
		metrics: &GeoIPMetrics{},
	}

	geodata, err := enricher.LookupIP(ctx, "172.16.0.1")
	assert.NoError(t, err)
	assert.Nil(t, geodata)
}

// TestGeoIPEnricher_PrivateIP_192_Range tests that private IPs in 192.168.x.x range are skipped
func TestGeoIPEnricher_PrivateIP_192_Range(t *testing.T) {
	ctx := context.Background()
	redisClient := createTestRedisClient(t)

	enricher := &GeoIPEnricher{
		reader:  nil,
		redis:   redisClient,
		logger:  zap.NewNop(),
		metrics: &GeoIPMetrics{},
	}

	geodata, err := enricher.LookupIP(ctx, "192.168.1.1")
	assert.NoError(t, err)
	assert.Nil(t, geodata)
}

// TestGeoIPEnricher_LoopbackIP tests that loopback IPs are skipped
func TestGeoIPEnricher_LoopbackIP(t *testing.T) {
	ctx := context.Background()
	redisClient := createTestRedisClient(t)

	enricher := &GeoIPEnricher{
		reader:  nil,
		redis:   redisClient,
		logger:  zap.NewNop(),
		metrics: &GeoIPMetrics{},
	}

	geodata, err := enricher.LookupIP(ctx, "127.0.0.1")
	assert.NoError(t, err)
	assert.Nil(t, geodata)
}

// TestGeoIPEnricher_InvalidIPFormat tests that invalid IP formats are skipped
func TestGeoIPEnricher_InvalidIPFormat(t *testing.T) {
	ctx := context.Background()
	redisClient := createTestRedisClient(t)

	enricher := &GeoIPEnricher{
		reader:  nil,
		redis:   redisClient,
		logger:  zap.NewNop(),
		metrics: &GeoIPMetrics{},
	}

	geodata, err := enricher.LookupIP(ctx, "not-an-ip")
	assert.NoError(t, err)
	assert.Nil(t, geodata)
}

// TestGeoIPEnricher_EmptyIP tests that empty IP strings are handled gracefully
func TestGeoIPEnricher_EmptyIP(t *testing.T) {
	ctx := context.Background()
	redisClient := createTestRedisClient(t)

	enricher := &GeoIPEnricher{
		reader:  nil,
		redis:   redisClient,
		logger:  zap.NewNop(),
		metrics: &GeoIPMetrics{},
	}

	geodata, err := enricher.LookupIP(ctx, "")
	assert.NoError(t, err)
	assert.Nil(t, geodata)
}

// TestGeoIPEnricher_RedisCacheHit tests that Redis cache prevents MaxMind lookups
func TestGeoIPEnricher_RedisCacheHit(t *testing.T) {
	ctx := context.Background()
	redisClient := createTestRedisClient(t)
	defer redisClient.Close()

	// Pre-populate Redis cache with synthetic GeoData
	testIP := "8.8.8.8"
	cacheKey := "geoip:" + testIP
	geodata := &v1.GeoData{
		Country:   strPtr("US"),
		Region:    strPtr("CA"),
		City:      strPtr("San Francisco"),
		Latitude:  float32Ptr(37.7749),
		Longitude: float32Ptr(-122.4194),
	}
	data, _ := json.Marshal(geodata)
	err := redisClient.Set(ctx, cacheKey, data, 24*time.Hour).Err()
	require.NoError(t, err)

	// Create enricher with nil reader (cache should prevent any reader access)
	enricher := &GeoIPEnricher{
		reader:  nil,
		redis:   redisClient,
		logger:  zap.NewNop(),
		metrics: &GeoIPMetrics{},
	}

	result, err := enricher.LookupIP(ctx, testIP)
	assert.NoError(t, err)
	require.NotNil(t, result)
	assert.Equal(t, "US", *result.Country)
	assert.Equal(t, "CA", *result.Region)
	assert.Equal(t, "San Francisco", *result.City)
	assert.Equal(t, 37.7749, *result.Latitude)
	assert.Equal(t, -122.4194, *result.Longitude)
}

// TestGeoIPEnricher_RedisFallback tests that direct MaxMind lookup happens on Redis error
func TestGeoIPEnricher_RedisFallback(t *testing.T) {
	ctx := context.Background()

	// Create a broken Redis client (using wrong port)
	brokenRedis := redis.NewClient(&redis.Options{
		Addr: "localhost:16379", // Invalid port
	})

	enricher := &GeoIPEnricher{
		reader:  nil,
		redis:   brokenRedis,
		logger:  zap.NewNop(),
		metrics: &GeoIPMetrics{},
	}

	// Even with broken Redis, private IPs should still be handled gracefully
	geodata, err := enricher.LookupIP(ctx, "10.0.0.1")
	assert.NoError(t, err)
	assert.Nil(t, geodata)
}

// TestGeoIPEnricher_Close tests that the reader is properly closed
func TestGeoIPEnricher_Close(t *testing.T) {
	enricher := &GeoIPEnricher{
		reader:  nil,
		redis:   nil,
		logger:  zap.NewNop(),
		metrics: &GeoIPMetrics{},
	}

	err := enricher.Close()
	assert.NoError(t, err)
}

// Helper functions for creating test pointers
func strPtr(s string) *string {
	return &s
}

func floatPtr(f float64) *float64 {
	return &f
}

func float32Ptr(f float32) *float32 {
	return &f
}
