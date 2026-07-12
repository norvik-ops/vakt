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
	"time"

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

	// RenewalToken lets the customer's instance fetch its next key by itself.
	//
	// Without it the mail hands over a key that expires in 395 days and nothing
	// else — the customer would have to paste a new one by hand, while
	// .env.example and the docs promise "kein manueller Eingriff bei
	// Verlängerungen". The token used to exist only for customers who bought
	// through Polar; the invoice flow shipped without one.
	//
	// Empty is allowed (the admin CLI signs keys without a quote request behind
	// them) — the mail then simply omits the auto-renewal section.
	RenewalToken string
}

// SignUntil produces a key that expires exactly when the caller says.
//
// Real sales use this, not Sign: the expiry must be capped at the period the customer
// actually paid for (lexware.Entitlement). Deriving it from the interval instead would
// let a customer who stopped paying keep renewing forever — the subscription's status
// stays "paid" once it has ever been paid.
func (i *Issuer) SignUntil(r Request, expires time.Time) (string, error) {
	if !i.Enabled() {
		return "", fmt.Errorf("licensing: no signing key configured (VAKT_LICENSE_PRIVATE_KEY)")
	}
	org := strings.TrimSpace(r.OrgName)
	if org == "" {
		org = r.Email
	}
	return license.SignWithToken(i.privateKeyPEM, "pro", org, r.RenewalToken, features.ProTier, &expires)
}

// Sign produces the license key without sending mail. Used by the CLI, and by
// Issue below.
func (i *Issuer) Sign(r Request) (string, error) {
	key, _, err := i.sign(r)
	return key, err
}

// sign returns the key AND the moment it stops working.
//
// Issue needs both, because the mail names the expiry date. Deriving that date a
// second time inside sendMail would be a second place to get it wrong — and the
// mail would eventually promise something the key does not do.
func (i *Issuer) sign(r Request) (string, time.Time, error) {
	if !i.Enabled() {
		return "", time.Time{}, fmt.Errorf("licensing: no signing key configured (VAKT_LICENSE_PRIVATE_KEY)")
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
	key, err := license.SignWithToken(i.privateKeyPEM, "pro", org, r.RenewalToken, features.ProTier, &expiry)
	return key, expiry, err
}

// termOf names the period the customer actually bought.
//
// The mail used to say "volle Jahreslaufzeit" no matter what was sold. A customer on
// the Monatslizenz paid 299 €, read that he was getting a full year, and received a
// key that dies after 30 days. The Interval sat in the Request the whole time — the
// text simply ignored it.
//
// The branch mirrors license.KeyExpiry exactly (year, else month). If the two ever
// disagree, the mail is the one that lies.
func termOf(interval string) string {
	if interval == "year" {
		return "ein volles Jahr"
	}
	return "einen vollen Monat"
}

// IssueUntil signs a key with an explicit expiry and mails it. This is what a real
// sale uses — the expiry is capped at the period the customer actually paid for.
func (i *Issuer) IssueUntil(r Request, expires time.Time, invoicePDF []byte, invoiceName string) (string, error) {
	key, err := i.SignUntil(r, expires)
	if err != nil {
		return "", err
	}
	if err := i.sendMail(r, key, expires, invoicePDF, invoiceName); err != nil {
		return key, err
	}
	return key, nil
}

// Issue signs a key and mails it to the customer. If invoicePDF is non-empty it
// is attached — so the invoice and the key that unlocks the product arrive in
// one message, from us, rather than the customer chasing two senders.
func (i *Issuer) Issue(r Request, invoicePDF []byte, invoiceName string) (string, error) {
	key, expires, err := i.sign(r)
	if err != nil {
		return "", err
	}
	if err := i.sendMail(r, key, expires, invoicePDF, invoiceName); err != nil {
		// The key is signed and valid at this point; the caller must persist it
		// even though delivery failed, otherwise a retry would mint a second key.
		return key, fmt.Errorf("send license mail: %w", err)
	}
	return key, nil
}

func (i *Issuer) sendMail(r Request, key string, expires time.Time, pdf []byte, pdfName string) error {
	subject, body := licenseMail(r, key, expires)
	return i.Send(r.Email, subject, body, pdf, pdfName)
}

// licenseMail builds the mail. Pure on purpose: it is the only claim we make to a
// paying customer about what he just bought, and a function that needs an SMTP
// server to run is a function nobody tests.
func licenseMail(r Request, key string, expires time.Time) (subject, body string) {
	// Zwei Mails, zwei Texte. Der Nicht-Trial-Fall ist die Mail NACH dem
	// Zahlungseingang — sie traegt keinen Anhang mehr, weil die Rechnung schon
	// mit der ersten Mail rausging. Ein "anbei findest du deine Rechnung" haette
	// den Kunden vergeblich nach einem Anhang suchen lassen.
	//
	// Beide nennen das echte Ablaufdatum des mitgeschickten Schluessels. Eine
	// Laufzeit-Floskel ("volle Jahreslaufzeit") stand hier frueher fest im Text und
	// war fuer jeden Monatskunden schlicht falsch; ein Datum kann gar nicht erst
	// vom ausgestellten Schluessel abweichen, weil es aus ihm stammt.
	until := expires.Format("02.01.2006")

	subject = "Deine Vakt Pro Lizenz — Zahlung eingegangen"
	intro := "deine Zahlung ist eingegangen — vielen Dank. Hier ist dein Lizenzschlüssel für " + termOf(r.Interval) + ".\r\n\r\n" +
		"Er ersetzt den 45-Tage-Schlüssel aus der Auftragsbestätigung: einfach in deiner Instanz eintragen, dann läuft die Lizenz bis zum " + until + "."
	if r.Trial {
		subject = "Deine Vakt Pro Lizenz (45 Tage, bis zum Zahlungseingang)"
		intro = "vielen Dank für deinen Auftrag. Anbei findest du deine Rechnung.\r\n\r\n" +
			"Damit du sofort loslegen kannst, liegt schon jetzt ein Lizenzschlüssel bei — er läuft bis zum " + until + ". " +
			"Sobald deine Zahlung eingegangen ist, bekommst du automatisch den Schlüssel für " + termOf(r.Interval) + ". " +
			"Du musst dafür nichts tun."
	}

	// Auto-renewal. Only meaningful when the key came from a quote request — the
	// CLI signs keys that have no row to renew against.
	renewal := ""
	if r.RenewalToken != "" {
		renewal = fmt.Sprintf(`
Damit sich die Lizenz künftig von selbst verlängert, trage zusätzlich ein:
  VAKT_LICENSE_TOKEN=%s

Deine Instanz holt sich damit einmal täglich den aktuellen Schlüssel. Bei einer
Verlängerung musst du dann nichts mehr eintragen. Übertragen wird ausschließlich
dieser Token — keine Daten aus deiner Instanz. Der Token ist optional; ohne ihn
funktioniert alles wie gewohnt, nur der Schlüsselwechsel bleibt manuell.
`, r.RenewalToken)
	}

	body = fmt.Sprintf(`Hallo,

%s

Dein Lizenzschlüssel:

%s

So aktivierst du ihn:
  1. In deiner Vakt-Instanz auf "Einstellungen" → "Lizenz"
  2. Schlüssel einfügen, speichern — fertig.

Alternativ per Umgebungsvariable in der .env deiner Instanz:
  VAKT_LICENSE_KEY=%s
%s
Fragen? Antworte einfach auf diese Mail.

Viele Grüße
Stefan
Norvik Ops
`, intro, key, key, renewal)

	return subject, body
}

// Send delivers one mail. It is the only place that talks to SMTP.
//
// Extracted from sendMail so a renewal invoice — which carries no key, because the
// key is only issued once the money lands — reuses the exact same hardened
// delivery instead of a second copy that would drift. The header sanitising below
// is the whole reason: the company name arrives from a PUBLIC web form, and an
// unsanitised "\r\nBcc:" in it would turn this into an open relay.
func (i *Issuer) Send(toAddr, subject, body string, pdf []byte, pdfName string) error {
	from := mailhdr.Sanitize(i.smtp.From)
	to := mailhdr.Sanitize(toAddr)
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
	return smtp.SendMail(i.smtp.Host+":"+i.smtp.Port, auth, i.smtp.From, []string{toAddr}, []byte(msg.String()))
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
