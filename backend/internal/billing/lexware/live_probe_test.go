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
// Was sie anlegt, und was das kostet — Stand 2026-07-19, ZWEI Tests:
//
//	Profile()                        nichts. Reiner Lesezugriff.
//
//	TestLiveLexwareAcceptsOurInvoices (Rabatt-Probe)
//	  1 Kontakt   "ZZZ TESTKONTAKT (bitte loeschen)"
//	  2 Entwuerfe (0 % und 20 % Rabatt)
//
//	TestLiveLexwareTaxTypes (Steuer-Probe, S130)
//	  3 Kontakte  "ZZZ TESTKONTAKT DE/AT/CH (bitte loeschen)"
//	  bis zu 6 Entwuerfe (je taxType-Kandidat einer; abgelehnte erzeugen keinen)
//
// Zusammen also bis zu 4 Kontakte und 8 Entwuerfe. Alle im Web-UI loeschbar, ohne
// Buchhaltungswirkung. Lexware hat laut Doku KEINEN DELETE-Endpoint fuer Belege —
// das Aufraeumen ist Handarbeit. Beide Tests loggen am Ende, was wegzuraeumen ist.
//
// Sie finalisieren NICHTS. Entwuerfe tragen keine Rechnungsnummer und keine Buchung,
// ein Storno ist deshalb NICHT noetig. Eine finalisierte Rechnung wuerde eine
// fortlaufende Nummer unter der eigenen Steuernummer verbrauchen und waere nur per
// Storno zurueckzuholen; das ist eine Entscheidung, die ein Test nicht treffen darf.
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

// ── Steuer-Probe fuer S130 (DACH) ────────────────────────────────────────────

// taxDraft legt einen Entwurf mit einem bestimmten taxType an und gibt zurueck, was
// Lexware daraus rechnet.
//
// Gibt einen FEHLER zurueck, statt den Test abzubrechen: Eine Ablehnung ist hier ein
// Ergebnis, kein Testfehler. Die Frage der Probe ist gerade, WELCHE Werte der echte
// Mandant annimmt — ein t.Fatal beim ersten abgelehnten Wert wuerde die restlichen
// Faelle nie messen.
func taxDraft(t *testing.T, c *Client, contactID, taxType, note string, ratePct float64) (taxDraftResult, error) {
	t.Helper()
	now := time.Now().Format(lexwareTime)

	req := invoiceRequest{
		VoucherDate: now,
		Address:     invoiceAddress{ContactID: contactID},
		LineItems: []lineItem{{
			Type:        "custom",
			Name:        "ZZZ TEST — Vakt Pro (Steuer-Probe S130)",
			Description: "Testbeleg, bitte loeschen",
			Quantity:    1,
			UnitName:    "Stück",
			UnitPrice: unitPrice{
				Currency:          "EUR",
				NetAmount:         2990,
				TaxRatePercentage: ratePct,
			},
		}},
		TotalPrice:        totalPrice{Currency: "EUR", TotalDiscountPercentage: 0},
		TaxConditions:     taxConditions{TaxType: taxType, TaxTypeNote: note},
		PaymentConditions: paymentConditions{PaymentTermLabel: "Zahlbar innerhalb von 14 Tagen ohne Abzug.", PaymentTermDuration: 14},
		ShippingConditions: shippingConditions{
			ShippingType: "service", ShippingDate: now,
		},
		Title:        "Rechnung",
		Introduction: "TESTBELEG — Steuer-Probe, keine echte Rechnung",
	}

	body, err := json.Marshal(req)
	if err != nil {
		return taxDraftResult{}, err
	}

	// Kein ?finalize=true. Entwurf: keine Rechnungsnummer, keine Buchung, kein Storno noetig.
	raw, err := c.do(context.Background(), http.MethodPost, "/v1/invoices", body, "")
	if err != nil {
		return taxDraftResult{}, err
	}
	var created struct {
		ID string `json:"id"`
	}
	if err := json.Unmarshal(raw, &created); err != nil {
		return taxDraftResult{}, err
	}

	// Die POST-Antwort traegt nur die ID; gerechnet wird erst im gespeicherten Beleg.
	raw, err = c.do(context.Background(), http.MethodGet, "/v1/invoices/"+created.ID, nil, "")
	if err != nil {
		return taxDraftResult{ID: created.ID}, err
	}
	var voucher struct {
		TotalPrice struct {
			TotalNetAmount   float64 `json:"totalNetAmount"`
			TotalGrossAmount float64 `json:"totalGrossAmount"`
			TotalTaxAmount   float64 `json:"totalTaxAmount"`
		} `json:"totalPrice"`
		TaxConditions struct {
			TaxType     string `json:"taxType"`
			TaxTypeNote string `json:"taxTypeNote"`
		} `json:"taxConditions"`
	}
	if err := json.Unmarshal(raw, &voucher); err != nil {
		return taxDraftResult{ID: created.ID}, err
	}

	return taxDraftResult{
		ID:         created.ID,
		Net:        voucher.TotalPrice.TotalNetAmount,
		Tax:        voucher.TotalPrice.TotalTaxAmount,
		Gross:      voucher.TotalPrice.TotalGrossAmount,
		EchoedType: voucher.TaxConditions.TaxType,
		EchoedNote: voucher.TaxConditions.TaxTypeNote,
	}, nil
}

type taxDraftResult struct {
	ID         string
	Net        float64
	Tax        float64
	Gross      float64
	EchoedType string
	EchoedNote string
}

// TestLiveLexwareTaxTypes probiert die taxType-Werte, die S130 braucht, gegen den echten
// Mandanten. Ihre AUSGABE ist das Ergebnis, nicht ihr gruener Haken.
//
// Beantwortet vier Fragen, die keine Doku beantworten kann:
//
//  1. Welche taxType-Werte akzeptiert DIESER Mandant ueberhaupt? Solange er als
//     Kleinunternehmer gefuehrt wird (§ 19), ist gut moeglich, dass Lexware "net" mit
//     19 % ABLEHNT. Das waere kein Fehler, sondern der Beleg dafuer, dass Code-Umstellung
//     und Mandanten-Umstellung zusammen passieren muessen — und dass diese Probe nach dem
//     Wechsel zu wiederholen ist.
//  2. Rechnet Lexware bei "net"/19 die Steuer selbst? (Brutto/Steuer im gespeicherten Beleg)
//  3. Prueft Lexware den taxType gegen das LAND des Kontakts? Deshalb je ein eigener
//     Kontakt fuer DE, AT und CH statt eines einzigen wiederverwendeten.
//  4. Kommt ein explizit gesetztes taxTypeNote unveraendert zurueck — oder ueberschreibt
//     Lexware es mit dem Organisations-Default (heute der § 19-Text)? Das ist die stille
//     Falle aus S130: ein weggelassener Hinweis stempelt den Kleinunternehmer-Satz auf
//     eine Reverse-Charge-Rechnung.
func TestLiveLexwareTaxTypes(t *testing.T) {
	c := liveClient(t)
	ctx := context.Background()

	prof, err := c.Profile(ctx)
	if err != nil {
		t.Fatalf("Profil nicht lesbar — Key ungueltig? %v", err)
	}
	t.Logf("Mandant: %s | smallBusiness=%v | taxType=%s",
		prof.CompanyName, prof.SmallBusiness, prof.TaxType)
	if prof.SmallBusiness {
		t.Log("HINWEIS: Mandant ist noch Kleinunternehmer. Ablehnungen von \"net\" sind " +
			"in diesem Zustand ERWARTBAR und selbst ein Ergebnis — Probe nach der " +
			"Umstellung auf Regelbesteuerung wiederholen.")
	}

	// Ein Kontakt je Land. Die AT-UID ist eine syntaktisch gueltige Dummy-Nummer; lehnt
	// Lexware sie ab, ist auch das ein Befund (dann validiert Lexware gegen VIES).
	countries := []struct{ code, vatID, city string }{
		{"DE", "", "Teststadt"},
		{"AT", "ATU00000000", "Testwien"},
		{"CH", "", "Testzuerich"},
	}
	contactID := map[string]string{}
	for _, cc := range countries {
		id, err := c.CreateContact(ctx, ContactInput{
			CompanyName: "ZZZ TESTKONTAKT " + cc.code + " (bitte loeschen)",
			VATID:       cc.vatID,
			ContactName: "Test",
			Email:       "test@example.invalid",
			Street:      "Teststr. 1", Zip: "12345", City: cc.city, CountryCode: cc.code,
		})
		if err != nil {
			t.Errorf("Kontakt %s nicht anlegbar: %v", cc.code, err)
			continue
		}
		contactID[cc.code] = id
		t.Logf("Kontakt %s angelegt: %s  → im Web-UI loeschen", cc.code, id)
	}

	const rcNote = "Steuerschuldnerschaft des Leistungsempfängers (Reverse Charge)"

	cases := []struct {
		label, country, taxType, note string
		rate                          float64
	}{
		{"DE Regelbesteuerung", "DE", "net", "", 19},
		{"DE heutiger Zustand (Referenz)", "DE", "vatfree", "", 0},
		{"AT Variante A — Leistung § 13b", "AT", "externalService13b", rcNote, 0},
		{"AT Variante B — innergem. Lieferung", "AT", "intraCommunitySupply", rcNote, 0},
		{"CH Variante A — Dienstleistung Drittland", "CH", "thirdPartyCountryService", "Nicht steuerbare sonstige Leistung (Drittland)", 0},
		{"CH Variante B — Ausfuhrlieferung", "CH", "thirdPartyCountryDelivery", "Steuerfreie Ausfuhrlieferung", 0},
	}

	var drafts []string
	for _, tc := range cases {
		id, ok := contactID[tc.country]
		if !ok {
			t.Logf("%-42s ÜBERSPRUNGEN (kein Kontakt fuer %s)", tc.label, tc.country)
			continue
		}
		res, err := taxDraft(t, c, id, tc.taxType, tc.note, tc.rate)
		if res.ID != "" {
			drafts = append(drafts, res.ID)
		}
		if err != nil {
			t.Logf("%-42s ABGELEHNT  taxType=%-26s → %v", tc.label, tc.taxType, err)
			continue
		}
		t.Logf("%-42s AKZEPTIERT taxType=%-26s netto %.2f € | Steuer %.2f € | brutto %.2f €",
			tc.label, tc.taxType, res.Net, res.Tax, res.Gross)
		if res.EchoedType != tc.taxType {
			t.Logf("%-42s   ⚠ Lexware meldet taxType=%q zurueck, gesendet war %q",
				"", res.EchoedType, tc.taxType)
		}
		if tc.note != "" && res.EchoedNote != tc.note {
			t.Logf("%-42s   ⚠ taxTypeNote weicht ab — gesendet %q, zurueck %q",
				"", tc.note, res.EchoedNote)
		}
	}

	t.Logf("AUFRAEUMEN im Lexware-Web-UI: %d Kontakte + %d Rechnungsentwuerfe. "+
		"Die API hat dafuer keinen DELETE-Endpoint. Entwuerfe tragen keine "+
		"Rechnungsnummer und keine Buchung — ein Storno ist NICHT noetig.",
		len(contactID), len(drafts))
}
