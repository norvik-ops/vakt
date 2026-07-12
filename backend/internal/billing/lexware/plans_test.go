// Copyright (c) 2026 NorvikOps. All rights reserved.
// SPDX-License-Identifier: Elastic-2.0
// See LICENSE file in the project root for terms.

package lexware

import (
	"testing"
	"time"

	"github.com/matharnica/vakt/internal/license"
)

// TestPlanExpiryMatchesLicense guards the invariant the whole billing cycle rests on.
//
// A plan says the customer paid for PeriodDays and their key survives GraceDays
// beyond that. license.KeyExpiry — a different package, written long before plans
// existed — decides how long the signed key is ACTUALLY valid. If the two ever
// disagree, one of two things happens to a paying customer:
//
//	key shorter than period  -> their instance goes dark mid-period, having paid
//	key longer than period   -> they keep Pro after cancelling
//
// Neither shows up in any other test, because both packages are individually
// correct. Only their relationship is wrong.
//
// If this fails, fix the numbers — not the test.
func TestPlanExpiryMatchesLicense(t *testing.T) {
	for key, p := range plans {
		t.Run(key, func(t *testing.T) {
			want := p.PeriodDays + p.GraceDays
			expiry := license.KeyExpiry(p.Interval, "")
			got := int(time.Until(expiry).Hours()/24 + 0.5)
			if got != want {
				t.Errorf("plan %s: PeriodDays+GraceDays = %d, but license.KeyExpiry gives %d days\n"+
					"A paying customer's key would %s.",
					key, want, got,
					map[bool]string{true: "expire before the period they paid for ends", false: "outlive the period they paid for"}[got < want])
			}
		})
	}
}

// TestNextInvoiceLeavesTimeToPay: the follow-up invoice must arrive while the old
// key is still valid, with enough runway to actually pay it. LeadDays before the
// period ends, plus GraceDays after — that is the payment window. If LeadDays were
// 0, the invoice would land on the day the key dies.
func TestNextInvoiceLeavesTimeToPay(t *testing.T) {
	for key, p := range plans {
		t.Run(key, func(t *testing.T) {
			start := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
			_, end := p.Period(start)
			next := p.NextInvoiceAt(end)

			if !next.Before(end) {
				t.Fatalf("plan %s: next invoice (%s) is not before the period ends (%s) — "+
					"the customer would be billed after their key already lapsed",
					key, next.Format("2006-01-02"), end.Format("2006-01-02"))
			}
			// The window between "invoice lands" and "key dies" must cover the
			// invoice's own payment term, plus slack for a bank transfer to clear.
			// Without the slack, a customer who pays on the very last day of the
			// term still goes dark while the money is in flight.
			const transferSlackDays = 3
			window := int(end.AddDate(0, 0, p.GraceDays).Sub(next).Hours() / 24)
			if window < p.DueDays+transferSlackDays {
				t.Errorf("plan %s: %d days between invoice and key expiry, but the invoice "+
					"is due in %d — a customer who pays on the last day of the term would go "+
					"dark while the transfer is still in flight. Need at least %d.",
					key, window, p.DueDays, p.DueDays+transferSlackDays)
			}
		})
	}
}

func TestPlanForRejectsUnknown(t *testing.T) {
	// Managed Hosting and MSP are planned but NOT sellable — Managed is gated on the
	// AVV, MSP on the open ELv2 managed-service question. Until a price lands in the
	// catalogue, asking for one must fail loudly rather than fall back to a default:
	// a silent fallback would invoice a real customer the wrong amount.
	for _, tc := range []struct{ product, interval string }{
		{"managed", "month"},
		{"managed", "year"},
		{"msp", "year"},
		{"pro", "weekly"},
		{"", ""},
	} {
		if _, err := PlanFor(tc.product, tc.interval); err == nil {
			t.Errorf("PlanFor(%q, %q) must fail — it is not something we sell", tc.product, tc.interval)
		}
	}
}

func TestPlanCentsIsExact(t *testing.T) {
	// Money as float64 is a footgun: 299.0*100 can land on 29899.999... and truncate
	// to 29899. One cent wrong on every invoice is the kind of thing an accountant
	// finds a year later.
	for key, p := range plans {
		if got, want := p.Cents(), int64(p.NetEUR*100+0.5); got != want {
			t.Errorf("plan %s: Cents() = %d, want %d", key, got, want)
		}
	}
	if got := plans[PeriodKey("pro", "month")].Cents(); got != 29900 {
		t.Errorf("pro/month = %d cents, want 29900", got)
	}
	if got := plans[PeriodKey("pro", "year")].Cents(); got != 299000 {
		t.Errorf("pro/year = %d cents, want 299000", got)
	}
}
