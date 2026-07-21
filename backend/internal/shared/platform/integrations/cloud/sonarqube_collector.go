// Copyright (c) 2026 NorvikOps. All rights reserved.
// SPDX-License-Identifier: Elastic-2.0

package cloud

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/matharnica/vakt/internal/shared/httputil"
	"github.com/rs/zerolog/log"
)

const sonarqubeSource = "sonarqube-collector"

// SonarQubeCollector collects compliance evidence from a SonarQube or SonarCloud instance.
type SonarQubeCollector struct {
	db         *pgxpool.Pool
	evidence   EvidenceWriter
	httpClient *http.Client // injectable for tests; nil in production (see clientFor)
}

// NewSonarQubeCollector creates a new SonarQubeCollector.
func NewSonarQubeCollector(db *pgxpool.Pool, evidence EvidenceWriter) *SonarQubeCollector {
	return &SonarQubeCollector{
		db:       db,
		evidence: evidence,
	}
}

// clientFor returns the client for this run. Tests inject c.httpClient
// directly and it is used as-is; in production it is nil and we build a
// dial-guarded client honouring the config's allow_private_target
// (S121-F4 / F1-Inj: closes the DNS-rebinding TOCTOU window that
// ValidateOutboundURL alone leaves open).
func (c *SonarQubeCollector) clientFor(allowPrivate bool) *http.Client {
	if c.httpClient != nil {
		return c.httpClient
	}
	return httputil.GuardedClient(30*time.Second, allowPrivate)
}

// sonarProject is a minimal SonarQube project representation.
type sonarProject struct {
	Key  string `json:"key"`
	Name string `json:"name"`
}

type sonarProjectsResponse struct {
	Paging struct {
		PageIndex int `json:"pageIndex"`
		PageSize  int `json:"pageSize"`
		Total     int `json:"total"`
	} `json:"paging"`
	Components []sonarProject `json:"components"`
}

type sonarQGStatus struct {
	ProjectStatus struct {
		Status string `json:"status"` // "OK" | "WARN" | "ERROR" | "NONE"
	} `json:"projectStatus"`
}

type sonarHotspotsResponse struct {
	Paging struct {
		Total int `json:"total"`
	} `json:"paging"`
	Hotspots []sonarHotspot `json:"hotspots"`
}

type sonarHotspot struct {
	Key                      string `json:"key"`
	Component                string `json:"component"`
	SecurityCategory         string `json:"securityCategory"`
	VulnerabilityProbability string `json:"vulnerabilityProbability"`
	Status                   string `json:"status"`
	Message                  string `json:"message"`
	RuleKey                  string `json:"ruleKey"`
	Project                  string `json:"project"`
}

type sonarIssuesResponse struct {
	Total  int          `json:"total"`
	Issues []sonarIssue `json:"issues"`
}

type sonarIssue struct {
	Key       string `json:"key"`
	Component string `json:"component"`
	Severity  string `json:"severity"`
	Type      string `json:"type"`
	Message   string `json:"message"`
	Rule      string `json:"rule"`
	Project   string `json:"project"`
	Line      int    `json:"line"`
}

// Collect runs all SonarQube evidence collectors for the given org and config.
func (c *SonarQubeCollector) Collect(ctx context.Context, orgID string, cfg SonarQubeConfig) (int, error) {
	client := c.clientFor(cfg.AllowPrivateTarget)

	projects, err := c.listProjects(ctx, client, cfg)
	if err != nil {
		return 0, fmt.Errorf("sonarqube: list projects: %w", err)
	}

	sdlcControls, _ := c.evidence.FindControlsByKeywords(ctx, orgID, []string{"secure development", "sdlc", "sast", "code quality"})
	vulnControls, _ := c.evidence.FindControlsByKeywords(ctx, orgID, []string{"vulnerability", "patch", "cve"})
	assetControls, _ := c.evidence.FindControlsByKeywords(ctx, orgID, []string{"asset", "inventory", "software"})

	total := 0
	// F3/R-H20: accumulate sub-collector failures (see cloud collector comment).
	var errs []error

	// Inventory
	invDetails := map[string]any{
		"collected_at":  time.Now().UTC().Format(time.RFC3339),
		"project_count": len(projects),
		"sonarqube_url": cfg.BaseURL,
	}
	if err := c.addEvidence(ctx, orgID, firstControlID(assetControls), "SonarQube Projekt-Inventar", invDetails); err == nil {
		total++
	}

	// Quality Gates
	failedQG := []string{}
	okQG := []string{}
	for _, p := range projects {
		status, err := c.getQualityGateStatus(ctx, client, cfg, p.Key)
		if err != nil {
			log.Warn().Err(err).Str("project", p.Key).Msg("sonarqube_collector: quality gate status failed")
			errs = append(errs, err)
			continue
		}
		switch status {
		case "ERROR", "WARN":
			failedQG = append(failedQG, p.Name)
			details := map[string]any{
				"collected_at": time.Now().UTC().Format(time.RFC3339),
				"project_key":  p.Key,
				"project_name": p.Name,
				"qg_status":    status,
				"warning":      fmt.Sprintf("SonarQube: %s Quality Gate %s.", p.Name, status),
			}
			if err := c.addEvidence(ctx, orgID, firstControlID(sdlcControls),
				fmt.Sprintf("SonarQube Quality Gate FAILED: %s", p.Name), details); err == nil {
				total++
			}
		case "OK":
			okQG = append(okQG, p.Name)
		}
	}

	// Summary: all OK
	if len(failedQG) == 0 && len(okQG) > 0 {
		details := map[string]any{
			"collected_at":  time.Now().UTC().Format(time.RFC3339),
			"project_count": len(okQG),
			"projects":      okQG,
		}
		if err := c.addEvidence(ctx, orgID, firstControlID(sdlcControls),
			fmt.Sprintf("SonarQube Quality Gate: %d Projekte alle grün", len(okQG)), details); err == nil {
			total++
		}
	}

	// Security Hotspots → vaktscan findings
	hotspotCount, err := c.collectSecurityHotspots(ctx, client, orgID, cfg)
	if err != nil {
		log.Warn().Err(err).Msg("sonarqube_collector: hotspot collection failed")
		errs = append(errs, err)
	} else {
		total += hotspotCount
	}

	// Critical/Blocker vulnerabilities → vaktscan findings
	vulnCount, err := c.collectVulnerabilities(ctx, client, orgID, cfg)
	if err != nil {
		log.Warn().Err(err).Msg("sonarqube_collector: vulnerability collection failed")
		errs = append(errs, err)
	} else {
		total += vulnCount
	}

	// Hotspot summary evidence
	if hotspotCount > 0 {
		details := map[string]any{
			"collected_at":  time.Now().UTC().Format(time.RFC3339),
			"hotspot_count": hotspotCount,
		}
		if err := c.addEvidence(ctx, orgID, firstControlID(vulnControls),
			fmt.Sprintf("SonarQube: %d unreviewed Security Hotspots importiert", hotspotCount), details); err == nil {
			total++
		}
	}

	if total == 0 && len(errs) > 0 {
		return 0, errors.Join(errs...)
	}
	return total, nil
}

// listProjects returns all SonarQube projects (paginated).
func (c *SonarQubeCollector) listProjects(ctx context.Context, client *http.Client, cfg SonarQubeConfig) ([]sonarProject, error) {
	var all []sonarProject
	page := 1
	const pageSize = 500

	for {
		apiURL := fmt.Sprintf("%s/api/projects/search?p=%d&ps=%d",
			strings.TrimRight(cfg.BaseURL, "/"), page, pageSize)

		body, err := c.sonarRequest(ctx, client, cfg.Token, apiURL)
		if err != nil {
			return nil, err
		}

		var resp sonarProjectsResponse
		if err := json.Unmarshal(body, &resp); err != nil {
			return nil, fmt.Errorf("parse projects response: %w", err)
		}

		all = append(all, resp.Components...)

		if len(all) >= resp.Paging.Total {
			break
		}
		page++
	}
	return all, nil
}

// getQualityGateStatus returns "OK", "WARN", "ERROR", or "NONE" for a project.
func (c *SonarQubeCollector) getQualityGateStatus(ctx context.Context, client *http.Client, cfg SonarQubeConfig, projectKey string) (string, error) {
	apiURL := fmt.Sprintf("%s/api/qualitygates/project_status?projectKey=%s",
		strings.TrimRight(cfg.BaseURL, "/"), projectKey)

	body, err := c.sonarRequest(ctx, client, cfg.Token, apiURL)
	if err != nil {
		return "", err
	}

	var resp sonarQGStatus
	if err := json.Unmarshal(body, &resp); err != nil {
		return "", err
	}
	return resp.ProjectStatus.Status, nil
}

// collectSecurityHotspots imports unreviewed SonarQube security hotspots as vaktscan findings.
func (c *SonarQubeCollector) collectSecurityHotspots(ctx context.Context, client *http.Client, orgID string, cfg SonarQubeConfig) (int, error) {
	apiURL := fmt.Sprintf("%s/api/hotspots/search?status=TO_REVIEW&ps=500",
		strings.TrimRight(cfg.BaseURL, "/"))

	body, err := c.sonarRequest(ctx, client, cfg.Token, apiURL)
	if err != nil {
		return 0, err
	}

	var resp sonarHotspotsResponse
	if err := json.Unmarshal(body, &resp); err != nil {
		return 0, err
	}

	baseURLHash := sonarURLHash(cfg.BaseURL)
	count := 0

	for _, h := range resp.Hotspots {
		rawID := fmt.Sprintf("sonarqube:%s:%s:%s:%s", baseURLHash, h.Project, h.RuleKey, h.Component)
		title := fmt.Sprintf("SonarQube Hotspot: %s", h.Message)
		if len(title) > 200 {
			title = title[:200]
		}

		_, err := c.db.Exec(ctx, `
			INSERT INTO vb_findings
				(org_id, asset_id, title, description, severity, status, scanner, raw_id, sources, last_seen_at)
			VALUES
				($1::uuid, '', $2, $3, 'high', 'open', 'sonarqube', $4, ARRAY['sonarqube'], NOW())
			ON CONFLICT (org_id, raw_id, scanner) WHERE raw_id IS NOT NULL
			DO UPDATE SET last_seen_at = NOW()`,
			orgID, title,
			fmt.Sprintf("Kategorie: %s — %s in %s", h.SecurityCategory, h.Message, h.Component),
			rawID,
		)
		if err != nil {
			log.Warn().Err(err).Str("raw_id", rawID).Msg("sonarqube_collector: upsert hotspot finding")
			continue
		}
		count++
	}
	return count, nil
}

// collectVulnerabilities imports BLOCKER/CRITICAL vulnerability issues as vaktscan findings.
func (c *SonarQubeCollector) collectVulnerabilities(ctx context.Context, client *http.Client, orgID string, cfg SonarQubeConfig) (int, error) {
	apiURL := fmt.Sprintf("%s/api/issues/search?severities=BLOCKER,CRITICAL&types=VULNERABILITY&ps=500",
		strings.TrimRight(cfg.BaseURL, "/"))

	body, err := c.sonarRequest(ctx, client, cfg.Token, apiURL)
	if err != nil {
		return 0, err
	}

	var resp sonarIssuesResponse
	if err := json.Unmarshal(body, &resp); err != nil {
		return 0, err
	}

	baseURLHash := sonarURLHash(cfg.BaseURL)
	count := 0

	for _, issue := range resp.Issues {
		rawID := fmt.Sprintf("sonarqube:%s:%s:%s:%s:%d", baseURLHash, issue.Project, issue.Rule, issue.Component, issue.Line)
		severity := "high"
		if issue.Severity == "BLOCKER" {
			severity = "critical"
		}
		title := issue.Message
		if len(title) > 200 {
			title = title[:200]
		}
		if title == "" {
			title = fmt.Sprintf("SonarQube Vulnerability: %s", issue.Rule)
		}

		_, err := c.db.Exec(ctx, `
			INSERT INTO vb_findings
				(org_id, asset_id, title, description, severity, status, scanner, raw_id, sources, last_seen_at)
			VALUES
				($1::uuid, '', $2, $3, $4, 'open', 'sonarqube', $5, ARRAY['sonarqube'], NOW())
			ON CONFLICT (org_id, raw_id, scanner) WHERE raw_id IS NOT NULL
			DO UPDATE SET last_seen_at = NOW()`,
			orgID, title,
			fmt.Sprintf("Rule: %s — %s in %s", issue.Rule, issue.Message, issue.Component),
			severity, rawID,
		)
		if err != nil {
			log.Warn().Err(err).Str("raw_id", rawID).Msg("sonarqube_collector: upsert vuln finding")
			continue
		}
		count++
	}
	return count, nil
}

// sonarRequest performs a GET request to the SonarQube API with token auth.
func (c *SonarQubeCollector) sonarRequest(ctx context.Context, client *http.Client, token, apiURL string) ([]byte, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, apiURL, nil)
	if err != nil {
		return nil, err
	}
	req.SetBasicAuth(token, "")

	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusUnauthorized {
		return nil, fmt.Errorf("sonarqube API: unauthorized (401) — token ungültig")
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("sonarqube API: status %d for %s", resp.StatusCode, apiURL)
	}

	return io.ReadAll(resp.Body)
}

func (c *SonarQubeCollector) addEvidence(ctx context.Context, orgID, controlID, title string, details map[string]any) error {
	if controlID == "" {
		return nil
	}
	data, _ := json.Marshal(details)
	return c.evidence.AddCollectorEvidence(ctx, orgID, controlID, "", sonarqubeSource, title, data)
}

// sonarURLHash returns a short hash of the SonarQube base URL for raw_id deduplication.
func sonarURLHash(baseURL string) string {
	h := sha256.Sum256([]byte(strings.ToLower(strings.TrimRight(baseURL, "/"))))
	return fmt.Sprintf("%x", h[:4])
}

// CountQualityGateFailed returns the number of projects with a failed Quality Gate.
func (c *SonarQubeCollector) CountQualityGateFailed(ctx context.Context, cfg SonarQubeConfig) (projectCount, failedCount int, err error) {
	client := c.clientFor(cfg.AllowPrivateTarget)

	projects, err := c.listProjects(ctx, client, cfg)
	if err != nil {
		return 0, 0, err
	}
	for _, p := range projects {
		status, qErr := c.getQualityGateStatus(ctx, client, cfg, p.Key)
		if qErr != nil {
			continue
		}
		if status == "ERROR" || status == "WARN" {
			failedCount++
		}
	}
	return len(projects), failedCount, nil
}
