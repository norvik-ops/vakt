// Copyright (c) 2026 NorvikOps. All rights reserved.
// SPDX-License-Identifier: Elastic-2.0
// See LICENSE file in the project root for terms.

package main

import (
	"context"
	"fmt"

	"github.com/hibiken/asynq"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/rs/zerolog/log"

	"github.com/matharnica/vakt/internal/config"
	ghintegration "github.com/matharnica/vakt/internal/shared/platform/integrations/github"
)

const taskGitHubCISync = "github:ci_evidence:sync"

// handleGitHubCISync collects GitHub Actions CI run evidence for all organisations.
// For each org, it queries all GitHub integrations and fetches the 10 most recent
// completed runs, inserting a ck_evidence row for each successful run.
func handleGitHubCISync(cfg *config.Config, pool *pgxpool.Pool) asynq.HandlerFunc {
	return func(ctx context.Context, _ *asynq.Task) error {
		orgIDs, err := nonDemoOrgIDs(ctx, pool)
		if err != nil {
			return fmt.Errorf("github_ci_sync: %w", err)
		}

		for _, orgID := range orgIDs {
			if err := ghintegration.CollectCIEvidence(ctx, pool, orgID); err != nil {
				log.Error().Err(err).Str("org_id", orgID).Msg("github_ci_sync: org failed")
			}
		}
		log.Info().Int("orgs", len(orgIDs)).Msg("github_ci_sync: completed")
		return nil
	}
}

// handleCloudSync runs evidence collection for all enabled AWS + Azure cloud integrations.
func handleCloudSync(cfg *config.Config, pool *pgxpool.Pool) asynq.HandlerFunc {
	return func(ctx context.Context, _ *asynq.Task) error {
		if cfg == nil || cfg.SecretKey == "" {
			log.Warn().Msg("cloud_sync: master key not configured, skipping")
			return nil
		}
		svc := newCloudService(cfg, pool)
		if err := svc.SyncAllEnabled(ctx); err != nil {
			log.Error().Err(err).Msg("cloud_sync: failed")
			return err
		}
		log.Info().Msg("cloud_sync: completed")
		return nil
	}
}
