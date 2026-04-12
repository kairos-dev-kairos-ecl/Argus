package main

import (
	"strings"

	"go.uber.org/zap"
)

// newLogger creates a zap logger configured for dev or production mode.
func newLogger(isDev bool) (*zap.Logger, error) {
	if isDev {
		return zap.NewDevelopment()
	}
	return zap.NewProduction()
}

// maskDSN masks sensitive parts of a DSN for safe logging.
// Replaces password in DSN patterns like "user:password@host" with "user:***@host".
func maskDSN(dsn string) string {
	// PostgreSQL: postgresql://user:password@host:port/db
	if strings.Contains(dsn, "://") {
		parts := strings.Split(dsn, "://")
		if len(parts) == 2 {
			scheme := parts[0]
			rest := parts[1]

			// Find @ to split auth from host
			if idx := strings.LastIndex(rest, "@"); idx > 0 {
				auth := rest[:idx]
				host := rest[idx:]

				// Mask password in auth (user:password format)
				if idx := strings.Index(auth, ":"); idx > 0 {
					user := auth[:idx]
					auth = user + ":***"
				}

				return scheme + "://" + auth + host
			}
		}
	}

	// ClickHouse: clickhouse://host:port (no auth typical, but be safe)
	if strings.Contains(dsn, ":") && !strings.Contains(dsn, "://") {
		// Simple host:port, no masking needed
		return dsn
	}

	// Fallback: return as-is
	return dsn
}
