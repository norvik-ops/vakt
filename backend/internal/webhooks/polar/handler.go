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
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/smtp"
	"strconv"
	"strings"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/labstack/echo/v4"
	"github.com/labstack/echo/v4/middleware"
	"github.com/redis/go-redis/v9"
	"github.com/rs/zerolog/log"
	"golang.org/x/time/rate"

	"github.com/matharnica/vakt/internal/license"
	"github.com/matharnica/vakt/internal/shared/logsafe"
	"github.com/matharnica/vakt/internal/shared/mailhdr"
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

// Register mounts the Polar webhook endpoint and the public license-refresh endpoint.
// The license-refresh endpoint is rate-limited to 60 requests/hour per IP.
func Register(g *echo.Group, h *Handler) {
	g.POST("/billing/webhook", h.Handle)

	refreshLimiter := middleware.RateLimiter(middleware.NewRateLimiterMemoryStoreWithConfig(
		middleware.RateLimiterMemoryStoreConfig{
			Rate:      rate.Limit(60.0 / 3600.0),
			Burst:     5,
			ExpiresIn: 10 * time.Minute,
		},
	))
	g.GET("/billing/license", h.GetLicenseByToken, refreshLimiter)
}

// proFeatures is the full set of features included in every issued Pro key.
// TISAX, DORA, ISO 42001, and multi_framework are not offered publicly — gates
// remain in code but no tier issues them via Polar.
// proFeatures is the Pro feature set. Single source of truth: features.ProTier —
// a license issued by the CLI (direct/invoice sale) must unlock exactly the same
// features as one issued by this webhook, otherwise the two paths drift apart.
var proFeatures = features.ProTier

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
	// Price.RecurringInterval is "month" or "year" — used to set the key expiry.
	Price struct {
		RecurringInterval string `json:"recurring_interval"`
	} `json:"price"`
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

	hdr := c.Request().Header
	if !h.verifySignature(hdr.Get("webhook-id"), hdr.Get("webhook-timestamp"), hdr.Get("webhook-signature"), body) {
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
		// "trialing" is issued too — a trial subscriber needs a (short) key to
		// actually try Pro; keyExpiry caps it to the trial window.
		if event.Data.Status != "active" && event.Data.Status != "trialing" {
			return c.NoContent(http.StatusOK)
		}
		if err := h.issueKey(ctx, event.Data.Customer.Email, event.Data.Customer.Name, event.Data.ID, event.Data.Price.RecurringInterval, event.Data.Status, false); err != nil {
			log.Error().Err(err).
				Str("email_redacted", logsafe.RedactEmail(event.Data.Customer.Email)).
				Str("subscription_id", event.Data.ID).
				Msg("polar: issueKey failed — returning 500 so Polar retries")
			return c.JSON(http.StatusInternalServerError, map[string]string{"error": "key issuance failed"})
		}
		return c.NoContent(http.StatusOK)

	case "subscription.updated", "subscription.uncanceled":
		// subscription.updated fires on renewals (status flips back to "active").
		// subscription.uncanceled fires when a customer reverses a cancellation.
		// Only issue a new key when the subscription is active or trialing.
		if event.Data.Status != "active" && event.Data.Status != "trialing" {
			return c.NoContent(http.StatusOK)
		}
		if err := h.issueKey(ctx, event.Data.Customer.Email, event.Data.Customer.Name, event.Data.ID, event.Data.Price.RecurringInterval, event.Data.Status, true); err != nil {
			log.Error().Err(err).
				Str("email_redacted", logsafe.RedactEmail(event.Data.Customer.Email)).
				Str("subscription_id", event.Data.ID).
				Msg("polar: renewal issueKey failed — returning 500 so Polar retries")
			return c.JSON(http.StatusInternalServerError, map[string]string{"error": "key renewal failed"})
		}
		return c.NoContent(http.StatusOK)

	case "subscription.revoked", "subscription.canceled":
		h.handleCancellation(ctx, event.Data.ID, event.Type)
		return c.NoContent(http.StatusOK)

	default:
		return c.NoContent(http.StatusOK)
	}
}

// verifySignature verifies the Polar.sh webhook signature per the Standard Webhooks
// spec (https://www.standardwebhooks.com), which Polar follows.
//
// Polar sends three headers: webhook-id, webhook-timestamp, webhook-signature.
// The signed content is "{id}.{timestamp}.{body}", HMAC-SHA256 keyed with the raw
// secret string (Polar uses the secret as raw UTF-8, NOT base64-decoded — unlike
// Svix/Clerk), then base64-encoded. webhook-signature is a space-separated list of
// "v1,<base64>" signatures (there can be several during secret rotation); the request
// is valid if any one matches.
func (h *Handler) verifySignature(id, timestamp, sigHeader string, body []byte) bool {
	if h.webhookSecret == "" || id == "" || timestamp == "" || sigHeader == "" {
		return false
	}
	// Reject stale or far-future timestamps to prevent replay of a captured (but
	// legitimately signed) event. Standard Webhooks recommends a ±5 min tolerance.
	// The sha256(body) dedup table is the second line of defense; this is the first.
	ts, err := strconv.ParseInt(timestamp, 10, 64)
	if err != nil {
		return false
	}
	if diff := time.Now().Unix() - ts; diff > 300 || diff < -300 {
		return false
	}
	signedContent := id + "." + timestamp + "." + string(body)
	mac := hmac.New(sha256.New, []byte(h.webhookSecret))
	mac.Write([]byte(signedContent))
	expected := base64.StdEncoding.EncodeToString(mac.Sum(nil))
	for _, part := range strings.Fields(sigHeader) {
		// Each part is "v1,<base64>"; compare the base64 portion in constant time.
		if _, sig, ok := strings.Cut(part, ","); ok && hmac.Equal([]byte(sig), []byte(expected)) {
			return true
		}
	}
	return false
}

// keyExpiry returns the license validity duration based on the Polar recurring interval.
// Monthly subscriptions get 35 days (31 + 4 day grace), yearly get 395 days (365 + 30 day grace).
// Unknown intervals fall back to the monthly duration.
// A "trialing" subscription is capped to the trial window (30 day trial + 15 day
// grace) regardless of interval — the grace gives a manual-activation customer time
// to paste the full key that is mailed on conversion. The full-interval key is
// issued on conversion to "active".
func keyExpiry(interval, status string) time.Time {
	if status == "trialing" {
		return time.Now().Add(45 * 24 * time.Hour)
	}
	if interval == "year" {
		return time.Now().Add(395 * 24 * time.Hour)
	}
	return time.Now().Add(35 * 24 * time.Hour)
}

// issueKey generates a Pro license key with expiry, persists the subscription record, and emails the key.
// isRenewal controls the email subject line.
func (h *Handler) issueKey(ctx context.Context, email, orgName, polarSubID, interval, status string, isRenewal bool) error {
	if orgName == "" {
		orgName = email
	}

	var renewalToken string
	if h.db != nil && polarSubID != "" {
		_, dbErr := h.db.Exec(ctx,
			`INSERT INTO polar_subscriptions (polar_subscription_id, customer_email, tier, status)
			 VALUES ($1, $2, 'pro', 'active')
			 ON CONFLICT (polar_subscription_id) DO UPDATE SET status = 'active', updated_at = NOW()`,
			polarSubID, email,
		)
		if dbErr != nil {
			return fmt.Errorf("persist subscription record: %w", dbErr)
		}
		// Fetch the stable renewal_token for this subscription (generated on INSERT).
		_ = h.db.QueryRow(ctx,
			`SELECT renewal_token FROM polar_subscriptions WHERE polar_subscription_id = $1`,
			polarSubID,
		).Scan(&renewalToken)
	}

	expiry := keyExpiry(interval, status)
	key, err := license.Sign(h.privateKeyPEM, "pro", orgName, proFeatures, &expiry)
	if err != nil {
		return fmt.Errorf("generate license key: %w", err)
	}

	// Persist the generated key so GET /billing/license/:token can serve it.
	if h.db != nil && polarSubID != "" {
		_, _ = h.db.Exec(ctx,
			`UPDATE polar_subscriptions SET license_key = $1, updated_at = NOW()
			 WHERE polar_subscription_id = $2`,
			key, polarSubID,
		)
	}

	if err := h.sendLicenseEmail(email, orgName, key, renewalToken, isRenewal, status == "trialing"); err != nil {
		return fmt.Errorf("send license email: %w", err)
	}

	log.Info().
		Str("email_redacted", logsafe.RedactEmail(email)).
		Str("org", orgName).
		Str("interval", interval).
		Time("expires_at", expiry).
		Bool("renewal", isRenewal).
		Msg("polar: Pro license issued and sent")
	return nil
}

// GetLicenseByToken serves GET /billing/license.
// The renewal token is passed in the Authorization: Bearer <token> header so it
// does not appear in access logs or server-side URL records.
func (h *Handler) GetLicenseByToken(c echo.Context) error {
	auth := c.Request().Header.Get("Authorization")
	token := strings.TrimPrefix(auth, "Bearer ")
	if token == "" || token == auth {
		return c.JSON(http.StatusUnauthorized, map[string]string{"error": "Authorization: Bearer <token> required"})
	}
	if h.db == nil {
		return c.JSON(http.StatusServiceUnavailable, map[string]string{"error": "billing not configured"})
	}
	var key string
	err := h.db.QueryRow(c.Request().Context(),
		`SELECT license_key FROM polar_subscriptions
		 WHERE renewal_token = $1::uuid AND status = 'active' AND license_key IS NOT NULL`,
		token,
	).Scan(&key)
	if err != nil {
		// 404 for not-found or wrong token — same response to prevent oracle attacks.
		return c.JSON(http.StatusNotFound, map[string]string{"error": "not found"})
	}
	return c.JSON(http.StatusOK, map[string]string{"key": key})
}

// handleCancellation records the cancellation in polar_subscriptions.
// Revocation for self-hosted instances works purely through key expiry — Norvik has no
// access to the customer's database and cannot push a revocation there.
func (h *Handler) handleCancellation(ctx context.Context, polarSubID, reason string) {
	if h.db == nil {
		log.Info().Str("polar_subscription_id", polarSubID).Str("reason", reason).
			Msg("polar: subscription cancelled — key will expire at its scheduled expiry")
		return
	}
	if _, err := h.db.Exec(ctx,
		`UPDATE polar_subscriptions SET status = $1, updated_at = NOW() WHERE polar_subscription_id = $2`,
		reason, polarSubID,
	); err != nil {
		log.Warn().Err(err).Str("polar_subscription_id", polarSubID).
			Msg("polar: could not update subscription status on cancellation")
	}
	log.Info().Str("polar_subscription_id", polarSubID).Str("reason", reason).
		Msg("polar: subscription cancelled — key will expire at its scheduled expiry")
}

func (h *Handler) sendLicenseEmail(to, orgName, key, renewalToken string, isRenewal, isTrial bool) error {
	subject := "Dein Vakt Pro License Key"
	intro := "vielen Dank für deine Vakt Pro Lizenz!"
	switch {
	case isTrial:
		subject = "Dein Vakt Pro License Key (Testphase)"
		intro = "willkommen zu deiner 30-tägigen Vakt Pro Testphase! Dein License Key ist für die Dauer der Testphase gültig. " +
			"Es ist keine weitere Aktion nötig: Wandelt sich die Testphase in ein bezahltes Abo um, bekommst du automatisch einen neuen Key mit voller Laufzeit. " +
			"Kündigst du vor Ende der Testphase, läuft der Key einfach aus."
	case isRenewal:
		subject = "Dein neuer Vakt Pro License Key"
		intro = "deine Vakt Pro Lizenz wurde verlängert. Hier ist dein neuer License Key — bitte aktiviere ihn in deiner Vakt-Instanz, damit die Laufzeit aktualisiert wird."
	}

	autoRenewalSection := ""
	if renewalToken != "" {
		autoRenewalSection = fmt.Sprintf(`
Auto-Renewal (optional):
Damit deine Instanz den Key automatisch erneuert, setze in deiner .env:

  VAKT_LICENSE_TOKEN=%s

Die Instanz holt sich dann täglich den aktuellen Key — kein manueller Eingriff mehr nötig.
Ausgehende Verbindung: api.norvikops.de (nur Lizenzdaten, keine Geschäftsdaten).
`, renewalToken)
	}

	body := fmt.Sprintf(`Hallo%s,

%s

Dein License Key:

%s
%s
Manuelle Aktivierung in der Vakt-Oberfläche:
→ Einstellungen → Lizenz → License Key eingeben → Aktivieren

Bei Fragen: hello@norvikops.de

NorvikOps Team`,
		func() string {
			if orgName != "" && orgName != to {
				return " " + orgName
			}
			return ""
		}(),
		intro,
		key,
		autoRenewalSection,
	)

	msg := "From: " + mailhdr.Sanitize(h.smtp.From) + "\r\n" +
		"To: " + mailhdr.Sanitize(to) + "\r\n" +
		"Date: " + time.Now().UTC().Format(time.RFC1123Z) + "\r\n" +
		"Subject: " + mailhdr.Sanitize(subject) + "\r\n" +
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
