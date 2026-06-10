// Copyright (c) 2026 NorvikOps. All rights reserved.
// SPDX-License-Identifier: Elastic-2.0
// See LICENSE file in the project root for terms.

package main

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/hibiken/asynq"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/rs/zerolog/log"

	"github.com/matharnica/vakt/internal/config"
	"github.com/matharnica/vakt/internal/modules/vaktcomply"
	"github.com/matharnica/vakt/internal/modules/vaktvault"
	cloudintegration "github.com/matharnica/vakt/internal/shared/platform/integrations/cloud"
	ghintegration "github.com/matharnica/vakt/internal/shared/platform/integrations/github"
)

const taskGitHubCISync = "github:ci_evidence:sync"

// handleGitScan handles vaktvault:git_scan jobs.
// Credentials stored in the payload are AES-256-GCM-encrypted; they are
// decrypted here using the master key from config before the scan runs.
func handleGitScan(cfg *config.Config, pool *pgxpool.Pool) asynq.HandlerFunc {
	return func(ctx context.Context, t *asynq.Task) error {
		var payload struct {
			ScanID               string `json:"scan_id"`
			OrgID                string `json:"org_id"`
			RepoURL              string `json:"repo_url"`
			Branch               string `json:"branch"`
			EncryptedCredentials string `json:"encrypted_credentials,omitempty"`
		}
		if err := json.Unmarshal(t.Payload(), &payload); err != nil {
			return fmt.Errorf("parse git_scan payload: %w", err)
		}

		// Decrypt credentials if present.
		var creds *vaktvault.GitScanCredentials
		if payload.EncryptedCredentials != "" {
			if cfg == nil || cfg.SecretKey == "" {
				return fmt.Errorf("git_scan: master key not configured, cannot decrypt credentials")
			}
			plainJSON, decErr := vaktvault.DecryptPayloadField(payload.EncryptedCredentials, workerKey(cfg, "vakt-vault-v1"))
			if decErr != nil {
				return fmt.Errorf("git_scan: decrypt credentials: %w", decErr)
			}
			var c vaktvault.GitScanCredentials
			if jsonErr := json.Unmarshal([]byte(plainJSON), &c); jsonErr != nil {
				return fmt.Errorf("git_scan: unmarshal credentials: %w", jsonErr)
			}
			creds = &c
		}

		repo := vaktvault.NewRepository(pool)

		if err := repo.UpdateGitScanStatus(ctx, payload.ScanID, payload.OrgID, "running", 0, 0, 0, "", nil); err != nil {
			return fmt.Errorf("mark scan running: %w", err)
		}

		results, scanErr := vaktvault.RunGitScan(ctx, vaktvault.TriggerGitScanInput{
			RepoURL:     payload.RepoURL,
			Branch:      payload.Branch,
			Credentials: creds,
		})

		scannedAt := time.Now().UTC()
		if scanErr != nil {
			errMsg := scanErr.Error()
			return repo.UpdateGitScanStatus(ctx, payload.ScanID, payload.OrgID, "failed", 0, 0, 0, errMsg, &scannedAt)
		}

		if len(results) > 0 {
			if err := repo.SaveScanResults(ctx, payload.OrgID, payload.ScanID, results); err != nil {
				return fmt.Errorf("save scan results: %w", err)
			}
		}

		openCount := len(results)
		if err := repo.UpdateGitScanStatus(ctx, payload.ScanID, payload.OrgID, "completed", openCount, openCount, 0, "", &scannedAt); err != nil {
			return fmt.Errorf("mark scan completed: %w", err)
		}

		log.Info().
			Str("scan_id", payload.ScanID).
			Str("repo_url", payload.RepoURL).
			Int("findings", openCount).
			Msg("git scan completed")

		if openCount > 0 {
			createGitLeakIncident(ctx, pool, payload.OrgID, payload.ScanID, payload.RepoURL, openCount, scannedAt)
		}

		return nil
	}
}

// createGitLeakIncident creates a vaktcomply incident for an open git credential leak.
// Best-effort: errors are logged but never returned — the scan result is already persisted.
func createGitLeakIncident(ctx context.Context, pool *pgxpool.Pool, orgID, scanID, repoURL string, findingCount int, discoveredAt time.Time) {
	complyRepo := vaktcomply.NewRepository(pool)
	incident, err := complyRepo.CreateIncident(ctx, orgID, vaktcomply.CreateIncidentInput{
		Title: fmt.Sprintf("[Git-Credential-Leak] %s", repoURL),
		Description: fmt.Sprintf(
			"Der Git-Scan des Repositories %s hat %d offene Credential-Leaks gefunden (Scan-ID: %s). "+
				"Bitte prüfen Sie die betroffenen Commits und rotieren Sie alle exponierten Credentials umgehend.",
			repoURL, findingCount, scanID,
		),
		Severity:        "high",
		DiscoveredAt:    discoveredAt,
		AffectedSystems: []string{repoURL},
	}, nil)
	if err != nil {
		log.Error().Err(err).Str("scan_id", scanID).Msg("vaktvault→vaktcomply: failed to create incident from git leak")
		return
	}
	log.Info().
		Str("scan_id", scanID).
		Str("incident_id", incident.ID).
		Str("org_id", orgID).
		Int("findings", findingCount).
		Msg("vaktvault→vaktcomply: incident created from git leak")
}

// handleGitHubCISync collects GitHub Actions CI run evidence for all organisations.
// For each org, it queries all GitHub integrations and fetches the 10 most recent
// completed runs, inserting a ck_evidence row for each successful run.
func handleGitHubCISync(cfg *config.Config, pool *pgxpool.Pool) asynq.HandlerFunc {
	return func(ctx context.Context, _ *asynq.Task) error {
		rows, err := pool.Query(ctx, `SELECT id::text FROM organizations`)
		if err != nil {
			return fmt.Errorf("github_ci_sync: list orgs: %w", err)
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
			return err
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
		svc := cloudintegration.NewService(pool, workerKey(cfg, "vakt-cloud-v1"), cloudintegration.NoopEvidenceWriter())
		if err := svc.SyncAllEnabled(ctx); err != nil {
			log.Error().Err(err).Msg("cloud_sync: failed")
			return err
		}
		log.Info().Msg("cloud_sync: completed")
		return nil
	}
}
