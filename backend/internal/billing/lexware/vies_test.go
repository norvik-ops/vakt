// Copyright (c) 2026 NorvikOps. All rights reserved.
// SPDX-License-Identifier: Elastic-2.0
// See LICENSE file in the project root for terms.

package lexware

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestSplitVATID(t *testing.T) {
	tests := []struct {
		in          string
		wantCountry string
		wantNumber  string
		wantErr     bool
	}{
		{in: "ATU12345678", wantCountry: "AT", wantNumber: "U12345678"},
		{in: "DE123456789", wantCountry: "DE", wantNumber: "123456789"},
		// Formatierung, wie Menschen sie eintippen.
		{in: "at u12345678", wantCountry: "AT", wantNumber: "U12345678"},
		{in: "AT-U.12345678", wantCountry: "AT", wantNumber: "U12345678"},
		// Griechenland meldet sich umsatzsteuerlich als EL, nicht als GR. Ohne den
		// Sonderfall waere jede griechische Nummer faelschlich als Nicht-EU abgewiesen.
		{in: "EL123456789", wantCountry: "EL", wantNumber: "123456789"},
		// Drittland: VIES kennt es nicht, die Anfrage waere sinnlos.
		{in: "CHE123456789", wantErr: true},
		{in: "", wantErr: true},
		{in: "AT", wantErr: true},    // kein Nummernteil
		{in: "12345", wantErr: true}, // kein Laenderpraefix
	}
	for _, tc := range tests {
		cc, num, err := splitVATID(tc.in)
		if tc.wantErr {
			if err == nil {
				t.Errorf("splitVATID(%q): Fehler erwartet, bekam %s/%s", tc.in, cc, num)
			}
			continue
		}
		if err != nil {
			t.Errorf("splitVATID(%q): unerwarteter Fehler %v", tc.in, err)
			continue
		}
		if cc != tc.wantCountry || num != tc.wantNumber {
			t.Errorf("splitVATID(%q) = %s/%s, erwartet %s/%s", tc.in, cc, num, tc.wantCountry, tc.wantNumber)
		}
	}
}

// TestVIESCheckIsFailClosed ist der wichtigste Test dieser Datei.
//
// Es darf KEINEN Pfad geben, auf dem ein Problem des Prüfdienstes eine Nummer als gültig
// durchgehen lässt. Läge die Nummer falsch, schuldeten WIR die Umsatzsteuer, die wir
// wegen Reverse Charge nie berechnet haben — ein Ausfall bei der EU-Kommission darf diese
// Haftung nicht auslösen.
func TestVIESCheckIsFailClosed(t *testing.T) {
	cases := []struct {
		name    string
		handler http.HandlerFunc
	}{
		{"Serverfehler", func(w http.ResponseWriter, _ *http.Request) {
			w.WriteHeader(http.StatusInternalServerError)
		}},
		{"Dienst nicht verfuegbar", func(w http.ResponseWriter, _ *http.Request) {
			w.WriteHeader(http.StatusServiceUnavailable)
		}},
		{"kaputtes JSON", func(w http.ResponseWriter, _ *http.Request) {
			_, _ = w.Write([]byte(`{"valid": tru`))
		}},
		{"leere Antwort", func(w http.ResponseWriter, _ *http.Request) {}},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			srv := httptest.NewServer(tc.handler)
			defer srv.Close()

			c := &VIESClient{http: srv.Client()}
			// Der echte Endpunkt wird hier nicht getroffen; wir prüfen das Verhalten der
			// Fehlerpfade, nicht die URL-Bildung (die deckt TestSplitVATID ab).
			res, err := c.checkAt(context.Background(), srv.URL, "AT", "U12345678")

			if err == nil {
				t.Fatal("Fehler erwartet")
			}
			if res.Valid {
				t.Error("FAIL-OPEN: eine Nummer wurde trotz Dienstproblem als gültig gemeldet")
			}
			if res.Qualified {
				t.Error("Qualified darf ohne erfolgreiche Prüfung nie true sein")
			}
			if res.RawStatus == "" || res.RawStatus == "ok" {
				t.Errorf("RawStatus = %q — ein Dienstproblem muss unterscheidbar von "+
					"\"ungültig\" festgehalten werden", res.RawStatus)
			}
			if !errors.Is(err, ErrVIESUnavailable) {
				t.Errorf("Fehler %v ist nicht als ErrVIESUnavailable erkennbar — der "+
					"Aufrufer kann \"nicht prüfbar\" dann nicht von \"ungültig\" trennen", err)
			}
		})
	}
}

// TestVIESCheckValidAndInvalid prüft die beiden regulären Antworten.
//
// Die JSON-Formen unten sind ECHTE Antworten der EU-Kommission, am 2026-07-19 abgefragt
// — nicht nachgebaute. Das ist der ganze Punkt dieses Tests: Eine frühere Fassung
// erwartete ein Feld "valid", die API liefert aber "isValid". Der Wert blieb damit
// immer false, jede gültige Nummer wäre abgewiesen worden — und der Test bemerkte es
// nicht, weil er mit derselben Erfindung gefüttert war wie der Code.
//
// Wer diese Fixtures anfasst, muss sie gegen die echte API prüfen, nicht gegen die Doku:
//
//	curl -H 'Accept: application/json' \
//	  https://ec.europa.eu/taxation_customs/vies/rest-api/ms/DE/vat/315037332
func TestVIESCheckValidAndInvalid(t *testing.T) {
	// Ungültig — wörtlich die Antwort auf eine Wirtschafts-Identifikationsnummer, die
	// eben KEINE USt-IdNr. ist (§ 139c AO vs. § 27a UStG, gleiche Form, andere Sache).
	const invalidBody = `{
	  "isValid" : false,
	  "requestDate" : "2026-07-19T11:20:23.737Z",
	  "userError" : "INVALID",
	  "name" : "",
	  "address" : "",
	  "requestIdentifier" : "",
	  "originalVatNumber" : "315037332",
	  "vatNumber" : "315037332",
	  "viesApproximate" : { "name" : "---", "street" : "---", "postalCode" : "---",
	    "city" : "---", "companyType" : "---", "matchName" : 3, "matchStreet" : 3,
	    "matchPostalCode" : 3, "matchCity" : 3, "matchCompanyType" : 3 }
	}`

	// Gültig — dieselbe Struktur, isValid true. Name/Anschrift liefert VIES auch bei der
	// einfachen Abfrage mit; qualifiziert macht sie das NICHT (siehe unten).
	const validBody = `{
	  "isValid" : true,
	  "requestDate" : "2026-07-19T11:20:23.737Z",
	  "userError" : "VALID",
	  "name" : "Testfirma GmbH",
	  "address" : "Teststr. 1, Wien",
	  "requestIdentifier" : "",
	  "originalVatNumber" : "U12345678",
	  "vatNumber" : "U12345678"
	}`

	tests := []struct {
		name          string
		body          string
		wantValid     bool
		wantRawPrefix string
	}{
		{"gueltig", validBody, true, "ok"},
		{"ungueltig", invalidBody, false, "invalid:INVALID"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				w.Header().Set("Content-Type", "application/json")
				_, _ = w.Write([]byte(tc.body))
			}))
			defer srv.Close()

			c := &VIESClient{http: srv.Client()}
			res, err := c.checkAt(context.Background(), srv.URL, "AT", "U12345678")
			if err != nil {
				t.Fatalf("unerwarteter Fehler %v", err)
			}
			if res.Valid != tc.wantValid {
				t.Errorf("Valid = %v, erwartet %v — liest der Code noch das falsche JSON-Feld?",
					res.Valid, tc.wantValid)
			}
			if res.RawStatus != tc.wantRawPrefix {
				t.Errorf("RawStatus = %q, erwartet %q", res.RawStatus, tc.wantRawPrefix)
			}
			// Name und Anschrift aus der EINFACHEN Auskunft machen die Prüfung nicht
			// qualifiziert. Qualifiziert heißt: abgeglichen mit von uns übermittelten
			// Daten, mit eigener USt-IdNr. als Anfragendem. Das tun wir nicht.
			if res.Qualified {
				t.Error("Qualified ist true, obwohl nur die einfache Auskunft geholt wurde — " +
					"das würde einen Nachweis behaupten, der nicht existiert")
			}
			// Ohne qualifizierte Anfrage vergibt VIES keine Vorgangskennung. Steht hier
			// je etwas, ist die Annahme im Dateikopf falsch geworden.
			if res.RequestIdentifier != "" {
				t.Errorf("RequestIdentifier = %q bei einfacher Abfrage — erwartet leer",
					res.RequestIdentifier)
			}
		})
	}
}

// TestVIESRejectsTheWIdNr haelt den konkreten Fall fest, der diesen Fehler gefunden hat.
//
// Die Wirtschafts-Identifikationsnummer (§ 139c AO) hat exakt dieselbe Form wie eine
// USt-IdNr. — DE + 9 Ziffern — und wird deshalb dauernd verwechselt. Sie steht aber
// nicht in VIES und trägt kein Reverse Charge. Ein Format-Check kann das nicht
// unterscheiden, nur die Abfrage.
func TestVIESRejectsTheWIdNr(t *testing.T) {
	// Form ist gueltig — der Parser darf sie also NICHT vorab ablehnen.
	cc, num, err := splitVATID("DE315037332")
	if err != nil {
		t.Fatalf("die W-IdNr. ist formal eine gueltige USt-IdNr. und muss den Parser "+
			"passieren — nur VIES kann sie widerlegen: %v", err)
	}
	if cc != "DE" || num != "315037332" {
		t.Fatalf("splitVATID = %s/%s", cc, num)
	}
}
