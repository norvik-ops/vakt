// Copyright (c) 2026 NorvikOps. All rights reserved.
// SPDX-License-Identifier: Elastic-2.0

package vaktcomply

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/go-pdf/fpdf"
	"github.com/rs/zerolog/log"
)

// ErrSoANotInitialized is returned when no dedicated SoA exists for the org.
var ErrSoANotInitialized = errors.New("SoA not initialized — call POST /soa/init first")

// ErrExclusionReasonRequired is returned when applicable=false entries lack an exclusion_reason.
var ErrExclusionReasonRequired = errors.New("all excluded controls must have an exclusion_reason")

// InitDedicatedSoA creates version 1 with all 93 Annex A controls for the org.
// It is idempotent — calling it again when entries exist is a no-op.
func (s *Service) InitDedicatedSoA(ctx context.Context, orgID string) error {
	exists, err := s.repo.HasDedicatedSoA(ctx, orgID)
	if err != nil {
		return fmt.Errorf("check dedicated SoA: %w", err)
	}
	if exists {
		return nil
	}
	if err := s.repo.CreateSoAVersion(ctx, orgID, 1); err != nil {
		return fmt.Errorf("create SoA version: %w", err)
	}
	if err := s.repo.InitSoAEntries(ctx, orgID, 1, iso27001AnnexAControls); err != nil {
		return fmt.Errorf("init SoA entries: %w", err)
	}
	log.Info().Str("org_id", orgID).Int("controls", len(iso27001AnnexAControls)).Msg("dedicated SoA initialized")
	return nil
}

// ListDedicatedSoAEntries returns entries for the org's current (highest) version.
func (s *Service) ListDedicatedSoAEntries(ctx context.Context, orgID string) ([]SoADedicatedEntry, error) {
	version, err := s.repo.GetCurrentSoAVersion(ctx, orgID)
	if err != nil {
		return nil, err
	}
	if version == 0 {
		return nil, ErrSoANotInitialized
	}
	return s.repo.ListSoAEntries(ctx, orgID, version)
}

// GetDedicatedSoAEntry returns a single entry by control_ref at the current version.
func (s *Service) GetDedicatedSoAEntry(ctx context.Context, orgID, controlRef string) (*SoADedicatedEntry, error) {
	version, err := s.repo.GetCurrentSoAVersion(ctx, orgID)
	if err != nil {
		return nil, err
	}
	if version == 0 {
		return nil, ErrSoANotInitialized
	}
	return s.repo.GetSoAEntry(ctx, orgID, controlRef, version)
}

// UpdateDedicatedSoAEntry updates a single entry. Returns an error if the current version is approved.
func (s *Service) UpdateDedicatedSoAEntry(ctx context.Context, orgID, controlRef string, in UpdateSoAEntryInput) error {
	version, err := s.repo.GetCurrentSoAVersion(ctx, orgID)
	if err != nil {
		return err
	}
	if version == 0 {
		return ErrSoANotInitialized
	}

	// If not applicable, exclusion_reason is required for save
	if !in.Applicable && strings.TrimSpace(in.ExclusionReason) == "" {
		return errors.New("exclusion_reason is required when applicable = false")
	}

	// If current version is approved, create a new draft first
	versions, err := s.repo.ListSoAVersions(ctx, orgID)
	if err != nil {
		return err
	}
	if len(versions) > 0 && versions[0].Status == "approved" {
		newVersion := version + 1
		if err := s.repo.CreateSoAVersion(ctx, orgID, newVersion); err != nil {
			return fmt.Errorf("create draft version: %w", err)
		}
		if err := s.repo.CopyVersionEntries(ctx, orgID, version, newVersion); err != nil {
			return fmt.Errorf("copy version entries: %w", err)
		}
		version = newVersion
	}

	return s.repo.UpdateSoAEntry(ctx, orgID, controlRef, version, in)
}

// ApproveDedicatedSoA approves the current draft version.
// Returns an error if any excluded entry lacks an exclusion_reason.
func (s *Service) ApproveDedicatedSoA(ctx context.Context, orgID, approverID string) error {
	version, err := s.repo.GetCurrentSoAVersion(ctx, orgID)
	if err != nil {
		return err
	}
	if version == 0 {
		return ErrSoANotInitialized
	}

	missing, err := s.repo.CountExcludedWithoutReason(ctx, orgID, version)
	if err != nil {
		return err
	}
	if missing > 0 {
		return fmt.Errorf("%w: %d excluded controls have no reason", ErrExclusionReasonRequired, missing)
	}

	return s.repo.ApproveSoAVersion(ctx, orgID, version, approverID)
}

// GetDedicatedSoAVersions returns all versions for the org.
func (s *Service) GetDedicatedSoAVersions(ctx context.Context, orgID string) ([]SoAVersion, error) {
	return s.repo.ListSoAVersions(ctx, orgID)
}

// GetDedicatedSoASummary returns aggregate statistics for the current version.
func (s *Service) GetDedicatedSoASummary(ctx context.Context, orgID string) (*SoASummary, error) {
	version, err := s.repo.GetCurrentSoAVersion(ctx, orgID)
	if err != nil {
		return nil, err
	}
	if version == 0 {
		return &SoASummary{}, nil
	}
	return s.repo.GetSoASummary(ctx, orgID, version)
}

// ExportDedicatedSoAPDF generates a PDF for the current (or specified) version.
func (s *Service) ExportDedicatedSoAPDF(ctx context.Context, orgID string) ([]byte, error) {
	version, err := s.repo.GetCurrentSoAVersion(ctx, orgID)
	if err != nil {
		return nil, err
	}
	if version == 0 {
		return nil, ErrSoANotInitialized
	}
	entries, err := s.repo.ListSoAEntries(ctx, orgID, version)
	if err != nil {
		return nil, err
	}
	summary, err := s.repo.GetSoASummary(ctx, orgID, version)
	if err != nil {
		return nil, err
	}
	return buildSoAPDF(entries, summary, version)
}

// ExportDedicatedSoACSV generates a CSV for the current version.
func (s *Service) ExportDedicatedSoACSV(ctx context.Context, orgID string) ([][]string, error) {
	version, err := s.repo.GetCurrentSoAVersion(ctx, orgID)
	if err != nil {
		return nil, err
	}
	if version == 0 {
		return nil, ErrSoANotInitialized
	}
	entries, err := s.repo.ListSoAEntries(ctx, orgID, version)
	if err != nil {
		return nil, err
	}

	rows := [][]string{
		{"Control-ID", "Control-Name", "Gruppe", "Anwendbar", "Begründung", "Ausschluss-Begründung", "Status", "Evidence-Referenz"},
	}
	for _, e := range entries {
		applicable := "Ja"
		if !e.Applicable {
			applicable = "Nein"
		}
		rows = append(rows, []string{
			e.ControlRef, e.ControlName, "A." + e.ControlGroup,
			applicable, e.Justification, e.ExclusionReason,
			e.ImplementationStatus, e.EvidenceReference,
		})
	}
	return rows, nil
}

// buildSoAPDF renders a PDF with all SoA entries.
func buildSoAPDF(entries []SoADedicatedEntry, summary *SoASummary, version int) ([]byte, error) {
	pdf := fpdf.NewCustom(&fpdf.InitType{
		OrientationStr: "L",
		UnitStr:        "mm",
		SizeStr:        "A4",
	})
	pdf.SetMargins(10, 12, 10)
	pdf.SetAutoPageBreak(true, 12)

	exportedAt := time.Now().UTC()

	pdf.SetFooterFunc(func() {
		pdf.SetY(-10)
		pdf.SetFont("Helvetica", "I", 7)
		pdf.SetTextColor(150, 150, 160)
		pdf.CellFormat(0, 5, fmt.Sprintf("Vakt — Statement of Applicability v%d — %s — Seite %d/{nb}",
			version, exportedAt.Format("02.01.2006"), pdf.PageNo()), "", 0, "C", false, 0, "")
	})
	pdf.AliasNbPages("{nb}")

	// Cover page
	pdf.AddPage()
	pdf.SetFont("Helvetica", "B", 20)
	pdf.SetTextColor(30, 30, 30)
	pdf.SetY(60)
	pdf.CellFormat(0, 12, "Statement of Applicability", "", 1, "C", false, 0, "")
	pdf.SetFont("Helvetica", "", 13)
	pdf.CellFormat(0, 8, fmt.Sprintf("Version %d — ISO 27001:2022 Annex A", version), "", 1, "C", false, 0, "")
	pdf.SetFont("Helvetica", "", 10)
	pdf.SetTextColor(100, 100, 110)
	pdf.CellFormat(0, 7, fmt.Sprintf("Erstellt: %s", exportedAt.Format("02. January 2006")), "", 1, "C", false, 0, "")
	if summary.Status == "approved" {
		pdf.CellFormat(0, 7, "Status: Genehmigt", "", 1, "C", false, 0, "")
	} else {
		pdf.CellFormat(0, 7, "Status: Entwurf", "", 1, "C", false, 0, "")
	}

	// Statistics page
	pdf.AddPage()
	pdf.SetFont("Helvetica", "B", 12)
	pdf.SetTextColor(30, 30, 30)
	pdf.CellFormat(0, 8, "Übersicht", "", 1, "L", false, 0, "")
	pdf.SetFont("Helvetica", "", 10)
	pdf.CellFormat(60, 6, fmt.Sprintf("Controls gesamt: %d", summary.ApplicableCount+summary.ExcludedCount), "", 1, "L", false, 0, "")
	pdf.CellFormat(60, 6, fmt.Sprintf("Anwendbar: %d", summary.ApplicableCount), "", 1, "L", false, 0, "")
	pdf.CellFormat(60, 6, fmt.Sprintf("Nicht anwendbar: %d", summary.ExcludedCount), "", 1, "L", false, 0, "")
	pdf.CellFormat(60, 6, fmt.Sprintf("Implementiert: %d", summary.ImplementedCount), "", 1, "L", false, 0, "")
	pdf.CellFormat(60, 6, fmt.Sprintf("Teilweise: %d", summary.PartialCount), "", 1, "L", false, 0, "")
	pdf.CellFormat(60, 6, fmt.Sprintf("Geplant: %d", summary.PlannedCount), "", 1, "L", false, 0, "")
	pdf.CellFormat(60, 6, fmt.Sprintf("Nicht begonnen: %d", summary.NotStartedCount), "", 1, "L", false, 0, "")
	pdf.CellFormat(60, 6, fmt.Sprintf("Implementierungsgrad: %.0f%%", summary.ImplementationPct), "", 1, "L", false, 0, "")

	// Controls table
	pdf.AddPage()
	colWidths := []float64{18, 80, 12, 50, 40, 35, 42}
	headers := []string{"Control", "Name", "Applic.", "Begründung / Ausschluss", "Status", "Evidence-Referenz", "Gruppe"}

	renderHeader := func() {
		pdf.SetFont("Helvetica", "B", 7)
		pdf.SetFillColor(45, 55, 72)
		pdf.SetTextColor(255, 255, 255)
		for i, h := range headers {
			pdf.CellFormat(colWidths[i], 6, h, "1", 0, "C", true, 0, "")
		}
		pdf.Ln(-1)
	}
	renderHeader()

	pdf.SetFont("Helvetica", "", 7)
	pdf.SetTextColor(30, 30, 30)

	groupLabels := map[string]string{"5": "5 — Organisatorisch", "6": "6 — Personal", "7": "7 — Physisch", "8": "8 — Technologisch"}
	currentGroup := ""

	for _, e := range entries {
		if e.ControlGroup != currentGroup {
			currentGroup = e.ControlGroup
			pdf.SetFont("Helvetica", "B", 7)
			pdf.SetFillColor(240, 242, 245)
			pdf.SetTextColor(30, 30, 30)
			label := groupLabels[e.ControlGroup]
			pdf.CellFormat(sum(colWidths), 5, "  "+label, "1", 1, "L", true, 0, "")
			pdf.SetFont("Helvetica", "", 7)
		}
		applicable := "Ja"
		if !e.Applicable {
			applicable = "Nein"
		}
		justText := e.Justification
		if !e.Applicable {
			justText = e.ExclusionReason
		}
		statusLabel := statusLabel(e.ImplementationStatus)
		if !e.Applicable {
			statusLabel = "N/A"
		}

		pdf.SetFillColor(255, 255, 255)
		pdf.CellFormat(colWidths[0], 5, e.ControlRef, "1", 0, "L", false, 0, "")
		pdf.CellFormat(colWidths[1], 5, truncate(e.ControlName, 55), "1", 0, "L", false, 0, "")
		pdf.CellFormat(colWidths[2], 5, applicable, "1", 0, "C", false, 0, "")
		pdf.CellFormat(colWidths[3], 5, truncate(justText, 40), "1", 0, "L", false, 0, "")
		pdf.CellFormat(colWidths[4], 5, statusLabel, "1", 0, "C", false, 0, "")
		pdf.CellFormat(colWidths[5], 5, truncate(e.EvidenceReference, 28), "1", 0, "L", false, 0, "")
		pdf.CellFormat(colWidths[6], 5, groupLabels[e.ControlGroup], "1", 1, "L", false, 0, "")

		if pdf.GetY() > 185 {
			pdf.AddPage()
			renderHeader()
		}
	}

	var buf bytes.Buffer
	if err := pdf.Output(&buf); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

func sum(widths []float64) float64 {
	var total float64
	for _, w := range widths {
		total += w
	}
	return total
}

func truncate(s string, max int) string {
	if len(s) <= max {
		return s
	}
	return s[:max-1] + "…"
}

func statusLabel(s string) string {
	switch s {
	case "implemented":
		return "Implementiert"
	case "partial":
		return "Teilweise"
	case "planned":
		return "Geplant"
	default:
		return "Nicht begonnen"
	}
}
