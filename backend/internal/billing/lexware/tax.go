// Copyright (c) 2026 NorvikOps. All rights reserved.
// SPDX-License-Identifier: Elastic-2.0
// See LICENSE file in the project root for terms.

package lexware

import (
	"fmt"
	"strings"
)

// Die steuerliche Einordnung eines Verkaufs — die EINZIGE Stelle, an der diese Logik lebt.
//
// Warum eine reine Funktion und nicht zwei Zuweisungen im Request-Bau: `taxType` und
// `taxRatePercentage` MÜSSEN zusammen gesetzt werden. Wer den Typ auf "net" stellt und
// den Satz bei 0 vergisst, erzeugt eine Rechnung mit 0 % ausgewiesener Umsatzsteuer, die
// trotzdem geschuldet wird — § 14c UStG, und zwar **ohne Fehler und ohne Log**. ADR-0073
// markiert genau diese Zeile (`client.go`, `TaxRatePercentage: 0`) als die gefährlichste
// beim Wechsel. Solange beide Werte nur gemeinsam aus einer Funktion herausfallen, ist
// dieser Fehler nicht mehr formulierbar.
//
// Siehe ADR-0074 (warum wir die Umsatzsteuer selbst tragen) und
// docs/stories/s130-umsatzsteuer-dach.md (Zuordnung, offene Punkte).

// TaxTreatment ist, was auf den Beleg geht.
type TaxTreatment struct {
	Type    string  // Lexware taxType
	RatePct float64 // Steuersatz der Position
	Note    string  // taxTypeNote — Pflichthinweis bei steuerfreien Typen
}

// TaxContext ist alles, was die Einordnung braucht.
type TaxContext struct {
	CountryCode   string // ISO-3166-1 alpha-2, Land des Kunden
	VATIDVerified bool   // USt-IdNr. qualifiziert geprüft (VIES) — NICHT bloß "ausgefüllt"
	SmallBusiness bool   // § 19 UStG: wir weisen keine Umsatzsteuer aus
}

// Die taxType-Werte für Auslandsfälle stehen bewusst als Konstanten hier und nicht
// verstreut im Code: Welcher Wert für eine Softwarelizenz fachlich richtig ist, ist eine
// Frage an den Steuerberater (S130, offene Punkte 1 und 2). Fällt die Antwort anders aus,
// ist das eine Zeile — nicht eine Suche durch den Verkaufspfad.
const (
	// Inland, Regelbesteuerung.
	taxTypeDomestic = "net"
	// Heutiger Zustand unter § 19 UStG.
	taxTypeSmallBusiness = "vatfree"
	// EU-Ausland B2B. Alternative laut Lexware-API: "intraCommunitySupply" — das ist
	// jedoch die innergemeinschaftliche LIEFERUNG (Waren). Eine Lizenz ist eine sonstige
	// Leistung, deshalb der § 13b-Wert. Bestätigung ausstehend.
	taxTypeEUReverseCharge = "externalService13b"
	// Drittland (u. a. CH). Alternative: "thirdPartyCountryDelivery" — das ist die
	// Ausfuhrlieferung (Waren). Bestätigung ausstehend.
	taxTypeThirdCountry = "thirdPartyCountryService"

	// Regelsteuersatz Inland.
	vatRateDE = 19.0
)

// Pflichthinweise. Der Wortlaut ist ebenfalls ein offener Punkt für den Steuerberater
// (S130, Punkt 3 und 4) — aber er darf NIE leer bleiben, wenn der Typ steuerfrei ist.
//
// Grund: Lexware setzt bei steuerfreien Belegen den Organisations-Default ein, wenn
// taxTypeNote fehlt ("When omitted Lexware sets the organization's default"). Dieser
// Default ist heute der § 19-Kleinunternehmer-Text. Auf einer Reverse-Charge-Rechnung
// stünde damit eine Aussage, die schlicht falsch ist — und diese Zeile lebt in den
// Lexware-Einstellungen, also außerhalb der Versionskontrolle: im Diff unsichtbar.
const (
	noteEUReverseCharge = "Steuerschuldnerschaft des Leistungsempfängers (Reverse Charge)"
	noteThirdCountry    = "Nicht im Inland steuerbare sonstige Leistung (Drittland)"
)

// ErrVATIDRequired sagt, dass der Verkauf so nicht abgerechnet werden darf.
//
// Ohne qualifiziert geprüfte USt-IdNr. lässt sich die Unternehmereigenschaft des
// EU-Auslandskunden nicht nachweisen, also greift kein Reverse Charge. Der Umsatz
// rutschte dann in die B2C-Behandlung für elektronisch erbrachte Leistungen — und
// damit ins OSS-Verfahren, das wir bewusst nicht betreiben (S130, "Nicht in Scope").
//
// Der Fehler stoppt die Rechnung, statt sie falsch zu stellen. Das ist Absicht: Eine
// finalisierte Lexware-Rechnung ist über die API nicht zurückzunehmen.
var ErrVATIDRequired = fmt.Errorf("lexware: EU-Auslandsverkauf ohne geprüfte USt-IdNr. — Reverse Charge nicht möglich")

// euCountries sind die 27 Mitgliedstaaten.
//
// Bewusst NICHT dieselbe Liste wie die Auswahl im Bestellformular: Die dort ist eine
// MARKTentscheidung ("an wen verkaufen wir"), diese hier ist eine steuerliche TATSACHE.
// Sie enthält deshalb kein CH, NO, LI oder GB — die fallen korrekt ins Drittland.
var euCountries = map[string]bool{
	"AT": true, "BE": true, "BG": true, "CY": true, "CZ": true, "DE": true,
	"DK": true, "EE": true, "ES": true, "FI": true, "FR": true, "GR": true,
	"HR": true, "HU": true, "IE": true, "IT": true, "LT": true, "LU": true,
	"LV": true, "MT": true, "NL": true, "PL": true, "PT": true, "RO": true,
	"SE": true, "SI": true, "SK": true,
}

// IsEUCountry sagt, ob ein Ländercode zur EU gehört.
//
// Exportiert, weil das Billing-Panel dieselbe Frage stellt — und sie MUSS dieselbe
// Antwort bekommen. Eine zweite Länderliste im Panel wäre der Weg, auf dem die
// Kontrollansicht einen Fall für unauffällig hält, den die Rechnungsstellung anders
// einordnet: Der Zweck der Ansicht ist gerade, solche Abweichungen zu zeigen.
func IsEUCountry(countryCode string) bool {
	return euCountries[strings.ToUpper(strings.TrimSpace(countryCode))]
}

// taxTreatmentFor ordnet einen Verkauf steuerlich ein.
//
// Reihenfolge der Prüfungen ist bedeutsam: Der Kleinunternehmer-Fall kommt ZUERST und
// bedingungslos. Solange § 19 gilt, gibt es keine Fallunterscheidung nach Land — und das
// Ergebnis ist byte-identisch zu dem, was der Code vor S130 gesendet hat (leerer Note,
// damit Lexware weiterhin seinen § 19-Textbaustein setzt). Das ist die Zusicherung,
// dass der Schalter in seiner heutigen Stellung nichts verändert.
func taxTreatmentFor(in TaxContext) (TaxTreatment, error) {
	if in.SmallBusiness {
		return TaxTreatment{Type: taxTypeSmallBusiness, RatePct: 0, Note: ""}, nil
	}

	country := strings.ToUpper(strings.TrimSpace(in.CountryCode))
	if country == "" {
		return TaxTreatment{}, fmt.Errorf("lexware: steuerliche Einordnung ohne Land nicht möglich")
	}

	if country == "DE" {
		return TaxTreatment{Type: taxTypeDomestic, RatePct: vatRateDE, Note: ""}, nil
	}

	if euCountries[country] {
		if !in.VATIDVerified {
			return TaxTreatment{}, ErrVATIDRequired
		}
		return TaxTreatment{Type: taxTypeEUReverseCharge, RatePct: 0, Note: noteEUReverseCharge}, nil
	}

	return TaxTreatment{Type: taxTypeThirdCountry, RatePct: 0, Note: noteThirdCountry}, nil
}

// TaxOn rechnet den Steuerbetrag zu einem Nettobetrag aus — in ganzen Cent.
//
// Warum aus Cent und nicht aus Euro: Der Nettobetrag steht in der Datenbank als int64
// Cent, und die Rechnung, die der Kunde bekommt, muss cent-genau dazu passen. Ein
// Zwischenschritt über float64 Euro kann einen halben Cent erzeugen, der sich erst in
// der Buchhaltung als Differenz zeigt.
//
// Gerundet wird kaufmaennisch auf den naechsten Cent. Bei 19 % auf 299,00 € sind das
// 56,81 € (29900 * 19 / 100 = 568100 / 100 = 5681, exakt). Krumme Faelle entstehen erst
// mit Rabatt — deshalb wird hier gerundet und nicht abgeschnitten.
func (t TaxTreatment) TaxOn(netCents int64) int64 {
	if t.RatePct == 0 {
		return 0
	}
	// +0.5 vor dem Abschneiden = kaufmaennisches Runden fuer positive Betraege.
	return int64(float64(netCents)*t.RatePct/100 + 0.5)
}

// InvoiceAmounts ist, was tatsaechlich auf der Rechnung stand.
//
// Gross wird als Net + Tax GEBILDET, nie unabhaengig gerechnet. Damit kann die Identitaet,
// die der CHECK in Migration 244 erzwingt, gar nicht verletzt werden — es gibt keinen
// Pfad, auf dem zwei getrennte Rechnungen auseinanderlaufen.
type InvoiceAmounts struct {
	NetCents   int64
	TaxCents   int64
	GrossCents int64
	TaxType    string
	TaxRatePct float64
}

func amountsFor(netCents int64, t TaxTreatment) InvoiceAmounts {
	tax := t.TaxOn(netCents)
	return InvoiceAmounts{
		NetCents:   netCents,
		TaxCents:   tax,
		GrossCents: netCents + tax,
		TaxType:    t.Type,
		TaxRatePct: t.RatePct,
	}
}
