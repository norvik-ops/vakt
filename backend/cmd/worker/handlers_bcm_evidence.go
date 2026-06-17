// Copyright (c) 2026 NorvikOps. All rights reserved.
// SPDX-License-Identifier: Elastic-2.0

// S86-4: Daily BCM evidence sync for DER.4 controls.

package main

import (
	"context"

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
		orgIDs, err := nonDemoOrgIDs(ctx, pool)
		if err != nil {
			return err
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
