// Copyright (c) 2026 NorvikOps. All rights reserved.
// SPDX-License-Identifier: Elastic-2.0

// S74-4: BSI Referenzberichte A1–A6 — Service Layer

package bsi

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"time"
)

// ListBSIReportExports returns all report export audit log entries for an org.
func (s *Service) ListBSIReportExports(ctx context.Context, orgID string) ([]BSIReportExport, error) {
	rows, err := s.db.Query(ctx, `
		SELECT id, org_id, report_type, generated_by, generated_at,
		       sha256, file_size_bytes, metadata
		FROM ck_bsi_report_exports
		WHERE org_id=$1
		ORDER BY generated_at DESC
		LIMIT 100`, orgID)
	if err != nil {
		return nil, fmt.Errorf("list bsi report exports: %w", err)
	}
	defer rows.Close()

	var out []BSIReportExport
	for rows.Next() {
		var e BSIReportExport
		if err := rows.Scan(&e.ID, &e.OrgID, &e.ReportType, &e.GeneratedBy,
			&e.GeneratedAt, &e.SHA256, &e.FileSizeBytes, &e.Metadata); err != nil {
			return nil, fmt.Errorf("scan bsi report export: %w", err)
		}
		out = append(out, e)
	}
	if out == nil {
		out = []BSIReportExport{}
	}
	return out, rows.Err()
}

// GenerateBSIReport generates a PDF report of the given type, stores the audit log entry,
// and returns the PDF bytes.
func (s *Service) GenerateBSIReport(ctx context.Context, orgID, userID, reportType string) ([]byte, error) {
	renderer := NewBSIReportRenderer(s.db, orgID)

	var data []byte
	var err error
	switch reportType {
	case "A1":
		data, err = renderer.RenderA1(ctx)
	case "A2":
		data, err = renderer.RenderA2(ctx)
	case "A3":
		data, err = renderer.RenderA3(ctx)
	case "A4":
		data, err = renderer.RenderA4(ctx)
	case "A5":
		data, err = renderer.RenderA5(ctx)
	case "A6":
		data, err = renderer.RenderA6(ctx)
	case "full":
		data, err = renderer.RenderFull(ctx)
	default:
		return nil, fmt.Errorf("unknown report type: %s", reportType)
	}
	if err != nil {
		return nil, fmt.Errorf("render %s: %w", reportType, err)
	}

	// Compute SHA-256 for audit log.
	sum := sha256.Sum256(data)
	hashStr := hex.EncodeToString(sum[:])
	size := len(data)

	var generatedByArg any
	if userID != "" {
		generatedByArg = userID
	}

	_, insertErr := s.db.Exec(ctx, `
		INSERT INTO ck_bsi_report_exports
		  (org_id, report_type, generated_by, generated_at, sha256, file_size_bytes, metadata)
		VALUES ($1,$2,$3,$4,$5,$6,$7)`,
		orgID, reportType, generatedByArg, time.Now().UTC(), hashStr, size,
		map[string]any{"generated_at": time.Now().UTC().Format(time.RFC3339)})
	if insertErr != nil {
		// Non-fatal — still return the PDF.
		_ = insertErr
	}

	return data, nil
}

// LogBCMReportExport records a SHA-256 audit entry for the Notfallhandbuch PDF export.
// Errors are non-fatal — the download already succeeded by the time this is called.
func (s *Service) LogBCMReportExport(ctx context.Context, orgID, userID string, data []byte) {
	sum := sha256.Sum256(data)
	hashStr := hex.EncodeToString(sum[:])
	var generatedByArg any
	if userID != "" {
		generatedByArg = userID
	}
	_, _ = s.db.Exec(ctx, `
		INSERT INTO ck_bsi_report_exports
		  (org_id, report_type, generated_by, generated_at, sha256, file_size_bytes, metadata)
		VALUES ($1,$2,$3,$4,$5,$6,$7)`,
		orgID, "bcm_notfallhandbuch", generatedByArg, time.Now().UTC(), hashStr, len(data),
		map[string]any{"generated_at": time.Now().UTC().Format(time.RFC3339)})
}

// GetBSIReportPreview returns JSON preview data for a report type (before PDF download).
func (s *Service) GetBSIReportPreview(ctx context.Context, orgID, reportType string) (map[string]any, error) {
	switch reportType {
	case "A2", "full":
		svc := NewService(s.db)
		objects, err := svc.ListBSITargetObjects(ctx, orgID)
		if err != nil {
			return nil, err
		}
		return map[string]any{
			"report_type":       reportType,
			"zielobjekt_count":  len(objects),
			"preview_available": true,
		}, nil
	case "A5":
		var count int
		_ = s.db.QueryRow(ctx, `SELECT COUNT(*) FROM ck_bsi_check_results WHERE org_id=$1`, orgID).Scan(&count)
		return map[string]any{
			"report_type":       reportType,
			"anforderung_count": count,
			"preview_available": true,
		}, nil
	case "A6":
		var count int
		_ = s.db.QueryRow(ctx, `SELECT COUNT(*) FROM ck_bsi_risk_assessments WHERE org_id=$1`, orgID).Scan(&count)
		return map[string]any{
			"report_type":       reportType,
			"risk_count":        count,
			"preview_available": true,
		}, nil
	default:
		return map[string]any{
			"report_type":       reportType,
			"preview_available": false,
		}, nil
	}
}
