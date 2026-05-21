package alerting

import (
	"bytes"
	"context"
	"crypto/aes"
	"crypto/cipher"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/smtp"
	"strings"
	"sync"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/rs/zerolog/log"

	"github.com/matharnica/vakt/internal/shared/safego"
)

// SMTPConfig holds the SMTP settings needed for email-type channels.
type SMTPConfig struct {
	Host string
	Port string
	User string
	Pass string
	From string
}

// Service handles all alerting business logic.
type Service struct {
	repo      *Repository
	masterKey []byte
	client    *http.Client
	smtp      SMTPConfig
}

// NewService creates a new alerting Service.
func NewService(db *pgxpool.Pool, masterKey []byte, smtp SMTPConfig) *Service {
	return &Service{
		repo:      NewRepository(db),
		masterKey: masterKey,
		client:    &http.Client{Timeout: 10 * time.Second},
		smtp:      smtp,
	}
}

// encrypt encrypts plaintext with AES-256-GCM. The 12-byte nonce is prepended to the ciphertext.
func (s *Service) encrypt(plaintext []byte) ([]byte, error) {
	block, err := aes.NewCipher(s.masterKey)
	if err != nil {
		return nil, fmt.Errorf("aes new cipher: %w", err)
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, fmt.Errorf("new gcm: %w", err)
	}
	nonce := make([]byte, gcm.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return nil, fmt.Errorf("generate nonce: %w", err)
	}
	return gcm.Seal(nonce, nonce, plaintext, nil), nil
}

// decrypt decrypts AES-256-GCM ciphertext where the nonce is prepended.
func (s *Service) decrypt(ciphertext []byte) ([]byte, error) {
	block, err := aes.NewCipher(s.masterKey)
	if err != nil {
		return nil, fmt.Errorf("aes new cipher: %w", err)
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, fmt.Errorf("new gcm: %w", err)
	}
	nonceSize := gcm.NonceSize()
	if len(ciphertext) < nonceSize {
		return nil, fmt.Errorf("ciphertext too short")
	}
	nonce, ct := ciphertext[:nonceSize], ciphertext[nonceSize:]
	plaintext, err := gcm.Open(nil, nonce, ct, nil)
	if err != nil {
		return nil, fmt.Errorf("gcm open: %w", err)
	}
	return plaintext, nil
}

// ListChannels returns all notification channels for the org.
func (s *Service) ListChannels(ctx context.Context, orgID string) ([]Channel, error) {
	return s.repo.ListChannels(ctx, orgID)
}

// CreateChannel encrypts the URL, generates an HMAC secret, and stores a new notification channel.
// It returns the created channel, the plaintext hex HMAC secret (shown once), and any error.
func (s *Service) CreateChannel(ctx context.Context, orgID string, in CreateChannelInput) (*Channel, string, error) {
	encryptedURL, err := s.encrypt([]byte(in.URL))
	if err != nil {
		return nil, "", fmt.Errorf("encrypt url: %w", err)
	}

	// Generate 32 random bytes and encode as hex (64-char string).
	secretRaw := make([]byte, 32)
	if _, err := io.ReadFull(rand.Reader, secretRaw); err != nil {
		return nil, "", fmt.Errorf("generate hmac secret: %w", err)
	}
	hexSecret := hex.EncodeToString(secretRaw)

	encryptedHmacSecret, err := s.encrypt([]byte(hexSecret))
	if err != nil {
		return nil, "", fmt.Errorf("encrypt hmac secret: %w", err)
	}

	ch, err := s.repo.CreateChannel(ctx, orgID, in, encryptedURL, encryptedHmacSecret)
	if err != nil {
		return nil, "", err
	}
	return ch, hexSecret, nil
}

// DeleteChannel removes a notification channel.
func (s *Service) DeleteChannel(ctx context.Context, orgID, id string) error {
	return s.repo.DeleteChannel(ctx, orgID, id)
}

// ToggleChannel enables or disables a notification channel.
func (s *Service) ToggleChannel(ctx context.Context, orgID, id string, enabled bool) error {
	return s.repo.ToggleChannel(ctx, orgID, id, enabled)
}

// TestChannel sends a test payload to the channel's webhook URL.
func (s *Service) TestChannel(ctx context.Context, orgID, id string) error {
	raw, err := s.repo.GetChannelRaw(ctx, orgID, id)
	if err != nil {
		return fmt.Errorf("get channel: %w", err)
	}
	urlBytes, err := s.decrypt(raw.URLEncrypted)
	if err != nil {
		return fmt.Errorf("decrypt url: %w", err)
	}

	if raw.Type == "email" {
		to := strings.TrimSpace(string(urlBytes))
		return s.sendEmail(to, "Vakt Test Alert", "Dies ist eine Test-Benachrichtigung von Vakt.")
	}

	testPayload := map[string]any{"text": "Vakt test alert"}
	body, _ := json.Marshal(testPayload)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, string(urlBytes), bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("build request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := s.client.Do(req)
	if err != nil {
		return fmt.Errorf("send test: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("non-2xx response: %d", resp.StatusCode)
	}
	return nil
}

// Fire dispatches an event to all enabled channels subscribed to that event.
// It is fire-and-forget: it returns immediately and delivers in a background
// goroutine. Deliveries are bounded to 10 concurrent goroutines and the whole
// dispatch times out after 30 seconds. Delivery failures are non-fatal.
func (s *Service) Fire(ctx context.Context, orgID, event string, payload map[string]any) {
	channels, err := s.repo.GetEnabledChannelsForEvent(ctx, orgID, event)
	if err != nil {
		log.Error().Err(err).Str("event", event).Str("org_id", orgID).Msg("alerting: get channels failed")
		return
	}

	body, _ := json.Marshal(payload)

	// ADR-0018: outer dispatch + per-channel delivery laufen über safego.Run.
	// Parent-Context wird durchgereicht, der 30 s-Timeout wird über
	// context.WithoutCancel(ctx) gehängt — so überlebt das Fan-out einen
	// Request-Cancel UND respektiert weiterhin shutdown-Signale, falls der
	// Aufrufer einen lifecycle-Context übergibt.
	safego.Run(ctx, "alerting.fanout.dispatch", func(parent context.Context) error {
		fireCtx, cancel := context.WithTimeout(context.WithoutCancel(parent), 30*time.Second)
		defer cancel()

		var wg sync.WaitGroup
		sem := make(chan struct{}, 10) // max 10 concurrent deliveries

		for _, ch := range channels {
			ch := ch // capture loop var
			wg.Add(1)
			sem <- struct{}{}
			safego.Run(fireCtx, "alerting.fanout.deliver", func(c context.Context) error {
				defer wg.Done()
				defer func() { <-sem }()

				urlBytes, err := s.decrypt(ch.URLEncrypted)
				if err != nil {
					log.Error().Err(err).Str("channel_id", ch.ID).Msg("alerting: decrypt url failed")
					status := "failed"
					_ = s.repo.LogDelivery(fireCtx, orgID, &ch.ID, event, status, nil, payload)
					return nil
				}

				// Email channels: deliver via SMTP instead of HTTP.
				if ch.Type == "email" {
					to := strings.TrimSpace(string(urlBytes))
					subject := "Vakt Alert: " + formatEventText(event, payload)
					lines := []string{formatEventText(event, payload), ""}
					for k, v := range payload {
						lines = append(lines, fmt.Sprintf("%s: %v", k, v))
					}
					emailBody := strings.Join(lines, "\r\n")
					err := s.sendEmail(to, subject, emailBody)
					st := "sent"
					if err != nil {
						log.Error().Err(err).Str("channel_id", ch.ID).Msg("alerting: email delivery failed")
						st = "failed"
					}
					_ = s.repo.LogDelivery(fireCtx, orgID, &ch.ID, event, st, nil, payload)
					return nil
				}

				// Format payload according to channel type.
				var bodyBytes []byte
				switch ch.Type {
				case "slack":
					text := formatEventText(event, payload)
					slackBody := map[string]any{
						"text": text,
						"attachments": []map[string]any{{
							"color":  severityColor(event),
							"fields": payloadToFields(payload),
							"footer": "Vakt",
							"ts":     time.Now().Unix(),
						}},
					}
					bodyBytes, _ = json.Marshal(slackBody)
				case "teams":
					text := formatEventText(event, payload)
					teamsBody := map[string]any{
						"@type":      "MessageCard",
						"@context":   "http://schema.org/extensions",
						"summary":    text,
						"themeColor": severityColor(event),
						"title":      "Vakt Alert",
						"sections": []map[string]any{{
							"activityTitle":    text,
							"activitySubtitle": "Event: " + event,
							"facts":            payloadToFacts(payload),
						}},
					}
					bodyBytes, _ = json.Marshal(teamsBody)
				default:
					// webhook: keep original generic format
					bodyBytes = body
				}

				var responseCode *int
				status := "sent"
				var lastErr error
				delays := []time.Duration{0, time.Second, 2 * time.Second, 4 * time.Second}
			retry:
				for attempt, delay := range delays {
					if delay > 0 {
						select {
						case <-fireCtx.Done():
							lastErr = fireCtx.Err()
							break retry
						case <-time.After(delay):
						}
					}
					reqRetry, err := http.NewRequestWithContext(fireCtx, http.MethodPost, string(urlBytes), bytes.NewReader(bodyBytes))
					if err != nil {
						lastErr = err
						break
					}
					reqRetry.Header.Set("Content-Type", "application/json")
					reqRetry.Header.Set("X-Vakt-Event", event)
					if len(ch.HmacSecretEncrypted) > 0 {
						if secretBytes, decErr := s.decrypt(ch.HmacSecretEncrypted); decErr == nil {
							mac := hmac.New(sha256.New, secretBytes)
							mac.Write(bodyBytes)
							reqRetry.Header.Set("X-Vakt-Signature", "sha256="+hex.EncodeToString(mac.Sum(nil)))
						}
					}
					resp, doErr := s.client.Do(reqRetry)
					if doErr != nil {
						lastErr = doErr
						log.Warn().Err(doErr).Int("attempt", attempt+1).Str("channel_id", ch.ID).Msg("alerting: delivery attempt failed")
						continue
					}
					code := resp.StatusCode
					_ = resp.Body.Close()
					if code >= 200 && code < 300 {
						responseCode = &code
						lastErr = nil
						break
					}
					lastErr = fmt.Errorf("non-2xx: %d", code)
					responseCode = &code
					log.Warn().Int("status", code).Int("attempt", attempt+1).Str("channel_id", ch.ID).Msg("alerting: non-2xx response")
				}
				if lastErr != nil {
					log.Error().Err(lastErr).Str("channel_id", ch.ID).Str("event", event).Msg("alerting: delivery failed after retries")
					status = "failed"
				}
				_ = s.repo.LogDelivery(fireCtx, orgID, &ch.ID, event, status, responseCode, payload)
				return nil
			})
		}

		wg.Wait()
		return nil
	})
}

// sendEmail sends a plain-text alert email via the configured SMTP server.
func (s *Service) sendEmail(to, subject, body string) error {
	if s.smtp.Host == "" {
		return fmt.Errorf("SMTP not configured")
	}
	from := s.smtp.From
	if from == "" {
		from = "vakt@" + s.smtp.Host
	}
	port := s.smtp.Port
	if port == "" {
		port = "25"
	}
	headers := fmt.Sprintf("From: %s\r\nTo: %s\r\nSubject: %s\r\nMIME-Version: 1.0\r\nContent-Type: text/plain; charset=UTF-8\r\n\r\n", from, to, subject)
	msg := []byte(headers + body)
	addr := s.smtp.Host + ":" + port
	if s.smtp.User != "" && s.smtp.Pass != "" {
		auth := smtp.PlainAuth("", s.smtp.User, s.smtp.Pass, s.smtp.Host)
		return smtp.SendMail(addr, auth, from, []string{to}, msg)
	}
	return smtp.SendMail(addr, nil, from, []string{to}, msg)
}

// ListDeliveryLog returns the last 100 delivery log entries for the org.
func (s *Service) ListDeliveryLog(ctx context.Context, orgID string) ([]DeliveryLogEntry, error) {
	return s.repo.ListDeliveryLog(ctx, orgID, 100)
}

// ListChannelDeliveries returns the last 50 delivery log entries for a specific channel.
func (s *Service) ListChannelDeliveries(ctx context.Context, orgID, channelID string) ([]DeliveryLogEntry, error) {
	return s.repo.ListChannelDeliveries(ctx, orgID, channelID, 50)
}

// formatEventText creates a human-readable summary for Slack/Teams messages.
func formatEventText(event string, payload map[string]any) string {
	messages := map[string]string{
		"finding.sla_overdue":  "SLA-Frist uberschritten: Offene Sicherheitslucken uberfällig",
		"breach.created":       "Neue Datenpanne erfasst — Art.-33-Meldepflicht prufen",
		"dsr.overdue":          "DSR-Anfrage uberfällig — Bearbeitungsfrist abgelaufen",
		"avv.expired":          "AVV abgelaufen — Auftragsverarbeitervertrag erneuern",
		"scan.failed":          "Scanner-Fehler — Scan konnte nicht abgeschlossen werden",
		"finding.new_critical": "Kritischer Fund entdeckt — sofortiger Handlungsbedarf",
	}
	if msg, ok := messages[event]; ok {
		return msg
	}
	if msgVal, ok := payload["message"]; ok {
		return fmt.Sprintf("Vakt: %s — %v", event, msgVal)
	}
	return "Vakt Alert: " + event
}

// severityColor maps events to brand colors for Slack/Teams.
func severityColor(event string) string {
	switch event {
	case "breach.created", "finding.sla_overdue", "finding.new_critical":
		return "#ef4444"
	case "dsr.overdue", "avv.expired":
		return "#f59e0b"
	default:
		return "#6366f1"
	}
}

// payloadToFields converts a payload map to Slack attachment fields.
func payloadToFields(payload map[string]any) []map[string]any {
	var fields []map[string]any
	for k, v := range payload {
		if k == "message" {
			continue
		}
		fields = append(fields, map[string]any{
			"title": k,
			"value": fmt.Sprint(v),
			"short": true,
		})
	}
	return fields
}

// payloadToFacts converts a payload map to Teams MessageCard facts.
func payloadToFacts(payload map[string]any) []map[string]string {
	var facts []map[string]string
	for k, v := range payload {
		if k == "message" {
			continue
		}
		facts = append(facts, map[string]string{
			"name":  k,
			"value": fmt.Sprint(v),
		})
	}
	return facts
}
