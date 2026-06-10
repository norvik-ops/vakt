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

// handleISMSKPISnapshot iterates all organisations and computes + persists the
// daily ISMS KPI snapshot for each. Runs every day at 06:00 UTC (S61-7).
// Uses errgroup with a concurrency limit of 5 to avoid DB saturation.
func handleISMSKPISnapshot(pool *pgxpool.Pool) asynq.HandlerFunc {
	return func(ctx context.Context, _ *asynq.Task) error {
		rows, err := pool.Query(ctx, `SELECT id::text FROM organizations`)
		if err != nil {
			return fmt.Errorf("isms_kpi_snapshot: list orgs: %w", err)
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
			return fmt.Errorf("isms_kpi_snapshot: scan orgs: %w", err)
		}

		svc := vaktcomply.NewService(pool)

		g, gCtx := errgroup.WithContext(ctx)
		sem := make(chan struct{}, 5)
		for _, orgID := range orgIDs {
			orgID := orgID
			sem <- struct{}{}
			g.Go(func() error {
				defer func() { <-sem }()
				if err := svc.CalculateAndStoreKPIs(gCtx, orgID); err != nil {
					log.Error().Err(err).Str("org_id", orgID).Msg("isms_kpi_snapshot: failed for org")
				}
				return nil
			})
		}
		if err := g.Wait(); err != nil {
			return err
		}

		log.Info().Int("orgs", len(orgIDs)).Msg("isms_kpi_snapshot: completed")
		return nil
	}
}
