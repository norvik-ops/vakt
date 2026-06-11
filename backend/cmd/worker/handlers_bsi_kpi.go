// Copyright (c) 2026 NorvikOps. All rights reserved.
// SPDX-License-Identifier: Elastic-2.0

package main

import (
	"context"
	"fmt"

	"github.com/hibiken/asynq"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/rs/zerolog/log"
	"golang.org/x/sync/errgroup"

	"github.com/matharnica/vakt/internal/modules/vaktcomply"
)

// handleBSIKPISnapshot updates bsi_check_pct in ck_isms_kpi_snapshots for all orgs.
// Runs daily at 06:15 UTC (S74-2).
func handleBSIKPISnapshot(pool *pgxpool.Pool) asynq.HandlerFunc {
	return func(ctx context.Context, _ *asynq.Task) error {
		rows, err := pool.Query(ctx, `SELECT id::text FROM organizations`)
		if err != nil {
			return fmt.Errorf("bsi_kpi_snapshot: list orgs: %w", err)
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
			return fmt.Errorf("bsi_kpi_snapshot: scan orgs: %w", err)
		}

		svc := vaktcomply.NewService(pool)

		g, gCtx := errgroup.WithContext(ctx)
		sem := make(chan struct{}, 5)
		for _, orgID := range orgIDs {
			orgID := orgID
			sem <- struct{}{}
			g.Go(func() error {
				defer func() { <-sem }()
				if err := svc.CalculateAndStoreBSIKPISnapshot(gCtx, orgID); err != nil {
					log.Warn().Err(err).Str("org_id", orgID).Msg("bsi_kpi_snapshot: failed for org")
				}
				return nil
			})
		}
		if err := g.Wait(); err != nil {
			return err
		}

		log.Info().Int("orgs", len(orgIDs)).Msg("bsi_kpi_snapshot: completed")
		return nil
	}
}
