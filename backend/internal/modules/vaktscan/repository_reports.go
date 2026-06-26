// Copyright (c) 2026 NorvikOps. All rights reserved.
// SPDX-License-Identifier: Elastic-2.0

package vaktscan

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5/pgtype"

	"github.com/matharnica/vakt/internal/db"
)

// ---------------------------------------------------------------------------
// Reports
// ---------------------------------------------------------------------------

// CreateReport inserts a new report record.
func (r *Repository) CreateReport(ctx context.Context, orgID, userID string, scope ReportScope) (*Report, error) {
	scopeJSON, err := json.Marshal(scope)
	if err != nil {
		return nil, fmt.Errorf("marshal report scope: %w", err)
	}
	row, err := r.q.CreateSPReport(ctx, db.CreateSPReportParams{
		OrgID:       orgID,
		GeneratedBy: spOptUUID(&userID),
		Scope:       scopeJSON,
	})
	if err != nil {
		return nil, fmt.Errorf("insert report: %w", err)
	}
	rpt := reportFromFields(reportFields{
		ID: row.ID, OrgID: row.OrgID, GeneratedBy: row.GeneratedBy,
		Scope: row.Scope, FilePath: row.FilePath, Status: row.Status,
		ExpiresAt: row.ExpiresAt, CreatedAt: row.CreatedAt,
	})
	return &rpt, nil
}

// GetReport fetches a report by ID within the org.
func (r *Repository) GetReport(ctx context.Context, orgID, reportID string) (*Report, error) {
	row, err := r.q.GetSPReport(ctx, db.GetSPReportParams{ID: reportID, OrgID: orgID})
	if err != nil {
		return nil, fmt.Errorf("get report: %w", err)
	}
	rpt := reportFromFields(reportFields{
		ID: row.ID, OrgID: row.OrgID, GeneratedBy: row.GeneratedBy,
		Scope: row.Scope, FilePath: row.FilePath, Status: row.Status,
		ExpiresAt: row.ExpiresAt, CreatedAt: row.CreatedAt,
	})
	return &rpt, nil
}

// ListReports returns reports for an org, newest first (metadata only — no PDF blob).
func (r *Repository) ListReports(ctx context.Context, orgID string) ([]Report, error) {
	rows, err := r.q.ListSPReports(ctx, orgID)
	if err != nil {
		return nil, fmt.Errorf("list reports: %w", err)
	}
	out := make([]Report, 0, len(rows))
	for _, row := range rows {
		out = append(out, reportFromFields(reportFields{
			ID: row.ID, OrgID: row.OrgID, GeneratedBy: row.GeneratedBy,
			Scope: row.Scope, FilePath: row.FilePath, Status: row.Status,
			ExpiresAt: row.ExpiresAt, CreatedAt: row.CreatedAt,
		}))
	}
	return out, nil
}

// UpsertFindingByRawID inserts a finding or updates it on conflict of
// (org_id, raw_id, scanner). This is used for import operations (SARIF, CycloneDX, CSV).
func (r *Repository) UpsertFindingByRawID(ctx context.Context, orgID string, f Finding) (*Finding, error) {
	sources := f.Sources
	if sources == nil {
		sources = []string{}
	}
	row, err := r.q.UpsertSPFindingByRawID(ctx, db.UpsertSPFindingByRawIDParams{
		OrgID:       orgID,
		AssetID:     f.AssetID,
		CveID:       optTextPtr(f.CVEID),
		Title:       f.Title,
		Description: spOptText(f.Description),
		Severity:    f.Severity,
		CvssScore:   float64PtrToNumeric(f.CVSSScore),
		Status:      f.Status,
		Scanner:     f.Scanner,
		RawID:       spOptText(f.RawID),
		Sources:     sources,
		SlaDueAt:    spOptTs(f.SLADueAt),
	})
	if err != nil {
		return nil, fmt.Errorf("upsert finding by raw_id: %w", err)
	}
	out := findingFromVbFindings(row)
	return &out, nil
}

// UpdateReport updates a report's file path, status, and expiry.
func (r *Repository) UpdateReport(ctx context.Context, reportID, filePath, status string, expiresAt *time.Time) error {
	err := r.q.UpdateSPReport(ctx, db.UpdateSPReportParams{
		ID:        reportID,
		FilePath:  spOptText(filePath),
		Status:    status,
		ExpiresAt: spOptTs(expiresAt),
	})
	if err != nil {
		return fmt.Errorf("update report: %w", err)
	}
	return nil
}

// StoreReportContent saves a generated PDF and marks the report completed.
func (r *Repository) StoreReportContent(ctx context.Context, reportID string, content []byte, expiresAt time.Time) error {
	err := r.q.StoreSPReportContent(ctx, db.StoreSPReportContentParams{
		ID:        reportID,
		Content:   content,
		ExpiresAt: pgtype.Timestamptz{Time: expiresAt, Valid: true},
	})
	if err != nil {
		return fmt.Errorf("store report content: %w", err)
	}
	return nil
}

// GetReportContent returns the raw PDF bytes and title for a completed report.
func (r *Repository) GetReportContent(ctx context.Context, orgID, reportID string) ([]byte, string, error) {
	row, err := r.q.GetSPReportContent(ctx, db.GetSPReportContentParams{ID: reportID, OrgID: orgID})
	if err != nil {
		return nil, "", fmt.Errorf("get report content: %w", err)
	}
	var scope ReportScope
	if len(row.Scope) > 0 {
		_ = json.Unmarshal(row.Scope, &scope)
	}
	title := scope.Title
	if title == "" {
		title = "report"
	}
	return row.Content, title, nil
}
