// Copyright (c) 2026 NorvikOps. All rights reserved.
// SPDX-License-Identifier: Elastic-2.0

package cloud

import (
	"context"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/rs/zerolog/log"
)

const wazuhSource = "wazuh-collector"

// WazuhPullCollector collects compliance evidence from a Wazuh manager via REST API.
type WazuhPullCollector struct {
	db       *pgxpool.Pool
	evidence EvidenceWriter
}

// NewWazuhPullCollector creates a new WazuhPullCollector.
func NewWazuhPullCollector(db *pgxpool.Pool, evidence EvidenceWriter) *WazuhPullCollector {
	return &WazuhPullCollector{db: db, evidence: evidence}
}

// Internal Wazuh API response types.
type wazuhAgent struct {
	ID            string `json:"id"`
	Name          string `json:"name"`
	IP            string `json:"ip"`
	Status        string `json:"status"`
	LastKeepAlive string `json:"lastKeepAlive"`
	OS            struct {
		Name    string `json:"name"`
		Version string `json:"version"`
	} `json:"os"`
}

type wazuhVulnSummary struct {
	Critical int `json:"Critical"`
	High     int `json:"High"`
	Medium   int `json:"Medium"`
	Low      int `json:"Low"`
}

type wazuhSCAPolicy struct {
	Name      string  `json:"name"`
	PassCount int     `json:"pass"`
	FailCount int     `json:"fail"`
	Score     float64 `json:"score"`
}

// Collect runs all Wazuh evidence collectors for the given org and config.
func (c *WazuhPullCollector) Collect(ctx context.Context, orgID string, cfg WazuhConfig) (int, error) {
	token, err := c.authenticate(ctx, cfg)
	if err != nil {
		return 0, fmt.Errorf("wazuh auth: %w", err)
	}

	httpClient := c.newHTTPClient(cfg)

	agents, err := c.fetchAgents(ctx, httpClient, cfg.BaseURL, token)
	if err != nil {
		return 0, fmt.Errorf("wazuh: fetch agents: %w", err)
	}

	inventoryControls, _ := c.evidence.FindControlsByKeywords(ctx, orgID, []string{"asset", "inventory", "endpoint"})
	vulnControls, _ := c.evidence.FindControlsByKeywords(ctx, orgID, []string{"vulnerability", "patch", "cve"})
	configControls, _ := c.evidence.FindControlsByKeywords(ctx, orgID, []string{"configuration", "hardening", "baseline"})
	monitorControls, _ := c.evidence.FindControlsByKeywords(ctx, orgID, []string{"monitoring", "logging", "fim", "integrity"})

	total := 0

	// Asset inventory evidence (one per agent)
	if n, err := c.collectAgentInventory(ctx, orgID, agents, inventoryControls); err != nil {
		log.Warn().Err(err).Str("org_id", orgID).Msg("wazuh_collector: agent inventory failed")
	} else {
		total += n
	}

	// Per-agent vulnerability + SCA + FIM evidence
	for _, agent := range agents {
		if agent.Status != "active" {
			continue
		}
		if n, err := c.collectVulnerabilities(ctx, httpClient, cfg.BaseURL, token, orgID, agent, vulnControls); err != nil {
			log.Warn().Err(err).Str("agent", agent.Name).Msg("wazuh_collector: vuln collection failed")
		} else {
			total += n
		}

		if n, err := c.collectSCA(ctx, httpClient, cfg.BaseURL, token, orgID, agent, configControls); err != nil {
			log.Warn().Err(err).Str("agent", agent.Name).Msg("wazuh_collector: sca collection failed")
		} else {
			total += n
		}

		if n, err := c.collectFIM(ctx, httpClient, cfg.BaseURL, token, orgID, agent, monitorControls); err != nil {
			log.Warn().Err(err).Str("agent", agent.Name).Msg("wazuh_collector: fim collection failed")
		} else {
			total += n
		}
	}

	return total, nil
}

// authenticate fetches a JWT token from the Wazuh API.
func (c *WazuhPullCollector) authenticate(ctx context.Context, cfg WazuhConfig) (string, error) {
	client := c.newHTTPClient(cfg)
	url := cfg.BaseURL + "/security/user/authenticate"

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, nil)
	if err != nil {
		return "", fmt.Errorf("build auth request: %w", err)
	}
	req.SetBasicAuth(cfg.Username, cfg.Password)
	req.Header.Set("Content-Type", "application/json")

	resp, err := client.Do(req)
	if err != nil {
		return "", fmt.Errorf("auth request: %w", err)
	}
	defer resp.Body.Close()

	raw, _ := io.ReadAll(io.LimitReader(resp.Body, 64*1024))
	if resp.StatusCode == http.StatusUnauthorized {
		return "", fmt.Errorf("authentication failed (401): invalid credentials")
	}
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("auth endpoint returned %d", resp.StatusCode)
	}

	var authResp struct {
		Data struct {
			Token string `json:"token"`
		} `json:"data"`
	}
	if err := json.Unmarshal(raw, &authResp); err != nil {
		return "", fmt.Errorf("parse auth response: %w", err)
	}
	if authResp.Data.Token == "" {
		return "", fmt.Errorf("empty token in auth response")
	}
	return authResp.Data.Token, nil
}

func (c *WazuhPullCollector) newHTTPClient(cfg WazuhConfig) *http.Client {
	transport := http.DefaultTransport
	if !cfg.VerifyTLS {
		transport = &http.Transport{
			TLSClientConfig: &tls.Config{InsecureSkipVerify: true}, //nolint:gosec // intentional for self-signed on-prem
		}
	}
	return &http.Client{
		Timeout:   30 * time.Second,
		Transport: transport,
	}
}

func (c *WazuhPullCollector) doGet(ctx context.Context, client *http.Client, baseURL, token, path string) (map[string]any, error) {
	url := baseURL + path
	if !strings.HasPrefix(path, "/") {
		url = baseURL + "/" + path
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, fmt.Errorf("build request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Accept", "application/json")

	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("get %s: %w", path, err)
	}
	defer resp.Body.Close()

	raw, err := io.ReadAll(io.LimitReader(resp.Body, 4*1024*1024))
	if err != nil {
		return nil, fmt.Errorf("read response: %w", err)
	}

	if resp.StatusCode == http.StatusUnauthorized {
		return nil, fmt.Errorf("token expired (401)")
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("wazuh api %s returned %d", path, resp.StatusCode)
	}

	var result map[string]any
	if err := json.Unmarshal(raw, &result); err != nil {
		return nil, fmt.Errorf("parse response: %w", err)
	}
	return result, nil
}

func (c *WazuhPullCollector) fetchAgents(ctx context.Context, client *http.Client, baseURL, token string) ([]wazuhAgent, error) {
	result, err := c.doGet(ctx, client, baseURL, token, "/agents")
	if err != nil {
		return nil, err
	}

	var agents []wazuhAgent
	if data, ok := result["data"].(map[string]any); ok {
		if items, ok := data["affected_items"].([]any); ok {
			for _, item := range items {
				raw, _ := json.Marshal(item)
				var a wazuhAgent
				if json.Unmarshal(raw, &a) == nil {
					agents = append(agents, a)
				}
			}
		}
	}
	return agents, nil
}

func (c *WazuhPullCollector) collectAgentInventory(ctx context.Context, orgID string, agents []wazuhAgent, controls []ControlMatch) (int, error) {
	now := time.Now().UTC()
	cutoff24h := now.Add(-24 * time.Hour)

	summaries := make([]map[string]any, 0, len(agents))
	offlineCount := 0

	for _, a := range agents {
		entry := map[string]any{
			"id":     a.ID,
			"name":   a.Name,
			"ip":     a.IP,
			"status": a.Status,
		}
		if a.OS.Name != "" {
			entry["os"] = a.OS.Name + " " + a.OS.Version
		}

		// Check if agent is offline > 24h
		if a.Status != "active" {
			if t, err := time.Parse(time.RFC3339, a.LastKeepAlive); err == nil {
				if t.Before(cutoff24h) {
					offlineCount++
					entry["offline_duration"] = now.Sub(t).String()
				}
			} else {
				offlineCount++
			}
		}
		summaries = append(summaries, entry)
	}

	details := map[string]any{
		"collected_at":   now.Format(time.RFC3339),
		"agent_count":    len(agents),
		"agents_offline": offlineCount,
		"agents":         summaries,
	}

	controlID := firstControlID(controls)
	if err := c.addEvidence(ctx, orgID, controlID, "Wazuh Agent-Übersicht", details); err != nil {
		return 0, err
	}
	return 1, nil
}

func (c *WazuhPullCollector) collectVulnerabilities(ctx context.Context, client *http.Client, baseURL, token, orgID string, agent wazuhAgent, controls []ControlMatch) (int, error) {
	result, err := c.doGet(ctx, client, baseURL, token, "/vulnerability/"+agent.ID+"/summary")
	if err != nil {
		return 0, err
	}

	summary := wazuhVulnSummary{}
	if data, ok := result["data"].(map[string]any); ok {
		raw, _ := json.Marshal(data)
		_ = json.Unmarshal(raw, &summary)
	}

	details := map[string]any{
		"collected_at": time.Now().UTC().Format(time.RFC3339),
		"agent_name":   agent.Name,
		"agent_id":     agent.ID,
		"critical":     summary.Critical,
		"high":         summary.High,
		"medium":       summary.Medium,
		"low":          summary.Low,
	}

	title := fmt.Sprintf("Wazuh Vulnerability-Summary: %s", agent.Name)
	if summary.Critical > 0 {
		details["warning"] = fmt.Sprintf("Agent %s: %d kritische CVEs offen.", agent.Name, summary.Critical)
	}

	controlID := firstControlID(controls)
	if err := c.addEvidence(ctx, orgID, controlID, title, details); err != nil {
		return 0, err
	}
	return 1, nil
}

func (c *WazuhPullCollector) collectSCA(ctx context.Context, client *http.Client, baseURL, token, orgID string, agent wazuhAgent, controls []ControlMatch) (int, error) {
	result, err := c.doGet(ctx, client, baseURL, token, "/sca/"+agent.ID)
	if err != nil {
		return 0, err
	}

	policies := []wazuhSCAPolicy{}
	if data, ok := result["data"].(map[string]any); ok {
		if items, ok := data["affected_items"].([]any); ok {
			for _, item := range items {
				raw, _ := json.Marshal(item)
				var p wazuhSCAPolicy
				if json.Unmarshal(raw, &p) == nil {
					policies = append(policies, p)
				}
			}
		}
	}

	policySummaries := make([]map[string]any, 0, len(policies))
	for _, p := range policies {
		policySummaries = append(policySummaries, map[string]any{
			"name":       p.Name,
			"pass_count": p.PassCount,
			"fail_count": p.FailCount,
			"score":      p.Score,
		})
	}

	details := map[string]any{
		"collected_at":    time.Now().UTC().Format(time.RFC3339),
		"agent_name":      agent.Name,
		"agent_id":        agent.ID,
		"policy_count":    len(policies),
		"policies":        policySummaries,
	}

	controlID := firstControlID(controls)
	if err := c.addEvidence(ctx, orgID, controlID, fmt.Sprintf("Wazuh SCA-Score: %s", agent.Name), details); err != nil {
		return 0, err
	}
	return 1, nil
}

func (c *WazuhPullCollector) collectFIM(ctx context.Context, client *http.Client, baseURL, token, orgID string, agent wazuhAgent, controls []ControlMatch) (int, error) {
	result, err := c.doGet(ctx, client, baseURL, token, "/syscheck/"+agent.ID)
	if err != nil {
		return 0, err
	}

	var changeCount int
	var lastScan string
	cutoff24h := time.Now().Add(-24 * time.Hour)

	if data, ok := result["data"].(map[string]any); ok {
		if total, ok := data["total_affected_items"].(float64); ok {
			changeCount = int(total)
		}
		if items, ok := data["affected_items"].([]any); ok {
			for _, item := range items {
				if m, ok := item.(map[string]any); ok {
					if mtime, ok := m["mtime"].(string); ok {
						if t, err := time.Parse(time.RFC3339, mtime); err == nil && t.After(cutoff24h) {
							lastScan = mtime
						}
					}
				}
			}
		}
	}

	details := map[string]any{
		"collected_at":     time.Now().UTC().Format(time.RFC3339),
		"agent_name":       agent.Name,
		"agent_id":         agent.ID,
		"changes_detected": changeCount,
		"last_scan":        lastScan,
	}

	if changeCount > 0 {
		details["summary"] = fmt.Sprintf("File Integrity Monitoring aktiv: %d Änderungen erkannt.", changeCount)
	}

	controlID := firstControlID(controls)
	if err := c.addEvidence(ctx, orgID, controlID, fmt.Sprintf("Wazuh FIM: %s", agent.Name), details); err != nil {
		return 0, err
	}
	return 1, nil
}

func (c *WazuhPullCollector) addEvidence(ctx context.Context, orgID, controlID, title string, details map[string]any) error {
	data, _ := json.Marshal(details)
	if controlID == "" {
		log.Debug().Str("org_id", orgID).Str("title", title).Msg("wazuh_collector: no matching control, skipping")
		return nil
	}
	return c.evidence.AddCollectorEvidence(ctx, orgID, controlID, "", wazuhSource, title, data)
}

// CountAgents returns online/offline counts for status endpoint.
func (c *WazuhPullCollector) CountAgents(ctx context.Context, cfg WazuhConfig) (total, offline int, err error) {
	token, err := c.authenticate(ctx, cfg)
	if err != nil {
		return 0, 0, err
	}
	client := c.newHTTPClient(cfg)
	agents, err := c.fetchAgents(ctx, client, cfg.BaseURL, token)
	if err != nil {
		return 0, 0, err
	}
	now := time.Now().UTC()
	cutoff := now.Add(-24 * time.Hour)
	for _, a := range agents {
		total++
		if a.Status != "active" {
			if t, parseErr := time.Parse(time.RFC3339, a.LastKeepAlive); parseErr == nil && t.Before(cutoff) {
				offline++
			} else if parseErr != nil {
				offline++
			}
		}
	}
	return total, offline, nil
}
