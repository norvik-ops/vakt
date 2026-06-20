//go:build integration

package integration_test

import (
	"context"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/require"
	"github.com/testcontainers/testcontainers-go/modules/postgres"

	"github.com/matharnica/vakt/internal/shared/audit"
	shareddb "github.com/matharnica/vakt/internal/shared/db"
)

// TestAuditPartitionMaintenance verifies that audit.MaintainPartitions pre-creates
// upcoming year partitions and drops partitions past the retention window, while
// leaving the DEFAULT partition and in-window partitions intact (S98-10).
func TestAuditPartitionMaintenance(t *testing.T) {
	if testing.Short() {
		t.Skip("integration: -short mode")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Minute)
	defer cancel()

	pgC, err := postgres.Run(ctx,
		"postgres:16-alpine",
		postgres.WithDatabase("vakt_test"),
		postgres.WithUsername("vakt"),
		postgres.WithPassword("vakt"),
		postgres.WithSQLDriver("pgx"),
		postgres.BasicWaitStrategies(),
	)
	if err != nil {
		if strings.Contains(err.Error(), "permission denied") ||
			strings.Contains(err.Error(), "Cannot connect to the Docker daemon") {
			t.Skipf("integration: Docker unavailable (%v)", err)
		}
		t.Fatalf("postgres container: %v", err)
	}
	defer func() { _ = pgC.Terminate(ctx) }()

	dsn, err := pgC.ConnectionString(ctx, "sslmode=disable")
	require.NoError(t, err)
	require.NoError(t, shareddb.RunMigrations(dsn, migrationsDir(t)))

	pool, err := pgxpool.New(ctx, dsn)
	require.NoError(t, err)
	defer pool.Close()

	thisYear := time.Now().UTC().Year()

	// Seed an old partition well outside any reasonable retention window.
	oldYear := thisYear - 10
	_, err = pool.Exec(ctx, fmt.Sprintf(
		`CREATE TABLE audit_log_%d PARTITION OF audit_log FOR VALUES FROM ('%d-01-01') TO ('%d-01-01')`,
		oldYear, oldYear, oldYear+1))
	require.NoError(t, err)

	// Retention = 6 years. cutoff = thisYear-6; oldYear (=thisYear-10) must be dropped.
	require.NoError(t, audit.MaintainPartitions(ctx, pool, 6))

	// Upcoming partitions must now exist.
	for _, y := range []int{thisYear, thisYear + 1, thisYear + 2} {
		require.True(t, partitionExists(ctx, t, pool, y), "partition for %d must exist", y)
	}
	// Old partition must be gone.
	require.False(t, partitionExists(ctx, t, pool, oldYear), "partition for %d must be dropped", oldYear)
	// DEFAULT partition must be untouched.
	var hasDefault bool
	require.NoError(t, pool.QueryRow(ctx,
		`SELECT EXISTS (SELECT 1 FROM pg_class WHERE relname = 'audit_log_default')`).Scan(&hasDefault))
	require.True(t, hasDefault, "DEFAULT partition must survive")

	// Idempotent: a second run must not error.
	require.NoError(t, audit.MaintainPartitions(ctx, pool, 6))

	// retentionYears=0 disables dropping (pre-creation still runs, nothing else removed).
	require.NoError(t, audit.MaintainPartitions(ctx, pool, 0))
	require.True(t, partitionExists(ctx, t, pool, thisYear+2))
}

func partitionExists(ctx context.Context, t *testing.T, pool *pgxpool.Pool, year int) bool {
	t.Helper()
	var exists bool
	require.NoError(t, pool.QueryRow(ctx,
		`SELECT EXISTS (SELECT 1 FROM pg_class WHERE relname = $1)`,
		fmt.Sprintf("audit_log_%d", year)).Scan(&exists))
	return exists
}
