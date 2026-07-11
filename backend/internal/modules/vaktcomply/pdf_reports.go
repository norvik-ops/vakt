// Copyright (c) 2026 NorvikOps. All rights reserved.
// SPDX-License-Identifier: Elastic-2.0

package vaktcomply

import (
	"bytes"
	"encoding/json"
	"fmt"
	"strconv"
	"time"

	"github.com/go-pdf/fpdf"

	auditmod "github.com/matharnica/vakt/internal/modules/vaktcomply/audit"
	"github.com/matharnica/vakt/internal/modules/vaktcomply/reporting"
)

// reportHeaderBar draws the standard blue Vakt header bar with a title and the
// organisation name, matching GenerateBoardReportPDF. It leaves the cursor just
// below the bar.
func reportHeaderBar(pdf *fpdf.Fpdf, title, orgName string) {
	pdf.SetFillColor(37, 99, 235)
	pdf.Rect(0, 0, 210, 28, "F")
	pdf.SetTextColor(255, 255, 255)
	pdf.SetFont("Helvetica", "B", 14)
	pdf.SetXY(15, 8)
	pdf.CellFormat(180, 8, title, "", 1, "L", false, 0, "")
	pdf.SetFont("Helvetica", "", 9)
	pdf.SetXY(15, 17)
	pdf.CellFormat(180, 6, orgName, "", 1, "L", false, 0, "")
	pdf.SetTextColor(30, 30, 40)
	pdf.SetXY(15, 34)
}

// reportFieldBlock renders a bold label followed by a wrapped multi-line value.
// Empty values render as an em dash so the section stays legible.
func reportFieldBlock(pdf *fpdf.Fpdf, label, value string) {
	if value == "" {
		value = "—"
	}
	pdf.SetFont("Helvetica", "B", 10)
	pdf.SetTextColor(30, 30, 40)
	pdf.MultiCell(180, 5.5, label, "", "L", false)
	pdf.SetFont("Helvetica", "", 10)
	pdf.SetTextColor(60, 60, 70)
	pdf.MultiCell(180, 5.5, value, "", "L", false)
	pdf.Ln(2)
}

// GenerateKPIReportPDF renders the current KPI dashboard snapshot as a PDF.
// Every KPI is optional (pointer types) — a missing value renders as "n/a".
func GenerateKPIReportPDF(dashboard reporting.KPIDashboard, orgName string, generatedAt time.Time) ([]byte, error) {
	pdf := fpdf.New("P", "mm", "A4", "")
	pdf.SetMargins(15, 15, 15)
	pdf.SetAutoPageBreak(true, 18)
	pdf.AliasNbPages("{nb}")
	pdf.SetFooterFunc(func() {
		pdf.SetY(-12)
		pdf.SetFont("Helvetica", "I", 7)
		pdf.SetTextColor(150, 150, 160)
		pdf.CellFormat(0, 5,
			fmt.Sprintf("Vakt Comply — %s — Seite %d/{nb}", orgName, pdf.PageNo()),
			"", 0, "C", false, 0, "")
	})
	pdf.AddPage()
	reportHeaderBar(pdf, "Vakt — KPI-Report", orgName)

	pdf.SetFont("Helvetica", "B", 17)
	pdf.CellFormat(180, 10, "ISMS-Kennzahlen", "", 1, "L", false, 0, "")
	pdf.SetFont("Helvetica", "", 9)
	pdf.SetTextColor(110, 110, 120)
	pdf.CellFormat(180, 6, "Stand: "+generatedAt.Format("02.01.2006 15:04")+" UTC", "", 1, "L", false, 0, "")
	pdf.Ln(4)

	cur := dashboard.Current
	rows := [][2]string{
		{"Compliance-Score", pctPtr(kpiFloat(cur, func(s *reporting.KPISnapshot) *float64 { return s.ComplianceScore }))},
		{"Offene kritische Controls", intPtrStr(kpiInt(cur, func(s *reporting.KPISnapshot) *int { return s.OpenCriticalControls }))},
		{"Offene hohe Risiken", intPtrStr(kpiInt(cur, func(s *reporting.KPISnapshot) *int { return s.OpenHighRisks }))},
		{"Ø Restrisiko", numPtr(kpiFloat(cur, func(s *reporting.KPISnapshot) *float64 { return s.ResidualRiskAvg }))},
		{"Offene Incidents", intPtrStr(kpiInt(cur, func(s *reporting.KPISnapshot) *int { return s.OpenIncidents }))},
		{"Incident-MTTR (Tage)", numPtr(kpiFloat(cur, func(s *reporting.KPISnapshot) *float64 { return s.IncidentMTTRDays }))},
		{"Evidence-Abdeckung", pctPtr(kpiFloat(cur, func(s *reporting.KPISnapshot) *float64 { return s.EvidenceCoverage }))},
		{"Ablaufende Evidence", intPtrStr(kpiInt(cur, func(s *reporting.KPISnapshot) *int { return s.ExpiringEvidenceCount }))},
		{"Finding-SLA-Einhaltung", pctPtr(kpiFloat(cur, func(s *reporting.KPISnapshot) *float64 { return s.FindingSLACompliance }))},
		{"Offene Major-NCs", intPtrStr(kpiInt(cur, func(s *reporting.KPISnapshot) *int { return s.OpenMajorNCs }))},
		{"Lieferanten überfällig", pctPtr(kpiFloat(cur, func(s *reporting.KPISnapshot) *float64 { return s.SuppliersOverduePct }))},
		{"Phishing-Klickrate", pctPtr(kpiFloat(cur, func(s *reporting.KPISnapshot) *float64 { return s.PhishingClickRate }))},
	}

	// Two-column KPI table with zebra striping.
	pdf.SetFont("Helvetica", "", 10)
	for i, r := range rows {
		if i%2 == 0 {
			pdf.SetFillColor(244, 246, 251)
		} else {
			pdf.SetFillColor(255, 255, 255)
		}
		pdf.SetTextColor(40, 40, 50)
		pdf.CellFormat(120, 9, "  "+r[0], "", 0, "L", true, 0, "")
		pdf.SetFont("Helvetica", "B", 10)
		pdf.CellFormat(60, 9, r[1]+"  ", "", 1, "R", true, 0, "")
		pdf.SetFont("Helvetica", "", 10)
	}

	if cur == nil {
		pdf.Ln(4)
		pdf.SetFont("Helvetica", "I", 9)
		pdf.SetTextColor(150, 150, 160)
		pdf.MultiCell(180, 5, "Noch kein KPI-Snapshot vorhanden. Kennzahlen werden beim nächsten geplanten Lauf erfasst.", "", "L", false)
	}

	return pdfBytesMain(pdf)
}

// GenerateManagementReviewPDF renders an ISO 27001 management review (Clause 9.3)
// as an audit-ready PDF with all input and output sections.
func GenerateManagementReviewPDF(review auditmod.ManagementReview, orgName string, generatedAt time.Time) ([]byte, error) {
	pdf := fpdf.New("P", "mm", "A4", "")
	pdf.SetMargins(15, 15, 15)
	pdf.SetAutoPageBreak(true, 18)
	pdf.AliasNbPages("{nb}")
	pdf.SetFooterFunc(func() {
		pdf.SetY(-12)
		pdf.SetFont("Helvetica", "I", 7)
		pdf.SetTextColor(150, 150, 160)
		pdf.CellFormat(0, 5,
			fmt.Sprintf("Vakt Comply — %s — Seite %d/{nb}", orgName, pdf.PageNo()),
			"", 0, "C", false, 0, "")
	})
	pdf.AddPage()
	reportHeaderBar(pdf, "Vakt — Managementbewertung (ISO 27001 Kap. 9.3)", orgName)

	pdf.SetFont("Helvetica", "B", 16)
	pdf.CellFormat(180, 9, "Managementbewertung", "", 1, "L", false, 0, "")
	pdf.SetFont("Helvetica", "", 9)
	pdf.SetTextColor(110, 110, 120)
	meta := fmt.Sprintf("Datum: %s   |   Typ: %s   |   Status: %s",
		review.ReviewDate, reviewTypeLabel(review.ReviewType), reviewStatusLabel(review.Status))
	pdf.CellFormat(180, 6, meta, "", 1, "L", false, 0, "")
	if review.ApprovedAt != nil {
		pdf.CellFormat(180, 6, "Freigegeben am: "+review.ApprovedAt.Format("02.01.2006"), "", 1, "L", false, 0, "")
	}
	pdf.Ln(3)

	section := func(title string) {
		pdf.Ln(1)
		pdf.SetFillColor(37, 99, 235)
		pdf.SetTextColor(255, 255, 255)
		pdf.SetFont("Helvetica", "B", 11)
		pdf.CellFormat(180, 8, "  "+title, "", 1, "L", true, 0, "")
		pdf.Ln(2)
	}

	section("Eingaben (Clause 9.3.2)")
	reportFieldBlock(pdf, "Audit-Ergebnisse", review.AuditFindingsSummary)
	reportFieldBlock(pdf, "Sicherheitsvorfälle", review.IncidentSummary)
	reportFieldBlock(pdf, "Risikostatus", review.RiskStatusSummary)
	reportFieldBlock(pdf, "Status früherer Maßnahmen", review.PreviousActionsStatus)
	reportFieldBlock(pdf, "Kontextänderungen", review.ContextChanges)
	reportFieldBlock(pdf, "Rückmeldungen interessierter Parteien", review.CustomerFeedback)

	section("Ergebnisse (Clause 9.3.3)")
	reportFieldBlock(pdf, "Verbesserungsentscheidungen", jsonListToText(review.ImprovementDecisions))
	reportFieldBlock(pdf, "Ressourcenentscheidungen", review.ResourceDecisions)
	reportFieldBlock(pdf, "Änderungen am ISMS", review.ISMSChanges)
	if review.NextReviewDate != nil {
		reportFieldBlock(pdf, "Nächste Bewertung", *review.NextReviewDate)
	}

	return pdfBytesMain(pdf)
}

// --- helpers ---

func pdfBytesMain(pdf *fpdf.Fpdf) ([]byte, error) {
	var buf bytes.Buffer
	if err := pdf.Output(&buf); err != nil {
		return nil, fmt.Errorf("render pdf: %w", err)
	}
	return buf.Bytes(), nil
}

func kpiFloat(s *reporting.KPISnapshot, get func(*reporting.KPISnapshot) *float64) *float64 {
	if s == nil {
		return nil
	}
	return get(s)
}

func kpiInt(s *reporting.KPISnapshot, get func(*reporting.KPISnapshot) *int) *int {
	if s == nil {
		return nil
	}
	return get(s)
}

func pctPtr(f *float64) string {
	if f == nil {
		return "n/a"
	}
	return strconv.FormatFloat(*f, 'f', 1, 64) + " %"
}

func numPtr(f *float64) string {
	if f == nil {
		return "n/a"
	}
	return strconv.FormatFloat(*f, 'f', 1, 64)
}

func intPtrStr(i *int) string {
	if i == nil {
		return "n/a"
	}
	return strconv.Itoa(*i)
}

func reviewTypeLabel(t string) string {
	switch t {
	case "annual":
		return "Jährlich"
	case "extraordinary":
		return "Außerordentlich"
	default:
		return t
	}
}

func reviewStatusLabel(s string) string {
	switch s {
	case "draft":
		return "Entwurf"
	case "in_progress":
		return "In Bearbeitung"
	case "completed":
		return "Abgeschlossen"
	case "approved":
		return "Freigegeben"
	default:
		return s
	}
}

// decisionItem covers the field names an improvement-decision object may use.
// Kept as a typed struct — not an untyped JSON map — so the interface ratchet
// stays flat.
type decisionItem struct {
	Text        string `json:"text"`
	Description string `json:"description"`
	Decision    string `json:"decision"`
	Title       string `json:"title"`
}

func (d decisionItem) label() string {
	for _, s := range []string{d.Text, d.Description, d.Decision, d.Title} {
		if s != "" {
			return s
		}
	}
	return ""
}

// jsonListToText renders a JSON array of strings (or of {text}/{description}
// objects) as newline-separated bullet points. Falls back to the raw string for
// anything it can't decode, so a partial value never blocks the export.
func jsonListToText(raw json.RawMessage) string {
	if len(raw) == 0 {
		return ""
	}
	var arr []json.RawMessage
	if err := json.Unmarshal(raw, &arr); err != nil {
		return string(raw)
	}
	out := ""
	for _, el := range arr {
		var s string
		if json.Unmarshal(el, &s) == nil {
			out += "• " + s + "\n"
			continue
		}
		var obj decisionItem
		if json.Unmarshal(el, &obj) == nil {
			if v := obj.label(); v != "" {
				out += "• " + v + "\n"
			}
		}
	}
	return out
}
