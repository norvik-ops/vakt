// Copyright (c) 2026 NorvikOps. All rights reserved.
// SPDX-License-Identifier: Elastic-2.0
// See LICENSE file in the project root for terms.

package lexware

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/hex"
	"fmt"
	"net/http"
	"net/mail"
	"net/smtp"
	"strings"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/labstack/echo/v4"
	"github.com/rs/zerolog/log"

	"github.com/matharnica/vakt/internal/billing/licensing"
	"github.com/matharnica/vakt/internal/shared/logsafe"
	"github.com/matharnica/vakt/internal/shared/mailhdr"
)

// ProNetAmountEUR is the yearly Pro price. Net, because as a §19 small business
// no VAT is charged — the invoice carries Lexware's stored § 19 note instead.
const ProNetAmountEUR = 2990.0

// Handler serves the direct-sale flow: quote request -> human approval ->
// invoice -> payment -> license.
type Handler struct {
	db       *pgxpool.Pool
	client   *Client
	issuer   *licensing.Issuer
	smtp     licensing.SMTPConfig
	baseURL  string // public URL of this billing API, for the approval link
	notifyTo string // where the "new request, approve?" mail goes
}

func NewHandler(db *pgxpool.Pool, c *Client, iss *licensing.Issuer, smtpCfg licensing.SMTPConfig, baseURL, notifyTo string) *Handler {
	return &Handler{db: db, client: c, issuer: iss, smtp: smtpCfg, baseURL: strings.TrimRight(baseURL, "/"), notifyTo: notifyTo}
}

// ── 1. Public: request a quote ───────────────────────────────────────────────

type quoteRequestInput struct {
	CompanyName string `json:"company_name"`
	ContactName string `json:"contact_name"`
	Email       string `json:"email"`
	VATID       string `json:"vat_id"`
	Street      string `json:"street"`
	Zip         string `json:"zip"`
	City        string `json:"city"`
	CountryCode string `json:"country_code"`
	Note        string `json:"note"`
	Interval    string `json:"interval"` // year | month
	Website     string `json:"website"`  // honeypot — humans never fill this
}

// RequestQuote accepts a quote request from the public pricing page.
//
// It deliberately does NOT create an invoice. A public endpoint that mints
// finalized invoices under our tax number would let anyone flood the books with
// junk — and an invoice, once finalized, cannot be un-finalized through the API.
// Instead we store the request and mail a one-click approval link to the seller.
func (h *Handler) RequestQuote(c echo.Context) error {
	var in quoteRequestInput
	if err := c.Bind(&in); err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "invalid request"})
	}

	// Honeypot: bots fill every field they find. Answer 200 so they learn nothing.
	if strings.TrimSpace(in.Website) != "" {
		return c.JSON(http.StatusOK, map[string]bool{"ok": true})
	}

	in.CompanyName = strings.TrimSpace(in.CompanyName)
	in.Email = strings.TrimSpace(in.Email)
	if in.CompanyName == "" || in.Email == "" {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "company_name and email are required"})
	}
	if _, err := mail.ParseAddress(in.Email); err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "invalid email"})
	}
	if len(in.CompanyName) > 200 || len(in.Note) > 2000 {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "field too long"})
	}
	if in.Interval != "month" {
		in.Interval = "year"
	}
	if in.CountryCode == "" {
		in.CountryCode = "DE"
	}

	// The approval token is only ever stored hashed. A leaked database backup
	// must not let anyone approve invoices.
	tokenBytes := make([]byte, 32)
	if _, err := rand.Read(tokenBytes); err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "internal error"})
	}
	token := hex.EncodeToString(tokenBytes)
	sum := sha256.Sum256([]byte(token))

	ctx := c.Request().Context()
	var id string
	err := h.db.QueryRow(ctx, `
		INSERT INTO billing_quote_requests
			(company_name, contact_name, email, vat_id, street, zip, city, country_code, note, interval, approval_token_hash)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11)
		RETURNING id`,
		in.CompanyName, in.ContactName, in.Email, in.VATID, in.Street, in.Zip, in.City,
		in.CountryCode, in.Note, in.Interval, hex.EncodeToString(sum[:]),
	).Scan(&id)
	if err != nil {
		log.Error().Err(err).Msg("billing: persist quote request")
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "internal error"})
	}

	if err := h.notifySeller(id, token, in); err != nil {
		// The request is safely stored; a failed notification must not lose it.
		log.Error().Err(err).Str("request_id", id).Msg("billing: notify seller failed")
	}

	log.Info().
		Str("request_id", id).
		Str("email_redacted", logsafe.RedactEmail(in.Email)).
		Msg("billing: quote requested")

	return c.JSON(http.StatusOK, map[string]bool{"ok": true})
}

func (h *Handler) notifySeller(id, token string, in quoteRequestInput) error {
	link := fmt.Sprintf("%s/api/v1/billing/quote-request/%s/approve?token=%s", h.baseURL, id, token)

	body := fmt.Sprintf(`Neue Angebotsanfrage für Vakt Pro.

  Firma        : %s
  Ansprechpartner: %s
  E-Mail       : %s
  USt-IdNr.    : %s
  Adresse      : %s, %s %s (%s)
  Laufzeit     : %s
  Anmerkung    : %s

Prüfe kurz, ob das eine echte Firma ist. Wenn ja, hier klicken — das legt den
Kontakt in Lexware an, erstellt eine FINALISIERTE Rechnung, schickt sie dem
Kunden mit einem 45-Tage-Lizenzschlüssel und ist nicht mehr rückgängig zu machen:

%s

Sobald das Geld da ist: Zahlung in Lexware zuordnen. Den Rest (Vollkey über 395
Tage, Versand) macht Vakt automatisch.
`,
		in.CompanyName, in.ContactName, in.Email, in.VATID,
		in.Street, in.Zip, in.City, in.CountryCode, in.Interval, in.Note, link)

	// Header fields carry form input -> sanitise. A "\r\nBcc:" in a company name
	// would otherwise turn this notification into a relay.
	msg := "From: " + mailhdr.Sanitize(h.smtp.From) + "\r\n" +
		"To: " + mailhdr.Sanitize(h.notifyTo) + "\r\n" +
		"Subject: " + mailhdr.Sanitize("[Vakt] Angebotsanfrage: "+in.CompanyName) + "\r\n" +
		"Content-Type: text/plain; charset=\"utf-8\"\r\n\r\n" + body

	var auth smtp.Auth
	if h.smtp.User != "" {
		auth = smtp.PlainAuth("", h.smtp.User, h.smtp.Pass, h.smtp.Host)
	}
	return smtp.SendMail(h.smtp.Host+":"+h.smtp.Port, auth, h.smtp.From, []string{h.notifyTo}, []byte(msg))
}

// ── 2. Seller approves: create contact + invoice, send it with a trial key ────

// Approve is reached from the one-click link in the notification mail.
//
// Guarded by a 32-byte token compared in constant time against a stored hash.
// It is idempotent: a second click on an already-approved request returns the
// same answer instead of issuing a second invoice — mail clients prefetch links,
// and an accidental double-invoice is a real-world annoyance for the customer.
func (h *Handler) Approve(c echo.Context) error {
	id := c.Param("id")
	token := c.QueryParam("token")
	if id == "" || token == "" {
		return c.String(http.StatusBadRequest, "fehlender Token")
	}

	ctx := c.Request().Context()
	var (
		storedHash, status, company, contact, email, vatID string
		street, zip, city, country, interval               string
	)
	err := h.db.QueryRow(ctx, `
		SELECT approval_token_hash, status, company_name, contact_name, email, vat_id,
		       street, zip, city, country_code, interval
		  FROM billing_quote_requests WHERE id = $1`, id).
		Scan(&storedHash, &status, &company, &contact, &email, &vatID, &street, &zip, &city, &country, &interval)
	if err != nil {
		return c.String(http.StatusNotFound, "Anfrage nicht gefunden")
	}

	sum := sha256.Sum256([]byte(token))
	if subtle.ConstantTimeCompare([]byte(hex.EncodeToString(sum[:])), []byte(storedHash)) != 1 {
		log.Warn().Str("request_id", id).Msg("billing: approval token mismatch")
		return c.String(http.StatusForbidden, "Token ungültig")
	}
	if status != "requested" {
		return c.String(http.StatusOK, "Diese Anfrage wurde bereits bearbeitet (Status: "+status+"). Es wurde nichts erneut erstellt.")
	}
	if !h.client.Enabled() || !h.issuer.Enabled() {
		return c.String(http.StatusServiceUnavailable, "Billing ist auf dieser Instanz nicht konfiguriert")
	}

	contactID, err := h.client.CreateContact(ctx, ContactInput{
		CompanyName: company, VATID: vatID, ContactName: contact, Email: email,
		Street: street, Zip: zip, City: city, CountryCode: country,
	})
	if err != nil {
		log.Error().Err(err).Str("request_id", id).Msg("billing: create lexware contact")
		// 200, nicht 502: Diese Seite liest ein MENSCH. Cloudflare ersetzt 5xx
		// durch seine eigene "Bad gateway"-Seite, und die Fehlerursache — die
		// hier steht — ginge verloren.
		return c.String(http.StatusOK, "FEHLER: Lexware-Kontakt konnte nicht angelegt werden.\n\n"+err.Error()+"\n\nEs wurde nichts erstellt. Der Link bleibt gültig — nach dem Fix erneut klicken.")
	}

	amount := ProNetAmountEUR
	title := "Vakt Pro — Jahreslizenz"
	if interval == "month" {
		amount = 299.0
		title = "Vakt Pro — Monatslizenz"
	}

	invoiceID, err := h.client.CreateInvoice(ctx, InvoiceInput{
		ContactID:   contactID,
		Title:       title,
		Intro:       "vielen Dank für Ihren Auftrag. Wir stellen Ihnen wie vereinbart in Rechnung:",
		Description: "Vakt Pro — self-hosted ISMS-Plattform, unbegrenzte Nutzer",
		NetAmount:   amount,
		DueInDays:   14,
	})
	if err != nil {
		log.Error().Err(err).Str("request_id", id).Msg("billing: create lexware invoice")
		return c.String(http.StatusOK, "FEHLER: Rechnung konnte nicht erstellt werden.\n\n"+err.Error()+"\n\nDer Kontakt wurde in Lexware angelegt, die Rechnung nicht. Der Link bleibt gültig — nach dem Fix erneut klicken.")
	}

	pdf, err := h.client.InvoicePDF(ctx, invoiceID)
	if err != nil {
		// Non-fatal: the invoice exists in Lexware either way. Better to send the
		// key without the PDF than to leave the customer with nothing.
		log.Error().Err(err).Str("invoice_id", invoiceID).Msg("billing: fetch invoice pdf")
		pdf = nil
	}

	// The customer gets a 45-day key straight away. Making a B2B buyer wait days
	// for a bank transfer to clear before they can even start would be a strange
	// way to sell software.
	key, mailErr := h.issuer.Issue(licensing.Request{
		OrgName: company, Email: email, Interval: interval, Trial: true,
	}, pdf, "Rechnung-Vakt-Pro.pdf")

	if _, dbErr := h.db.Exec(ctx, `
		UPDATE billing_quote_requests
		   SET status = 'approved', lexware_contact_id = $2, lexware_invoice_id = $3,
		       license_key = $4, approved_at = NOW()
		 WHERE id = $1`, id, contactID, invoiceID, key); dbErr != nil {
		// The invoice is out and cannot be recalled — losing the link between it
		// and this request would orphan the payment webhook. Loud failure.
		log.Error().Err(dbErr).Str("request_id", id).Str("invoice_id", invoiceID).
			Msg("billing: CRITICAL — invoice sent but request not updated")
		return c.String(http.StatusInternalServerError,
			"Rechnung wurde erstellt ("+invoiceID+"), aber die Anfrage konnte nicht aktualisiert werden. Bitte manuell prüfen.")
	}

	if mailErr != nil {
		log.Error().Err(mailErr).Str("request_id", id).Msg("billing: license mail failed")
		return c.String(http.StatusOK,
			"Rechnung "+invoiceID+" wurde erstellt, aber die E-Mail an den Kunden ist fehlgeschlagen. Bitte manuell versenden.")
	}

	log.Info().Str("request_id", id).Str("invoice_id", invoiceID).Msg("billing: invoice sent, trial key issued")
	return c.String(http.StatusOK,
		"Erledigt.\n\nRechnung "+invoiceID+" wurde finalisiert und mit einem 45-Tage-Lizenzschlüssel an "+email+" geschickt.\n\n"+
			"Sobald die Zahlung eingeht: in Lexware zuordnen — den Vollkey verschickt Vakt dann automatisch.")
}

// ── 3. Lexware webhook: payment landed ───────────────────────────────────────

// Webhook receives payment.changed.
//
// The payload is treated as an UNTRUSTED HINT and nothing more. It carries only
// a resource ID, so we go and ask the Lexware API what actually happened. Two
// attacks and one accident are defeated by that alone:
//
//   - A forged webhook cannot mint a license: the API would report the invoice
//     as unpaid.
//   - A replayed webhook cannot mint a second license: status is already 'paid'.
//   - A PARTIAL payment cannot mint a license: payment.changed fires for those
//     too, so we require paymentStatus == "balanced" rather than assuming the
//     event means "settled". A 100 € transfer must not unlock a 2.990 € licence.
//
// Lexware retries on non-2xx, so transient failures are safe to surface.
func (h *Handler) Webhook(c echo.Context) error {
	var ev WebhookEvent
	if err := c.Bind(&ev); err != nil {
		return c.NoContent(http.StatusBadRequest)
	}
	if ev.EventType != EventPaymentChanged || ev.ResourceID == "" {
		return c.NoContent(http.StatusOK) // not ours; ack so Lexware stops retrying
	}
	if !h.client.Enabled() || !h.issuer.Enabled() {
		return c.NoContent(http.StatusOK)
	}

	// Lexware's read timeout is 5s. Do the work on our own clock, not theirs.
	go h.settle(context.WithoutCancel(c.Request().Context()), ev.ResourceID)
	return c.NoContent(http.StatusOK)
}

func (h *Handler) settle(ctx context.Context, invoiceID string) {
	ctx, cancel := context.WithTimeout(ctx, 60*time.Second)
	defer cancel()

	pay, err := h.client.PaymentStatus(ctx, invoiceID)
	if err != nil {
		log.Error().Err(err).Str("invoice_id", invoiceID).Msg("billing: read payment status")
		return
	}
	if pay == nil {
		return // 406: voucher went back to draft. Nothing to do.
	}
	if !pay.Paid() {
		log.Info().Str("invoice_id", invoiceID).Str("payment_status", pay.PaymentStatus).
			Msg("billing: payment changed but not settled — no key issued")
		return
	}

	var id, company, email, interval, status string
	err = h.db.QueryRow(ctx, `
		SELECT id, company_name, email, interval, status
		  FROM billing_quote_requests WHERE lexware_invoice_id = $1`, invoiceID).
		Scan(&id, &company, &email, &interval, &status)
	if err != nil {
		// An invoice paid in Lexware that we never issued — e.g. a manual invoice
		// for something else entirely. Not an error.
		log.Info().Str("invoice_id", invoiceID).Msg("billing: paid invoice has no quote request — ignoring")
		return
	}
	if status == "paid" {
		return // already settled; webhook replay
	}

	key, mailErr := h.issuer.Issue(licensing.Request{
		OrgName: company, Email: email, Interval: interval, Trial: false,
	}, nil, "")
	if key == "" {
		log.Error().Err(mailErr).Str("request_id", id).Msg("billing: could not sign full license key")
		return
	}

	if _, err := h.db.Exec(ctx, `
		UPDATE billing_quote_requests SET status = 'paid', license_key = $2, paid_at = NOW()
		 WHERE id = $1`, id, key); err != nil {
		log.Error().Err(err).Str("request_id", id).Msg("billing: persist paid state")
	}

	if mailErr != nil {
		log.Error().Err(mailErr).Str("request_id", id).Msg("billing: full license mail failed — send manually")
		return
	}
	log.Info().
		Str("request_id", id).
		Str("email_redacted", logsafe.RedactEmail(email)).
		Msg("billing: payment settled, full license key issued")
}
