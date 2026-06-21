// Copyright (c) 2026 NorvikOps. All rights reserved.
// SPDX-License-Identifier: Elastic-2.0

package vaktcomply

import (
	"bytes"
	"context"
	"fmt"
	"github.com/matharnica/vakt/internal/modules/vaktcomply/policy"
	"time"

	"github.com/go-pdf/fpdf"
)

// defaultInterestedParties contains the 6 standard ISMS stakeholders for DACH organisations.
var defaultInterestedParties = []CreateInterestedPartyInput{
	{
		Name:         "Kunden",
		Category:     "customer",
		Requirements: "Vertraulichkeit und Integrität ihrer Daten; Verfügbarkeit der angebotenen Dienste",
	},
	{
		Name:         "Datenschutzbehörde / BSI",
		Category:     "regulator",
		Requirements: "Einhaltung DSGVO, NIS2 und branchenspezifischer Regulierung; Meldepflichten bei Vorfällen",
	},
	{
		Name:         "Geschäftsführung / Eigentümer",
		Category:     "shareholder",
		Requirements: "Risikominimierung, Geschäftskontinuität, Reputationsschutz; Return on Security Investment",
	},
	{
		Name:         "Mitarbeiter",
		Category:     "employee",
		Requirements: "Klare Sicherheitsrichtlinien; sichere Arbeitsumgebung; Datenschutz der eigenen Daten",
	},
	{
		Name:         "Lieferanten und Auftragnehmer",
		Category:     "supplier",
		Requirements: "Klare vertragliche Sicherheitsanforderungen; Auditrechte; Incident-Reporting-Pflichten",
	},
	{
		Name:         "Cyber-Versicherung",
		Category:     "insurer",
		Requirements: "Nachweis angemessener Sicherheitsmaßnahmen; Incident-Reporting innerhalb vereinbarter Fristen",
	},
}

// ListInterestedParties returns all interested parties for the org.
func (s *Service) ListInterestedParties(ctx context.Context, orgID string) ([]InterestedParty, error) {
	return s.repo.ListInterestedParties(ctx, orgID)
}

// CreateInterestedParty persists a new interested party entry.
func (s *Service) CreateInterestedParty(ctx context.Context, orgID string, in CreateInterestedPartyInput) (*InterestedParty, error) {
	return s.repo.CreateInterestedParty(ctx, orgID, in, false)
}

// UpdateInterestedParty modifies an existing entry.
func (s *Service) UpdateInterestedParty(ctx context.Context, orgID, id string, in CreateInterestedPartyInput) (*InterestedParty, error) {
	return s.repo.UpdateInterestedParty(ctx, orgID, id, in)
}

// DeleteInterestedParty removes an entry.
func (s *Service) DeleteInterestedParty(ctx context.Context, orgID, id string) error {
	return s.repo.DeleteInterestedParty(ctx, orgID, id)
}

// SeedDefaultInterestedParties inserts the 6 default stakeholders if the org has none.
// It is idempotent — calling it again when entries exist is a no-op.
func (s *Service) SeedDefaultInterestedParties(ctx context.Context, orgID string) error {
	count, err := s.repo.CountInterestedParties(ctx, orgID)
	if err != nil {
		return err
	}
	if count > 0 {
		return nil
	}
	for _, tmpl := range defaultInterestedParties {
		if _, err := s.repo.CreateInterestedParty(ctx, orgID, tmpl, true); err != nil {
			return fmt.Errorf("seed interested party %q: %w", tmpl.Name, err)
		}
	}
	return nil
}

// GetClause42Status returns true if Clause 4.2 is considered fulfilled (≥3 entries with requirements).
func (s *Service) GetClause42Status(ctx context.Context, orgID string) (bool, error) {
	return s.repo.CheckClause42Fulfilled(ctx, orgID)
}

// ExportInterestedPartiesPDF generates an audit-ready PDF of all interested parties.
func (s *Service) ExportInterestedPartiesPDF(ctx context.Context, orgID string) ([]byte, error) {
	parties, err := s.repo.ListInterestedParties(ctx, orgID)
	if err != nil {
		return nil, err
	}
	return buildInterestedPartiesPDF(parties)
}

func buildInterestedPartiesPDF(parties []InterestedParty) ([]byte, error) {
	pdf := fpdf.New("L", "mm", "A4", "")
	pdf.SetMargins(12, 12, 12)
	pdf.SetAutoPageBreak(true, 12)
	exportedAt := time.Now().UTC()

	pdf.SetFooterFunc(func() {
		pdf.SetY(-10)
		pdf.SetFont("Helvetica", "I", 7)
		pdf.SetTextColor(150, 150, 160)
		pdf.CellFormat(0, 5, fmt.Sprintf("Vakt — Interessierte Parteien (ISO 27001 Clause 4.2) — %s — Seite %d/{nb}", exportedAt.Format("02.01.2006"), pdf.PageNo()), "", 0, "C", false, 0, "")
	})
	pdf.AliasNbPages("{nb}")

	pdf.AddPage()
	pdf.SetFont("Helvetica", "B", 14)
	pdf.SetTextColor(30, 30, 30)
	pdf.CellFormat(0, 10, "Interessierte Parteien — ISO 27001 Clause 4.2", "", 1, "L", false, 0, "")
	pdf.SetFont("Helvetica", "", 9)
	pdf.SetTextColor(100, 100, 110)
	pdf.CellFormat(0, 6, fmt.Sprintf("Erstellt: %s", exportedAt.Format("02. January 2006")), "", 1, "L", false, 0, "")
	pdf.Ln(4)

	headers := []string{"Name", "Kategorie", "Anforderungen / Erwartungen", "Anliegen / Risiken", "Überprüfung"}
	colW := []float64{50, 35, 85, 70, 27}

	pdf.SetFont("Helvetica", "B", 8)
	pdf.SetFillColor(45, 55, 72)
	pdf.SetTextColor(255, 255, 255)
	for i, h := range headers {
		pdf.CellFormat(colW[i], 6, h, "1", 0, "C", true, 0, "")
	}
	pdf.Ln(-1)

	catLabels := map[string]string{
		"customer": "Kunden", "regulator": "Behörde", "employee": "Mitarbeiter",
		"shareholder": "Eigentümer", "supplier": "Lieferant", "insurer": "Versicherung",
		"it_provider": "IT-Dienstleister", "other": "Sonstige",
	}
	pdf.SetFont("Helvetica", "", 8)
	pdf.SetTextColor(30, 30, 30)

	for _, p := range parties {
		cat := catLabels[p.Category]
		if cat == "" {
			cat = p.Category
		}
		reviewDate := ""
		if p.ReviewDate != nil {
			reviewDate = *p.ReviewDate
		}
		pdf.SetFillColor(255, 255, 255)
		pdf.CellFormat(colW[0], 5, p.Name, "1", 0, "L", false, 0, "")
		pdf.CellFormat(colW[1], 5, cat, "1", 0, "L", false, 0, "")
		pdf.CellFormat(colW[2], 5, policy.Truncate(p.Requirements, 55), "1", 0, "L", false, 0, "")
		pdf.CellFormat(colW[3], 5, policy.Truncate(p.Concerns, 45), "1", 0, "L", false, 0, "")
		pdf.CellFormat(colW[4], 5, reviewDate, "1", 1, "C", false, 0, "")
	}

	var buf bytes.Buffer
	if err := pdf.Output(&buf); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}
