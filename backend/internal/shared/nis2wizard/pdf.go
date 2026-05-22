package nis2wizard

// Sprint 28 / S28-2 (S19-8): NIS2 Branded PDF-Export.
//
// RenderAssessmentPDF generiert ein PDF aus einem NIS2-Assessment-Ergebnis.
// Analog zu auditreport/render.go aber spezialisiert auf den Wizard-Output.

import (
	"bytes"
	"fmt"
	"strings"
	"time"

	"github.com/go-pdf/fpdf"
)

const (
	pdfPageW    = 210.0
	pdfPageH    = 297.0 //nolint:unused
	pdfMarginL  = 15.0
	pdfMarginR  = 15.0
	pdfMarginT  = 15.0
	pdfMarginB  = 15.0
	pdfContentW = pdfPageW - pdfMarginL - pdfMarginR
	pdfHeaderH  = 28.0
)

// brand colours (Vakt indigo) — same as auditreport/render.go
var (
	pdfBrandR, pdfBrandG, pdfBrandB    = 37, 99, 235
	pdfLightR, pdfLightG, pdfLightB    = 238, 242, 255
	pdfDarkR, pdfDarkG, pdfDarkB       = 30, 30, 40
	pdfSubtleR, pdfSubtleG, pdfSubtleB = 100, 100, 120
	pdfAltRowR, pdfAltRowG, pdfAltRowB = 245, 247, 255
)

// RenderAssessmentPDF generiert ein PDF aus einem NIS2-Assessment-Ergebnis.
// orgName ist der Anzeigename der Organisation (für Header + Cover).
// run muss Score und ScoreByArea gesetzt haben.
func RenderAssessmentPDF(orgName string, run *Run) ([]byte, error) {
	if run == nil {
		return nil, fmt.Errorf("run must not be nil")
	}

	now := time.Now().UTC()
	pdf := fpdf.New("P", "mm", "A4", "")
	pdf.SetMargins(pdfMarginL, pdfMarginT, pdfMarginR)
	pdf.SetAutoPageBreak(true, pdfMarginB+5)

	// Footer on every page.
	pdf.SetFooterFunc(func() {
		pdf.SetY(-12)
		pdf.SetFont("Helvetica", "I", 7)
		pdf.SetTextColor(pdfSubtleR, pdfSubtleG, pdfSubtleB)
		pdf.CellFormat(0, 5,
			fmt.Sprintf("Erstellt mit Vakt · vakt.io — NIS2-Assessment — %s — Seite %d/{nb}", orgName, pdf.PageNo()),
			"", 0, "C", false, 0, "")
	})
	pdf.AliasNbPages("{nb}")

	// ─────────────────────────────────────────────────────────────────────────
	// PAGE 1 — Cover
	// ─────────────────────────────────────────────────────────────────────────
	pdf.AddPage()
	nisAddPageHeader(pdf, orgName)

	pdf.SetY(50)
	pdf.SetFont("Helvetica", "B", 24)
	pdf.SetTextColor(pdfDarkR, pdfDarkG, pdfDarkB)
	pdf.CellFormat(pdfContentW, 12, "NIS2-Self-Assessment", "", 1, "C", false, 0, "")

	pdf.SetFont("Helvetica", "", 12)
	pdf.SetTextColor(pdfSubtleR, pdfSubtleG, pdfSubtleB)
	pdf.CellFormat(pdfContentW, 8, orgName, "", 1, "C", false, 0, "")

	pdf.SetFont("Helvetica", "", 10)
	pdf.CellFormat(pdfContentW, 7,
		fmt.Sprintf("Erstellt am %s", now.Format("02.01.2006 15:04 Uhr")),
		"", 1, "C", false, 0, "")

	// Gesamt-Score.
	score := 0
	if run.Score != nil {
		score = *run.Score
	}

	pdf.Ln(6)
	pdf.SetFont("Helvetica", "B", 48)
	sc := nisScoreToRGB(score)
	pdf.SetTextColor(sc[0], sc[1], sc[2])
	pdf.CellFormat(pdfContentW, 20, fmt.Sprintf("%d/100", score), "", 1, "C", false, 0, "")
	pdf.SetFont("Helvetica", "", 10)
	pdf.SetTextColor(pdfSubtleR, pdfSubtleG, pdfSubtleB)
	pdf.CellFormat(pdfContentW, 6, "NIS2 Compliance Score (0 = nicht implementiert, 100 = vollständig)", "", 1, "C", false, 0, "")

	// Score-Label.
	pdf.Ln(4)
	pdf.SetFont("Helvetica", "B", 12)
	labelColor := sc
	pdf.SetTextColor(labelColor[0], labelColor[1], labelColor[2])
	pdf.CellFormat(pdfContentW, 8, nisScoreLabel(score), "", 1, "C", false, 0, "")

	// ─────────────────────────────────────────────────────────────────────────
	// PAGE 2 — Bereichs-Scores
	// ─────────────────────────────────────────────────────────────────────────
	pdf.AddPage()
	nisAddPageHeader(pdf, orgName)

	nisSectionTitle(pdf, "NIS2 Compliance Score: Ergebnis nach Bereichen")
	pdf.Ln(2)

	// Table header
	nisTableHeader(pdf, []string{"Bereich (NIS2 Art. 21)", "Score", "Bewertung"}, []float64{110, 25, 45})

	for i, area := range AllAreas {
		areaScore := 0
		if run.ScoreByArea != nil {
			areaScore = run.ScoreByArea[area]
		}
		if i%2 == 0 {
			pdf.SetFillColor(pdfAltRowR, pdfAltRowG, pdfAltRowB)
		} else {
			pdf.SetFillColor(255, 255, 255)
		}
		asc := nisScoreToRGB(areaScore)
		pdf.SetTextColor(pdfDarkR, pdfDarkG, pdfDarkB)
		pdf.SetFont("Helvetica", "", 8)
		pdf.CellFormat(110, 6, AreaTitle(area), "0", 0, "L", true, 0, "")
		pdf.SetTextColor(asc[0], asc[1], asc[2])
		pdf.SetFont("Helvetica", "B", 8)
		pdf.CellFormat(25, 6, fmt.Sprintf("%d%%", areaScore), "0", 0, "C", true, 0, "")
		pdf.SetFont("Helvetica", "", 8)
		pdf.SetTextColor(asc[0], asc[1], asc[2])
		pdf.CellFormat(45, 6, nisScoreLabel(areaScore), "0", 1, "C", true, 0, "")
	}

	// ─────────────────────────────────────────────────────────────────────────
	// Top-3-Lücken
	// ─────────────────────────────────────────────────────────────────────────
	pdf.Ln(6)
	nisSectionTitle(pdf, "Top-3 Handlungsfelder (Bereiche mit größtem Optimierungsbedarf)")
	pdf.Ln(2)

	gaps := run.TopGaps(3)
	if len(gaps) == 0 {
		pdf.SetFont("Helvetica", "I", 9)
		pdf.SetTextColor(pdfSubtleR, pdfSubtleG, pdfSubtleB)
		pdf.CellFormat(pdfContentW, 6, "Keine Lücken identifiziert.", "", 1, "L", false, 0, "")
	} else {
		for i, gap := range gaps {
			gsc := nisScoreToRGB(gap.Score)
			pdf.SetFillColor(pdfAltRowR, pdfAltRowG, pdfAltRowB)
			if i%2 != 0 {
				pdf.SetFillColor(255, 255, 255)
			}
			pdf.SetTextColor(pdfDarkR, pdfDarkG, pdfDarkB)
			pdf.SetFont("Helvetica", "B", 9)
			bullet := fmt.Sprintf("%d.", i+1)
			pdf.CellFormat(8, 7, bullet, "0", 0, "C", true, 0, "")
			pdf.SetFont("Helvetica", "", 9)
			pdf.CellFormat(127, 7, gap.AreaTitle, "0", 0, "L", true, 0, "")
			pdf.SetTextColor(gsc[0], gsc[1], gsc[2])
			pdf.SetFont("Helvetica", "B", 9)
			pdf.CellFormat(25, 7, fmt.Sprintf("Score: %d%%", gap.Score), "0", 0, "C", true, 0, "")
			pdf.SetFont("Helvetica", "", 9)
			pdf.CellFormat(20, 7, nisScoreLabel(gap.Score), "0", 1, "C", true, 0, "")
		}
	}

	// ─────────────────────────────────────────────────────────────────────────
	// PAGE 3 — Antworten im Detail
	// ─────────────────────────────────────────────────────────────────────────
	pdf.AddPage()
	nisAddPageHeader(pdf, orgName)
	nisSectionTitle(pdf, "Detaillierte Antworten")
	pdf.Ln(2)

	currentArea := Area("")
	for _, q := range Questions {
		ans, hasAns := run.Answers[q.ID]

		// Bereichs-Überschrift wenn neuer Bereich beginnt.
		if q.Area != currentArea {
			currentArea = q.Area
			if pdf.GetY() > 255 {
				pdf.AddPage()
				nisAddPageHeader(pdf, orgName)
			}
			pdf.Ln(2)
			pdf.SetFillColor(pdfLightR, pdfLightG, pdfLightB)
			pdf.SetTextColor(pdfBrandR, pdfBrandG, pdfBrandB)
			pdf.SetFont("Helvetica", "B", 9)
			pdf.CellFormat(pdfContentW, 6, "  "+AreaTitle(currentArea), "0", 1, "L", true, 0, "")
			pdf.Ln(1)
		}

		if pdf.GetY() > 268 {
			pdf.AddPage()
			nisAddPageHeader(pdf, orgName)
		}

		valueStr := "–"
		commentStr := ""
		if hasAns {
			valueStr = nisValueLabel(ans.Value)
			commentStr = ans.Comment
		}

		vsc := nisValueColor(ans.Value, hasAns)
		pdf.SetTextColor(vsc[0], vsc[1], vsc[2])
		pdf.SetFont("Helvetica", "B", 7.5)
		pdf.CellFormat(6, 5, nisValueIcon(ans.Value, hasAns), "0", 0, "C", false, 0, "")
		pdf.SetTextColor(pdfDarkR, pdfDarkG, pdfDarkB)
		pdf.SetFont("Helvetica", "", 7.5)
		pdf.CellFormat(130, 5, nissTruncate(q.Title, 75), "0", 0, "L", false, 0, "")
		pdf.SetTextColor(vsc[0], vsc[1], vsc[2])
		pdf.SetFont("Helvetica", "I", 7)
		pdf.CellFormat(44, 5, valueStr, "0", 1, "R", false, 0, "")

		if commentStr != "" {
			pdf.SetTextColor(pdfSubtleR, pdfSubtleG, pdfSubtleB)
			pdf.SetFont("Helvetica", "I", 6.5)
			pdf.SetX(pdfMarginL + 6)
			pdf.CellFormat(pdfContentW-6, 4, "  Kommentar: "+nissTruncate(commentStr, 90), "0", 1, "L", false, 0, "")
		}
	}

	var buf bytes.Buffer
	if err := pdf.Output(&buf); err != nil {
		return nil, fmt.Errorf("pdf output: %w", err)
	}
	return buf.Bytes(), nil
}

// ─── helper functions ─────────────────────────────────────────────────────────

func nisAddPageHeader(pdf *fpdf.Fpdf, orgName string) {
	pdf.SetFillColor(pdfBrandR, pdfBrandG, pdfBrandB)
	pdf.Rect(0, 0, pdfPageW, pdfHeaderH, "F")
	pdf.SetTextColor(255, 255, 255)
	pdf.SetFont("Helvetica", "B", 12)
	pdf.SetXY(pdfMarginL, 8)
	pdf.CellFormat(pdfContentW, 7, "Vakt — NIS2-Self-Assessment", "", 1, "L", false, 0, "")
	pdf.SetFont("Helvetica", "", 8)
	pdf.SetXY(pdfMarginL, 17)
	pdf.CellFormat(pdfContentW, 6, orgName, "", 1, "L", false, 0, "")
	pdf.SetY(pdfMarginT + pdfHeaderH - 14)
}

func nisSectionTitle(pdf *fpdf.Fpdf, title string) {
	pdf.SetFont("Helvetica", "B", 11)
	pdf.SetTextColor(pdfDarkR, pdfDarkG, pdfDarkB)
	pdf.SetFillColor(pdfLightR, pdfLightG, pdfLightB)
	pdf.CellFormat(pdfContentW, 8, "  "+title, "0", 1, "L", true, 0, "")
	pdf.Ln(1)
}

func nisTableHeader(pdf *fpdf.Fpdf, cols []string, widths []float64) {
	pdf.SetFillColor(pdfBrandR, pdfBrandG, pdfBrandB)
	pdf.SetTextColor(255, 255, 255)
	pdf.SetFont("Helvetica", "B", 8)
	for i, col := range cols {
		pdf.CellFormat(widths[i], 7, col, "0", 0, "C", true, 0, "")
	}
	pdf.Ln(7)
}

func nisScoreToRGB(score int) [3]int {
	switch {
	case score >= 70:
		return [3]int{22, 163, 74} // green
	case score >= 40:
		return [3]int{234, 179, 8} // yellow
	default:
		return [3]int{220, 38, 38} // red
	}
}

func nisScoreLabel(score int) string {
	switch {
	case score >= 70:
		return "Gut"
	case score >= 40:
		return "Verbesserungsbedarf"
	default:
		return "Kritisch"
	}
}

func nisValueLabel(v int) string {
	labels := []string{
		"Nicht implementiert",
		"In Planung",
		"Teilweise umgesetzt",
		"Weitgehend umgesetzt",
		"Vollständig + getestet",
	}
	if v >= 0 && v < len(labels) {
		return labels[v]
	}
	return "–"
}

func nisValueIcon(v int, hasAns bool) string {
	if !hasAns {
		return "–"
	}
	switch {
	case v >= 3:
		return "+"
	case v >= 1:
		return "~"
	default:
		return "-"
	}
}

func nisValueColor(v int, hasAns bool) [3]int {
	if !hasAns {
		return [3]int{156, 163, 175}
	}
	switch {
	case v >= 3:
		return [3]int{22, 163, 74}
	case v >= 1:
		return [3]int{234, 179, 8}
	default:
		return [3]int{220, 38, 38}
	}
}

func nissTruncate(s string, n int) string {
	runes := []rune(strings.TrimSpace(s))
	if len(runes) <= n {
		return string(runes)
	}
	return string(runes[:n-3]) + "..."
}
