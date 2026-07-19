// Copyright (c) 2026 NorvikOps. All rights reserved.
// SPDX-License-Identifier: Elastic-2.0
// See LICENSE file in the project root for terms.

package lexware

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"strings"
	"time"

	"github.com/rs/zerolog/log"

	"github.com/matharnica/vakt/internal/shared/logsafe"
)

// NewSubscription is a sale that never went through the web form.
type NewSubscription struct {
	CompanyName     string
	ContactName     string
	Email           string
	VATID           string
	Street          string
	Zip             string
	City            string
	CountryCode     string
	Product         string
	Interval        string
	Quantity        int
	DiscountPercent int
	IsFree          bool // Freilizenz: keine Rechnung, kein Lexware-Kontakt (free.go)
	Notes           string
}

// CreateSubscription records a customer who phoned instead of filling in the form.
//
// Without it there is exactly one way to sell to such a customer: raise the invoice
// directly in Lexware. Vakt then does not know the sale exists — no subscription, no
// renewal, no key, and the invoice shows up in the reconciliation as "nur in Lexware".
// That is not a footnote: EVERY number in the panel is a partial truth while such a
// sale is missing, and the customer never gets an automatic renewal.
//
// It creates the request in the same shape the public form would, so exactly one code
// path takes it from there — ApproveRequest. No second, subtly different flow.
func (h *Handler) CreateSubscription(ctx context.Context, in NewSubscription, by string) (string, error) {
	in.CompanyName = strings.TrimSpace(in.CompanyName)
	in.Email = strings.TrimSpace(in.Email)
	if in.CompanyName == "" || in.Email == "" {
		return "", fmt.Errorf("Firma und E-Mail sind Pflicht")
	}
	if in.Quantity < 1 {
		in.Quantity = 1
	}
	if in.CountryCode == "" {
		in.CountryCode = "DE"
	}
	if in.Product == "" {
		in.Product = "pro"
	}
	if _, err := PlanFor(in.Product, in.Interval); err != nil {
		return "", fmt.Errorf("kein Tarif für %s/%s", in.Product, in.Interval)
	}
	// Rejected here, at the door. The CHECK constraint on the column would catch it
	// too, but as an opaque database error — and the reason a 100 % rebate is refused
	// needs explaining, not a 23514.
	if err := ValidateDiscount(in.DiscountPercent); err != nil {
		return "", err
	}

	// An approval token even though nobody will click a link: the row must have the
	// same shape as one from the form, or the next person to read this table finds two
	// kinds of subscription and has to work out which invariants hold for which.
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	sum := sha256.Sum256([]byte(hex.EncodeToString(b)))

	var id string
	err := h.db.QueryRow(ctx, `
		INSERT INTO billing_quote_requests
			(company_name, contact_name, email, vat_id, street, zip, city, country_code,
			 note, notes, product, quantity, interval, discount_percent, is_free,
			 approval_token_hash)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14,$15,$16)
		RETURNING id`,
		in.CompanyName, in.ContactName, in.Email, in.VATID, in.Street, in.Zip, in.City,
		in.CountryCode, "von Hand im Panel angelegt", in.Notes,
		in.Product, in.Quantity, in.Interval, in.DiscountPercent, in.IsFree,
		hex.EncodeToString(sum[:])).Scan(&id)
	if err != nil {
		return "", err
	}
	log.Info().Str("subscription_id", id).Str("by", by).
		Int("discount_percent", in.DiscountPercent).Bool("free", in.IsFree).
		Str("email_redacted", logsafe.RedactEmail(in.Email)).
		Msg("billing: subscription created by hand")
	return id, nil
}

// SetDiscount grants or changes a customer's permanent rebate.
//
// The percentage lives on the SUBSCRIPTION, which is what makes it permanent: every
// invoice — the first and every renewal — is priced from it. That is also why this
// returns a sentence rather than nil: what the change actually DOES depends on where
// the subscription stands, and getting that wrong costs real money in a real
// customer's inbox.
//
//   - Still „angefragt": the rebate lands on the invoice that has not gone out yet.
//   - Already approved or paid: invoices already raised are untouched (a finalised
//     Lexware invoice cannot be changed, and rewriting our copy would make our books
//     disagree with the customer's). It takes effect at the next renewal.
//
// Cancelled subscriptions are refused outright — they will never be invoiced again,
// so a rebate on one is a silent no-op, and a silent no-op on a money field is how
// somebody ends up believing a customer got a discount that they did not.
func (h *Handler) SetDiscount(ctx context.Context, subID string, percent int, by string) (string, error) {
	if err := ValidateDiscount(percent); err != nil {
		return "", err
	}

	var status string
	var cancelled *time.Time
	var product, interval, country string
	var quantity, old int
	var isFree bool
	if err := h.db.QueryRow(ctx, `
		SELECT status, cancelled_at, product, interval, quantity, discount_percent, is_free,
		       country_code
		  FROM billing_quote_requests WHERE id = $1`, subID).
		Scan(&status, &cancelled, &product, &interval, &quantity, &old, &isFree, &country); err != nil {
		return "", fmt.Errorf("Abo nicht gefunden")
	}
	// Ein Rabatt auf eine Freilizenz ist ein Rabatt auf null. Er waere kein Fehler,
	// sondern schlimmer: eine stille Nulloperation auf einem Geldfeld — jemand setzt
	// 20 %, es passiert nichts, und niemand erfaehrt es.
	if isFree {
		return "", fmt.Errorf("Das ist eine Freilizenz — es wird ohnehin nichts berechnet. " +
			"Ein Rabatt darauf haette keine Wirkung")
	}
	if cancelled != nil {
		return "", fmt.Errorf("Das Abo ist gekündigt — es wird nie wieder eine Rechnung gestellt. " +
			"Ein Rabatt darauf hätte keine Wirkung")
	}

	plan, err := PlanFor(product, interval)
	if err != nil {
		return "", err
	}
	charge, err := plan.Charge(quantity, percent)
	if err != nil {
		return "", err
	}

	if _, err := h.db.Exec(ctx,
		`UPDATE billing_quote_requests SET discount_percent = $2 WHERE id = $1`,
		subID, percent); err != nil {
		return "", fmt.Errorf("Rabatt konnte nicht gespeichert werden: %w", err)
	}

	log.Warn().Str("subscription_id", subID).Str("by", by).
		Int("from_percent", old).Int("to_percent", percent).
		Int64("net_cents", charge.NetCents).
		Msg("billing: discount changed")

	// Say what the next invoice will read, in euros. „20 % gespeichert" is not an
	// answer to the question the person pressing this button actually has.
	//
	// Der genannte Betrag ist der RECHNUNGSBETRAG (brutto) — das ist die Zahl, die der
	// Kunde überweist. Der Rabattvergleich daneben bleibt netto, weil der Nachlass auf
	// den Nettobetrag gewährt wird. Unter § 19 UStG sind beide identisch; ab der
	// Regelbesteuerung wäre eine Nettozahl hier eine Antwort auf eine Frage, die niemand
	// gestellt hat.
	//
	// vatVerified=false: Diese Vorschau darf keinen Nachweis behaupten, den es nicht
	// gibt. Für einen EU-Auslandskunden scheitert die Einordnung deshalb — dann wird der
	// Nettobetrag genannt und der Grund dazu, statt eine erfundene Zahl.
	betrag := fmtEUR(charge.NetEUR())
	if amounts, taxErr := h.client.InvoiceAmountsFor(charge.NetCents, country, false); taxErr == nil {
		betrag = fmtEUR(float64(amounts.GrossCents) / 100)
	}
	next := "Die nächste Rechnung lautet über " + betrag
	if charge.Discounted() {
		next += " (netto " + fmtEUR(charge.NetEUR()) + " statt " + fmtEUR(charge.ListEUR()) + ")"
	}

	switch {
	case percent == 0:
		return "Rabatt entfernt. " + next + " (Listenpreis).", nil
	case status == "requested":
		return fmt.Sprintf("Rabatt auf %d %% gesetzt. %s — er geht auf die Rechnung, die beim "+
			"Freigeben erstellt wird, und auf jede Verlängerung.", percent, next), nil
	default:
		return fmt.Sprintf("Rabatt auf %d %% gesetzt. Bereits gestellte Rechnungen bleiben, wie "+
			"sie sind — geändert werden können sie nur per Storno in Lexware. %s (ab der nächsten "+
			"Verlängerung).", percent, next), nil
	}
}

// SetNotes stores free text about a customer.
//
// It sounds like a nicety. It is the place where everything that fits nowhere else
// ends up — "will die Rechnung per Post", "USt-ID kommt nach", "Ansprechpartner
// wechselt zum 1.9." — and today all of that lives in one person's head.
func (h *Handler) SetNotes(ctx context.Context, subID, notes string) error {
	if len(notes) > 5000 {
		notes = notes[:5000]
	}
	_, err := h.db.Exec(ctx,
		`UPDATE billing_quote_requests SET notes = $2 WHERE id = $1`, subID, notes)
	return err
}

// SendReminder nudges a customer about an unpaid invoice.
//
// Deliberately manual. An automatic dunning run is a machine that mails your customers
// while you sleep, and the first time it fires at the wrong one — because a payment was
// booked late, or the storno had not synced — you cannot take it back. With a handful of
// customers, a button you press is better than a cron you trust.
func (h *Handler) SendReminder(ctx context.Context, invoiceID, by string) error {
	var company, email, number string
	var cents int64
	var created time.Time
	var reminded *time.Time
	if err := h.db.QueryRow(ctx, `
		SELECT s.company_name, s.email, i.lexware_invoice_id, i.net_amount_cents,
		       i.created_at, i.reminded_at
		  FROM billing_invoices i
		  JOIN billing_quote_requests s ON s.id = i.subscription_id
		 WHERE i.lexware_invoice_id = $1 AND i.status = 'open'`, invoiceID).
		Scan(&company, &email, &number, &cents, &created, &reminded); err != nil {
		return fmt.Errorf("keine offene Rechnung mit dieser Nummer")
	}

	// Not twice in a day. Somebody double-clicks, or opens the panel from two devices,
	// and the customer gets two identical reminders — which reads as either sloppy or
	// aggressive, and both are worse than one reminder.
	if reminded != nil && time.Since(*reminded) < 24*time.Hour {
		return fmt.Errorf("es wurde heute bereits erinnert (%s)", reminded.Format("15:04"))
	}

	pdf, err := h.client.InvoicePDF(ctx, invoiceID)
	if err != nil {
		log.Warn().Err(err).Str("invoice_id", invoiceID).Msg("billing: reminder without PDF")
		pdf = nil
	}

	body := fmt.Sprintf(`Hallo,

eine kurze Erinnerung: Die Rechnung vom %s über %s ist bei uns noch nicht als bezahlt
eingegangen. Anbei noch einmal die Rechnung.

Falls die Überweisung schon unterwegs ist, betrachte diese Mail bitte als gegenstandslos —
Zahlungen brauchen ein paar Tage, bis sie bei uns ankommen.

Dein Lizenzschlüssel läuft weiter. Er wird nur nicht verlängert, solange die Rechnung
offen ist.

Fragen? Antworte einfach auf diese Mail.

Viele Grüße
Stefan
Norvik Ops
`, created.Format("02.01.2006"), fmtEUR(float64(cents)/100))

	if err := h.issuer.Send(email, "Erinnerung: offene Rechnung für "+company, body, pdf, "Rechnung-Vakt.pdf"); err != nil {
		return fmt.Errorf("Mail konnte nicht verschickt werden: %w", err)
	}

	if _, err := h.db.Exec(ctx,
		`UPDATE billing_invoices SET reminded_at = NOW() WHERE lexware_invoice_id = $1`, invoiceID); err != nil {
		log.Error().Err(err).Str("invoice_id", invoiceID).Msg("billing: reminder sent but not recorded")
	}
	log.Info().Str("invoice_id", invoiceID).Str("by", by).
		Str("email_redacted", logsafe.RedactEmail(email)).Msg("billing: payment reminder sent")
	return nil
}

// ResendKey mails an existing licence key again.
//
// The customer deleted the mail. Without this, the only copy they can reach is in our
// database — and getting it out means somebody SSHing into a production server and
// running a SELECT, which is a bad habit to need.
//
// It signs NOTHING new: the same key, to the same address, exactly as it went out the
// first time.
func (h *Handler) ResendKey(ctx context.Context, renewalToken, sendTo, by string) error {
	var orgName, key, subEmail, interval string
	var expires time.Time
	if err := h.db.QueryRow(ctx, `
		SELECT l.org_name, l.license_key, l.expires_at, s.email, s.interval
		  FROM billing_licenses l
		  JOIN billing_quote_requests s ON s.id = l.subscription_id
		 WHERE l.renewal_token = $1::uuid AND l.license_key <> ''`, renewalToken).
		Scan(&orgName, &key, &expires, &subEmail, &interval); err != nil {
		return fmt.Errorf("kein Schlüssel mit diesem Token")
	}
	if sendTo == "" {
		sendTo = subEmail
	}

	body := fmt.Sprintf(`Hallo,

hier ist der Lizenzschlüssel für %s noch einmal — es ist derselbe wie beim ersten Mal,
es wurde nichts neu ausgestellt.

  VAKT_LICENSE_KEY=%s

Gültig bis %s.

So aktivierst du ihn:
  1. In deiner Vakt-Instanz auf "Einstellungen" → "Lizenz"
  2. Schlüssel einfügen, speichern — fertig.

Viele Grüße
Stefan
Norvik Ops
`, orgName, key, expires.Format("02.01.2006"))

	if err := h.issuer.Send(sendTo, "Dein Vakt-Lizenzschlüssel", body, nil, ""); err != nil {
		return fmt.Errorf("Mail konnte nicht verschickt werden: %w", err)
	}
	log.Info().Str("org", orgName).Str("by", by).
		Str("email_redacted", logsafe.RedactEmail(sendTo)).Msg("billing: licence key re-sent")
	return nil
}

// InvoicePDF fetches the PDF so the panel can show it without a detour through Lexware.
func (h *Handler) InvoicePDF(ctx context.Context, invoiceID string) ([]byte, error) {
	return h.client.InvoicePDF(ctx, invoiceID)
}
