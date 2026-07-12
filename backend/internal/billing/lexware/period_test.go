// Copyright (c) 2026 NorvikOps. All rights reserved.
// SPDX-License-Identifier: Elastic-2.0
// See LICENSE file in the project root for terms.

package lexware

import (
	"testing"
	"time"
)

func d(y int, m time.Month, day int) time.Time {
	return time.Date(y, m, day, 10, 0, 0, 0, time.UTC)
}

// A month is a calendar month. It used to be 30 days, which meant "299 € / Monat"
// billed 12.17 times a year (3.639 € instead of 3.588 €) and the invoice date walked
// backwards through the month: 1.3. → 31.3. → 30.4. → 30.5.
func TestMonthlyPeriodIsACalendarMonth(t *testing.T) {
	p, err := PlanFor("pro", "month")
	if err != nil {
		t.Fatal(err)
	}

	for _, tc := range []struct {
		start, want time.Time
		why         string
	}{
		{d(2026, time.March, 1), d(2026, time.April, 1), "31-day month"},
		{d(2026, time.February, 1), d(2026, time.March, 1), "28-day month"},
		{d(2026, time.January, 31), d(2026, time.February, 28), "31.01. must clamp to 28.02., not overflow to 03.03."},
		{d(2028, time.January, 31), d(2028, time.February, 29), "leap year: the 29th exists"},
		{d(2026, time.December, 15), d(2027, time.January, 15), "across the year boundary"},
		{d(2026, time.May, 31), d(2026, time.June, 30), "31.05. clamps to 30.06."},
	} {
		if _, got := p.Period(tc.start); !got.Equal(tc.want) {
			t.Errorf("%s: Period(%s) endete %s, erwartet %s",
				tc.why, tc.start.Format("02.01.2006"), got.Format("02.01.2006"), tc.want.Format("02.01.2006"))
		}
	}
}

// Twelve monthly invoices per calendar year — the number the price page is built on:
// 12 × 299 € = 3.588 €, and the annual plan at 2.990 € saves exactly 598 €, i.e. the
// "~2 Monate gratis" that Pricing.astro claims. With 30-day periods it was 12.17.
func TestTwelveMonthlyPeriodsCoverExactlyOneYear(t *testing.T) {
	p, _ := PlanFor("pro", "month")

	at := d(2026, time.March, 1)
	for i := 0; i < 12; i++ {
		_, at = p.Period(at)
	}
	if want := d(2027, time.March, 1); !at.Equal(want) {
		t.Fatalf("12 Monatsperioden ab 01.03.2026 enden am %s, erwartet %s",
			at.Format("02.01.2006"), want.Format("02.01.2006"))
	}
}

// A year is a calendar year, so a leap day does not shift the renewal.
func TestYearlyPeriodIsACalendarYear(t *testing.T) {
	p, _ := PlanFor("pro", "year")

	if _, got := p.Period(d(2026, time.July, 12)); !got.Equal(d(2027, time.July, 12)) {
		t.Errorf("Jahresperiode ab 12.07.2026 endet %s, erwartet 12.07.2027", got.Format("02.01.2006"))
	}
	// 2028 is a leap year, 2029 is not: the 29th has to clamp, not roll into March.
	if _, got := p.Period(d(2028, time.February, 29)); !got.Equal(d(2029, time.February, 28)) {
		t.Errorf("Jahresperiode ab 29.02.2028 endet %s, erwartet 28.02.2029", got.Format("02.01.2006"))
	}
}

// Documents the accepted consequence of clamping: renewOne chains each period from the
// previous end, so a purchase on the 31st settles on the 28th and stays there. If this
// ever needs to be an anniversary instead, the anchor day has to be threaded through
// the renewal query — this test is the reminder that the current behaviour was chosen.
func TestPeriodChainSettlesOnTheClampedDay(t *testing.T) {
	p, _ := PlanFor("pro", "month")

	at := d(2026, time.January, 31)
	got := make([]string, 0, 3)
	for i := 0; i < 3; i++ {
		_, at = p.Period(at)
		got = append(got, at.Format("02.01."))
	}

	want := []string{"28.02.", "28.03.", "28.04."}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("Kette ab 31.01. lief %v, erwartet %v (geklemmt, nicht Jahrestag)", got, want)
		}
	}
}
