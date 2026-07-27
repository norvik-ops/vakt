// Copyright (c) 2026 NorvikOps. All rights reserved.
// SPDX-License-Identifier: Elastic-2.0

package cloud

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/matharnica/vakt/internal/shared/httputil"
	"github.com/rs/zerolog/log"
)

const gitlabSource = "gitlab-collector"

// GitLabCollector collects compliance evidence from a GitLab instance (self-managed or GitLab.com).
type GitLabCollector struct {
	db         *pgxpool.Pool
	evidence   EvidenceWriter
	httpClient *http.Client // injectable for tests; nil in production (see clientFor)
}

// NewGitLabCollector creates a new GitLabCollector.
func NewGitLabCollector(db *pgxpool.Pool, evidence EvidenceWriter) *GitLabCollector {
	return &GitLabCollector{
		db:       db,
		evidence: evidence,
	}
}

// clientFor returns the client for this run. Tests inject c.httpClient
// directly and it is used as-is; in production it is nil and we build a
// dial-guarded client honouring the config's allow_private_target
// (S121-F4 / F1-Inj: closes the DNS-rebinding TOCTOU window that
// ValidateOutboundURL alone leaves open).
func (c *GitLabCollector) clientFor(allowPrivate bool) *http.Client {
	if c.httpClient != nil {
		return c.httpClient
	}
	return httputil.GuardedClient(30*time.Second, allowPrivate)
}

// gitlabProject is a minimal GitLab project representation.
type gitlabProject struct {
	ID                int    `json:"id"`
	Name              string `json:"name"`
	PathWithNamespace string `json:"path_with_namespace"`
	Visibility        string `json:"visibility"`
	DefaultBranch     string `json:"default_branch"`
}

type gitlabProtectedBranch struct {
	Name             string `json:"name"`
	AllowForcePush   bool   `json:"allow_force_push"`
	PushAccessLevels []struct {
		AccessLevel int `json:"access_level"`
	} `json:"push_access_levels"`
}

type gitlabApprovalSettings struct {
	ApprovalsBeforeMerge int `json:"approvals_before_merge"`
}

type gitlabJob struct {
	Name   string `json:"name"`
	Status string `json:"status"`
}

type gitlabVulnFinding struct {
	ID       string `json:"id"`
	Severity string `json:"severity"`
	Name     string `json:"name"`
	Location struct {
		File string `json:"file"`
	} `json:"location"`
	Scanner struct {
		ExternalID string `json:"external_id"`
	} `json:"scanner"`
}

// Collect runs all GitLab evidence collectors for the given org and config.
func (c *GitLabCollector) Collect(ctx context.Context, orgID string, cfg GitLabConfig) (int, error) {
	client := c.clientFor(cfg.AllowPrivateTarget)

	projects, err := c.listProjects(ctx, client, cfg)
	if err != nil {
		return 0, fmt.Errorf("gitlab: list projects: %w", err)
	}

	sdlcControls, _ := c.evidence.FindControlsByKeywords(ctx, orgID, []string{"secure development", "sdlc", "code", "source"})
	changeControls, _ := c.evidence.FindControlsByKeywords(ctx, orgID, []string{"change management", "approval", "review"})
	assetControls, _ := c.evidence.FindControlsByKeywords(ctx, orgID, []string{"asset", "inventory", "software"})

	total := 0
	// F3/R-H20: accumulate sub-collector failures (see cloud collector comment).
	var errs []error

	unprotectedBranches := []string{}
	noApprovalProjects := []string{}
	sastProjects := []string{}

	for _, p := range projects {
		// Branch protection
		protected, err := c.collectBranchProtection(ctx, client, cfg, p)
		if err != nil {
			log.Warn().Err(err).Int("project_id", p.ID).Msg("gitlab_collector: branch protection failed")
			errs = append(errs, err)
		} else {
			if !protected {
				unprotectedBranches = append(unprotectedBranches, p.PathWithNamespace)
			}
		}

		// MR approvals
		hasApproval, err := c.collectMRApprovals(ctx, client, cfg, p)
		if err != nil {
			log.Warn().Err(err).Int("project_id", p.ID).Msg("gitlab_collector: mr approvals failed")
			errs = append(errs, err)
		} else {
			if !hasApproval {
				noApprovalProjects = append(noApprovalProjects, p.PathWithNamespace)
			}
		}

		// SAST presence
		hasSAST, err := c.collectSASTPresence(ctx, client, cfg, p)
		if err != nil {
			log.Warn().Err(err).Int("project_id", p.ID).Msg("gitlab_collector: sast presence failed")
			errs = append(errs, err)
		} else if hasSAST {
			sastProjects = append(sastProjects, p.PathWithNamespace)
		}

		// Vulnerability findings (GitLab EE/Ultimate only — 403 = skip)
		n, err := c.collectVulnerabilityFindings(ctx, client, orgID, cfg, p)
		if err != nil {
			log.Warn().Err(err).Int("project_id", p.ID).Msg("gitlab_collector: vuln findings failed")
			errs = append(errs, err)
		} else {
			total += n
		}
	}

	// Inventory evidence
	invDetails := map[string]any{
		"collected_at":  time.Now().UTC().Format(time.RFC3339),
		"project_count": len(projects),
		"gitlab_url":    cfg.GitLabURL,
	}
	if err := c.addEvidence(ctx, orgID, firstControlID(assetControls), "GitLab Projekt-Inventar", invDetails); err != nil {
		log.Warn().Err(err).Msg("gitlab_collector: write inventory evidence")
		errs = append(errs, err)
	} else {
		total++
	}

	// Unprotected branch warnings
	for _, name := range unprotectedBranches {
		details := map[string]any{
			"collected_at": time.Now().UTC().Format(time.RFC3339),
			"project":      name,
			"warning":      fmt.Sprintf("Projekt %s: Default Branch ungeschützt.", name),
		}
		if err := c.addEvidence(ctx, orgID, firstControlID(sdlcControls),
			fmt.Sprintf("GitLab Warnung: %s hat ungeschützten Default-Branch", name), details); err == nil {
			total++
		}
	}

	// No-approval warnings
	for _, name := range noApprovalProjects {
		details := map[string]any{
			"collected_at": time.Now().UTC().Format(time.RFC3339),
			"project":      name,
			"warning":      fmt.Sprintf("Projekt %s: MR ohne Approval-Pflicht.", name),
		}
		if err := c.addEvidence(ctx, orgID, firstControlID(changeControls),
			fmt.Sprintf("GitLab Warnung: %s ohne MR-Approval-Pflicht", name), details); err == nil {
			total++
		}
	}

	// SAST summary evidence
	if len(sastProjects) > 0 {
		sastDetails := map[string]any{
			"collected_at":  time.Now().UTC().Format(time.RFC3339),
			"sast_projects": sastProjects,
			"project_count": len(sastProjects),
		}
		if err := c.addEvidence(ctx, orgID, firstControlID(sdlcControls),
			fmt.Sprintf("GitLab SAST aktiv in %d Projekten", len(sastProjects)), sastDetails); err == nil {
			total++
		}
	}

	// Branch-protection summary evidence
	summaryDetails := map[string]any{
		"collected_at":             time.Now().UTC().Format(time.RFC3339),
		"total_projects":           len(projects),
		"unprotected_branch_count": len(unprotectedBranches),
		"no_approval_count":        len(noApprovalProjects),
		"sast_active_count":        len(sastProjects),
	}
	if err := c.addEvidence(ctx, orgID, firstControlID(sdlcControls),
		"GitLab Branch-Protection & Approval-Übersicht", summaryDetails); err == nil {
		total++
	}

	if total == 0 && len(errs) > 0 {
		return 0, errors.Join(errs...)
	}
	return total, nil
}

// listProjects returns all accessible projects for the configured group or membership.
func (c *GitLabCollector) listProjects(ctx context.Context, client *http.Client, cfg GitLabConfig) ([]gitlabProject, error) {
	var baseURL string
	if cfg.GroupID != "" {
		baseURL = fmt.Sprintf("%s/api/v4/groups/%s/projects?per_page=100&include_subgroups=true&order_by=id",
			strings.TrimRight(cfg.GitLabURL, "/"), url.PathEscape(cfg.GroupID))
	} else {
		baseURL = fmt.Sprintf("%s/api/v4/projects?membership=true&per_page=100&order_by=id",
			strings.TrimRight(cfg.GitLabURL, "/"))
	}

	raw, err := c.gitlabGetAll(ctx, client, baseURL, cfg.AccessToken)
	if err != nil {
		return nil, err
	}

	var projects []gitlabProject
	for _, item := range raw {
		var p gitlabProject
		if err := json.Unmarshal(item, &p); err == nil {
			projects = append(projects, p)
		}
	}
	return projects, nil
}

// collectBranchProtection returns true if the project's default branch is protected.
func (c *GitLabCollector) collectBranchProtection(ctx context.Context, client *http.Client, cfg GitLabConfig, p gitlabProject) (bool, error) {
	branch := p.DefaultBranch
	if branch == "" {
		branch = "main"
	}
	apiURL := fmt.Sprintf("%s/api/v4/projects/%d/protected_branches",
		strings.TrimRight(cfg.GitLabURL, "/"), p.ID)

	raw, err := c.gitlabGetAll(ctx, client, apiURL, cfg.AccessToken)
	if err != nil {
		return false, err
	}

	for _, item := range raw {
		var pb gitlabProtectedBranch
		if err := json.Unmarshal(item, &pb); err != nil {
			continue
		}
		if pb.Name == branch || pb.Name == "main" || pb.Name == "master" {
			if !pb.AllowForcePush {
				return true, nil
			}
		}
	}
	return false, nil
}

// collectMRApprovals returns true if the project requires at least one approver.
func (c *GitLabCollector) collectMRApprovals(ctx context.Context, client *http.Client, cfg GitLabConfig, p gitlabProject) (bool, error) {
	apiURL := fmt.Sprintf("%s/api/v4/projects/%d/approvals",
		strings.TrimRight(cfg.GitLabURL, "/"), p.ID)

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, apiURL, nil)
	if err != nil {
		return false, err
	}
	req.Header.Set("PRIVATE-TOKEN", cfg.AccessToken)

	resp, err := client.Do(req)
	if err != nil {
		return false, err
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusNotFound || resp.StatusCode == http.StatusForbidden {
		return true, nil // assume protected if we can't check
	}
	if resp.StatusCode != http.StatusOK {
		return false, fmt.Errorf("approvals API: status %d", resp.StatusCode)
	}

	body, _ := io.ReadAll(resp.Body)
	var settings gitlabApprovalSettings
	if err := json.Unmarshal(body, &settings); err != nil {
		return false, err
	}
	return settings.ApprovalsBeforeMerge >= 1, nil
}

// collectSASTPresence returns true if any recent successful job has "sast" in its name.
func (c *GitLabCollector) collectSASTPresence(ctx context.Context, client *http.Client, cfg GitLabConfig, p gitlabProject) (bool, error) {
	apiURL := fmt.Sprintf("%s/api/v4/projects/%d/jobs?scope=success&per_page=100",
		strings.TrimRight(cfg.GitLabURL, "/"), p.ID)

	raw, err := c.gitlabGetAll(ctx, client, apiURL, cfg.AccessToken)
	if err != nil {
		return false, err
	}

	for _, item := range raw {
		var job gitlabJob
		if err := json.Unmarshal(item, &job); err != nil {
			continue
		}
		if hasSASTJob(job.Name) {
			return true, nil
		}
	}
	return false, nil
}

// hasSASTJob checks if a job name indicates SAST scanning.
func hasSASTJob(name string) bool {
	lower := strings.ToLower(name)
	return strings.Contains(lower, "sast") ||
		strings.Contains(lower, "security_scan") ||
		strings.Contains(lower, "semgrep") ||
		strings.Contains(lower, "bandit") ||
		strings.Contains(lower, "gosec")
}

// collectVulnerabilityFindings imports GitLab EE/Ultimate vulnerability findings as vaktscan findings.
// Returns 0, nil if GitLab EE is not available (403).
func (c *GitLabCollector) collectVulnerabilityFindings(ctx context.Context, client *http.Client, orgID string, cfg GitLabConfig, p gitlabProject) (int, error) {
	apiURL := fmt.Sprintf("%s/api/v4/projects/%d/vulnerability_findings?state=detected&severity=critical,high&per_page=100",
		strings.TrimRight(cfg.GitLabURL, "/"), p.ID)

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, apiURL, nil)
	if err != nil {
		return 0, err
	}
	req.Header.Set("PRIVATE-TOKEN", cfg.AccessToken)

	resp, err := client.Do(req)
	if err != nil {
		return 0, err
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusForbidden || resp.StatusCode == http.StatusNotFound {
		// GitLab CE/EE without Ultimate — skip silently
		return 0, nil
	}
	if resp.StatusCode == http.StatusUnauthorized {
		return 0, fmt.Errorf("gitlab vulnerability_findings: unauthorized (401)")
	}
	if resp.StatusCode != http.StatusOK {
		return 0, fmt.Errorf("gitlab vulnerability_findings: status %d", resp.StatusCode)
	}

	body, _ := io.ReadAll(resp.Body)
	var findings []gitlabVulnFinding
	if err := json.Unmarshal(body, &findings); err != nil {
		return 0, err
	}
	if len(findings) == 0 {
		return 0, nil
	}

	assetID, err := resolveRepoAssetID(ctx, c.db, orgID, p.PathWithNamespace)
	if err != nil {
		return 0, fmt.Errorf("resolve asset for %s: %w", p.PathWithNamespace, err)
	}

	count := 0
	for _, f := range findings {
		rawID := fmt.Sprintf("gitlab:%d:%s", p.ID, f.ID)
		severity := strings.ToLower(f.Severity)
		if severity != "critical" && severity != "high" {
			continue
		}

		title := f.Name
		if title == "" {
			title = fmt.Sprintf("GitLab Security Finding (project %d)", p.ID)
		}

		_, err := c.db.Exec(ctx, `
			INSERT INTO vb_findings
				(org_id, asset_id, title, description, severity, status, scanner, raw_id, sources, last_seen_at)
			VALUES
				($1::uuid, $2::uuid, $3, $4, $5, 'open', 'gitlab', $6, ARRAY['gitlab'], NOW())
			ON CONFLICT (org_id, raw_id, scanner) WHERE raw_id IS NOT NULL
			DO UPDATE SET
				last_seen_at = NOW(),
				severity = EXCLUDED.severity`,
			orgID, assetID, title,
			fmt.Sprintf("GitLab: %s in %s/%s", f.Name, p.PathWithNamespace, f.Location.File),
			severity, dedupKey(rawID),
		)
		if err != nil {
			log.Warn().Err(err).Str("raw_id", rawID).Msg("gitlab_collector: upsert finding")
			continue
		}
		count++
	}
	return count, nil
}

// gitlabGetAll fetches all pages from a GitLab REST API endpoint (Link-header pagination).
func (c *GitLabCollector) gitlabGetAll(ctx context.Context, client *http.Client, startURL, token string) ([]json.RawMessage, error) {
	var all []json.RawMessage
	nextURL := startURL

	for nextURL != "" {
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, nextURL, nil)
		if err != nil {
			return nil, err
		}
		req.Header.Set("PRIVATE-TOKEN", token)

		resp, err := client.Do(req)
		if err != nil {
			return nil, err
		}

		if resp.StatusCode == http.StatusUnauthorized {
			_ = resp.Body.Close()
			return nil, fmt.Errorf("gitlab API: unauthorized (401) — token ungültig")
		}
		if resp.StatusCode != http.StatusOK {
			_ = resp.Body.Close()
			return nil, fmt.Errorf("gitlab API: status %d for %s", resp.StatusCode, nextURL)
		}

		body, err := io.ReadAll(resp.Body)
		_ = resp.Body.Close()
		if err != nil {
			return nil, err
		}

		var page []json.RawMessage
		if err := json.Unmarshal(body, &page); err != nil {
			return nil, fmt.Errorf("parse gitlab response: %w", err)
		}
		all = append(all, page...)

		nextURL = parseLinkNext(resp.Header.Get("Link"))
		if nextURL == "" {
			// Also check X-Next-Page header
			if next := resp.Header.Get("X-Next-Page"); next != "" {
				// Reconstruct URL with page param, stripping any existing query string first.
				base := startURL
				if idx := strings.Index(base, "?"); idx != -1 {
					base = base[:idx]
				}
				u, parseErr := url.Parse(base)
				if parseErr == nil {
					q := u.Query()
					q.Set("page", next)
					u.RawQuery = q.Encode()
					nextURL = u.String()
				}
			}
		}
	}
	return all, nil
}

// parseLinkNext extracts the "next" URL from a Link header value.
// e.g. `<https://gitlab.com/api/v4/...?page=2>; rel="next", <...>; rel="last"`
func parseLinkNext(link string) string {
	if link == "" {
		return ""
	}
	for _, part := range strings.Split(link, ",") {
		part = strings.TrimSpace(part)
		segments := strings.Split(part, ";")
		if len(segments) < 2 {
			continue
		}
		urlPart := strings.TrimSpace(segments[0])
		relPart := strings.TrimSpace(segments[1])
		if strings.Contains(relPart, `rel="next"`) {
			urlPart = strings.TrimPrefix(urlPart, "<")
			urlPart = strings.TrimSuffix(urlPart, ">")
			return urlPart
		}
	}
	return ""
}

func (c *GitLabCollector) addEvidence(ctx context.Context, orgID, controlID, title string, details map[string]any) error {
	if controlID == "" {
		log.Debug().Str("org_id", orgID).Str("title", title).Msg("gitlab_collector: no matching control, skipping")
		return nil
	}
	data, _ := json.Marshal(details)
	return c.evidence.AddCollectorEvidence(ctx, orgID, controlID, "", gitlabSource, title, data)
}

// CountUnprotectedBranches returns the number of projects without branch protection (used by status).
func (c *GitLabCollector) CountUnprotectedBranches(ctx context.Context, cfg GitLabConfig) (projectCount, unprotectedCount int, err error) {
	client := c.clientFor(cfg.AllowPrivateTarget)

	projects, err := c.listProjects(ctx, client, cfg)
	if err != nil {
		return 0, 0, err
	}
	for _, p := range projects {
		protected, _ := c.collectBranchProtection(ctx, client, cfg, p)
		if !protected {
			unprotectedCount++
		}
	}
	return len(projects), unprotectedCount, nil
}

// resolveRepoAssetID finds or creates the vb_assets row that cloud-sourced
// (GitLab/SonarQube) findings attach to, and returns its UUID.
//
// vb_findings.asset_id is a NOT NULL UUID foreign key into vb_assets — these
// collectors used to insert the literal ” for it. Postgres coerces VALUES()
// literals to the target column's type, so ” was parsed as ::uuid and every
// single insert failed with 22P02 ("invalid input syntax for type uuid").
// Cloud findings never landed in the database, silently: BatchUpsertFindings-
// style callers here just log.Warn and continue past the error (see
// collectVulnerabilityFindings/collectSecurityHotspots/collectVulnerabilities
// below), so a scan reported success with zero visible symptoms.
//
// One vb_assets row per GitLab project / SonarQube project (type 'repo')
// keeps findings correctly scoped and filterable by asset, instead of
// inventing a single org-wide placeholder. vb_assets has no unique
// constraint on (org_id, name), so this is a plain select-then-insert rather
// than an INSERT ... ON CONFLICT — a rare concurrent double-run can create a
// duplicate asset row, which is a cosmetic annoyance, not a correctness bug
// (findings dedupe on their own raw_id key regardless of which of the two
// duplicate assets they land on).
func resolveRepoAssetID(ctx context.Context, db *pgxpool.Pool, orgID, name string) (string, error) {
	var id string
	err := db.QueryRow(ctx,
		`SELECT id::text FROM vb_assets WHERE org_id = $1::uuid AND name = $2 AND is_deleted = FALSE LIMIT 1`,
		orgID, name,
	).Scan(&id)
	if err == nil {
		return id, nil
	}
	if !errors.Is(err, pgx.ErrNoRows) {
		return "", err
	}
	err = db.QueryRow(ctx, `
		INSERT INTO vb_assets (org_id, name, type)
		VALUES ($1::uuid, $2, 'repo')
		RETURNING id::text`,
		orgID, name,
	).Scan(&id)
	if err != nil {
		return "", err
	}
	return id, nil
}

// dedupKey turns an optional text value into what vb_findings expects: NULL
// when it is absent. vb_findings has partial unique indexes on raw_id (and
// cve_id/template_id) WHERE ... IS NOT NULL (migration 120) — an empty
// string is NOT NULL and collides with every other empty-string row under
// the same dedup key. Local copy of the same pattern documented in
// internal/modules/vaktscan/dedup_keys.go; not reused directly because
// package cloud must not import a vaktscan-module package.
func dedupKey(s string) *string {
	if s == "" {
		return nil
	}
	return &s
}
