// Copyright (c) 2026 NorvikOps. All rights reserved.
// SPDX-License-Identifier: Elastic-2.0
// See LICENSE file in the project root for terms.

package admin

import (
	"fmt"
	"net/http"
	"sort"
	"time"

	"github.com/labstack/echo/v4"

	"github.com/matharnica/vakt/internal/billing/lexware"
)

// Steuerübersicht — eine KONTROLLansicht, keine Meldung.
//
// Sie erzeugt ausdrücklich keine Umsatzsteuer-Voranmeldung und keine Zusammenfassende
// Meldung. Das macht Lexware Office (Tarif L/XL) und übermittelt beides direkt ans
// Finanzamt bzw. ans BZSt, ohne eigenes ELSTER-Zertifikat. Eine eigene Meldungserzeugung
// wäre eine zweite Quelle der Wahrheit neben dem Buchhaltungssystem — genau die Drift,
// an der dieses Projekt wiederholt geblutet hat.
//
// Was diese Seite stattdessen tut: zeigen, was Lexware übermitteln WIRD, damit ein Mensch
// es vorher gegenlesen kann. Ein AT-Kunde ohne geprüfte USt-IdNr. oder eine Rechnung mit
// taxType "net" bei 0 % fällt hier auf — nicht erst in einer Korrekturmeldung.
//
// Siehe docs/stories/s130-umsatzsteuer-dach.md (AP5) und ADR-0074.

// maxQuarters begrenzt, wie weit zurück die Seite reicht.
//
// Die Zahl der nicht gezeigten Quartale wird AUSGEWIESEN, nicht verschwiegen. Ein Bericht,
// der still abschneidet, liest sich wie Vollständigkeit — dieselbe Klasse Fehlsignal wie
// ein Gate, das nicht parsebare Eingaben überspringt und trotzdem "OK" meldet.
const maxQuarters = 8

type taxRow struct {
	Company string
	Country string
	VATID   string
	Invoice string
	Period  string
	TaxType string
	Rate    string
	Net     string
	Tax     string
	Gross   string
	Warn    string // leer = unauffällig
	Severe  bool
}

// taxBucket fasst ein Quartal je Steuerart zusammen — die Zahlen, die in der
// Voranmeldung nebeneinander stehen.
type taxBucket struct {
	TaxType string
	Label   string
	Count   int
	Net     string
	Tax     string
	Gross   string
}

type taxQuarter struct {
	Label    string
	Buckets  []taxBucket
	Rows     []taxRow
	Net      string
	Tax      string
	Gross    string
	Warnings int
}

type taxData struct {
	page
	Quarters  []taxQuarter
	Warnings  int
	Hidden    int // ältere Quartale, die nicht gezeigt werden
	SmallBiz  bool
	VATID     string
	NoInvoice bool
}

// taxTypeLabel übersetzt Lexwares taxType in das, was ein Mensch sucht.
func taxTypeLabel(t string) string {
	switch t {
	case "net":
		return "Inland, steuerpflichtig"
	case "vatfree":
		return "steuerfrei (§ 19 UStG)"
	case "externalService13b", "intraCommunitySupply":
		return "EU-Ausland, Reverse Charge"
	case "thirdPartyCountryService", "thirdPartyCountryDelivery":
		return "Drittland, nicht steuerbar"
	default:
		return t
	}
}

// TaxOverview rendert die Kontrollansicht.
func (h *Handler) TaxOverview(c echo.Context) error {
	ctx := c.Request().Context()

	// Der jüngste VIES-Nachweis je Abo. LEFT JOIN, weil ein fehlender Nachweis genau
	// das ist, was auffallen soll — ein INNER JOIN würde die kritischen Zeilen
	// stillschweigend aussortieren.
	rows, err := h.db.Query(ctx, `
		SELECT s.company_name, s.country_code, COALESCE(s.vat_id, ''),
		       i.lexware_invoice_id, i.period_start, i.period_end,
		       i.net_amount_cents, i.tax_amount_cents, i.gross_amount_cents,
		       i.tax_rate_pct, i.tax_type,
		       COALESCE((SELECT v.valid FROM billing_vat_checks v
		                  WHERE v.subscription_id = s.id
		                  ORDER BY v.checked_at DESC LIMIT 1), false)
		  FROM billing_invoices i
		  JOIN billing_quote_requests s ON s.id = i.subscription_id
		 WHERE i.status <> 'voided' AND NOT s.is_free
		 ORDER BY i.period_start DESC`)
	if err != nil {
		return err
	}
	defer rows.Close()

	byQuarter := map[string]*taxQuarter{}
	sums := map[string]map[string][3]int64{} // Quartal -> taxType -> [net, tax, gross]
	counts := map[string]map[string]int{}

	for rows.Next() {
		var r taxRow
		var from, to time.Time
		var net, tax, gross int64
		var rate float64
		var vatValid bool
		if err := rows.Scan(&r.Company, &r.Country, &r.VATID, &r.Invoice, &from, &to,
			&net, &tax, &gross, &rate, &r.TaxType, &vatValid); err != nil {
			return err
		}

		q := fmt.Sprintf("%d Q%d", from.Year(), (int(from.Month())-1)/3+1)
		r.Period = day(from) + " – " + day(to)
		r.Rate = fmt.Sprintf("%.0f %%", rate)
		r.Net, r.Tax, r.Gross = eur(net), eur(tax), eur(gross)
		r.Warn, r.Severe = taxAnomaly(r.Country, r.TaxType, rate, vatValid)

		if byQuarter[q] == nil {
			byQuarter[q] = &taxQuarter{Label: q}
			sums[q] = map[string][3]int64{}
			counts[q] = map[string]int{}
		}
		bq := byQuarter[q]
		bq.Rows = append(bq.Rows, r)
		if r.Warn != "" {
			bq.Warnings++
		}
		s := sums[q][r.TaxType]
		sums[q][r.TaxType] = [3]int64{s[0] + net, s[1] + tax, s[2] + gross}
		counts[q][r.TaxType]++
	}

	d := taxData{page: h.chrome(c, "Steuern", "tax")}

	labels := make([]string, 0, len(byQuarter))
	for q := range byQuarter {
		labels = append(labels, q)
	}
	sort.Sort(sort.Reverse(sort.StringSlice(labels))) // "2026 Q3" sortiert lexikografisch korrekt

	if len(labels) > maxQuarters {
		d.Hidden = len(labels) - maxQuarters
		labels = labels[:maxQuarters]
	}

	for _, q := range labels {
		bq := byQuarter[q]
		var n, t, g int64
		types := make([]string, 0, len(sums[q]))
		for tt := range sums[q] {
			types = append(types, tt)
		}
		sort.Strings(types)
		for _, tt := range types {
			s := sums[q][tt]
			n, t, g = n+s[0], t+s[1], g+s[2]
			bq.Buckets = append(bq.Buckets, taxBucket{
				TaxType: tt, Label: taxTypeLabel(tt), Count: counts[q][tt],
				Net: eur(s[0]), Tax: eur(s[1]), Gross: eur(s[2]),
			})
		}
		bq.Net, bq.Tax, bq.Gross = eur(n), eur(t), eur(g)
		d.Warnings += bq.Warnings
		d.Quarters = append(d.Quarters, *bq)
	}
	d.NoInvoice = len(d.Quarters) == 0

	return c.Render(http.StatusOK, "tax.html", d)
}

// taxAnomaly prüft eine Rechnungszeile gegen das, was für sie gelten müsste.
//
// Die Prüfung läuft NACHTRÄGLICH gegen bereits gestellte Belege — sie kann nichts mehr
// verhindern, aber sie kann verhindern, dass ein falscher Beleg unbemerkt in eine
// Meldung wandert. Deshalb ist sie hier und nicht nur im Rechnungsweg.
func taxAnomaly(country, taxType string, rate float64, vatValid bool) (string, bool) {
	switch {
	// Der teuerste Fall: Regelbesteuerung ausgewiesen, aber kein Satz. Die Steuer wird
	// geschuldet und ist nicht ausgewiesen (§ 14c UStG) — ohne Fehler und ohne Log.
	// Der CHECK in Migration 244 faengt die Betragsseite; das hier faengt die Steuerart.
	case taxType == "net" && rate == 0:
		return "Steuerart „net“, aber 0 % — Umsatzsteuer geschuldet, nicht ausgewiesen (§ 14c UStG)", true

	// Reverse Charge ohne Nachweis: Ohne gueltige USt-IdNr. traegt die Verlagerung
	// nicht, und die nicht berechnete Steuer bleibt bei uns haengen.
	case lexware.IsEUCountry(country) && country != "DE" && rate == 0 && !vatValid:
		return "EU-Ausland ohne gültige USt-IdNr.-Prüfung — Reverse Charge nicht belegt", true

	case country == "DE" && rate == 0 && taxType != "vatfree":
		return "Inlandsrechnung ohne Umsatzsteuer", true

	// Kein Fehler, aber erklaerungsbeduerftig, sobald die Regelbesteuerung laeuft.
	case taxType == "vatfree":
		return "steuerfrei nach § 19 UStG — nur bis zum Wechsel in die Regelbesteuerung korrekt", false
	}
	return "", false
}
