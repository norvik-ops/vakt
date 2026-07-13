// Copyright (c) 2026 NorvikOps. All rights reserved.
// SPDX-License-Identifier: Elastic-2.0
// See LICENSE file in the project root for terms.

package lexware

import (
	"context"
	"fmt"
	"time"

	"github.com/rs/zerolog/log"

	"github.com/matharnica/vakt/internal/billing/licensing"
	"github.com/matharnica/vakt/internal/shared/logsafe"
)

// A free licence is a design partner, a reference customer, an association, a beta
// tester — someone who gets Vakt Pro and is not invoiced for it.
//
// It is a SEPARATE case, not a 100 % discount, and the reason is worth stating once:
// a 0 € invoice is never transferred, so Lexware never reports it "balanced", so
// settle() never runs — and settle() is the only thing that issues the full key and
// sets next_invoice_at. A 100 % customer would silently go dark on day 45. The rebate
// therefore stops at 90 % (see ValidateDiscount) and "free" comes through here, where
// no invoice is involved at all.
//
// ── The one design decision worth understanding ──────────────────────────────
//
// A free period is recorded as a PAID invoice of 0 cents, with a synthetic reference
// ("free:<sub>:<date>") in place of a Lexware voucher number.
//
// That is not a hack to dodge a NOT NULL. It is what keeps the rest of the system
// honest: entitlement, key re-signing, the VAKT_LICENSE_TOKEN auto-renewal, the MSP
// seat count and the renewal sweep ALL answer their questions by reading paid periods.
// Give them a paid period and every one of them works, unchanged, for a free customer.
// The alternative — teaching each of those places to also understand "or free" — is
// five edits, and the fifth one gets forgotten, and the customer it belongs to goes
// dark without anyone noticing. Which is exactly the failure this whole file exists
// to avoid.
//
// The price of the trick is that these rows have no counterpart in Lexware, so
// Reconcile() would report them as "nur in Vakt" — its most severe finding, meaning
// "we invented an invoice". Reconcile() therefore skips free subscriptions explicitly.

// freeInvoiceRef is the stand-in for a Lexware voucher number.
//
// Unique per period, because billing_invoices.lexware_invoice_id is UNIQUE and a free
// subscription raises one of these per cycle. Prefixed so that a human reading the
// table — or a query that has forgotten about free licences — can see instantly that
// this is not a Lexware document.
func freeInvoiceRef(subID string, periodStart time.Time) string {
	return fmt.Sprintf("free:%s:%s", subID, periodStart.Format("2006-01-02"))
}

// approveFree issues a licence with no invoice, no Lexware contact, no payment.
//
// Unlike the paid path, this is NOT irreversible: nothing is created outside our own
// database, so a mistake here is fixed by cancelling the subscription. That is the
// whole reason it can be this short.
func (h *Handler) approveFree(ctx context.Context, id, company, email, product, interval string, quantity int, by string) ApproveResult {
	if !h.issuer.Enabled() {
		return ApproveResult{Message: "Der Lizenz-Signierschlüssel fehlt auf dieser Instanz " +
			"(VAKT_LICENSE_PRIVATE_KEY). Es wurde nichts erstellt."}
	}

	plan, err := PlanFor(product, interval)
	if err != nil {
		return ApproveResult{Message: "FEHLER: Für diese Kombination gibt es keinen Tarif (" +
			product + "/" + interval + "). Es wurde nichts erstellt."}
	}

	from, to := plan.Period(time.Now())
	// Same rule as a paying customer: the key covers the period they were granted, plus
	// grace. Not "forever" — a free licence that never expires is one we can never take
	// back, and the only lever we have is not renewing it.
	entitledTo := to.AddDate(0, 0, plan.GraceDays)

	// The licence row first, because the key must carry ITS renewal token — that token
	// is what lets the customer's instance fetch its own renewals, and it cannot be
	// added afterwards.
	var renewalToken string
	if err := h.db.QueryRow(ctx, `
		INSERT INTO billing_licenses (subscription_id, org_name, license_key, expires_at, kind, note)
		VALUES ($1, $2, '', $3, 'full', 'Freilizenz — keine Rechnung')
		RETURNING renewal_token`,
		id, company, entitledTo).Scan(&renewalToken); err != nil {
		log.Error().Err(err).Str("request_id", id).Msg("billing: create free licence row")
		return ApproveResult{Message: "FEHLER: Lizenz-Datensatz konnte nicht angelegt werden.\n\n" + err.Error()}
	}

	// A FULL key straight away — not a 45-day trial. A trial key is a placeholder for a
	// payment that is coming; here no payment is coming, so there is nothing to hold the
	// key back for.
	key, err := h.issuer.SignUntil(licensing.Request{
		OrgName: company, Email: email, Interval: interval, RenewalToken: renewalToken,
	}, entitledTo)
	if err != nil {
		log.Error().Err(err).Str("request_id", id).Msg("billing: sign free licence")
		return ApproveResult{Message: "FEHLER: Schlüssel konnte nicht signiert werden.\n\n" + err.Error()}
	}

	// Both writes together. The invoice row IS the entitlement — without it the customer
	// has a key that nothing will ever renew, and with it but no subscription update the
	// panel shows a paid-looking subscription with no licence.
	tx, err := h.db.Begin(ctx)
	if err != nil {
		return ApproveResult{Message: "FEHLER: " + err.Error()}
	}
	_, err = tx.Exec(ctx, `
		UPDATE billing_quote_requests
		   SET status = 'paid', approved_at = NOW(), paid_at = NOW(),
		       license_key = $2, next_invoice_at = $3
		 WHERE id = $1`, id, key, plan.NextInvoiceAt(to))
	if err == nil {
		_, err = tx.Exec(ctx, `
			UPDATE billing_licenses SET license_key = $2 WHERE renewal_token = $1::uuid`,
			renewalToken, key)
	}
	if err == nil {
		// The 0 € "paid invoice" that makes every downstream query work. discount_percent
		// is 100 and list_amount_cents is the real list price, so the panel can still say
		// what this licence would have cost — which is the number you want when a design
		// partner turns into a customer.
		charge, cerr := plan.Charge(quantity, 0)
		if cerr != nil {
			err = cerr
		} else {
			_, err = tx.Exec(ctx, `
				INSERT INTO billing_invoices
					(subscription_id, lexware_invoice_id, period_start, period_end,
					 net_amount_cents, list_amount_cents, discount_percent, status, paid_at)
				VALUES ($1, $2, $3, $4, 0, $5, 100, 'paid', NOW())`,
				id, freeInvoiceRef(id, from), from, to, charge.ListCents)
		}
	}
	if err == nil {
		err = tx.Commit(ctx)
	} else {
		_ = tx.Rollback(ctx)
	}
	if err != nil {
		log.Error().Err(err).Str("request_id", id).Msg("billing: record free licence")
		return ApproveResult{Message: "FEHLER: Freilizenz konnte nicht verbucht werden.\n\n" + err.Error()}
	}

	body := fmt.Sprintf(`Hallo,

hier ist euer Lizenzschlüssel für Vakt Pro — kostenlos, es kommt keine Rechnung.

  VAKT_LICENSE_KEY=%s

Gültig bis %s.

So aktiviert ihr ihn:
  1. In eurer Vakt-Instanz auf "Einstellungen" → "Lizenz"
  2. Schlüssel einfügen, speichern — fertig.

Damit er sich von allein verlängert, könnt ihr zusätzlich

  VAKT_LICENSE_TOKEN=%s

eintragen. Dann holt sich eure Instanz den jeweils aktuellen Schlüssel selbst.
Übertragen wird dabei ausschließlich dieser Token — keine Daten aus eurer Instanz,
keine Nutzungsstatistik, nichts über eure Compliance.

Viele Grüße
Stefan
Norvik Ops
`, key, entitledTo.Format("02.01.2006"), renewalToken)

	mailErr := h.issuer.Send(email, "Euer Vakt-Lizenzschlüssel", body, nil, "")

	log.Info().Str("request_id", id).Str("by", by).
		Str("expires", entitledTo.Format("2006-01-02")).
		Str("email_redacted", logsafe.RedactEmail(email)).
		Msg("billing: free licence issued — no invoice, no Lexware")

	if mailErr != nil {
		return ApproveResult{
			Message: "Die Freilizenz ist ausgestellt (gültig bis " + entitledTo.Format("02.01.2006") +
				"), aber die E-Mail an " + email + " ist fehlgeschlagen. Den Schlüssel findest du " +
				"unten unter „Ausgestellte Lizenzen“ — bitte von Hand schicken."}
	}
	return ApproveResult{OK: true,
		Message: "Freilizenz ausgestellt und an " + email + " verschickt — gültig bis " +
			entitledTo.Format("02.01.2006") + ". Es wurde KEINE Rechnung erstellt und kein " +
			"Lexware-Kontakt angelegt. Sie verlängert sich automatisch, bis du das Abo kündigst."}
}

// ConvertInput is the billing address a free customer never needed — plus the price.
//
// A free subscription has no Lexware contact and may have no address at all: nobody
// asked, because nobody was going to be invoiced. Turning them into a paying customer
// is therefore the moment those fields are collected, not a formality.
type ConvertInput struct {
	ContactName     string
	VATID           string
	Street          string
	Zip             string
	City            string
	CountryCode     string
	DiscountPercent int
	InvoiceNow      bool
}

// ConvertToPaid turns a free licence into a paying subscription — IN PLACE.
//
// In place is the entire point. The obvious alternative — cancel the free subscription
// and create a new paid one — looks equivalent and is not: the customer's licence lives
// on the OLD subscription, and with it their renewal token. Cancelling it makes
// GetLicense answer 404 (it checks cancelled_at), so the VAKT_LICENSE_TOKEN sitting in
// their .env stops working the moment they agree to pay us. They would get a new key by
// mail and have to swap the token by hand — a papercut delivered precisely at the moment
// of the sale.
//
// Here the subscription, the licence row and the renewal token all survive. From the
// customer's side, nothing happens except that an invoice arrives.
//
// Order matters, and it is chosen so that no failure leaves a mess:
//
//  1. Create the Lexware contact. Nothing has changed on our side yet, so a failure
//     here changes nothing at all.
//  2. Flip the subscription to paying, with next_invoice_at chained to the end of the
//     free period. This is a SAFE resting state: it is now an ordinary paying
//     subscription that will be invoiced by the normal sweep when its granted period
//     runs out. If step 3 fails, we stop here and say so.
//  3. Optionally raise the first invoice immediately — the irreversible bit, and the
//     only one, deliberately last.
func (h *Handler) ConvertToPaid(ctx context.Context, subID string, in ConvertInput, by string) (string, error) {
	if !h.client.Enabled() {
		return "", fmt.Errorf("Lexware ist auf dieser Instanz nicht konfiguriert")
	}
	if err := ValidateDiscount(in.DiscountPercent); err != nil {
		return "", err
	}

	var company, email, product, interval, status string
	var quantity int
	var isFree bool
	var cancelled *time.Time
	var periodEnd *time.Time
	if err := h.db.QueryRow(ctx, `
		SELECT s.company_name, s.email, s.product, s.interval, s.status, s.quantity,
		       s.is_free, s.cancelled_at,
		       (SELECT MAX(bi.period_end) FROM billing_invoices bi
		         WHERE bi.subscription_id = s.id AND bi.status = 'paid')
		  FROM billing_quote_requests s WHERE s.id = $1`, subID).
		Scan(&company, &email, &product, &interval, &status, &quantity,
			&isFree, &cancelled, &periodEnd); err != nil {
		return "", fmt.Errorf("Abo nicht gefunden")
	}
	if !isFree {
		return "", fmt.Errorf("Das ist bereits ein zahlendes Abo")
	}
	if cancelled != nil {
		return "", fmt.Errorf("Das Abo ist gekündigt. Umwandeln würde ein Abo wiederbeleben, " +
			"das beendet ist — leg lieber ein neues an")
	}
	if status != "paid" || periodEnd == nil {
		return "", fmt.Errorf("Die Freilizenz ist noch nicht ausgestellt. Erst freigeben, " +
			"dann umwandeln")
	}
	if in.Street == "" || in.Zip == "" || in.City == "" {
		return "", fmt.Errorf("Für eine Rechnung braucht Lexware eine Adresse " +
			"(Straße, PLZ, Ort). Bei einer Freilizenz wurde danach nie gefragt")
	}
	if in.CountryCode == "" {
		in.CountryCode = "DE"
	}

	plan, err := PlanFor(product, interval)
	if err != nil {
		return "", err
	}

	// ── 1. Lexware-Kontakt. Bis hierhin ist bei uns nichts passiert.
	contactID, err := h.client.CreateContact(ctx, ContactInput{
		CompanyName: company, VATID: in.VATID, ContactName: in.ContactName, Email: email,
		Street: in.Street, Zip: in.Zip, City: in.City, CountryCode: in.CountryCode,
	})
	if err != nil {
		return "", fmt.Errorf("Lexware-Kontakt konnte nicht angelegt werden: %w. "+
			"Es wurde nichts geändert", err)
	}

	// ── 2. Der sichere Zwischenzustand.
	//
	// next_invoice_at haengt an das ENDE des bereits geschenkten Zeitraums an — der
	// Kunde behaelt, was ihm zugesagt wurde, und die erste Rechnung kommt, wenn dieser
	// Zeitraum ausläuft. Genau wie bei jedem anderen zahlenden Kunden, mit demselben
	// Sweep. Kein Sonderweg, der spaeter vergessen wird.
	//
	// Die Adresse wird mitgeschrieben: sonst stuende sie nur in Lexware, und das naechste
	// Mal, dass jemand hier hinsieht, fehlte sie wieder.
	if _, err := h.db.Exec(ctx, `
		UPDATE billing_quote_requests
		   SET is_free = false,
		       lexware_contact_id = $2,
		       discount_percent = $3,
		       contact_name = COALESCE(NULLIF($4, ''), contact_name),
		       vat_id = COALESCE(NULLIF($5, ''), vat_id),
		       street = $6, zip = $7, city = $8, country_code = $9,
		       next_invoice_at = $10
		 WHERE id = $1`,
		subID, contactID, in.DiscountPercent, in.ContactName, in.VATID,
		in.Street, in.Zip, in.City, in.CountryCode,
		plan.NextInvoiceAt(*periodEnd)); err != nil {
		// Der Kontakt steht jetzt in Lexware, das Abo ist unveraendert. Das ist harmlos
		// (ein Kontakt ohne Rechnung kostet nichts) und beim naechsten Versuch faellt
		// nur ein doppelter Kontakt an — laut sagen, nicht stillschweigend weitermachen.
		return "", fmt.Errorf("Der Lexware-Kontakt wurde angelegt (%s), aber das Abo konnte "+
			"nicht umgestellt werden: %w. Bitte nachsehen", contactID, err)
	}

	log.Warn().Str("subscription_id", subID).Str("by", by).
		Str("lexware_contact_id", contactID).Int("discount_percent", in.DiscountPercent).
		Bool("invoice_now", in.InvoiceNow).
		Str("email_redacted", logsafe.RedactEmail(email)).
		Msg("billing: free licence converted to a paying subscription")

	charge, err := plan.Charge(quantity, in.DiscountPercent)
	if err != nil {
		return "", err
	}

	if !in.InvoiceNow {
		return fmt.Sprintf("Umgewandelt. %s zahlt ab jetzt — die erste Rechnung über %s geht "+
			"automatisch raus, wenn der geschenkte Zeitraum am %s ausläuft. Der Kunde behält "+
			"seinen Schlüssel UND seinen Renewal-Token: für ihn ändert sich nichts, bis die "+
			"Rechnung kommt.",
			company, fmtEUR(charge.NetEUR()), periodEnd.Format("02.01.2006")), nil
	}

	// ── 3. Sofort abrechnen — der unumkehrbare Teil, bewusst zuletzt.
	//
	// Genau derselbe Code wie jede Verlaengerung (renewOne): Rechnung in Lexware,
	// Zeile in billing_invoices, PDF per Mail. Ein zweiter, leicht anderer Rechnungsweg
	// waere die Stelle, an der die beiden irgendwann auseinanderlaufen.
	//
	// periodEnd = jetzt, damit die bezahlte Periode HEUTE beginnt und nicht erst, wenn
	// der Gratiszeitraum endet.
	if err := h.renewOne(ctx, dueSubscription{
		id: subID, company: company, email: email, product: product, interval: interval,
		quantity: quantity, discount: in.DiscountPercent, isFree: false,
		contactID: contactID, periodEnd: time.Now(),
	}); err != nil {
		return "", fmt.Errorf("Das Abo ist umgestellt (zahlend, Kontakt %s), aber die Rechnung "+
			"konnte NICHT erstellt werden: %w.\n\nDas ist kein Notfall: Die erste Rechnung geht "+
			"sonst automatisch raus, wenn der geschenkte Zeitraum am %s ausläuft",
			contactID, err, periodEnd.Format("02.01.2006"))
	}

	return fmt.Sprintf("Umgewandelt und abgerechnet. %s hat eine finalisierte Rechnung über %s "+
		"per Mail bekommen; ab Zahlungseingang läuft das Abo wie jedes andere. Der Kunde behält "+
		"seinen Schlüssel und seinen Renewal-Token — er muss in seiner Instanz nichts anfassen. "+
		"(Hinweis: Der bereits geschenkte Zeitraum lief noch bis %s — die bezahlte Periode "+
		"beginnt heute und überlappt ihn.)",
		company, fmtEUR(charge.NetEUR()), periodEnd.Format("02.01.2006")), nil
}

// renewFree extends a free licence by one period.
//
// It creates nothing outside our database and sends no mail: it simply records the next
// period as paid. Everything that hands the customer a fresh key — the auto-renewal
// endpoint (GetLicense) and the expiring-key sweep (MailExpiringKeys) — already reads
// entitlement from paid periods, so extending the entitlement IS the renewal.
//
// A free subscription therefore keeps running until somebody cancels it. That is the
// intended behaviour: the way to end a free licence is Cancel, exactly as for a paying
// customer — the key is not revoked, it simply stops being renewed.
func (h *Handler) renewFree(ctx context.Context, d dueSubscription) error {
	plan, err := PlanFor(d.product, d.interval)
	if err != nil {
		return err
	}
	charge, err := plan.Charge(d.quantity, 0)
	if err != nil {
		return err
	}

	from, to := plan.Period(d.periodEnd)

	if _, err := h.db.Exec(ctx, `
		INSERT INTO billing_invoices
			(subscription_id, lexware_invoice_id, period_start, period_end,
			 net_amount_cents, list_amount_cents, discount_percent, status, paid_at)
		VALUES ($1, $2, $3, $4, 0, $5, 100, 'paid', NOW())`,
		d.id, freeInvoiceRef(d.id, from), from, to, charge.ListCents); err != nil {
		return fmt.Errorf("persist free period: %w", err)
	}

	if _, err := h.db.Exec(ctx,
		`UPDATE billing_quote_requests SET next_invoice_at = $2 WHERE id = $1`,
		d.id, plan.NextInvoiceAt(to)); err != nil {
		return fmt.Errorf("advance free cycle: %w", err)
	}

	log.Info().Str("subscription_id", d.id).
		Str("period", from.Format("2006-01-02")+"→"+to.Format("2006-01-02")).
		Msg("billing: free licence extended — no invoice raised")
	return nil
}
