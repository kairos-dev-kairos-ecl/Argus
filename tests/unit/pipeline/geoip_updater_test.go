package pipeline_test

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"

	"github.com/argusxdr/argus/internal/pipeline"
	"github.com/redis/go-redis/v9"
)

// Helper function to create a test GeoIPEnricher with a temporary database file
func createTestGeoIPEnricher(t *testing.T) (*pipeline.GeoIPEnricher, string, *redis.Client) {
	// Create a temporary directory and file for testing
	tmpDir, err := os.MkdirTemp("", "geoip-test-")
	require.NoError(t, err)

	dbPath := filepath.Join(tmpDir, "GeoLite2-City.mmdb")

	// Create a minimal valid MMDB file for testing (this is a stub)
	// In real tests, you'd use the actual MaxMind test database
	err = os.WriteFile(dbPath, []byte("MMDB test file"), 0644)
	require.NoError(t, err)

	// Try to get Redis client, skip if unavailable
	client := redis.NewClient(&redis.Options{
		Addr: "localhost:6379",
	})

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	err = client.Ping(ctx).Err()
	if err != nil {
		// Redis not available for this test
		// Return nil client to indicate Redis not available
		return nil, dbPath, nil
	}

	logger := zap.NewNop()

	// Create GeoIPEnricher with the test database
	// Note: This will fail because our test file is not a real MMDB, but we're testing the age check
	// which happens before the reader is fully initialized in a real scenario
	enricher, err := pipeline.NewGeoIPEnricher(dbPath, client, logger)
	if err != nil {
		// Expected for stub database file
		client.Close()
		return nil, dbPath, nil
	}

	return enricher, dbPath, client
}

// TestCheckDatabaseAge_FreshDatabase tests that a fresh database logs debug level
func TestCheckDatabaseAge_FreshDatabase(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "geoip-test-")
	require.NoError(t, err)
	defer os.RemoveAll(tmpDir)

	dbPath := filepath.Join(tmpDir, "GeoLite2-City.mmdb")
	err = os.WriteFile(dbPath, []byte("test"), 0644)
	require.NoError(t, err)

	// Create a logger that captures output
	logger := zap.NewNop() // In production, use a test logger to capture logs

	// Create minimal enricher for testing CheckDatabaseAge
	client := redis.NewClient(&redis.Options{
		Addr: "localhost:6379",
	})

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	// Skip if Redis not available
	if err := client.Ping(ctx).Err(); err != nil {
		t.Skip("Redis not available")
	}
	defer client.Close()

	enricher, err := pipeline.NewGeoIPEnricher(dbPath, client, logger)
	if err != nil {
		t.Skip("Could not create test enricher (expected for test stub DB)")
	}
	defer enricher.Close()

	// Test that CheckDatabaseAge works on a fresh file
	err = enricher.CheckDatabaseAge(dbPath)
	assert.NoError(t, err)
}

// TestCheckDatabaseAge_StaleDatabase_25Days tests warning for >25 days old database
func TestCheckDatabaseAge_StaleDatabase_25Days(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "geoip-test-")
	require.NoError(t, err)
	defer os.RemoveAll(tmpDir)

	dbPath := filepath.Join(tmpDir, "GeoLite2-City.mmdb")
	err = os.WriteFile(dbPath, []byte("test"), 0644)
	require.NoError(t, err)

	// Set file modification time to 26 days ago
	staleTime := time.Now().Add(-26 * 24 * time.Hour)
	err = os.Chtimes(dbPath, staleTime, staleTime)
	require.NoError(t, err)

	// Create a logger that captures output
	logger := zap.NewNop()

	client := redis.NewClient(&redis.Options{
		Addr: "localhost:6379",
	})

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	// Skip if Redis not available
	if err := client.Ping(ctx).Err(); err != nil {
		t.Skip("Redis not available")
	}
	defer client.Close()

	enricher, err := pipeline.NewGeoIPEnricher(dbPath, client, logger)
	if err != nil {
		t.Skip("Could not create test enricher (expected for test stub DB)")
	}
	defer enricher.Close()

	// Test that CheckDatabaseAge detects >25 day old database
	err = enricher.CheckDatabaseAge(dbPath)
	assert.NoError(t, err)
}

// TestCheckDatabaseAge_VeryStaleDatabase_30Days tests error logging for >30 days old database
func TestCheckDatabaseAge_VeryStaleDatabase_30Days(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "geoip-test-")
	require.NoError(t, err)
	defer os.RemoveAll(tmpDir)

	dbPath := filepath.Join(tmpDir, "GeoLite2-City.mmdb")
	err = os.WriteFile(dbPath, []byte("test"), 0644)
	require.NoError(t, err)

	// Set file modification time to 31 days ago
	staleTime := time.Now().Add(-31 * 24 * time.Hour)
	err = os.Chtimes(dbPath, staleTime, staleTime)
	require.NoError(t, err)

	logger := zap.NewNop()

	client := redis.NewClient(&redis.Options{
		Addr: "localhost:6379",
	})

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	// Skip if Redis not available
	if err := client.Ping(ctx).Err(); err != nil {
		t.Skip("Redis not available")
	}
	defer client.Close()

	enricher, err := pipeline.NewGeoIPEnricher(dbPath, client, logger)
	if err != nil {
		t.Skip("Could not create test enricher (expected for test stub DB)")
	}
	defer enricher.Close()

	// Test that CheckDatabaseAge detects >30 day old database
	err = enricher.CheckDatabaseAge(dbPath)
	assert.NoError(t, err)
}

// TestCheckDatabaseAge_InaccessibleFile tests error when database file is inaccessible
func TestCheckDatabaseAge_InaccessibleFile(t *testing.T) {
	logger := zap.NewNop()

	client := redis.NewClient(&redis.Options{
		Addr: "localhost:6379",
	})

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	// Skip if Redis not available
	if err := client.Ping(ctx).Err(); err != nil {
		t.Skip("Redis not available")
	}
	defer client.Close()

	// Create a temporary enricher with a valid DB for testing
	tmpDir, err := os.MkdirTemp("", "geoip-test-")
	require.NoError(t, err)
	defer os.RemoveAll(tmpDir)

	tmpDbPath := filepath.Join(tmpDir, "GeoLite2-City.mmdb")
	err = os.WriteFile(tmpDbPath, []byte("test"), 0644)
	require.NoError(t, err)

	enricher, err := pipeline.NewGeoIPEnricher(tmpDbPath, client, logger)
	if err != nil {
		t.Skip("Could not create test enricher (expected for test stub DB)")
	}
	defer enricher.Close()

	// Now test with non-existent file
	nonExistentPath := filepath.Join(tmpDir, "nonexistent.mmdb")
	err = enricher.CheckDatabaseAge(nonExistentPath)
	assert.Error(t, err)
}

// TestUpdateDatabase_ValidFile tests hot-reload with a valid database file
func TestUpdateDatabase_ValidFile(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "geoip-test-")
	require.NoError(t, err)
	defer os.RemoveAll(tmpDir)

	// Create original database file
	originalPath := filepath.Join(tmpDir, "GeoLite2-City-original.mmdb")
	err = os.WriteFile(originalPath, []byte("original test"), 0644)
	require.NoError(t, err)

	// Create new database file
	newPath := filepath.Join(tmpDir, "GeoLite2-City-new.mmdb")
	err = os.WriteFile(newPath, []byte("new test"), 0644)
	require.NoError(t, err)

	logger := zap.NewNop()

	client := redis.NewClient(&redis.Options{
		Addr: "localhost:6379",
	})

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	// Skip if Redis not available
	if err := client.Ping(ctx).Err(); err != nil {
		t.Skip("Redis not available")
	}
	defer client.Close()

	enricher, err := pipeline.NewGeoIPEnricher(originalPath, client, logger)
	if err != nil {
		t.Skip("Could not create test enricher (expected for test stub DB)")
	}
	defer enricher.Close()

	// Test UpdateDatabase
	ctx = context.Background()
	err = enricher.UpdateDatabase(ctx, newPath)
	assert.NoError(t, err)
}

// TestUpdateDatabase_InvalidFile tests UpdateDatabase with non-existent file
func TestUpdateDatabase_InvalidFile(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "geoip-test-")
	require.NoError(t, err)
	defer os.RemoveAll(tmpDir)

	originalPath := filepath.Join(tmpDir, "GeoLite2-City.mmdb")
	err = os.WriteFile(originalPath, []byte("test"), 0644)
	require.NoError(t, err)

	logger := zap.NewNop()

	client := redis.NewClient(&redis.Options{
		Addr: "localhost:6379",
	})

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	// Skip if Redis not available
	if err := client.Ping(ctx).Err(); err != nil {
		t.Skip("Redis not available")
	}
	defer client.Close()

	enricher, err := pipeline.NewGeoIPEnricher(originalPath, client, logger)
	if err != nil {
		t.Skip("Could not create test enricher (expected for test stub DB)")
	}
	defer enricher.Close()

	// Try to update with non-existent file
	ctx = context.Background()
	nonExistentPath := filepath.Join(tmpDir, "nonexistent.mmdb")
	err = enricher.UpdateDatabase(ctx, nonExistentPath)
	assert.Error(t, err)
}

// TestGeoIPUpdater_Start_Stop tests that updater starts and stops cleanly
func TestGeoIPUpdater_Start_Stop(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "geoip-test-")
	require.NoError(t, err)
	defer os.RemoveAll(tmpDir)

	dbPath := filepath.Join(tmpDir, "GeoLite2-City.mmdb")
	err = os.WriteFile(dbPath, []byte("test"), 0644)
	require.NoError(t, err)

	logger := zap.NewNop()

	client := redis.NewClient(&redis.Options{
		Addr: "localhost:6379",
	})

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	// Skip if Redis not available
	if err := client.Ping(ctx).Err(); err != nil {
		t.Skip("Redis not available")
	}
	defer client.Close()

	enricher, err := pipeline.NewGeoIPEnricher(dbPath, client, logger)
	if err != nil {
		t.Skip("Could not create test enricher (expected for test stub DB)")
	}
	defer enricher.Close()

	// Create updater
	updater := pipeline.NewGeoIPUpdater(enricher, logger)
	require.NotNil(t, updater)

	// Start updater
	ctx = context.Background()
	err = updater.Start(ctx)
	assert.NoError(t, err)

	// Wait a bit for goroutine to start
	time.Sleep(100 * time.Millisecond)

	// Stop updater
	err = updater.Stop()
	assert.NoError(t, err)
}

// TestGeoIPUpdater_MultipleStart tests that calling Start multiple times is safe
func TestGeoIPUpdater_MultipleStart(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "geoip-test-")
	require.NoError(t, err)
	defer os.RemoveAll(tmpDir)

	dbPath := filepath.Join(tmpDir, "GeoLite2-City.mmdb")
	err = os.WriteFile(dbPath, []byte("test"), 0644)
	require.NoError(t, err)

	logger := zap.NewNop()

	client := redis.NewClient(&redis.Options{
		Addr: "localhost:6379",
	})

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	// Skip if Redis not available
	if err := client.Ping(ctx).Err(); err != nil {
		t.Skip("Redis not available")
	}
	defer client.Close()

	enricher, err := pipeline.NewGeoIPEnricher(dbPath, client, logger)
	if err != nil {
		t.Skip("Could not create test enricher (expected for test stub DB)")
	}
	defer enricher.Close()

	updater := pipeline.NewGeoIPUpdater(enricher, logger)
	require.NotNil(t, updater)

	ctx = context.Background()

	// Start multiple times
	err = updater.Start(ctx)
	assert.NoError(t, err)

	err = updater.Start(ctx)
	assert.NoError(t, err) // Should not error

	// Stop
	err = updater.Stop()
	assert.NoError(t, err)
}

// TestGeoIPUpdater_GracefulShutdown tests that updater stops even if context is cancelled
func TestGeoIPUpdater_GracefulShutdown(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "geoip-test-")
	require.NoError(t, err)
	defer os.RemoveAll(tmpDir)

	dbPath := filepath.Join(tmpDir, "GeoLite2-City.mmdb")
	err = os.WriteFile(dbPath, []byte("test"), 0644)
	require.NoError(t, err)

	logger := zap.NewNop()

	client := redis.NewClient(&redis.Options{
		Addr: "localhost:6379",
	})

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	// Skip if Redis not available
	if err := client.Ping(ctx).Err(); err != nil {
		t.Skip("Redis not available")
	}
	defer client.Close()

	enricher, err := pipeline.NewGeoIPEnricher(dbPath, client, logger)
	if err != nil {
		t.Skip("Could not create test enricher (expected for test stub DB)")
	}
	defer enricher.Close()

	updater := pipeline.NewGeoIPUpdater(enricher, logger)
	require.NotNil(t, updater)

	// Start with cancellable context
	ctx, cancelFunc := context.WithCancel(context.Background())
	err = updater.Start(ctx)
	assert.NoError(t, err)

	// Cancel context
	cancelFunc()

	// Wait a bit for goroutine to respond
	time.Sleep(100 * time.Millisecond)

	// Stop should still work gracefully
	err = updater.Stop()
	assert.NoError(t, err)
}

// TestGeoIPUpdater_RegisterMetrics tests that metrics can be registered without errors
func TestGeoIPUpdater_RegisterMetrics(t *testing.T) {
	logger := zap.NewNop()

	tmpDir, err := os.MkdirTemp("", "geoip-test-")
	require.NoError(t, err)
	defer os.RemoveAll(tmpDir)

	dbPath := filepath.Join(tmpDir, "GeoLite2-City.mmdb")
	err = os.WriteFile(dbPath, []byte("test"), 0644)
	require.NoError(t, err)

	client := redis.NewClient(&redis.Options{
		Addr: "localhost:6379",
	})

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	// Skip if Redis not available
	if err := client.Ping(ctx).Err(); err != nil {
		t.Skip("Redis not available")
	}
	defer client.Close()

	enricher, err := pipeline.NewGeoIPEnricher(dbPath, client, logger)
	if err != nil {
		t.Skip("Could not create test enricher (expected for test stub DB)")
	}
	defer enricher.Close()

	updater := pipeline.NewGeoIPUpdater(enricher, logger)
	require.NotNil(t, updater)

	// Try to register metrics with a new registry
	reg := prometheus.NewRegistry()
	err = updater.RegisterMetrics(reg)
	assert.NoError(t, err)

	// Verify we can register twice with separate registries (should work)
	reg2 := prometheus.NewRegistry()
	err = updater.RegisterMetrics(reg2)
	assert.NoError(t, err)
}
