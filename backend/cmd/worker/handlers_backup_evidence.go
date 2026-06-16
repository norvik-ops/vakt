// Copyright (c) 2026 NorvikOps. All rights reserved.
// SPDX-License-Identifier: Elastic-2.0

// S88-2: Daily backup-freshness check — flags overdue backups/restore tests and
// syncs ISO A.8.13 / DER.4 evidence for all orgs.

package main

import (
	"context"
	"fmt"

	"github.com/hibiken/asynq"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/rs/zerolog/log"

	"github.com/matharnica/vakt/internal/modules/vaktcomply"
)

func handleBackupFreshnessCheck(pool *pgxpool.Pool) asynq.HandlerFunc {
	return func(ctx context.Context, _ *asynq.Task) error {
		rows, err := pool.Query(ctx, `SELECT id::text FROM organizations`)
		if err != nil {
			return fmt.Errorf("backup_freshness_check: list orgs: %w", err)
		}
		defer rows.Close()

		var orgIDs []string
		for rows.Next() {
			var id string
			if err := rows.Scan(&id); err != nil {
				continue
			}
			orgIDs = append(orgIDs, id)
		}
		if err := rows.Err(); err != nil {
			return fmt.Errorf("backup_freshness_check: scan orgs: %w", err)
		}

		svc := vaktcomply.NewService(pool)
		var failed, overdueTotal int
		for _, orgID := range orgIDs {
			overdue, err := svc.CheckBackupFreshness(ctx, orgID)
			if err != nil {
				log.Warn().Err(err).Str("org_id", orgID).Msg("backup_freshness_check: failed for org")
				failed++
				continue
			}
			overdueTotal += overdue
		}
		log.Info().Int("orgs", len(orgIDs)).Int("failed", failed).Int("overdue", overdueTotal).
			Msg("backup_freshness_check: completed")
		return nil
	}
}
