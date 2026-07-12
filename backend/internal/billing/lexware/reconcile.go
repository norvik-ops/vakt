// Copyright (c) 2026 NorvikOps. All rights reserved.
// SPDX-License-Identifier: Elastic-2.0
// See LICENSE file in the project root for terms.

package lexware

import (
	"context"
	"fmt"
	"time"

	"github.com/rs/zerolog/log"
)

// Drift is one disagreement between Vakt and Lexware.
type Drift struct {
	Kind      string // storniert | storniert-mit-key | nur-in-lexware | nur-in-vakt
	InvoiceID string
	Number    string // Lexware's RE0004 — what a human recognises
	Company   string
	Amount    string
	Detail    string
	Severe    bool // needs a decision, not just a glance
}

// Reconcile asks Lexware what it thinks, and reports where the two disagree.
//
// Vakt listens for payments. It does NOT hear a storno: cancelling an invoice in
// Lexware produces no payment event, so an invoice we booked as paid — for which we
// already signed and mailed a licence key — can quietly become void, and nothing in
// our database would ever notice. The first test invoice was exactly that case.
//
// Two directions of drift, both real:
//
//	storniert     Lexware voided it; we still count it as open or paid.
//	nur-in-lexware  raised by hand in Lexware; the panel does not know it exists, so
//	              every revenue figure here is a partial truth without it.
//
// What it does NOT do is act on its own. A storno is not always a cancellation: it is
// just as often a correction (wrong address -> void, re-issue). Automatically revoking
// the customer's licence on a storno would punish a paying customer for OUR typo. So
// this reports, and a human decides.
//
// The one thing it does write: an invoice voided in Lexware is marked voided here too,
// so the payment poller stops asking about a bill that will never be paid, and the
// revenue figures stop counting it.
func (h *Handler) Reconcile(ctx context.Context) ([]Drift, error) {
	vouchers, err := h.client.Invoices(ctx)
	if err != nil {
		return nil, err
	}
	byID := make(map[string]Voucher, len(vouchers))
	for _, v := range vouchers {
		byID[v.ID] = v
	}

	rows, err := h.db.Query(ctx, `
		SELECT i.lexware_invoice_id, i.status, i.net_amount_cents, s.company_name,
		       -- Lizenzen, die noch GELTEN: gesperrt oder abgelaufen brauchen keine
		       -- Entscheidung mehr.
		       (SELECT count(*) FROM billing_licenses bl
		         WHERE bl.subscription_id = i.subscription_id
		           AND bl.license_key <> '' AND bl.revoked_at IS NULL
		           AND bl.expires_at > NOW()),
		       -- Eine ANDERE bezahlte Rechnung desselben Abos, die denselben Zeitraum
		       -- abdeckt: dann war der Storno eine Korrektur, und es ist alles gut.
		       (SELECT count(*) FROM billing_invoices o
		         WHERE o.subscription_id = i.subscription_id
		           AND o.id <> i.id AND o.status = 'paid'
		           AND o.period_end >= i.period_start)
		  FROM billing_invoices i
		  JOIN billing_quote_requests s ON s.id = i.subscription_id`)
	if err != nil {
		return nil, err
	}

	var drifts []Drift
	seen := map[string]bool{}
	var toVoid []string

	for rows.Next() {
		var id, status, company string
		var cents int64
		var unrevoked, otherPaid int
		if err := rows.Scan(&id, &status, &cents, &company, &unrevoked, &otherPaid); err != nil {
			continue
		}
		seen[id] = true

		v, ok := byID[id]
		if !ok {
			drifts = append(drifts, Drift{
				Kind: "nur-in-vakt", InvoiceID: id, Company: company,
				Amount: eurCents(cents), Severe: true,
				Detail: "Vakt kennt diese Rechnung, Lexware nicht. Das darf nicht vorkommen — " +
					"wir legen sie erst an, NACHDEM Lexware sie bestätigt hat.",
			})
			continue
		}
		if !v.Voided() {
			continue
		}

		// Erstens: den Status nachziehen. Das ist die EINZIGE automatische Aktion.
		if status != "voided" {
			toVoid = append(toVoid, id)
		}

		// Zweitens — und das ist der Punkt: Der schwerwiegende Fall wird aus dem
		// ZUSTAND abgeleitet, nicht aus dem Übergang.
		//
		// Die erste Fassung meldete ihn nur, wenn `status != "voided"` — also genau
		// EINMAL. Beim nächsten Lauf war der Status nachgezogen, die Bedingung traf
		// nicht mehr, und die Warnung, die eine menschliche Entscheidung braucht,
		// LÖSCHTE SICH SELBST. Ein Storno für eine Rechnung, deren Lizenzschlüssel
		// bereits beim Kunden liegt, muss stehen bleiben, bis jemand handelt — nicht
		// bis der nächste Sweep drüberläuft.
		//
		// Er verschwindet, wenn die Lizenz gesperrt wurde (echte Stornierung) ODER
		// wenn eine andere bezahlte Rechnung den Zeitraum abdeckt (es war eine
		// Korrektur: storniert, neu gestellt, bezahlt). Beides sind Handlungen.
		if unrevoked > 0 && otherPaid == 0 {
			drifts = append(drifts, Drift{
				Kind: "storniert-mit-key", InvoiceID: id, Number: v.VoucherNumber,
				Company: company, Amount: eurCents(cents), Severe: true,
				Detail: "In Lexware storniert — aber der Kunde hat bereits einen gültigen " +
					"Lizenzschlüssel, und es gibt keine andere bezahlte Rechnung, die den Zeitraum " +
					"abdeckt. Ein Schlüssel lässt sich nicht zurückholen (kein Phone-Home). " +
					"Entscheide: War es eine Korrektur? Dann neue Rechnung stellen — diese Meldung " +
					"verschwindet, sobald sie bezahlt ist. War es eine echte Stornierung? Dann die " +
					"Lizenz sperren — sie läuft dann zum Ablaufdatum aus.",
			})
			continue
		}

		if status != "voided" {
			drifts = append(drifts, Drift{
				Kind: "storniert", InvoiceID: id, Number: v.VoucherNumber, Company: company,
				Amount: eurCents(cents),
				Detail: "In Lexware storniert. Vakt hatte sie als „" + status + "“ geführt — " +
					"ein Storno löst kein Zahlungsereignis aus und wäre uns nie aufgefallen. " +
					"Jetzt als storniert markiert; der Poller fragt nicht mehr nach, und sie zählt " +
					"nicht mehr als Umsatz.",
			})
		}
	}
	rows.Close()

	for _, v := range vouchers {
		if seen[v.ID] || v.Voided() {
			continue
		}
		drifts = append(drifts, Drift{
			Kind: "nur-in-lexware", InvoiceID: v.ID, Number: v.VoucherNumber,
			Company: v.ContactName, Amount: eurFloat(v.TotalAmount),
			Detail: "Nur in Lexware — vermutlich von Hand gestellt. Vakt kennt sie nicht, " +
				"also fehlt sie in jeder Zahl in diesem Panel.",
		})
	}

	// The one write: stop polling and stop counting an invoice that will never be paid.
	for _, id := range toVoid {
		if _, err := h.db.Exec(ctx,
			`UPDATE billing_invoices SET status = 'voided' WHERE lexware_invoice_id = $1`, id); err != nil {
			log.Error().Err(err).Str("invoice_id", id).Msg("billing: could not mark invoice voided")
		}
	}
	if len(toVoid) > 0 {
		log.Warn().Int("count", len(toVoid)).
			Msg("billing: invoices voided in Lexware — marked voided here; a storno raises no payment event")
	}
	return drifts, nil
}

// ReconcileLoop runs the check in the background, so a storno does not sit unnoticed
// until somebody happens to open the panel.
func (h *Handler) ReconcileLoop(ctx context.Context, every time.Duration) {
	if h.db == nil || !h.client.Enabled() {
		return
	}
	t := time.NewTicker(every)
	defer t.Stop()

	// Einmal sofort. Sonst stehen die Metriken nach einem Neustart bis zu 6 h auf
	// null — und ein Zabbix-Trigger, der auf "reconcile_age_seconds" schaut, feuert
	// nach jedem Deploy grundlos.
	first := time.After(30 * time.Second)

	for {
		select {
		case <-ctx.Done():
			return
		case <-first:
			c, cancel := context.WithTimeout(ctx, 2*time.Minute)
			if drifts, err := h.Reconcile(c); err != nil {
				h.RefreshMetrics(c, 0, true)
			} else {
				severe := 0
				for _, d := range drifts {
					if d.Severe {
						severe++
					}
				}
				h.RefreshMetrics(c, severe, false)
			}
			cancel()
		case <-t.C:
			c, cancel := context.WithTimeout(ctx, 2*time.Minute)
			drifts, err := h.Reconcile(c)
			if err != nil {
				log.Error().Err(err).Msg("billing: Lexware reconciliation failed")
				// Die Metriken MUESSEN das mitbekommen. Ein Abgleich, der still
				// fehlschlaegt und die alten Zahlen stehen laesst, sieht aus wie ein
				// gesunder — und ist gefaehrlicher als gar keiner.
				h.RefreshMetrics(c, 0, true)
				cancel()
				continue
			}
			severe := 0
			for _, d := range drifts {
				if d.Severe {
					severe++
					log.Warn().Str("kind", d.Kind).Str("invoice", d.Number).Str("company", d.Company).
						Msg("billing: RECONCILIATION — needs a decision, see the panel")
				}
			}
			h.RefreshMetrics(c, severe, false)
			cancel()
		}
	}
}

func eurCents(c int64) string { return fmtEUR(float64(c) / 100) }
func eurFloat(f float64) string {
	return fmtEUR(f)
}

// fmtEUR renders money the German way: 2.990,00 €. A human reads these numbers and
// compares them against Lexware by eye, so "299000" is not good enough.
func fmtEUR(v float64) string {
	neg := v < 0
	if neg {
		v = -v
	}
	cents := int64(v*100 + 0.5)
	whole, frac := cents/100, cents%100

	s := fmt.Sprint(whole)
	var out []byte
	for i, d := range []byte(s) {
		if i > 0 && (len(s)-i)%3 == 0 {
			out = append(out, '.')
		}
		out = append(out, d)
	}
	sign := ""
	if neg {
		sign = "−"
	}
	return fmt.Sprintf("%s%s,%02d €", sign, out, frac)
}
