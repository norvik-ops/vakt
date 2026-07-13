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
// The numbers are load-bearing and must stay consistent:
//
//	PeriodMonths  what the customer paid for — CALENDAR months, see Period()
//	GraceDays     how long past the period end we keep serving them before cutting off
//	LeadDays      how early the next invoice goes out, so it can be paid in time
//	DueDays       the payment term printed on that invoice
//
// The period is in months and the rest in days on purpose. A period is a calendar
// span the customer recognises ("bis zum 1. jedes Monats"); grace, lead and payment
// terms are durations that have nothing to do with the calendar.
//
// These describe the INVOICE cycle only. The licence key's lifetime is deliberately
// NOT derived from them — see license.KeyLifetimeDays. A key lives 90 days and is
// renewed continuously while the subscription is paid; tying it to the billing period
// meant a yearly customer held a 395-day key, so revoking a licence took a year to
// bite. Two different clocks, two different jobs.
//
// One invariant remains, and it is enforced by a test because it failed silently
// once: LeadDays+GraceDays MUST exceed DueDays with room for a bank transfer to
// clear. The first draft had a monthly plan where the payment window was 12 days —
// while the invoice itself was due in 14. The customer would have been cut off before
// their bill was even overdue.
//
// If a test here fails, fix the numbers. Not the test.
type Plan struct {
	Product      string // "pro" — "managed" and "msp" are planned, see below
	Interval     string // "month" | "year"
	NetEUR       float64
	Title        string // invoice title
	Desc         string // invoice line item
	PeriodMonths int
	GraceDays    int
	LeadDays     int
	DueDays      int
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
		// The key survives the period by the payment window, so a transfer that takes
		// a few days does not black out a paying customer. The invoice goes out 10
		// days early and is due in 10 — that leaves 15 days of key for a 10-day term,
		// i.e. 5 days of slack for the transfer to clear.
		PeriodMonths: 1,
		GraceDays:    5,
		LeadDays:     10,
		DueDays:      10,
	},
	PeriodKey("pro", "year"): {
		Product:      "pro",
		Interval:     "year",
		NetEUR:       2990.0,
		Title:        "Vakt Pro — Jahreslizenz",
		Desc:         "Vakt Pro — self-hosted ISMS-Plattform, unbegrenzte Nutzer",
		PeriodMonths: 12,
		GraceDays:    30,
		LeadDays:     21,
		DueDays:      14,
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
//
// Calendar months, not 30-day blocks. "299 € / Monat" has to mean TWELVE invoices in
// a year; with 30-day periods it means 12.17, so a monthly customer would pay 3.639 €
// where the price page promises 3.588 € — and the "~2 Monate gratis" on the annual
// plan (12 × 299 − 2.990 = 598 = 2 × 299) would stop being true. The invoice date
// would also walk backwards through the month (1.3. → 31.3. → 30.4. → 30.5.), which
// is exactly what an accounts-payable department does not want.
//
// The annual plan was 365 days for the same reason: correct until a leap year, then
// the renewal slides a day earlier and keeps sliding.
//
// Known and accepted: periods CHAIN from the previous period end (renewOne), so a
// customer who buys on the 31st is billed on the 28th from February onwards and stays
// there — the clamp is not undone on the way back out. Anchoring to the original
// purchase day instead would need the anchor threaded through the renewal query and
// three call sites, to recover at most three days, once, in the customer's favour.
// Not worth it. It is a decision, not an oversight, and TestPeriodChainSettlesOnTheClampedDay
// pins it.
func (p Plan) Period(start time.Time) (from, to time.Time) {
	return start, addMonthsClamped(start, p.PeriodMonths)
}

// addMonthsClamped adds calendar months and clamps to a day the target month has.
//
// time.AddDate normalises overflow instead of clamping: 31.01. + 1 month is 03.03.,
// not 28.02. Left alone, a customer who bought on the 31st drifts a few days deeper
// into the following month with every renewal, and a 29.02. annual renewal lands on
// 01.03. forever after. Clamping to the last day the month actually has is the rule
// every subscription business uses, and it keeps the anniversary stable.
func addMonthsClamped(t time.Time, months int) time.Time {
	y, m, d := t.Date()
	// Day 1 first: constructing with the original day would normalise before we get
	// a chance to clamp it. time.Date handles month > 12 by rolling the year.
	target := time.Date(y, m+time.Month(months), 1,
		t.Hour(), t.Minute(), t.Second(), t.Nanosecond(), t.Location())
	if last := daysInMonth(target.Year(), target.Month()); d > last {
		d = last
	}
	return time.Date(target.Year(), target.Month(), d,
		t.Hour(), t.Minute(), t.Second(), t.Nanosecond(), t.Location())
}

// daysInMonth: day 0 of the next month IS the last day of this one.
func daysInMonth(year int, m time.Month) int {
	return time.Date(year, m+1, 0, 0, 0, 0, 0, time.UTC).Day()
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

// MaxDiscountPercent caps what may be granted, and 90 is not a round number picked
// for looks.
//
// 100 % would be a 0 € invoice. The customer transfers nothing, so Lexware never
// reports the voucher as "balanced", so settle() never runs — and settle() is the
// ONLY place that issues the full licence key and sets next_invoice_at. The
// customer's trial key would expire on day 45 and the subscription would be dead,
// without a single error anywhere.
//
// Giving Vakt away is a legitimate thing to want, and it has its own path: a free
// licence (is_free, see free.go) skips Lexware entirely — no contact, no invoice,
// nothing to wait for. That is a different thing from charging someone nothing, and
// conflating the two is what breaks.
const MaxDiscountPercent = 90

// ValidateDiscount is the one gate. Every entry point calls it — a percentage that
// reaches the plan maths unchecked would either invoice a negative amount (>100) or
// silently kill the subscription (=100).
func ValidateDiscount(percent int) error {
	if percent < 0 {
		return fmt.Errorf("Rabatt kann nicht negativ sein (%d %%)", percent)
	}
	if percent > MaxDiscountPercent {
		return fmt.Errorf("Rabatt über %d %% ist nicht möglich (%d %% angefragt). "+
			"Ein 100-%%-Rabatt wäre eine 0-€-Rechnung — die wird nie überwiesen, also meldet "+
			"Lexware sie nie als bezahlt, also bekäme der Kunde nie einen Vollschlüssel. "+
			"Wer gratis vergeben will, legt das Abo als FREILIZENZ an (Haken „Freilizenz“ "+
			"beim Anlegen) — dann entsteht gar keine Rechnung",
			MaxDiscountPercent, percent)
	}
	return nil
}

// Charge is what ONE invoice actually costs: list price, the discount granted, and
// the net that ends up on the paper.
//
// It exists so the amount cannot be computed twice. Before, the figure sent to
// Lexware came from TotalEUR (a float) and the figure written to billing_invoices
// came from TotalCents (an integer), and the two agreed only because they were both
// a plain multiplication. Put a percentage between them and they can round apart by
// a cent — after which the panel shows one number, the customer's invoice another,
// and nothing in the system considers that an error. Now there is one computation,
// in cents, and the float handed to Lexware is derived FROM the cents.
//
// The identity ListCents = DiscountCents + NetCents holds by construction: the
// discount is rounded, and the net is the remainder. It is never the other way
// round.
type Charge struct {
	Quantity      int
	Percent       int
	ListCents     int64 // before discount — what the price page says
	DiscountCents int64
	NetCents      int64 // what is invoiced, and what the customer transfers
}

// Charge prices one invoice. Quantity below 1 is clamped rather than rejected: it is
// an entitlement count, and the callers have already bounded it. The percentage is
// NOT clamped — an out-of-range one is a bug at the call site and must have been
// caught by ValidateDiscount, so we refuse to invent a price for it.
func (p Plan) Charge(quantity, discountPercent int) (Charge, error) {
	if quantity < 1 {
		quantity = 1
	}
	if err := ValidateDiscount(discountPercent); err != nil {
		return Charge{}, err
	}

	list := p.Cents() * int64(quantity)

	// Integer maths, rounded half up. With whole-euro list prices and an integer
	// percentage this is exact to the cent — no fraction ever arises, which is what
	// lets our stored amount and the amount Lexware computes from
	// totalDiscountPercentage agree without either of us trusting the other's
	// rounding. TestChargeIsExactToTheCent pins that; if a future plan is priced at
	// 299,50 €, it fails and says so.
	discount := (list*int64(discountPercent) + 50) / 100

	return Charge{
		Quantity:      quantity,
		Percent:       discountPercent,
		ListCents:     list,
		DiscountCents: discount,
		NetCents:      list - discount,
	}, nil
}

// NetEUR is the net amount as Lexware's JSON wants it. Derived from the cents, never
// computed alongside them.
func (c Charge) NetEUR() float64 { return float64(c.NetCents) / 100 }

// ListEUR is the pre-discount total — this is what goes on the invoice LINE, with
// the rebate applied by Lexware at the bottom, so the customer sees what they were
// given instead of just a smaller number.
func (c Charge) ListEUR() float64 { return float64(c.ListCents) / 100 }

// Discounted reports whether there is anything to show the customer at all.
func (c Charge) Discounted() bool { return c.Percent > 0 && c.DiscountCents > 0 }

// LineDesc names what is being sold, so an MSP's invoice does not silently read
// like a single licence at ten times the price.
func (p Plan) LineDesc(quantity int) string {
	if quantity > 1 {
		return fmt.Sprintf("%s — %d Lizenzen à %.2f €", p.Desc, quantity, p.NetEUR)
	}
	return p.Desc
}
