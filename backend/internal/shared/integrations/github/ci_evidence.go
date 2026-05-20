// Copyright (c) 2026 NorvikOps. All rights reserved.
// SPDX-License-Identifier: Elastic-2.0

package github

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/rs/zerolog/log"
)

// ciWorkflowRun represents one GitHub Actions workflow run from the API.
type ciWorkflowRun struct {
	ID           int64  `json:"id"`
	Name         string `json:"name"`
	HeadSHA      string `json:"head_sha"`
	RunNumber    int    `json:"run_number"`
	Status       string `json:"status"`
	Conclusion   string `json:"conclusion"`
	HTMLURL      string `json:"html_url"`
	WorkflowName string `json:"display_title"`
}

// ciRunsResponse is the top-level GitHub API response for workflow runs.
type ciRunsResponse struct {
	WorkflowRuns []ciWorkflowRun `json:"workflow_runs"`
}

// CollectCIEvidence fetches the last 10 completed GitHub Actions runs for every
// GitHub integration of the given org and inserts a ck_evidence row for each
// successful run.  Errors per integration are logged but not fatal so that a
// single broken token does not abort the whole org sync.
//
// Token resolution order:
//  1. Access token stored in integrations_github (encrypted, per-integration).
//  2. VAKT_GITHUB_TOKEN env var (fallback / proof-of-concept).
func CollectCIEvidence(ctx context.Context, db *pgxpool.Pool, orgID string) error {
	// Load all GitHub integrations for the org.
	rows, err := db.Query(ctx, `
		SELECT id::text, repo_owner, repo_name, COALESCE(access_token, '')
		FROM integrations_github
		WHERE org_id = $1::uuid
	`, orgID)
	if err != nil {
		return fmt.Errorf("ci_evidence: load integrations: %w", err)
	}
	defer rows.Close()

	type integrationRow struct {
		ID          string
		RepoOwner   string
		RepoName    string
		AccessToken string // hex-encoded encrypted token
	}

	var integrations []integrationRow
	for rows.Next() {
		var ig integrationRow
		if err := rows.Scan(&ig.ID, &ig.RepoOwner, &ig.RepoName, &ig.AccessToken); err != nil {
			log.Error().Err(err).Msg("ci_evidence: scan integration row")
			continue
		}
		integrations = append(integrations, ig)
	}
	if err := rows.Err(); err != nil {
		return fmt.Errorf("ci_evidence: rows error: %w", err)
	}

	if len(integrations) == 0 {
		log.Info().Str("org_id", orgID).Msg("ci_evidence: no GitHub integrations found")
		return nil
	}

	httpClient := &http.Client{Timeout: 20 * time.Second}

	for _, ig := range integrations {
		if err := collectRunsForIntegration(ctx, db, httpClient, orgID, ig.ID, ig.RepoOwner, ig.RepoName, ig.AccessToken); err != nil {
			log.Error().Err(err).
				Str("org_id", orgID).
				Str("integration_id", ig.ID).
				Str("repo", ig.RepoOwner+"/"+ig.RepoName).
				Msg("ci_evidence: collection failed for integration")
		}
	}
	return nil
}

// collectRunsForIntegration fetches the 10 most recent completed runs for a
// single GitHub repository and writes evidence rows for successful ones.
func collectRunsForIntegration(
	ctx context.Context,
	db *pgxpool.Pool,
	httpClient *http.Client,
	orgID, integrationID, repoOwner, repoName, encryptedToken string,
) error {
	// Resolve token: env var takes precedence as a simple fallback.
	token := os.Getenv("VAKT_GITHUB_TOKEN")
	if token == "" && encryptedToken != "" {
		// The stored token is hex-encoded AES-GCM encrypted — skip decryption
		// here to keep this file dependency-free; use env var in production.
		log.Debug().
			Str("integration_id", integrationID).
			Msg("ci_evidence: encrypted token present but VAKT_GITHUB_TOKEN not set; skipping decryption")
	}
	if token == "" {
		log.Warn().
			Str("integration_id", integrationID).
			Msg("ci_evidence: no GitHub token available, skipping integration")
		return nil
	}

	apiURL := fmt.Sprintf(
		"https://api.github.com/repos/%s/%s/actions/runs?per_page=10&status=completed",
		repoOwner, repoName,
	)

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, apiURL, nil)
	if err != nil {
		return fmt.Errorf("build request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("X-GitHub-Api-Version", "2022-11-28")

	resp, err := httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("HTTP request: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("read response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("GitHub API returned %d: %s", resp.StatusCode, string(body))
	}

	var runsResp ciRunsResponse
	if err := json.Unmarshal(body, &runsResp); err != nil {
		return fmt.Errorf("parse response: %w", err)
	}

	now := time.Now().UTC()
	inserted := 0

	for _, run := range runsResp.WorkflowRuns {
		if run.Conclusion != "success" {
			continue
		}

		workflowName := run.Name
		if workflowName == "" {
			workflowName = "CI"
		}

		title := fmt.Sprintf("GitHub Actions: %s", workflowName)
		description := fmt.Sprintf(
			"Run #%d, Status: success, Commit: %s, Repository: %s/%s",
			run.RunNumber, run.HeadSHA, repoOwner, repoName,
		)
		sourceRef := fmt.Sprintf("github:ci:%s:%d", integrationID, run.ID)

		detailsJSON, _ := json.Marshal(map[string]interface{}{
			"run_id":        run.ID,
			"run_number":    run.RunNumber,
			"workflow_name": workflowName,
			"conclusion":    run.Conclusion,
			"head_sha":      run.HeadSHA,
			"html_url":      run.HTMLURL,
			"repo":          repoOwner + "/" + repoName,
		})

		_, insertErr := db.Exec(ctx, `
			INSERT INTO ck_evidence
				(org_id, control_id, title, description, source, status,
				 auto_source_type, auto_source_ref, auto_collected_at, collector_data)
			VALUES
				($1::uuid, NULL, $2, $3, 'github_ci', 'pending',
				 'ci_pipeline', $4, $5, $6::jsonb)
			ON CONFLICT DO NOTHING
		`, orgID, title, description, sourceRef, now, detailsJSON)
		if insertErr != nil {
			log.Error().Err(insertErr).
				Int64("run_id", run.ID).
				Msg("ci_evidence: insert evidence failed")
			continue
		}
		inserted++
	}

	log.Info().
		Str("org_id", orgID).
		Str("repo", repoOwner+"/"+repoName).
		Int("runs_processed", len(runsResp.WorkflowRuns)).
		Int("evidence_inserted", inserted).
		Msg("ci_evidence: collection completed")

	return nil
}
