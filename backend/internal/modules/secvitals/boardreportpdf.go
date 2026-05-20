// Copyright (c) 2026 NorvikOps. All rights reserved.
// SPDX-License-Identifier: Elastic-2.0

package secvitals

import (
	"bytes"
	"fmt"
	"math"
	"time"

	"github.com/go-pdf/fpdf"
)

// BoardReportData holds all data needed to render the management board report PDF.
type BoardReportData struct {
	OrgName         string
	Score           int
	ScorePrevious   int // 0 if unknown
	OpenRisks       int
	CriticalRisks   int
	OpenCAPAs       int
	OverdueCAPAs    int
	RecentIncidents int
	GeneratedAt     time.Time
}

// GenerateBoardReportPDF renders a 1-click management board report as PDF bytes.
// Style matches the SecReflex campaign report (blue header bar, Helvetica, fpdf).
func GenerateBoardReportPDF(data BoardReportData) ([]byte, error) {
	pdf := fpdf.New("P", "mm", "A4", "")
	pdf.SetMargins(15, 15, 15)
	pdf.SetAutoPageBreak(true, 18)
	pdf.AliasNbPages("{nb}")

	// Footer must be registered before AddPage.
	pdf.SetFooterFunc(func() {
		pdf.SetY(-12)
		pdf.SetFont("Helvetica", "I", 7)
		pdf.SetTextColor(150, 150, 160)
		pdf.CellFormat(0, 5,
			fmt.Sprintf("Vakt Comply — %s — Seite %d/{nb}", data.OrgName, pdf.PageNo()),
			"", 0, "C", false, 0, "")
	})

	pdf.AddPage()

	// ── Blue header bar ───────────────────────────────────────────────────────
	pdf.SetFillColor(37, 99, 235)
	pdf.Rect(0, 0, 210, 28, "F")
	pdf.SetTextColor(255, 255, 255)
	pdf.SetFont("Helvetica", "B", 14)
	pdf.SetXY(15, 8)
	pdf.CellFormat(180, 8, "Vakt — Management-Bericht", "", 1, "L", false, 0, "")
	pdf.SetFont("Helvetica", "", 9)
	pdf.SetXY(15, 17)
	pdf.CellFormat(180, 6, data.OrgName, "", 1, "L", false, 0, "")

	// ── Report subtitle ───────────────────────────────────────────────────────
	pdf.SetTextColor(30, 30, 40)
	pdf.SetXY(15, 33)
	pdf.SetFont("Helvetica", "B", 17)
	pdf.CellFormat(130, 10, "Board-Bericht", "", 0, "L", false, 0, "")
	pdf.SetFont("Helvetica", "", 9)
	pdf.SetTextColor(100, 100, 120)
	pdf.SetXY(15, 44)
	pdf.CellFormat(180, 6,
		fmt.Sprintf("Erstellt am %s", data.GeneratedAt.Format("02.01.2006 15:04 Uhr")),
		"", 1, "L", false, 0, "")

	// ── Executive Summary box ────────────────────────────────────────────────
	pdf.SetY(pdf.GetY() + 4)
	execSummary := buildExecutiveSummary(data)
	drawExecutiveSummaryBox(pdf, execSummary)

	// ── 1. Compliance-Score ───────────────────────────────────────────────────
	pdf.SetY(pdf.GetY() + 6)
	sectionHeader(pdf, "1. Compliance-Score")

	// Score color: green >= 80, yellow >= 60, red < 60
	scoreR, scoreG, scoreB := 220, 38, 38 // red
	if data.Score >= 80 {
		scoreR, scoreG, scoreB = 22, 163, 74 // green
	} else if data.Score >= 60 {
		scoreR, scoreG, scoreB = 202, 138, 4 // yellow/amber
	}

	scoreY := pdf.GetY() + 4

	// Donut chart — outer radius 18mm, inner hole radius 11mm, centered at (40, scoreY+18)
	const (
		cx     = 40.0
		outerR = 18.0
		innerR = 11.0
	)
	cy := scoreY + outerR

	// Gray background ring (full 360°)
	drawDonutArc(pdf, cx, cy, outerR, innerR, -90, 270, 220, 220, 230)
	// Colored arc for score portion
	endDeg := -90.0 + float64(data.Score)/100.0*360.0
	drawDonutArc(pdf, cx, cy, outerR, innerR, -90, endDeg, scoreR, scoreG, scoreB)

	// Center label
	pdf.SetFont("Helvetica", "B", 14)
	pdf.SetTextColor(30, 30, 40)
	pdf.SetXY(cx-outerR, cy-5)
	pdf.CellFormat(outerR*2, 8, fmt.Sprintf("%d%%", data.Score), "", 0, "C", false, 0, "")
	pdf.SetFont("Helvetica", "", 7)
	pdf.SetTextColor(100, 100, 120)
	pdf.SetXY(cx-outerR, cy+3)
	pdf.CellFormat(outerR*2, 5, "Score", "", 0, "C", false, 0, "")

	// Trend indicator
	if data.ScorePrevious > 0 {
		diff := data.Score - data.ScorePrevious
		trendLabel := fmt.Sprintf("Vorperiode: %d%%", data.ScorePrevious)
		if diff > 0 {
			trendLabel += fmt.Sprintf(" (+%d%%)", diff)
		} else if diff < 0 {
			trendLabel += fmt.Sprintf(" (%d%%)", diff)
		}
		pdf.SetTextColor(100, 100, 120)
		pdf.SetFont("Helvetica", "I", 9)
		pdf.SetXY(68, cy-5)
		pdf.CellFormat(120, 8, trendLabel, "", 1, "L", false, 0, "")
	}

	// Score scale legend
	pdf.SetTextColor(30, 30, 40)
	pdf.SetFont("Helvetica", "", 8)
	legendItems := []struct {
		label   string
		r, g, b int
	}{
		{"≥80%  Gut", 22, 163, 74},
		{"≥60%  Ausreichend", 202, 138, 4},
		{"<60%  Kritisch", 220, 38, 38},
	}
	ly := cy + 3
	for _, li := range legendItems {
		pdf.SetFillColor(li.r, li.g, li.b)
		pdf.Rect(68, ly+1, 3, 3, "F")
		pdf.SetTextColor(30, 30, 40)
		pdf.SetXY(73, ly-1)
		pdf.CellFormat(55, 6, li.label, "", 1, "L", false, 0, "")
		ly += 6
	}

	pdf.SetY(scoreY + outerR*2 + 6)

	// ── 2. Risiko-Übersicht ───────────────────────────────────────────────────
	sectionHeader(pdf, "2. Risiko-Übersicht")
	statBoxRow(pdf, []statBox{
		{label: "Offene Risiken", value: fmt.Sprintf("%d", data.OpenRisks), r: 55, g: 65, b: 81},
		{label: "Kritische Risiken", value: fmt.Sprintf("%d", data.CriticalRisks), r: 220, g: 38, b: 38},
	})

	// ── 3. CAPAs ─────────────────────────────────────────────────────────────
	sectionHeader(pdf, "3. Korrektur- und Vorbeugungsmaßnahmen (CAPAs)")
	statBoxRow(pdf, []statBox{
		{label: "Offene CAPAs", value: fmt.Sprintf("%d", data.OpenCAPAs), r: 59, g: 130, b: 246},
		{label: "Überfällige CAPAs", value: fmt.Sprintf("%d", data.OverdueCAPAs), r: 220, g: 38, b: 38},
	})

	// ── 4. Vorfälle ───────────────────────────────────────────────────────────
	sectionHeader(pdf, "4. Vorfälle (letzte 30 Tage)")
	statBoxRow(pdf, []statBox{
		{label: "Neue Vorfälle (30 Tage)", value: fmt.Sprintf("%d", data.RecentIncidents), r: 234, g: 88, b: 12},
	})

	// ── 5. Handlungsempfehlungen ──────────────────────────────────────────────
	sectionHeader(pdf, "5. Handlungsempfehlungen")
	drawRecommendations(pdf, data)

	// ── 6. Metadaten ──────────────────────────────────────────────────────────
	sectionHeader(pdf, "6. Bericht-Informationen")
	pdf.SetFont("Helvetica", "", 9)
	pdf.SetTextColor(30, 30, 40)
	metaRows := []struct{ k, v string }{
		{"Organisation", data.OrgName},
		{"Erstellt am", data.GeneratedAt.Format("02.01.2006 15:04:05 UTC")},
		{"Erstellt mit", "Vakt Comply — Management-Bericht"},
	}
	for _, row := range metaRows {
		pdf.SetFont("Helvetica", "B", 9)
		pdf.CellFormat(50, 6, row.k+":", "0", 0, "L", false, 0, "")
		pdf.SetFont("Helvetica", "", 9)
		pdf.CellFormat(130, 6, row.v, "0", 1, "L", false, 0, "")
	}

	var buf bytes.Buffer
	if err := pdf.Output(&buf); err != nil {
		return nil, fmt.Errorf("board report pdf output: %w", err)
	}
	return buf.Bytes(), nil
}

// sectionHeader renders a bold section title with a separator line.
func sectionHeader(pdf *fpdf.Fpdf, title string) {
	pdf.SetY(pdf.GetY() + 4)
	pdf.SetFont("Helvetica", "B", 11)
	pdf.SetTextColor(30, 30, 40)
	pdf.CellFormat(180, 8, title, "B", 1, "L", false, 0, "")
	pdf.SetY(pdf.GetY() + 2)
}

// statBox holds data for a single stat box in a row.
type statBox struct {
	label   string
	value   string
	r, g, b int
}

// drawDonutArc renders a donut-shaped arc segment by filling thin polygon slices.
// startDeg and endDeg are in degrees (0 = 3 o'clock, -90 = 12 o'clock).
func drawDonutArc(pdf *fpdf.Fpdf, cx, cy, outerR, innerR, startDeg, endDeg float64, r, g, b int) {
	if endDeg <= startDeg {
		return
	}
	const sliceDeg = 5.0 // degrees per slice — fine enough to look smooth
	pdf.SetFillColor(r, g, b)
	for deg := startDeg; deg < endDeg; deg += sliceDeg {
		a1 := deg * math.Pi / 180
		a2 := math.Min(deg+sliceDeg, endDeg) * math.Pi / 180
		pts := []fpdf.PointType{
			{X: cx + outerR*math.Cos(a1), Y: cy + outerR*math.Sin(a1)},
			{X: cx + outerR*math.Cos(a2), Y: cy + outerR*math.Sin(a2)},
			{X: cx + innerR*math.Cos(a2), Y: cy + innerR*math.Sin(a2)},
			{X: cx + innerR*math.Cos(a1), Y: cy + innerR*math.Sin(a1)},
		}
		pdf.Polygon(pts, "F")
	}
}

// buildExecutiveSummary generates the three-sentence executive summary text
// from existing BoardReportData fields — no additional DB queries required.
func buildExecutiveSummary(data BoardReportData) [3]string {
	// Sentence 1 — Compliance score + optional trend
	s1 := fmt.Sprintf("Der Compliance-Score beträgt %d%%.", data.Score)
	if data.ScorePrevious > 0 {
		diff := data.Score - data.ScorePrevious
		switch {
		case diff > 0:
			s1 += fmt.Sprintf(" Im Vergleich zur Vorperiode (%d%%) eine Verbesserung um %d Prozentpunkte.", data.ScorePrevious, diff)
		case diff < 0:
			s1 += fmt.Sprintf(" Im Vergleich zur Vorperiode (%d%%) ein Rückgang um %d Prozentpunkte.", data.ScorePrevious, -diff)
		default:
			s1 += fmt.Sprintf(" Der Score ist gegenüber der Vorperiode (%d%%) unverändert.", data.ScorePrevious)
		}
	}

	// Sentence 2 — Risk situation
	s2 := fmt.Sprintf("Es bestehen %d offene Risiken, davon %d kritisch.", data.OpenRisks, data.CriticalRisks)

	// Sentence 3 — CAPAs and incidents
	s3 := fmt.Sprintf(
		"%d CAPAs sind offen, %d davon überfällig. %d Sicherheitsvorfall(e) in den letzten 30 Tagen.",
		data.OpenCAPAs, data.OverdueCAPAs, data.RecentIncidents,
	)

	return [3]string{s1, s2, s3}
}

// drawExecutiveSummaryBox renders a light-gray rounded box containing the
// three executive-summary sentences.
func drawExecutiveSummaryBox(pdf *fpdf.Fpdf, sentences [3]string) {
	const (
		boxX     = 15.0
		boxW     = 180.0
		padX     = 6.0
		padY     = 5.0
		lineH    = 5.5
		fontSize = 8.0
	)

	// Measure total height: padY top + 3 lines + padY bottom
	boxH := padY*2 + 3*lineH

	y := pdf.GetY()
	// Gray background
	pdf.SetFillColor(245, 245, 248)
	pdf.RoundedRect(boxX, y, boxW, boxH, 2, "1234", "F")
	// Subtle border
	pdf.SetDrawColor(210, 210, 220)
	pdf.RoundedRect(boxX, y, boxW, boxH, 2, "1234", "D")

	pdf.SetFont("Helvetica", "", fontSize)
	pdf.SetTextColor(50, 50, 65)

	for i, sentence := range sentences {
		pdf.SetXY(boxX+padX, y+padY+float64(i)*lineH)
		pdf.CellFormat(boxW-padX*2, lineH, sentence, "", 1, "L", false, 0, "")
	}

	pdf.SetY(y + boxH + 2)
}

// buildRecommendations derives 3-5 concrete action items from the report data.
func buildRecommendations(data BoardReportData) []string {
	var recs []string

	if data.CriticalRisks > 0 {
		recs = append(recs, fmt.Sprintf("Kritische Risiken priorisiert behandeln (%d offen)", data.CriticalRisks))
	}
	if data.OverdueCAPAs > 0 {
		recs = append(recs, fmt.Sprintf("Überfällige CAPAs sofort bearbeiten (%d Maßnahmen)", data.OverdueCAPAs))
	}
	if data.Score < 60 {
		recs = append(recs, "Compliance-Score unter 60% — Framework-Controls priorisieren")
	} else if data.Score < 80 {
		recs = append(recs, "Compliance-Score verbessern: weitere Controls auf 'implementiert' setzen")
	}
	if data.RecentIncidents > 0 {
		recs = append(recs, fmt.Sprintf("Vorfälle der letzten 30 Tage analysieren und dokumentieren (%d Vorfall(e))", data.RecentIncidents))
	}
	// Always-present reminder
	recs = append(recs, "Board-Bericht quartalsweise aktualisieren und intern freigeben")

	return recs
}

// drawRecommendations renders a numbered list of action recommendations.
func drawRecommendations(pdf *fpdf.Fpdf, data BoardReportData) {
	recs := buildRecommendations(data)
	pdf.SetTextColor(30, 30, 40)
	for i, rec := range recs {
		y := pdf.GetY()
		// Bold number
		pdf.SetFont("Helvetica", "B", 9)
		pdf.SetXY(15, y)
		num := fmt.Sprintf("%d.", i+1)
		pdf.CellFormat(8, 6, num, "", 0, "L", false, 0, "")
		// Normal text
		pdf.SetFont("Helvetica", "", 9)
		pdf.SetXY(23, y)
		pdf.CellFormat(172, 6, rec, "", 1, "L", false, 0, "")
	}
	pdf.SetY(pdf.GetY() + 2)
}

// statBoxRow renders a row of coloured stat boxes.
func statBoxRow(pdf *fpdf.Fpdf, boxes []statBox) {
	const boxW, gap, startX = 44.0, 4.0, 15.0
	y := pdf.GetY()
	for i, b := range boxes {
		x := startX + float64(i)*(boxW+gap)
		pdf.SetFillColor(b.r, b.g, b.b)
		pdf.RoundedRect(x, y, boxW, 22, 2, "1234", "F")
		pdf.SetTextColor(255, 255, 255)
		pdf.SetFont("Helvetica", "B", 16)
		pdf.SetXY(x, y+3)
		pdf.CellFormat(boxW, 10, b.value, "", 1, "C", false, 0, "")
		pdf.SetFont("Helvetica", "", 8)
		pdf.SetXY(x, y+13)
		pdf.CellFormat(boxW, 6, b.label, "", 1, "C", false, 0, "")
	}
	pdf.SetY(y + 26)
}
