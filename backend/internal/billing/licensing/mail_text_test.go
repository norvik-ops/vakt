// Copyright (c) 2026 NorvikOps. All rights reserved.
// SPDX-License-Identifier: Elastic-2.0
// See LICENSE file in the project root for terms.

package licensing

import (
	"strings"
	"testing"
	"time"
)

// The mail is the only statement we make to a paying customer about what he bought.
// It used to say "volle Jahreslaufzeit" regardless of the plan: a Monatslizenz
// customer paid 299 €, read that he was getting a full year, and got a key that
// stops after 30 days. Nothing failed — the text was simply wrong, and it stayed
// wrong because no test ever read it.
func TestLicenceMailNeverPromisesAYearToAMonthlyCustomer(t *testing.T) {
	expires := time.Date(2026, 8, 26, 12, 0, 0, 0, time.UTC)

	for _, tc := range []struct {
		name     string
		interval string
		trial    bool
	}{
		{"monthly, invoice sent", "month", true},
		{"monthly, paid", "month", false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			_, body := licenseMail(Request{
				OrgName:  "Beispiel GmbH",
				Email:    "kunde@example.com",
				Interval: tc.interval,
				Trial:    tc.trial,
			}, "vakt_key", expires)

			// Deliberately the bare word: "Jahreslaufzeit", "das volle Jahr" and
			// "Jahreslizenz" are all lies in a monthly mail, and a test that lists
			// the phrasings we happen to use today would miss the next one.
			if strings.Contains(body, "Jahr") {
				t.Fatalf("Monatslizenz-Mail spricht von einem Jahr:\n%s", body)
			}
			if !strings.Contains(body, "einen vollen Monat") {
				t.Errorf("Monatslizenz-Mail benennt die Laufzeit nicht:\n%s", body)
			}
		})
	}
}

// The expiry date is stated, not described — a date cannot drift away from the key
// it was read out of, whereas any wording eventually can.
func TestLicenceMailStatesTheRealExpiryDate(t *testing.T) {
	expires := time.Date(2027, 7, 12, 9, 30, 0, 0, time.UTC)

	for _, trial := range []bool{true, false} {
		_, body := licenseMail(Request{
			OrgName: "Beispiel GmbH", Email: "kunde@example.com",
			Interval: "year", Trial: trial,
		}, "vakt_key", expires)

		if !strings.Contains(body, "12.07.2027") {
			t.Errorf("trial=%v: Ablaufdatum fehlt in der Mail:\n%s", trial, body)
		}
		if !strings.Contains(body, "ein volles Jahr") {
			t.Errorf("trial=%v: Jahreslizenz-Mail benennt die Laufzeit nicht", trial)
		}
	}
}

// termOf must branch exactly like license.KeyExpiry, which treats anything that is
// not "year" as a month. If it ever defaulted the other way, an empty interval would
// promise a year and hand over a 35-day key — the original bug, reintroduced.
func TestTermOfDefaultsToTheShorterPeriod(t *testing.T) {
	if got := termOf(""); got != "einen vollen Monat" {
		t.Errorf("termOf(%q) = %q — ein unbekanntes Intervall darf nie ein Jahr versprechen", "", got)
	}
	if got := termOf("year"); got != "ein volles Jahr" {
		t.Errorf("termOf(\"year\") = %q", got)
	}
}
