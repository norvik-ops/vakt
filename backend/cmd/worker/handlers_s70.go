// Copyright (c) 2026 NorvikOps. All rights reserved.
// SPDX-License-Identifier: Elastic-2.0
// See LICENSE file in the project root for terms.

// Sprint 70: Contractor expiry check + Vault quarterly access review.

package main

import (
	"context"
	"time"

	"github.com/hibiken/asynq"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/rs/zerolog/log"

	"github.com/matharnica/vakt/internal/modules/vakthr"
	"github.com/matharnica/vakt/internal/modules/vaktvault"
)

// handleContractorExpiryCheck handles hr:contractor_expiry_check jobs (S70-4).
// Marks contractors expiring within 14 days as expiring_soon and those past
// contract_end as offboarding.
func handleContractorExpiryCheck(pool *pgxpool.Pool) asynq.HandlerFunc {
	return func(ctx context.Context, _ *asynq.Task) error {
		svc := vakthr.NewServiceFromPool(pool)
		if err := svc.CheckContractorExpiry(ctx); err != nil {
			log.Error().Err(err).Msg("contractor expiry check failed")
			return err
		}
		return nil
	}
}

// handleQuarterlyAccessReview handles vault:quarterly_access_review jobs (S70-5).
// Creates a new access review for every org that does not yet have one for the
// current quarter.
func handleQuarterlyAccessReview(pool *pgxpool.Pool) asynq.HandlerFunc {
	return func(ctx context.Context, _ *asynq.Task) error {
		rows, err := pool.Query(ctx, `
			SELECT DISTINCT org_id::text
			FROM so_projects`)
		if err != nil {
			log.Error().Err(err).Msg("quarterly access review: list orgs failed")
			return err
		}
		defer rows.Close()

		label := vaktvault.CurrentQuarterLabel(time.Now().UTC())
		// masterKey and queue are not needed for access review operations.
		svc := vaktvault.NewService(pool, nil, nil)

		for rows.Next() {
			var orgID string
			if err := rows.Scan(&orgID); err != nil {
				continue
			}
			// Skip if a review for this quarter already exists.
			var cnt int
			_ = pool.QueryRow(ctx, `
				SELECT COUNT(*) FROM so_access_reviews
				WHERE org_id = $1 AND period_label = $2`, orgID, label,
			).Scan(&cnt)
			if cnt > 0 {
				continue
			}
			if _, err := svc.CreateAccessReview(ctx, orgID); err != nil {
				log.Error().Err(err).Str("org_id", orgID).Msg("quarterly access review: create failed")
			} else {
				log.Info().Str("org_id", orgID).Str("period", label).Msg("quarterly access review created")
			}
		}
		return nil
	}
}
