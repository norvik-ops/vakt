package secvitals

import (
	"bytes"
	"fmt"
	"strings"
	"time"

	"github.com/go-pdf/fpdf"
)

// GenerateAuditIndexPDF builds the INDEX.pdf included in the audit-package ZIP.
// It renders a title page followed by a per-control table with evidence counts.
func GenerateAuditIndexPDF(frameworkName, orgName string, controlOrder []string, controlMap map[string]*auditControlEntry, exportedAt time.Time) ([]byte, error) {
	pdf := fpdf.New("P", "mm", "A4", "")
	pdf.SetMargins(15, 15, 15)
	pdf.SetAutoPageBreak(true, 15)

	// Footer
	pdf.SetFooterFunc(func() {
		pdf.SetY(-12)
		pdf.SetFont("Helvetica", "I", 7)
		pdf.SetTextColor(150, 150, 160)
		pdf.CellFormat(0, 5,
			fmt.Sprintf("Erstellt mit Vakt — %s — Seite %d/{nb}", exportedAt.Format("02.01.2006"), pdf.PageNo()),
			"", 0, "C", false, 0, "")
	})
	pdf.AliasNbPages("{nb}")

	// ── Cover page ────────────────────────────────────────────────────────────
	pdf.AddPage()

	pdf.SetFillColor(37, 99, 235)
	pdf.Rect(0, 0, 210, 28, "F")
	pdf.SetTextColor(255, 255, 255)
	pdf.SetFont("Helvetica", "B", 14)
	pdf.SetXY(15, 8)
	pdf.CellFormat(180, 8, "Vakt — Audit-Paket", "", 1, "L", false, 0, "")
	pdf.SetFont("Helvetica", "", 9)
	pdf.SetXY(15, 17)
	pdf.CellFormat(180, 6, fmt.Sprintf("%s  |  %s", orgName, exportedAt.Format("02.01.2006")), "", 1, "L", false, 0, "")

	pdf.SetTextColor(30, 30, 40)
	pdf.SetXY(15, 38)
	pdf.SetFont("Helvetica", "B", 18)
	pdf.CellFormat(180, 12, frameworkName, "", 1, "L", false, 0, "")

	pdf.SetFont("Helvetica", "", 10)
	pdf.SetTextColor(100, 100, 120)
	pdf.CellFormat(180, 7, "Vollständige Compliance-Nachweise — Audit-Paket", "", 1, "L", false, 0, "")

	// Summary stats
	totalControls := len(controlOrder)
	withEvidence := 0
	totalEvidence := 0
	for _, ce := range controlMap {
		if len(ce.Evidence) > 0 {
			withEvidence++
		}
		totalEvidence += len(ce.Evidence)
	}

	pdf.SetY(pdf.GetY() + 10)
	pdf.SetFont("Helvetica", "B", 11)
	pdf.SetTextColor(30, 30, 40)
	pdf.CellFormat(180, 8, "Zusammenfassung", "", 1, "L", false, 0, "")

	renderStat := func(label, value string) {
		pdf.SetFont("Helvetica", "B", 9)
		pdf.SetTextColor(100, 100, 120)
		pdf.CellFormat(80, 6, label, "0", 0, "L", false, 0, "")
		pdf.SetFont("Helvetica", "", 9)
		pdf.SetTextColor(30, 30, 40)
		pdf.CellFormat(100, 6, value, "0", 1, "L", false, 0, "")
	}
	renderStat("Framework:", frameworkName)
	renderStat("Organisation:", orgName)
	renderStat("Export-Datum:", exportedAt.Format("02.01.2006 15:04 UTC"))
	renderStat("Controls gesamt:", fmt.Sprintf("%d", totalControls))
	renderStat("Controls mit Nachweis:", fmt.Sprintf("%d", withEvidence))
	renderStat("Controls ohne Nachweis:", fmt.Sprintf("%d", totalControls-withEvidence))
	renderStat("Nachweise gesamt:", fmt.Sprintf("%d", totalEvidence))

	// ── Control table page(s) ─────────────────────────────────────────────────
	pdf.AddPage()

	pdf.SetTextColor(30, 30, 40)
	pdf.SetFont("Helvetica", "B", 13)
	pdf.SetXY(15, 15)
	pdf.CellFormat(180, 9, "Control-Übersicht", "", 1, "L", false, 0, "")

	// Table header
	renderTableHeader := func() {
		pdf.SetFillColor(37, 99, 235)
		pdf.SetTextColor(255, 255, 255)
		pdf.SetFont("Helvetica", "B", 8)
		pdf.CellFormat(30, 6, "Control-ID", "0", 0, "L", true, 0, "")
		pdf.CellFormat(110, 6, "Bezeichnung", "0", 0, "L", true, 0, "")
		pdf.CellFormat(20, 6, "Nachweise", "0", 0, "C", true, 0, "")
		pdf.CellFormat(20, 6, "Status", "0", 1, "C", true, 0, "")
	}
	renderTableHeader()

	for i, ctrlID := range controlOrder {
		if pdf.GetY() > 270 {
			pdf.AddPage()
			renderTableHeader()
		}
		ce := controlMap[ctrlID]
		evCount := len(ce.Evidence)

		if i%2 == 0 {
			pdf.SetFillColor(245, 247, 255)
		} else {
			pdf.SetFillColor(255, 255, 255)
		}
		pdf.SetTextColor(30, 30, 40)
		pdf.SetFont("Helvetica", "", 8)

		cid := ce.ControlID
		if len(cid) > 18 {
			cid = cid[:15] + "..."
		}
		title := ce.ControlTitle
		if len(title) > 66 {
			title = title[:63] + "..."
		}

		pdf.CellFormat(30, 6, cid, "0", 0, "L", true, 0, "")
		pdf.CellFormat(110, 6, title, "0", 0, "L", true, 0, "")

		if evCount > 0 {
			pdf.SetTextColor(34, 197, 94)
		} else {
			pdf.SetTextColor(220, 38, 38)
		}
		pdf.SetFont("Helvetica", "B", 8)
		pdf.CellFormat(20, 6, fmt.Sprintf("%d", evCount), "0", 0, "C", true, 0, "")

		statusLabel := "Offen"
		if evCount > 0 {
			statusLabel = "Belegt"
		}
		pdf.CellFormat(20, 6, statusLabel, "0", 1, "C", true, 0, "")
		pdf.SetTextColor(30, 30, 40)
	}

	var buf bytes.Buffer
	if err := pdf.Output(&buf); err != nil {
		return nil, fmt.Errorf("audit index pdf output: %w", err)
	}
	return buf.Bytes(), nil
}

// GenerateIncidentReportPDF renders a BaFin-style DORA incident report as PDF bytes.
// For major incidents (is_major_incident = true) it adds the Art. 18 DORA label prominently.
func GenerateIncidentReportPDF(incident *Incident, orgName string) ([]byte, error) {
	pdf := fpdf.New("P", "mm", "A4", "")
	pdf.SetMargins(15, 15, 15)
	pdf.SetAutoPageBreak(true, 15)
	pdf.AddPage()

	now := time.Now()

	// ── Header bar ────────────────────────────────────────────────────────────
	headerColor := [3]int{37, 99, 235}
	if incident.IsMajorIncident {
		// Red header for major incidents
		headerColor = [3]int{185, 28, 28}
	}
	pdf.SetFillColor(headerColor[0], headerColor[1], headerColor[2])
	pdf.Rect(0, 0, 210, 28, "F")
	pdf.SetTextColor(255, 255, 255)
	pdf.SetFont("Helvetica", "B", 14)
	pdf.SetXY(15, 8)
	pdf.CellFormat(180, 8, "Vakt — IKT-Vorfallbericht", "", 1, "L", false, 0, "")
	pdf.SetFont("Helvetica", "", 9)
	pdf.SetXY(15, 17)
	pdf.CellFormat(180, 6, orgName, "", 1, "L", false, 0, "")

	// ── Major incident banner ─────────────────────────────────────────────────
	if incident.IsMajorIncident {
		pdf.SetFillColor(185, 28, 28)
		pdf.SetY(28)
		pdf.SetX(0)
		pdf.SetFillColor(254, 226, 226)
		pdf.Rect(0, 28, 210, 12, "F")
		pdf.SetTextColor(185, 28, 28)
		pdf.SetFont("Helvetica", "B", 10)
		pdf.SetXY(15, 31)
		pdf.CellFormat(180, 6, "SCHWERWIEGENDER IKT-VORFALL gem. Art. 18 DORA", "", 1, "C", false, 0, "")
	}

	topY := 38.0
	if incident.IsMajorIncident {
		topY = 46.0
	}

	// ── Title ─────────────────────────────────────────────────────────────────
	pdf.SetTextColor(30, 30, 40)
	pdf.SetXY(15, topY)
	pdf.SetFont("Helvetica", "B", 16)
	title := incident.Title
	if len(title) > 80 {
		title = title[:77] + "..."
	}
	pdf.MultiCell(180, 9, title, "", "L", false)

	pdf.SetFont("Helvetica", "", 9)
	pdf.SetTextColor(100, 100, 120)
	pdf.CellFormat(180, 6, fmt.Sprintf("Erstellt am %s", now.Format("02.01.2006 15:04")), "", 1, "L", false, 0, "")

	// ── Severity and metadata ─────────────────────────────────────────────────
	pdf.SetY(pdf.GetY() + 6)
	pdf.SetTextColor(30, 30, 40)
	pdf.SetFont("Helvetica", "B", 11)
	pdf.CellFormat(180, 8, "Vorfalldetails", "", 1, "L", false, 0, "")

	severityColor := map[string][3]int{
		"low":      {34, 197, 94},
		"medium":   {234, 179, 8},
		"high":     {249, 115, 22},
		"critical": {220, 38, 38},
	}
	sevCol := [3]int{100, 100, 120}
	if c, ok := severityColor[incident.Severity]; ok {
		sevCol = c
	}

	renderRow := func(label, value string) {
		if pdf.GetY() > 265 {
			pdf.AddPage()
		}
		pdf.SetFont("Helvetica", "B", 9)
		pdf.SetTextColor(100, 100, 120)
		pdf.CellFormat(55, 6, label, "0", 0, "L", false, 0, "")
		pdf.SetFont("Helvetica", "", 9)
		pdf.SetTextColor(30, 30, 40)
		pdf.MultiCell(125, 6, value, "0", "L", false)
	}

	renderRow("Entdeckt am:", incident.DiscoveredAt.Format("02.01.2006 15:04 UTC"))
	pdf.SetFont("Helvetica", "B", 9)
	pdf.SetTextColor(100, 100, 120)
	pdf.CellFormat(55, 6, "Schweregrad:", "0", 0, "L", false, 0, "")
	pdf.SetFont("Helvetica", "B", 9)
	pdf.SetTextColor(sevCol[0], sevCol[1], sevCol[2])
	pdf.CellFormat(125, 6, strings.ToUpper(incident.Severity), "0", 1, "L", false, 0, "")
	pdf.SetTextColor(30, 30, 40)

	renderRow("Vorfalltyp:", strings.ToUpper(incident.IncidentType))
	if incident.NotificationAuthority != "" {
		renderRow("Meldebehörde:", incident.NotificationAuthority)
	}

	// ── DORA-specific fields ──────────────────────────────────────────────────
	if incident.IncidentType == "dora" {
		pdf.SetY(pdf.GetY() + 4)
		pdf.SetFont("Helvetica", "B", 11)
		pdf.SetTextColor(30, 30, 40)
		pdf.CellFormat(180, 8, "DORA-spezifische Angaben", "", 1, "L", false, 0, "")

		if incident.AffectedCustomers != nil {
			renderRow("Betroffene Kunden:", fmt.Sprintf("%d", *incident.AffectedCustomers))
		} else {
			renderRow("Betroffene Kunden:", "k.A.")
		}
		if incident.FinancialImpactEstimate != nil && *incident.FinancialImpactEstimate != "" {
			renderRow("Geschätzter finanzieller Schaden:", *incident.FinancialImpactEstimate)
		} else {
			renderRow("Geschätzter finanzieller Schaden:", "k.A.")
		}
		majorLabel := "Nein"
		if incident.IsMajorIncident {
			majorLabel = "JA — Schwerwiegender IKT-Vorfall (Art. 18 DORA)"
		}
		renderRow("Schwerwiegender IKT-Vorfall:", majorLabel)
	}

	// ── Deadline status ───────────────────────────────────────────────────────
	ds := computeDeadlineStatus(incident)
	if ds != nil {
		pdf.SetY(pdf.GetY() + 4)
		pdf.SetFont("Helvetica", "B", 11)
		pdf.SetTextColor(30, 30, 40)
		pdf.CellFormat(180, 8, "Meldefristen", "", 1, "L", false, 0, "")

		// Table header
		pdf.SetFillColor(37, 99, 235)
		pdf.SetTextColor(255, 255, 255)
		pdf.SetFont("Helvetica", "B", 8)
		pdf.CellFormat(30, 6, "Frist", "0", 0, "L", true, 0, "")
		pdf.CellFormat(60, 6, "Ablauf", "0", 0, "L", true, 0, "")
		pdf.CellFormat(50, 6, "Gemeldet am", "0", 0, "L", true, 0, "")
		pdf.CellFormat(40, 6, "Status", "0", 1, "L", true, 0, "")

		type dlRow struct {
			label string
			info  *DeadlineInfo
		}
		rows := []dlRow{}
		if ds.Has4h && ds.D4h != nil {
			rows = append(rows, dlRow{"4h (Erstmeldung)", ds.D4h})
		}
		if ds.Has24h && ds.D24h != nil {
			rows = append(rows, dlRow{"24h (Frühmeldung)", ds.D24h})
		}
		if ds.Has72h && ds.D72h != nil {
			rows = append(rows, dlRow{"72h (Vollständig)", ds.D72h})
		}
		if ds.Has30d && ds.D30d != nil {
			rows = append(rows, dlRow{"30 Tage (Abschluss)", ds.D30d})
		}

		statusLabel := map[string]string{
			"green":  "Offen (fristgerecht)",
			"yellow": "Offen (bald fällig)",
			"red":    "ÜBERFÄLLIG",
			"done":   "Gemeldet",
		}
		statusColor := map[string][3]int{
			"green":  {34, 197, 94},
			"yellow": {234, 179, 8},
			"red":    {220, 38, 38},
			"done":   {34, 197, 94},
		}

		for i, r := range rows {
			if pdf.GetY() > 265 {
				pdf.AddPage()
			}
			if i%2 == 0 {
				pdf.SetFillColor(245, 247, 255)
			} else {
				pdf.SetFillColor(255, 255, 255)
			}
			pdf.SetTextColor(30, 30, 40)
			pdf.SetFont("Helvetica", "", 8)
			dlStr := ""
			if r.info.Deadline != nil {
				dlStr = r.info.Deadline.Format("02.01.2006 15:04")
			}
			reportedStr := "—"
			if r.info.ReportedAt != nil {
				reportedStr = r.info.ReportedAt.Format("02.01.2006 15:04")
			}
			label := statusLabel[r.info.Status]
			if label == "" {
				label = r.info.Status
			}
			col := statusColor[r.info.Status]
			if col == ([3]int{}) {
				col = [3]int{100, 100, 120}
			}
			pdf.CellFormat(30, 6, r.label, "0", 0, "L", true, 0, "")
			pdf.CellFormat(60, 6, dlStr, "0", 0, "L", true, 0, "")
			pdf.CellFormat(50, 6, reportedStr, "0", 0, "L", true, 0, "")
			pdf.SetTextColor(col[0], col[1], col[2])
			pdf.SetFont("Helvetica", "B", 8)
			pdf.CellFormat(40, 6, label, "0", 1, "L", true, 0, "")
			pdf.SetTextColor(30, 30, 40)
		}
	}

	// ── Footer ────────────────────────────────────────────────────────────────
	pdf.SetFooterFunc(func() {
		pdf.SetY(-12)
		pdf.SetFont("Helvetica", "I", 7)
		pdf.SetTextColor(150, 150, 160)
		pdf.CellFormat(0, 5,
			fmt.Sprintf("Vakt Comply — %s — Seite %d/{nb}", orgName, pdf.PageNo()),
			"", 0, "C", false, 0, "")
	})
	pdf.AliasNbPages("{nb}")

	var buf bytes.Buffer
	if err := pdf.Output(&buf); err != nil {
		return nil, fmt.Errorf("pdf output: %w", err)
	}
	return buf.Bytes(), nil
}

// GenerateDORAPDF renders a DORA readiness report as PDF bytes.
func GenerateDORAPDF(dashboard *DORADashboard, orgName string) ([]byte, error) {
	pdf := fpdf.New("P", "mm", "A4", "")
	pdf.SetMargins(15, 15, 15)
	pdf.SetAutoPageBreak(true, 15)
	pdf.AddPage()

	now := time.Now()

	// ── Header bar ────────────────────────────────────────────────────────────
	pdf.SetFillColor(37, 99, 235)
	pdf.Rect(0, 0, 210, 28, "F")
	pdf.SetTextColor(255, 255, 255)
	pdf.SetFont("Helvetica", "B", 14)
	pdf.SetXY(15, 8)
	pdf.CellFormat(180, 8, "Vakt — DORA Bereitschaftsbericht", "", 1, "L", false, 0, "")
	pdf.SetFont("Helvetica", "", 9)
	pdf.SetXY(15, 17)
	pdf.CellFormat(180, 6, fmt.Sprintf("%s  |  %s", orgName, now.Format("02.01.2006")), "", 1, "L", false, 0, "")

	// ── Section 1: Executive Summary ─────────────────────────────────────────
	pdf.SetTextColor(30, 30, 40)
	pdf.SetXY(15, 35)
	pdf.SetFont("Helvetica", "B", 13)
	pdf.CellFormat(180, 8, "1. Executive Summary", "", 1, "L", false, 0, "")

	score := dashboard.ReadinessPct
	scoreColor := [3]int{220, 38, 38}
	if score >= 80 {
		scoreColor = [3]int{34, 197, 94}
	} else if score >= 50 {
		scoreColor = [3]int{234, 179, 8}
	}
	cy := pdf.GetY() + 2
	pdf.SetFillColor(scoreColor[0], scoreColor[1], scoreColor[2])
	pdf.RoundedRect(15, cy, 42, 22, 3, "1234", "F")
	pdf.SetTextColor(255, 255, 255)
	pdf.SetFont("Helvetica", "B", 18)
	pdf.SetXY(15, cy+3)
	pdf.CellFormat(42, 12, fmt.Sprintf("%.0f%%", score), "", 1, "C", false, 0, "")

	infoX := 62.0
	pdf.SetTextColor(30, 30, 40)
	pdf.SetFont("Helvetica", "", 10)
	pdf.SetXY(infoX, cy+3)
	pdf.CellFormat(130, 8, "DORA-Bereitschaftsgrad", "0", 1, "L", false, 0, "")
	pdf.SetXY(infoX, pdf.GetY())
	if score >= 80 {
		pdf.SetTextColor(34, 197, 94)
		pdf.CellFormat(130, 6, "Gut — Anforderungen weitgehend erfüllt", "0", 1, "L", false, 0, "")
	} else if score >= 50 {
		pdf.SetTextColor(234, 179, 8)
		pdf.CellFormat(130, 6, "Mittel — Handlungsbedarf vorhanden", "0", 1, "L", false, 0, "")
	} else {
		pdf.SetTextColor(220, 38, 38)
		pdf.CellFormat(130, 6, "Kritisch — erheblicher Handlungsbedarf", "0", 1, "L", false, 0, "")
	}

	// ── Section 2: Offene Critical Controls ──────────────────────────────────
	pdf.SetY(cy + 30)
	pdf.SetTextColor(30, 30, 40)
	pdf.SetFont("Helvetica", "B", 13)
	pdf.CellFormat(180, 8, "2. Offene Critical Controls", "", 1, "L", false, 0, "")
	pdf.SetFont("Helvetica", "", 10)
	if dashboard.OpenCriticalControls == 0 {
		pdf.SetTextColor(34, 197, 94)
		pdf.CellFormat(180, 7, "Keine offenen Critical Controls — alle kritischen Anforderungen erfüllt.", "0", 1, "L", false, 0, "")
	} else {
		pdf.SetTextColor(220, 38, 38)
		pdf.CellFormat(180, 7, fmt.Sprintf("%d offene Critical Controls (Gewichtung ≥ 3, nicht abgedeckt)", dashboard.OpenCriticalControls), "0", 1, "L", false, 0, "")
	}

	// ── Section 3: Nächste Meldepflicht ──────────────────────────────────────
	pdf.SetY(pdf.GetY() + 4)
	pdf.SetTextColor(30, 30, 40)
	pdf.SetFont("Helvetica", "B", 13)
	pdf.CellFormat(180, 8, "3. Nächste Meldepflicht", "", 1, "L", false, 0, "")
	pdf.SetFont("Helvetica", "", 10)
	if dashboard.NextDeadline == nil {
		pdf.SetTextColor(34, 197, 94)
		pdf.CellFormat(180, 7, "Keine offenen Fristen", "0", 1, "L", false, 0, "")
	} else {
		nd := dashboard.NextDeadline
		pdf.SetTextColor(30, 30, 40)
		pdf.CellFormat(180, 7, fmt.Sprintf("Vorfall: %s", nd.Title), "0", 1, "L", false, 0, "")
		pdf.CellFormat(180, 7, fmt.Sprintf("Fristtyp: %s  |  Fällig am: %s", nd.DeadlineType, nd.DeadlineAt.Format("02.01.2006 15:04 UTC")), "0", 1, "L", false, 0, "")
	}

	// ── Section 4: IKT-Drittanbieter (Art. 28-44) ────────────────────────────
	pdf.SetY(pdf.GetY() + 4)
	pdf.SetTextColor(30, 30, 40)
	pdf.SetFont("Helvetica", "B", 13)
	pdf.CellFormat(180, 8, "4. IKT-Drittanbieter-Register (Art. 28-44)", "", 1, "L", false, 0, "")
	pdf.SetFont("Helvetica", "", 10)
	if dashboard.ThirdPartyCount == 0 {
		pdf.SetTextColor(220, 38, 38)
		pdf.CellFormat(180, 7, "Kein IKT-Drittanbieter eingetragen — Register muss gemäß Art. 28 DORA geführt werden.", "0", 1, "L", false, 0, "")
	} else {
		pdf.SetTextColor(30, 30, 40)
		pdf.CellFormat(180, 7, fmt.Sprintf("Eingetragene Drittanbieter: %d  |  davon kritisch: %d", dashboard.ThirdPartyCount, dashboard.CriticalThirdParties), "0", 1, "L", false, 0, "")
		if dashboard.MissingExitStrategies > 0 {
			pdf.SetTextColor(220, 38, 38)
			pdf.CellFormat(180, 7, fmt.Sprintf("WARNUNG: %d kritische Drittanbieter ohne Ausstiegsstrategie (Art. 43 DORA).", dashboard.MissingExitStrategies), "0", 1, "L", false, 0, "")
		} else {
			pdf.SetTextColor(34, 197, 94)
			pdf.CellFormat(180, 7, "Alle kritischen Drittanbieter haben eine dokumentierte Ausstiegsstrategie.", "0", 1, "L", false, 0, "")
		}
	}
	if dashboard.ExpiredSuppliers > 0 {
		pdf.SetTextColor(234, 179, 8)
		pdf.CellFormat(180, 7, fmt.Sprintf("%d Lieferanten mit abgelaufenem Vertrag — Überprüfung erforderlich.", dashboard.ExpiredSuppliers), "0", 1, "L", false, 0, "")
	}

	// ── Section 5: Resilienztests (Art. 24-27) ───────────────────────────────
	pdf.SetY(pdf.GetY() + 4)
	pdf.SetTextColor(30, 30, 40)
	pdf.SetFont("Helvetica", "B", 13)
	pdf.CellFormat(180, 8, "5. Resilienztests (Art. 24-27)", "", 1, "L", false, 0, "")
	pdf.SetFont("Helvetica", "", 10)
	if dashboard.TLPTOverdueWarning {
		pdf.SetTextColor(220, 38, 38)
		pdf.CellFormat(180, 7, "WARNUNG: Kein TLPT-Test in den letzten 3 Jahren — DORA Art. 26 nicht erfüllt.", "0", 1, "L", false, 0, "")
	} else {
		pdf.SetTextColor(34, 197, 94)
		pdf.CellFormat(180, 7, "TLPT-Tests aktuell — DORA Art. 26 erfüllt.", "0", 1, "L", false, 0, "")
	}
	if len(dashboard.RecentResilienceTests) > 0 {
		pdf.SetTextColor(30, 30, 40)
		pdf.SetFont("Helvetica", "B", 9)
		pdf.CellFormat(180, 6, "Letzte Tests:", "0", 1, "L", false, 0, "")
		pdf.SetFont("Helvetica", "", 9)
		for _, rt := range dashboard.RecentResilienceTests {
			provider := rt.Provider
			if provider == "" {
				provider = "–"
			}
			pdf.CellFormat(180, 5,
				fmt.Sprintf("  • %s | %s | %s | Abhilfe: %s",
					rt.Type, rt.TestDate.Format("02.01.2006"), provider, rt.RemediationStatus),
				"0", 1, "L", false, 0, "")
		}
	}

	// ── Footer ────────────────────────────────────────────────────────────────
	pdf.SetFooterFunc(func() {
		pdf.SetY(-12)
		pdf.SetFont("Helvetica", "I", 7)
		pdf.SetTextColor(150, 150, 160)
		pdf.CellFormat(0, 5,
			"Erstellt mit Vakt — Digital Operational Resilience Act (EU 2022/2554)",
			"", 0, "C", false, 0, "")
	})
	pdf.AliasNbPages("{nb}")

	var buf bytes.Buffer
	if err := pdf.Output(&buf); err != nil {
		return nil, fmt.Errorf("pdf output: %w", err)
	}
	return buf.Bytes(), nil
}

// GenerateTISAXReportPDF renders a TISAX® Bereitschaftsbericht as PDF bytes.
// protectionLevel: "normal" | "high" | "very_high"
// assessmentLevel: "AL1" | "AL2" | "AL3"
func GenerateTISAXReportPDF(report *ReadinessReport, controls []Control, gaps *TISAXGapAnalysis, orgName, protectionLevel, assessmentLevel string, assessmentDate time.Time) ([]byte, error) {
	pdf := fpdf.New("P", "mm", "A4", "")
	pdf.SetMargins(15, 15, 15)
	pdf.SetAutoPageBreak(true, 15)

	// ── Footer setup ──────────────────────────────────────────────────────────
	pdf.SetFooterFunc(func() {
		pdf.SetY(-12)
		pdf.SetFont("Helvetica", "I", 7)
		pdf.SetTextColor(150, 150, 160)
		pdf.CellFormat(0, 5,
			fmt.Sprintf("Vakt Comply — TISAX® Bereitschaftsbericht — %s — Seite %d/{nb}", orgName, pdf.PageNo()),
			"", 0, "C", false, 0, "")
	})
	pdf.AliasNbPages("{nb}")

	// ────────────────────────────────────────────────────────────────────────
	// Page 1: Cover
	// ────────────────────────────────────────────────────────────────────────
	pdf.AddPage()

	// Header bar
	pdf.SetFillColor(37, 99, 235)
	pdf.Rect(0, 0, 210, 28, "F")
	pdf.SetTextColor(255, 255, 255)
	pdf.SetFont("Helvetica", "B", 14)
	pdf.SetXY(15, 8)
	pdf.CellFormat(180, 8, "Vakt — TISAX® Bereitschaftsbericht", "", 1, "L", false, 0, "")
	pdf.SetFont("Helvetica", "", 9)
	pdf.SetXY(15, 17)
	pdf.CellFormat(180, 6, fmt.Sprintf("%s  |  %s", orgName, assessmentDate.Format("02.01.2006")), "", 1, "L", false, 0, "")

	// Title
	pdf.SetTextColor(30, 30, 40)
	pdf.SetXY(15, 38)
	pdf.SetFont("Helvetica", "B", 20)
	pdf.CellFormat(180, 12, "TISAX® Bereitschaftsbericht", "", 1, "L", false, 0, "")

	// Protection level badge
	pdf.SetFont("Helvetica", "", 10)
	pdf.SetTextColor(100, 100, 120)
	plLabel := map[string]string{
		"normal":    "Normal",
		"high":      "Hoch",
		"very_high": "Sehr hoch",
	}
	plText := plLabel[protectionLevel]
	if plText == "" {
		plText = protectionLevel
	}
	pdf.SetXY(15, 54)
	pdf.CellFormat(60, 7, "Schutzbedarfsstufe:", "0", 0, "L", false, 0, "")
	pdf.SetTextColor(37, 99, 235)
	pdf.SetFont("Helvetica", "B", 10)
	pdf.CellFormat(120, 7, plText, "0", 1, "L", false, 0, "")

	// Assessment level
	pdf.SetFont("Helvetica", "", 10)
	pdf.SetTextColor(100, 100, 120)
	pdf.SetXY(15, 63)
	pdf.CellFormat(60, 7, "Assessment-Level:", "0", 0, "L", false, 0, "")
	alLabel := map[string]string{
		"AL1": "AL1 — Standort",
		"AL2": "AL2 — Standard",
		"AL3": "AL3 — Vollassessment",
	}
	alText := alLabel[assessmentLevel]
	if alText == "" {
		alText = assessmentLevel
	}
	pdf.SetTextColor(37, 99, 235)
	pdf.SetFont("Helvetica", "B", 10)
	pdf.CellFormat(120, 7, alText, "0", 1, "L", false, 0, "")

	// Date
	pdf.SetFont("Helvetica", "", 10)
	pdf.SetTextColor(100, 100, 120)
	pdf.SetXY(15, 72)
	pdf.CellFormat(60, 7, "Berichtsdatum:", "0", 0, "L", false, 0, "")
	pdf.SetTextColor(30, 30, 40)
	pdf.CellFormat(120, 7, assessmentDate.Format("02. January 2006"), "0", 1, "L", false, 0, "")

	// Overall readiness score badge
	pdf.SetY(86)
	var readinessPct float64
	if report != nil && report.TISAXMaturity != nil {
		readinessPct = report.TISAXMaturity.ReadinessPercent
	}
	scoreColor := [3]int{220, 38, 38}
	if readinessPct >= 80 {
		scoreColor = [3]int{34, 197, 94}
	} else if readinessPct >= 50 {
		scoreColor = [3]int{234, 179, 8}
	}
	cy := pdf.GetY()
	pdf.SetFillColor(scoreColor[0], scoreColor[1], scoreColor[2])
	pdf.RoundedRect(15, cy, 50, 26, 3, "1234", "F")
	pdf.SetTextColor(255, 255, 255)
	pdf.SetFont("Helvetica", "B", 20)
	pdf.SetXY(15, cy+4)
	pdf.CellFormat(50, 14, fmt.Sprintf("%.0f%%", readinessPct), "", 1, "C", false, 0, "")

	pdf.SetXY(72, cy+4)
	pdf.SetTextColor(30, 30, 40)
	pdf.SetFont("Helvetica", "B", 12)
	pdf.CellFormat(120, 8, "Gesamtbereitschaft", "0", 1, "L", false, 0, "")
	pdf.SetXY(72, cy+14)
	pdf.SetFont("Helvetica", "", 9)
	pdf.SetTextColor(100, 100, 120)
	if readinessPct >= 80 {
		pdf.CellFormat(120, 7, "Gut — Anforderungen weitgehend erfüllt", "0", 1, "L", false, 0, "")
	} else if readinessPct >= 50 {
		pdf.CellFormat(120, 7, "Mittel — Handlungsbedarf vorhanden", "0", 1, "L", false, 0, "")
	} else {
		pdf.CellFormat(120, 7, "Kritisch — erheblicher Handlungsbedarf", "0", 1, "L", false, 0, "")
	}

	// Controls summary
	if report != nil {
		pdf.SetY(cy + 36)
		pdf.SetTextColor(30, 30, 40)
		pdf.SetFont("Helvetica", "", 9)
		pdf.CellFormat(180, 6, fmt.Sprintf("Gesamt: %d Controls  |  Abgedeckt: %d  |  Teilweise: %d  |  Fehlend: %d",
			report.TotalControls, report.Covered, report.Partial, report.Missing), "0", 1, "L", false, 0, "")
	}

	// ────────────────────────────────────────────────────────────────────────
	// Page 2: Chapter overview table
	// ────────────────────────────────────────────────────────────────────────
	pdf.AddPage()

	pdf.SetTextColor(30, 30, 40)
	pdf.SetFont("Helvetica", "B", 14)
	pdf.SetXY(15, 15)
	pdf.CellFormat(180, 9, "Kapitelübersicht", "", 1, "L", false, 0, "")

	pdf.SetFillColor(37, 99, 235)
	pdf.SetTextColor(255, 255, 255)
	pdf.SetFont("Helvetica", "B", 8)
	pdf.CellFormat(80, 6, "Domäne", "0", 0, "L", true, 0, "")
	pdf.CellFormat(25, 6, "Controls", "0", 0, "C", true, 0, "")
	pdf.CellFormat(30, 6, "Ø Score", "0", 0, "C", true, 0, "")
	pdf.CellFormat(30, 6, "Vollständig", "0", 0, "C", true, 0, "")
	pdf.CellFormat(15, 6, "Status", "0", 1, "C", true, 0, "")

	chapters := []ChapterMaturity{}
	if report != nil && report.TISAXMaturity != nil {
		chapters = report.TISAXMaturity.ByChapter
	}

	if len(chapters) == 0 {
		pdf.SetFillColor(245, 247, 255)
		pdf.SetTextColor(100, 100, 120)
		pdf.SetFont("Helvetica", "I", 8)
		pdf.CellFormat(180, 6, "Keine Einträge", "0", 1, "C", true, 0, "")
	} else {
		trafficLight := map[string]string{"green": "●", "yellow": "●", "red": "●"}
		trafficColor := map[string][3]int{
			"green":  {34, 197, 94},
			"yellow": {234, 179, 8},
			"red":    {220, 38, 38},
		}
		for i, ch := range chapters {
			if pdf.GetY() > 270 {
				pdf.AddPage()
			}
			if i%2 == 0 {
				pdf.SetFillColor(245, 247, 255)
			} else {
				pdf.SetFillColor(255, 255, 255)
			}
			pdf.SetTextColor(30, 30, 40)
			pdf.SetFont("Helvetica", "", 8)
			name := ch.Domain
			if len(name) > 48 {
				name = name[:45] + "..."
			}
			pdf.CellFormat(80, 6, name, "0", 0, "L", true, 0, "")
			pdf.CellFormat(25, 6, fmt.Sprintf("%d", ch.TotalControls), "0", 0, "C", true, 0, "")
			pdf.CellFormat(30, 6, fmt.Sprintf("%.2f / 3", ch.AvgScore), "0", 0, "C", true, 0, "")
			pdf.CellFormat(30, 6, fmt.Sprintf("%d", ch.FullyMature), "0", 0, "C", true, 0, "")
			col := trafficColor[ch.Color]
			if col == ([3]int{}) {
				col = [3]int{100, 100, 120}
			}
			pdf.SetTextColor(col[0], col[1], col[2])
			pdf.SetFont("Helvetica", "B", 10)
			pdf.CellFormat(15, 6, trafficLight[ch.Color], "0", 1, "C", true, 0, "")
		}
	}

	// ────────────────────────────────────────────────────────────────────────
	// Page 3+: Open controls (maturity_score < 3)
	// ────────────────────────────────────────────────────────────────────────
	pdf.AddPage()

	pdf.SetTextColor(30, 30, 40)
	pdf.SetFont("Helvetica", "B", 14)
	pdf.SetXY(15, 15)

	openControls := []Control{}
	for _, c := range controls {
		if c.MaturityScore < 3 {
			openControls = append(openControls, c)
		}
	}
	pdf.CellFormat(180, 9, fmt.Sprintf("Offene Maßnahmen (%d)", len(openControls)), "", 1, "L", false, 0, "")

	pdf.SetFillColor(37, 99, 235)
	pdf.SetTextColor(255, 255, 255)
	pdf.SetFont("Helvetica", "B", 8)
	pdf.CellFormat(30, 6, "Control-ID", "0", 0, "L", true, 0, "")
	pdf.CellFormat(95, 6, "Titel", "0", 0, "L", true, 0, "")
	pdf.CellFormat(20, 6, "Score", "0", 0, "C", true, 0, "")
	pdf.CellFormat(35, 6, "Lücke", "0", 1, "C", true, 0, "")

	if len(openControls) == 0 {
		pdf.SetFillColor(245, 247, 255)
		pdf.SetTextColor(34, 197, 94)
		pdf.SetFont("Helvetica", "I", 8)
		pdf.CellFormat(180, 6, "Keine offenen Maßnahmen — alle Controls vollständig umgesetzt!", "0", 1, "C", true, 0, "")
	} else {
		gapMap := make(map[string]int) // controlID → gap
		if gaps != nil {
			for _, g := range gaps.Gaps {
				gapMap[g.Control.ID] = g.MaturityGap
			}
		}
		for i, c := range openControls {
			if pdf.GetY() > 270 {
				pdf.AddPage()
			}
			if i%2 == 0 {
				pdf.SetFillColor(255, 245, 245)
			} else {
				pdf.SetFillColor(255, 255, 255)
			}
			pdf.SetTextColor(30, 30, 40)
			pdf.SetFont("Helvetica", "", 8)
			cid := c.ControlID
			if len(cid) > 18 {
				cid = cid[:15] + "..."
			}
			title := c.Title
			if len(title) > 57 {
				title = title[:54] + "..."
			}
			gap := gapMap[c.ID]
			if gap == 0 {
				gap = 3 - c.MaturityScore
			}
			pdf.CellFormat(30, 6, cid, "0", 0, "L", true, 0, "")
			pdf.CellFormat(95, 6, title, "0", 0, "L", true, 0, "")
			pdf.SetTextColor(220, 38, 38)
			pdf.SetFont("Helvetica", "B", 8)
			pdf.CellFormat(20, 6, fmt.Sprintf("%d / 3", c.MaturityScore), "0", 0, "C", true, 0, "")
			pdf.CellFormat(35, 6, fmt.Sprintf("-%d fehlen", gap), "0", 1, "C", true, 0, "")
		}
	}

	// ────────────────────────────────────────────────────────────────────────
	// Final page: Evidence list
	// ────────────────────────────────────────────────────────────────────────
	pdf.AddPage()

	pdf.SetTextColor(30, 30, 40)
	pdf.SetFont("Helvetica", "B", 14)
	pdf.SetXY(15, 15)
	pdf.CellFormat(180, 9, "Nachweisliste", "", 1, "L", false, 0, "")

	pdf.SetFillColor(37, 99, 235)
	pdf.SetTextColor(255, 255, 255)
	pdf.SetFont("Helvetica", "B", 8)
	pdf.CellFormat(30, 6, "Control-ID", "0", 0, "L", true, 0, "")
	pdf.CellFormat(115, 6, "Titel", "0", 0, "L", true, 0, "")
	pdf.CellFormat(35, 6, "Nachweise", "0", 1, "C", true, 0, "")

	if len(controls) == 0 {
		pdf.SetFillColor(245, 247, 255)
		pdf.SetTextColor(100, 100, 120)
		pdf.SetFont("Helvetica", "I", 8)
		pdf.CellFormat(180, 6, "Keine Einträge", "0", 1, "C", true, 0, "")
	} else {
		for i, c := range controls {
			if pdf.GetY() > 270 {
				pdf.AddPage()
			}
			if i%2 == 0 {
				pdf.SetFillColor(245, 247, 255)
			} else {
				pdf.SetFillColor(255, 255, 255)
			}
			pdf.SetTextColor(30, 30, 40)
			pdf.SetFont("Helvetica", "", 8)
			cid := c.ControlID
			if len(cid) > 18 {
				cid = cid[:15] + "..."
			}
			title := c.Title
			if len(title) > 69 {
				title = title[:66] + "..."
			}
			pdf.CellFormat(30, 6, cid, "0", 0, "L", true, 0, "")
			pdf.CellFormat(115, 6, title, "0", 0, "L", true, 0, "")
			countText := fmt.Sprintf("%d", c.EvidenceCount)
			if c.EvidenceCount == 0 {
				pdf.SetTextColor(220, 38, 38)
			} else {
				pdf.SetTextColor(34, 197, 94)
			}
			pdf.SetFont("Helvetica", "B", 8)
			pdf.CellFormat(35, 6, countText, "0", 1, "C", true, 0, "")
		}
	}

	var buf bytes.Buffer
	if err := pdf.Output(&buf); err != nil {
		return nil, fmt.Errorf("pdf output: %w", err)
	}
	return buf.Bytes(), nil
}

// GenerateFrameworkPDF renders a human-readable compliance overview as PDF bytes.
func GenerateFrameworkPDF(report *ReadinessReport, gaps *GapAnalysis, orgName string) ([]byte, error) {
	pdf := fpdf.New("P", "mm", "A4", "")
	pdf.SetMargins(15, 15, 15)
	pdf.SetAutoPageBreak(true, 15)
	pdf.AddPage()

	now := time.Now()

	// ── Header bar ────────────────────────────────────────────────────────────
	pdf.SetFillColor(37, 99, 235)
	pdf.Rect(0, 0, 210, 28, "F")
	pdf.SetTextColor(255, 255, 255)
	pdf.SetFont("Helvetica", "B", 14)
	pdf.SetXY(15, 8)
	pdf.CellFormat(180, 8, "Vakt — Compliance-Übersicht", "", 1, "L", false, 0, "")
	pdf.SetFont("Helvetica", "", 9)
	pdf.SetXY(15, 17)
	pdf.CellFormat(180, 6, orgName, "", 1, "L", false, 0, "")

	// ── Title ─────────────────────────────────────────────────────────────────
	pdf.SetTextColor(30, 30, 40)
	pdf.SetXY(15, 35)
	pdf.SetFont("Helvetica", "B", 18)
	pdfTitle := report.FrameworkName
	if report.FrameworkName == "DORA" {
		pdfTitle = "DORA-Framework"
	}
	pdf.CellFormat(180, 10, pdfTitle, "", 1, "L", false, 0, "")
	pdf.SetFont("Helvetica", "", 10)
	pdf.SetTextColor(100, 100, 120)
	pdf.CellFormat(180, 7, fmt.Sprintf("Erstellt am %s", now.Format("02.01.2006 15:04")), "", 1, "L", false, 0, "")

	// ── Readiness score ───────────────────────────────────────────────────────
	pdf.SetY(pdf.GetY() + 8)
	pdf.SetTextColor(30, 30, 40)
	pdf.SetFont("Helvetica", "B", 11)
	pdf.CellFormat(180, 8, "Bereitschaftsgrad", "", 1, "L", false, 0, "")

	score := report.ReadinessScore
	scoreColor := [3]int{220, 38, 38}
	if score >= 80 {
		scoreColor = [3]int{34, 197, 94}
	} else if score >= 50 {
		scoreColor = [3]int{234, 179, 8}
	}

	cy := pdf.GetY() + 2
	pdf.SetFillColor(scoreColor[0], scoreColor[1], scoreColor[2])
	pdf.RoundedRect(15, cy, 42, 22, 3, "1234", "F")
	pdf.SetTextColor(255, 255, 255)
	pdf.SetFont("Helvetica", "B", 18)
	pdf.SetXY(15, cy+3)
	pdf.CellFormat(42, 12, fmt.Sprintf("%.0f%%", score), "", 1, "C", false, 0, "")

	infoX := 62.0
	pdf.SetTextColor(30, 30, 40)
	pdf.SetFont("Helvetica", "", 9)
	pdf.SetXY(infoX, cy+2)
	pdf.CellFormat(130, 5, fmt.Sprintf("Gesamt: %d Controls", report.TotalControls), "0", 1, "L", false, 0, "")
	pdf.SetXY(infoX, pdf.GetY())
	pdf.SetTextColor(34, 197, 94)
	pdf.CellFormat(130, 5, fmt.Sprintf("Abgedeckt: %d", report.Covered), "0", 1, "L", false, 0, "")
	pdf.SetXY(infoX, pdf.GetY())
	pdf.SetTextColor(234, 179, 8)
	pdf.CellFormat(130, 5, fmt.Sprintf("Teilweise: %d", report.Partial), "0", 1, "L", false, 0, "")
	pdf.SetXY(infoX, pdf.GetY())
	pdf.SetTextColor(220, 38, 38)
	pdf.CellFormat(130, 5, fmt.Sprintf("Fehlend: %d", report.Missing), "0", 1, "L", false, 0, "")

	// ── By-domain table ───────────────────────────────────────────────────────
	pdf.SetY(cy + 30)
	pdf.SetTextColor(30, 30, 40)
	pdf.SetFont("Helvetica", "B", 11)
	pdf.CellFormat(180, 8, "Domänen-Übersicht", "", 1, "L", false, 0, "")

	if len(report.ByDomain) > 0 {
		pdf.SetFillColor(37, 99, 235)
		pdf.SetTextColor(255, 255, 255)
		pdf.SetFont("Helvetica", "B", 8)
		pdf.CellFormat(100, 6, "Domäne", "0", 0, "L", true, 0, "")
		pdf.CellFormat(25, 6, "Controls", "0", 0, "C", true, 0, "")
		pdf.CellFormat(25, 6, "Abgedeckt", "0", 0, "C", true, 0, "")
		pdf.CellFormat(30, 6, "Bereitschaft", "0", 1, "C", true, 0, "")

		for i, d := range report.ByDomain {
			if pdf.GetY() > 270 {
				pdf.AddPage()
			}
			if i%2 == 0 {
				pdf.SetFillColor(245, 247, 255)
			} else {
				pdf.SetFillColor(255, 255, 255)
			}
			pdf.SetTextColor(30, 30, 40)
			pdf.SetFont("Helvetica", "", 8)
			name := d.Domain
			if len(name) > 60 {
				name = name[:57] + "..."
			}
			pdf.CellFormat(100, 6, name, "0", 0, "L", true, 0, "")
			pdf.CellFormat(25, 6, fmt.Sprintf("%d", d.Total), "0", 0, "C", true, 0, "")
			pdf.CellFormat(25, 6, fmt.Sprintf("%d", d.Covered), "0", 0, "C", true, 0, "")
			col := [3]int{220, 38, 38}
			if d.Score >= 80 {
				col = [3]int{34, 197, 94}
			} else if d.Score >= 50 {
				col = [3]int{234, 179, 8}
			}
			pdf.SetTextColor(col[0], col[1], col[2])
			pdf.SetFont("Helvetica", "B", 8)
			pdf.CellFormat(30, 6, fmt.Sprintf("%.0f%%", d.Score), "0", 1, "C", true, 0, "")
		}
	}

	// ── Gap list ──────────────────────────────────────────────────────────────
	if gaps != nil && len(gaps.Gaps) > 0 {
		pdf.SetY(pdf.GetY() + 6)
		pdf.SetTextColor(30, 30, 40)
		pdf.SetFont("Helvetica", "B", 11)
		pdf.CellFormat(180, 8, fmt.Sprintf("Lücken (%d)", len(gaps.Gaps)), "", 1, "L", false, 0, "")

		pdf.SetFillColor(37, 99, 235)
		pdf.SetTextColor(255, 255, 255)
		pdf.SetFont("Helvetica", "B", 8)
		pdf.CellFormat(30, 6, "Control-ID", "0", 0, "L", true, 0, "")
		pdf.CellFormat(100, 6, "Titel", "0", 0, "L", true, 0, "")
		pdf.CellFormat(50, 6, "Grund", "0", 1, "L", true, 0, "")

		reasonMap := map[string]string{
			"no_evidence":       "Kein Nachweis",
			"evidence_expiring": "Nachweis läuft ab",
			"review_pending":    "Prüfung ausstehend",
		}

		for i, gap := range gaps.Gaps {
			if pdf.GetY() > 270 {
				pdf.AddPage()
			}
			if i%2 == 0 {
				pdf.SetFillColor(255, 240, 240)
			} else {
				pdf.SetFillColor(255, 255, 255)
			}
			cid := gap.Control.ControlID
			if len(cid) > 18 {
				cid = cid[:15] + "..."
			}
			title := gap.Control.Title
			if len(title) > 60 {
				title = title[:57] + "..."
			}
			reason := reasonMap[gap.Reason]
			if reason == "" {
				reason = gap.Reason
			}
			pdf.SetTextColor(30, 30, 40)
			pdf.SetFont("Helvetica", "", 8)
			pdf.CellFormat(30, 6, cid, "0", 0, "L", true, 0, "")
			pdf.CellFormat(100, 6, title, "0", 0, "L", true, 0, "")
			pdf.SetTextColor(180, 60, 0)
			pdf.CellFormat(50, 6, reason, "0", 1, "L", true, 0, "")
		}
	} else {
		pdf.SetY(pdf.GetY() + 6)
		pdf.SetFont("Helvetica", "I", 10)
		pdf.SetTextColor(100, 100, 120)
		pdf.CellFormat(180, 8, "Keine Lücken gefunden — vollständige Abdeckung!", "", 1, "C", false, 0, "")
	}

	// ── Footer ────────────────────────────────────────────────────────────────
	pdf.SetFooterFunc(func() {
		pdf.SetY(-12)
		pdf.SetFont("Helvetica", "I", 7)
		pdf.SetTextColor(150, 150, 160)
		pdf.CellFormat(0, 5,
			fmt.Sprintf("Vakt Comply — %s — Seite %d/{nb}", orgName, pdf.PageNo()),
			"", 0, "C", false, 0, "")
	})
	pdf.AliasNbPages("{nb}")

	var buf bytes.Buffer
	if err := pdf.Output(&buf); err != nil {
		return nil, fmt.Errorf("pdf output: %w", err)
	}
	return buf.Bytes(), nil
}

// GenerateAssessmentReportPDFBytes renders a supplier assessment report as PDF.
func GenerateAssessmentReportPDFBytes(asm *AssessmentWithQuestionnaire, supplier *Supplier, answers []AnswerWithReview, status SupplierStatus) ([]byte, error) {
	pdf := fpdf.New("P", "mm", "A4", "")
	pdf.SetMargins(15, 15, 15)
	pdf.SetAutoPageBreak(true, 15)
	pdf.AddPage()

	pdf.SetFillColor(37, 99, 235)
	pdf.Rect(0, 0, 210, 28, "F")
	pdf.SetTextColor(255, 255, 255)
	pdf.SetFont("Helvetica", "B", 14)
	pdf.SetXY(15, 8)
	pdf.CellFormat(180, 8, "Vakt - Lieferanten-Assessment-Report", "", 1, "L", false, 0, "")
	pdf.SetFont("Helvetica", "", 9)
	pdf.SetXY(15, 17)
	pdf.CellFormat(180, 6, supplier.Name, "", 1, "L", false, 0, "")

	pdf.SetTextColor(30, 30, 40)
	pdf.SetXY(15, 35)
	pdf.SetFont("Helvetica", "B", 11)
	pdf.CellFormat(60, 6, "Lieferant:", "", 0, "L", false, 0, "")
	pdf.SetFont("Helvetica", "", 11)
	pdf.CellFormat(120, 6, supplier.Name, "", 1, "L", false, 0, "")

	assessmentDate := asm.CreatedAt.Format("02.01.2006")
	if asm.SubmittedAt != nil {
		assessmentDate = asm.SubmittedAt.Format("02.01.2006")
	}
	pdf.SetXY(15, pdf.GetY())
	pdf.SetFont("Helvetica", "B", 11)
	pdf.CellFormat(60, 6, "Assessment-Datum:", "", 0, "L", false, 0, "")
	pdf.SetFont("Helvetica", "", 11)
	pdf.CellFormat(120, 6, assessmentDate, "", 1, "L", false, 0, "")

	pdf.SetXY(15, pdf.GetY())
	pdf.SetFont("Helvetica", "B", 11)
	pdf.CellFormat(60, 6, "Status:", "", 0, "L", false, 0, "")
	pdf.SetFont("Helvetica", "", 11)
	statusLabels := map[string]string{"green": "Gruen (OK)", "yellow": "Gelb (Warnung)", "red": "Rot (Kritisch)"}
	statusLabel := statusLabels[status.Status]
	if statusLabel == "" {
		statusLabel = status.Status
	}
	pdf.CellFormat(120, 6, statusLabel, "", 1, "L", false, 0, "")

	pdf.SetXY(15, pdf.GetY())
	pdf.SetFont("Helvetica", "B", 11)
	pdf.CellFormat(60, 6, "Score:", "", 0, "L", false, 0, "")
	pdf.SetFont("Helvetica", "", 11)
	pdf.CellFormat(120, 6, fmt.Sprintf("%d / 100", status.Score), "", 1, "L", false, 0, "")

	pdf.SetXY(15, pdf.GetY()+4)

	pdf.SetFont("Helvetica", "B", 10)
	pdf.SetFillColor(240, 240, 240)
	pdf.SetTextColor(30, 30, 40)
	pdf.CellFormat(90, 7, "Frage", "1", 0, "L", true, 0, "")
	pdf.CellFormat(70, 7, "Antwort", "1", 0, "L", true, 0, "")
	pdf.CellFormat(30, 7, "Review", "1", 1, "L", true, 0, "")

	pdf.SetFont("Helvetica", "", 9)
	for _, a := range answers {
		reviewLabel := "-"
		if a.ReviewStatus != nil {
			switch *a.ReviewStatus {
			case "accepted":
				reviewLabel = "Akzeptiert"
			case "needs_rework":
				reviewLabel = "Nacharbeit"
			}
		}
		answerText := a.AnswerText
		if answerText == "" && a.FileURL != "" {
			answerText = "[Datei hochgeladen]"
		}
		if len(answerText) > 80 {
			answerText = answerText[:77] + "..."
		}
		questionText := a.QuestionText
		if len(questionText) > 80 {
			questionText = questionText[:77] + "..."
		}
		rowY := pdf.GetY()
		pdf.SetXY(15, rowY)
		pdf.CellFormat(90, 5, questionText, "1", 0, "L", false, 0, "")
		pdf.CellFormat(70, 5, answerText, "1", 0, "L", false, 0, "")
		pdf.CellFormat(30, 5, reviewLabel, "1", 1, "L", false, 0, "")
	}

	var buf bytes.Buffer
	if err := pdf.Output(&buf); err != nil {
		return nil, fmt.Errorf("assessment pdf output: %w", err)
	}
	return buf.Bytes(), nil
}

// GenerateAIDocumentationPDF produces the technical dossier PDF for an AI system (Art. 11, Annex IV EU AI Act).
func GenerateAIDocumentationPDF(system *AISystem, doc *AIDocumentation) ([]byte, error) {
	pdf := fpdf.New("P", "mm", "A4", "")
	pdf.SetMargins(15, 15, 15)
	pdf.SetAutoPageBreak(true, 15)
	pdf.AddPage()

	now := time.Now()

	// Header
	pdf.SetFillColor(37, 99, 235)
	pdf.Rect(0, 0, 210, 28, "F")
	pdf.SetTextColor(255, 255, 255)
	pdf.SetFont("Helvetica", "B", 14)
	pdf.SetXY(15, 8)
	pdf.CellFormat(180, 8, "Vakt — Technisches Dossier (EU AI Act Art. 11)", "", 1, "L", false, 0, "")
	pdf.SetFont("Helvetica", "", 9)
	pdf.SetXY(15, 17)
	pdf.CellFormat(180, 6, fmt.Sprintf("KI-System: %s | Version: %d | Erstellt: %s", system.Name, doc.Version, now.Format("02.01.2006")), "", 1, "L", false, 0, "")

	pdf.SetTextColor(30, 30, 40)
	pdf.SetY(35)

	type section struct {
		title   string
		content string
		article string
	}

	sections := []section{
		{"1. Systembeschreibung und Verwendungszweck", doc.SystemDescription + "\n\n" + doc.IntendedPurpose, "Annex IV Nr. 1"},
		{"2. Trainingsdaten und Datenqualität", doc.TrainingData + "\n\n" + doc.DataQuality, "Annex IV Nr. 2"},
		{"3. Leistungsmetriken und Systemgrenzen", doc.PerformanceMetrics + "\n\n" + doc.SystemLimits, "Annex IV Nr. 3"},
		{"4. Risikomanagementsystem", doc.RiskManagement, "Art. 9 EU AI Act"},
		{"5. Maßnahmen zur menschlichen Aufsicht", doc.HumanOversight, "Art. 14 EU AI Act"},
		{"6. Protokollierung und Audit-Trail", doc.LoggingAuditTrail, "Art. 12 EU AI Act"},
	}

	for _, s := range sections {
		pdf.SetFont("Helvetica", "B", 11)
		pdf.SetTextColor(30, 30, 40)
		pdf.CellFormat(0, 8, s.title, "", 1, "L", false, 0, "")
		pdf.SetFont("Helvetica", "I", 8)
		pdf.SetTextColor(100, 100, 140)
		pdf.CellFormat(0, 5, s.article, "", 1, "L", false, 0, "")
		pdf.SetFont("Helvetica", "", 10)
		pdf.SetTextColor(50, 50, 60)
		content := s.content
		if content == "" || content == "\n\n" {
			content = "(Noch nicht ausgefüllt)"
		}
		pdf.MultiCell(0, 6, content, "LB", "L", false)
		pdf.SetY(pdf.GetY() + 4)
	}

	// Footer
	pdf.SetFont("Helvetica", "I", 8)
	pdf.SetTextColor(150, 150, 160)
	if doc.AuthoredBy != "" {
		pdf.SetXY(15, 270)
		pdf.CellFormat(0, 5, fmt.Sprintf("Verfasst von: %s", doc.AuthoredBy), "", 1, "L", false, 0, "")
	}

	var buf bytes.Buffer
	if err := pdf.Output(&buf); err != nil {
		return nil, fmt.Errorf("render ai documentation pdf: %w", err)
	}
	return buf.Bytes(), nil
}

// GenerateEUAIActReportPDF produces the full EU AI Act compliance report PDF.
func GenerateEUAIActReportPDF(dashboard *EUAIActDashboard, systems []AISystem) ([]byte, error) {
	pdf := fpdf.New("P", "mm", "A4", "")
	pdf.SetMargins(15, 15, 15)
	pdf.SetAutoPageBreak(true, 15)
	pdf.AddPage()

	now := time.Now()

	// Header
	pdf.SetFillColor(37, 99, 235)
	pdf.Rect(0, 0, 210, 28, "F")
	pdf.SetTextColor(255, 255, 255)
	pdf.SetFont("Helvetica", "B", 14)
	pdf.SetXY(15, 8)
	pdf.CellFormat(180, 8, "Vakt — EU AI Act Compliance Report", "", 1, "L", false, 0, "")
	pdf.SetFont("Helvetica", "", 9)
	pdf.SetXY(15, 17)
	pdf.CellFormat(180, 6, fmt.Sprintf("Erstellt: %s | KI-Systeme: %d | Frist Hochrisiko: %s", now.Format("02.01.2006"), dashboard.TotalSystems, dashboard.HighRiskDeadline), "", 1, "L", false, 0, "")

	pdf.SetTextColor(30, 30, 40)
	pdf.SetY(35)

	// Summary section
	pdf.SetFont("Helvetica", "B", 12)
	pdf.CellFormat(0, 8, "Zusammenfassung", "", 1, "L", false, 0, "")
	pdf.SetFont("Helvetica", "", 10)
	pdf.CellFormat(0, 6, fmt.Sprintf("Gesamtanzahl KI-Systeme: %d", dashboard.TotalSystems), "", 1, "L", false, 0, "")
	pdf.CellFormat(0, 6, fmt.Sprintf("Ohne technisches Dossier: %d", dashboard.SystemsWithoutDocs), "", 1, "L", false, 0, "")
	pdf.CellFormat(0, 6, fmt.Sprintf("Tage bis Hochrisiko-Frist (%s): %d", dashboard.HighRiskDeadline, dashboard.HighRiskDeadlineDaysLeft), "", 1, "L", false, 0, "")
	pdf.SetY(pdf.GetY() + 4)

	// Systems by risk class
	pdf.SetFont("Helvetica", "B", 11)
	pdf.CellFormat(0, 8, "Systeme nach Risikoklasse", "", 1, "L", false, 0, "")
	pdf.SetFont("Helvetica", "", 10)
	riskOrder := []string{"unacceptable", "high", "limited", "minimal", "unclassified"}
	riskLabels := map[string]string{
		"unacceptable": "Inakzeptables Risiko (Verboten)",
		"high":         "Hohes Risiko (Annex III)",
		"limited":      "Begrenztes Risiko (Transparenzpflicht)",
		"minimal":      "Minimales Risiko",
		"unclassified": "Noch nicht klassifiziert",
	}
	for _, rc := range riskOrder {
		if count, ok := dashboard.SystemsByRiskClass[rc]; ok && count > 0 {
			pdf.CellFormat(0, 6, fmt.Sprintf("  %s: %d", riskLabels[rc], count), "", 1, "L", false, 0, "")
		}
	}
	pdf.SetY(pdf.GetY() + 4)

	// System inventory table
	if len(systems) > 0 {
		pdf.SetFont("Helvetica", "B", 11)
		pdf.CellFormat(0, 8, "KI-System-Inventar", "", 1, "L", false, 0, "")
		pdf.SetFont("Helvetica", "B", 9)
		pdf.SetFillColor(230, 235, 245)
		pdf.CellFormat(70, 6, "Name", "B", 0, "L", true, 0, "")
		pdf.CellFormat(45, 6, "Risikoklasse", "B", 0, "L", true, 0, "")
		pdf.CellFormat(35, 6, "Status", "B", 0, "L", true, 0, "")
		pdf.CellFormat(30, 6, "Klassifiziert", "B", 1, "L", true, 0, "")
		pdf.SetFont("Helvetica", "", 9)
		for _, s := range systems {
			rc := s.RiskClass
			if rc == "" {
				rc = "—"
			}
			by := s.ClassifiedBy
			if by == "" {
				by = "—"
			}
			pdf.CellFormat(70, 5, s.Name, "", 0, "L", false, 0, "")
			pdf.CellFormat(45, 5, rc, "", 0, "L", false, 0, "")
			pdf.CellFormat(35, 5, s.Status, "", 0, "L", false, 0, "")
			pdf.CellFormat(30, 5, by, "", 1, "L", false, 0, "")
		}
		pdf.SetY(pdf.GetY() + 4)
	}

	// ISO 27001 mapping table
	if len(dashboard.ISO27001Mappings) > 0 {
		pdf.AddPage()
		pdf.SetFont("Helvetica", "B", 12)
		pdf.CellFormat(0, 8, "Mapping: EU AI Act ↔ ISO 27001", "", 1, "L", false, 0, "")
		pdf.SetFont("Helvetica", "", 9)
		pdf.SetTextColor(100, 100, 120)
		pdf.MultiCell(0, 5, "Nachfolgende Controls überschneiden sich zwischen EU AI Act und ISO 27001 — eine ISO-27001-konforme Organisation hat damit bereits wesentliche EU AI Act Anforderungen abgedeckt.", "", "L", false)
		pdf.SetTextColor(30, 30, 40)
		pdf.SetY(pdf.GetY() + 4)
		pdf.SetFont("Helvetica", "B", 9)
		pdf.SetFillColor(230, 235, 245)
		pdf.CellFormat(30, 6, "Artikel", "B", 0, "L", true, 0, "")
		pdf.CellFormat(55, 6, "EU AI Act Anforderung", "B", 0, "L", true, 0, "")
		pdf.CellFormat(30, 6, "ISO 27001", "B", 0, "L", true, 0, "")
		pdf.CellFormat(65, 6, "ISO 27001 Titel", "B", 1, "L", true, 0, "")
		pdf.SetFont("Helvetica", "", 9)
		for _, m := range dashboard.ISO27001Mappings {
			pdf.CellFormat(30, 5, m.EUAIActArticle, "", 0, "L", false, 0, "")
			pdf.CellFormat(55, 5, m.EUAIActTopic, "", 0, "L", false, 0, "")
			pdf.CellFormat(30, 5, m.ISO27001Control, "", 0, "L", false, 0, "")
			pdf.CellFormat(65, 5, m.ISO27001Title, "", 1, "L", false, 0, "")
		}
	}

	var buf bytes.Buffer
	if err := pdf.Output(&buf); err != nil {
		return nil, fmt.Errorf("render eu ai act report pdf: %w", err)
	}
	return buf.Bytes(), nil
}

// GenerateNIS2ReportFormPDF generates a BSI-layout NIS2 Meldungsformular PDF.
// reportType is "24h", "72h", or "30d".
func GenerateNIS2ReportFormPDF(incident *Incident, reportType, orgName string) ([]byte, error) {
	authority := incident.NotificationAuthority
	if authority == "" {
		authority = "BSI"
	}
	authInfo, ok := incidentAuthorityDirectory[authority]
	if !ok {
		authInfo = incidentAuthorityDirectory["BSI"]
	}

	titleMap := map[string]string{
		"24h": "NIS2 Frühmeldung (T+24h)",
		"72h": "NIS2 Vollmeldung (T+72h)",
		"30d": "NIS2 Abschlussbericht (T+30d)",
	}
	formTitle := titleMap[reportType]
	if formTitle == "" {
		formTitle = "NIS2 Meldung"
	}

	pdf := fpdf.New("P", "mm", "A4", "")
	pdf.SetMargins(15, 15, 15)
	pdf.SetAutoPageBreak(true, 15)
	pdf.AddPage()
	now := time.Now()

	// Header
	pdf.SetFillColor(37, 99, 235)
	pdf.Rect(0, 0, 210, 28, "F")
	pdf.SetTextColor(255, 255, 255)
	pdf.SetFont("Helvetica", "B", 14)
	pdf.SetXY(15, 8)
	pdf.CellFormat(180, 8, "Vakt Comply — "+formTitle, "", 1, "L", false, 0, "")
	pdf.SetFont("Helvetica", "", 9)
	pdf.SetXY(15, 17)
	pdf.CellFormat(90, 6, orgName, "", 0, "L", false, 0, "")
	pdf.CellFormat(90, 6, "Erstellt: "+now.Format("02.01.2006 15:04"), "", 1, "R", false, 0, "")

	addSection := func(label string) {
		pdf.SetY(pdf.GetY() + 4)
		pdf.SetFillColor(240, 244, 255)
		pdf.SetTextColor(30, 30, 40)
		pdf.SetFont("Helvetica", "B", 10)
		pdf.CellFormat(180, 7, label, "1", 1, "L", true, 0, "")
		pdf.SetFont("Helvetica", "", 9)
		pdf.SetTextColor(50, 50, 60)
	}
	addField := func(label, value string) {
		pdf.SetFont("Helvetica", "B", 9)
		pdf.SetTextColor(80, 80, 90)
		pdf.CellFormat(55, 6, label+":", "", 0, "L", false, 0, "")
		pdf.SetFont("Helvetica", "", 9)
		pdf.SetTextColor(30, 30, 40)
		pdf.MultiCell(125, 6, value, "", "L", false)
	}

	// Section 1: Meldende Organisation
	addSection("1. Meldende Organisation")
	addField("Organisation", orgName)
	addField("Zuständige Behörde", authInfo.Name)
	addField("Einreichungskanal", authInfo.Portal)

	// Section 2: Vorfall-Identifikation
	addSection("2. Vorfall-Identifikation")
	addField("Vorfallbezeichnung", incident.Title)
	addField("Vorfalls-ID", incident.ID)
	addField("Schweregrad", incident.Severity)
	addField("Status", incident.Status)
	addField("Ersterkennung", incident.DiscoveredAt.Format("02.01.2006 15:04 Uhr"))

	// Section 3: Beschreibung
	addSection("3. Beschreibung des Vorfalls")
	pdf.SetFont("Helvetica", "", 9)
	if incident.Description != "" {
		pdf.MultiCell(180, 6, incident.Description, "", "L", false)
	} else {
		pdf.MultiCell(180, 6, "(keine Beschreibung hinterlegt)", "", "L", false)
	}

	// Section 4: Betroffene Systeme
	addSection("4. Betroffene Systeme")
	systems := "(keine Angabe)"
	if len(incident.AffectedSystems) > 0 {
		systems = ""
		for _, s := range incident.AffectedSystems {
			systems += "• " + s + "\n"
		}
	}
	pdf.MultiCell(180, 6, systems, "", "L", false)

	// Section 5: Ergriffene Maßnahmen (placeholder)
	addSection("5. Ergriffene Sofortmaßnahmen")
	pdf.MultiCell(180, 6, "(Bitte ergänzen Sie vor der Einreichung die ergriffenen Sofortmaßnahmen.)", "", "L", false)

	// Section 6: Meldefrist
	addSection("6. Meldetermin")
	addField("Meldetyp", formTitle)
	deadline := "(nicht gesetzt)"
	switch reportType {
	case "24h":
		if incident.Deadline24h != nil {
			deadline = incident.Deadline24h.Format("02.01.2006 15:04 Uhr")
		}
	case "72h":
		if incident.Deadline72h != nil {
			deadline = incident.Deadline72h.Format("02.01.2006 15:04 Uhr")
		}
	case "30d":
		if incident.Deadline30d != nil {
			deadline = incident.Deadline30d.Format("02.01.2006 15:04 Uhr")
		}
	}
	addField("Frist", deadline)

	// Section 7: Einreichungshinweis
	addSection("7. Einreichungshinweis")
	pdf.SetFont("Helvetica", "", 9)
	pdf.SetTextColor(50, 50, 60)
	pdf.MultiCell(180, 6, authInfo.SubmitNote, "", "L", false)
	pdf.SetY(pdf.GetY() + 2)
	pdf.SetFont("Helvetica", "", 8)
	pdf.SetTextColor(100, 100, 120)
	pdf.MultiCell(180, 5,
		"Notfall-Hotline: "+authInfo.Phone+"\n"+
			"Dieses Dokument wurde automatisch erstellt und ersetzt keine Rechtsberatung.",
		"", "L", false)

	var out bytes.Buffer
	if err := pdf.Output(&out); err != nil {
		return nil, fmt.Errorf("render nis2 report form pdf: %w", err)
	}
	return out.Bytes(), nil
}

// GenerateSoAPDF renders an ISO 27001 Statement of Applicability as a PDF document.
// Controls are grouped by domain (A.5, A.6, A.7, A.8).
func GenerateSoAPDF(rows []SoARow, frameworkName, orgName string, generatedAt time.Time) ([]byte, error) {
	pdf := fpdf.New("L", "mm", "A4", "") // Landscape for the wide table
	pdf.SetMargins(12, 12, 12)
	pdf.SetAutoPageBreak(true, 14)
	pdf.AddPage()

	// ── Header ───────────────────────────────────────────────────────────────
	pdf.SetFillColor(37, 99, 235)
	pdf.Rect(0, 0, 297, 24, "F")
	pdf.SetTextColor(255, 255, 255)
	pdf.SetFont("Helvetica", "B", 13)
	pdf.SetXY(12, 6)
	pdf.CellFormat(180, 8, "Vakt — Statement of Applicability", "", 0, "L", false, 0, "")
	pdf.SetFont("Helvetica", "", 9)
	pdf.SetXY(12, 15)
	pdf.CellFormat(180, 6, frameworkName+" · "+orgName, "", 0, "L", false, 0, "")
	pdf.SetXY(210, 6)
	pdf.CellFormat(75, 14, "Erstellt: "+generatedAt.Format("02.01.2006"), "", 0, "R", false, 0, "")

	// ── Intro ────────────────────────────────────────────────────────────────
	pdf.SetTextColor(30, 30, 40)
	pdf.SetXY(12, 28)
	pdf.SetFont("Helvetica", "", 8)
	pdf.MultiCell(273, 4.5,
		"Dieses Dokument listet alle Maßnahmen gemäß "+frameworkName+
			" und gibt für jede an, ob sie anwendbar ist (inkl. Begründung) sowie den aktuellen Umsetzungsstand."+
			" Es dient als formelles Nachweis-Dokument für interne und externe Audits.",
		"", "L", false)

	pdf.SetY(pdf.GetY() + 3)

	// Group rows by domain
	seen := make(map[string]bool)
	var order []string
	byDomain := make(map[string][]SoARow)
	for _, r := range rows {
		if !seen[r.Domain] {
			seen[r.Domain] = true
			order = append(order, r.Domain)
		}
		byDomain[r.Domain] = append(byDomain[r.Domain], r)
	}

	// Column widths (landscape A4 usable = 273mm)
	colW := [6]float64{18, 70, 16, 65, 65, 20}
	headers := [6]string{"Control", "Bezeichnung", "Anwendbar", "Begründung / Ausschluss", "Umsetzungsstand", "Evidence"}

	renderTableHeader := func() {
		pdf.SetFillColor(30, 64, 175)
		pdf.SetTextColor(255, 255, 255)
		pdf.SetFont("Helvetica", "B", 7)
		x := pdf.GetX()
		y := pdf.GetY()
		for i, w := range colW {
			pdf.SetXY(x, y)
			pdf.CellFormat(w, 6, headers[i], "1", 0, "C", true, 0, "")
			x += w
		}
		pdf.Ln(6)
	}

	for _, domain := range order {
		domainRows := byDomain[domain]

		// Domain section header
		pdf.SetFillColor(219, 234, 254)
		pdf.SetTextColor(30, 64, 175)
		pdf.SetFont("Helvetica", "B", 8)
		pdf.CellFormat(273, 6, "  "+domain, "1", 1, "L", true, 0, "")
		pdf.SetTextColor(30, 30, 40)

		renderTableHeader()

		for i, row := range domainRows {
			// Alternate row shading
			if i%2 == 0 {
				pdf.SetFillColor(248, 250, 252)
			} else {
				pdf.SetFillColor(255, 255, 255)
			}

			applicable := "Ja"
			if !row.Applicable {
				applicable = "Nein"
			}

			status := row.ManualStatus
			if status == "" && row.EvidenceCount > 0 {
				status = "in Umsetzung"
			} else if status == "implemented" {
				status = "Umgesetzt"
			} else if status == "in_progress" {
				status = "In Bearbeitung"
			} else if status == "" {
				status = "Offen"
			}

			// Calculate row height based on longest cell
			lineH := 4.2
			cells := []struct {
				text string
				w    float64
			}{
				{row.ControlID, colW[0]},
				{row.Title, colW[1]},
				{applicable, colW[2]},
				{row.Justification, colW[3]},
				{row.Implementation + "\n" + row.Responsible, colW[4]},
			}
			maxLines := 1
			for _, cell := range cells {
				nLines := len(pdf.SplitLines([]byte(cell.text), cell.w-2))
				if nLines > maxLines {
					maxLines = nLines
				}
			}
			rowH := float64(maxLines) * lineH
			if rowH < lineH*2 {
				rowH = lineH * 2
			}

			x := 12.0
			y := pdf.GetY()

			renderCell := func(text string, w, h float64, align string) {
				pdf.SetXY(x, y)
				pdf.SetFont("Helvetica", "", 7)
				pdf.MultiCell(w, lineH, text, "1", align, true)
				x += w
			}

			renderCell(row.ControlID, colW[0], rowH, "C")
			renderCell(row.Title, colW[1], rowH, "L")

			// Applicable cell with color
			pdf.SetXY(x, y)
			if row.Applicable {
				pdf.SetTextColor(22, 163, 74)
			} else {
				pdf.SetTextColor(220, 38, 38)
			}
			pdf.SetFont("Helvetica", "B", 7)
			pdf.MultiCell(colW[2], rowH, applicable, "1", "C", true)
			pdf.SetTextColor(30, 30, 40)
			x += colW[2]

			renderCell(row.Justification, colW[3], rowH, "L")

			impl := status
			if row.Implementation != "" {
				impl += "\n" + row.Implementation
			}
			if row.Responsible != "" {
				impl += "\nVerantw.: " + row.Responsible
			}
			renderCell(impl, colW[4], rowH, "L")

			evStr := fmt.Sprintf("%d", row.EvidenceCount)
			renderCell(evStr, colW[5], rowH, "C")

			pdf.SetY(y + rowH)
		}

		pdf.Ln(3)
	}

	// ── Footer ───────────────────────────────────────────────────────────────
	pdf.SetY(-12)
	pdf.SetFont("Helvetica", "I", 7)
	pdf.SetTextColor(120, 120, 140)
	pdf.CellFormat(273, 5,
		fmt.Sprintf("Vakt — %s · Seite %d · Generiert %s · Dieses Dokument ersetzt keine Rechtsberatung.",
			frameworkName, pdf.PageNo(), generatedAt.Format("02.01.2006 15:04")),
		"", 0, "C", false, 0, "")

	var out bytes.Buffer
	if err := pdf.Output(&out); err != nil {
		return nil, fmt.Errorf("render soa pdf: %w", err)
	}
	return out.Bytes(), nil
}

// GenerateExecutiveSummaryPDF renders a one-page Compliance Executive Summary PDF.
// Sections: 1 — Gesamtstatus, 2 — Framework-Übersicht, 3 — Top 5 Risiken, 4 — Letzte 30 Tage.
func GenerateExecutiveSummaryPDF(d *ExecutiveSummaryData) ([]byte, error) {
	pdf := fpdf.New("P", "mm", "A4", "")
	pdf.SetMargins(15, 15, 15)
	pdf.SetAutoPageBreak(true, 15)

	// ── Footer ────────────────────────────────────────────────────────────────
	pdf.SetFooterFunc(func() {
		pdf.SetY(-12)
		pdf.SetFont("Helvetica", "I", 7)
		pdf.SetTextColor(150, 150, 160)
		pdf.CellFormat(0, 5,
			fmt.Sprintf("Vakt Comply — Executive Summary — %s — Seite %d/{nb}", d.OrgName, pdf.PageNo()),
			"", 0, "C", false, 0, "")
	})
	pdf.AliasNbPages("{nb}")

	// ── Cover header ──────────────────────────────────────────────────────────
	pdf.AddPage()
	pdf.SetFillColor(37, 99, 235)
	pdf.Rect(0, 0, 210, 28, "F")
	pdf.SetTextColor(255, 255, 255)
	pdf.SetFont("Helvetica", "B", 14)
	pdf.SetXY(15, 8)
	pdf.CellFormat(180, 8, "Vakt — Compliance Executive Summary", "", 1, "L", false, 0, "")
	pdf.SetFont("Helvetica", "", 9)
	pdf.SetXY(15, 17)
	pdf.CellFormat(180, 6,
		fmt.Sprintf("%s  |  %s", d.OrgName, d.GeneratedAt.Format("02.01.2006")),
		"", 1, "L", false, 0, "")

	// ── Section 1: Gesamtstatus ───────────────────────────────────────────────
	pdf.SetTextColor(30, 30, 40)
	pdf.SetXY(15, 35)
	pdf.SetFont("Helvetica", "B", 13)
	pdf.CellFormat(180, 8, "1. Gesamtstatus", "", 1, "L", false, 0, "")

	scoreColor := [3]int{220, 38, 38}
	if d.OverallScore >= 80 {
		scoreColor = [3]int{34, 197, 94}
	} else if d.OverallScore >= 50 {
		scoreColor = [3]int{234, 179, 8}
	}
	cy := pdf.GetY() + 2
	pdf.SetFillColor(scoreColor[0], scoreColor[1], scoreColor[2])
	pdf.RoundedRect(15, cy, 50, 26, 3, "1234", "F")
	pdf.SetTextColor(255, 255, 255)
	pdf.SetFont("Helvetica", "B", 22)
	pdf.SetXY(15, cy+4)
	pdf.CellFormat(50, 14, fmt.Sprintf("%.0f%%", d.OverallScore), "", 1, "C", false, 0, "")

	pdf.SetXY(72, cy+4)
	pdf.SetTextColor(30, 30, 40)
	pdf.SetFont("Helvetica", "B", 12)
	pdf.CellFormat(130, 8, "Gesamtbereitschaft (gewichteter Durchschnitt)", "0", 1, "L", false, 0, "")
	pdf.SetXY(72, cy+14)
	pdf.SetFont("Helvetica", "", 9)
	pdf.SetTextColor(100, 100, 120)
	switch {
	case d.OverallScore >= 80:
		pdf.CellFormat(130, 7, "Gut — Compliance-Anforderungen weitgehend erfüllt.", "0", 1, "L", false, 0, "")
	case d.OverallScore >= 50:
		pdf.CellFormat(130, 7, "Mittel — Handlungsbedarf vorhanden.", "0", 1, "L", false, 0, "")
	default:
		pdf.CellFormat(130, 7, "Kritisch — erheblicher Handlungsbedarf.", "0", 1, "L", false, 0, "")
	}

	// ── Section 2: Framework-Übersicht ────────────────────────────────────────
	pdf.SetY(cy + 36)
	pdf.SetTextColor(30, 30, 40)
	pdf.SetFont("Helvetica", "B", 13)
	pdf.CellFormat(180, 8, "2. Framework-Übersicht", "", 1, "L", false, 0, "")

	if len(d.Frameworks) == 0 {
		pdf.SetFont("Helvetica", "I", 9)
		pdf.SetTextColor(100, 100, 120)
		pdf.CellFormat(180, 7, "Keine Frameworks aktiviert.", "0", 1, "L", false, 0, "")
	} else {
		// Table header
		pdf.SetFillColor(37, 99, 235)
		pdf.SetTextColor(255, 255, 255)
		pdf.SetFont("Helvetica", "B", 8)
		pdf.CellFormat(80, 6, "Framework", "0", 0, "L", true, 0, "")
		pdf.CellFormat(35, 6, "Score", "0", 0, "C", true, 0, "")
		pdf.CellFormat(65, 6, "Controls umgesetzt / gesamt", "0", 1, "C", true, 0, "")

		for i, fw := range d.Frameworks {
			if pdf.GetY() > 265 {
				pdf.AddPage()
			}
			if i%2 == 0 {
				pdf.SetFillColor(245, 247, 255)
			} else {
				pdf.SetFillColor(255, 255, 255)
			}
			name := fw.Name
			if len(name) > 46 {
				name = name[:43] + "..."
			}
			col := [3]int{220, 38, 38}
			if fw.Score >= 80 {
				col = [3]int{34, 197, 94}
			} else if fw.Score >= 50 {
				col = [3]int{234, 179, 8}
			}
			pdf.SetTextColor(30, 30, 40)
			pdf.SetFont("Helvetica", "", 8)
			pdf.CellFormat(80, 6, name, "0", 0, "L", true, 0, "")
			pdf.SetTextColor(col[0], col[1], col[2])
			pdf.SetFont("Helvetica", "B", 8)
			pdf.CellFormat(35, 6, fmt.Sprintf("%.0f%%", fw.Score), "0", 0, "C", true, 0, "")
			pdf.SetTextColor(30, 30, 40)
			pdf.SetFont("Helvetica", "", 8)
			pdf.CellFormat(65, 6, fmt.Sprintf("%d / %d", fw.Implemented, fw.Total), "0", 1, "C", true, 0, "")
		}
	}

	// ── Section 3: Top 5 offene Risiken ──────────────────────────────────────
	pdf.SetY(pdf.GetY() + 6)
	pdf.SetTextColor(30, 30, 40)
	pdf.SetFont("Helvetica", "B", 13)
	pdf.CellFormat(180, 8, "3. Top 5 offene Risiken", "", 1, "L", false, 0, "")

	if len(d.TopRisks) == 0 {
		pdf.SetFont("Helvetica", "I", 9)
		pdf.SetTextColor(34, 197, 94)
		pdf.CellFormat(180, 7, "Keine offenen Risiken — ausgezeichnet!", "0", 1, "L", false, 0, "")
	} else {
		pdf.SetFillColor(37, 99, 235)
		pdf.SetTextColor(255, 255, 255)
		pdf.SetFont("Helvetica", "B", 8)
		pdf.CellFormat(120, 6, "Risikobezeichnung", "0", 0, "L", true, 0, "")
		pdf.CellFormat(30, 6, "Risikoscore", "0", 0, "C", true, 0, "")
		pdf.CellFormat(30, 6, "Bewertung", "0", 1, "C", true, 0, "")

		sevColors := map[string][3]int{
			"critical": {220, 38, 38},
			"high":     {249, 115, 22},
			"medium":   {234, 179, 8},
			"low":      {34, 197, 94},
		}
		sevLabels := map[string]string{
			"critical": "Kritisch",
			"high":     "Hoch",
			"medium":   "Mittel",
			"low":      "Niedrig",
		}

		for i, r := range d.TopRisks {
			if pdf.GetY() > 265 {
				pdf.AddPage()
			}
			if i%2 == 0 {
				pdf.SetFillColor(255, 245, 245)
			} else {
				pdf.SetFillColor(255, 255, 255)
			}
			title := r.Title
			if len(title) > 72 {
				title = title[:69] + "..."
			}
			col := sevColors[r.Severity]
			if col == ([3]int{}) {
				col = [3]int{100, 100, 120}
			}
			label := sevLabels[r.Severity]
			if label == "" {
				label = r.Severity
			}
			pdf.SetTextColor(30, 30, 40)
			pdf.SetFont("Helvetica", "", 8)
			pdf.CellFormat(120, 6, title, "0", 0, "L", true, 0, "")
			pdf.SetFont("Helvetica", "B", 8)
			pdf.CellFormat(30, 6, fmt.Sprintf("%d", r.Score), "0", 0, "C", true, 0, "")
			pdf.SetTextColor(col[0], col[1], col[2])
			pdf.CellFormat(30, 6, label, "0", 1, "C", true, 0, "")
		}
	}

	// ── Section 4: Letzte 30 Tage ─────────────────────────────────────────────
	pdf.SetY(pdf.GetY() + 6)
	pdf.SetTextColor(30, 30, 40)
	pdf.SetFont("Helvetica", "B", 13)
	pdf.CellFormat(180, 8, "4. Aktivitäten der letzten 30 Tage", "", 1, "L", false, 0, "")

	act := d.Last30DaysActivity
	renderActivityRow := func(label, value string, positive bool) {
		if pdf.GetY() > 265 {
			pdf.AddPage()
		}
		pdf.SetFont("Helvetica", "B", 9)
		pdf.SetTextColor(100, 100, 120)
		pdf.CellFormat(130, 7, label, "0", 0, "L", false, 0, "")
		if positive {
			pdf.SetTextColor(34, 197, 94)
		} else {
			pdf.SetTextColor(30, 30, 40)
		}
		pdf.SetFont("Helvetica", "B", 9)
		pdf.CellFormat(50, 7, value, "0", 1, "R", false, 0, "")
	}
	pdf.SetY(pdf.GetY() + 2)
	renderActivityRow("Controls als 'Umgesetzt' markiert:", fmt.Sprintf("%d", act.ClosedControls), act.ClosedControls > 0)
	renderActivityRow("Neue Vorfälle erfasst:", fmt.Sprintf("%d", act.NewIncidents), false)
	renderActivityRow("Findings als 'Behoben' markiert:", fmt.Sprintf("%d", act.ResolvedFindings), act.ResolvedFindings > 0)

	var buf bytes.Buffer
	if err := pdf.Output(&buf); err != nil {
		return nil, fmt.Errorf("executive summary pdf output: %w", err)
	}
	return buf.Bytes(), nil
}
