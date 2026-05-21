// Copyright (c) 2026 NorvikOps. All rights reserved.
// SPDX-License-Identifier: Elastic-2.0

package secpulse

// POST /api/v1/secpulse/ci-evidence
// Accepts CI/CD pipeline results as compliance evidence.
// Request body:
// {
//   "pipeline": "github-actions|gitlab-ci|jenkins|custom",
//   "repo":    "owner/repo",
//   "branch":  "main",
//   "ref":     "sha or tag",
//   "status":  "success|failure|cancelled",
//   "tests_total":   100,   // optional
//   "tests_passed":  98,    // optional
//   "tests_failed":  2,     // optional
//   "coverage_pct":  87.5,  // optional
//   "workflow_name": "CI",  // optional
//   "run_url":       "https://...",  // optional, link to the run
//   "ran_at":        "2026-05-18T12:00:00Z"  // optional RFC3339, defaults to now
// }

import (
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/labstack/echo/v4"
	"github.com/rs/zerolog/log"
)

// CIEvidenceInput is the request body for the CI/CD evidence webhook endpoint.
type CIEvidenceInput struct {
	Pipeline     string   `json:"pipeline"      validate:"required,oneof=github-actions gitlab-ci jenkins custom"`
	Repo         string   `json:"repo"          validate:"required"`
	Branch       string   `json:"branch"        validate:"required"`
	Ref          string   `json:"ref"`
	Status       string   `json:"status"        validate:"required,oneof=success failure cancelled"`
	TestsTotal   *int     `json:"tests_total"`
	TestsPassed  *int     `json:"tests_passed"`
	TestsFailed  *int     `json:"tests_failed"`
	CoveragePct  *float64 `json:"coverage_pct"`
	WorkflowName string   `json:"workflow_name"`
	RunURL       string   `json:"run_url"`
	RanAt        *string  `json:"ran_at"`
}

// ReceiveCIEvidence handles POST /api/v1/secpulse/ci-evidence.
// Accepts CI/CD pipeline results from any system (GitHub Actions, GitLab CI,
// Jenkins, or custom) and stores them as compliance evidence in ck_evidence.
func (h *Handler) ReceiveCIEvidence(c echo.Context) error {
	orgID, _ := c.Get("org_id").(string)
	if orgID == "" {
		return c.JSON(http.StatusUnauthorized, map[string]string{
			"error": "unauthorized",
			"code":  "UNAUTHORIZED",
		})
	}

	var input CIEvidenceInput
	if err := c.Bind(&input); err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{
			"error": "invalid request body",
			"code":  "VB_BAD_REQUEST",
		})
	}
	if err := h.validate.Struct(input); err != nil {
		return c.JSON(http.StatusUnprocessableEntity, map[string]string{
			"error": "Ungültige Eingabe",
			"code":  "VALIDATION_ERROR",
		})
	}

	// Parse ran_at; fall back to now if absent or unparseable.
	collectedAt := time.Now().UTC()
	if input.RanAt != nil && *input.RanAt != "" {
		if t, err := time.Parse(time.RFC3339, *input.RanAt); err == nil {
			collectedAt = t.UTC()
		} else {
			log.Warn().Str("ran_at", *input.RanAt).Msg("ci-evidence: could not parse ran_at, using now")
		}
	}

	// Build evidence title.
	title := "CI: " + input.Pipeline
	if input.WorkflowName != "" {
		title = "CI: " + input.WorkflowName + " (" + input.Pipeline + ")"
	}

	// Build evidence description.
	description := fmt.Sprintf(
		"Pipeline: %s | Repo: %s | Branch: %s | Status: %s",
		input.Pipeline, input.Repo, input.Branch, input.Status,
	)

	// Build collector_data JSON for full fidelity.
	collectorData := map[string]any{
		"pipeline":      input.Pipeline,
		"repo":          input.Repo,
		"branch":        input.Branch,
		"ref":           input.Ref,
		"status":        input.Status,
		"workflow_name": input.WorkflowName,
		"run_url":       input.RunURL,
		"ran_at":        collectedAt.Format(time.RFC3339),
	}
	if input.TestsTotal != nil {
		collectorData["tests_total"] = *input.TestsTotal
	}
	if input.TestsPassed != nil {
		collectorData["tests_passed"] = *input.TestsPassed
	}
	if input.TestsFailed != nil {
		collectorData["tests_failed"] = *input.TestsFailed
	}
	if input.CoveragePct != nil {
		collectorData["coverage_pct"] = *input.CoveragePct
	}

	collectorJSON, err := json.Marshal(collectorData)
	if err != nil {
		log.Error().Err(err).Str("org_id", orgID).Msg("ci-evidence: marshal collector data")
		return c.JSON(http.StatusInternalServerError, map[string]string{
			"error": "internal error",
			"code":  "VB_INTERNAL_ERROR",
		})
	}

	// Use a unique source ref to enable idempotent inserts from the same run.
	sourceRef := fmt.Sprintf("ci_webhook:%s:%s:%s", input.Pipeline, input.Repo, collectedAt.Format(time.RFC3339))

	var evidenceID string
	err = h.service.db.QueryRow(c.Request().Context(), `
		INSERT INTO ck_evidence
			(org_id, control_id, title, description, source, status,
			 auto_source_type, auto_source_ref, auto_collected_at, collector_data)
		VALUES
			($1::uuid, NULL, $2, $3, 'ci_webhook', 'pending',
			 'ci_webhook', $4, $5, $6::jsonb)
		RETURNING id::text
	`, orgID, title, description, sourceRef, collectedAt, collectorJSON).Scan(&evidenceID)
	if err != nil {
		log.Error().Err(err).
			Str("org_id", orgID).
			Str("pipeline", input.Pipeline).
			Str("repo", input.Repo).
			Msg("ci-evidence: insert failed")
		return c.JSON(http.StatusInternalServerError, map[string]string{
			"error": "failed to record CI evidence",
			"code":  "VB_CI_EVIDENCE_ERROR",
		})
	}

	log.Info().
		Str("org_id", orgID).
		Str("pipeline", input.Pipeline).
		Str("repo", input.Repo).
		Str("branch", input.Branch).
		Str("status", input.Status).
		Str("evidence_id", evidenceID).
		Msg("ci-evidence: evidence recorded")

	h.audit(c, "create", "secpulse/ci-evidence", evidenceID, title)

	return c.JSON(http.StatusCreated, map[string]string{
		"id":      evidenceID,
		"message": "CI evidence recorded",
	})
}
