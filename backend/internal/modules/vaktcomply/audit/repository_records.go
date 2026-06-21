// Copyright (c) 2026 NorvikOps. All rights reserved.
// SPDX-License-Identifier: Elastic-2.0

package audit

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5/pgtype"

	"github.com/matharnica/vakt/internal/db"
)

// --- Internal Audit Records (FR-CK15) ---

// auditRecordFields is shared between Create/Get/List/Update Row types.
type auditRecordFields struct {
	ID, OrgID, Title, Scope, Auditor, Status, Findings, Recommendations string
	AuditDate                                                           pgtype.Date
	CreatedAt, UpdatedAt                                                pgtype.Timestamptz
}

func auditRecordFromFields(f auditRecordFields) AuditRecord {
	rec := AuditRecord{
		ID:              f.ID,
		OrgID:           f.OrgID,
		Title:           f.Title,
		Scope:           f.Scope,
		Auditor:         f.Auditor,
		Status:          f.Status,
		Findings:        f.Findings,
		Recommendations: f.Recommendations,
		CreatedAt:       ckTsToTime(f.CreatedAt),
		UpdatedAt:       ckTsToTime(f.UpdatedAt),
	}
	if f.AuditDate.Valid {
		rec.AuditDate = f.AuditDate.Time
	}
	return rec
}

func (r *Repository) ListAuditRecords(ctx context.Context, orgID string) ([]AuditRecord, error) {
	rows, err := r.q.ListCKAuditRecords(ctx, orgID)
	if err != nil {
		return nil, fmt.Errorf("list audit records: %w", err)
	}
	out := make([]AuditRecord, 0, len(rows))
	for _, row := range rows {
		out = append(out, auditRecordFromFields(auditRecordFields{
			ID: row.ID, OrgID: row.OrgID, Title: row.Title, Scope: row.Scope,
			Auditor: row.Auditor, AuditDate: row.AuditDate, Status: row.Status,
			Findings: row.Findings, Recommendations: row.Recommendations,
			CreatedAt: row.CreatedAt, UpdatedAt: row.UpdatedAt,
		}))
	}
	return out, nil
}

func (r *Repository) GetAuditRecord(ctx context.Context, orgID, id string) (*AuditRecord, error) {
	row, err := r.q.GetCKAuditRecord(ctx, db.GetCKAuditRecordParams{ID: id, OrgID: orgID})
	if err != nil {
		return nil, fmt.Errorf("get audit record: %w", err)
	}
	rec := auditRecordFromFields(auditRecordFields{
		ID: row.ID, OrgID: row.OrgID, Title: row.Title, Scope: row.Scope,
		Auditor: row.Auditor, AuditDate: row.AuditDate, Status: row.Status,
		Findings: row.Findings, Recommendations: row.Recommendations,
		CreatedAt: row.CreatedAt, UpdatedAt: row.UpdatedAt,
	})
	return &rec, nil
}

func (r *Repository) UpdateAuditRecord(ctx context.Context, orgID, id string, in UpdateAuditRecordInput) (*AuditRecord, error) {
	row, err := r.q.UpdateCKAuditRecord(ctx, db.UpdateCKAuditRecordParams{
		ID:              id,
		OrgID:           orgID,
		Title:           in.Title,
		Scope:           in.Scope,
		Auditor:         in.Auditor,
		AuditDate:       pgtype.Date{Time: in.AuditDate, Valid: true},
		Status:          in.Status,
		Findings:        in.Findings,
		Recommendations: in.Recommendations,
	})
	if err != nil {
		return nil, fmt.Errorf("update audit record: %w", err)
	}
	rec := auditRecordFromFields(auditRecordFields{
		ID: row.ID, OrgID: row.OrgID, Title: row.Title, Scope: row.Scope,
		Auditor: row.Auditor, AuditDate: row.AuditDate, Status: row.Status,
		Findings: row.Findings, Recommendations: row.Recommendations,
		CreatedAt: row.CreatedAt, UpdatedAt: row.UpdatedAt,
	})
	return &rec, nil
}

func (r *Repository) CreateAuditRecord(ctx context.Context, orgID string, in CreateAuditRecordInput) (*AuditRecord, error) {
	row, err := r.q.CreateCKAuditRecord(ctx, db.CreateCKAuditRecordParams{
		OrgID:           orgID,
		Title:           in.Title,
		Scope:           in.Scope,
		Auditor:         in.Auditor,
		AuditDate:       pgtype.Date{Time: in.AuditDate, Valid: true},
		Findings:        in.Findings,
		Recommendations: in.Recommendations,
	})
	if err != nil {
		return nil, fmt.Errorf("create audit record: %w", err)
	}
	rec := auditRecordFromFields(auditRecordFields{
		ID: row.ID, OrgID: row.OrgID, Title: row.Title, Scope: row.Scope,
		Auditor: row.Auditor, AuditDate: row.AuditDate, Status: row.Status,
		Findings: row.Findings, Recommendations: row.Recommendations,
		CreatedAt: row.CreatedAt, UpdatedAt: row.UpdatedAt,
	})
	return &rec, nil
}
