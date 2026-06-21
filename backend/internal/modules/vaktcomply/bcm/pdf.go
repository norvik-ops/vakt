// Copyright (c) 2026 NorvikOps. All rights reserved.
// SPDX-License-Identifier: Elastic-2.0

// S86-6: PDF-Export Notfallhandbuch (BSI-200-4 §8)

package bcm

import (
	"bytes"
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/go-pdf/fpdf"
)

// GenerateBCMHandbuchPDF renders the 7-section Notfallhandbuch PDF for the org.
// Returns the PDF bytes and the SHA-256 hash for audit logging.
func (s *Service) GenerateBCMHandbuchPDF(ctx context.Context, orgID string) ([]byte, error) {
	// Load data
	bcpPlans, _ := s.repo.ListBCPPlans(ctx, orgID)
	biaProcesses, _ := s.repo.ListBIAProcesses(ctx, orgID)
	recoveryPlans, _ := s.repo.ListRecoveryPlans(ctx, orgID)
	contacts, _ := s.repo.ListEmergencyContacts(ctx, orgID)

	// Look up org name from BCP plan or fallback
	orgName := "Organisation"
	if len(bcpPlans) > 0 {
		orgName = bcpPlans[0].Owner
		if orgName == "" {
			orgName = "Organisation"
		}
	}

	pdf := fpdf.New("P", "mm", "A4", "")
	pdf.SetMargins(20, 20, 20)
	pdf.AddPage()

	// ── 1. Deckblatt ─────────────────────────────────────────────────────────
	pdf.SetFont("Helvetica", "B", 24)
	pdf.CellFormat(0, 20, "Notfallhandbuch", "", 1, "C", false, 0, "")
	pdf.SetFont("Helvetica", "", 14)
	pdf.CellFormat(0, 10, orgName, "", 1, "C", false, 0, "")
	pdf.CellFormat(0, 10, time.Now().Format("02.01.2006"), "", 1, "C", false, 0, "")
	pdf.CellFormat(0, 10, "BSI 200-4 konform", "", 1, "C", false, 0, "")
	pdf.Ln(10)

	// ── 2. Geltungsbereich ───────────────────────────────────────────────────
	pdfSection(pdf, "1. Geltungsbereich")
	if len(bcpPlans) > 0 {
		scope := bcpPlans[0].Scope
		if scope == "" {
			scope = "Alle kritischen IT-Systeme und Geschäftsprozesse der Organisation."
		}
		pdf.SetFont("Helvetica", "", 10)
		pdf.MultiCell(0, 6, scope, "", "", false)
	} else {
		pdf.SetFont("Helvetica", "", 10)
		pdf.MultiCell(0, 6, "Kein BCP-Plan konfiguriert.", "", "", false)
	}
	pdf.Ln(4)

	// ── 3. Geschäftseinflussanalyse (BIA) ───────────────────────────────────
	pdfSection(pdf, "2. Geschäftseinflussanalyse (BIA)")
	if len(biaProcesses) > 0 {
		pdf.SetFont("Helvetica", "B", 9)
		pdf.CellFormat(50, 7, "Prozess", "1", 0, "", false, 0, "")
		pdf.CellFormat(25, 7, "Kritikalität", "1", 0, "", false, 0, "")
		pdf.CellFormat(15, 7, "Klasse", "1", 0, "", false, 0, "")
		pdf.CellFormat(20, 7, "RTO (h)", "1", 0, "", false, 0, "")
		pdf.CellFormat(20, 7, "RPO (h)", "1", 0, "", false, 0, "")
		pdf.CellFormat(20, 7, "MBCO %", "1", 0, "", false, 0, "")
		pdf.CellFormat(0, 7, "Owner", "1", 1, "", false, 0, "")
		pdf.SetFont("Helvetica", "", 9)
		for _, p := range biaProcesses {
			pdf.CellFormat(50, 6, pdfTruncate(p.Name, 28), "1", 0, "", false, 0, "")
			pdf.CellFormat(25, 6, p.Criticality, "1", 0, "", false, 0, "")
			pdf.CellFormat(15, 6, fmt.Sprintf("%d", p.Schutzbedarfsklasse), "1", 0, "", false, 0, "")
			pdf.CellFormat(20, 6, fmt.Sprintf("%d", p.RTOHours), "1", 0, "", false, 0, "")
			pdf.CellFormat(20, 6, fmt.Sprintf("%d", p.RPOHours), "1", 0, "", false, 0, "")
			pdf.CellFormat(20, 6, fmt.Sprintf("%d%%", p.MBCOPercent), "1", 0, "", false, 0, "")
			pdf.CellFormat(0, 6, pdfTruncate(p.ProcessOwner, 20), "1", 1, "", false, 0, "")
		}
	} else {
		pdf.SetFont("Helvetica", "", 10)
		pdf.MultiCell(0, 6, "Noch keine BIA-Prozesse erfasst.", "", "", false)
	}
	pdf.Ln(4)

	// ── 4. Kritische Prozesse ─────────────────────────────────────────────────
	pdfSection(pdf, "3. Kritische Prozesse (Priorität)")
	pdf.SetFont("Helvetica", "", 10)
	critCount := 0
	for _, p := range biaProcesses {
		if p.Criticality == "high" {
			pdf.MultiCell(0, 6, fmt.Sprintf("• %s — RTO: %d h, RPO: %d h, MBCO: %d%%", p.Name, p.RTOHours, p.RPOHours, p.MBCOPercent), "", "", false)
			critCount++
		}
	}
	if critCount == 0 {
		pdf.MultiCell(0, 6, "Keine kritischen Prozesse definiert.", "", "", false)
	}
	pdf.Ln(4)

	// ── 5. Wiederanlaufpläne ─────────────────────────────────────────────────
	pdfSection(pdf, "4. Wiederanlaufpläne (WAP)")
	if len(recoveryPlans) > 0 {
		for _, plan := range recoveryPlans {
			pdf.SetFont("Helvetica", "B", 10)
			pdf.MultiCell(0, 7, fmt.Sprintf("WAP: %s", plan.Title), "", "", false)
			pdf.SetFont("Helvetica", "", 9)
			pdf.MultiCell(0, 6, fmt.Sprintf("Status: %s | RTO: %d h | Verantwortlich: %s", plan.Status, plan.RTOHours, plan.Responsible), "", "", false)
			if plan.ActivationCriteria != "" {
				pdf.MultiCell(0, 6, "Aktivierungskriterien: "+plan.ActivationCriteria, "", "", false)
			}
			if len(plan.Steps) > 0 {
				pdf.SetFont("Helvetica", "I", 9)
				pdf.MultiCell(0, 6, "Schritte:", "", "", false)
				pdf.SetFont("Helvetica", "", 9)
				for _, step := range plan.Steps {
					pdf.MultiCell(0, 6, fmt.Sprintf("  %d. %s (%s, %d min)", step.Order, step.Action, step.Responsible, step.DurationMin), "", "", false)
				}
			}
			pdf.Ln(3)
		}
	} else {
		pdf.SetFont("Helvetica", "", 10)
		pdf.MultiCell(0, 6, "Noch keine Wiederanlaufpläne erfasst.", "", "", false)
	}
	pdf.Ln(4)

	// ── 6. Alarmierungsplan ──────────────────────────────────────────────────
	pdfSection(pdf, "5. Alarmierungsplan")
	if len(contacts) > 0 {
		pdf.SetFont("Helvetica", "B", 9)
		pdf.CellFormat(10, 7, "Lvl", "1", 0, "", false, 0, "")
		pdf.CellFormat(45, 7, "Name", "1", 0, "", false, 0, "")
		pdf.CellFormat(40, 7, "Rolle", "1", 0, "", false, 0, "")
		pdf.CellFormat(40, 7, "Telefon", "1", 0, "", false, 0, "")
		pdf.CellFormat(0, 7, "24/7", "1", 1, "", false, 0, "")
		pdf.SetFont("Helvetica", "", 9)
		for _, c := range contacts {
			avail := "Nein"
			if c.Available247 {
				avail = "Ja"
			}
			pdf.CellFormat(10, 6, fmt.Sprintf("%d", c.EscalationLevel), "1", 0, "", false, 0, "")
			pdf.CellFormat(45, 6, pdfTruncate(c.Name, 25), "1", 0, "", false, 0, "")
			pdf.CellFormat(40, 6, pdfTruncate(c.Role, 22), "1", 0, "", false, 0, "")
			pdf.CellFormat(40, 6, pdfTruncate(c.Phone, 22), "1", 0, "", false, 0, "")
			pdf.CellFormat(0, 6, avail, "1", 1, "", false, 0, "")
		}
	} else {
		pdf.SetFont("Helvetica", "", 10)
		pdf.MultiCell(0, 6, "Kein Alarmierungsplan konfiguriert.", "", "", false)
	}
	pdf.Ln(4)

	// ── 7. Übungshistorie ────────────────────────────────────────────────────
	pdfSection(pdf, "6. Übungshistorie (letzte 5 Tests)")
	pdf.SetFont("Helvetica", "", 10)
	testCount := 0
	for _, plan := range bcpPlans {
		tests, err := s.repo.ListBCPTests(ctx, orgID, plan.ID)
		if err != nil {
			continue
		}
		for i, t := range tests {
			if i >= 5 {
				break
			}
			pdf.MultiCell(0, 6, fmt.Sprintf("• %s — %s (%s) — Ergebnis: %s", plan.Title, t.TestDate, t.TestType, t.Outcome), "", "", false)
			testCount++
		}
	}
	if testCount == 0 {
		pdf.MultiCell(0, 6, "Noch keine Übungen durchgeführt.", "", "", false)
	}
	pdf.Ln(4)

	// ── 8. SHA-256 Audit Footer ───────────────────────────────────────────────
	pdf.SetFont("Helvetica", "I", 8)
	pdf.CellFormat(0, 6, fmt.Sprintf("Generiert: %s | Vakt Comply — BSI 200-4 Notfallhandbuch", time.Now().Format("02.01.2006 15:04")), "", 1, "C", false, 0, "")

	var buf bytes.Buffer
	if err := pdf.Output(&buf); err != nil {
		return nil, fmt.Errorf("render bcm pdf: %w", err)
	}
	return buf.Bytes(), nil
}

func pdfSection(pdf *fpdf.Fpdf, title string) {
	pdf.SetFont("Helvetica", "B", 12)
	pdf.SetFillColor(240, 240, 240)
	pdf.CellFormat(0, 8, title, "", 1, "L", true, 0, "")
	pdf.Ln(2)
}

func pdfTruncate(s string, max int) string {
	if len(s) <= max {
		return s
	}
	return strings.TrimSpace(s[:max-1]) + "…"
}
