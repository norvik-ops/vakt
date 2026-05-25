// Copyright (c) 2026 NorvikOps. All rights reserved.
// SPDX-License-Identifier: Elastic-2.0

// Package polar handles inbound Polar.sh webhook events for license issuance and revocation.
//
// Signature verification uses HMAC-SHA256 over the raw request body.
// Polar sends the signature in the "webhook-signature" header as "v1=<hex-digest>".
// See: https://docs.polar.sh/developers/webhooks
package polar

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/smtp"
	"strings"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/labstack/echo/v4"
	"github.com/redis/go-redis/v9"
	"github.com/rs/zerolog/log"

	"github.com/matharnica/vakt/internal/license"
	"github.com/matharnica/vakt/internal/shared/platform/features"
)

// SMTPConfig holds mail delivery settings (reuses values from the main config).
type SMTPConfig struct {
	Host string
	Port string
	User string
	Pass string
	From string
}

// Handler processes inbound Polar.sh webhook events.
type Handler struct {
	webhookSecret string
	privateKeyPEM string
	smtp          SMTPConfig
	db            *pgxpool.Pool
	rdb           *redis.Client
}

// NewHandler constructs a Handler.
// privateKeyPEM is the PEM-encoded ECDSA private key used to sign license keys.
// When webhookSecret is empty the handler rejects every request.
func NewHandler(webhookSecret, privateKeyPEM string, smtpCfg SMTPConfig) *Handler {
	if webhookSecret == "" {
		log.Warn().Msg("polar: VAKT_POLAR_WEBHOOK_SECRET is empty — " +
			"webhook signature verification will reject every request.")
	}
	return &Handler{
		webhookSecret: webhookSecret,
		privateKeyPEM: privateKeyPEM,
		smtp:          smtpCfg,
	}
}

// WithDB attaches a database pool to the handler for subscription tracking.
func (h *Handler) WithDB(db *pgxpool.Pool) *Handler {
	h.db = db
	return h
}

// WithRedis attaches a Redis client so the handler can invalidate the license
// cache immediately after a subscription is revoked.
func (h *Handler) WithRedis(rdb *redis.Client) *Handler {
	h.rdb = rdb
	return h
}

// Register mounts the Polar webhook endpoint on the given group.
func Register(g *echo.Group, h *Handler) {
	g.POST("/billing/webhook", h.Handle)
}

// polarSubscription is the subscription object in Polar webhook payloads.
type polarSubscription struct {
	ID       string `json:"id"`
	Status   string `json:"status"`
	Customer struct {
		Email string `json:"email"`
		Name  string `json:"name"`
	} `json:"customer"`
	Product struct {
		Name string `json:"name"`
	} `json:"product"`
}

// polarEvent is the top-level Polar.sh webhook event structure.
type polarEvent struct {
	Type string            `json:"type"`
	Data polarSubscription `json:"data"`
}

func (h *Handler) Handle(c echo.Context) error {
	body, err := io.ReadAll(c.Request().Body)
	if err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "cannot read body"})
	}

	if !h.verifySignature(c.Request().Header.Get("webhook-signature"), body) {
		log.Warn().Msg("polar webhook: invalid signature")
		return c.JSON(http.StatusUnauthorized, map[string]string{"error": "invalid signature"})
	}

	var event polarEvent
	if err := json.Unmarshal(body, &event); err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "invalid json"})
	}

	ctx := c.Request().Context()

	// Replay protection: deduplicate on sha256(body) before business logic.
	if h.db != nil {
		sum := sha256.Sum256(body)
		eventHash := hex.EncodeToString(sum[:])
		tag, dedupErr := h.db.Exec(ctx,
			`INSERT INTO polar_webhook_events (event_hash, event_type)
			 VALUES ($1, $2) ON CONFLICT (event_hash) DO NOTHING`,
			eventHash, event.Type,
		)
		if dedupErr != nil {
			log.Error().Err(dedupErr).Str("event_hash", eventHash).
				Msg("polar: dedup insert failed")
			return c.JSON(http.StatusInternalServerError, map[string]string{"error": "dedup persistence failed"})
		}
		if tag.RowsAffected() == 0 {
			log.Info().Str("event_hash", eventHash).Str("event_type", event.Type).
				Msg("polar: duplicate webhook detected — skipping replay")
			return c.NoContent(http.StatusOK)
		}
	}

	switch event.Type {
	case "subscription.created", "subscription.active":
		if event.Data.Status != "active" {
			return c.NoContent(http.StatusOK)
		}
		if err := h.issueKey(ctx, event.Data.Customer.Email, event.Data.Customer.Name, event.Data.ID); err != nil {
			log.Error().Err(err).
				Str("email", event.Data.Customer.Email).
				Str("subscription_id", event.Data.ID).
				Msg("polar: issueKey failed — returning 500 so Polar retries")
			return c.JSON(http.StatusInternalServerError, map[string]string{"error": "key issuance failed"})
		}
		return c.NoContent(http.StatusOK)

	case "subscription.revoked", "subscription.canceled":
		if err := h.handleCancellation(ctx, event.Data.ID, event.Type); err != nil {
			log.Error().Err(err).
				Str("subscription_id", event.Data.ID).
				Str("event_type", event.Type).
				Msg("polar: cancellation handling failed")
			return c.JSON(http.StatusInternalServerError, map[string]string{"error": "cancellation handling failed"})
		}
		return c.NoContent(http.StatusOK)

	default:
		return c.NoContent(http.StatusOK)
	}
}

// verifySignature verifies the Polar.sh HMAC-SHA256 webhook signature.
// Polar sends the signature as "v1=<hex-digest>" in the "webhook-signature" header.
func (h *Handler) verifySignature(sig string, body []byte) bool {
	if h.webhookSecret == "" || sig == "" {
		return false
	}
	// Strip the "v1=" prefix if present.
	hexSig := strings.TrimPrefix(sig, "v1=")
	mac := hmac.New(sha256.New, []byte(h.webhookSecret))
	mac.Write(body)
	expected := hex.EncodeToString(mac.Sum(nil))
	return hmac.Equal([]byte(hexSig), []byte(expected))
}

// issueKey generates a Pro license key, persists the subscription record, and emails the key.
func (h *Handler) issueKey(ctx context.Context, email, orgName, polarSubID string) error {
	if orgName == "" {
		orgName = email
	}

	// Persist the subscription record BEFORE generating/sending the key.
	// ON CONFLICT DO NOTHING ensures Polar retries are idempotent.
	if h.db != nil && polarSubID != "" {
		_, dbErr := h.db.Exec(ctx,
			`INSERT INTO polar_subscriptions (polar_subscription_id, customer_email, tier, status)
			 VALUES ($1, $2, 'pro', 'active')
			 ON CONFLICT (polar_subscription_id) DO NOTHING`,
			polarSubID, email,
		)
		if dbErr != nil {
			return fmt.Errorf("persist subscription record: %w", dbErr)
		}
	}

	proFeatures := []string{
		features.FeatureTISAX,
		features.FeatureDORA,
		features.FeatureEUAIAct,
		features.FeatureCRA,
		features.FeatureAIAdvisor,
		features.FeatureAuditPDF,
		features.FeatureSSO,
		features.FeatureAPI,
		features.FeatureSecReflex,
		features.FeatureSecPulse,
		features.FeatureGranularPermissions,
		features.FeatureSupplierPortal,
		features.FeatureNIS2Reporting,
		features.FeatureSAMLAuth,
		features.FeatureMultiFramework,
	}

	key, err := license.Sign(h.privateKeyPEM, "pro", orgName, proFeatures, nil)
	if err != nil {
		return fmt.Errorf("generate license key: %w", err)
	}

	if err := h.sendLicenseEmail(email, orgName, key); err != nil {
		return fmt.Errorf("send license email: %w", err)
	}

	log.Info().Str("email", email).Str("org", orgName).Msg("polar: Pro license issued and sent")
	return nil
}

// handleCancellation revokes the subscription and downgrades the org to Community.
func (h *Handler) handleCancellation(ctx context.Context, polarSubID, reason string) error {
	if h.db == nil {
		log.Warn().Str("polar_subscription_id", polarSubID).
			Msg("polar: no DB configured — cannot process cancellation")
		return nil
	}

	var customerEmail string
	err := h.db.QueryRow(ctx,
		`SELECT customer_email FROM polar_subscriptions WHERE polar_subscription_id = $1`,
		polarSubID,
	).Scan(&customerEmail)
	if err != nil {
		log.Warn().Err(err).Str("polar_subscription_id", polarSubID).
			Msg("polar: subscription not found in DB — skipping revocation")
		return nil
	}

	var userID string
	err = h.db.QueryRow(ctx,
		`SELECT id::text FROM users WHERE email = $1 LIMIT 1`,
		customerEmail,
	).Scan(&userID)
	if err != nil {
		return fmt.Errorf("user not found for email %s: %w", customerEmail, err)
	}

	var orgID string
	err = h.db.QueryRow(ctx,
		`SELECT org_id::text FROM org_members WHERE user_id = $1::uuid ORDER BY (role = 'owner') DESC LIMIT 1`,
		userID,
	).Scan(&orgID)
	if err != nil {
		return fmt.Errorf("org not found for user %s: %w", userID, err)
	}

	_, err = h.db.Exec(ctx,
		`UPDATE polar_subscriptions SET status = $1, updated_at = NOW() WHERE polar_subscription_id = $2`,
		reason, polarSubID,
	)
	if err != nil {
		return fmt.Errorf("update subscription status: %w", err)
	}

	_, err = h.db.Exec(ctx,
		`INSERT INTO ls_revoked_subscriptions (org_id, reason, revoked_at)
		 VALUES ($1::uuid, $2, NOW())
		 ON CONFLICT (org_id) DO UPDATE SET reason = $2, revoked_at = NOW()`,
		orgID, reason,
	)
	if err != nil {
		return fmt.Errorf("insert revocation record: %w", err)
	}

	license.InvalidateLicenseCache(ctx, h.rdb, orgID)

	log.Info().Str("org_id", orgID).Str("reason", reason).
		Msg("polar: subscription revoked — org downgraded to community tier")
	return nil
}

func (h *Handler) sendLicenseEmail(to, orgName, key string) error {
	subject := "Dein Vakt Pro License Key"
	body := fmt.Sprintf(`Hallo%s,

vielen Dank für deine Vakt Pro Lizenz!

Dein License Key:

%s

Aktivierung in der Vakt-Oberfläche:
→ Einstellungen → Lizenz → License Key eingeben → Aktivieren

Bei Fragen: hello@norvikops.de

NorvikOps Team`,
		func() string {
			if orgName != "" && orgName != to {
				return " " + orgName
			}
			return ""
		}(),
		key,
	)

	msg := "From: " + h.smtp.From + "\r\n" +
		"To: " + to + "\r\n" +
		"Date: " + time.Now().UTC().Format(time.RFC1123Z) + "\r\n" +
		"Subject: " + subject + "\r\n" +
		"Content-Type: text/plain; charset=UTF-8\r\n" +
		"\r\n" +
		strings.ReplaceAll(body, "\n", "\r\n")

	addr := h.smtp.Host + ":" + h.smtp.Port
	var auth smtp.Auth
	if h.smtp.User != "" {
		auth = smtp.PlainAuth("", h.smtp.User, h.smtp.Pass, h.smtp.Host)
	}
	return smtp.SendMail(addr, auth, h.smtp.From, []string{to}, []byte(msg))
}
