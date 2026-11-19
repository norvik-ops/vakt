// Copyright (c) 2026 NorvikOps. All rights reserved.
// SPDX-License-Identifier: Elastic-2.0
// See LICENSE file in the project root for terms.

package lexware

import "testing"

// Diese Tests pruefen NUR die reine Mapping-Funktion summariesFrom (ueber summaryOf
// und firstBusinessEmail). Die HTTP-Ebene (SearchContacts) laeuft gegen die Konstante
// BaseURL ohne Injektion und wird wie im Paket ueblich nicht per httptest getestet.
func TestSummariesFrom(t *testing.T) {
	t.Run("Firmenname, E-Mail und Ort korrekt gezogen", func(t *testing.T) {
		list := contactList{Content: []contactListEntry{{
			ID:             "id-1",
			Company:        contactListCompany{Name: "Acme GmbH"},
			EmailAddresses: contactEmailAddresses{Business: []string{"ben@acme.de"}},
			Addresses: contactListAddresses{Billing: []contactListAddress{
				{Zip: "10115", City: "Berlin"},
				{Zip: "99999", City: "Zweitadresse"},
			}},
		}}}
		got := summariesFrom(list)
		if len(got) != 1 {
			t.Fatalf("len = %d, want 1", len(got))
		}
		want := ContactSummary{ID: "id-1", CompanyName: "Acme GmbH", Email: "ben@acme.de", Zip: "10115", City: "Berlin"}
		if got[0] != want {
			t.Fatalf("summary = %+v, want %+v", got[0], want)
		}
	})

	t.Run("E-Mail-Fallback business->office->other", func(t *testing.T) {
		cases := []struct {
			name  string
			mails contactEmailAddresses
			want  string
		}{
			{"business gewinnt", contactEmailAddresses{Business: []string{"b@x.de"}, Office: []string{"o@x.de"}}, "b@x.de"},
			{"office wenn business leer", contactEmailAddresses{Office: []string{"o@x.de"}, Other: []string{"ot@x.de"}}, "o@x.de"},
			{"other wenn business+office leer", contactEmailAddresses{Other: []string{"ot@x.de"}}, "ot@x.de"},
			{"private wird NICHT genutzt", contactEmailAddresses{Private: []string{"p@x.de"}}, ""},
			{"gar keine", contactEmailAddresses{}, ""},
		}
		for _, tc := range cases {
			t.Run(tc.name, func(t *testing.T) {
				got := summaryOf(contactListEntry{ID: "x", EmailAddresses: tc.mails})
				if got.Email != tc.want {
					t.Fatalf("Email = %q, want %q", got.Email, tc.want)
				}
			})
		}
	})

	t.Run("kein Ort ohne Billing-Adresse", func(t *testing.T) {
		got := summaryOf(contactListEntry{ID: "x", Company: contactListCompany{Name: "Ohne Adresse"}})
		if got.Zip != "" || got.City != "" {
			t.Fatalf("Zip/City = %q/%q, want leer", got.Zip, got.City)
		}
	})

	t.Run("leere Liste ergibt leere Rueckgabe", func(t *testing.T) {
		got := summariesFrom(contactList{})
		if len(got) != 0 {
			t.Fatalf("len = %d, want 0", len(got))
		}
	})

	t.Run("Begrenzung auf 20", func(t *testing.T) {
		var entries []contactListEntry
		for i := 0; i < 50; i++ {
			entries = append(entries, contactListEntry{ID: "id", Company: contactListCompany{Name: "Firma"}})
		}
		got := summariesFrom(contactList{Content: entries})
		if len(got) != maxContactSummaries {
			t.Fatalf("len = %d, want %d", len(got), maxContactSummaries)
		}
	})
}
