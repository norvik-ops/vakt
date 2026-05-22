// Package siem provides SIEM integration for Vakt — forwarding audit log entries
// to external SIEM systems (Splunk, Elasticsearch, generic webhook).
package siem

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"
)

// Adapter is the interface for SIEM backends.
type Adapter interface {
	// Forward sends a batch of audit entries to the SIEM. Returns error on failure.
	Forward(ctx context.Context, entries []AuditEntry) error
	// Name returns a human-readable adapter name.
	Name() string
}

// AuditEntry is the portable representation of an audit log row for SIEM forwarding.
type AuditEntry struct {
	ID           string
	OrgID        string
	Action       string
	ResourceType string
	ResourceID   string
	UserEmail    string
	IPAddress    string
	Details      json.RawMessage
	CreatedAt    time.Time
}

func (e AuditEntry) toMap() map[string]any {
	m := map[string]any{
		"id":            e.ID,
		"org_id":        e.OrgID,
		"action":        e.Action,
		"resource_type": e.ResourceType,
		"resource_id":   e.ResourceID,
		"user_email":    e.UserEmail,
		"ip_address":    e.IPAddress,
		"created_at":    e.CreatedAt.UTC().Format(time.RFC3339),
	}
	if len(e.Details) > 0 {
		m["details"] = json.RawMessage(e.Details)
	}
	return m
}

// httpClient is the shared HTTP client with a 10-second timeout.
var httpClient = &http.Client{Timeout: 10 * time.Second}

// ── Splunk HEC ────────────────────────────────────────────────────────────────

// SplunkHECAdapter forwards audit entries to a Splunk HTTP Event Collector.
type SplunkHECAdapter struct {
	endpoint string
	token    string
}

// NewSplunkHECAdapter constructs a SplunkHECAdapter.
func NewSplunkHECAdapter(endpoint, token string) *SplunkHECAdapter {
	return &SplunkHECAdapter{endpoint: endpoint, token: token}
}

func (a *SplunkHECAdapter) Name() string { return "splunk_hec" }

func (a *SplunkHECAdapter) Forward(ctx context.Context, entries []AuditEntry) error {
	for _, e := range entries {
		body := map[string]any{
			"event":      e.toMap(),
			"sourcetype": "vakt:audit",
			"time":       float64(e.CreatedAt.Unix()),
		}
		data, err := json.Marshal(body)
		if err != nil {
			return fmt.Errorf("marshal splunk event: %w", err)
		}

		req, err := http.NewRequestWithContext(ctx, http.MethodPost,
			a.endpoint+"/services/collector/event", bytes.NewReader(data))
		if err != nil {
			return fmt.Errorf("build splunk request: %w", err)
		}
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Authorization", "Splunk "+a.token)

		resp, err := httpClient.Do(req)
		if err != nil {
			return fmt.Errorf("splunk hec forward: %w", err)
		}
		resp.Body.Close()
		if resp.StatusCode >= 400 {
			return fmt.Errorf("splunk hec returned %d", resp.StatusCode)
		}
	}
	return nil
}

// ── Elasticsearch ─────────────────────────────────────────────────────────────

// ElasticAdapter forwards audit entries via the Elasticsearch Bulk API.
type ElasticAdapter struct {
	endpoint string
	token    string
}

// NewElasticAdapter constructs an ElasticAdapter.
func NewElasticAdapter(endpoint, token string) *ElasticAdapter {
	return &ElasticAdapter{endpoint: endpoint, token: token}
}

func (a *ElasticAdapter) Name() string { return "elastic" }

func (a *ElasticAdapter) Forward(ctx context.Context, entries []AuditEntry) error {
	if len(entries) == 0 {
		return nil
	}

	var buf bytes.Buffer
	indexMeta := []byte(`{"index":{"_index":"vakt-audit"}}` + "\n")
	for _, e := range entries {
		buf.Write(indexMeta)
		doc, err := json.Marshal(e.toMap())
		if err != nil {
			return fmt.Errorf("marshal elastic doc: %w", err)
		}
		buf.Write(doc)
		buf.WriteByte('\n')
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost,
		a.endpoint+"/_bulk", bytes.NewReader(buf.Bytes()))
	if err != nil {
		return fmt.Errorf("build elastic request: %w", err)
	}
	req.Header.Set("Content-Type", "application/x-ndjson")
	req.Header.Set("Authorization", "ApiKey "+a.token)

	resp, err := httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("elastic bulk forward: %w", err)
	}
	resp.Body.Close()
	if resp.StatusCode >= 400 {
		return fmt.Errorf("elastic bulk returned %d", resp.StatusCode)
	}
	return nil
}

// ── Webhook ───────────────────────────────────────────────────────────────────

// WebhookAdapter forwards audit entries as a JSON array to a generic HTTP endpoint.
type WebhookAdapter struct {
	endpoint string
	token    string
}

// NewWebhookAdapter constructs a WebhookAdapter.
func NewWebhookAdapter(endpoint, token string) *WebhookAdapter {
	return &WebhookAdapter{endpoint: endpoint, token: token}
}

func (a *WebhookAdapter) Name() string { return "webhook" }

func (a *WebhookAdapter) Forward(ctx context.Context, entries []AuditEntry) error {
	if len(entries) == 0 {
		return nil
	}

	payload := make([]map[string]any, len(entries))
	for i, e := range entries {
		payload[i] = e.toMap()
	}
	data, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("marshal webhook payload: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, a.endpoint, bytes.NewReader(data))
	if err != nil {
		return fmt.Errorf("build webhook request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	if a.token != "" {
		req.Header.Set("Authorization", "Bearer "+a.token)
	}

	resp, err := httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("webhook forward: %w", err)
	}
	resp.Body.Close()
	if resp.StatusCode >= 400 {
		return fmt.Errorf("webhook returned %d", resp.StatusCode)
	}
	return nil
}

// newAdapter constructs the correct Adapter from an OrgSIEMConfig.
func newAdapter(cfg OrgSIEMConfig) (Adapter, error) {
	switch cfg.Adapter {
	case "splunk_hec":
		return NewSplunkHECAdapter(cfg.Endpoint, cfg.Token), nil
	case "elastic":
		return NewElasticAdapter(cfg.Endpoint, cfg.Token), nil
	case "webhook":
		return NewWebhookAdapter(cfg.Endpoint, cfg.Token), nil
	default:
		return nil, fmt.Errorf("unknown siem adapter: %s", cfg.Adapter)
	}
}
