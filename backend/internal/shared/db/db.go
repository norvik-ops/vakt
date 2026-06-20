package db

import (
	"context"
	"fmt"
	"math"
	"os"
	"strconv"
	"time"

	pgx "github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// Connect creates and validates a pgx connection pool.
// Pool size is controlled by VAKT_DB_MAX_CONNS (default 25).
func Connect(ctx context.Context, connStr string) (*pgxpool.Pool, error) {
	cfg, err := pgxpool.ParseConfig(connStr)
	if err != nil {
		return nil, fmt.Errorf("parse db config: %w", err)
	}

	// ponytail: 15 leaves headroom for pgBouncer's own pool (default pool_size=20)
	// without saturating the Postgres max_connections (default 100). Raise via
	// VAKT_DB_MAX_CONNS only when running without pgBouncer. (PERF-M01)
	maxConns := int32(15)
	if v := os.Getenv("VAKT_DB_MAX_CONNS"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 && n <= math.MaxInt32 { // nosemgrep: string-to-int-signedness-cast -- bounds checked in condition
			maxConns = int32(n)
		}
	}
	cfg.MaxConns = maxConns
	cfg.MinConns = 2
	cfg.MaxConnLifetime = 30 * time.Minute
	cfg.MaxConnIdleTime = 5 * time.Minute
	cfg.HealthCheckPeriod = 1 * time.Minute
	// CacheDescribe avoids prepared statements — required for pgBouncer transaction mode.
	cfg.ConnConfig.DefaultQueryExecMode = pgx.QueryExecModeCacheDescribe

	pool, err := pgxpool.NewWithConfig(ctx, cfg)
	if err != nil {
		return nil, fmt.Errorf("create pool: %w", err)
	}

	pingCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	if err := pool.Ping(pingCtx); err != nil {
		pool.Close()
		return nil, fmt.Errorf("ping db: %w", err)
	}

	return pool, nil
}
