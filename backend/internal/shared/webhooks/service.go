// Copyright (c) 2026 NorvikOps. All rights reserved.
// SPDX-License-Identifier: Elastic-2.0

// Package webhooks provides outgoing webhook delivery for Vakt platform events.
package webhooks

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"net/url"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/rs/zerolog/log"
)

// Event type constants — callers use these when firing events.
const (
	EventFindingCreated        = "finding.created"
	EventFindingSeverityChanged = "finding.severity_changed"
	EventIncidentCreated       = "incident.created"
	EventIncidentStatusChanged = "incident.status_changed"
	EventControlStatusChanged  = "control.status_changed"
)

// Webhook is the stored configuration for a single outgoing webhook endpoint.
type Webhook struct {
	ID               string     `json:"id"`
	OrgID            string     `json:"org_id"`
	Name             string     `json:"name"`
	URL              string     `json:"url"`
	Secret           *string    `json:"secret,omitempty"`
	Events           []string   `json:"events"`
	Active           bool       `json:"active"`
	CreatedAt        time.Time  `json:"created_at"`
	LastTriggeredAt  *time.Time `json:"last_triggered_at,omitempty"`
	LastStatusCode   *int       `json:"last_status_code,omitempty"`
}

// CreateWebhookInput is the validated input for creating a webhook.
type CreateWebhookInput struct {
	Name   string   `json:"name"   validate:"required,min=1,max=255"`
	URL    string   `json:"url"    validate:"required,url"`
	Secret string   `json:"secret"`
	Events []string `json:"events" validate:"required,min=1"`
	Active bool     `json:"active"`
}

// UpdateWebhookInput is the validated input for updating a webhook.
type UpdateWebhookInput struct {
	Name   *string  `json:"name"   validate:"omitempty,min=1,max=255"`
	URL    *string  `json:"url"    validate:"omitempty,url"`
	Secret *string  `json:"secret"`
	Events []string `json:"events"`
	Active *bool    `json:"active"`
}

// WebhookService manages webhook delivery and CRUD operations.
type WebhookService struct {
	db         *pgxpool.Pool
	httpClient *http.Client
}

// NewWebhookService constructs a WebhookService with sensible HTTP timeouts.
func NewWebhookService(db *pgxpool.Pool) *WebhookService {
	return &WebhookService{
		db: db,
		httpClient: &http.Client{
			Timeout: 10 * time.Second,
		},
	}
}

// ListWebhooks returns all webhooks for the given org.
func (s *WebhookService) ListWebhooks(ctx context.Context, orgID string) ([]Webhook, error) {
	rows, err := s.db.Query(ctx, `
		SELECT id::text, org_id::text, name, url, secret, events,
		       active, created_at, last_triggered_at, last_status_code
		FROM webhooks
		WHERE org_id = $1::uuid
		ORDER BY created_at DESC`,
		orgID,
	)
	if err != nil {
		return nil, fmt.Errorf("list webhooks: %w", err)
	}
	defer rows.Close()

	var out []Webhook
	for rows.Next() {
		var w Webhook
		if err := rows.Scan(
			&w.ID, &w.OrgID, &w.Name, &w.URL, &w.Secret, &w.Events,
			&w.Active, &w.CreatedAt, &w.LastTriggeredAt, &w.LastStatusCode,
		); err != nil {
			return nil, fmt.Errorf("scan webhook row: %w", err)
		}
		out = append(out, w)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate webhook rows: %w", err)
	}
	return out, nil
}

// CreateWebhook inserts a new webhook for the org.
// validateWebhookURL rejects non-HTTPS URLs and SSRF targets (loopback, RFC1918, link-local).
func validateWebhookURL(rawURL string) error {
	u, err := url.ParseRequestURI(rawURL)
	if err != nil {
		return fmt.Errorf("invalid webhook URL: %w", err)
	}
	if u.Scheme != "https" {
		return fmt.Errorf("webhook URL must use HTTPS")
	}
	host := u.Hostname()
	ips, err := net.LookupHost(host)
	if err != nil {
		// DNS failure is non-fatal at save time — block only on confirmed private IPs.
		return nil
	}
	privateRanges := []string{
		"10.0.0.0/8", "172.16.0.0/12", "192.168.0.0/16",
		"127.0.0.0/8", "::1/128", "169.254.0.0/16", "fc00::/7",
	}
	var privNets []*net.IPNet
	for _, cidr := range privateRanges {
		_, n, _ := net.ParseCIDR(cidr)
		if n != nil {
			privNets = append(privNets, n)
		}
	}
	for _, ipStr := range ips {
		ip := net.ParseIP(ipStr)
		if ip == nil {
			continue
		}
		for _, n := range privNets {
			if n.Contains(ip) {
				return fmt.Errorf("webhook URL must not resolve to a private or loopback address")
			}
		}
	}
	return nil
}

func (s *WebhookService) CreateWebhook(ctx context.Context, orgID string, input CreateWebhookInput) (*Webhook, error) {
	if err := validateWebhookURL(input.URL); err != nil {
		return nil, err
	}

	if input.Events == nil {
		input.Events = []string{}
	}

	var secretPtr *string
	if input.Secret != "" {
		secretPtr = &input.Secret
	}

	var w Webhook
	err := s.db.QueryRow(ctx, `
		INSERT INTO webhooks (org_id, name, url, secret, events, active)
		VALUES ($1::uuid, $2, $3, $4, $5, $6)
		RETURNING id::text, org_id::text, name, url, secret, events,
		          active, created_at, last_triggered_at, last_status_code`,
		orgID, input.Name, input.URL, secretPtr, input.Events, input.Active,
	).Scan(
		&w.ID, &w.OrgID, &w.Name, &w.URL, &w.Secret, &w.Events,
		&w.Active, &w.CreatedAt, &w.LastTriggeredAt, &w.LastStatusCode,
	)
	if err != nil {
		return nil, fmt.Errorf("create webhook: %w", err)
	}
	return &w, nil
}

// UpdateWebhook applies a partial update to a webhook owned by orgID.
func (s *WebhookService) UpdateWebhook(ctx context.Context, id, orgID string, input UpdateWebhookInput) (*Webhook, error) {
	if input.URL != nil {
		if err := validateWebhookURL(*input.URL); err != nil {
			return nil, err
		}
	}

	var w Webhook
	err := s.db.QueryRow(ctx, `
		UPDATE webhooks
		SET
			name   = COALESCE($3,   name),
			url    = COALESCE($4,   url),
			secret = CASE WHEN $5::boolean THEN $6 ELSE secret END,
			events = CASE WHEN $7::boolean THEN $8 ELSE events END,
			active = COALESCE($9,   active)
		WHERE id = $1::uuid AND org_id = $2::uuid
		RETURNING id::text, org_id::text, name, url, secret, events,
		          active, created_at, last_triggered_at, last_status_code`,
		id, orgID,
		input.Name, input.URL,
		input.Secret != nil, input.Secret,
		input.Events != nil, input.Events,
		input.Active,
	).Scan(
		&w.ID, &w.OrgID, &w.Name, &w.URL, &w.Secret, &w.Events,
		&w.Active, &w.CreatedAt, &w.LastTriggeredAt, &w.LastStatusCode,
	)
	if err != nil {
		return nil, fmt.Errorf("update webhook: %w", err)
	}
	return &w, nil
}

// DeleteWebhook removes a webhook owned by orgID.
func (s *WebhookService) DeleteWebhook(ctx context.Context, id, orgID string) error {
	tag, err := s.db.Exec(ctx, `
		DELETE FROM webhooks WHERE id = $1::uuid AND org_id = $2::uuid`,
		id, orgID,
	)
	if err != nil {
		return fmt.Errorf("delete webhook: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return fmt.Errorf("webhook not found")
	}
	return nil
}

// TriggerEvent delivers payload to all active webhooks of orgID that are
// subscribed to eventType.  Delivery failures are logged but do not propagate
// to the caller — the caller's operation must not fail because a downstream
// webhook is unreachable.
func (s *WebhookService) TriggerEvent(ctx context.Context, orgID, eventType string, payload any) {
	webhooks, err := s.activeWebhooksForEvent(ctx, orgID, eventType)
	if err != nil {
		log.Error().Err(err).
			Str("org_id", orgID).
			Str("event", eventType).
			Msg("webhooks: failed to query active webhooks")
		return
	}

	body, err := json.Marshal(map[string]any{
		"event":   eventType,
		"org_id":  orgID,
		"payload": payload,
	})
	if err != nil {
		log.Error().Err(err).Msg("webhooks: failed to marshal event payload")
		return
	}

	for _, wh := range webhooks {
		s.deliver(ctx, wh, eventType, body)
	}
}

// TestWebhook sends a test ping to a single webhook.
func (s *WebhookService) TestWebhook(ctx context.Context, id, orgID string) (int, error) {
	var wh Webhook
	err := s.db.QueryRow(ctx, `
		SELECT id::text, org_id::text, name, url, secret, events,
		       active, created_at, last_triggered_at, last_status_code
		FROM webhooks
		WHERE id = $1::uuid AND org_id = $2::uuid`,
		id, orgID,
	).Scan(
		&wh.ID, &wh.OrgID, &wh.Name, &wh.URL, &wh.Secret, &wh.Events,
		&wh.Active, &wh.CreatedAt, &wh.LastTriggeredAt, &wh.LastStatusCode,
	)
	if err != nil {
		return 0, fmt.Errorf("webhook not found: %w", err)
	}

	body, _ := json.Marshal(map[string]string{
		"event":   "ping",
		"message": "Vakt webhook test",
	})

	statusCode := s.deliver(ctx, wh, "ping", body)
	return statusCode, nil
}

// activeWebhooksForEvent returns active webhooks for an org that subscribe to eventType.
func (s *WebhookService) activeWebhooksForEvent(ctx context.Context, orgID, eventType string) ([]Webhook, error) {
	rows, err := s.db.Query(ctx, `
		SELECT id::text, org_id::text, name, url, secret, events,
		       active, created_at, last_triggered_at, last_status_code
		FROM webhooks
		WHERE org_id = $1::uuid
		  AND active = true
		  AND ($2 = ANY(events) OR '*' = ANY(events))`,
		orgID, eventType,
	)
	if err != nil {
		return nil, fmt.Errorf("query active webhooks: %w", err)
	}
	defer rows.Close()

	var out []Webhook
	for rows.Next() {
		var w Webhook
		if err := rows.Scan(
			&w.ID, &w.OrgID, &w.Name, &w.URL, &w.Secret, &w.Events,
			&w.Active, &w.CreatedAt, &w.LastTriggeredAt, &w.LastStatusCode,
		); err != nil {
			return nil, fmt.Errorf("scan webhook: %w", err)
		}
		out = append(out, w)
	}
	return out, rows.Err()
}

// deliver POSTs body to wh.URL, optionally signing with HMAC-SHA256, and
// records the response code.  Returns the HTTP status code (0 on network error).
func (s *WebhookService) deliver(ctx context.Context, wh Webhook, eventType string, body []byte) int {
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, wh.URL, bytes.NewReader(body))
	if err != nil {
		log.Error().Err(err).Str("webhook_id", wh.ID).Msg("webhooks: build request failed")
		s.recordDelivery(ctx, wh.ID, 0)
		return 0
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Vakt-Event", eventType)

	if wh.Secret != nil && *wh.Secret != "" {
		sig := computeHMAC(*wh.Secret, body)
		req.Header.Set("X-Vakt-Signature", "sha256="+sig)
	}

	resp, err := s.httpClient.Do(req)
	if err != nil {
		log.Error().Err(err).Str("webhook_id", wh.ID).Msg("webhooks: delivery failed")
		s.recordDelivery(ctx, wh.ID, 0)
		return 0
	}
	defer resp.Body.Close()

	code := resp.StatusCode
	s.recordDelivery(ctx, wh.ID, code)
	return code
}

// recordDelivery updates last_triggered_at and last_status_code.
func (s *WebhookService) recordDelivery(ctx context.Context, webhookID string, statusCode int) {
	var codePtr *int
	if statusCode != 0 {
		codePtr = &statusCode
	}
	if _, err := s.db.Exec(ctx, `
		UPDATE webhooks
		SET last_triggered_at = NOW(), last_status_code = $2
		WHERE id = $1::uuid`,
		webhookID, codePtr,
	); err != nil {
		log.Warn().Err(err).Str("webhook_id", webhookID).Msg("webhooks: record delivery failed")
	}
}

// computeHMAC returns the lowercase hex-encoded HMAC-SHA256 of body using secret.
func computeHMAC(secret string, body []byte) string {
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write(body)
	return hex.EncodeToString(mac.Sum(nil))
}
