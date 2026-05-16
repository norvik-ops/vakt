package secreflex

import (
	"bytes"
	"fmt"
	"time"

	"github.com/go-pdf/fpdf"
)

// GenerateCampaignReportPDF renders a phishing-simulation campaign report as PDF bytes.
func GenerateCampaignReportPDF(campaign *Campaign, stats *CampaignStats, orgName string) ([]byte, error) {
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
	pdf.CellFormat(180, 8, "Vakt — Phishing-Simulation Bericht", "", 1, "L", false, 0, "")
	pdf.SetFont("Helvetica", "", 9)
	pdf.SetXY(15, 17)
	pdf.CellFormat(180, 6, orgName, "", 1, "L", false, 0, "")

	// ── Title ─────────────────────────────────────────────────────────────────
	pdf.SetTextColor(30, 30, 40)
	pdf.SetXY(15, 35)
	pdf.SetFont("Helvetica", "B", 18)
	pdf.CellFormat(180, 10, campaign.Name, "", 1, "L", false, 0, "")
	pdf.SetFont("Helvetica", "", 10)
	pdf.SetTextColor(100, 100, 120)
	pdf.CellFormat(180, 7, fmt.Sprintf("Erstellt am %s", now.Format("02.01.2006 15:04")), "", 1, "L", false, 0, "")

	// ── Campaign meta ─────────────────────────────────────────────────────────
	pdf.SetY(pdf.GetY() + 6)
	pdf.SetTextColor(30, 30, 40)
	pdf.SetFont("Helvetica", "B", 11)
	pdf.CellFormat(180, 8, "Kampagnen-Details", "", 1, "L", false, 0, "")

	type metaRow struct{ label, value string }
	meta := []metaRow{{"Status", campaign.Status}}
	if campaign.StartedAt != nil {
		meta = append(meta, metaRow{"Gestartet", campaign.StartedAt.Format("02.01.2006 15:04")})
	}
	if campaign.CompletedAt != nil {
		meta = append(meta, metaRow{"Abgeschlossen", campaign.CompletedAt.Format("02.01.2006 15:04")})
	}
	if campaign.BetriebsratMode {
		meta = append(meta, metaRow{"Betriebsrat-Modus", "Ja (anonymisiert)"})
	}
	for _, row := range meta {
		pdf.SetFont("Helvetica", "B", 9)
		pdf.CellFormat(45, 6, row.label+":", "0", 0, "L", false, 0, "")
		pdf.SetFont("Helvetica", "", 9)
		pdf.CellFormat(135, 6, row.value, "0", 1, "L", false, 0, "")
	}

	// ── Stats boxes ───────────────────────────────────────────────────────────
	pdf.SetY(pdf.GetY() + 5)
	pdf.SetFont("Helvetica", "B", 11)
	pdf.SetTextColor(30, 30, 40)
	pdf.CellFormat(180, 8, "Ergebnisse", "", 1, "L", false, 0, "")

	type statBox struct {
		label   string
		value   string
		r, g, b int
	}
	boxes := []statBox{
		{"Ziele", fmt.Sprintf("%d", stats.TotalTargets), 55, 65, 81},
		{"Gesendet", fmt.Sprintf("%d", stats.EmailsSent), 55, 65, 81},
		{"Öffnungen", fmt.Sprintf("%d", stats.Opens), 59, 130, 246},
		{"Klicks", fmt.Sprintf("%d", stats.Clicks), 234, 88, 12},
		{"Formulare", fmt.Sprintf("%d", stats.FormSubmissions), 220, 38, 38},
		{"Klickrate", fmt.Sprintf("%.1f%%", stats.ClickRate), 234, 88, 12},
	}

	const boxW, gap, startX = 28.0, 2.0, 15.0
	y := pdf.GetY() + 4
	for i, b := range boxes {
		x := startX + float64(i)*(boxW+gap)
		pdf.SetFillColor(b.r, b.g, b.b)
		pdf.RoundedRect(x, y, boxW, 22, 2, "1234", "F")
		pdf.SetTextColor(255, 255, 255)
		pdf.SetFont("Helvetica", "B", 13)
		pdf.SetXY(x, y+4)
		pdf.CellFormat(boxW, 8, b.value, "", 1, "C", false, 0, "")
		pdf.SetFont("Helvetica", "", 7)
		pdf.SetXY(x, y+12)
		pdf.CellFormat(boxW, 6, b.label, "", 1, "C", false, 0, "")
	}

	// ── Rate bars ─────────────────────────────────────────────────────────────
	pdf.SetY(y + 30)
	pdf.SetTextColor(30, 30, 40)
	pdf.SetFont("Helvetica", "B", 11)
	pdf.CellFormat(180, 8, "Raten", "", 1, "L", false, 0, "")

	type rateBar struct {
		label   string
		rate    float64
		r, g, b int
	}
	rates := []rateBar{
		{"Öffnungsrate", stats.OpenRate, 59, 130, 246},
		{"Klickrate", stats.ClickRate, 234, 88, 12},
		{"Formular-Abgaberate", stats.SubmissionRate, 220, 38, 38},
	}
	const barX, barW, barH = 70.0, 100.0, 4.0
	for _, rate := range rates {
		pdf.SetFont("Helvetica", "", 9)
		pdf.SetTextColor(30, 30, 40)
		pdf.CellFormat(55, 6, rate.label, "0", 0, "L", false, 0, "")
		barY := pdf.GetY() + 1
		pct := rate.rate / 100.0
		if pct > 1 {
			pct = 1
		}
		pdf.SetFillColor(230, 232, 240)
		pdf.Rect(barX, barY, barW, barH, "F")
		pdf.SetFillColor(rate.r, rate.g, rate.b)
		if pct > 0 {
			pdf.Rect(barX, barY, barW*pct, barH, "F")
		}
		pdf.SetTextColor(30, 30, 40)
		pdf.SetFont("Helvetica", "B", 9)
		pdf.SetXY(barX+barW+3, barY-1)
		pdf.CellFormat(20, 6, fmt.Sprintf("%.1f%%", rate.rate), "0", 0, "L", false, 0, "")
		pdf.Ln(7)
	}

	// ── Betriebsrat note ──────────────────────────────────────────────────────
	if campaign.BetriebsratMode {
		pdf.SetY(pdf.GetY() + 5)
		noteY := pdf.GetY()
		pdf.SetFillColor(254, 243, 199)
		pdf.Rect(15, noteY, 180, 14, "F")
		pdf.SetTextColor(120, 90, 0)
		pdf.SetFont("Helvetica", "B", 9)
		pdf.SetXY(18, noteY+2)
		pdf.CellFormat(174, 5, "Betriebsrat-Modus aktiv", "", 1, "L", false, 0, "")
		pdf.SetFont("Helvetica", "", 8)
		pdf.SetXY(18, pdf.GetY())
		pdf.CellFormat(174, 5, "Individuelle Ergebnisse nicht erfasst. Nur aggregierte Statistiken werden angezeigt.", "", 1, "L", false, 0, "")
	}

	// ── Footer ────────────────────────────────────────────────────────────────
	pdf.SetFooterFunc(func() {
		pdf.SetY(-12)
		pdf.SetFont("Helvetica", "I", 7)
		pdf.SetTextColor(150, 150, 160)
		pdf.CellFormat(0, 5,
			fmt.Sprintf("Vakt Aware — %s — Seite %d/{nb}", orgName, pdf.PageNo()),
			"", 0, "C", false, 0, "")
	})
	pdf.AliasNbPages("{nb}")

	var buf bytes.Buffer
	if err := pdf.Output(&buf); err != nil {
		return nil, fmt.Errorf("pdf output: %w", err)
	}
	return buf.Bytes(), nil
}
