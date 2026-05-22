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
	"net"
	"net/http"
	"os"
	"os/exec"
	"strings"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/rs/zerolog/log"
)

// ErrNotConfigured is returned when a scanner is not configured in the environment.
var ErrNotConfigured = errors.New("scanner not configured")

// privateRanges holds the RFC-1918 CIDR blocks that are considered private.
var privateRanges = []net.IPNet{
	parseCIDR("10.0.0.0/8"),
	parseCIDR("172.16.0.0/12"),
	parseCIDR("192.168.0.0/16"),
}

// parseCIDR parses a hardcoded CIDR block; terminates with a fatal log on
// invalid input so startup failures appear in structured logs rather than a
// bare panic trace.
func parseCIDR(cidr string) net.IPNet {
	_, network, err := net.ParseCIDR(cidr)
	if err != nil {
		log.Fatal().Err(err).Str("cidr", cidr).Msg("secpulse: invalid built-in CIDR")
	}
	return *network
}

// isPrivateOrLoopback returns true when target resolves to a loopback address,
// an IPv6 link-local address, or an RFC-1918 private range.  It also catches
// the string literals "localhost" and "::1" before any DNS resolution.
// When VAKT_SCAN_ALLOW_PRIVATE=true the caller bypasses this check entirely.
func isPrivateOrLoopback(target string) bool {
	// Strip port if present (e.g. "192.168.1.1:8080" or "[::1]:443").
	host := target
	if h, _, err := net.SplitHostPort(target); err == nil {
		host = h
	}

	// Fast-path: well-known names that net.ParseIP won't catch.
	if strings.EqualFold(host, "localhost") || host == "::1" {
		return true
	}

	ip := net.ParseIP(host)
	if ip == nil {
		// Not a bare IP — not a private literal, let the scanner handle DNS.
		return false
	}

	if ip.IsLoopback() || ip.IsLinkLocalUnicast() {
		return true
	}

	for _, network := range privateRanges {
		if network.Contains(ip) {
			return true
		}
	}
	return false
}

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

	// Reject argument-injection patterns in asset name targets.
	if strings.HasPrefix(target, "-") || strings.ContainsAny(target, `/\`) {
		return fmt.Errorf("trivy: invalid scan target %q", target)
	}

	// Block scans against private/loopback addresses to prevent SSRF-style
	// internal infrastructure scanning unless the operator has explicitly
	// opted in via VAKT_SCAN_ALLOW_PRIVATE=true.
	if os.Getenv("VAKT_SCAN_ALLOW_PRIVATE") == "true" {
		log.Info().Str("scan_id", payload.ScanID).Msg("trivy: VAKT_SCAN_ALLOW_PRIVATE=true — private/loopback targets permitted")
	} else if isPrivateOrLoopback(target) {
		return fmt.Errorf("scan target %q is in a private or loopback range — configure VAKT_SCAN_ALLOW_PRIVATE=true to allow internal scans", target)
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

	// Reject argument-injection patterns in asset name targets.
	if strings.HasPrefix(target, "-") || strings.ContainsAny(target, `/\`) {
		return fmt.Errorf("nuclei: invalid scan target %q", target)
	}

	// Block scans against private/loopback addresses to prevent SSRF-style
	// internal infrastructure scanning unless the operator has explicitly
	// opted in via VAKT_SCAN_ALLOW_PRIVATE=true.
	if os.Getenv("VAKT_SCAN_ALLOW_PRIVATE") == "true" {
		log.Info().Str("scan_id", payload.ScanID).Msg("nuclei: VAKT_SCAN_ALLOW_PRIVATE=true — private/loopback targets permitted")
	} else if isPrivateOrLoopback(target) {
		return fmt.Errorf("scan target %q is in a private or loopback range — configure VAKT_SCAN_ALLOW_PRIVATE=true to allow internal scans", target)
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
	_ = resp.Body.Close()
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
		_ = resp.Body.Close()

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

// epssAPIResponse is the parsed response from https://api.first.org/data/v1/epss.
type epssAPIResponse struct {
	Data []struct {
		CVE        string `json:"cve"`
		EPSS       string `json:"epss"`
		Percentile string `json:"percentile"`
	} `json:"data"`
}

// UpdateEPSSScores fetches EPSS scores from the FIRST.org API for all open findings
// that have a CVE ID and updates epss_score + epss_percentile in the database.
// Findings without a CVE are skipped. HTTP errors are logged and not fatal.
func UpdateEPSSScores(ctx context.Context, db *pgxpool.Pool, orgID string) error {
	// 1. Collect distinct CVE IDs for open findings in this org.
	rows, err := db.Query(ctx, `
		SELECT DISTINCT cve_id
		FROM vb_findings
		WHERE org_id = $1::uuid
		  AND cve_id IS NOT NULL
		  AND cve_id <> ''
		  AND status NOT IN ('resolved', 'false_positive')
	`, orgID)
	if err != nil {
		return fmt.Errorf("epss: query cve ids: %w", err)
	}
	defer rows.Close()

	var cveIDs []string
	for rows.Next() {
		var cve string
		if err := rows.Scan(&cve); err != nil {
			continue
		}
		cveIDs = append(cveIDs, cve)
	}
	if err := rows.Err(); err != nil {
		return fmt.Errorf("epss: scan cve ids: %w", err)
	}
	if len(cveIDs) == 0 {
		log.Info().Str("org_id", orgID).Msg("epss: no CVE IDs found, skipping enrichment")
		return nil
	}

	// 2. Process in batches of 100 (FIRST API limit).
	const batchSize = 100
	httpClient := &http.Client{Timeout: 30 * time.Second}

	for i := 0; i < len(cveIDs); i += batchSize {
		end := i + batchSize
		if end > len(cveIDs) {
			end = len(cveIDs)
		}
		batch := cveIDs[i:end]

		cveParam := strings.Join(batch, ",")
		apiURL := "https://api.first.org/data/v1/epss?cve=" + cveParam

		req, err := http.NewRequestWithContext(ctx, http.MethodGet, apiURL, nil)
		if err != nil {
			log.Warn().Err(err).Msg("epss: build request failed, skipping batch")
			continue
		}

		resp, err := httpClient.Do(req)
		if err != nil {
			log.Warn().Err(err).Msg("epss: HTTP request failed, skipping batch")
			continue
		}

		body, readErr := io.ReadAll(resp.Body)
		_ = resp.Body.Close()
		if readErr != nil {
			log.Warn().Err(readErr).Msg("epss: read response body failed, skipping batch")
			continue
		}

		if resp.StatusCode != http.StatusOK {
			log.Warn().
				Int("status", resp.StatusCode).
				Str("body", string(bytes.TrimSpace(body))).
				Msg("epss: non-200 response, skipping batch")
			continue
		}

		var apiResp epssAPIResponse
		if err := json.Unmarshal(body, &apiResp); err != nil {
			log.Warn().Err(err).Msg("epss: parse response failed, skipping batch")
			continue
		}

		// 3. Update each CVE that returned data.
		for _, entry := range apiResp.Data {
			if entry.CVE == "" || entry.EPSS == "" {
				continue
			}

			var epssScore, epssPercentile float64
			if _, err := fmt.Sscanf(entry.EPSS, "%f", &epssScore); err != nil {
				log.Warn().Str("cve", entry.CVE).Str("epss", entry.EPSS).Msg("epss: parse epss score failed")
				continue
			}
			if _, err := fmt.Sscanf(entry.Percentile, "%f", &epssPercentile); err != nil {
				log.Warn().Str("cve", entry.CVE).Str("percentile", entry.Percentile).Msg("epss: parse percentile failed")
				continue
			}

			_, updateErr := db.Exec(ctx, `
				UPDATE vb_findings
				SET epss_score      = $1,
				    epss_percentile = $2,
				    updated_at      = NOW()
				WHERE org_id = $3::uuid
				  AND cve_id = $4
				  AND status NOT IN ('resolved', 'false_positive')
			`, epssScore, epssPercentile, orgID, entry.CVE)
			if updateErr != nil {
				log.Warn().Err(updateErr).Str("cve", entry.CVE).Msg("epss: update finding failed")
			}
		}

		log.Info().
			Str("org_id", orgID).
			Int("batch_start", i).
			Int("batch_size", len(batch)).
			Int("results", len(apiResp.Data)).
			Msg("epss: batch enriched")
	}

	return nil
}
