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
	Product     string `json:"product"`  // pro (managed, msp planned) — empty means pro
	Quantity    int    `json:"quantity"` // seats; an MSP buys more than one
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

	// The form posts a product. Reject anything we do not sell right here, at the
	// public edge: an unknown value would sail through to Approve() and only fail
	// once a human had already clicked "freigeben", which is a rotten place to
	// discover it. Empty means "pro" — the form shipped before products existed.
	product := in.Product
	if product == "" {
		product = "pro"
	}
	if _, err := PlanFor(product, in.Interval); err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "unknown product or interval"})
	}

	// Seats. Bounded on both sides: 0 or negative would invoice nothing, and an
	// absurd number typed into a public form would produce an absurd invoice under
	// our tax number. 500 is far above any real MSP and far below "someone is
	// having fun with the form".
	quantity := in.Quantity
	if quantity < 1 {
		quantity = 1
	}
	if quantity > 500 {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "quantity out of range"})
	}

	ctx := c.Request().Context()
	var id string
	err := h.db.QueryRow(ctx, `
		INSERT INTO billing_quote_requests
			(company_name, contact_name, email, vat_id, street, zip, city, country_code, note,
			 product, quantity, interval, approval_token_hash)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13)
		RETURNING id`,
		in.CompanyName, in.ContactName, in.Email, in.VATID, in.Street, in.Zip, in.City,
		in.CountryCode, in.Note, product, quantity, in.Interval, hex.EncodeToString(sum[:]),
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
		street, zip, city, country, interval, product      string
		renewalToken                                       string
		quantity                                           int
	)
	err := h.db.QueryRow(ctx, `
		SELECT approval_token_hash, status, company_name, contact_name, email, vat_id,
		       street, zip, city, country_code, interval, product, quantity, renewal_token
		  FROM billing_quote_requests WHERE id = $1`, id).
		Scan(&storedHash, &status, &company, &contact, &email, &vatID, &street, &zip, &city, &country,
			&interval, &product, &quantity, &renewalToken)
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

	// Before anything is created in Lexware: is this even a thing we sell? An
	// unknown product/interval must fail HERE, not silently fall back to some
	// default amount — that would invoice a real customer the wrong price.
	plan, err := PlanFor(product, interval)
	if err != nil {
		log.Error().Err(err).Str("request_id", id).Msg("billing: unknown plan")
		return c.String(http.StatusOK,
			"FEHLER: Für diese Kombination gibt es keinen Tarif ("+product+"/"+interval+").\n\n"+
				"Es wurde nichts erstellt. Wenn das Produkt verkauft werden soll, gehört es in plans.go.")
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

	invoiceID, err := h.client.CreateInvoice(ctx, InvoiceInput{
		ContactID:   contactID,
		Title:       plan.Title,
		Intro:       "vielen Dank für Ihren Auftrag. Wir stellen Ihnen wie vereinbart in Rechnung:",
		Description: plan.LineDesc(quantity),
		NetAmount:   plan.TotalEUR(quantity),
		DueInDays:   plan.DueDays,
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
		RenewalToken: renewalToken,
	}, pdf, "Rechnung-Vakt-Pro.pdf")

	// The quote request becomes the subscription here, and the invoice gets its own
	// row: from now on there will be many invoices against this one subscription,
	// and settle() has to know which period a payment belongs to.
	//
	// Both writes in one transaction. An invoice row without its subscription
	// update (or the reverse) would either orphan the payment webhook — customer
	// pays, no key — or bill someone twice.
	from, to := plan.Period(time.Now())
	tx, dbErr := h.db.Begin(ctx)
	if dbErr == nil {
		_, dbErr = tx.Exec(ctx, `
			UPDATE billing_quote_requests
			   SET status = 'approved', lexware_contact_id = $2, lexware_invoice_id = $3,
			       license_key = $4, approved_at = NOW()
			 WHERE id = $1`, id, contactID, invoiceID, key)
		if dbErr == nil {
			_, dbErr = tx.Exec(ctx, `
				INSERT INTO billing_invoices
					(subscription_id, lexware_invoice_id, period_start, period_end, net_amount_cents, status)
				VALUES ($1, $2, $3, $4, $5, 'open')`,
				id, invoiceID, from, to, plan.TotalCents(quantity))
		}
		if dbErr == nil {
			dbErr = tx.Commit(ctx)
		} else {
			_ = tx.Rollback(ctx)
		}
	}
	if dbErr != nil {
		// The invoice is out in Lexware and cannot be recalled — losing the link
		// between it and this request would orphan the payment webhook. Loud failure.
		log.Error().Err(dbErr).Str("request_id", id).Str("invoice_id", invoiceID).
			Msg("billing: CRITICAL — invoice sent but request not updated")
		return c.String(http.StatusOK,
			"FEHLER: Rechnung "+invoiceID+" wurde in Lexware erstellt, aber die Anfrage konnte nicht "+
				"aktualisiert werden.\n\n"+dbErr.Error()+"\n\nBitte manuell prüfen — die Zahlung kann sonst "+
				"nicht zugeordnet werden.")
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
		return // 406: voucher is a draft. Nothing to do.
	}
	if !pay.Paid() {
		log.Info().Str("invoice_id", invoiceID).Str("payment_status", pay.PaymentStatus).
			Msg("billing: payment changed but not settled — no key issued")
		return
	}

	// Claim the INVOICE before doing anything irreversible.
	//
	// Two webhooks can arrive at once, and the fallback poller can run while a
	// webhook is in flight. Reading the status and then writing it would let both
	// pass the check and mail the customer two different license keys for the same
	// invoice — confusing at best, and the second one silently replaces the first.
	// A conditional UPDATE makes exactly one of them the winner.
	//
	// The claim moved from the subscription row to the invoice row when billing
	// became recurring: a subscription is paid many times, so "status = 'approved'"
	// stopped being a usable guard after the first cycle. The invoice is the thing
	// that is paid exactly once.
	var subID string
	var periodEnd time.Time
	err = h.db.QueryRow(ctx, `
		UPDATE billing_invoices
		   SET status = 'paid', paid_at = NOW()
		 WHERE lexware_invoice_id = $1 AND status = 'open'
		RETURNING subscription_id, period_end`, invoiceID).
		Scan(&subID, &periodEnd)
	if err != nil {
		// No row claimed: either already settled (webhook replay, poller overlap)
		// or the invoice belongs to no subscription at all — someone paid a manual
		// invoice that has nothing to do with Vakt. Neither is an error.
		return
	}

	var company, email, interval, product, renewalToken string
	if err := h.db.QueryRow(ctx, `
		SELECT company_name, email, interval, product, renewal_token
		  FROM billing_quote_requests WHERE id = $1`, subID).
		Scan(&company, &email, &interval, &product, &renewalToken); err != nil {
		log.Error().Err(err).Str("invoice_id", invoiceID).Str("subscription_id", subID).
			Msg("billing: CRITICAL — invoice paid but its subscription could not be read")
		return
	}

	plan, err := PlanFor(product, interval)
	if err != nil {
		log.Error().Err(err).Str("subscription_id", subID).
			Msg("billing: CRITICAL — payment settled for a plan that no longer exists")
		return
	}

	key, mailErr := h.issuer.Issue(licensing.Request{
		OrgName: company, Email: email, Interval: interval, Trial: false,
		RenewalToken: renewalToken,
	}, nil, "")
	if key == "" {
		log.Error().Err(mailErr).Str("invoice_id", invoiceID).
			Msg("billing: CRITICAL — payment settled but license key could not be signed")
		return
	}

	// The new key and the next billing date land together. next_invoice_at is set
	// HERE and nowhere else: an invoice is only ever raised for a customer whose
	// previous one was paid.
	if _, err := h.db.Exec(ctx, `
		UPDATE billing_quote_requests
		   SET status = 'paid', paid_at = COALESCE(paid_at, NOW()),
		       license_key = $2, next_invoice_at = $3
		 WHERE id = $1`, subID, key, plan.NextInvoiceAt(periodEnd)); err != nil {
		log.Error().Err(err).Str("invoice_id", invoiceID).Msg("billing: persist license key")
	}

	if mailErr != nil {
		log.Error().Err(mailErr).Str("invoice_id", invoiceID).
			Msg("billing: CRITICAL — key issued but mail failed. Send it manually from the DB.")
		return
	}
	log.Info().
		Str("invoice_id", invoiceID).
		Str("subscription_id", subID).
		Str("next_invoice_at", plan.NextInvoiceAt(periodEnd).Format("2006-01-02")).
		Str("email_redacted", logsafe.RedactEmail(email)).
		Msg("billing: payment settled, full license key issued, cycle advanced")
}

// PollPayments periodically asks Lexware whether an approved-but-unpaid invoice
// has been settled, and issues the key if so.
//
// The webhook is the fast path, not the only path. It is a single point of
// failure with a nasty failure mode: rotating the Lexware API key silently
// deletes every event subscription, and Lexware itself drops subscriptions whose
// callback looks dead. If it stops firing, a customer pays 2.990 € and receives
// nothing — and nobody notices until they complain. That is the worst possible
// way to find out.
//
// So we ask, too. Slower (up to one interval), but it cannot silently stop.
func (h *Handler) PollPayments(ctx context.Context, interval time.Duration) {
	if !h.client.Enabled() || !h.issuer.Enabled() {
		return
	}
	t := time.NewTicker(interval)
	defer t.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-t.C:
			h.pollOnce(ctx)
		}
	}
}

func (h *Handler) pollOnce(ctx context.Context) {
	ctx, cancel := context.WithTimeout(ctx, 5*time.Minute)
	defer cancel()

	// Every OPEN invoice, not just the first one of a subscription. Once billing
	// became recurring, "the subscription is still in status approved" stopped
	// meaning "an invoice is waiting for money" — a subscription in its fifth month
	// is long since 'paid', and its fifth invoice would never have been polled.
	rows, err := h.db.Query(ctx, `
		SELECT lexware_invoice_id
		  FROM billing_invoices
		 WHERE status = 'open'
		   AND created_at > NOW() - INTERVAL '180 days'`)
	if err != nil {
		log.Error().Err(err).Msg("billing: poll open invoices")
		return
	}
	var ids []string
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err == nil {
			ids = append(ids, id)
		}
	}
	rows.Close()

	for _, id := range ids {
		// settle() re-checks payment status against Lexware and claims the row
		// atomically, so running it here is safe even while a webhook fires.
		h.settle(ctx, id)
	}
}

// GetLicense hands a customer's instance its current licence key.
//
// This is the endpoint behind VAKT_LICENSE_TOKEN: the instance calls it once a
// day and swaps in whatever key it gets back, so a renewal needs no manual step.
// It used to live in the Polar webhook package and read polar_subscriptions,
// which meant it only ever worked for customers who bought through Polar. Invoice
// customers received a key by e-mail and nothing else — their auto-renewal was
// dead on arrival, while .env.example and the docs promised otherwise.
//
// The token travels in the Authorization header, not the query string: query
// strings end up in access logs, and these are shipped to Loki on another host.
// (redactQuery in cmd/api/middleware.go would mask it, but not relying on that is
// cheaper than relying on it.)
//
// Not-found and wrong-token return the same 404 on purpose — a distinguishable
// response would turn this into an oracle for guessing tokens.
func (h *Handler) GetLicense(c echo.Context) error {
	auth := c.Request().Header.Get("Authorization")
	token := strings.TrimPrefix(auth, "Bearer ")
	if token == "" || token == auth {
		return c.JSON(http.StatusUnauthorized, map[string]string{"error": "Authorization: Bearer <token> required"})
	}
	if h.db == nil {
		return c.JSON(http.StatusServiceUnavailable, map[string]string{"error": "billing not configured"})
	}

	var key string
	err := h.db.QueryRow(c.Request().Context(), `
		SELECT license_key
		  FROM billing_quote_requests
		 WHERE renewal_token = $1::uuid
		   AND status = 'paid'
		   AND license_key IS NOT NULL`, token).Scan(&key)
	if err != nil {
		return c.JSON(http.StatusNotFound, map[string]string{"error": "not found"})
	}
	return c.JSON(http.StatusOK, map[string]string{"key": key})
}
