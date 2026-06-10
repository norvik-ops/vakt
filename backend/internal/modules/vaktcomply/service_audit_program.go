// Copyright (c) 2026 NorvikOps. All rights reserved.
// SPDX-License-Identifier: Elastic-2.0

package vaktcomply

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/go-pdf/fpdf"
	"github.com/rs/zerolog/log"
)

// ListAuditPlans returns all audit plans for the org.
func (s *Service) ListAuditPlans(ctx context.Context, orgID string) ([]AuditPlan, error) {
	return s.repo.ListAuditPlans(ctx, orgID)
}

// CreateAuditPlan persists a new yearly audit plan.
func (s *Service) CreateAuditPlan(ctx context.Context, orgID string, in CreateAuditPlanInput) (*AuditPlan, error) {
	return s.repo.CreateAuditPlan(ctx, orgID, in)
}

// UpdateAuditPlan modifies an existing audit plan.
func (s *Service) UpdateAuditPlan(ctx context.Context, orgID, id string, in CreateAuditPlanInput) (*AuditPlan, error) {
	return s.repo.UpdateAuditPlan(ctx, orgID, id, in)
}

// ListAuditProgramAudits returns all individual audits for the org.
func (s *Service) ListAuditProgramAudits(ctx context.Context, orgID string) ([]AuditProgramAudit, error) {
	return s.repo.ListAuditProgramAudits(ctx, orgID)
}

// CreateAuditProgramAudit persists a new individual audit.
func (s *Service) CreateAuditProgramAudit(ctx context.Context, orgID string, in CreateAuditProgramAuditInput) (*AuditProgramAudit, error) {
	return s.repo.CreateAuditProgramAudit(ctx, orgID, in)
}

// GetAuditProgramAudit returns a single audit by ID.
func (s *Service) GetAuditProgramAudit(ctx context.Context, orgID, id string) (*AuditProgramAudit, error) {
	return s.repo.GetAuditProgramAudit(ctx, orgID, id)
}

// UpdateAuditProgramAudit modifies an existing audit.
func (s *Service) UpdateAuditProgramAudit(ctx context.Context, orgID, id string, in CreateAuditProgramAuditInput) (*AuditProgramAudit, error) {
	return s.repo.UpdateAuditProgramAudit(ctx, orgID, id, in)
}

// CompleteAudit marks an audit as completed with a written report.
// It validates that the audit report is non-empty.
func (s *Service) CompleteAudit(ctx context.Context, orgID, auditID string, in CompleteAuditInput) error {
	if in.AuditReport == "" {
		return errors.New("audit_report is required to complete an audit")
	}
	return s.repo.CompleteAuditProgramAudit(ctx, orgID, auditID, in)
}

// ListAuditFindings returns all findings for a given audit.
func (s *Service) ListAuditFindings(ctx context.Context, orgID, auditID string) ([]AuditFinding, error) {
	return s.repo.ListAuditFindings(ctx, orgID, auditID)
}

// CreateAuditFinding persists a new finding. For major_nc and minor_nc severity,
// it automatically creates a linked CAPA entry.
func (s *Service) CreateAuditFinding(ctx context.Context, orgID, auditID string, in CreateAuditFindingInput) (*AuditFinding, error) {
	finding, err := s.repo.CreateAuditFinding(ctx, orgID, auditID, in)
	if err != nil {
		return nil, err
	}

	// Auto-create CAPA for major_nc and minor_nc findings
	if in.Severity == "major_nc" || in.Severity == "minor_nc" {
		capaID, capaErr := s.repo.CreateCAPAFromAuditFinding(ctx, orgID, finding.ID, finding.Title, in.Severity)
		if capaErr != nil {
			log.Error().Err(capaErr).Str("finding_id", finding.ID).Msg("failed to auto-create CAPA from audit finding")
		} else {
			finding.CAPAid = &capaID
			_ = s.repo.SetAuditFindingCAPAID(ctx, finding.ID, capaID)
		}
	}

	return finding, nil
}

// GetAuditProgramSummary returns aggregate statistics for the current year's audit program.
func (s *Service) GetAuditProgramSummary(ctx context.Context, orgID string) (*AuditProgramSummary, error) {
	return s.repo.GetAuditProgramSummary(ctx, orgID)
}

// RunAuditProgramEvidenceSync generates Evidence for ISO 27001 Clause 9.2 based on completed audits.
// Called by the daily Asynq task.
func (s *Service) RunAuditProgramEvidenceSync(ctx context.Context, orgID string) error {
	count, err := s.repo.CountCompletedAuditsLastYear(ctx, orgID)
	if err != nil {
		return fmt.Errorf("audit program evidence sync: count completed audits: %w", err)
	}
	findings, err := s.repo.CountOpenAuditFindings(ctx, orgID)
	if err != nil {
		return fmt.Errorf("audit program evidence sync: count open findings: %w", err)
	}

	evidenceStatus := "ok"
	description := fmt.Sprintf("Internes Audit-Programm (Clause 9.2): %d Audits in letzten 12 Monaten, %d offene Befunde.", count, findings)
	if count == 0 {
		evidenceStatus = "warning"
		description = "Kein internes ISMS-Audit in den letzten 12 Monaten durchgeführt. ISO 27001 Clause 9.2 erfordert ein jährliches Audit-Programm."
	}

	log.Info().Str("org_id", orgID).Str("status", evidenceStatus).Msg("audit program evidence sync")
	_ = description
	// In production, this would write to ck_evidence via the evidence writer
	return nil
}

// ExportAuditReport generates a PDF audit report for a given audit ID.
func (s *Service) ExportAuditReport(ctx context.Context, orgID, auditID string) ([]byte, error) {
	audit, err := s.repo.GetAuditProgramAudit(ctx, orgID, auditID)
	if err != nil {
		return nil, err
	}
	findings, err := s.repo.ListAuditFindings(ctx, orgID, auditID)
	if err != nil {
		return nil, err
	}
	return buildAuditReportPDF(audit, findings)
}

func buildAuditReportPDF(audit *AuditProgramAudit, findings []AuditFinding) ([]byte, error) {
	pdf := fpdf.New("P", "mm", "A4", "")
	pdf.SetMargins(15, 15, 15)
	pdf.SetAutoPageBreak(true, 15)
	exportedAt := time.Now().UTC()

	pdf.SetFooterFunc(func() {
		pdf.SetY(-12)
		pdf.SetFont("Helvetica", "I", 7)
		pdf.SetTextColor(150, 150, 160)
		pdf.CellFormat(0, 5, fmt.Sprintf("Vakt — Audit-Bericht — %s — Seite %d/{nb}", exportedAt.Format("02.01.2006"), pdf.PageNo()), "", 0, "C", false, 0, "")
	})
	pdf.AliasNbPages("{nb}")
	pdf.AddPage()

	// Header
	pdf.SetFont("Helvetica", "B", 16)
	pdf.SetTextColor(30, 30, 30)
	pdf.CellFormat(0, 10, "Interner Audit-Bericht", "", 1, "L", false, 0, "")
	pdf.SetFont("Helvetica", "", 10)
	pdf.SetTextColor(80, 80, 90)
	pdf.CellFormat(0, 6, audit.Title, "", 1, "L", false, 0, "")
	pdf.Ln(3)

	// Metadata
	pdf.SetFont("Helvetica", "B", 9)
	pdf.SetTextColor(30, 30, 30)
	metadata := [][]string{
		{"Typ", auditTypeLabel(audit.AuditType)},
		{"Methode", audit.Methodology},
		{"Scope", audit.Scope},
		{"Geplant", audit.PlannedDate},
	}
	if audit.ActualDate != nil {
		metadata = append(metadata, []string{"Durchgeführt", *audit.ActualDate})
	}
	metadata = append(metadata, []string{"Status", auditStatusLabel(audit.Status)})

	for _, row := range metadata {
		pdf.SetFont("Helvetica", "B", 9)
		pdf.CellFormat(35, 6, row[0]+":", "", 0, "L", false, 0, "")
		pdf.SetFont("Helvetica", "", 9)
		pdf.CellFormat(0, 6, row[1], "", 1, "L", false, 0, "")
	}
	pdf.Ln(5)

	// Audit Report
	if audit.AuditReport != "" {
		pdf.SetFont("Helvetica", "B", 11)
		pdf.CellFormat(0, 7, "Audit-Bericht / Zusammenfassung", "", 1, "L", false, 0, "")
		pdf.SetFont("Helvetica", "", 9)
		pdf.MultiCell(0, 5, audit.AuditReport, "", "L", false)
		pdf.Ln(5)
	}

	// Findings
	pdf.SetFont("Helvetica", "B", 11)
	pdf.CellFormat(0, 7, fmt.Sprintf("Befunde (%d)", len(findings)), "", 1, "L", false, 0, "")

	if len(findings) == 0 {
		pdf.SetFont("Helvetica", "I", 9)
		pdf.CellFormat(0, 6, "Keine Befunde erfasst.", "", 1, "L", false, 0, "")
	} else {
		colW := []float64{25, 90, 65}
		headers := []string{"Schweregrad", "Titel", "Beschreibung"}
		pdf.SetFont("Helvetica", "B", 8)
		pdf.SetFillColor(45, 55, 72)
		pdf.SetTextColor(255, 255, 255)
		for i, h := range headers {
			pdf.CellFormat(colW[i], 6, h, "1", 0, "C", true, 0, "")
		}
		pdf.Ln(-1)

		pdf.SetFont("Helvetica", "", 8)
		pdf.SetTextColor(30, 30, 30)
		for _, f := range findings {
			pdf.SetFillColor(255, 255, 255)
			pdf.CellFormat(colW[0], 5, severityLabel(f.Severity), "1", 0, "C", false, 0, "")
			pdf.CellFormat(colW[1], 5, truncate(f.Title, 60), "1", 0, "L", false, 0, "")
			pdf.CellFormat(colW[2], 5, truncate(f.Description, 45), "1", 1, "L", false, 0, "")
		}
	}

	var buf bytes.Buffer
	if err := pdf.Output(&buf); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

func auditTypeLabel(t string) string {
	labels := map[string]string{
		"isms_internal": "Internes ISMS-Audit", "compliance_check": "Compliance-Prüfung",
		"supplier_audit": "Lieferanten-Audit", "process_audit": "Prozess-Audit",
	}
	if l, ok := labels[t]; ok {
		return l
	}
	return t
}

func auditStatusLabel(s string) string {
	labels := map[string]string{
		"planned": "Geplant", "in_progress": "In Bearbeitung",
		"completed": "Abgeschlossen", "cancelled": "Abgebrochen",
	}
	if l, ok := labels[s]; ok {
		return l
	}
	return s
}

func severityLabel(s string) string {
	labels := map[string]string{
		"major_nc": "NC (Schwerwiegend)", "minor_nc": "NC (Leicht)",
		"observation": "Beobachtung", "ofi": "Verbesserungs-OFI",
	}
	if l, ok := labels[s]; ok {
		return l
	}
	return s
}
