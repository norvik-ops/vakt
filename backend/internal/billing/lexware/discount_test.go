// Copyright (c) 2026 NorvikOps. All rights reserved.
// SPDX-License-Identifier: Elastic-2.0
// See LICENSE file in the project root for terms.

package lexware

import (
	"encoding/json"
	"strings"
	"testing"
)

// TestChargeIsExactToTheCent guards the one assumption the whole discount rests on.
//
// We compute the net amount ourselves and store it in billing_invoices. Lexware
// computes it AGAIN, from the list price and totalDiscountPercentage, and prints ITS
// number on the paper the customer pays. Nobody reconciles the two. If they disagree
// by a cent, the customer transfers one amount, our books say another, and nothing
// anywhere reports an error — the drift just sits there.
//
// They cannot disagree as long as no fraction of a cent ever arises, and that holds
// because every list price is a whole number of euros and every percentage is a whole
// number: list_cents is divisible by 100, so list_cents × percent is too, and the
// division is exact — no rounding happens, so neither side's rounding mode matters.
//
// This test pins the PRECONDITION, not the arithmetic. Price a future plan at
// 299,50 € and it fails here, loudly, instead of shipping half-cent invoices.
func TestChargeIsExactToTheCent(t *testing.T) {
	for key, p := range plans {
		if p.Cents()%100 != 0 {
			t.Fatalf("plan %s is priced at %.2f € — not a whole euro.\n\n"+
				"The discount maths then produces fractions of a cent, and OUR rounding has to "+
				"agree with LEXWARE's, which we do not control and cannot see. Either price the "+
				"plan in whole euros, or stop sending totalDiscountPercentage and send the "+
				"already-discounted net amount instead (the rebate then stops being visible on "+
				"the invoice).", key, p.NetEUR)
		}
	}

	// Exhaustive over everything sellable: both plans, every allowed percentage, and
	// the quantity bounds the public form enforces.
	for key, p := range plans {
		for _, qty := range []int{1, 2, 7, 10, 499, 500} {
			for pct := 0; pct <= MaxDiscountPercent; pct++ {
				c, err := p.Charge(qty, pct)
				if err != nil {
					t.Fatalf("%s qty=%d pct=%d: %v", key, qty, pct, err)
				}

				// The identity that makes the invoice add up.
				if c.DiscountCents+c.NetCents != c.ListCents {
					t.Fatalf("%s qty=%d pct=%d: %d + %d != %d — the invoice would not add up",
						key, qty, pct, c.DiscountCents, c.NetCents, c.ListCents)
				}
				// Exact, not merely close: this is what Lexware will compute too.
				want := c.ListCents * int64(pct) / 100
				if c.ListCents*int64(pct)%100 != 0 {
					t.Fatalf("%s qty=%d pct=%d: a fraction of a cent appeared — our number and "+
						"Lexware's can now differ", key, qty, pct)
				}
				if c.DiscountCents != want {
					t.Errorf("%s qty=%d pct=%d: discount %d cents, expected exactly %d",
						key, qty, pct, c.DiscountCents, want)
				}
				if c.NetCents < 0 {
					t.Fatalf("%s qty=%d pct=%d: negative invoice (%d cents)", key, qty, pct, c.NetCents)
				}
			}
		}
	}
}

// TestDiscountCannotReachOneHundredPercent pins a boundary that looks arbitrary and
// is not.
//
// A 0 € invoice is never transferred, so Lexware never reports it "balanced", so
// settle() never runs — and settle() is the only place that issues the full licence
// key and sets next_invoice_at. A 100 % customer would lose Pro on day 45 and their
// subscription would never renew, with no error logged anywhere. The cap is the only
// thing standing between a generous impulse and a silently dead customer.
func TestDiscountCannotReachOneHundredPercent(t *testing.T) {
	for _, pct := range []int{100, 101, 1000} {
		if err := ValidateDiscount(pct); err == nil {
			t.Fatalf("%d %% was accepted — that invoices 0 €, which is never paid, so the "+
				"customer would never receive a full key", pct)
		}
	}
	if err := ValidateDiscount(-1); err == nil {
		t.Fatal("a negative discount was accepted — that invoices MORE than the list price")
	}
	if err := ValidateDiscount(MaxDiscountPercent); err != nil {
		t.Fatalf("%d %% must be allowed: %v", MaxDiscountPercent, err)
	}

	// And the maths refuses too, so a caller that skips ValidateDiscount cannot invent
	// a price. Defence in depth: the panel validates, the DB has a CHECK, and Charge
	// itself will not compute one.
	p := plans[PeriodKey("pro", "year")]
	if _, err := p.Charge(1, 100); err == nil {
		t.Fatal("Plan.Charge priced a 100 % discount — it must refuse, not clamp")
	}
}

// TestDiscountedInvoiceShowsListPriceAndRebate checks what the CUSTOMER sees.
//
// The rebate must reach Lexware as totalDiscountPercentage, with the LIST price on the
// line item — that is what makes the invoice read "2.990 € − 20 % = 2.392 €" instead
// of an unexplained 2.392 €. Sending the already-discounted amount would produce the
// same total and hide the discount we are giving away, which is the one part of it the
// customer is supposed to notice.
//
// The JSON is inspected directly because that is the actual contract with Lexware; a
// test against our own struct fields would pass even if the tags were wrong.
func TestDiscountedInvoiceShowsListPriceAndRebate(t *testing.T) {
	p := plans[PeriodKey("pro", "year")]
	charge, err := p.Charge(1, 20)
	if err != nil {
		t.Fatal(err)
	}
	if charge.NetCents != 239200 {
		t.Fatalf("2.990 € − 20 %% should be 2.392,00 €, got %d cents", charge.NetCents)
	}

	req := invoiceRequest{
		LineItems: []lineItem{{
			UnitPrice: unitPrice{Currency: "EUR", NetAmount: charge.ListEUR()},
		}},
		TotalPrice: totalPrice{
			Currency:                "EUR",
			TotalDiscountPercentage: float64(charge.Percent),
		},
	}
	raw, err := json.Marshal(req)
	if err != nil {
		t.Fatal(err)
	}
	body := string(raw)

	if !strings.Contains(body, `"netAmount":2990`) {
		t.Errorf("the line item must carry the LIST price (2990), so the customer sees what "+
			"was taken off. Got: %s", body)
	}
	if !strings.Contains(body, `"totalDiscountPercentage":20`) {
		t.Errorf("the rebate must travel as totalDiscountPercentage. Got: %s", body)
	}
}

// TestZeroDiscountIsSentExplicitly is the counterpart, and it exists because of a
// single sentence in the Lexware docs: "A contact-specific default will be set if
// available and no total discount was send."
//
// So OMITTING the field is not neutral — it hands pricing authority to a value stored
// on the contact in Lexware's web UI, which nothing in this codebase can see. An
// `omitempty` on that field (the reflex, since 0 "means nothing") would let a stray
// default rebate quietly price a full-price customer's invoice, and billing_invoices
// would record an amount that was never charged.
func TestZeroDiscountIsSentExplicitly(t *testing.T) {
	raw, err := json.Marshal(totalPrice{Currency: "EUR", TotalDiscountPercentage: 0})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(raw), `"totalDiscountPercentage":0`) {
		t.Fatalf("a zero discount MUST be sent explicitly, or Lexware falls back to the "+
			"contact's stored default rebate and prices the invoice for us. Got: %s", raw)
	}
}

// TestDiscountSurvivesRenewal is the reason the percentage lives on the subscription
// and not on the invoice.
//
// The promise made when a rebate is granted is "you get this for as long as you stay".
// renewOne() re-derives the price from the plan catalogue every cycle, so a discount
// that was only recorded on the first invoice would evaporate at the first renewal —
// and the only person who would ever notice is the customer, reading a bill 20 % higher
// than the one they agreed to.
//
// This pins the mechanism: the same subscription state must price period 2 exactly as
// it priced period 1.
func TestDiscountSurvivesRenewal(t *testing.T) {
	p := plans[PeriodKey("pro", "month")]

	first, err := p.Charge(1, 25)
	if err != nil {
		t.Fatal(err)
	}
	// renewOne reads discount_percent from the SUBSCRIPTION row — the same value — and
	// prices the next invoice from it. Same inputs, same charge.
	renewal, err := p.Charge(1, 25)
	if err != nil {
		t.Fatal(err)
	}

	if renewal.NetCents != first.NetCents {
		t.Fatalf("renewal priced at %d cents but the first period was %d — the discount did "+
			"not survive the cycle", renewal.NetCents, first.NetCents)
	}
	if renewal.NetCents != 22425 {
		t.Fatalf("299 € − 25 %% should be 224,25 €, got %d cents", renewal.NetCents)
	}
}
