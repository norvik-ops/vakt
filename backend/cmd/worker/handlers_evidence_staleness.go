// Copyright (c) 2026 NorvikOps. All rights reserved.
// SPDX-License-Identifier: Elastic-2.0

package main

import (
	"context"
	"fmt"

	"github.com/hibiken/asynq"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/rs/zerolog/log"

	"github.com/matharnica/vakt/internal/modules/vaktcomply"
)

// handleEvidenceStalenessCheck runs the daily evidence staleness sweep for
// all organisations. Updates evidence_status on ck_controls and recomputes
// the compliance score used by the KPI dashboard (S67-4).
func handleEvidenceStalenessCheck(pool *pgxpool.Pool) asynq.HandlerFunc {
	return func(ctx context.Context, _ *asynq.Task) error {
		rows, err := pool.Query(ctx, `SELECT id::text FROM organizations`)
		if err != nil {
			return fmt.Errorf("evidence_staleness: list orgs: %w", err)
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
			return fmt.Errorf("evidence_staleness: scan orgs: %w", err)
		}

		svc := vaktcomply.NewService(pool)
		var failed int
		for _, orgID := range orgIDs {
			if err := svc.RunStalenessCheck(ctx, orgID); err != nil {
				log.Error().Err(err).Str("org_id", orgID).Msg("evidence_staleness: check failed")
				failed++
			}
		}

		log.Info().Int("orgs", len(orgIDs)).Int("failed", failed).Msg("evidence_staleness: sweep completed")
		return nil
	}
}
