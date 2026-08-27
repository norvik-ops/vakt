// Package notifications sends transactional deadline-alert emails to compliance officers.
package notifications

import (
	"errors"
	"fmt"
	"net/smtp"

	"github.com/matharnica/vakt/internal/config"
	"github.com/matharnica/vakt/internal/shared/mailhdr"
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

// ErrNotConfigured meldet, dass NICHTS gesendet wurde, weil kein SMTP-Server
// eingerichtet ist.
//
// Frueher gab Send in diesem Fall nil zurueck — "graceful no-op". Das war die
// gefaehrlichste Zeile in diesem Paket: die Aufrufer in alerts.go werten nil
// als "zugestellt" und rufen danach markSent auf, das eine Zeile mit
// EINDEUTIGEM SCHLUESSEL UND OHNE ABLAUF in notification_log schreibt. Eine
// Instanz ohne SMTP hat damit ihre Art.-33-Datenpannen-Warnung ENDGUELTIG
// verbrannt: die Meldung gilt als erledigt und wird nie wieder versucht, auch
// nicht, nachdem SMTP eingerichtet wurde. Dasselbe galt fuer ueberfaellige
// Betroffenenanfragen, ablaufende AVV und gescheiterte Compliance-Pruefungen.
//
// Der Sentinel macht den Unterschied sichtbar, den nil verschluckt hat:
// "nicht gesendet" ist kein Erfolg. Die Aufrufer ueberspringen markSent bei
// jedem Fehler, also genuegt es, die Wahrheit zurueckzugeben — sie loggen den
// Fall nur bewusst als Warnung statt als Fehler, weil eine Instanz ohne SMTP
// ein zulaessiger Betriebszustand ist und kein Stoerfall.
var ErrNotConfigured = errors.New("notifications: SMTP nicht eingerichtet — nichts gesendet")

// Send sends a plain-text email.
// Returns ErrNotConfigured — NOT nil — if no SMTP host is configured.
func (m *Mailer) Send(to, subject, body string) error {
	if m.cfg == nil || m.cfg.SMTPHost == "" || m.cfg.SMTPHost == "localhost" {
		// localhost gilt weiterhin als "nicht eingerichtet": der Wert stammt aus
		// Entwicklungs-Defaults. Diese Einstufung bleibt unveraendert — geaendert
		// wird nur, was sie MELDET.
		return ErrNotConfigured
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
		mailhdr.Sanitize(from), mailhdr.Sanitize(to), mailhdr.Sanitize(subject),
	)
	msg := []byte(headers + body)

	addr := m.cfg.SMTPHost + ":" + port

	if m.cfg.SMTPUser != "" && m.cfg.SMTPPass != "" {
		auth := smtp.PlainAuth("", m.cfg.SMTPUser, m.cfg.SMTPPass, m.cfg.SMTPHost)
		return smtp.SendMail(addr, auth, from, []string{to}, msg)
	}
	return smtp.SendMail(addr, nil, from, []string{to}, msg)
}
