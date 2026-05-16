// Copyright (c) 2026 NorvikOps. All rights reserved.
// SPDX-License-Identifier: Elastic-2.0

package lemonsqueezy

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

	"github.com/sechealth-app/sechealth/internal/license"
)

// SMTPConfig holds mail delivery settings (reuses values from the main config).
type SMTPConfig struct {
	Host string
	Port string
	User string
	Pass string
	From string
}

// Handler processes inbound LemonSqueezy webhook events.
type Handler struct {
	webhookSecret string
	privateKeyPEM string
	smtp          SMTPConfig
	db            *pgxpool.Pool
	rdb           *redis.Client
}

// NewHandler constructs a Handler.
// privateKeyPEM is the PEM-encoded ECDSA private key used to sign license keys.
func NewHandler(webhookSecret, privateKeyPEM string, smtpCfg SMTPConfig) *Handler {
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

// Register mounts the LemonSqueezy webhook endpoint on the given group.
func Register(g *echo.Group, h *Handler) {
	g.POST("/webhooks/lemonsqueezy", h.Handle)
}

type lsEvent struct {
	Meta struct {
		EventName string `json:"event_name"`
	} `json:"meta"`
	Data struct {
		ID         string `json:"id"` // LemonSqueezy subscription/order ID
		Attributes struct {
			UserEmail string `json:"user_email"`
			UserName  string `json:"user_name"`
			Status    string `json:"status"`
			// order_id is present on order_refunded events; subscription_id on subscription events.
			OrderID        *int64 `json:"order_id,omitempty"`
			SubscriptionID *int64 `json:"subscription_id,omitempty"`
		} `json:"attributes"`
	} `json:"data"`
}

func (h *Handler) Handle(c echo.Context) error {
	body, err := io.ReadAll(c.Request().Body)
	if err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "cannot read body"})
	}

	if !h.verifySignature(c.Request().Header.Get("X-Signature"), body) {
		log.Warn().Msg("lemonsqueezy webhook: invalid signature")
		return c.JSON(http.StatusUnauthorized, map[string]string{"error": "invalid signature"})
	}

	var event lsEvent
	if err := json.Unmarshal(body, &event); err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "invalid json"})
	}

	ctx := c.Request().Context()

	switch event.Meta.EventName {
	case "subscription_created":
		if event.Data.Attributes.Status != "active" {
			return c.NoContent(http.StatusOK)
		}
		// issueKey is called synchronously so that LemonSqueezy receives a 500
		// and retries if SMTP or key generation fails.
		lsSubID := event.Data.ID
		if err := h.issueKey(ctx, event.Data.Attributes.UserEmail, event.Data.Attributes.UserName, lsSubID); err != nil {
			log.Error().Err(err).
				Str("email", event.Data.Attributes.UserEmail).
				Msg("lemonsqueezy: issueKey failed — returning 500 so LemonSqueezy retries")
			return c.JSON(http.StatusInternalServerError, map[string]string{"error": "key issuance failed"})
		}
		return c.NoContent(http.StatusOK)

	case "subscription_cancelled", "subscription_expired":
		lsSubID := event.Data.ID
		if err := h.handleCancellation(ctx, lsSubID, event.Meta.EventName); err != nil {
			log.Error().Err(err).
				Str("ls_subscription_id", lsSubID).
				Str("event", event.Meta.EventName).
				Msg("lemonsqueezy: cancellation handling failed")
			return c.JSON(http.StatusInternalServerError, map[string]string{"error": "cancellation handling failed"})
		}
		return c.NoContent(http.StatusOK)

	case "order_refunded":
		// For order_refunded, data.id is the ORDER ID — not the subscription ID.
		// The subscription ID lives in attributes.subscription_id.
		if event.Data.Attributes.SubscriptionID == nil {
			log.Warn().Str("order_id", event.Data.ID).
				Msg("lemonsqueezy: order_refunded has no subscription_id — skipping revocation")
			return c.NoContent(http.StatusOK)
		}
		lsSubID := fmt.Sprintf("%d", *event.Data.Attributes.SubscriptionID)
		if err := h.handleCancellation(ctx, lsSubID, "refunded"); err != nil {
			log.Error().Err(err).
				Str("ls_subscription_id", lsSubID).
				Msg("lemonsqueezy: refund handling failed")
			return c.JSON(http.StatusInternalServerError, map[string]string{"error": "refund handling failed"})
		}
		return c.NoContent(http.StatusOK)

	default:
		return c.NoContent(http.StatusOK)
	}
}

func (h *Handler) verifySignature(sig string, body []byte) bool {
	if h.webhookSecret == "" || sig == "" {
		return false
	}
	mac := hmac.New(sha256.New, []byte(h.webhookSecret))
	mac.Write(body)
	return hmac.Equal([]byte(sig), []byte(hex.EncodeToString(mac.Sum(nil))))
}

// issueKey generates a Pro license key, persists the subscription record in the
// database, and then emails the key to the customer.
//
// Order matters for idempotency: if DB insert succeeds but email fails, LemonSqueezy
// retries — the ON CONFLICT clause prevents a duplicate row and we re-send the email.
// If DB fails we never send the email, so the retry will start fresh and succeed.
func (h *Handler) issueKey(ctx context.Context, email, orgName, lsSubID string) error {
	if orgName == "" {
		orgName = email
	}

	// Persist the subscription record BEFORE generating/sending the key so that
	// LemonSqueezy retries are idempotent: ON CONFLICT DO NOTHING prevents double rows.
	// If the DB is unavailable the function returns an error and LS retries — no email
	// is ever sent before the record is safely stored.
	if h.db != nil && lsSubID != "" {
		_, dbErr := h.db.Exec(ctx,
			`INSERT INTO ls_subscriptions (org_id, ls_subscription_id, customer_email, tier, status)
			 VALUES (gen_random_uuid(), $1, $2, 'pro', 'active')
			 ON CONFLICT (ls_subscription_id) DO NOTHING`,
			lsSubID, email,
		)
		if dbErr != nil {
			return fmt.Errorf("persist subscription record: %w", dbErr)
		}
	}

	proFeatures := []string{
		license.FeatureTISAX,
		license.FeatureDORA,
		license.FeatureEUAIAct,
		license.FeatureCRA,
		license.FeatureAIAdvisor,
		license.FeatureAuditPDF,
		license.FeatureSSO,
		license.FeatureAPI,
		license.FeatureSecReflex,
		license.FeatureSecPulse,
	}

	key, err := license.Sign(h.privateKeyPEM, "pro", orgName, proFeatures, nil)
	if err != nil {
		return fmt.Errorf("generate license key: %w", err)
	}

	if err := h.sendLicenseEmail(email, orgName, key); err != nil {
		return fmt.Errorf("send license email: %w", err)
	}

	log.Info().Str("email", email).Str("org", orgName).Msg("lemonsqueezy: Pro license issued and sent")
	return nil
}

// handleCancellation looks up the real org_id for the subscription, adds it to the
// revocation blocklist and marks the subscription as cancelled/expired/refunded.
//
// Fix: the old code stored gen_random_uuid() in ls_subscriptions.org_id at insert
// time, so the UUID fetched here was a random value that never matched any real org.
// We now resolve the real org_id via customer_email → users → org_members.
func (h *Handler) handleCancellation(ctx context.Context, lsSubID, reason string) error {
	if h.db == nil {
		log.Warn().Str("ls_subscription_id", lsSubID).
			Msg("lemonsqueezy: no DB configured — cannot process cancellation")
		return nil
	}

	// Step 1: resolve customer_email from the subscription record.
	var customerEmail string
	err := h.db.QueryRow(ctx,
		`SELECT customer_email FROM ls_subscriptions WHERE ls_subscription_id = $1`,
		lsSubID,
	).Scan(&customerEmail)
	if err != nil {
		// Unknown subscription — log and skip (may pre-date tracking table).
		log.Warn().Err(err).Str("ls_subscription_id", lsSubID).
			Msg("lemonsqueezy: subscription not found in DB — skipping revocation")
		return nil
	}

	// Step 2: look up the user's UUID by email.
	// Return an error so LemonSqueezy retries — the user may not have registered yet.
	var userID string
	err = h.db.QueryRow(ctx,
		`SELECT id::text FROM users WHERE email = $1 LIMIT 1`,
		customerEmail,
	).Scan(&userID)
	if err != nil {
		log.Warn().Err(err).
			Str("ls_subscription_id", lsSubID).
			Str("email", customerEmail).
			Msg("lemonsqueezy: user not found for subscription email — will retry")
		return fmt.Errorf("user not found for email %s: %w", customerEmail, err)
	}

	// Step 3: resolve org_id via org_members (owner role preferred, any role accepted).
	// Return an error so LemonSqueezy retries — the org may not exist yet.
	var orgID string
	err = h.db.QueryRow(ctx,
		`SELECT org_id::text FROM org_members WHERE user_id = $1::uuid ORDER BY (role = 'owner') DESC LIMIT 1`,
		userID,
	).Scan(&orgID)
	if err != nil {
		log.Warn().Err(err).
			Str("ls_subscription_id", lsSubID).
			Str("user_id", userID).
			Msg("lemonsqueezy: org not found for user — will retry")
		return fmt.Errorf("org not found for user %s: %w", userID, err)
	}

	// Mark subscription as cancelled/expired/refunded.
	_, err = h.db.Exec(ctx,
		`UPDATE ls_subscriptions SET status = $1, updated_at = NOW() WHERE ls_subscription_id = $2`,
		reason, lsSubID,
	)
	if err != nil {
		return fmt.Errorf("update subscription status: %w", err)
	}

	// Add org to revocation blocklist so the license middleware returns Community tier.
	_, err = h.db.Exec(ctx,
		`INSERT INTO ls_revoked_subscriptions (org_id, reason, revoked_at)
		 VALUES ($1::uuid, $2, NOW())
		 ON CONFLICT (org_id) DO UPDATE SET reason = $2, revoked_at = NOW()`,
		orgID, reason,
	)
	if err != nil {
		return fmt.Errorf("insert revocation record: %w", err)
	}

	// Invalidate the cached license immediately so the org cannot use Pro features
	// for the remainder of the 60 s cache TTL after revocation.
	license.InvalidateLicenseCache(ctx, h.rdb, orgID)

	log.Info().Str("org_id", orgID).Str("reason", reason).
		Msg("lemonsqueezy: subscription revoked — org downgraded to community tier")
	return nil
}

func (h *Handler) sendLicenseEmail(to, orgName, key string) error {
	subject := "Dein Vakt Pro License Key"
	body := fmt.Sprintf(`Hallo%s,

vielen Dank für deine Vakt Pro Lizenz!

Dein License Key:

%s

Aktivierung — füge diese Zeile in deine .env Datei ein:

  VAKT_LICENSE_KEY=%s

Dann neu starten:

  docker compose restart

Bei Fragen: hello@norvikops.de

NorvikOps Team`,
		func() string {
			if orgName != "" && orgName != to {
				return " " + orgName
			}
			return ""
		}(),
		key, key,
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
