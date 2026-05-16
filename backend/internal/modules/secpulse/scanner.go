// Copyright (c) 2026 NorvikOps. All rights reserved.
// SPDX-License-Identifier: Elastic-2.0

package secpulse

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os/exec"
	"strings"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/rs/zerolog/log"
)

// ErrNotConfigured is returned when a scanner is not configured in the environment.
var ErrNotConfigured = errors.New("scanner not configured")

// trivyOutput matches the top-level structure of trivy JSON output.
type trivyOutput struct {
	Results []trivyResult `json:"Results"`
}

type trivyResult struct {
	Vulnerabilities []trivyVuln `json:"Vulnerabilities"`
}

type trivyVuln struct {
	VulnerabilityID string    `json:"VulnerabilityID"`
	Title           string    `json:"Title"`
	Description     string    `json:"Description"`
	Severity        string    `json:"Severity"`
	CVSS            trivyCVSS `json:"CVSS"`
}

type trivyCVSS struct {
	NVD struct {
		V3Score float64 `json:"V3Score"`
	} `json:"nvd"`
}

// nucleiResult matches one JSON line from nuclei -json output.
type nucleiResult struct {
	TemplateID string `json:"template-id"`
	Info       struct {
		Name     string `json:"name"`
		Severity string `json:"severity"`
	} `json:"info"`
	MatchedAt string `json:"matched-at"`
}

// RunTrivyScan executes trivy against the scan target, normalises findings into
// vb_findings, and updates the vb_scans record accordingly.
func RunTrivyScan(ctx context.Context, db *pgxpool.Pool, payload ScanPayload) error {
	repo := NewRepository(db)
	startedAt := time.Now()

	if err := repo.UpdateScanStatus(ctx, payload.ScanID, "running",
		WithStartedAt(startedAt)); err != nil {
		return fmt.Errorf("mark scan running: %w", err)
	}

	target := payload.TargetURL
	if target == "" {
		target = payload.AssetName
	}

	args := []string{"image", "--format", "json", "--quiet", target}
	if payload.TargetIP != "" {
		args = []string{"fs", "--format", "json", "--quiet", payload.TargetIP}
	}

	out, runErr := exec.CommandContext(ctx, "trivy", args...).Output()
	durationMs := time.Since(startedAt).Milliseconds()

	if runErr != nil {
		_ = repo.UpdateScanStatus(ctx, payload.ScanID, "failed",
			WithErrorMessage(runErr.Error()),
			WithDurationMs(durationMs),
			WithCompletedAt(time.Now()))
		return fmt.Errorf("trivy exec: %w", runErr)
	}

	var trivyOut trivyOutput
	if err := json.Unmarshal(out, &trivyOut); err != nil {
		_ = repo.UpdateScanStatus(ctx, payload.ScanID, "failed",
			WithErrorMessage("failed to parse trivy output: "+err.Error()),
			WithDurationMs(durationMs),
			WithCompletedAt(time.Now()))
		return fmt.Errorf("parse trivy output: %w", err)
	}

	scanIDPtr := &payload.ScanID
	var findings []Finding
	for _, result := range trivyOut.Results {
		for _, vuln := range result.Vulnerabilities {
			severity := strings.ToLower(vuln.Severity)
			if severity == "" {
				severity = "info"
			}

			var cvss *float64
			if vuln.CVSS.NVD.V3Score > 0 {
				v := vuln.CVSS.NVD.V3Score
				cvss = &v
			}

			var cveIDPtr *string
			if vuln.VulnerabilityID != "" {
				cveID := vuln.VulnerabilityID
				cveIDPtr = &cveID
			}

			f := Finding{
				OrgID:       payload.OrgID,
				AssetID:     payload.AssetID,
				ScanID:      scanIDPtr,
				CVEID:       cveIDPtr,
				Title:       vuln.Title,
				Description: vuln.Description,
				Severity:    severity,
				CVSSScore:   cvss,
				Scanner:     "trivy",
				Sources:     []string{"trivy"},
				Status:      "open",
				LastSeenAt:  time.Now(),
			}
			ComputeRiskScore(&f)
			findings = append(findings, f)
		}
	}
	count, _ := repo.BatchUpsertFindings(ctx, payload.OrgID, findings)

	_ = repo.UpdateScanStatus(ctx, payload.ScanID, "completed",
		WithFindingCount(count),
		WithDurationMs(durationMs),
		WithCompletedAt(time.Now()))

	log.Info().Str("scan_id", payload.ScanID).Int("findings", count).Msg("trivy scan complete")
	return nil
}

// RunNucleiScan executes nuclei against the scan target and normalises findings.
func RunNucleiScan(ctx context.Context, db *pgxpool.Pool, payload ScanPayload) error {
	repo := NewRepository(db)
	startedAt := time.Now()

	if err := repo.UpdateScanStatus(ctx, payload.ScanID, "running",
		WithStartedAt(startedAt)); err != nil {
		return fmt.Errorf("mark scan running: %w", err)
	}

	target := payload.TargetURL
	if target == "" {
		target = payload.TargetIP
	}
	if target == "" {
		target = payload.AssetName
	}

	out, runErr := exec.CommandContext(ctx, "nuclei",
		"-target", target,
		"-json",
		"-silent",
	).Output()
	durationMs := time.Since(startedAt).Milliseconds()

	if runErr != nil {
		_ = repo.UpdateScanStatus(ctx, payload.ScanID, "failed",
			WithErrorMessage(runErr.Error()),
			WithDurationMs(durationMs),
			WithCompletedAt(time.Now()))
		return fmt.Errorf("nuclei exec: %w", runErr)
	}

	scanIDPtr := &payload.ScanID
	var findings []Finding
	decoder := json.NewDecoder(bytes.NewReader(out))
	for decoder.More() {
		var r nucleiResult
		if err := decoder.Decode(&r); err != nil {
			continue
		}

		severity := strings.ToLower(r.Info.Severity)
		if severity == "" {
			severity = "info"
		}

		f := Finding{
			OrgID:      payload.OrgID,
			AssetID:    payload.AssetID,
			ScanID:     scanIDPtr,
			Title:      r.Info.Name,
			Severity:   severity,
			Scanner:    "nuclei",
			TemplateID: r.TemplateID,
			Sources:    []string{"nuclei"},
			Status:     "open",
			LastSeenAt: time.Now(),
		}
		ComputeRiskScore(&f)
		findings = append(findings, f)
	}
	count, _ := repo.BatchUpsertFindings(ctx, payload.OrgID, findings)

	_ = repo.UpdateScanStatus(ctx, payload.ScanID, "completed",
		WithFindingCount(count),
		WithDurationMs(durationMs),
		WithCompletedAt(time.Now()))

	log.Info().Str("scan_id", payload.ScanID).Int("findings", count).Msg("nuclei scan complete")
	return nil
}

// gvmTask represents a GVM task object returned by the REST API.
type gvmTask struct {
	ID     string `json:"id"`
	Status string `json:"status"`
}

// gvmResult represents a single vulnerability result from GVM.
type gvmResult struct {
	ID          string  `json:"id"`
	Name        string  `json:"name"`
	Description string  `json:"description"`
	Severity    float64 `json:"severity"`
	NVT         struct {
		CVSS string `json:"cvss_base"`
		OID  string `json:"oid"`
	} `json:"nvt"`
}

// gvmClient is a minimal GVM REST API client.
type gvmClient struct {
	baseURL string
	user    string
	pass    string
	http    *http.Client
}

// newGVMClient creates a GVM REST client from env variables.
// Returns (nil, ErrNotConfigured) when VAKT_OPENVAS_URL is not set.
func newGVMClient() (*gvmClient, error) {
	baseURL := getEnv("VAKT_OPENVAS_URL")
	if baseURL == "" {
		return nil, fmt.Errorf("OpenVAS not configured (VAKT_OPENVAS_URL not set): %w", ErrNotConfigured)
	}
	return &gvmClient{
		baseURL: strings.TrimRight(baseURL, "/"),
		user:    getEnv("VAKT_OPENVAS_USER"),
		pass:    getEnv("VAKT_OPENVAS_PASS"),
		http:    &http.Client{Timeout: 30 * time.Second},
	}, nil
}

// do executes an authenticated HTTP request against the GVM REST API.
func (c *gvmClient) do(ctx context.Context, method, path string, body io.Reader) (*http.Response, error) {
	req, err := http.NewRequestWithContext(ctx, method, c.baseURL+path, body)
	if err != nil {
		return nil, fmt.Errorf("gvm: build request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	if c.user != "" {
		req.SetBasicAuth(c.user, c.pass)
	}
	resp, err := c.http.Do(req)
	if err != nil {
		return nil, fmt.Errorf("gvm: %s %s: %w", method, path, err)
	}
	if resp.StatusCode >= 400 {
		_ = resp.Body.Close()
		return nil, fmt.Errorf("gvm: %s %s returned HTTP %d", method, path, resp.StatusCode)
	}
	return resp, nil
}

// createTask creates a GVM scan task for the given target and returns its ID.
func (c *gvmClient) createTask(ctx context.Context, target string) (string, error) {
	reqBody, _ := json.Marshal(map[string]string{
		"name":   "secpulse-scan-" + target,
		"target": target,
	})
	resp, err := c.do(ctx, http.MethodPost, "/gvm/tasks", bytes.NewReader(reqBody))
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	var task gvmTask
	if err := json.NewDecoder(resp.Body).Decode(&task); err != nil {
		return "", fmt.Errorf("gvm: decode createTask response: %w", err)
	}
	return task.ID, nil
}

// startTask starts a previously created GVM task.
func (c *gvmClient) startTask(ctx context.Context, taskID string) error {
	resp, err := c.do(ctx, http.MethodPost, "/gvm/tasks/"+taskID+"/start", nil)
	if err != nil {
		return err
	}
	resp.Body.Close()
	return nil
}

// pollTask polls the task status until it is "Done" or "Stopped", with a 10 min
// timeout and 10-second polling interval.
func (c *gvmClient) pollTask(ctx context.Context, taskID string) error {
	deadline := time.Now().Add(10 * time.Minute)
	for {
		if time.Now().After(deadline) {
			return fmt.Errorf("gvm: task %s timed out after 10 minutes", taskID)
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(10 * time.Second):
		}

		resp, err := c.do(ctx, http.MethodGet, "/gvm/tasks/"+taskID, nil)
		if err != nil {
			return err
		}
		var task gvmTask
		_ = json.NewDecoder(resp.Body).Decode(&task)
		resp.Body.Close()

		switch task.Status {
		case "Done":
			return nil
		case "Stopped", "Interrupted":
			return fmt.Errorf("gvm: task %s ended with status %q", taskID, task.Status)
		}
	}
}

// fetchResults retrieves all vulnerability results for the given task.
func (c *gvmClient) fetchResults(ctx context.Context, taskID string) ([]gvmResult, error) {
	resp, err := c.do(ctx, http.MethodGet, "/gvm/results?task_id="+taskID, nil)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	var results []gvmResult
	if err := json.NewDecoder(resp.Body).Decode(&results); err != nil {
		return nil, fmt.Errorf("gvm: decode results: %w", err)
	}
	return results, nil
}

// RunOpenVASScan calls the GVM REST API if VAKT_OPENVAS_URL is set in the environment.
// Returns ErrNotConfigured with a clear message when VAKT_OPENVAS_URL is absent.
func RunOpenVASScan(ctx context.Context, db *pgxpool.Pool, payload ScanPayload) error {
	client, err := newGVMClient()
	if err != nil {
		// Surface a clear error instead of a silent success stub.
		return err
	}

	repo := NewRepository(db)
	startedAt := time.Now()
	if err := repo.UpdateScanStatus(ctx, payload.ScanID, "running",
		WithStartedAt(startedAt)); err != nil {
		return fmt.Errorf("mark scan running: %w", err)
	}

	target := payload.TargetIP
	if target == "" {
		target = payload.TargetURL
	}
	if target == "" {
		target = payload.AssetName
	}

	log.Info().Str("scan_id", payload.ScanID).Str("target", target).Msg("openvas: creating GVM task")

	taskID, err := client.createTask(ctx, target)
	if err != nil {
		_ = repo.UpdateScanStatus(ctx, payload.ScanID, "failed",
			WithErrorMessage("gvm createTask: "+err.Error()),
			WithDurationMs(time.Since(startedAt).Milliseconds()),
			WithCompletedAt(time.Now()))
		return fmt.Errorf("openvas createTask: %w", err)
	}

	if err := client.startTask(ctx, taskID); err != nil {
		_ = repo.UpdateScanStatus(ctx, payload.ScanID, "failed",
			WithErrorMessage("gvm startTask: "+err.Error()),
			WithDurationMs(time.Since(startedAt).Milliseconds()),
			WithCompletedAt(time.Now()))
		return fmt.Errorf("openvas startTask: %w", err)
	}

	log.Info().Str("scan_id", payload.ScanID).Str("gvm_task_id", taskID).Msg("openvas: task started, polling for completion")

	if err := client.pollTask(ctx, taskID); err != nil {
		_ = repo.UpdateScanStatus(ctx, payload.ScanID, "failed",
			WithErrorMessage("gvm pollTask: "+err.Error()),
			WithDurationMs(time.Since(startedAt).Milliseconds()),
			WithCompletedAt(time.Now()))
		return fmt.Errorf("openvas pollTask: %w", err)
	}

	results, err := client.fetchResults(ctx, taskID)
	if err != nil {
		_ = repo.UpdateScanStatus(ctx, payload.ScanID, "failed",
			WithErrorMessage("gvm fetchResults: "+err.Error()),
			WithDurationMs(time.Since(startedAt).Milliseconds()),
			WithCompletedAt(time.Now()))
		return fmt.Errorf("openvas fetchResults: %w", err)
	}

	durationMs := time.Since(startedAt).Milliseconds()
	scanIDPtr := &payload.ScanID
	var findings []Finding
	for _, r := range results {
		severity := "info"
		switch {
		case r.Severity >= 9.0:
			severity = "critical"
		case r.Severity >= 7.0:
			severity = "high"
		case r.Severity >= 4.0:
			severity = "medium"
		case r.Severity > 0:
			severity = "low"
		}

		var cvss *float64
		if r.Severity > 0 {
			v := r.Severity
			cvss = &v
		}

		f := Finding{
			OrgID:       payload.OrgID,
			AssetID:     payload.AssetID,
			ScanID:      scanIDPtr,
			Title:       r.Name,
			Description: r.Description,
			Severity:    severity,
			CVSSScore:   cvss,
			Scanner:     "openvas",
			Sources:     []string{"openvas"},
			Status:      "open",
			LastSeenAt:  time.Now(),
		}
		ComputeRiskScore(&f)
		findings = append(findings, f)
	}
	count, _ := repo.BatchUpsertFindings(ctx, payload.OrgID, findings)

	_ = repo.UpdateScanStatus(ctx, payload.ScanID, "completed",
		WithFindingCount(count),
		WithDurationMs(durationMs),
		WithCompletedAt(time.Now()))

	log.Info().Str("scan_id", payload.ScanID).Str("gvm_task_id", taskID).Int("findings", count).Msg("openvas scan complete")
	return nil
}

// ComputeRiskScore calculates and sets the risk_score on a Finding.
// risk_score = cvss_score * (1 + epss_percentile) * criticality_multiplier
// If cvss_score is nil, defaults to 5.0.
func ComputeRiskScore(f *Finding) {
	cvss := 5.0
	if f.CVSSScore != nil {
		cvss = *f.CVSSScore
	}

	epssMultiplier := 1.0
	if f.EPSSPercentile != nil {
		epssMultiplier = 1.0 + *f.EPSSPercentile
	}

	var critMultiplier float64
	switch f.Severity {
	case "critical":
		critMultiplier = 2.0
	case "high":
		critMultiplier = 1.5
	case "medium":
		critMultiplier = 1.0
	case "low":
		critMultiplier = 0.5
	default:
		critMultiplier = 0.25
	}

	score := cvss * epssMultiplier * critMultiplier
	f.RiskScore = &score
}

// UpdateEPSSScores is a placeholder for EPSS enrichment.
// A real implementation would call the FIRST EPSS API and update cvss/epss fields.
func UpdateEPSSScores(ctx context.Context, db *pgxpool.Pool, orgID string) error {
	log.Info().Str("org_id", orgID).
		Msg("EPSS enrichment would call FIRST-EPSS API here")
	return nil
}
