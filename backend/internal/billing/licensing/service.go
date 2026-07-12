// Copyright (c) 2026 NorvikOps. All rights reserved.
// SPDX-License-Identifier: Elastic-2.0
// See LICENSE file in the project root for terms.

// Package licensing issues Vakt Pro license keys and mails them out.
//
// It is the single issuance path for the invoice/direct sale (Lexware) and the
// admin CLI. The Polar webhook keeps its own persistence, but shares the two
// things that must never diverge: the Pro feature set (features.ProTier) and the
// key lifetime (license.KeyExpiry). A customer who paid by bank transfer must
// get exactly the license a customer who paid by card gets.
package licensing

import (
	"encoding/base64"
	"fmt"
	"net/smtp"
	"strings"

	"github.com/matharnica/vakt/internal/license"
	"github.com/matharnica/vakt/internal/shared/mailhdr"
	"github.com/matharnica/vakt/internal/shared/platform/features"
)

// SMTPConfig mirrors the mail settings used elsewhere on the billing instance.
type SMTPConfig struct {
	Host string
	Port string
	User string
	Pass string
	From string
	// ReplyTo is where a customer lands when they hit "Reply" on an invoice.
	// The From address is pinned to the SMTP login (Proton only sends as
	// addresses that exist in the account), so this is how we stay reachable
	// without provisioning a second mailbox. Empty = header omitted.
	ReplyTo string
}

// Issuer signs and delivers license keys.
type Issuer struct {
	privateKeyPEM string
	smtp          SMTPConfig
}

func NewIssuer(privateKeyPEM string, smtpCfg SMTPConfig) *Issuer {
	return &Issuer{privateKeyPEM: privateKeyPEM, smtp: smtpCfg}
}

// Enabled reports whether key signing is configured. On a customer's
// self-hosted instance the signing key is absent and issuance must not be
// attempted — only the billing instance holds it.
func (i *Issuer) Enabled() bool { return i != nil && i.privateKeyPEM != "" }

// Request describes one license to issue.
type Request struct {
	OrgName  string // shown inside the key; also the customer's company name
	Email    string
	Interval string // "year" or "month"
	Trial    bool   // 45-day key issued at invoice time, before payment lands
}

// Sign produces the license key without sending mail. Used by the CLI, and by
// Issue below.
func (i *Issuer) Sign(r Request) (string, error) {
	if !i.Enabled() {
		return "", fmt.Errorf("licensing: no signing key configured (VAKT_LICENSE_PRIVATE_KEY)")
	}
	org := strings.TrimSpace(r.OrgName)
	if org == "" {
		org = r.Email
	}
	status := ""
	if r.Trial {
		status = "trialing"
	}
	expiry := license.KeyExpiry(r.Interval, status)
	return license.Sign(i.privateKeyPEM, "pro", org, features.ProTier, &expiry)
}

// Issue signs a key and mails it to the customer. If invoicePDF is non-empty it
// is attached — so the invoice and the key that unlocks the product arrive in
// one message, from us, rather than the customer chasing two senders.
func (i *Issuer) Issue(r Request, invoicePDF []byte, invoiceName string) (string, error) {
	key, err := i.Sign(r)
	if err != nil {
		return "", err
	}
	if err := i.sendMail(r, key, invoicePDF, invoiceName); err != nil {
		// The key is signed and valid at this point; the caller must persist it
		// even though delivery failed, otherwise a retry would mint a second key.
		return key, fmt.Errorf("send license mail: %w", err)
	}
	return key, nil
}

func (i *Issuer) sendMail(r Request, key string, pdf []byte, pdfName string) error {
	subject := "Deine Vakt Pro Lizenz"
	intro := "vielen Dank für deinen Auftrag. Anbei findest du deine Rechnung und deinen Vakt Pro Lizenzschlüssel."
	if r.Trial {
		subject = "Deine Vakt Pro Lizenz (45 Tage, bis zum Zahlungseingang)"
		intro = "vielen Dank für deinen Auftrag. Anbei findest du deine Rechnung.\r\n\r\n" +
			"Damit du sofort loslegen kannst, liegt schon jetzt ein Lizenzschlüssel bei — er läuft 45 Tage. " +
			"Sobald deine Zahlung eingegangen ist, bekommst du automatisch den Schlüssel mit voller Jahreslaufzeit. " +
			"Du musst dafür nichts tun."
	}

	body := fmt.Sprintf(`Hallo,

%s

Dein Lizenzschlüssel:

%s

So aktivierst du ihn:
  1. In deiner Vakt-Instanz auf "Einstellungen" → "Lizenz"
  2. Schlüssel einfügen, speichern — fertig.

Alternativ per Umgebungsvariable in der .env deiner Instanz:
  VAKT_LICENSE_KEY=%s

Fragen? Antworte einfach auf diese Mail.

Viele Grüße
Stefan
Norvik Ops
`, intro, key, key)

	// Every header value that can carry attacker- or customer-supplied text goes
	// through mailhdr.Sanitize. The company name arrives from a public web form:
	// an unsanitised "\r\nBcc:" in it would turn this into an open relay.
	from := mailhdr.Sanitize(i.smtp.From)
	to := mailhdr.Sanitize(r.Email)
	subj := mailhdr.Sanitize(subject)
	replyTo := ""
	if i.smtp.ReplyTo != "" {
		replyTo = "Reply-To: " + mailhdr.Sanitize(i.smtp.ReplyTo) + "\r\n"
	}

	var msg strings.Builder
	if len(pdf) > 0 {
		const boundary = "vakt-license-boundary-8f3a1c"
		msg.WriteString("From: " + from + "\r\n")
		msg.WriteString("To: " + to + "\r\n")
		msg.WriteString(replyTo)
		msg.WriteString("Subject: " + subj + "\r\n")
		msg.WriteString("MIME-Version: 1.0\r\n")
		msg.WriteString("Content-Type: multipart/mixed; boundary=\"" + boundary + "\"\r\n\r\n")

		msg.WriteString("--" + boundary + "\r\n")
		msg.WriteString("Content-Type: text/plain; charset=\"utf-8\"\r\n\r\n")
		msg.WriteString(body + "\r\n")

		msg.WriteString("--" + boundary + "\r\n")
		msg.WriteString("Content-Type: application/pdf\r\n")
		msg.WriteString("Content-Transfer-Encoding: base64\r\n")
		msg.WriteString("Content-Disposition: attachment; filename=\"" + mailhdr.Sanitize(pdfName) + "\"\r\n\r\n")
		msg.WriteString(chunk76(base64.StdEncoding.EncodeToString(pdf)))
		msg.WriteString("\r\n--" + boundary + "--\r\n")
	} else {
		msg.WriteString("From: " + from + "\r\n")
		msg.WriteString("To: " + to + "\r\n")
		msg.WriteString(replyTo)
		msg.WriteString("Subject: " + subj + "\r\n")
		msg.WriteString("MIME-Version: 1.0\r\n")
		msg.WriteString("Content-Type: text/plain; charset=\"utf-8\"\r\n\r\n")
		msg.WriteString(body)
	}

	var auth smtp.Auth
	if i.smtp.User != "" {
		auth = smtp.PlainAuth("", i.smtp.User, i.smtp.Pass, i.smtp.Host)
	}
	return smtp.SendMail(i.smtp.Host+":"+i.smtp.Port, auth, i.smtp.From, []string{r.Email}, []byte(msg.String()))
}

// chunk76 wraps base64 at 76 characters, as RFC 2045 requires. Some MTAs reject
// or mangle longer lines, which would corrupt the attached invoice.
func chunk76(s string) string {
	var b strings.Builder
	for len(s) > 76 {
		b.WriteString(s[:76])
		b.WriteString("\r\n")
		s = s[76:]
	}
	b.WriteString(s)
	return b.String()
}
