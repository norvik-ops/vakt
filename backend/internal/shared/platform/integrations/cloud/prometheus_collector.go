// Copyright (c) 2026 NorvikOps. All rights reserved.
// SPDX-License-Identifier: Elastic-2.0

package cloud

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/matharnica/vakt/internal/shared/httputil"
	"github.com/rs/zerolog/log"
)

const prometheusSource = "prometheus-collector"

// PrometheusCollector collects compliance evidence from Prometheus and Alertmanager.
type PrometheusCollector struct {
	db         *pgxpool.Pool
	evidence   EvidenceWriter
	httpClient *http.Client // injectable for tests; nil in production (see clientFor)
}

// NewPrometheusCollector creates a new PrometheusCollector.
func NewPrometheusCollector(db *pgxpool.Pool, evidence EvidenceWriter) *PrometheusCollector {
	return &PrometheusCollector{
		db:       db,
		evidence: evidence,
	}
}

// clientFor returns the client for this run. Tests inject c.httpClient
// directly and it is used as-is; in production it is nil and we build a
// dial-guarded client honouring the config's allow_private_target
// (S121-F4 / F1-Inj: closes the DNS-rebinding TOCTOU window that
// ValidateOutboundURL alone leaves open).
func (c *PrometheusCollector) clientFor(allowPrivate bool) *http.Client {
	if c.httpClient != nil {
		return c.httpClient
	}
	return httputil.GuardedClient(30*time.Second, allowPrivate)
}

// Collect runs all Prometheus evidence collectors for the given org and config.
func (c *PrometheusCollector) Collect(ctx context.Context, orgID string, cfg PrometheusConfig) (int, error) {
	client := c.clientFor(cfg.AllowPrivateTarget)

	availabilityControls, _ := c.evidence.FindControlsByKeywords(ctx, orgID, []string{"availability", "capacity", "uptime"})
	monitoringControls, _ := c.evidence.FindControlsByKeywords(ctx, orgID, []string{"monitoring", "alerting", "observability"})

	total := 0

	if n, err := c.collectUptime(ctx, client, orgID, cfg, availabilityControls); err != nil {
		log.Warn().Err(err).Str("org_id", orgID).Msg("prometheus_collector: uptime collection failed")
	} else {
		total += n
	}

	if n, err := c.collectTargetHealth(ctx, client, orgID, cfg, monitoringControls); err != nil {
		log.Warn().Err(err).Str("org_id", orgID).Msg("prometheus_collector: target health failed")
	} else {
		total += n
	}

	if cfg.AlertmanagerURL != "" {
		if n, err := c.collectAlerts(ctx, client, orgID, cfg, monitoringControls); err != nil {
			log.Warn().Err(err).Str("org_id", orgID).Msg("prometheus_collector: alerts collection failed")
		} else {
			total += n
		}

		if n, err := c.collectAlertmanagerStatus(ctx, client, orgID, cfg, monitoringControls); err != nil {
			log.Warn().Err(err).Str("org_id", orgID).Msg("prometheus_collector: alertmanager status failed")
		} else {
			total += n
		}
	}

	return total, nil
}

func (c *PrometheusCollector) fetchRaw(ctx context.Context, client *http.Client, token, rawURL string) ([]byte, int, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, rawURL, nil)
	if err != nil {
		return nil, 0, fmt.Errorf("build request: %w", err)
	}
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	req.Header.Set("Accept", "application/json")

	resp, err := client.Do(req)
	if err != nil {
		return nil, 0, fmt.Errorf("get %s: %w", rawURL, err)
	}
	defer resp.Body.Close()

	raw, err := io.ReadAll(io.LimitReader(resp.Body, 4*1024*1024))
	if err != nil {
		return nil, resp.StatusCode, fmt.Errorf("read response: %w", err)
	}
	return raw, resp.StatusCode, nil
}

func (c *PrometheusCollector) doGet(ctx context.Context, client *http.Client, token, rawURL string) (map[string]any, error) {
	raw, status, err := c.fetchRaw(ctx, client, token, rawURL)
	if err != nil {
		return nil, err
	}
	if status < 200 || status >= 300 {
		return nil, fmt.Errorf("api %s returned %d", rawURL, status)
	}
	var result map[string]any
	if err := json.Unmarshal(raw, &result); err != nil {
		return nil, fmt.Errorf("parse response: %w", err)
	}
	return result, nil
}

func (c *PrometheusCollector) doGetArray(ctx context.Context, client *http.Client, token, rawURL string) ([]any, error) {
	raw, status, err := c.fetchRaw(ctx, client, token, rawURL)
	if err != nil {
		return nil, err
	}
	if status < 200 || status >= 300 {
		return nil, fmt.Errorf("api %s returned %d", rawURL, status)
	}
	var result []any
	if err := json.Unmarshal(raw, &result); err != nil {
		return nil, fmt.Errorf("parse array response: %w", err)
	}
	return result, nil
}

func (c *PrometheusCollector) queryPrometheus(ctx context.Context, client *http.Client, cfg PrometheusConfig, query string) (float64, error) {
	apiURL := cfg.PrometheusURL + "/api/v1/query?query=" + url.QueryEscape(query)
	result, err := c.doGet(ctx, client, cfg.Token, apiURL)
	if err != nil {
		return 0, err
	}

	if data, ok := result["data"].(map[string]any); ok {
		if results, ok := data["result"].([]any); ok && len(results) > 0 {
			if first, ok := results[0].(map[string]any); ok {
				if value, ok := first["value"].([]any); ok && len(value) >= 2 {
					switch v := value[1].(type) {
					case string:
						var f float64
						if _, err := fmt.Sscanf(v, "%f", &f); err == nil {
							return f, nil
						}
					case float64:
						return v, nil
					}
				}
			}
		}
	}
	return 0, nil
}

func (c *PrometheusCollector) collectUptime(ctx context.Context, client *http.Client, orgID string, cfg PrometheusConfig, controls []ControlMatch) (int, error) {
	uptime, err := c.queryPrometheus(ctx, client, cfg, "avg_over_time(up[24h])*100")
	if err != nil {
		return 0, err
	}

	status := "ok"
	if uptime < 90 {
		status = "warning"
	}

	details := map[string]any{
		"collected_at": time.Now().UTC().Format(time.RFC3339),
		"uptime_24h":   fmt.Sprintf("%.2f%%", uptime),
		"status":       status,
	}

	if status == "warning" {
		details["warning"] = fmt.Sprintf("Uptime unter 90%% (aktuell %.1f%% in letzten 24h).", uptime)
	}

	controlID := firstControlID(controls)
	if err := c.addEvidence(ctx, orgID, controlID,
		fmt.Sprintf("Prometheus Uptime (24h): %.1f%%", uptime), details); err != nil {
		return 0, err
	}
	return 1, nil
}

func (c *PrometheusCollector) collectTargetHealth(ctx context.Context, client *http.Client, orgID string, cfg PrometheusConfig, controls []ControlMatch) (int, error) {
	apiURL := cfg.PrometheusURL + "/api/v1/targets"
	result, err := c.doGet(ctx, client, cfg.Token, apiURL)
	if err != nil {
		return 0, err
	}

	totalTargets := 0
	upTargets := 0

	if data, ok := result["data"].(map[string]any); ok {
		if active, ok := data["activeTargets"].([]any); ok {
			totalTargets = len(active)
			for _, t := range active {
				if tm, ok := t.(map[string]any); ok {
					if health, ok := tm["health"].(string); ok && health == "up" {
						upTargets++
					}
				}
			}
		}
	}

	healthPercent := 100.0
	if totalTargets > 0 {
		healthPercent = float64(upTargets) / float64(totalTargets) * 100
	}

	status := "ok"
	if healthPercent < 90 {
		status = "warning"
	}

	details := map[string]any{
		"collected_at":   time.Now().UTC().Format(time.RFC3339),
		"total_targets":  totalTargets,
		"up_targets":     upTargets,
		"health_percent": fmt.Sprintf("%.1f%%", healthPercent),
		"status":         status,
	}

	controlID := firstControlID(controls)
	if err := c.addEvidence(ctx, orgID, controlID,
		fmt.Sprintf("Prometheus: %d/%d Scrape-Targets erreichbar", upTargets, totalTargets), details); err != nil {
		return 0, err
	}
	return 1, nil
}

func (c *PrometheusCollector) collectAlerts(ctx context.Context, client *http.Client, orgID string, cfg PrometheusConfig, controls []ControlMatch) (int, error) {
	// Alertmanager /api/v2/alerts returns a JSON array directly
	apiURL := cfg.AlertmanagerURL + "/api/v2/alerts?filter=severity=critical"
	alerts, err := c.doGetArray(ctx, client, cfg.Token, apiURL)
	if err != nil {
		return 0, err
	}

	alertNames := []string{}
	for _, a := range alerts {
		if am, ok := a.(map[string]any); ok {
			if labels, ok := am["labels"].(map[string]any); ok {
				if name, ok := labels["alertname"].(string); ok {
					alertNames = append(alertNames, name)
				}
			}
		}
	}

	details := map[string]any{
		"collected_at": time.Now().UTC().Format(time.RFC3339),
		"alert_count":  len(alerts),
		"alert_names":  alertNames,
	}

	if len(alerts) > 0 {
		details["warning"] = fmt.Sprintf("Aktive Critical-Alerts: %d — %v", len(alerts), alertNames)
	}

	controlID := firstControlID(controls)
	if err := c.addEvidence(ctx, orgID, controlID,
		fmt.Sprintf("Alertmanager: %d aktive Critical-Alerts", len(alerts)), details); err != nil {
		return 0, err
	}
	return 1, nil
}

func (c *PrometheusCollector) collectAlertmanagerStatus(ctx context.Context, client *http.Client, orgID string, cfg PrometheusConfig, controls []ControlMatch) (int, error) {
	apiURL := cfg.AlertmanagerURL + "/api/v2/status"
	result, err := c.doGet(ctx, client, cfg.Token, apiURL)
	if err != nil {
		return 0, err
	}

	receiverCount := 0
	if config, ok := result["config"].(map[string]any); ok {
		if original, ok := config["original"].(string); ok && original != "" {
			// Count "name:" occurrences as a rough receiver count
			for _, line := range splitLines(original) {
				if len(line) > 6 && line[:5] == "name:" {
					receiverCount++
				}
			}
		}
	}

	details := map[string]any{
		"collected_at":   time.Now().UTC().Format(time.RFC3339),
		"receiver_count": receiverCount,
	}

	controlID := firstControlID(controls)
	if err := c.addEvidence(ctx, orgID, controlID,
		fmt.Sprintf("Alertmanager aktiv: %d Receiver konfiguriert", receiverCount), details); err != nil {
		return 0, err
	}
	return 1, nil
}

func (c *PrometheusCollector) addEvidence(ctx context.Context, orgID, controlID, title string, details map[string]any) error {
	data, _ := json.Marshal(details)
	if controlID == "" {
		log.Debug().Str("org_id", orgID).Str("title", title).Msg("prometheus_collector: no matching control, skipping")
		return nil
	}
	return c.evidence.AddCollectorEvidence(ctx, orgID, controlID, "", prometheusSource, title, data)
}

// CountTargets returns target count for the status endpoint.
func (c *PrometheusCollector) CountTargets(ctx context.Context, cfg PrometheusConfig) (int, error) {
	client := c.clientFor(cfg.AllowPrivateTarget)
	apiURL := cfg.PrometheusURL + "/api/v1/targets"
	result, err := c.doGet(ctx, client, cfg.Token, apiURL)
	if err != nil {
		return 0, err
	}
	if data, ok := result["data"].(map[string]any); ok {
		if active, ok := data["activeTargets"].([]any); ok {
			return len(active), nil
		}
	}
	return 0, nil
}

// CountActiveAlerts returns critical alert count for the status endpoint.
func (c *PrometheusCollector) CountActiveAlerts(ctx context.Context, cfg PrometheusConfig) (int, error) {
	if cfg.AlertmanagerURL == "" {
		return 0, nil
	}
	client := c.clientFor(cfg.AllowPrivateTarget)
	apiURL := cfg.AlertmanagerURL + "/api/v2/alerts?filter=severity=critical"
	alerts, err := c.doGetArray(ctx, client, cfg.Token, apiURL)
	if err != nil {
		return 0, err
	}
	return len(alerts), nil
}

func splitLines(s string) []string {
	var lines []string
	start := 0
	for i, c := range s {
		if c == '\n' {
			line := s[start:i]
			if len(line) > 0 {
				lines = append(lines, line)
			}
			start = i + 1
		}
	}
	if start < len(s) {
		lines = append(lines, s[start:])
	}
	return lines
}
