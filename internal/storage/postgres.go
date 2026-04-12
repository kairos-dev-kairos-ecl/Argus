package storage

import (
	"context"
	"embed"
	"fmt"

	"github.com/golang-migrate/migrate/v4"
	_ "github.com/golang-migrate/migrate/v4/database/postgres"
	"github.com/golang-migrate/migrate/v4/source/iofs"
	"github.com/jackc/pgx/v5/pgxpool"
	"go.uber.org/zap"
)

// Postgres wraps the pgx/v5 connection pool and migration runner.
// It manages PostgreSQL connections for configuration storage and provides
// transaction support for the control plane.
type Postgres struct {
	Pool *pgxpool.Pool
}

//go:embed migrations/*.sql
var migrations embed.FS

// MigrateDB executes all pending migrations against a PostgreSQL connection pool.
// It is safe to call multiple times; migrations are idempotent (CREATE TABLE IF NOT EXISTS).
// Pass log=nil to disable logging.
func MigrateDB(ctx context.Context, pool *pgxpool.Pool, dsn string, log *zap.Logger) error {
	if log == nil {
		log = zap.NewNop()
	}

	// Run migrations using the iofs source driver
	d, err := iofs.New(migrations, "migrations")
	if err != nil {
		return fmt.Errorf("failed to create migration source: %w", err)
	}

	m, err := migrate.NewWithSourceInstance("iofs", d, dsn)
	if err != nil {
		return fmt.Errorf("failed to create migrator: %w", err)
	}

	// Run up migrations
	if err := m.Up(); err != nil && err != migrate.ErrNoChange {
		return fmt.Errorf("failed to run migrations: %w", err)
	}

	log.Info("database migrations completed successfully")

	// Clean up migration instance resources
	srcErr, dbErr := m.Close()
	if srcErr != nil {
		return fmt.Errorf("migration source error: %w", srcErr)
	}
	if dbErr != nil {
		return fmt.Errorf("migration db error: %w", dbErr)
	}

	return nil
}

// NewPostgres creates a pgx connection pool and runs all pending up-migrations.
// It is safe to call multiple times; migrations are idempotent (CREATE TABLE IF NOT EXISTS).
//
// dsn format: "postgres://[user[:password]@][netloc][:port][/dbname][?param1=value1&...]"
// Example: "postgres://postgres:password@localhost:5432/argus"
func NewPostgres(ctx context.Context, dsn string) (*Postgres, error) {
	// Create the connection pool
	pool, err := pgxpool.New(ctx, dsn)
	if err != nil {
		return nil, fmt.Errorf("failed to create postgres pool: %w", err)
	}

	// Verify connectivity with a ping
	if err := pool.Ping(ctx); err != nil {
		pool.Close()
		return nil, fmt.Errorf("failed to ping postgres: %w", err)
	}

	// Run migrations
	if err := MigrateDB(ctx, pool, dsn, zap.NewNop()); err != nil {
		pool.Close()
		return nil, err
	}

	return &Postgres{
		Pool: pool,
	}, nil
}

// Close closes the PostgreSQL connection pool.
// All pending transactions will be rolled back.
func (p *Postgres) Close() {
	p.Pool.Close()
}

// ExecContext is a helper for executing queries without results (DDL, DML).
func (p *Postgres) ExecContext(ctx context.Context, sql string, args ...interface{}) error {
	return p.Pool.QueryRow(ctx, sql, args...).Scan()
}

// QueryRowContext is a helper for executing queries that return a single row.
func (p *Postgres) QueryRowContext(ctx context.Context, sql string, args ...interface{}) interface{} {
	return p.Pool.QueryRow(ctx, sql, args...)
}

// QueryContext is a helper for executing queries that may return multiple rows.
func (p *Postgres) QueryContext(ctx context.Context, sql string, args ...interface{}) (interface{}, error) {
	return p.Pool.Query(ctx, sql, args...)
}

// Tx returns a new transaction for batch operations.
func (p *Postgres) Tx(ctx context.Context) (interface{}, error) {
	return p.Pool.Begin(ctx)
}
