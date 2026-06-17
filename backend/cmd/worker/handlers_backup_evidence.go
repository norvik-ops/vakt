// Copyright (c) 2026 NorvikOps. All rights reserved.
// SPDX-License-Identifier: Elastic-2.0

// S88-2: Daily backup-freshness check — flags overdue backups/restore tests and
// syncs ISO A.8.13 / DER.4 evidence for all orgs.

package main

import (
	"context"

	"github.com/hibiken/asynq"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/rs/zerolog/log"

	"github.com/matharnica/vakt/internal/modules/vaktcomply"
)

func handleBackupFreshnessCheck(pool *pgxpool.Pool) asynq.HandlerFunc {
	return func(ctx context.Context, _ *asynq.Task) error {
		orgIDs, err := nonDemoOrgIDs(ctx, pool)
		if err != nil {
			return err
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
