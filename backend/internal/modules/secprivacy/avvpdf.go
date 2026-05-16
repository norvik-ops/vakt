package secprivacy

import (
	"bytes"
	"fmt"
	"strings"
	"time"

	"github.com/go-pdf/fpdf"
)

// AVVWithBody holds all fields needed to render an AVV PDF.
type AVVWithBody struct {
	Name           string
	ProcessorName  string
	ControllerName string
	Purpose        string
	Body           string
	CreatedAt      time.Time
}

// AVVWithSCC extends AVVWithBody with EU Standard Contractual Clauses data.
type AVVWithSCC struct {
	AVVWithBody
	SCCModule string
	AnnexI    string
	AnnexII   string
	AnnexIII  string
}

// GenerateAVVPDF renders an AVV body as a PDF and returns the raw bytes.
// The document uses the same blue-header style as secvitals reports.
func GenerateAVVPDF(avv AVVWithBody, orgName string) ([]byte, error) {
	pdf := fpdf.New("P", "mm", "A4", "")
	pdf.SetMargins(15, 15, 15)
	pdf.SetAutoPageBreak(true, 15)

	// Footer must be registered before AddPage
	pdf.SetFooterFunc(func() {
		pdf.SetY(-12)
		pdf.SetFont("Helvetica", "I", 7)
		pdf.SetTextColor(150, 150, 160)
		pdf.CellFormat(0, 5,
			fmt.Sprintf("Vakt Comply — %s — Seite %d/{nb}", orgName, pdf.PageNo()),
			"", 0, "C", false, 0, "")
	})
	pdf.AliasNbPages("{nb}")

	pdf.AddPage()

	// ── Header bar ─────────────────────────────────────────────────────────────
	pdf.SetFillColor(37, 99, 235)
	pdf.Rect(0, 0, 210, 28, "F")
	pdf.SetTextColor(255, 255, 255)
	pdf.SetFont("Helvetica", "B", 14)
	pdf.SetXY(15, 8)
	pdf.CellFormat(180, 8, "Vakt — Auftragsverarbeitungsvertrag (Art. 28 DSGVO)", "", 1, "L", false, 0, "")
	pdf.SetFont("Helvetica", "", 9)
	pdf.SetXY(15, 17)
	pdf.CellFormat(180, 6, fmt.Sprintf("%s  |  Erstellt: %s", orgName, avv.CreatedAt.Format("02.01.2006")), "", 1, "L", false, 0, "")

	// ── Title block ────────────────────────────────────────────────────────────
	pdf.SetTextColor(30, 30, 40)
	pdf.SetXY(15, 34)
	pdf.SetFont("Helvetica", "B", 14)
	title := avv.Name
	if title == "" {
		title = avv.ProcessorName
	}
	pdf.MultiCell(180, 8, title, "", "L", false)

	pdf.SetFont("Helvetica", "", 9)
	pdf.SetTextColor(100, 100, 120)

	renderMeta := func(label, value string) {
		pdf.SetFont("Helvetica", "B", 9)
		pdf.SetTextColor(100, 100, 120)
		pdf.CellFormat(45, 6, label, "0", 0, "L", false, 0, "")
		pdf.SetFont("Helvetica", "", 9)
		pdf.SetTextColor(30, 30, 40)
		pdf.MultiCell(135, 6, value, "0", "L", false)
	}

	pdf.SetY(pdf.GetY() + 2)
	renderMeta("Auftraggeber:", avv.ControllerName)
	renderMeta("Auftragnehmer:", avv.ProcessorName)
	if avv.Purpose != "" {
		renderMeta("Zweck:", avv.Purpose)
	}

	// ── Divider ────────────────────────────────────────────────────────────────
	pdf.SetDrawColor(200, 210, 240)
	pdf.SetY(pdf.GetY() + 4)
	pdf.Line(15, pdf.GetY(), 195, pdf.GetY())
	pdf.SetY(pdf.GetY() + 6)

	// ── Body text (markdown rendered as plain paragraphs) ──────────────────────
	renderBody(pdf, avv.Body)

	var buf bytes.Buffer
	if err := pdf.Output(&buf); err != nil {
		return nil, fmt.Errorf("avv pdf output: %w", err)
	}
	return buf.Bytes(), nil
}

// GenerateSCCPDF renders an AVV with EU Standard Contractual Clauses (SCC) annexes as PDF.
func GenerateSCCPDF(avv AVVWithSCC, orgName string) ([]byte, error) {
	pdf := fpdf.New("P", "mm", "A4", "")
	pdf.SetMargins(15, 15, 15)
	pdf.SetAutoPageBreak(true, 15)

	pdf.SetFooterFunc(func() {
		pdf.SetY(-12)
		pdf.SetFont("Helvetica", "I", 7)
		pdf.SetTextColor(150, 150, 160)
		pdf.CellFormat(0, 5,
			fmt.Sprintf("Vakt Comply — %s — SCC — Seite %d/{nb}", orgName, pdf.PageNo()),
			"", 0, "C", false, 0, "")
	})
	pdf.AliasNbPages("{nb}")

	pdf.AddPage()

	// ── Header bar ─────────────────────────────────────────────────────────────
	pdf.SetFillColor(37, 99, 235)
	pdf.Rect(0, 0, 210, 28, "F")
	pdf.SetTextColor(255, 255, 255)
	pdf.SetFont("Helvetica", "B", 14)
	pdf.SetXY(15, 8)
	pdf.CellFormat(180, 8, "Vakt — EU-Standarddatenschutzklauseln (SCC)", "", 1, "L", false, 0, "")
	pdf.SetFont("Helvetica", "", 9)
	pdf.SetXY(15, 17)
	sccTitle := sccModuleLabel(avv.SCCModule)
	pdf.CellFormat(180, 6, fmt.Sprintf("%s  |  %s  |  %s", orgName, sccTitle, avv.CreatedAt.Format("02.01.2006")), "", 1, "L", false, 0, "")

	// ── SCC module banner ──────────────────────────────────────────────────────
	pdf.SetFillColor(239, 246, 255)
	pdf.Rect(0, 28, 210, 12, "F")
	pdf.SetTextColor(37, 99, 235)
	pdf.SetFont("Helvetica", "B", 9)
	pdf.SetXY(15, 31)
	pdf.CellFormat(180, 6, "Durchführungsbeschluss (EU) 2021/914 der Kommission vom 4. Juni 2021", "", 1, "C", false, 0, "")

	// ── Parties ────────────────────────────────────────────────────────────────
	pdf.SetTextColor(30, 30, 40)
	pdf.SetXY(15, 46)

	renderMeta := func(label, value string) {
		if pdf.GetY() > 265 {
			pdf.AddPage()
		}
		pdf.SetFont("Helvetica", "B", 9)
		pdf.SetTextColor(100, 100, 120)
		pdf.CellFormat(45, 6, label, "0", 0, "L", false, 0, "")
		pdf.SetFont("Helvetica", "", 9)
		pdf.SetTextColor(30, 30, 40)
		pdf.MultiCell(135, 6, value, "0", "L", false)
	}

	renderMeta("Datenexporteur:", avv.ControllerName)
	renderMeta("Datenimporteur:", avv.ProcessorName)
	renderMeta("SCC-Modul:", sccTitle)
	if avv.Purpose != "" {
		renderMeta("Zweck:", avv.Purpose)
	}

	// ── AVV body ───────────────────────────────────────────────────────────────
	if avv.Body != "" {
		pdf.SetY(pdf.GetY() + 4)
		pdf.SetDrawColor(200, 210, 240)
		pdf.Line(15, pdf.GetY(), 195, pdf.GetY())
		pdf.SetY(pdf.GetY() + 4)
		renderBody(pdf, avv.Body)
	}

	// ── Annex I ────────────────────────────────────────────────────────────────
	if avv.AnnexI != "" {
		pdf.AddPage()
		renderAnnexHeader(pdf, "Anhang I", "Beschreibung der Übermittlungen")
		renderBody(pdf, avv.AnnexI)
	}

	// ── Annex II ───────────────────────────────────────────────────────────────
	if avv.AnnexII != "" {
		pdf.AddPage()
		renderAnnexHeader(pdf, "Anhang II", "Technische und organisatorische Maßnahmen")
		renderBody(pdf, avv.AnnexII)
	}

	// ── Annex III ──────────────────────────────────────────────────────────────
	if avv.AnnexIII != "" {
		pdf.AddPage()
		renderAnnexHeader(pdf, "Anhang III", "Liste der Unterauftragsverarbeiter")
		renderBody(pdf, avv.AnnexIII)
	}

	var buf bytes.Buffer
	if err := pdf.Output(&buf); err != nil {
		return nil, fmt.Errorf("scc pdf output: %w", err)
	}
	return buf.Bytes(), nil
}

// renderAnnexHeader renders a section title for a SCC annex page.
func renderAnnexHeader(pdf *fpdf.Fpdf, annexID, annexTitle string) {
	pdf.SetFillColor(37, 99, 235)
	pdf.Rect(0, 0, 210, 18, "F")
	pdf.SetTextColor(255, 255, 255)
	pdf.SetFont("Helvetica", "B", 12)
	pdf.SetXY(15, 5)
	pdf.CellFormat(180, 8, fmt.Sprintf("%s — %s", annexID, annexTitle), "", 1, "L", false, 0, "")

	pdf.SetTextColor(30, 30, 40)
	pdf.SetY(24)
}

// renderBody converts simple markdown-ish text to fpdf output.
// It handles: # headings, ## subheadings, - list items, blank lines, and normal paragraphs.
func renderBody(pdf *fpdf.Fpdf, body string) {
	lines := strings.Split(body, "\n")
	for _, raw := range lines {
		line := strings.TrimRight(raw, " \t")

		if pdf.GetY() > 265 {
			pdf.AddPage()
		}

		switch {
		case strings.HasPrefix(line, "# "):
			pdf.SetY(pdf.GetY() + 3)
			pdf.SetFont("Helvetica", "B", 13)
			pdf.SetTextColor(30, 30, 40)
			pdf.MultiCell(180, 8, strings.TrimPrefix(line, "# "), "", "L", false)

		case strings.HasPrefix(line, "## "):
			pdf.SetY(pdf.GetY() + 2)
			pdf.SetFont("Helvetica", "B", 11)
			pdf.SetTextColor(37, 99, 235)
			pdf.MultiCell(180, 7, strings.TrimPrefix(line, "## "), "", "L", false)
			pdf.SetTextColor(30, 30, 40)

		case strings.HasPrefix(line, "### "):
			pdf.SetY(pdf.GetY() + 1)
			pdf.SetFont("Helvetica", "B", 10)
			pdf.SetTextColor(60, 60, 80)
			pdf.MultiCell(180, 6, strings.TrimPrefix(line, "### "), "", "L", false)
			pdf.SetTextColor(30, 30, 40)

		case strings.HasPrefix(line, "- ") || strings.HasPrefix(line, "* "):
			text := strings.TrimPrefix(strings.TrimPrefix(line, "- "), "* ")
			pdf.SetFont("Helvetica", "", 9)
			pdf.SetTextColor(30, 30, 40)
			x := pdf.GetX()
			pdf.SetX(15)
			pdf.CellFormat(6, 5, "\xe2\x80\xa2", "", 0, "L", false, 0, "")
			pdf.MultiCell(174, 5, text, "", "L", false)
			_ = x

		case strings.HasPrefix(line, "**") && strings.HasSuffix(line, "**"):
			pdf.SetFont("Helvetica", "B", 9)
			pdf.SetTextColor(30, 30, 40)
			pdf.MultiCell(180, 5, strings.Trim(line, "*"), "", "L", false)

		case line == "" || line == "---":
			pdf.SetY(pdf.GetY() + 3)

		default:
			pdf.SetFont("Helvetica", "", 9)
			pdf.SetTextColor(30, 30, 40)
			pdf.MultiCell(180, 5, line, "", "L", false)
		}
	}
}

// sccModuleLabel returns a short human-readable label for an SCC module ID.
func sccModuleLabel(moduleID string) string {
	labels := map[string]string{
		"module_1": "Modul 1 (C2C)",
		"module_2": "Modul 2 (C2P)",
		"module_3": "Modul 3 (P2P)",
		"module_4": "Modul 4 (P2C)",
	}
	if l, ok := labels[moduleID]; ok {
		return l
	}
	return moduleID
}
