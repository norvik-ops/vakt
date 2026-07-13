// Copyright (c) 2026 NorvikOps. All rights reserved.
// SPDX-License-Identifier: Elastic-2.0
// See LICENSE file in the project root for terms.

//go:build lexwarelive

// Diese Datei spricht mit der ECHTEN Lexware-API und dem ECHTEN Mandanten.
//
// Es gibt keinen Sandbox-Mandanten. Alles, was hier entsteht, entsteht in den echten
// Buechern — deshalb steht sie hinter einem eigenen Build-Tag (`lexwarelive`), das
// weder CI noch `go test ./...` je setzt. Sie laeuft nur, wenn jemand sie ausdruecklich
// meint:
//
//	VAKT_LEXWARE_API_KEY="$(cat /tmp/lexware.key)" \
//	  go test -tags=lexwarelive -run TestLive -v ./internal/billing/lexware/
//
// Was sie anlegt, und was das kostet:
//
//	Profile()        nichts. Reiner Lesezugriff.
//	CreateContact()  EINEN Kontakt, benannt "ZZZ TESTKONTAKT (bitte loeschen)".
//	                 Im Web-UI loeschbar, keine Buchhaltungswirkung.
//	Rechnungsentwurf ZWEI Entwuerfe (0 % und 20 %). KEINE Rechnungsnummer, keine
//	                 Buchung. Lexware hat laut Doku KEINEN DELETE-Endpoint fuer
//	                 Belege — die Entwuerfe muessen im Web-UI von Hand weg.
//
// Sie finalisiert NICHTS. Eine finalisierte Rechnung verbraucht eine fortlaufende
// Nummer unter der eigenen Steuernummer und ist nur per Storno zurueckzuholen; das ist
// eine Entscheidung, die ein Test nicht treffen darf.
package lexware

import (
	"context"
	"encoding/json"
	"net/http"
	"os"
	"testing"
	"time"
)

func liveClient(t *testing.T) *Client {
	t.Helper()
	key := os.Getenv("VAKT_LEXWARE_API_KEY")
	if key == "" {
		t.Skip("VAKT_LEXWARE_API_KEY nicht gesetzt")
	}
	return New(key)
}

// draftTotals legt einen Rechnungs-ENTWURF an (ohne ?finalize=true) und gibt zurueck,
// was Lexware selbst als Nettobetrag ausgerechnet hat.
//
// Der Request wird mit denselben Structs gebaut wie in CreateInvoice — eine
// nachgebaute Variante wuerde genau das nicht pruefen, worum es geht.
func draftTotals(t *testing.T, c *Client, contactID string, charge Charge) (float64, string) {
	t.Helper()
	now := time.Now().Format(lexwareTime)

	req := invoiceRequest{
		VoucherDate: now,
		Address:     invoiceAddress{ContactID: contactID},
		LineItems: []lineItem{{
			Type:        "custom",
			Name:        "ZZZ TEST — Vakt Pro",
			Description: "Testbeleg, bitte loeschen",
			Quantity:    1,
			UnitName:    "Stück",
			UnitPrice: unitPrice{
				Currency:          "EUR",
				NetAmount:         charge.ListEUR(),
				TaxRatePercentage: 0,
			},
		}},
		TotalPrice: totalPrice{
			Currency:                "EUR",
			TotalDiscountPercentage: float64(charge.Percent),
		},
		TaxConditions: taxConditions{TaxType: "vatfree"},
		PaymentConditions: paymentConditions{
			PaymentTermLabel:    "Zahlbar innerhalb von 14 Tagen ohne Abzug.",
			PaymentTermDuration: 14,
		},
		ShippingConditions: shippingConditions{ShippingType: "service", ShippingDate: now},
		Title:              "Rechnung",
		Introduction:       "TESTBELEG",
	}

	body, err := json.Marshal(req)
	if err != nil {
		t.Fatal(err)
	}

	// Kein ?finalize=true. Entwurf.
	raw, err := c.do(context.Background(), http.MethodPost, "/v1/invoices", body, "")
	if err != nil {
		t.Fatalf("Lexware lehnt den Beleg mit %d %% Rabatt ab: %v", charge.Percent, err)
	}

	var out struct {
		ID string `json:"id"`
	}
	if err := json.Unmarshal(raw, &out); err != nil {
		t.Fatal(err)
	}

	// Zurueckholen: die POST-Antwort traegt nur die ID, die berechneten Summen stehen
	// erst im gespeicherten Beleg.
	raw, err = c.do(context.Background(), http.MethodGet, "/v1/invoices/"+out.ID, nil, "")
	if err != nil {
		t.Fatalf("Entwurf %s nicht lesbar: %v", out.ID, err)
	}
	var voucher struct {
		TotalPrice struct {
			TotalNetAmount float64 `json:"totalNetAmount"`
		} `json:"totalPrice"`
	}
	if err := json.Unmarshal(raw, &voucher); err != nil {
		t.Fatal(err)
	}
	return voucher.TotalPrice.TotalNetAmount, out.ID
}

// TestLiveLexwareAcceptsOurInvoices ist die Probe, die kein Unit-Test ersetzen kann.
//
// Sie beantwortet drei Fragen, und die erste ist die wichtigste — sie betrifft nicht den
// Rabatt, sondern JEDE Rechnung:
//
//  1. Nimmt Lexware ein explizites `totalDiscountPercentage: 0` an?
//     Wir senden es seit dem Rabatt IMMER, statt es wegzulassen, weil ein fehlendes Feld
//     laut Doku einen am Kontakt hinterlegten Default-Rabatt zieht — einen Wert, den
//     dieser Code nicht sehen kann und der die Rechnung fuer uns bepreisen wuerde.
//     Lehnt Lexware die 0 ab, schlaegt ab sofort jede Rechnung fehl, auch die ohne
//     Rabatt. Das waere die einzige Regression, die der Rabatt eingeschleppt haben kann.
//
//  2. Nimmt Lexware `totalDiscountPercentage: 20` an?
//
//  3. Stimmt Lexwares selbst gerechneter Nettobetrag EXAKT mit unserem ueberein?
//     Wir schreiben unseren Cent-Betrag in billing_invoices, Lexware druckt seinen auf
//     das Papier, das der Kunde bezahlt — und niemand gleicht die beiden ab. Sie koennen
//     nur uebereinstimmen, wenn nirgends ein Bruchteil eines Cents entsteht.
//     TestChargeIsExactToTheCent behauptet das gegen unsere eigene Annahme. Hier wird es
//     gegen Lexware geprueft.
func TestLiveLexwareAcceptsOurInvoices(t *testing.T) {
	c := liveClient(t)
	ctx := context.Background()

	// Vorab: Die Annahme, auf der taxType "vatfree" steht.
	prof, err := c.Profile(ctx)
	if err != nil {
		t.Fatalf("Profil nicht lesbar — Key ungueltig? %v", err)
	}
	t.Logf("Mandant: %s | smallBusiness=%v | taxType=%s",
		prof.CompanyName, prof.SmallBusiness, prof.TaxType)
	if !prof.SmallBusiness {
		t.Errorf("Der Mandant ist NICHT mehr als Kleinunternehmer (§19) gefuehrt — " +
			"CreateInvoice sendet aber fest taxType \"vatfree\". Das bricht, sobald " +
			"Lexware Umsatzsteuer erwartet.")
	}

	contactID, err := c.CreateContact(ctx, ContactInput{
		CompanyName: "ZZZ TESTKONTAKT (bitte loeschen)",
		ContactName: "Test",
		Email:       "test@example.invalid",
		Street:      "Teststr. 1", Zip: "12345", City: "Teststadt", CountryCode: "DE",
	})
	if err != nil {
		t.Fatalf("CreateContact — genau der Aufruf, den ConvertToPaid macht — schlaegt fehl: %v", err)
	}
	t.Logf("Kontakt angelegt: %s  → im Web-UI loeschen", contactID)

	plan := plans[PeriodKey("pro", "year")]

	for _, pct := range []int{0, 20} {
		charge, err := plan.Charge(1, pct)
		if err != nil {
			t.Fatal(err)
		}

		got, draftID := draftTotals(t, c, contactID, charge)
		want := charge.NetEUR()

		t.Logf("%2d %% Rabatt → Entwurf %s: Lexware rechnet %.2f €, wir %.2f €",
			pct, draftID, got, want)

		// Cent-genau. Ein Vergleich mit Toleranz waere hier sinnlos: Die Behauptung ist
		// gerade, dass NIE ein Bruchteil eines Cents entsteht.
		if int64(got*100+0.5) != charge.NetCents {
			t.Errorf("Betragsdrift bei %d %%: Lexware %.2f €, wir %.2f €.\n\n"+
				"Der Kunde ueberweist Lexwares Zahl, unsere Buecher speichern unsere. "+
				"Niemand gleicht die beiden ab — die Differenz bliebe unentdeckt.",
				pct, got, want)
		}
	}

	t.Log("AUFRAEUMEN: 1 Kontakt + 2 Rechnungsentwuerfe im Lexware-Web-UI loeschen. " +
		"Die API hat dafuer keinen Endpoint.")
}
