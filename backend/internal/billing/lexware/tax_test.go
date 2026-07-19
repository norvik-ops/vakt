// Copyright (c) 2026 NorvikOps. All rights reserved.
// SPDX-License-Identifier: Elastic-2.0
// See LICENSE file in the project root for terms.

package lexware

import (
	"encoding/json"
	"errors"
	"strings"
	"testing"
)

func TestTaxTreatmentFor(t *testing.T) {
	tests := []struct {
		name     string
		in       TaxContext
		wantType string
		wantRate float64
		wantNote string
		wantErr  error
	}{
		// § 19 gewinnt bedingungslos und VOR jeder Länderprüfung. Auch für einen
		// AT-Kunden ohne USt-IdNr. — solange wir keine Umsatzsteuer ausweisen, gibt es
		// nichts zu verlagern.
		{
			name:     "Kleinunternehmer Inland",
			in:       TaxContext{CountryCode: "DE", SmallBusiness: true},
			wantType: "vatfree", wantRate: 0, wantNote: "",
		},
		{
			name:     "Kleinunternehmer schlaegt Laenderlogik",
			in:       TaxContext{CountryCode: "AT", VATIDVerified: false, SmallBusiness: true},
			wantType: "vatfree", wantRate: 0, wantNote: "",
		},

		// Regelbesteuerung.
		{
			name:     "Inland Regelbesteuerung",
			in:       TaxContext{CountryCode: "DE"},
			wantType: "net", wantRate: 19, wantNote: "",
		},
		{
			name:     "EU-Ausland mit gepruefter USt-IdNr",
			in:       TaxContext{CountryCode: "AT", VATIDVerified: true},
			wantType: "externalService13b", wantRate: 0, wantNote: noteEUReverseCharge,
		},
		{
			name:    "EU-Ausland OHNE gepruefte USt-IdNr wird abgelehnt",
			in:      TaxContext{CountryCode: "AT", VATIDVerified: false},
			wantErr: ErrVATIDRequired,
		},
		{
			// Eine ausgefuellte, aber ungepruefte Nummer ist genau der Fall, den
			// VATIDVerified von "Feld nicht leer" unterscheidet.
			name:    "EU-Ausland, Nummer vorhanden aber ungeprueft, wird abgelehnt",
			in:      TaxContext{CountryCode: "NL", VATIDVerified: false},
			wantErr: ErrVATIDRequired,
		},
		{
			name:     "Schweiz ist Drittland",
			in:       TaxContext{CountryCode: "CH"},
			wantType: "thirdPartyCountryService", wantRate: 0, wantNote: noteThirdCountry,
		},
		{
			// Drittland braucht KEINE USt-IdNr. — VIES kennt die Schweiz gar nicht.
			// Wenn dieser Fall je ErrVATIDRequired liefert, ist die EU-Liste kaputt.
			name:     "Drittland ohne USt-IdNr ist zulaessig",
			in:       TaxContext{CountryCode: "CH", VATIDVerified: false},
			wantType: "thirdPartyCountryService", wantRate: 0, wantNote: noteThirdCountry,
		},
		{
			// Die Nicht-EU-Europäer aus der Bestellformular-Liste. Sie sehen europäisch
			// aus und sind steuerlich Drittland — der haeufigste Denkfehler hier.
			name:     "Norwegen ist Drittland, nicht EU",
			in:       TaxContext{CountryCode: "NO"},
			wantType: "thirdPartyCountryService", wantRate: 0, wantNote: noteThirdCountry,
		},
		{
			name:     "Vereinigtes Koenigreich ist Drittland",
			in:       TaxContext{CountryCode: "GB"},
			wantType: "thirdPartyCountryService", wantRate: 0, wantNote: noteThirdCountry,
		},
		{
			name:     "Liechtenstein ist Drittland",
			in:       TaxContext{CountryCode: "LI"},
			wantType: "thirdPartyCountryService", wantRate: 0, wantNote: noteThirdCountry,
		},

		// Normalisierung: Das Land kommt aus einer DB-Spalte ohne Constraint.
		{
			name:     "Kleinschreibung wird normalisiert",
			in:       TaxContext{CountryCode: "at", VATIDVerified: true},
			wantType: "externalService13b", wantRate: 0, wantNote: noteEUReverseCharge,
		},
		{
			name:     "Leerraum wird normalisiert",
			in:       TaxContext{CountryCode: " DE "},
			wantType: "net", wantRate: 19, wantNote: "",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got, err := taxTreatmentFor(tc.in)

			if tc.wantErr != nil {
				if !errors.Is(err, tc.wantErr) {
					t.Fatalf("Fehler %v, erwartet %v", err, tc.wantErr)
				}
				return
			}
			if err != nil {
				t.Fatalf("unerwarteter Fehler: %v", err)
			}
			if got.Type != tc.wantType {
				t.Errorf("Type = %q, erwartet %q", got.Type, tc.wantType)
			}
			if got.RatePct != tc.wantRate {
				t.Errorf("RatePct = %v, erwartet %v", got.RatePct, tc.wantRate)
			}
			if got.Note != tc.wantNote {
				t.Errorf("Note = %q, erwartet %q", got.Note, tc.wantNote)
			}
		})
	}
}

// TestTaxTreatmentWithoutCountryFails haelt fest, dass ein fehlendes Land unter
// Regelbesteuerung KEIN stiller Default ist.
//
// Der Reflex waere, auf "DE" zu fallen — genau der Reflex, der das Bestellformular bis
// 2026-07-18 jeden Auslandskunden als deutschen Kunden hat anlegen lassen. Unter § 19
// war das folgenlos; mit Regelbesteuerung waere es ein falscher Beleg.
func TestTaxTreatmentWithoutCountryFails(t *testing.T) {
	if _, err := taxTreatmentFor(TaxContext{CountryCode: ""}); err == nil {
		t.Fatal("leeres Land wurde akzeptiert — es darf keinen stillen DE-Default geben")
	}
	// Unter § 19 ist das Land dagegen bedeutungslos und darf nicht scheitern.
	if _, err := taxTreatmentFor(TaxContext{CountryCode: "", SmallBusiness: true}); err != nil {
		t.Fatalf("§19 braucht kein Land, scheiterte aber: %v", err)
	}
}

// TestNonEmptyNoteForEveryTaxFreeType ist der Test gegen die stille Falle.
//
// Lexware setzt bei steuerfreien Belegen den Organisations-Default ein, wenn
// taxTypeNote fehlt — und der ist heute der § 19-Kleinunternehmer-Text. Auf einer
// Reverse-Charge- oder Drittlandsrechnung waere das eine falsche Aussage auf einem
// echten Beleg, ohne Fehler und ohne Log.
//
// Ausnahme mit Absicht: der Kleinunternehmer-Fall selbst. Dort IST der Default richtig,
// und ein leerer Note ist die Voraussetzung dafuer, dass sich heute nichts aendert.
func TestNonEmptyNoteForEveryTaxFreeType(t *testing.T) {
	cases := []TaxContext{
		{CountryCode: "AT", VATIDVerified: true},
		{CountryCode: "CH"},
		{CountryCode: "NO"},
	}
	for _, in := range cases {
		got, err := taxTreatmentFor(in)
		if err != nil {
			t.Fatalf("%s: %v", in.CountryCode, err)
		}
		if got.RatePct != 0 {
			continue // kein steuerfreier Typ
		}
		if strings.TrimSpace(got.Note) == "" {
			t.Errorf("%s: taxType %q ist steuerfrei, aber ohne Hinweis — Lexware wuerde "+
				"den §19-Text der Organisation einsetzen", in.CountryCode, got.Type)
		}
	}
}

// TestAmountsAreExactToTheCent prüft die Geldrechnung.
//
// Der Nettobetrag steht als int64 Cent in der Datenbank, und die Rechnung, die der Kunde
// überweist, muss cent-genau dazu passen. Der Bruttobetrag wird deshalb als Netto + Steuer
// GEBILDET, nie unabhängig gerechnet — sonst können zwei Wege auseinanderlaufen, und
// genau dieses Muster steckt schon einmal in diesem Paket (TotalEUR/TotalCents).
func TestAmountsAreExactToTheCent(t *testing.T) {
	tests := []struct {
		name               string
		netCents           int64
		rate               float64
		wantTax, wantGross int64
	}{
		{"Jahreslizenz 19 %", 299000, 19, 56810, 355810},
		{"Monatslizenz 19 %", 29900, 19, 5681, 35581},
		{"steuerfrei bleibt steuerfrei", 299000, 0, 0, 299000},
		{"Null bleibt Null", 0, 19, 0, 0},
		// Mit Rabatt entstehen krumme Netto-Betraege — hier zeigt sich, ob gerundet
		// oder abgeschnitten wird. 20 % Rabatt auf 2.990 € = 2.392 €.
		{"mit 20 % Rabatt", 239200, 19, 45448, 284648},
		// Ein Betrag, der bewusst einen halben Cent erzeugt: 19 % auf 1,05 € = 0,1995 €.
		// Kaufmaennisch gerundet sind das 20 Cent, abgeschnitten waeren es 19.
		{"halber Cent wird aufgerundet", 105, 19, 20, 125},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := amountsFor(tc.netCents, TaxTreatment{Type: "net", RatePct: tc.rate})

			if got.TaxCents != tc.wantTax {
				t.Errorf("Steuer = %d Cent, erwartet %d", got.TaxCents, tc.wantTax)
			}
			if got.GrossCents != tc.wantGross {
				t.Errorf("Brutto = %d Cent, erwartet %d", got.GrossCents, tc.wantGross)
			}
			// Die Identität, die auch der CHECK in Migration 244 erzwingt. Sie kann hier
			// gar nicht verletzt werden — der Test hält fest, dass das so BLEIBT.
			if got.GrossCents != got.NetCents+got.TaxCents {
				t.Errorf("Brutto %d != Netto %d + Steuer %d — der CHECK in Migration 244 "+
					"würde diese Zeile ablehnen", got.GrossCents, got.NetCents, got.TaxCents)
			}
		})
	}
}

// TestSmallBusinessAmountsAreUnchanged: Unter § 19 muss Brutto == Netto bleiben.
//
// Das ist die Betrags-Hälfte der Zusicherung "Risiko null": Solange der Schalter steht,
// darf sich keine einzige gespeicherte Zahl von der heutigen unterscheiden.
func TestSmallBusinessAmountsAreUnchanged(t *testing.T) {
	for _, country := range []string{"DE", "AT", "CH"} {
		tax, err := taxTreatmentFor(TaxContext{CountryCode: country, SmallBusiness: true})
		if err != nil {
			t.Fatalf("%s: %v", country, err)
		}
		got := amountsFor(299000, tax)
		if got.TaxCents != 0 || got.GrossCents != got.NetCents {
			t.Errorf("%s: unter §19 muss Brutto == Netto und Steuer == 0 sein, "+
				"bekam netto %d, Steuer %d, brutto %d",
				country, got.NetCents, got.TaxCents, got.GrossCents)
		}
	}
}

// TestSmallBusinessPayloadIsUnchanged ist die Zusicherung "Risiko null" aus S130.
//
// Sie prueft nicht die Absicht, sondern das Ergebnis: Mit dem Schalter in heutiger
// Stellung muss der steuerrelevante Teil des ausgehenden Lexware-Payloads exakt der
// sein, den der Code vor S130 gesendet hat — taxType "vatfree", Satz 0, KEIN
// taxTypeNote im JSON (omitempty, damit Lexware seinen §19-Baustein setzt).
//
// Wird sie rot, hat jemand das Verhalten im Bestand veraendert, waehrend er glaubte,
// nur den neuen Weg zu bauen.
func TestSmallBusinessPayloadIsUnchanged(t *testing.T) {
	for _, country := range []string{"DE", "AT", "CH", ""} {
		tax, err := taxTreatmentFor(TaxContext{
			CountryCode: country, VATIDVerified: false, SmallBusiness: true,
		})
		if err != nil {
			t.Fatalf("Land %q: %v", country, err)
		}

		body, err := json.Marshal(struct {
			UnitPrice     unitPrice     `json:"unitPrice"`
			TaxConditions taxConditions `json:"taxConditions"`
		}{
			UnitPrice:     unitPrice{Currency: "EUR", NetAmount: 2990, TaxRatePercentage: tax.RatePct},
			TaxConditions: taxConditions{TaxType: tax.Type, TaxTypeNote: tax.Note},
		})
		if err != nil {
			t.Fatal(err)
		}

		const want = `{"unitPrice":{"currency":"EUR","netAmount":2990,"taxRatePercentage":0},` +
			`"taxConditions":{"taxType":"vatfree"}}`
		if string(body) != want {
			t.Errorf("Land %q: Payload hat sich geaendert.\n gesendet: %s\n erwartet: %s",
				country, body, want)
		}
	}
}
