// Package database owns the PostgreSQL connection pool and schema migrations.
package database

import (
	"context"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/learna/learna-api/internal/config"
)

// DB wraps the pgx pool. Repositories take a *DB rather than the raw pool so
// that cross-cutting concerns (tracing, metrics, a query logger) can be added
// in one place later.
type DB struct {
	Pool *pgxpool.Pool
}

// Connect opens the pool and verifies it with a ping. The caller owns the
// returned DB and must Close it.
func Connect(ctx context.Context, cfg config.DBConfig) (*DB, error) {
	poolCfg, err := pgxpool.ParseConfig(cfg.DSN())
	if err != nil {
		return nil, fmt.Errorf("parse database dsn: %w", err)
	}

	poolCfg.MaxConns = cfg.MaxConns
	poolCfg.MinConns = cfg.MinConns
	poolCfg.MaxConnLifetime = cfg.MaxConnLifetime
	poolCfg.MaxConnIdleTime = 30 * time.Minute
	poolCfg.HealthCheckPeriod = time.Minute

	pool, err := pgxpool.NewWithConfig(ctx, poolCfg)
	if err != nil {
		return nil, fmt.Errorf("create connection pool: %w", err)
	}

	pingCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	if err := pool.Ping(pingCtx); err != nil {
		pool.Close()
		return nil, fmt.Errorf("ping database: %w", err)
	}

	return &DB{Pool: pool}, nil
}

// Ping reports whether the database is currently reachable. Used by /health.
func (d *DB) Ping(ctx context.Context) error {
	return d.Pool.Ping(ctx)
}

// Close releases every pooled connection.
func (d *DB) Close() {
	if d.Pool != nil {
		d.Pool.Close()
	}
}
