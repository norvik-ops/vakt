package vaktaware

import (
	"bytes"
	"fmt"
	"time"

	"github.com/go-pdf/fpdf"
)

// GenerateTrainingCertificatePDF renders a training completion certificate as PDF bytes.
func GenerateTrainingCertificatePDF(moduleName, userEmail string, score *int, passed bool, completedAt time.Time, orgName string) ([]byte, error) {
	pdf := fpdf.New("P", "mm", "A4", "")
	pdf.SetMargins(15, 15, 15)
	pdf.SetAutoPageBreak(true, 15)

	// Register footer before adding page so {nb} alias is resolved correctly.
	pdf.AliasNbPages("{nb}")
	pdf.SetFooterFunc(func() {
		pdf.SetY(-12)
		pdf.SetFont("Helvetica", "I", 7)
		pdf.SetTextColor(150, 150, 160)
		pdf.CellFormat(0, 5,
			fmt.Sprintf("Vakt SecReflex — %s — Seite %d/{nb}", orgName, pdf.PageNo()),
			"", 0, "C", false, 0, "")
	})

	pdf.AddPage()

	// ── Header bar ────────────────────────────────────────────────────────────
	pdf.SetFillColor(37, 99, 235)
	pdf.Rect(0, 0, 210, 28, "F")
	pdf.SetTextColor(255, 255, 255)
	pdf.SetFont("Helvetica", "B", 14)
	pdf.SetXY(15, 8)
	pdf.CellFormat(180, 8, "Vakt — Security Awareness Training", "", 1, "L", false, 0, "")
	pdf.SetFont("Helvetica", "", 9)
	pdf.SetXY(15, 17)
	pdf.CellFormat(180, 6, orgName, "", 1, "L", false, 0, "")

	// ── Title ─────────────────────────────────────────────────────────────────
	pdf.SetTextColor(30, 30, 40)
	pdf.SetXY(15, 38)
	pdf.SetFont("Helvetica", "B", 20)
	pdf.CellFormat(180, 12, "Schulungsabschluss-Zertifikat", "", 1, "C", false, 0, "")

	// ── Separator line ────────────────────────────────────────────────────────
	pdf.SetDrawColor(37, 99, 235)
	pdf.SetLineWidth(0.6)
	lineY := pdf.GetY() + 4
	pdf.Line(15, lineY, 195, lineY)
	pdf.SetY(lineY + 8)

	// ── Certificate details ───────────────────────────────────────────────────
	type row struct{ label, value string }
	rows := []row{
		{"Schulungsmodul", moduleName},
		{"Teilnehmer", userEmail},
		{"Abschlussdatum", completedAt.Format("02.01.2006")},
	}
	if score != nil {
		rows = append(rows, row{"Punktzahl", fmt.Sprintf("%d / 100", *score)})
	}

	pdf.SetFont("Helvetica", "", 11)
	for _, r := range rows {
		pdf.SetTextColor(100, 100, 120)
		pdf.SetFont("Helvetica", "B", 10)
		pdf.CellFormat(55, 8, r.label+":", "0", 0, "L", false, 0, "")
		pdf.SetTextColor(30, 30, 40)
		pdf.SetFont("Helvetica", "", 10)
		pdf.CellFormat(125, 8, r.value, "0", 1, "L", false, 0, "")
	}

	// ── Status badge ──────────────────────────────────────────────────────────
	pdf.SetY(pdf.GetY() + 8)
	badgeY := pdf.GetY()
	badgeW := 60.0
	badgeX := (210.0 - badgeW) / 2

	if passed {
		pdf.SetFillColor(22, 163, 74) // green-600
		pdf.RoundedRect(badgeX, badgeY, badgeW, 14, 3, "1234", "F")
		pdf.SetTextColor(255, 255, 255)
		pdf.SetFont("Helvetica", "B", 12)
		pdf.SetXY(badgeX, badgeY+2)
		pdf.CellFormat(badgeW, 10, "Bestanden", "", 1, "C", false, 0, "")
	} else {
		pdf.SetFillColor(220, 38, 38) // red-600
		pdf.RoundedRect(badgeX, badgeY, badgeW, 14, 3, "1234", "F")
		pdf.SetTextColor(255, 255, 255)
		pdf.SetFont("Helvetica", "B", 12)
		pdf.SetXY(badgeX, badgeY+2)
		pdf.CellFormat(badgeW, 10, "Nicht bestanden", "", 1, "C", false, 0, "")
	}

	// ── Footer note ───────────────────────────────────────────────────────────
	pdf.SetY(pdf.GetY() + 16)
	pdf.SetTextColor(140, 140, 155)
	pdf.SetFont("Helvetica", "I", 8)
	pdf.CellFormat(180, 6,
		fmt.Sprintf("Dieses Zertifikat wurde automatisch durch die Vakt-Plattform am %s ausgestellt.", time.Now().Format("02.01.2006")),
		"", 1, "C", false, 0, "")

	var buf bytes.Buffer
	if err := pdf.Output(&buf); err != nil {
		return nil, fmt.Errorf("pdf output: %w", err)
	}
	return buf.Bytes(), nil
}
