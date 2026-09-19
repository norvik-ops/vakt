// Copyright (c) 2026 NorvikOps. All rights reserved.
// SPDX-License-Identifier: Elastic-2.0

package github

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
)

// DependabotAlert represents a Dependabot vulnerability alert from GitHub.
type DependabotAlert struct {
	Number   int    `json:"number"`
	State    string `json:"state"` // "open" | "dismissed" | "fixed"
	Severity string `json:"severity"`
	CVEIDs   []string
	Summary  string
	Package  string
	Repo     string
}

// SecretScanningAlert represents a GitHub secret scanning alert.
type SecretScanningAlert struct {
	Number     int    `json:"number"`
	State      string `json:"state"` // "open" | "resolved"
	SecretType string `json:"secret_type"`
	Repo       string
}

// CodeScanningAlert represents a GitHub code scanning alert.
type CodeScanningAlert struct {
	Number   int    `json:"number"`
	State    string `json:"state"`
	Severity string `json:"severity"`
	RuleID   string
	Tool     string
	Repo     string
}

// R1-W0A-V1: every GHAS listing (dependabot / secret-scanning / code-scanning)
// used per_page=100 and made exactly ONE request, so a repo with >100 open
// alerts was silently truncated and the compliance evidence understated the risk.
// fetchGHASList follows the Link: rel="next" header to collect every page. The
// pagination technique mirrors parseLinkNext in
// internal/shared/platform/integrations/cloud/gitlab_collector.go — copied here
// (not imported) because it is unexported in that package.

// parseGHASLinkNext extracts the "next" URL from a GitHub Link header value, e.g.
// `<https://api.github.com/…?page=2>; rel="next", <…>; rel="last"`. Returns "" when
// there is no next page.
func parseGHASLinkNext(link string) string {
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
		for _, seg := range segments[1:] {
			if strings.Contains(seg, `rel="next"`) {
				urlPart = strings.TrimPrefix(urlPart, "<")
				urlPart = strings.TrimSuffix(urlPart, ">")
				return urlPart
			}
		}
	}
	return ""
}

// fetchGHASList fetches every page of a GHAS alert listing into T, following the
// Link: rel="next" header. It returns whatever has been collected so far (nil on
// the very first page) with a nil error on 403/404 — GHAS not enabled or no
// access — preserving the historical "skip silently" behaviour.
func fetchGHASList[T any](ctx context.Context, c *Client, firstURL, what string) ([]T, error) {
	var all []T
	next := firstURL
	for next != "" {
		resp, err := c.doGitHubRequest(ctx, next)
		if err != nil {
			return nil, err
		}

		if resp.StatusCode == http.StatusForbidden || resp.StatusCode == http.StatusNotFound {
			_ = resp.Body.Close()
			return all, nil
		}
		if resp.StatusCode != http.StatusOK {
			_ = resp.Body.Close()
			return nil, fmt.Errorf("%s: github api returned %d", what, resp.StatusCode)
		}

		raw, readErr := io.ReadAll(io.LimitReader(resp.Body, 4*1024*1024))
		linkHeader := resp.Header.Get("Link")
		_ = resp.Body.Close()
		if readErr != nil {
			return nil, fmt.Errorf("read %s response: %w", what, readErr)
		}

		var page []T
		if err := json.Unmarshal(raw, &page); err != nil {
			return nil, fmt.Errorf("parse %s: %w", what, err)
		}
		all = append(all, page...)
		next = parseGHASLinkNext(linkHeader)
	}
	return all, nil
}

// rawDependabotItem is one element of the Dependabot alerts JSON array.
type rawDependabotItem struct {
	Number    int    `json:"number"`
	State     string `json:"state"`
	DependsOn struct {
		Package struct {
			Name string `json:"name"`
		} `json:"package"`
		ManifestPath string `json:"manifest_path"`
	} `json:"dependency"`
	SecurityAdvisory struct {
		Summary string `json:"summary"`
		CVEIDs  []struct {
			Value string `json:"value"`
		} `json:"identifiers"`
		Severity string `json:"severity"`
	} `json:"security_advisory"`
	SecurityVulnerability struct {
		Severity string `json:"severity"`
	} `json:"security_vulnerability"`
}

// ListDependabotAlerts fetches all open Dependabot alerts for a repository across
// every page. Returns nil, nil if GHAS is not enabled for the repo (403).
func (c *Client) ListDependabotAlerts(ctx context.Context, owner, repo string) ([]DependabotAlert, error) {
	url := fmt.Sprintf("https://api.github.com/repos/%s/%s/dependabot/alerts?state=open&per_page=100", owner, repo)
	items, err := fetchGHASList[rawDependabotItem](ctx, c, url, "dependabot alerts")
	if err != nil {
		return nil, err
	}
	if len(items) == 0 {
		// GHAS disabled (403/404) or genuinely no open alerts — return nil, matching
		// the pre-pagination contract callers rely on.
		return nil, nil
	}

	alerts := make([]DependabotAlert, 0, len(items))
	for _, item := range items {
		a := DependabotAlert{
			Number:  item.Number,
			State:   item.State,
			Package: item.DependsOn.Package.Name,
			Summary: item.SecurityAdvisory.Summary,
			Repo:    owner + "/" + repo,
		}
		// Prefer vulnerability-level severity, fall back to advisory-level
		a.Severity = item.SecurityVulnerability.Severity
		if a.Severity == "" {
			a.Severity = item.SecurityAdvisory.Severity
		}
		for _, id := range item.SecurityAdvisory.CVEIDs {
			if id.Value != "" {
				a.CVEIDs = append(a.CVEIDs, id.Value)
			}
		}
		alerts = append(alerts, a)
	}
	return alerts, nil
}

// rawSecretScanningItem is one element of the secret-scanning alerts JSON array.
type rawSecretScanningItem struct {
	Number     int    `json:"number"`
	State      string `json:"state"`
	SecretType string `json:"secret_type"`
}

// ListSecretScanningAlerts fetches all open secret scanning alerts for a
// repository across every page. Returns nil, nil if GHAS is not enabled (403).
func (c *Client) ListSecretScanningAlerts(ctx context.Context, owner, repo string) ([]SecretScanningAlert, error) {
	url := fmt.Sprintf("https://api.github.com/repos/%s/%s/secret-scanning/alerts?state=open&per_page=100", owner, repo)
	items, err := fetchGHASList[rawSecretScanningItem](ctx, c, url, "secret scanning alerts")
	if err != nil {
		return nil, err
	}
	if len(items) == 0 {
		return nil, nil
	}

	alerts := make([]SecretScanningAlert, 0, len(items))
	for _, item := range items {
		alerts = append(alerts, SecretScanningAlert{
			Number:     item.Number,
			State:      item.State,
			SecretType: item.SecretType,
			Repo:       owner + "/" + repo,
		})
	}
	return alerts, nil
}

// rawCodeScanningItem is one element of the code-scanning alerts JSON array.
type rawCodeScanningItem struct {
	Number int    `json:"number"`
	State  string `json:"state"`
	Rule   struct {
		ID               string `json:"id"`
		SecuritySeverity string `json:"security_severity_level"`
		Severity         string `json:"severity"`
	} `json:"rule"`
	Tool struct {
		Name string `json:"name"`
	} `json:"tool"`
}

// ListCodeScanningAlerts fetches all open high+critical code scanning alerts
// across every page. Returns nil, nil if GHAS is not enabled (403).
func (c *Client) ListCodeScanningAlerts(ctx context.Context, owner, repo string) ([]CodeScanningAlert, error) {
	url := fmt.Sprintf("https://api.github.com/repos/%s/%s/code-scanning/alerts?state=open&per_page=100", owner, repo)
	items, err := fetchGHASList[rawCodeScanningItem](ctx, c, url, "code scanning alerts")
	if err != nil {
		return nil, err
	}
	if len(items) == 0 {
		return nil, nil
	}

	alerts := make([]CodeScanningAlert, 0, len(items))
	for _, item := range items {
		severity := item.Rule.SecuritySeverity
		if severity == "" {
			severity = item.Rule.Severity
		}
		// Only import high + critical
		if severity != "high" && severity != "critical" {
			continue
		}
		alerts = append(alerts, CodeScanningAlert{
			Number:   item.Number,
			State:    item.State,
			Severity: severity,
			RuleID:   item.Rule.ID,
			Tool:     item.Tool.Name,
			Repo:     owner + "/" + repo,
		})
	}
	return alerts, nil
}
