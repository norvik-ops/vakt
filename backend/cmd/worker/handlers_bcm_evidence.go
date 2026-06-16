// Copyright (c) 2026 NorvikOps. All rights reserved.
// SPDX-License-Identifier: Elastic-2.0

// S86-4: Daily BCM evidence sync for DER.4 controls.

package main

import (
	"context"
	"fmt"

	"github.com/hibiken/asynq"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/rs/zerolog/log"

	"github.com/matharnica/vakt/internal/modules/vaktcomply"
)

// handleBCMEvidenceSync creates/updates DER.4 evidence entries for all orgs
// based on current BIA, WAP, and emergency contact data.
// Runs daily at 07:00 UTC (S86-4).
func handleBCMEvidenceSync(pool *pgxpool.Pool) asynq.HandlerFunc {
	return func(ctx context.Context, _ *asynq.Task) error {
		rows, err := pool.Query(ctx, `SELECT id::text FROM organizations`)
		if err != nil {
			return fmt.Errorf("bcm_evidence_sync: list orgs: %w", err)
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
			return fmt.Errorf("bcm_evidence_sync: scan orgs: %w", err)
		}

		svc := vaktcomply.NewService(pool)
		var failed int
		for _, orgID := range orgIDs {
			if err := svc.SyncBCMEvidence(ctx, orgID); err != nil {
				log.Warn().Err(err).Str("org_id", orgID).Msg("bcm_evidence_sync: failed for org")
				failed++
			}
		}

		log.Info().Int("orgs", len(orgIDs)).Int("failed", failed).Msg("bcm_evidence_sync: completed")
		return nil
	}
}
