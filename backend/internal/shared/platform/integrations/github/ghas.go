// Copyright (c) 2026 NorvikOps. All rights reserved.
// SPDX-License-Identifier: Elastic-2.0

package github

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
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

// ListDependabotAlerts fetches open Dependabot alerts for a repository.
// Returns nil, nil if GHAS is not enabled for the repo (403).
func (c *Client) ListDependabotAlerts(ctx context.Context, owner, repo string) ([]DependabotAlert, error) {
	url := fmt.Sprintf("https://api.github.com/repos/%s/%s/dependabot/alerts?state=open&per_page=100", owner, repo)
	resp, err := c.doGitHubRequest(ctx, url)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusForbidden || resp.StatusCode == http.StatusNotFound {
		// GHAS not enabled or no access — skip silently
		return nil, nil
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("dependabot alerts: github api returned %d", resp.StatusCode)
	}

	raw, err := io.ReadAll(io.LimitReader(resp.Body, 4*1024*1024))
	if err != nil {
		return nil, fmt.Errorf("read dependabot response: %w", err)
	}

	var items []struct {
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
	if err := json.Unmarshal(raw, &items); err != nil {
		return nil, fmt.Errorf("parse dependabot alerts: %w", err)
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

// ListSecretScanningAlerts fetches open secret scanning alerts for a repository.
// Returns nil, nil if GHAS is not enabled for the repo (403).
func (c *Client) ListSecretScanningAlerts(ctx context.Context, owner, repo string) ([]SecretScanningAlert, error) {
	url := fmt.Sprintf("https://api.github.com/repos/%s/%s/secret-scanning/alerts?state=open&per_page=100", owner, repo)
	resp, err := c.doGitHubRequest(ctx, url)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusForbidden || resp.StatusCode == http.StatusNotFound {
		return nil, nil
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("secret scanning: github api returned %d", resp.StatusCode)
	}

	raw, err := io.ReadAll(io.LimitReader(resp.Body, 4*1024*1024))
	if err != nil {
		return nil, fmt.Errorf("read secret scanning response: %w", err)
	}

	var items []struct {
		Number     int    `json:"number"`
		State      string `json:"state"`
		SecretType string `json:"secret_type"`
	}
	if err := json.Unmarshal(raw, &items); err != nil {
		return nil, fmt.Errorf("parse secret scanning alerts: %w", err)
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

// ListCodeScanningAlerts fetches open high+critical code scanning alerts.
// Returns nil, nil if GHAS is not enabled for the repo (403).
func (c *Client) ListCodeScanningAlerts(ctx context.Context, owner, repo string) ([]CodeScanningAlert, error) {
	url := fmt.Sprintf("https://api.github.com/repos/%s/%s/code-scanning/alerts?state=open&per_page=100", owner, repo)
	resp, err := c.doGitHubRequest(ctx, url)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusForbidden || resp.StatusCode == http.StatusNotFound {
		return nil, nil
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("code scanning: github api returned %d", resp.StatusCode)
	}

	raw, err := io.ReadAll(io.LimitReader(resp.Body, 4*1024*1024))
	if err != nil {
		return nil, fmt.Errorf("read code scanning response: %w", err)
	}

	var items []struct {
		Number int    `json:"number"`
		State  string `json:"state"`
		Rule   struct {
			ID              string `json:"id"`
			SecuritySeverity string `json:"security_severity_level"`
			Severity        string `json:"severity"`
		} `json:"rule"`
		Tool struct {
			Name string `json:"name"`
		} `json:"tool"`
	}
	if err := json.Unmarshal(raw, &items); err != nil {
		return nil, fmt.Errorf("parse code scanning alerts: %w", err)
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
