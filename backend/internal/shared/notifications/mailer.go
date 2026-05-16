// Package notifications sends transactional deadline-alert emails to compliance officers.
package notifications

import (
	"fmt"
	"net/smtp"

	"github.com/sechealth-app/sechealth/internal/config"
)

// Mailer sends transactional notification emails using stdlib net/smtp.
// It mirrors the pattern used by emaildigest.DigestService.send.
type Mailer struct {
	cfg *config.Config
}

// NewMailer creates a Mailer backed by the application config.
func NewMailer(cfg *config.Config) *Mailer {
	return &Mailer{cfg: cfg}
}

// Send sends a plain-text email.
// Returns nil if SMTP host is not configured (graceful no-op).
func (m *Mailer) Send(to, subject, body string) error {
	if m.cfg == nil || m.cfg.SMTPHost == "" || m.cfg.SMTPHost == "localhost" {
		// Treat localhost-only or missing host as "not configured" for safety.
		// Callers can still rely on this being nil — no send means no error.
		return nil
	}

	from := m.cfg.SMTPFrom
	if from == "" {
		from = "vakt@" + m.cfg.SMTPHost
	}

	port := m.cfg.SMTPPort
	if port == "" {
		port = "25"
	}

	headers := fmt.Sprintf(
		"From: %s\r\nTo: %s\r\nSubject: %s\r\nMIME-Version: 1.0\r\nContent-Type: text/plain; charset=UTF-8\r\n\r\n",
		from, to, subject,
	)
	msg := []byte(headers + body)

	addr := m.cfg.SMTPHost + ":" + port

	if m.cfg.SMTPUser != "" && m.cfg.SMTPPass != "" {
		auth := smtp.PlainAuth("", m.cfg.SMTPUser, m.cfg.SMTPPass, m.cfg.SMTPHost)
		return smtp.SendMail(addr, auth, from, []string{to}, msg)
	}
	return smtp.SendMail(addr, nil, from, []string{to}, msg)
}
