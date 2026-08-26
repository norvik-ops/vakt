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
	"net"
	"net/http"
	"net/smtp"
	"net/url"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/rs/zerolog/log"

	"github.com/matharnica/vakt/internal/shared/httputil"
	"github.com/matharnica/vakt/internal/shared/mailhdr"
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

// defaultDispatchBudget bounds one Fire fan-out end to end. It was the
// timeout of the old fire-and-forget dispatch goroutine and stays the same
// number now that Fire blocks on its own fan-out (ADR-0083).
const defaultDispatchBudget = 30 * time.Second

// dispatchGrace is the belt-and-braces margin on top of the budget. Every
// delivery path is context-aware and must finish within the budget; the grace
// only covers a path that ignores its context, so that a stuck channel can
// never pin the caller forever.
const dispatchGrace = 5 * time.Second

// Service handles all alerting business logic.
type Service struct {
	repo           *Repository
	masterKey      []byte
	client         *http.Client
	smtp           SMTPConfig
	dispatchBudget time.Duration
}

// NewService creates a new alerting Service.
func NewService(db *pgxpool.Pool, masterKey []byte, smtp SMTPConfig) *Service {
	return &Service{
		repo:      NewRepository(db),
		masterKey: masterKey,
		// S124-2 (SA15-01): GuardedClient re-resolves+dials the same IP so a
		// customer-configured webhook host cannot DNS-rebind to an internal IP
		// after the URL was validated at save. allowPrivate=true keeps legitimate
		// on-prem webhook receivers working (self-hosted product) while still
		// closing the rebinding TOCTOU.
		client:         httputil.GuardedClient(10*time.Second, true),
		smtp:           smtp,
		dispatchBudget: defaultDispatchBudget,
	}
}

// WithDispatchBudget overrides how long one Fire fan-out may take. Tests use it
// to keep a deliberately unreachable channel from burning the full 30 seconds.
// A value of zero or less keeps the default.
func (s *Service) WithDispatchBudget(d time.Duration) *Service {
	if d > 0 {
		s.dispatchBudget = d
	}
	return s
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

// validateAlertingURL rejects URLs that resolve to loopback, private, link-local,
// or the cloud metadata service (169.254.169.254). Email-type channels pass an
// email address here, not a URL — callers must skip validation for type=email.
func validateAlertingURL(rawURL string) error {
	u, err := url.Parse(rawURL)
	if err != nil {
		return fmt.Errorf("invalid URL: %w", err)
	}
	if u.Scheme != "https" && u.Scheme != "http" {
		return fmt.Errorf("alerting channel URL scheme must be http or https")
	}
	host := u.Hostname()
	addrs, err := net.LookupHost(host)
	if err != nil {
		return fmt.Errorf("cannot resolve host %q: %w", host, err)
	}
	for _, addr := range addrs {
		ip := net.ParseIP(addr)
		if ip == nil {
			continue
		}
		if ip.IsLoopback() || ip.IsPrivate() || ip.IsLinkLocalUnicast() || ip.IsLinkLocalMulticast() {
			return fmt.Errorf("alerting channel URL resolves to a private/internal address — not allowed")
		}
		if ip.Equal(net.ParseIP("169.254.169.254")) {
			return fmt.Errorf("alerting channel URL resolves to cloud metadata service — not allowed")
		}
	}
	return nil
}

// ListChannels returns all notification channels for the org.
func (s *Service) ListChannels(ctx context.Context, orgID string) ([]Channel, error) {
	return s.repo.ListChannels(ctx, orgID)
}

// CreateChannel encrypts the URL, generates an HMAC secret, and stores a new notification channel.
// It returns the created channel, the plaintext hex HMAC secret (shown once), and any error.
func (s *Service) CreateChannel(ctx context.Context, orgID string, in CreateChannelInput) (*Channel, string, error) {
	// SSRF guard: validate webhook/slack/teams URLs before storing.
	// Email-type channels store an email address, not a URL — skip URL validation.
	if in.Type != "email" {
		if err := validateAlertingURL(in.URL); err != nil {
			return nil, "", fmt.Errorf("channel URL rejected: %w", err)
		}
	}

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
		return s.sendEmailCtx(ctx, to, "Vakt Test Alert", "Dies ist eine Test-Benachrichtigung von Vakt.")
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

// FireResult reports what one Fire dispatch actually achieved. It exists so a
// caller can tell "nobody was told" apart from "somebody was told" — the old
// Fire returned nothing at all, so every caller had to assume success
// (ADR-0083).
type FireResult struct {
	// Channels is how many enabled channels were subscribed to the event.
	Channels int
	// Sent is how many of them accepted the message inside the dispatch budget.
	Sent int
	// Failed is how many refused it, errored, or ran out of budget.
	Failed int
	// TimedOut reports that the fan-out was still running when the hard wait
	// guard expired. Channels unaccounted for at that moment count as neither
	// sent nor failed, so Sent stays honest.
	TimedOut bool
	// LookupErr is set when the channel list could not be read at all. No
	// delivery was attempted in that case.
	LookupErr error
}

// Delivered reports whether at least one channel accepted the message.
//
// Deliberately not "no error occurred": an org with zero configured channels
// produces no error and no delivery, and must not count as delivered — nobody
// was told. Callers that suppress repeat alerts hang their suppression mark on
// this, so it has to mean "somebody received it", nothing weaker.
func (r FireResult) Delivered() bool { return r.Sent > 0 }

// Fire dispatches an event to all enabled channels subscribed to that event and
// reports whether anything got through.
//
// It blocks until every channel has finished or the dispatch budget (30 s by
// default) runs out, whichever comes first — a hard wait guard on top of that
// makes sure a channel that ignores its context can never pin the caller.
// Deliveries run concurrently, bounded to 10 at a time. Individual delivery
// failures are non-fatal and are recorded in alert_delivery_log.
func (s *Service) Fire(ctx context.Context, orgID, event string, payload map[string]any) FireResult {
	channels, err := s.repo.GetEnabledChannelsForEvent(ctx, orgID, event)
	if err != nil {
		log.Error().Err(err).Str("event", event).Str("org_id", orgID).Msg("alerting: get channels failed")
		return FireResult{LookupErr: err}
	}
	res := FireResult{Channels: len(channels)}
	if len(channels) == 0 {
		// No subscriber is not an error, but it is also not a delivery.
		log.Warn().Str("event", event).Str("org_id", orgID).
			Msg("alerting: no enabled channel subscribed — event not delivered to anyone")
		return res
	}

	body, _ := json.Marshal(payload)

	budget := s.dispatchBudget
	if budget <= 0 {
		budget = defaultDispatchBudget
	}

	// ADR-0018: die Zustellungen laufen über safego.Run. Der Parent-Context
	// wird durchgereicht, das Zeitlimit hängt an context.WithoutCancel(ctx) —
	// so überlebt das Auffächern einen Request-Cancel und der Aufrufer bekommt
	// trotzdem ein Ergebnis, statt es an einem abgebrochenen Request zu
	// verlieren.
	fireCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), budget)
	defer cancel()

	var sent, failed atomic.Int64
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
				failed.Add(1)
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
				err := s.sendEmailCtx(fireCtx, to, subject, emailBody)
				st := "sent"
				if err != nil {
					log.Error().Err(err).Str("channel_id", ch.ID).Msg("alerting: email delivery failed")
					st = "failed"
					failed.Add(1)
				} else {
					sent.Add(1)
				}
				_ = s.repo.LogDelivery(fireCtx, orgID, &ch.ID, event, st, nil, payload)
				return nil
			}

			// Format payload according to channel type.
			var bodyBytes []byte
			switch ch.Type {
			case "slack":
				text := formatEventText(event, payload)
				bodyBytes, _ = json.Marshal(slackMessage{
					Text: text,
					Attachments: []slackAttachment{{
						Color:  severityColor(event),
						Fields: payloadToFields(payload),
						Footer: "Vakt",
						TS:     time.Now().Unix(),
					}},
				})
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
				failed.Add(1)
			} else {
				sent.Add(1)
			}
			_ = s.repo.LogDelivery(fireCtx, orgID, &ch.ID, event, status, responseCode, payload)
			return nil
		})
	}

	// Harte Wartegrenze. Jeder Zustellpfad hängt am fireCtx und ist damit
	// schon durch das Zeitlimit begrenzt; die Grenze hier greift nur, falls
	// ein Pfad seinen Context ignoriert. Ein Cronjob darf an einem hängenden
	// Mailserver nicht seinen Worker blockieren — deshalb warten wir nie
	// unbegrenzt, sondern geben zurück, was bis dahin bestätigt ist.
	done := make(chan struct{})
	safego.Run(fireCtx, "alerting.fanout.wait", func(context.Context) error {
		wg.Wait()
		close(done)
		return nil
	})

	timer := time.NewTimer(budget + dispatchGrace)
	defer timer.Stop()
	select {
	case <-done:
	case <-timer.C:
		res.TimedOut = true
		log.Error().Str("event", event).Str("org_id", orgID).Dur("budget", budget).
			Msg("alerting: fan-out exceeded its wait guard — result reported from confirmed deliveries only")
	}

	res.Sent = int(sent.Load())
	res.Failed = int(failed.Load())
	if !res.Delivered() {
		log.Error().Str("event", event).Str("org_id", orgID).
			Int("channels", res.Channels).Int("failed", res.Failed).Bool("timed_out", res.TimedOut).
			Msg("alerting: event reached no channel")
	}
	return res
}

// FireAndMark dispatches the event and records the repeat-suppression mark in
// notification_alert_state only if at least one channel accepted it.
//
// The three daily cron checks (SLA overdue, AVV expired, DSR overdue) used to
// set that mark unconditionally right after the old fire-and-forget Fire
// returned — which it did before anything had been delivered. A broken mail
// server or webhook therefore bought 24 hours of silence per event, and with a
// permanent delivery fault the alert never came back at all (ADR-0083).
func (s *Service) FireAndMark(ctx context.Context, orgID, event string, payload map[string]any) FireResult {
	res := s.Fire(ctx, orgID, event, payload)
	if !res.Delivered() {
		log.Warn().Str("event", event).Str("org_id", orgID).
			Msg("alerting: suppression mark NOT set — the next cron run will try again")
		return res
	}
	if err := s.repo.MarkFired(ctx, orgID, event); err != nil {
		// Der Alarm ist raus; nur die Sperre fehlt. Der nächste Lauf meldet
		// dasselbe Ereignis erneut — laut statt still, die sichere Richtung.
		log.Error().Err(err).Str("event", event).Str("org_id", orgID).
			Msg("alerting: could not record suppression mark")
	}
	return res
}

// sendEmailCtx bounds sendEmail by ctx.
//
// net/smtp honours no context: smtp.SendMail against a mail server that accepts
// the TCP connection and then stops answering blocks forever. That was
// tolerable while Fire was fire-and-forget (it only leaked a goroutine); now
// that the caller waits for the result, an unbounded send would pin a cron
// worker. The send itself is left alone — reimplementing dial/STARTTLS to get a
// deadline would risk the mail path for no gain here. The goroutine may outlive
// this call; the buffered channel keeps it from blocking on a receiver that
// already gave up.
func (s *Service) sendEmailCtx(ctx context.Context, to, subject, body string) error {
	done := make(chan error, 1)
	safego.Run(ctx, "alerting.smtp.send", func(context.Context) error {
		done <- s.sendEmail(to, subject, body)
		return nil
	})
	select {
	case err := <-done:
		return err
	case <-ctx.Done():
		return fmt.Errorf("smtp delivery exceeded the dispatch budget: %w", ctx.Err())
	}
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
	// S122-C5 (D10): strip CR/LF from every header field before assembly.
	// `subject` embeds formatEventText(event, payload), so an attacker-controlled
	// finding/incident title carrying "\r\n" could inject extra SMTP headers
	// (Bcc:, a forged body, …). This is the exact CRLF header-injection class
	// S120-3 closed in the form-handler; the variant here was never chased down.
	from = mailhdr.Sanitize(from)
	to = mailhdr.Sanitize(to)
	subject = mailhdr.Sanitize(subject)
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

// slackField is one key/value row inside a Slack attachment.
type slackField struct {
	Title string `json:"title"`
	Value string `json:"value"`
	Short bool   `json:"short"`
}

// slackAttachment is the coloured block Slack renders under the message text.
type slackAttachment struct {
	Color  string       `json:"color"`
	Fields []slackField `json:"fields"`
	Footer string       `json:"footer"`
	TS     int64        `json:"ts"`
}

// slackMessage is the body posted to a Slack incoming webhook.
type slackMessage struct {
	Text        string            `json:"text"`
	Attachments []slackAttachment `json:"attachments"`
}

// payloadToFields converts a payload map to Slack attachment fields.
func payloadToFields(payload map[string]any) []slackField {
	var fields []slackField
	for k, v := range payload {
		if k == "message" {
			continue
		}
		fields = append(fields, slackField{
			Title: k,
			Value: fmt.Sprint(v),
			Short: true,
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
