package auditreport

import (
	"bytes"
	"fmt"
	"strings"

	"github.com/go-pdf/fpdf"
)

const (
	pageW      = 210.0
	pageH      = 297.0
	marginL    = 15.0
	marginR    = 15.0
	marginT    = 15.0
	marginB    = 15.0
	contentW   = pageW - marginL - marginR
	headerH    = 28.0
)

// brand colours (Vakt indigo)
var (
	brandR, brandG, brandB       = 37, 99, 235
	lightR, lightG, lightB       = 238, 242, 255
	darkR, darkG, darkB          = 30, 30, 40
	subtleR, subtleG, subtleB    = 100, 100, 120
	altRowR, altRowG, altRowB    = 245, 247, 255
)

// Render generates the audit report PDF bytes from ReportData.
func Render(d *ReportData) ([]byte, error) {
	pdf := fpdf.New("P", "mm", "A4", "")
	pdf.SetMargins(marginL, marginT, marginR)
	pdf.SetAutoPageBreak(true, marginB+5)

	// Footer on every page
	pdf.SetFooterFunc(func() {
		pdf.SetY(-12)
		pdf.SetFont("Helvetica", "I", 7)
		pdf.SetTextColor(subtleR, subtleG, subtleB)
		pdf.CellFormat(0, 5,
			fmt.Sprintf("Vakt Comply — Audit-Bericht — %s — Seite %d/{nb}", d.OrgName, pdf.PageNo()),
			"", 0, "C", false, 0, "")
	})
	pdf.AliasNbPages("{nb}")

	// ─────────────────────────────────────────────────────────────────────────
	// PAGE 1 — Cover
	// ─────────────────────────────────────────────────────────────────────────
	pdf.AddPage()
	addPageHeader(pdf, d.OrgName)

	// Big title block
	pdf.SetY(50)
	pdf.SetFont("Helvetica", "B", 24)
	pdf.SetTextColor(darkR, darkG, darkB)
	pdf.CellFormat(contentW, 12, "Compliance Audit-Bericht", "", 1, "C", false, 0, "")

	pdf.SetFont("Helvetica", "", 12)
	pdf.SetTextColor(subtleR, subtleG, subtleB)
	pdf.CellFormat(contentW, 8, d.OrgName, "", 1, "C", false, 0, "")

	pdf.SetFont("Helvetica", "", 10)
	pdf.CellFormat(contentW, 7,
		fmt.Sprintf("Erstellt am %s", d.GeneratedAt.Format("02.01.2006 15:04 Uhr")),
		"", 1, "C", false, 0, "")

	// Overall score
	overallScore := computeOverallScore(d.Frameworks)
	pdf.Ln(6)
	pdf.SetFont("Helvetica", "B", 48)
	scoreColor := scoreToRGB(overallScore)
	pdf.SetTextColor(scoreColor[0], scoreColor[1], scoreColor[2])
	pdf.CellFormat(contentW, 20, fmt.Sprintf("%.0f%%", overallScore), "", 1, "C", false, 0, "")
	pdf.SetFont("Helvetica", "", 10)
	pdf.SetTextColor(subtleR, subtleG, subtleB)
	pdf.CellFormat(contentW, 6, "Gesamtbewertung (Durchschnitt aller Frameworks)", "", 1, "C", false, 0, "")

	// Cover KPI summary boxes
	pdf.Ln(10)
	type kpiBox struct {
		label string
		value string
		r, g, b int
	}
	kpis := []kpiBox{
		{"Frameworks", fmt.Sprintf("%d", len(d.Frameworks)), brandR, brandG, brandB},
		{"Controls gesamt", fmt.Sprintf("%d", d.ControlStats.Total), 55, 65, 81},
		{"Umgesetzt", fmt.Sprintf("%d", d.ControlStats.Implemented), 22, 163, 74},
		{"In Bearbeitung", fmt.Sprintf("%d", d.ControlStats.InProgress), 234, 179, 8},
		{"Nicht begonnen", fmt.Sprintf("%d", d.ControlStats.NotStarted), 156, 163, 175},
		{"Nachweise", fmt.Sprintf("%d", d.EvidenceCount), 124, 58, 237},
	}

	boxW := (contentW - float64(len(kpis)-1)*2) / float64(len(kpis))
	y := pdf.GetY() + 4
	x := marginL
	for _, k := range kpis {
		pdf.SetFillColor(k.r, k.g, k.b)
		pdf.RoundedRect(x, y, boxW, 18, 2, "1234", "F")
		pdf.SetTextColor(255, 255, 255)
		pdf.SetFont("Helvetica", "B", 11)
		pdf.SetXY(x, y+2)
		pdf.CellFormat(boxW, 7, k.value, "", 1, "C", false, 0, "")
		pdf.SetFont("Helvetica", "", 6.5)
		pdf.SetXY(x, y+9)
		pdf.CellFormat(boxW, 5, k.label, "", 0, "C", false, 0, "")
		x += boxW + 2
	}

	// ─────────────────────────────────────────────────────────────────────────
	// PAGE 2 — Executive Summary
	// ─────────────────────────────────────────────────────────────────────────
	pdf.AddPage()
	addPageHeader(pdf, d.OrgName)

	sectionTitle(pdf, "Zusammenfassung (Executive Summary)")

	// Framework scores table
	pdf.Ln(2)
	tableHeader(pdf, []string{"Framework", "Controls", "Umgesetzt", "In Bearb.", "Score"}, []float64{75, 28, 28, 28, 21})

	for i, fw := range d.Frameworks {
		if pdf.GetY() > 260 {
			pdf.AddPage()
			addPageHeader(pdf, d.OrgName)
			tableHeader(pdf, []string{"Framework", "Controls", "Umgesetzt", "In Bearb.", "Score"}, []float64{75, 28, 28, 28, 21})
		}
		if i%2 == 0 {
			pdf.SetFillColor(altRowR, altRowG, altRowB)
		} else {
			pdf.SetFillColor(255, 255, 255)
		}
		pdf.SetTextColor(darkR, darkG, darkB)
		pdf.SetFont("Helvetica", "", 8)
		pdf.CellFormat(75, 6, truncate(fw.Name, 42), "0", 0, "L", true, 0, "")
		pdf.CellFormat(28, 6, fmt.Sprintf("%d", fw.TotalControls), "0", 0, "C", true, 0, "")
		pdf.CellFormat(28, 6, fmt.Sprintf("%d", fw.Implemented), "0", 0, "C", true, 0, "")
		pdf.CellFormat(28, 6, fmt.Sprintf("%d", fw.InProgress), "0", 0, "C", true, 0, "")
		sc := scoreToRGB(fw.ScorePct)
		pdf.SetTextColor(sc[0], sc[1], sc[2])
		pdf.SetFont("Helvetica", "B", 8)
		pdf.CellFormat(21, 6, fmt.Sprintf("%.0f%%", fw.ScorePct), "0", 1, "C", true, 0, "")
		pdf.SetTextColor(darkR, darkG, darkB)
	}

	pdf.Ln(5)
	sectionTitle(pdf, "Wichtige Kennzahlen")
	pdf.Ln(2)

	// KPI row
	type kpiRow struct {
		label string
		value string
		note  string
	}
	openCAPACount := len(d.OpenCAPAs)
	critRisks := 0
	for _, r := range d.Risks {
		if r.Score >= 15 {
			critRisks++
		}
	}
	kpiRows := []kpiRow{
		{"Offene CAPAs", fmt.Sprintf("%d", openCAPACount), "Status: offen oder in Bearbeitung"},
		{"Kritische Risiken", fmt.Sprintf("%d", critRisks), "Score ≥ 15 (Wahrscheinlichkeit × Auswirkung)"},
		{"Offene Vorfälle", fmt.Sprintf("%d", len(d.OpenIncidents)), "Status: offen oder in Untersuchung"},
		{"Richtlinien gesamt", fmt.Sprintf("%d", len(d.ActivePolicies)), "Alle Richtlinien (alle Status)"},
		{"Aktive Richtlinien", fmt.Sprintf("%d", countActive(d.ActivePolicies)), "Status: aktiv / freigegeben"},
		{"Nachweise gesamt", fmt.Sprintf("%d", d.EvidenceCount), "Alle Nachweisdatensätze im System"},
	}

	for i, row := range kpiRows {
		if i%2 == 0 {
			pdf.SetFillColor(altRowR, altRowG, altRowB)
		} else {
			pdf.SetFillColor(255, 255, 255)
		}
		pdf.SetTextColor(darkR, darkG, darkB)
		pdf.SetFont("Helvetica", "B", 8)
		pdf.CellFormat(55, 6, row.label, "0", 0, "L", true, 0, "")
		pdf.SetFont("Helvetica", "", 8)
		pdf.SetTextColor(brandR, brandG, brandB)
		pdf.CellFormat(22, 6, row.value, "0", 0, "C", true, 0, "")
		pdf.SetTextColor(subtleR, subtleG, subtleB)
		pdf.CellFormat(103, 6, row.note, "0", 1, "L", true, 0, "")
	}

	// ─────────────────────────────────────────────────────────────────────────
	// PAGES 3+ — Frameworks detail
	// ─────────────────────────────────────────────────────────────────────────
	for _, fw := range d.Frameworks {
		pdf.AddPage()
		addPageHeader(pdf, d.OrgName)

		sectionTitle(pdf, fmt.Sprintf("Framework: %s", fw.Name))
		pdf.Ln(1)

		// Progress bar text
		pdf.SetFont("Helvetica", "", 9)
		pdf.SetTextColor(subtleR, subtleG, subtleB)
		bar := progressBar(fw.ScorePct, 20)
		sc := scoreToRGB(fw.ScorePct)
		pdf.SetTextColor(sc[0], sc[1], sc[2])
		pdf.SetFont("Helvetica", "B", 9)
		pdf.CellFormat(contentW, 6, fmt.Sprintf("%s  %.0f%%", bar, fw.ScorePct), "", 1, "L", false, 0, "")

		pdf.SetFont("Helvetica", "", 8)
		pdf.SetTextColor(subtleR, subtleG, subtleB)
		pdf.CellFormat(contentW, 5,
			fmt.Sprintf("%d Controls — %d umgesetzt, %d in Bearbeitung, %d nicht begonnen",
				fw.TotalControls, fw.Implemented, fw.InProgress, fw.NotStarted),
			"", 1, "L", false, 0, "")
		pdf.Ln(3)

		for _, domain := range fw.Domains {
			if pdf.GetY() > 255 {
				pdf.AddPage()
				addPageHeader(pdf, d.OrgName)
			}

			// Domain heading
			pdf.SetFillColor(lightR, lightG, lightB)
			pdf.SetTextColor(brandR, brandG, brandB)
			pdf.SetFont("Helvetica", "B", 8)
			pdf.CellFormat(contentW, 6, "  "+domain.Name, "0", 1, "L", true, 0, "")
			pdf.Ln(1)

			for _, ctrl := range domain.Controls {
				if pdf.GetY() > 268 {
					pdf.AddPage()
					addPageHeader(pdf, d.OrgName)
				}
				icon := statusIcon(ctrl.Status)
				iconColor := statusColor(ctrl.Status)

				pdf.SetTextColor(iconColor[0], iconColor[1], iconColor[2])
				pdf.SetFont("Helvetica", "B", 7.5)
				pdf.CellFormat(6, 5, icon, "0", 0, "C", false, 0, "")

				pdf.SetTextColor(darkR, darkG, darkB)
				pdf.SetFont("Helvetica", "", 7.5)
				ctrlLabel := fmt.Sprintf("%s  %s", ctrl.ControlID, truncate(ctrl.Title, 55))
				pdf.CellFormat(155, 5, ctrlLabel, "0", 0, "L", false, 0, "")

				pdf.SetTextColor(subtleR, subtleG, subtleB)
				pdf.SetFont("Helvetica", "I", 7)
				evText := ""
				if ctrl.EvidenceCount > 0 {
					evText = fmt.Sprintf("(%d Nachw.)", ctrl.EvidenceCount)
				}
				pdf.CellFormat(19, 5, evText, "0", 1, "R", false, 0, "")
			}
			pdf.Ln(2)
		}
	}

	// ─────────────────────────────────────────────────────────────────────────
	// Risks section
	// ─────────────────────────────────────────────────────────────────────────
	if len(d.Risks) > 0 {
		pdf.AddPage()
		addPageHeader(pdf, d.OrgName)
		sectionTitle(pdf, fmt.Sprintf("Risikoregister (Top %d)", len(d.Risks)))
		pdf.Ln(2)

		tableHeader(pdf, []string{"Titel", "Wahrsch.", "Auswirk.", "Score", "Status"}, []float64{88, 22, 22, 18, 30})

		for i, r := range d.Risks {
			if pdf.GetY() > 265 {
				pdf.AddPage()
				addPageHeader(pdf, d.OrgName)
				tableHeader(pdf, []string{"Titel", "Wahrsch.", "Auswirk.", "Score", "Status"}, []float64{88, 22, 22, 18, 30})
			}
			if i%2 == 0 {
				pdf.SetFillColor(altRowR, altRowG, altRowB)
			} else {
				pdf.SetFillColor(255, 255, 255)
			}
			pdf.SetTextColor(darkR, darkG, darkB)
			pdf.SetFont("Helvetica", "", 8)
			pdf.CellFormat(88, 6, truncate(r.Title, 50), "0", 0, "L", true, 0, "")
			pdf.CellFormat(22, 6, fmt.Sprintf("%d/5", r.Likelihood), "0", 0, "C", true, 0, "")
			pdf.CellFormat(22, 6, fmt.Sprintf("%d/5", r.Impact), "0", 0, "C", true, 0, "")
			sc := riskScoreColor(r.Score)
			pdf.SetTextColor(sc[0], sc[1], sc[2])
			pdf.SetFont("Helvetica", "B", 8)
			pdf.CellFormat(18, 6, fmt.Sprintf("%d", r.Score), "0", 0, "C", true, 0, "")
			pdf.SetTextColor(darkR, darkG, darkB)
			pdf.SetFont("Helvetica", "", 8)
			pdf.CellFormat(30, 6, statusLabel(r.Status), "0", 1, "C", true, 0, "")
		}
	}

	// ─────────────────────────────────────────────────────────────────────────
	// Incidents section
	// ─────────────────────────────────────────────────────────────────────────
	if len(d.OpenIncidents) > 0 {
		pdf.AddPage()
		addPageHeader(pdf, d.OrgName)
		sectionTitle(pdf, fmt.Sprintf("Offene Vorfälle (%d)", len(d.OpenIncidents)))
		pdf.Ln(2)

		tableHeader(pdf, []string{"Titel", "Schweregrad", "Status", "Datum"}, []float64{88, 30, 30, 32})
		for i, inc := range d.OpenIncidents {
			if pdf.GetY() > 265 {
				pdf.AddPage()
				addPageHeader(pdf, d.OrgName)
				tableHeader(pdf, []string{"Titel", "Schweregrad", "Status", "Datum"}, []float64{88, 30, 30, 32})
			}
			if i%2 == 0 {
				pdf.SetFillColor(altRowR, altRowG, altRowB)
			} else {
				pdf.SetFillColor(255, 255, 255)
			}
			pdf.SetTextColor(darkR, darkG, darkB)
			pdf.SetFont("Helvetica", "", 8)
			pdf.CellFormat(88, 6, truncate(inc.Title, 50), "0", 0, "L", true, 0, "")
			sev := severityColor(inc.Severity)
			pdf.SetTextColor(sev[0], sev[1], sev[2])
			pdf.SetFont("Helvetica", "B", 8)
			pdf.CellFormat(30, 6, severityLabel(inc.Severity), "0", 0, "C", true, 0, "")
			pdf.SetTextColor(darkR, darkG, darkB)
			pdf.SetFont("Helvetica", "", 8)
			pdf.CellFormat(30, 6, incidentStatusLabel(inc.Status), "0", 0, "C", true, 0, "")
			pdf.CellFormat(32, 6, inc.CreatedAt.Format("02.01.2006"), "0", 1, "L", true, 0, "")
		}
	}

	// ─────────────────────────────────────────────────────────────────────────
	// Policies section
	// ─────────────────────────────────────────────────────────────────────────
	if len(d.ActivePolicies) > 0 {
		pdf.AddPage()
		addPageHeader(pdf, d.OrgName)
		sectionTitle(pdf, fmt.Sprintf("Richtlinien (%d gesamt)", len(d.ActivePolicies)))
		pdf.Ln(2)

		tableHeader(pdf, []string{"Titel", "Version", "Status", "Letzte Änderung"}, []float64{90, 25, 30, 35})
		for i, p := range d.ActivePolicies {
			if pdf.GetY() > 265 {
				pdf.AddPage()
				addPageHeader(pdf, d.OrgName)
				tableHeader(pdf, []string{"Titel", "Version", "Status", "Letzte Änderung"}, []float64{90, 25, 30, 35})
			}
			if i%2 == 0 {
				pdf.SetFillColor(altRowR, altRowG, altRowB)
			} else {
				pdf.SetFillColor(255, 255, 255)
			}
			pdf.SetTextColor(darkR, darkG, darkB)
			pdf.SetFont("Helvetica", "", 8)
			pdf.CellFormat(90, 6, truncate(p.Title, 52), "0", 0, "L", true, 0, "")
			pdf.CellFormat(25, 6, p.Version, "0", 0, "C", true, 0, "")
			psc := policyStatusColor(p.Status)
			pdf.SetTextColor(psc[0], psc[1], psc[2])
			pdf.SetFont("Helvetica", "B", 8)
			pdf.CellFormat(30, 6, policyStatusLabel(p.Status), "0", 0, "C", true, 0, "")
			pdf.SetTextColor(darkR, darkG, darkB)
			pdf.SetFont("Helvetica", "", 8)
			pdf.CellFormat(35, 6, p.UpdatedAt.Format("02.01.2006"), "0", 1, "C", true, 0, "")
		}
	}

	// ─────────────────────────────────────────────────────────────────────────
	// CAPAs section
	// ─────────────────────────────────────────────────────────────────────────
	if len(d.OpenCAPAs) > 0 {
		pdf.AddPage()
		addPageHeader(pdf, d.OrgName)
		sectionTitle(pdf, fmt.Sprintf("Offene Korrekturmaßnahmen / CAPAs (%d)", len(d.OpenCAPAs)))
		pdf.Ln(2)

		tableHeader(pdf, []string{"Titel", "Quelle", "Status", "Fällig"}, []float64{90, 28, 35, 27})
		for i, ca := range d.OpenCAPAs {
			if pdf.GetY() > 265 {
				pdf.AddPage()
				addPageHeader(pdf, d.OrgName)
				tableHeader(pdf, []string{"Titel", "Quelle", "Status", "Fällig"}, []float64{90, 28, 35, 27})
			}
			if i%2 == 0 {
				pdf.SetFillColor(altRowR, altRowG, altRowB)
			} else {
				pdf.SetFillColor(255, 255, 255)
			}
			pdf.SetTextColor(darkR, darkG, darkB)
			pdf.SetFont("Helvetica", "", 8)
			pdf.CellFormat(90, 6, truncate(ca.Title, 52), "0", 0, "L", true, 0, "")
			pdf.CellFormat(28, 6, capaSourceLabel(ca.SourceType), "0", 0, "C", true, 0, "")
			pdf.CellFormat(35, 6, capaStatusLabel(ca.Status), "0", 0, "C", true, 0, "")
			dueStr := "–"
			if ca.DueDate != nil {
				dueStr = ca.DueDate.Format("02.01.2006")
			}
			pdf.CellFormat(27, 6, dueStr, "0", 1, "C", true, 0, "")
		}
	}

	var buf bytes.Buffer
	if err := pdf.Output(&buf); err != nil {
		return nil, fmt.Errorf("pdf output: %w", err)
	}
	return buf.Bytes(), nil
}

// ─── helper functions ─────────────────────────────────────────────────────────

func addPageHeader(pdf *fpdf.Fpdf, orgName string) {
	pdf.SetFillColor(brandR, brandG, brandB)
	pdf.Rect(0, 0, pageW, headerH, "F")
	pdf.SetTextColor(255, 255, 255)
	pdf.SetFont("Helvetica", "B", 12)
	pdf.SetXY(marginL, 8)
	pdf.CellFormat(contentW, 7, "Vakt Comply — Audit-Bericht", "", 1, "L", false, 0, "")
	pdf.SetFont("Helvetica", "", 8)
	pdf.SetXY(marginL, 17)
	pdf.CellFormat(contentW, 6, orgName, "", 1, "L", false, 0, "")
	pdf.SetY(marginT + headerH - 14)
}

func sectionTitle(pdf *fpdf.Fpdf, title string) {
	pdf.SetFont("Helvetica", "B", 11)
	pdf.SetTextColor(darkR, darkG, darkB)
	pdf.SetFillColor(lightR, lightG, lightB)
	pdf.CellFormat(contentW, 8, "  "+title, "0", 1, "L", true, 0, "")
	pdf.Ln(1)
}

func tableHeader(pdf *fpdf.Fpdf, cols []string, widths []float64) {
	pdf.SetFillColor(brandR, brandG, brandB)
	pdf.SetTextColor(255, 255, 255)
	pdf.SetFont("Helvetica", "B", 8)
	for i, col := range cols {
		pdf.CellFormat(widths[i], 7, col, "0", 0, "C", true, 0, "")
	}
	pdf.Ln(7)
}

func progressBar(pct float64, total int) string {
	filled := int(pct / 100 * float64(total))
	if filled > total {
		filled = total
	}
	return strings.Repeat("█", filled) + strings.Repeat("░", total-filled)
}

func truncate(s string, n int) string {
	runes := []rune(s)
	if len(runes) <= n {
		return s
	}
	return string(runes[:n-3]) + "..."
}

func computeOverallScore(fws []FrameworkSection) float64 {
	if len(fws) == 0 {
		return 0
	}
	var sum float64
	for _, fw := range fws {
		sum += fw.ScorePct
	}
	return sum / float64(len(fws))
}

func countActive(policies []PolicyRow) int {
	n := 0
	for _, p := range policies {
		if p.Status == "active" {
			n++
		}
	}
	return n
}

func scoreToRGB(pct float64) [3]int {
	switch {
	case pct >= 80:
		return [3]int{22, 163, 74} // green
	case pct >= 50:
		return [3]int{234, 179, 8} // yellow
	default:
		return [3]int{220, 38, 38} // red
	}
}

func riskScoreColor(score int) [3]int {
	switch {
	case score >= 15:
		return [3]int{220, 38, 38}
	case score >= 10:
		return [3]int{234, 88, 12}
	case score >= 5:
		return [3]int{234, 179, 8}
	default:
		return [3]int{59, 130, 246}
	}
}

func severityColor(sev string) [3]int {
	switch sev {
	case "critical":
		return [3]int{220, 38, 38}
	case "high":
		return [3]int{234, 88, 12}
	case "medium":
		return [3]int{234, 179, 8}
	default:
		return [3]int{59, 130, 246}
	}
}

func statusColor(status string) [3]int {
	switch status {
	case "implemented":
		return [3]int{22, 163, 74}
	case "in_progress":
		return [3]int{234, 179, 8}
	default:
		return [3]int{156, 163, 175}
	}
}

func policyStatusColor(status string) [3]int {
	switch status {
	case "active":
		return [3]int{22, 163, 74}
	case "draft":
		return [3]int{234, 179, 8}
	default:
		return [3]int{156, 163, 175}
	}
}

func statusIcon(status string) string {
	switch status {
	case "implemented":
		return "+"
	case "in_progress":
		return "~"
	default:
		return "-"
	}
}

// Label helpers

func statusLabel(s string) string {
	m := map[string]string{
		"open":      "Offen",
		"mitigated": "Behandelt",
		"accepted":  "Akzeptiert",
		"closed":    "Geschlossen",
	}
	if v, ok := m[s]; ok {
		return v
	}
	return s
}

func severityLabel(s string) string {
	m := map[string]string{
		"critical": "Kritisch",
		"high":     "Hoch",
		"medium":   "Mittel",
		"low":      "Niedrig",
	}
	if v, ok := m[s]; ok {
		return v
	}
	return s
}

func incidentStatusLabel(s string) string {
	m := map[string]string{
		"open":          "Offen",
		"investigating": "In Untersuchung",
		"resolved":      "Behoben",
		"closed":        "Geschlossen",
	}
	if v, ok := m[s]; ok {
		return v
	}
	return s
}

func policyStatusLabel(s string) string {
	m := map[string]string{
		"draft":    "Entwurf",
		"active":   "Aktiv",
		"archived": "Archiviert",
	}
	if v, ok := m[s]; ok {
		return v
	}
	return s
}

func capaSourceLabel(s string) string {
	m := map[string]string{
		"audit":    "Audit",
		"incident": "Vorfall",
		"risk":     "Risiko",
		"manual":   "Manuell",
	}
	if v, ok := m[s]; ok {
		return v
	}
	return s
}

func capaStatusLabel(s string) string {
	m := map[string]string{
		"open":        "Offen",
		"in_progress": "In Bearbeitung",
		"implemented": "Umgesetzt",
		"verified":    "Verifiziert",
		"closed":      "Geschlossen",
	}
	if v, ok := m[s]; ok {
		return v
	}
	return s
}
