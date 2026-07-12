// Copyright (c) 2026 NorvikOps. All rights reserved.
// SPDX-License-Identifier: Elastic-2.0
// See LICENSE file in the project root for terms.

package lexware

import (
	"context"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/rs/zerolog/log"

	"github.com/matharnica/vakt/internal/billing/licensing"
	"github.com/matharnica/vakt/internal/license"
	"github.com/matharnica/vakt/internal/shared/logsafe"
)

// RenewDue raises the next invoice for every subscription whose period is running
// out, then sleeps. It is the piece that makes a subscription a subscription.
//
// Without it, Approve() sent exactly one invoice and the story ended there: a
// customer who bought "Monatslizenz — 299 €" got a 35-day key and, on day 36,
// silence. This sweep is what turns that into a cycle.
//
// Three rules, each of which exists to stop a specific way of billing someone
// wrongly — the failure mode here is not a crash, it is a wrong invoice in a real
// customer's inbox, and that is very expensive to take back:
//
//   - Cancelled subscriptions are never invoiced again (cancelled_at IS NOT NULL).
//   - A subscription with an OPEN invoice is never invoiced again. Somebody who has
//     not paid month 1 must not receive a bill for month 2 — that is dunning, and
//     dunning by accident is worse than no dunning at all.
//   - next_invoice_at is only ever set by settle(), i.e. when money actually landed.
//     A subscription that was never paid has it NULL and drops out of the query
//     entirely.
//
// Runs on the billing instance only, in its own goroutine, alongside PollPayments.
func (h *Handler) RenewDue(ctx context.Context, every time.Duration) {
	if h.db == nil || !h.client.Enabled() || !h.issuer.Enabled() {
		return
	}
	t := time.NewTicker(every)
	defer t.Stop()
	h.renewOnce(ctx) // once at boot: a restart must not skip a due renewal
	for {
		select {
		case <-ctx.Done():
			return
		case <-t.C:
			h.renewOnce(ctx)
		}
	}
}

type dueSubscription struct {
	id        string
	company   string
	email     string
	product   string
	interval  string
	quantity  int
	contactID string
	periodEnd time.Time
}

func (h *Handler) renewOnce(ctx context.Context) {
	ctx, cancel := context.WithTimeout(ctx, 5*time.Minute)
	defer cancel()

	rows, err := h.db.Query(ctx, `
		SELECT s.id, s.company_name, s.email, s.product, s.interval, s.quantity,
		       s.lexware_contact_id,
		       (SELECT MAX(bi.period_end) FROM billing_invoices bi WHERE bi.subscription_id = s.id)
		  FROM billing_quote_requests s
		 WHERE s.status = 'paid'
		   AND s.cancelled_at IS NULL
		   AND s.next_invoice_at IS NOT NULL
		   AND s.next_invoice_at <= NOW()
		   AND s.lexware_contact_id IS NOT NULL
		   AND NOT EXISTS (
		         SELECT 1 FROM billing_invoices bi
		          WHERE bi.subscription_id = s.id AND bi.status = 'open')`)
	if err != nil {
		log.Error().Err(err).Msg("billing: renewal sweep query")
		return
	}
	var due []dueSubscription
	for rows.Next() {
		var d dueSubscription
		var end *time.Time
		if err := rows.Scan(&d.id, &d.company, &d.email, &d.product, &d.interval, &d.quantity,
			&d.contactID, &end); err != nil {
			log.Error().Err(err).Msg("billing: renewal sweep scan")
			continue
		}
		if end == nil {
			// Paid, due, but no invoice on record. Cannot compute the next period
			// without inventing one — and inventing a billing period is exactly the
			// kind of guess that produces a wrong invoice.
			log.Error().Str("subscription_id", d.id).
				Msg("billing: subscription is due but has no invoice history — skipped, needs a look")
			continue
		}
		d.periodEnd = *end
		due = append(due, d)
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		log.Error().Err(err).Msg("billing: renewal sweep iterate")
		return
	}

	for _, d := range due {
		if err := h.renewOne(ctx, d); err != nil {
			// Loud, but not fatal to the sweep: one customer's failed renewal must
			// not stop everyone else's.
			log.Error().Err(err).
				Str("subscription_id", d.id).
				Str("email_redacted", logsafe.RedactEmail(d.email)).
				Msg("billing: renewal failed — customer will NOT be invoiced this cycle")
		}
	}
}

// renewOne raises one follow-up invoice.
//
// It deliberately does NOT issue a new licence key. The key comes when the money
// lands (settle), exactly as it does for the first invoice — the customer's current
// key is still valid for GraceDays past the period end, which is the whole point of
// sending this LeadDays early.
func (h *Handler) renewOne(ctx context.Context, d dueSubscription) error {
	plan, err := PlanFor(d.product, d.interval)
	if err != nil {
		return err
	}

	from, to := plan.Period(d.periodEnd)

	invoiceID, err := h.client.CreateInvoice(ctx, InvoiceInput{
		ContactID: d.contactID,
		Title:     plan.Title,
		Intro: fmt.Sprintf("hier ist die Rechnung für den nächsten Abrechnungszeitraum (%s – %s).",
			from.Format("02.01.2006"), to.Format("02.01.2006")),
		Description: plan.LineDesc(d.quantity),
		NetAmount:   plan.TotalEUR(d.quantity),
		DueInDays:   plan.DueDays,
	})
	if err != nil {
		return fmt.Errorf("create invoice: %w", err)
	}

	// Record it BEFORE mailing. If the mail fails we can resend by hand; if the row
	// is missing, the payment webhook has nothing to match against and a paying
	// customer would never get their key.
	if _, err := h.db.Exec(ctx, `
		INSERT INTO billing_invoices
			(subscription_id, lexware_invoice_id, period_start, period_end, net_amount_cents, status)
		VALUES ($1, $2, $3, $4, $5, 'open')`,
		d.id, invoiceID, from, to, plan.TotalCents(d.quantity)); err != nil {
		return fmt.Errorf("persist invoice %s: %w", invoiceID, err)
	}

	// next_invoice_at goes back to NULL: the cycle only continues if this invoice is
	// paid. settle() sets the next date. An unpaid customer quietly falls out.
	if _, err := h.db.Exec(ctx,
		`UPDATE billing_quote_requests SET next_invoice_at = NULL WHERE id = $1`, d.id); err != nil {
		return fmt.Errorf("clear next_invoice_at: %w", err)
	}

	pdf, err := h.client.InvoicePDF(ctx, invoiceID)
	if err != nil {
		log.Error().Err(err).Str("invoice_id", invoiceID).Msg("billing: renewal pdf")
		pdf = nil
	}

	body := fmt.Sprintf(`Hallo,

anbei die Rechnung für den nächsten Abrechnungszeitraum (%s – %s).

Dein aktueller Lizenzschlüssel bleibt bis zum Zahlungseingang gültig — du musst
nichts tun und es geht nichts aus. Sobald die Zahlung da ist, verlängert sich die
Lizenz automatisch.

Du möchtest nicht verlängern? Antworte einfach auf diese Mail, dann beenden wir
das Abo zum Ende des laufenden Zeitraums.

Viele Grüße
Stefan
Norvik Ops
`, from.Format("02.01.2006"), to.Format("02.01.2006"))

	if err := h.issuer.Send(d.email, "Deine Vakt-Rechnung für den nächsten Zeitraum", body, pdf, "Rechnung-Vakt.pdf"); err != nil {
		// The invoice exists and is recorded; only the mail is missing. Recoverable
		// by hand, so warn rather than fail the whole renewal.
		log.Error().Err(err).Str("invoice_id", invoiceID).
			Msg("billing: renewal invoice created but mail failed — send it manually")
	}

	log.Info().
		Str("subscription_id", d.id).
		Str("invoice_id", invoiceID).
		Str("period", from.Format("2006-01-02")+"→"+to.Format("2006-01-02")).
		Msg("billing: renewal invoice sent")
	return nil
}

// Cancel ends a subscription at the end of the period the customer already paid
// for. No refund, no clawback, no key revocation — the key simply is not renewed.
//
// Deliberately not an HTTP endpoint: cancellations arrive by e-mail ("Antworte
// einfach auf diese Mail"), and a self-service cancel button on a product with a
// handful of customers is machinery nobody needs. Called from the admin CLI.
// Cancel takes the pool, not a narrower interface.
//
// A minimal Execer interface here would have to mirror pgx's variadic untyped
// arguments — and this codebase deliberately ratchets untyped parameters DOWN
// (scripts/check_interface_ratchet.py). Adding one so the admin CLI could pass a
// *pgx.Conn is a bad trade: the CLI can simply open a pool.
func Cancel(ctx context.Context, db *pgxpool.Pool, subscriptionID string) error {
	tag, err := db.Exec(ctx, `
		UPDATE billing_quote_requests
		   SET cancelled_at = NOW(), next_invoice_at = NULL
		 WHERE id = $1 AND cancelled_at IS NULL`, subscriptionID)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return fmt.Errorf("billing: no active subscription %s (already cancelled, or unknown)", subscriptionID)
	}
	return nil
}

// MailExpiringKeys is the safety net for customers who never set VAKT_LICENSE_TOKEN.
//
// Their key normally arrives on payment (settle) and is valid through the period they
// paid for, plus grace — they need nothing else, and our being down cannot hurt them.
// But mail fails, people delete things, and a key issued by hand can be short. This
// sweep catches a licence whose key is about to run out while the customer is STILL
// ENTITLED to one, signs a fresh key up to their paid-through date, and mails it.
//
// The cap is the point. It never extends anyone beyond what they paid for: someone who
// stopped paying is simply not selected, no key is mailed, and theirs runs out. Cutting
// a customer off does not require detecting that they are running — it requires not
// handing them the next key. That distinction is the whole reason Vakt need not phone
// home, and it is why the renewal token can stay opt-in.
//
// A customer WITH the token rarely reaches this: their instance already re-fetched.
func (h *Handler) MailExpiringKeys(ctx context.Context, every time.Duration) {
	if h.db == nil || !h.issuer.Enabled() {
		return
	}
	t := time.NewTicker(every)
	defer t.Stop()
	h.mailExpiringOnce(ctx)
	for {
		select {
		case <-ctx.Done():
			return
		case <-t.C:
			h.mailExpiringOnce(ctx)
		}
	}
}

func (h *Handler) mailExpiringOnce(ctx context.Context) {
	ctx, cancel := context.WithTimeout(ctx, 5*time.Minute)
	defer cancel()

	rows, err := h.db.Query(ctx, `
		SELECT bl.renewal_token, bl.org_name, s.email, s.interval
		  FROM billing_licenses bl
		  JOIN billing_quote_requests s ON s.id = bl.subscription_id
		 WHERE bl.revoked_at IS NULL
		   AND bl.license_key <> ''
		   AND bl.kind <> 'trial'
		   AND s.status = 'paid'
		   AND s.cancelled_at IS NULL
		   AND bl.expires_at < $1`,
		// Cutoff computed in Go, not as ($1 || ' days')::interval — that construction
		// is a documented pgx trap (the driver cannot type a bound param inside a cast)
		// and it has broken a query in this codebase before.
		time.Now().Add(license.MailBelowDays*24*time.Hour))
	if err != nil {
		log.Error().Err(err).Msg("billing: expiring-key sweep")
		return
	}

	type due struct{ token, org, email, interval string }
	var list []due
	for rows.Next() {
		var d due
		if err := rows.Scan(&d.token, &d.org, &d.email, &d.interval); err == nil {
			list = append(list, d)
		}
	}
	rows.Close()

	for _, d := range list {
		// Never past what they paid for. A customer whose period has run out and whose
		// next invoice is unpaid gets nothing — that IS the cut-off.
		expiry, err := EntitlementByToken(ctx, h.db, d.token)
		if err != nil || !expiry.After(time.Now()) {
			continue
		}

		key, err := h.issuer.SignUntil(licensing.Request{OrgName: d.org, Interval: d.interval}, expiry)
		if err != nil {
			log.Error().Err(err).Str("org", d.org).Msg("billing: could not sign renewal key")
			continue
		}

		body := fmt.Sprintf(`Hallo,

hier ist der neue Lizenzschlüssel für %s. Der bisherige läuft demnächst aus —
trag den neuen einfach in deiner Vakt-Instanz ein (Einstellungen → Lizenz) oder in
die .env:

  VAKT_LICENSE_KEY=%s

Gültig bis %s.

Das geht auch von allein: Trägst du zusätzlich

  VAKT_LICENSE_TOKEN=%s

ein, holt sich deine Instanz den jeweils aktuellen Schlüssel selbst und du bekommst
diese Mail nie wieder. Übertragen wird dabei ausschließlich dieser Token — keine
Daten aus deiner Instanz, keine Nutzungsstatistik, nichts über deine Compliance.
Freiwillig: Ohne den Token bekommst du weiter alle paar Wochen einen Schlüssel von
uns, und deine Instanz spricht nie mit uns.

Viele Grüße
Stefan
Norvik Ops
`, d.org, key, expiry.Format("02.01.2006"), d.token)

		if err := h.issuer.Send(d.email, "Dein neuer Vakt-Lizenzschlüssel", body, nil, ""); err != nil {
			// Do NOT store the key if the mail failed: the customer does not have it, and
			// pretending they do would leave them locked out with a key only we can see.
			log.Error().Err(err).Str("org", d.org).
				Msg("billing: CRITICAL — renewal key could not be mailed; customer will go dark")
			continue
		}

		if _, err := h.db.Exec(ctx, `
			UPDATE billing_licenses SET license_key = $2, expires_at = $3
			 WHERE renewal_token = $1::uuid`, d.token, key, expiry); err != nil {
			log.Error().Err(err).Str("org", d.org).Msg("billing: renewal key mailed but not stored")
		}

		log.Info().Str("org", d.org).Str("expires", expiry.Format("2006-01-02")).
			Str("email_redacted", logsafe.RedactEmail(d.email)).
			Msg("billing: renewal key mailed (no auto-renewal token set)")
	}
}
