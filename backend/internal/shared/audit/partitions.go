// Copyright (c) 2026 NorvikOps. All rights reserved.
// SPDX-License-Identifier: Elastic-2.0

package audit

import (
	"context"
	"fmt"
	"os"
	"strconv"
	"time"

	"github.com/hibiken/asynq"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/rs/zerolog/log"
)

// TaskPartitionMaint is the Asynq task type for the audit_log partition
// maintenance job (S98-10).
const TaskPartitionMaint = "audit:partition_maint"

// NewPartitionMaintTask creates the audit_log partition-maintenance job with a
// 23h uniqueness lock so multiple worker instances don't run it concurrently.
func NewPartitionMaintTask() *asynq.Task {
	return asynq.NewTask(TaskPartitionMaint, nil, asynq.Unique(23*time.Hour))
}

// RetentionYearsFromEnv reads VAKT_AUDIT_RETENTION_YEARS (default 6). A value of
// 0 disables dropping old partitions (pre-creation still runs).
func RetentionYearsFromEnv() int {
	if v := os.Getenv("VAKT_AUDIT_RETENTION_YEARS"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n >= 0 {
			return n
		}
	}
	return defaultRetentionYears
}

// defaultRetentionYears is the fallback when VAKT_AUDIT_RETENTION_YEARS is unset.
// 6 years comfortably covers typical legal retention (GoBD 6y, many ISMS 3–6y).
const defaultRetentionYears = 6

// MaintainPartitions keeps the yearly RANGE partitions of audit_log healthy:
//
//   - ensures a dedicated partition exists for the current year and the next two
//     (so rows never silently pile into the catch-all DEFAULT partition), and
//   - drops partitions whose entire year is older than retentionYears (DETACH then
//     DROP — cheap, unlike a row-level DELETE).
//
// It is idempotent and safe to run repeatedly. retentionYears <= 0 disables the
// drop step (partitions are still pre-created). The DEFAULT partition and the
// current/recent year partitions are never dropped.
func MaintainPartitions(ctx context.Context, db *pgxpool.Pool, retentionYears int) error {
	thisYear := time.Now().UTC().Year()

	// 1. Pre-create current + next two years.
	for y := thisYear; y <= thisYear+2; y++ {
		if err := ensureYearPartition(ctx, db, y); err != nil {
			return fmt.Errorf("ensure partition %d: %w", y, err)
		}
	}

	// 2. Drop partitions older than the retention window.
	if retentionYears > 0 {
		cutoff := thisYear - retentionYears
		if err := dropPartitionsBefore(ctx, db, cutoff); err != nil {
			return fmt.Errorf("drop old partitions before %d: %w", cutoff, err)
		}
	}
	return nil
}

// ensureYearPartition creates audit_log_<year> if it does not already exist.
// Postgres has no "CREATE TABLE ... PARTITION OF ... IF NOT EXISTS", so we check
// pg_class first. The year bounds are derived from a validated int, never user input.
func ensureYearPartition(ctx context.Context, db *pgxpool.Pool, year int) error {
	name := fmt.Sprintf("audit_log_%d", year)
	var exists bool
	if err := db.QueryRow(ctx, `SELECT EXISTS (SELECT 1 FROM pg_class WHERE relname = $1)`, name).Scan(&exists); err != nil {
		return err
	}
	if exists {
		return nil
	}
	// year is an int we control (thisYear..thisYear+2) — safe to interpolate.
	stmt := fmt.Sprintf(
		`CREATE TABLE IF NOT EXISTS %s PARTITION OF audit_log FOR VALUES FROM ('%d-01-01') TO ('%d-01-01')`,
		name, year, year+1,
	)
	if _, err := db.Exec(ctx, stmt); err != nil {
		return err
	}
	log.Info().Str("partition", name).Msg("audit_log: created year partition")
	return nil
}

// dropPartitionsBefore detaches and drops audit_log_<year> for every year strictly
// less than cutoff. The DEFAULT partition (audit_log_default) is never matched.
func dropPartitionsBefore(ctx context.Context, db *pgxpool.Pool, cutoff int) error {
	rows, err := db.Query(ctx, `
		SELECT c.relname
		FROM pg_inherits i
		JOIN pg_class c  ON c.oid = i.inhrelid
		JOIN pg_class p  ON p.oid = i.inhparent
		WHERE p.relname = 'audit_log'
		  AND c.relname ~ '^audit_log_[0-9]{4}$'`)
	if err != nil {
		return err
	}
	var names []string
	for rows.Next() {
		var n string
		if err := rows.Scan(&n); err != nil {
			rows.Close()
			return err
		}
		names = append(names, n)
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return err
	}

	for _, name := range names {
		var year int
		if _, scanErr := fmt.Sscanf(name, "audit_log_%d", &year); scanErr != nil {
			continue
		}
		if year >= cutoff {
			continue
		}
		// DETACH then DROP — interpolated name comes from pg_class, not user input.
		if _, err := db.Exec(ctx, fmt.Sprintf(`ALTER TABLE audit_log DETACH PARTITION %s`, name)); err != nil {
			return fmt.Errorf("detach %s: %w", name, err)
		}
		if _, err := db.Exec(ctx, fmt.Sprintf(`DROP TABLE %s`, name)); err != nil {
			return fmt.Errorf("drop %s: %w", name, err)
		}
		log.Warn().Str("partition", name).Int("year", year).Msg("audit_log: dropped partition past retention")
	}
	return nil
}
