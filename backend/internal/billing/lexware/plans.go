// Copyright (c) 2026 NorvikOps. All rights reserved.
// SPDX-License-Identifier: Elastic-2.0
// See LICENSE file in the project root for terms.

package lexware

import (
	"fmt"
	"time"
)

// Plan is one sellable thing: a product at a billing interval.
//
// This is the single source of truth for what a customer is charged and for how
// long their key stays valid. Before it existed, the amount lived as two magic
// numbers inside Approve() — and nothing at all decided when the NEXT invoice
// went out, because there was no next invoice. A customer who picked "Monatslizenz
// — 299 €" got one invoice, a 35-day key, and then silence: no second invoice was
// ever created, and on day 36 their Pro features went dark. They had bought a
// subscription and received a one-off.
//
// The three day-counts are load-bearing and must stay consistent:
//
//	PeriodDays  what the customer paid for
//	GraceDays   how long the key outlives the period, i.e. the payment window
//	LeadDays    how early the next invoice goes out, so it can be paid in time
//	DueDays     the payment term printed on that invoice
//
// Two invariants tie them together, and both are enforced by tests because both
// failed silently once:
//
//  1. PeriodDays+GraceDays MUST equal license.KeyExpiry for the same interval,
//     or a paying customer's key dies mid-period.
//  2. LeadDays+GraceDays MUST exceed DueDays with room for a bank transfer to
//     clear. The first draft had a monthly plan where the key expired 12 days
//     after the invoice went out — while that invoice still had 14 days to run.
//     The customer would have gone dark before their bill was even overdue.
//
// If a test here fails, fix the numbers. Not the test.
type Plan struct {
	Product    string // "pro" — "managed" and "msp" are planned, see below
	Interval   string // "month" | "year"
	NetEUR     float64
	Title      string // invoice title
	Desc       string // invoice line item
	PeriodDays int
	GraceDays  int
	LeadDays   int
	DueDays    int
}

// PeriodKey is how a plan is addressed in the catalogue and stored in the DB
// (billing_quote_requests.product + .interval).
func PeriodKey(product, interval string) string { return product + "/" + interval }

// plans is the catalogue.
//
// Only what is actually on sale belongs here. Vakt Pro Managed Hosting (599 €/month)
// and the MSP bundles are planned but not sellable yet — Managed is gated on the
// AVV (Art. 28 DSGVO makes us a processor for the customer's ISMS data, sprint
// 104-1), and the MSP case is gated on the open ELv2 question of whether an MSP
// running Vakt FOR a client counts as a "managed service". Adding a price here
// before those are answered would make an unsellable product look sellable to
// every code path that reads this map.
var plans = map[string]Plan{
	PeriodKey("pro", "month"): {
		Product:  "pro",
		Interval: "month",
		NetEUR:   299.0,
		Title:    "Vakt Pro — Monatslizenz",
		Desc:     "Vakt Pro — self-hosted ISMS-Plattform, unbegrenzte Nutzer",
		// 30 + 5: the key survives the period by the payment window, so a transfer
		// that takes a few days does not black out a paying customer. The invoice
		// goes out 10 days early and is due in 10 — that leaves 15 days of key for
		// a 10-day term, i.e. 5 days of slack for the transfer to clear.
		PeriodDays: 30,
		GraceDays:  5,
		LeadDays:   10,
		DueDays:    10,
	},
	PeriodKey("pro", "year"): {
		Product:    "pro",
		Interval:   "year",
		NetEUR:     2990.0,
		Title:      "Vakt Pro — Jahreslizenz",
		Desc:       "Vakt Pro — self-hosted ISMS-Plattform, unbegrenzte Nutzer",
		PeriodDays: 365,
		GraceDays:  30,
		LeadDays:   21,
		DueDays:    14,
	},
}

// PlanFor looks up a plan. An unknown combination is an error, never a silent
// default: guessing here would mean invoicing the wrong amount.
func PlanFor(product, interval string) (Plan, error) {
	p, ok := plans[PeriodKey(product, interval)]
	if !ok {
		return Plan{}, fmt.Errorf("billing: no plan for product=%q interval=%q", product, interval)
	}
	return p, nil
}

// Period returns the span invoice N covers, given when it starts.
func (p Plan) Period(start time.Time) (from, to time.Time) {
	return start, start.AddDate(0, 0, p.PeriodDays)
}

// NextInvoiceAt is when the invoice for the FOLLOWING period should go out:
// LeadDays before the paid period ends, so the customer has time to pay before
// their key expires.
func (p Plan) NextInvoiceAt(periodEnd time.Time) time.Time {
	return periodEnd.AddDate(0, 0, -p.LeadDays)
}

// Cents is the net amount for ONE seat, in cents — money is an integer, never a
// float.
func (p Plan) Cents() int64 { return int64(p.NetEUR*100 + 0.5) }

// TotalEUR is what actually goes on the invoice: one seat times the number bought.
// An MSP buys ten; a direct customer buys one.
func (p Plan) TotalEUR(quantity int) float64 {
	if quantity < 1 {
		quantity = 1
	}
	return p.NetEUR * float64(quantity)
}

// TotalCents mirrors TotalEUR for storage.
func (p Plan) TotalCents(quantity int) int64 {
	if quantity < 1 {
		quantity = 1
	}
	return p.Cents() * int64(quantity)
}

// LineDesc names what is being sold, so an MSP's invoice does not silently read
// like a single licence at ten times the price.
func (p Plan) LineDesc(quantity int) string {
	if quantity > 1 {
		return fmt.Sprintf("%s — %d Lizenzen à %.2f €", p.Desc, quantity, p.NetEUR)
	}
	return p.Desc
}
