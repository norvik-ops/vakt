// Copyright (c) 2026 NorvikOps. All rights reserved.
// SPDX-License-Identifier: Elastic-2.0
// See LICENSE file in the project root for terms.

package lexware

import "testing"

// Diese Tests pruefen NUR die reine Auswahlfunktion pickUniqueExactEmailContact.
// Die HTTP-Ebene (findCustomerContactByEmail) laeuft gegen die Konstante BaseURL
// ohne Base-URL-Injektion und wird im Paket per *_real_test.go abgedeckt, nicht
// per httptest.
//
// Der Kern ist die Absicherung gegen den Substring-Filter von Lexware: Nur ein
// exakter, eindeutiger Treffer darf wiederverwendet werden; alles andere fuehrt
// zum Neuanlegen, nie zum Raten.
func TestPickUniqueExactEmailContact(t *testing.T) {
	const want = "ben@acme.de"

	cases := []struct {
		name    string
		content []contactListEntry
		wantID  string
		wantOK  bool
	}{
		{
			name: "ein exakter business-Treffer",
			content: []contactListEntry{
				{ID: "id-1", EmailAddresses: contactEmailAddresses{Business: []string{want}}},
			},
			wantID: "id-1",
			wantOK: true,
		},
		{
			name: "nur Substring, kein exakter Treffer",
			content: []contactListEntry{
				{ID: "id-1", EmailAddresses: contactEmailAddresses{Business: []string{"benjamin@acme.de"}}},
				{ID: "id-2", EmailAddresses: contactEmailAddresses{Business: []string{"ben@acme.december.example"}}},
			},
			wantID: "",
			wantOK: false,
		},
		{
			name: "zwei exakte Treffer mit verschiedenen ids (mehrdeutig)",
			content: []contactListEntry{
				{ID: "id-1", EmailAddresses: contactEmailAddresses{Business: []string{want}}},
				{ID: "id-2", EmailAddresses: contactEmailAddresses{Office: []string{want}}},
			},
			wantID: "",
			wantOK: false,
		},
		{
			name: "exakter Treffer im office-Array",
			content: []contactListEntry{
				{ID: "id-9", EmailAddresses: contactEmailAddresses{Office: []string{want}}},
			},
			wantID: "id-9",
			wantOK: true,
		},
		{
			name:    "leeres content",
			content: nil,
			wantID:  "",
			wantOK:  false,
		},
		{
			name: "case-insensitiver Treffer",
			content: []contactListEntry{
				{ID: "id-7", EmailAddresses: contactEmailAddresses{Private: []string{"Ben@ACME.de"}}},
			},
			wantID: "id-7",
			wantOK: true,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			gotID, gotOK := pickUniqueExactEmailContact(tc.content, want)
			if gotID != tc.wantID || gotOK != tc.wantOK {
				t.Fatalf("pickUniqueExactEmailContact() = (%q, %v), want (%q, %v)",
					gotID, gotOK, tc.wantID, tc.wantOK)
			}
		})
	}
}
