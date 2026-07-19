// Copyright (c) 2026 NorvikOps. All rights reserved.
// SPDX-License-Identifier: Elastic-2.0
// See LICENSE file in the project root for terms.

package admin

import (
	"bytes"
	"strings"
	"testing"
)

// TestEveryPageRenders exists because nothing else in this repo would notice a broken
// admin template until it is opened in a browser.
//
// html/template resolves field names at EXECUTION, not at parse: a page that reads
// {{.Sub.Discount}} against a struct without that field parses cleanly and then fails
// with a 500 the first time somebody loads it. This panel is where invoices are
// approved and money is taken, and it has had no test coverage at all — so this walks
// every page with a populated model and insists it renders.
func TestEveryPageRenders(t *testing.T) {
	r, err := NewRenderer()
	if err != nil {
		t.Fatalf("templates do not parse: %v", err)
	}

	sub := subRow{
		ID: "11111111-1111-1111-1111-111111111111", Company: "Müller Maschinenbau GmbH",
		Email: "buchhaltung@mueller.de", Plan: "pro/year", Quantity: 3, SeatsUsed: 1,
		Status: "bezahlt", NextInvoice: "01.08.2026", Discount: 20,
		MRRCents: 19933, Notes: "Rechnung per Post.",
	}
	detail := subDetail{
		Sub: sub, SeatsLeft: 2, NextNet: "2.392,00 €", NextList: "2.990,00 €", NextGross: "2.846,48 €",
		Invoices: []invoiceRow{{
			LexwareID: "abc", Period: "01.08.2026 – 01.08.2027", Amount: "2.392,00 €",
			Paid: true, PaidOn: "03.08.2026",
		}},
		Licences: []licenceRow{{
			OrgName: "Müller", Kind: "full", Status: "aktiv", Expires: "01.09.2027",
			Key: "vakt_x", Token: "t",
		}},
	}

	pages := map[string]any{
		"dashboard.html":     dashboardData{MRR: "199,33 €", Active: []subRow{sub}, Pending: []subRow{sub}},
		"subscriptions.html": listData{Subs: []subRow{sub}},
		"invoices.html":      invoicesData{Open: []invoiceRow{{LexwareID: "abc", Company: "Müller", Amount: "2.392,00 €"}}, OpenSum: "2.392,00 €", PaidSum: "0,00 €"},
		"licences.html":      licencesData{Licences: []licenceRow{{OrgName: "Müller", Status: "aktiv"}}},
		"subscription.html":  detail,
		"new.html":           newSubData{},
		"tax.html": taxData{
			Quarters: []taxQuarter{{
				Label:   "2026 Q3",
				Buckets: []taxBucket{{TaxType: "net", Label: "Inland, steuerpflichtig", Count: 1, Net: "2.990,00 €", Tax: "568,10 €", Gross: "3.558,10 €"}},
				Rows: []taxRow{{
					Company: "Müller GmbH", Country: "DE", Invoice: "abc",
					Period: "01.08.2026 – 01.08.2027", TaxType: "net", Rate: "19 %",
					Net: "2.990,00 €", Tax: "568,10 €", Gross: "3.558,10 €",
				}},
				Net: "2.990,00 €", Tax: "568,10 €", Gross: "3.558,10 €",
			}},
		},
	}

	for name, data := range pages {
		t.Run(name, func(t *testing.T) {
			var buf bytes.Buffer
			if err := r.Render(&buf, name, data, nil); err != nil {
				t.Fatalf("%s does not render: %v", name, err)
			}
			if buf.Len() == 0 {
				t.Fatalf("%s rendered nothing", name)
			}
		})
	}
}

// TestSubscriptionPageShowsTheRebateInEuros pins what the page must actually SAY.
//
// The person pressing "Freigeben" is about to create a finalised invoice that cannot
// be taken back. "20 %" on its own is not enough to check that against — the amount
// has to be on the screen, in euros, next to the list price it was taken from. A
// template that renders but shows the wrong number is the failure this guards.
func TestSubscriptionPageShowsTheRebateInEuros(t *testing.T) {
	r, err := NewRenderer()
	if err != nil {
		t.Fatal(err)
	}

	var buf bytes.Buffer
	err = r.Render(&buf, "subscription.html", subDetail{
		Sub: subRow{
			ID: "1", Company: "Rabatt AG", Plan: "pro/year", Quantity: 1,
			Status: "angefragt", Discount: 20,
		},
		// Netto 2.392 € nach 20 % Rabatt, brutto mit 19 % USt: 2.846,48 €.
		NextNet: "2.392,00 €", NextList: "2.990,00 €", NextGross: "2.846,48 €",
	}, nil)
	if err != nil {
		t.Fatal(err)
	}
	html := buf.String()

	for _, want := range []string{
		"2.392,00", // der Nettobetrag, auf den der Rabatt wirkt
		"2.990,00", // wovon es abgezogen wurde
		"20",       // der Rabatt selbst
		"2.846,48", // was der Kunde ueberweist
	} {
		if !strings.Contains(html, want) {
			t.Errorf("die Abo-Seite nennt %q nicht — wer freigibt, sieht nicht, was er berechnet", want)
		}
	}

	// Der Freigabe-Knopf muss den BRUTTObetrag tragen, nicht nur "Rechnung erstellen"
	// und nicht den Nettobetrag: Bestaetigt wird eine unumkehrbare Rechnung ueber die
	// Summe, die der Kunde ueberweist. Unter § 19 UStG sind beide gleich, ab der
	// Regelbesteuerung nicht mehr — und dann waere Netto hier schlicht die falsche Zahl.
	if !strings.Contains(html, "Rechnung über 2.846,48 € erstellen") {
		t.Error("der Freigabe-Knopf nennt den Rechnungsbetrag nicht — die Aktion ist " +
			"unumkehrbar, der Betrag gehört auf den Knopf")
	}
	if strings.Contains(html, "Rechnung über 2.392,00 € erstellen") {
		t.Error("der Freigabe-Knopf nennt den NETTObetrag — der Kunde überweist aber brutto")
	}
}

// TestConversionFormOnlyAppearsOnAFreeLicence guards both directions, and both matter.
//
// Missing on a free licence: the conversion is unreachable, and the only way to make a
// design partner pay is a hand-edit in psql — which drops the subscription out of the
// renewal sweep and expires it silently. That is the exact trap the feature exists to
// close.
//
// Present on a paying one: a second click creates a SECOND Lexware contact for a customer
// who already has one.
func TestConversionFormOnlyAppearsOnAFreeLicence(t *testing.T) {
	r, err := NewRenderer()
	if err != nil {
		t.Fatal(err)
	}

	render := func(sub subRow) string {
		var buf bytes.Buffer
		if err := r.Render(&buf, "subscription.html", subDetail{
			Sub: sub, NextNet: "2.990,00 €", NextList: "2.990,00 €", NextGross: "3.558,10 €", CountryCode: "DE",
		}, nil); err != nil {
			t.Fatal(err)
		}
		return buf.String()
	}

	free := render(subRow{ID: "5", Company: "Partner GmbH", Plan: "pro/year", Quantity: 1,
		Status: "bezahlt", IsFree: true})
	if !strings.Contains(free, `action="/subscriptions/5/convert"`) {
		t.Error("eine Freilizenz zeigt kein Umwandeln-Formular — dann bleibt nur der " +
			"Hand-Edit in der DB, und der lässt das Abo lautlos auslaufen")
	}
	// Und der Rabatt-Block gehört hier NICHT hin: ein Rabatt auf null ist eine stille
	// Nulloperation auf einem Geldfeld.
	if strings.Contains(free, `action="/subscriptions/5/discount"`) {
		t.Error("eine Freilizenz zeigt ein Rabatt-Formular — es wäre wirkungslos")
	}

	paying := render(subRow{ID: "6", Company: "Kunde GmbH", Plan: "pro/year", Quantity: 1,
		Status: "bezahlt", IsFree: false})
	if strings.Contains(paying, `action="/subscriptions/6/convert"`) {
		t.Error("ein zahlendes Abo zeigt das Umwandeln-Formular — ein zweiter Klick legte " +
			"einen zweiten Lexware-Kontakt an")
	}
}

// TestDiscountIsEditableOnALiveSubscription guards a mistake that was actually made
// while building this: the rebate form was nested inside the {{if angefragt}} block.
//
// It renders perfectly — and a paid customer's discount can then never be changed,
// which is the case that matters most, because that is where it takes effect at the
// next renewal.
func TestDiscountIsEditableOnALiveSubscription(t *testing.T) {
	r, err := NewRenderer()
	if err != nil {
		t.Fatal(err)
	}

	var buf bytes.Buffer
	err = r.Render(&buf, "subscription.html", subDetail{
		Sub:     subRow{ID: "7", Company: "Bezahlt GmbH", Plan: "pro/month", Quantity: 1, Status: "bezahlt"},
		NextNet: "299,00 €", NextList: "299,00 €", NextGross: "355,81 €",
	}, nil)
	if err != nil {
		t.Fatal(err)
	}

	if !strings.Contains(buf.String(), `action="/subscriptions/7/discount"`) {
		t.Fatal("ein BEZAHLTES Abo zeigt kein Rabatt-Formular — genau dort wird der Rabatt " +
			"gebraucht, er greift ab der nächsten Verlängerung")
	}
}

// TestTaxAnomalyCatchesTheExpensiveCases haelt fest, WOFUER die Steueruebersicht da ist.
//
// Sie ist eine Kontrollansicht: Sie kann einen falschen Beleg nicht mehr verhindern, aber
// sie kann verhindern, dass er unbemerkt in eine Meldung wandert. Wenn diese Faelle nicht
// mehr auffallen, ist die Seite Dekoration.
func TestTaxAnomalyCatchesTheExpensiveCases(t *testing.T) {
	tests := []struct {
		name       string
		country    string
		taxType    string
		rate       float64
		vatValid   bool
		wantWarn   bool
		wantSevere bool
	}{
		{
			// Der teuerste Fall: Steuer geschuldet, aber nicht ausgewiesen (§ 14c UStG).
			// Kein Fehler, kein Log — nur ein falscher Beleg.
			name: "net bei 0 Prozent", country: "DE", taxType: "net", rate: 0,
			wantWarn: true, wantSevere: true,
		},
		{
			// Ohne gueltige USt-IdNr. traegt Reverse Charge nicht, und die nicht
			// berechnete Steuer bleibt bei uns haengen.
			name: "EU-Ausland ohne gepruefte USt-IdNr", country: "AT",
			taxType: "externalService13b", rate: 0, vatValid: false,
			wantWarn: true, wantSevere: true,
		},
		{
			name: "EU-Ausland MIT gepruefter USt-IdNr", country: "AT",
			taxType: "externalService13b", rate: 0, vatValid: true,
			wantWarn: false,
		},
		{
			name: "Inland korrekt", country: "DE", taxType: "net", rate: 19,
			wantWarn: false,
		},
		{
			// Drittland braucht keine USt-IdNr. — VIES kennt die Schweiz nicht.
			// Ein Warnhinweis hier waere ein Fehlalarm, und Fehlalarme machen die
			// Seite unbrauchbar: Wer jede Woche fuenf falsche sieht, liest den
			// sechsten, echten, nicht mehr.
			name: "Schweiz ohne USt-IdNr ist korrekt", country: "CH",
			taxType: "thirdPartyCountryService", rate: 0, vatValid: false,
			wantWarn: false,
		},
		{
			name: "Norwegen ist Drittland, kein EU-Fehlalarm", country: "NO",
			taxType: "thirdPartyCountryService", rate: 0, vatValid: false,
			wantWarn: false,
		},
		{
			// Nach dem Wechsel in die Regelbesteuerung erklaerungsbeduerftig,
			// aber kein Defekt — deshalb Hinweis ohne Severe.
			name: "vatfree wird milde markiert", country: "DE",
			taxType: "vatfree", rate: 0,
			wantWarn: true, wantSevere: false,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			warn, severe := taxAnomaly(tc.country, tc.taxType, tc.rate, tc.vatValid)
			if (warn != "") != tc.wantWarn {
				t.Errorf("Warnung = %q, erwartet warn=%v", warn, tc.wantWarn)
			}
			if severe != tc.wantSevere {
				t.Errorf("Severe = %v, erwartet %v (Warnung: %q)", severe, tc.wantSevere, warn)
			}
		})
	}
}
