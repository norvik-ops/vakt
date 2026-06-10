// Copyright (c) 2026 NorvikOps. All rights reserved.
// SPDX-License-Identifier: Elastic-2.0

package vaktprivacy

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	fpdf "github.com/go-pdf/fpdf"
	"github.com/rs/zerolog/log"

	"github.com/matharnica/vakt/internal/shared/notify"
)

// CheckOverdueRequests marks open DSRs as overdue when due_date < now,
// and sends notifications for requests due within 3 days.
// This runs daily via the privacy:dsr_deadline_check Asynq task.
func (s *Service) CheckOverdueRequests(ctx context.Context) error {
	// Mark all non-closed requests past their deadline as overdue
	_, err := s.db.Exec(ctx, `
		UPDATE po_dsr SET status = 'overdue', updated_at = NOW()
		WHERE status NOT IN ('completed', 'rejected', 'extended', 'overdue')
		  AND due_date < CURRENT_DATE`)
	if err != nil {
		return fmt.Errorf("mark overdue dsrs: %w", err)
	}

	// Notify orgs about requests due within 3 days
	rows, err := s.db.Query(ctx, `
		SELECT DISTINCT org_id FROM po_dsr
		WHERE status NOT IN ('completed', 'rejected', 'extended', 'overdue')
		  AND due_date BETWEEN CURRENT_DATE AND CURRENT_DATE + INTERVAL '3 days'`)
	if err != nil {
		return fmt.Errorf("query dsrs due soon: %w", err)
	}
	defer rows.Close()

	for rows.Next() {
		var orgID string
		if err := rows.Scan(&orgID); err != nil {
			continue
		}
		notify.Send(ctx, s.db, orgID,
			"DSR-Frist läuft bald ab",
			"Eine oder mehrere Betroffenenanfragen haben in weniger als 3 Tagen Frist. Bitte umgehend bearbeiten.",
			"dsr_due_soon", "vaktprivacy")
	}

	// Notify orgs about Art. 21 objections received today
	objRows, err := s.db.Query(ctx, `
		SELECT DISTINCT org_id FROM po_dsr
		WHERE type = 'objection' AND status = 'open'
		  AND received_at::date = CURRENT_DATE`)
	if err != nil {
		log.Error().Err(err).Msg("query new objections")
		return nil
	}
	defer objRows.Close()

	for objRows.Next() {
		var orgID string
		if err := objRows.Scan(&orgID); err != nil {
			continue
		}
		notify.Send(ctx, s.db, orgID,
			"Widerspruch nach Art. 21 eingegangen",
			"Ein Widerspruch (Art. 21 DSGVO) muss unverzüglich bearbeitet werden.",
			"dsr_objection_received", "vaktprivacy")
	}

	return nil
}

// ResolveDSR closes a DSR with a resolution type, notes, and optionally extends the deadline.
func (s *Service) ResolveDSR(ctx context.Context, orgID, id, resolvedByUserID string, in ResolveDSRInput) (*DSR, error) {
	if in.ResolutionType == "extended" && strings.TrimSpace(in.ExtensionReason) == "" {
		return nil, errors.New("extension_reason is required when resolution_type = 'extended' (DSGVO Art. 12 Abs. 3)")
	}

	// Compute new status and extension_due_at
	newStatus := in.ResolutionType // 'fulfilled' | 'rejected' | 'extended'
	if newStatus == "fulfilled" {
		newStatus = "completed"
	}

	var extDueAt *time.Time
	if in.ResolutionType == "extended" {
		// Fetch received_at to compute 90-day deadline from original receipt date
		var receivedAt time.Time
		if err := s.db.QueryRow(ctx, `SELECT received_at FROM po_dsr WHERE org_id = $1 AND id = $2`, orgID, id).Scan(&receivedAt); err != nil {
			return nil, fmt.Errorf("fetch dsr received_at: %w", err)
		}
		t := receivedAt.AddDate(0, 0, 90)
		extDueAt = &t
		newStatus = "extended"
	}

	// Update the record
	var row struct {
		ID, OrgID, RequesterName, RequesterEmail, Type, Status, Notes string
		DueDate                                                        *string
		ReceivedAt, CreatedAt, UpdatedAt                               time.Time
		CompletedAt, ExtensionDueAt                                    *time.Time
	}
	err := s.db.QueryRow(ctx, `
		UPDATE po_dsr SET
			status           = $1,
			notes            = COALESCE(NULLIF($2, ''), notes),
			extension_reason = NULLIF($3, ''),
			extension_due_at = $4,
			resolved_by      = $5,
			completed_at     = CASE WHEN $1 IN ('completed','rejected','extended') THEN NOW() ELSE completed_at END,
			updated_at       = NOW()
		WHERE org_id = $6 AND id = $7
		RETURNING id, org_id, requester_name, requester_email, type, status, notes,
		          to_char(due_date, 'YYYY-MM-DD'), received_at, completed_at,
		          extension_due_at, created_at, updated_at`,
		newStatus, in.ResolutionNotes, in.ExtensionReason, extDueAt, resolvedByUserID, orgID, id,
	).Scan(
		&row.ID, &row.OrgID, &row.RequesterName, &row.RequesterEmail, &row.Type, &row.Status, &row.Notes,
		&row.DueDate, &row.ReceivedAt, &row.CompletedAt, &row.ExtensionDueAt, &row.CreatedAt, &row.UpdatedAt,
	)
	if err != nil {
		return nil, fmt.Errorf("resolve dsr: %w", err)
	}

	dsr := &DSR{
		ID: row.ID, OrgID: row.OrgID,
		RequesterName: row.RequesterName, RequesterEmail: row.RequesterEmail,
		Type: row.Type, Status: row.Status, Notes: row.Notes,
		DueDate: row.DueDate, ReceivedAt: row.ReceivedAt, CompletedAt: row.CompletedAt,
		ExtensionDueAt: row.ExtensionDueAt,
		CreatedAt: row.CreatedAt, UpdatedAt: row.UpdatedAt,
	}
	return dsr, nil
}

// GetDSRSummary returns aggregate DSR statistics for the org.
func (s *Service) GetDSRSummary(ctx context.Context, orgID string) (*DSRSummary, error) {
	var summary DSRSummary
	err := s.db.QueryRow(ctx, `
		SELECT
			COUNT(*) FILTER (WHERE status IN ('open', 'in_progress'))           AS open_count,
			COUNT(*) FILTER (WHERE status = 'overdue')                          AS overdue_count,
			COUNT(*) FILTER (WHERE status IN ('completed','fulfilled')
			              AND completed_at > NOW() - INTERVAL '12 months')       AS fulfilled_12m,
			COUNT(*) FILTER (WHERE status = 'rejected'
			              AND completed_at > NOW() - INTERVAL '12 months')       AS rejected_12m
		FROM po_dsr WHERE org_id = $1`, orgID,
	).Scan(&summary.OpenCount, &summary.OverdueCount, &summary.FulfilledLast12M, &summary.RejectedLast12M)
	if err != nil {
		return nil, fmt.Errorf("get dsr summary: %w", err)
	}

	// On-time rate: fulfilled in time (completed_at <= due_date)
	var total, onTime int
	s.db.QueryRow(ctx, `
		SELECT
			COUNT(*),
			COUNT(*) FILTER (WHERE completed_at::date <= due_date)
		FROM po_dsr
		WHERE org_id = $1
		  AND status IN ('completed','fulfilled','rejected','extended')
		  AND completed_at > NOW() - INTERVAL '12 months'`, orgID,
	).Scan(&total, &onTime) //nolint:errcheck
	if total > 0 {
		summary.OnTimeRatePct = float64(onTime) / float64(total) * 100
	}

	return &summary, nil
}

// ExportDSRLogPDF generates an anonymised PDF audit log for the last N days.
func (s *Service) ExportDSRLogPDF(ctx context.Context, orgID string, periodDays int) ([]byte, error) {
	if periodDays <= 0 {
		periodDays = 365
	}

	rows, err := s.db.Query(ctx, `
		SELECT received_at, type, COALESCE(channel,''), status, COALESCE(reference_id,''),
		       COALESCE(due_date::text,''), COALESCE(completed_at::text,'')
		FROM po_dsr
		WHERE org_id = $1 AND received_at > NOW() - MAKE_INTERVAL(days => $2)
		ORDER BY received_at DESC`, orgID, periodDays)
	if err != nil {
		return nil, fmt.Errorf("query dsr log: %w", err)
	}
	defer rows.Close()

	type row struct {
		ReceivedAt  time.Time
		Type        string
		Channel     string
		Status      string
		ReferenceID string
		DueDate     string
		CompletedAt string
	}
	var records []row
	for rows.Next() {
		var r row
		if err := rows.Scan(&r.ReceivedAt, &r.Type, &r.Channel, &r.Status, &r.ReferenceID, &r.DueDate, &r.CompletedAt); err != nil {
			return nil, err
		}
		records = append(records, r)
	}

	pdf := fpdf.New("L", "mm", "A4", "")
	pdf.SetMargins(10, 12, 10)
	pdf.SetAutoPageBreak(true, 12)
	exportedAt := time.Now().UTC()

	pdf.SetFooterFunc(func() {
		pdf.SetY(-10)
		pdf.SetFont("Helvetica", "I", 7)
		pdf.SetTextColor(150, 150, 160)
		pdf.CellFormat(0, 5, fmt.Sprintf("Vakt Privacy — DSR-Verzeichnis — %s — Seite %d/{nb}", exportedAt.Format("02.01.2006"), pdf.PageNo()), "", 0, "C", false, 0, "")
	})
	pdf.AliasNbPages("{nb}")

	pdf.AddPage()
	pdf.SetFont("Helvetica", "B", 14)
	pdf.SetTextColor(30, 30, 30)
	pdf.CellFormat(0, 10, "DSR-Verzeichnis (Betroffenenrechte-Log)", "", 1, "L", false, 0, "")
	pdf.SetFont("Helvetica", "", 9)
	pdf.SetTextColor(100, 100, 110)
	pdf.CellFormat(0, 6, fmt.Sprintf("Zeitraum: letzte %d Tage — Erstellt: %s", periodDays, exportedAt.Format("02. January 2006 15:04 UTC")), "", 1, "L", false, 0, "")
	pdf.SetTextColor(150, 0, 0)
	pdf.CellFormat(0, 6, "Dieses Dokument enthält keine Personendaten des Antragstellers (Datensparsamkeit Art. 5 DSGVO).", "", 1, "L", false, 0, "")
	pdf.Ln(3)

	headers := []string{"Eingangsdatum", "Art (DSGVO)", "Kanal", "Status", "Referenz-Nr.", "Frist", "Abgeschlossen"}
	colW := []float64{30, 40, 25, 30, 30, 30, 32}
	pdf.SetFont("Helvetica", "B", 8)
	pdf.SetFillColor(45, 55, 72)
	pdf.SetTextColor(255, 255, 255)
	for i, h := range headers {
		pdf.CellFormat(colW[i], 6, h, "1", 0, "C", true, 0, "")
	}
	pdf.Ln(-1)

	pdf.SetFont("Helvetica", "", 8)
	pdf.SetTextColor(30, 30, 30)
	typeLabels := map[string]string{
		"access": "Auskunft (Art.15)", "rectification": "Berichtigung (Art.16)",
		"erasure": "Löschung (Art.17)", "restriction": "Einschränkung (Art.18)",
		"portability": "Portabilität (Art.20)", "objection": "Widerspruch (Art.21)",
		"no_profiling": "Kein Profiling (Art.22)",
	}
	statusLabels := map[string]string{
		"open": "Offen", "in_progress": "In Bearbeitung", "completed": "Erledigt",
		"rejected": "Abgelehnt", "extended": "Verlängert", "overdue": "Überfällig",
	}
	for _, r := range records {
		label := typeLabels[r.Type]
		if label == "" {
			label = r.Type
		}
		slabel := statusLabels[r.Status]
		if slabel == "" {
			slabel = r.Status
		}
		pdf.SetFillColor(255, 255, 255)
		pdf.CellFormat(colW[0], 5, r.ReceivedAt.Format("02.01.2006"), "1", 0, "L", false, 0, "")
		pdf.CellFormat(colW[1], 5, label, "1", 0, "L", false, 0, "")
		pdf.CellFormat(colW[2], 5, r.Channel, "1", 0, "L", false, 0, "")
		pdf.CellFormat(colW[3], 5, slabel, "1", 0, "L", false, 0, "")
		pdf.CellFormat(colW[4], 5, r.ReferenceID, "1", 0, "L", false, 0, "")
		pdf.CellFormat(colW[5], 5, r.DueDate, "1", 0, "L", false, 0, "")
		pdf.CellFormat(colW[6], 5, r.CompletedAt, "1", 1, "L", false, 0, "")
		if pdf.GetY() > 185 {
			pdf.AddPage()
			pdf.SetFont("Helvetica", "B", 8)
			pdf.SetFillColor(45, 55, 72)
			pdf.SetTextColor(255, 255, 255)
			for i, h := range headers {
				pdf.CellFormat(colW[i], 6, h, "1", 0, "C", true, 0, "")
			}
			pdf.Ln(-1)
			pdf.SetFont("Helvetica", "", 8)
			pdf.SetTextColor(30, 30, 30)
		}
	}

	var buf bytes.Buffer
	if err := pdf.Output(&buf); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}
