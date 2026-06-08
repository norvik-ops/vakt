// Copyright (c) 2026 NorvikOps. All rights reserved.
// SPDX-License-Identifier: Elastic-2.0

package vaktcomply

import (
	"context"
	"fmt"

	db "github.com/matharnica/vakt/internal/db"
)

// CreateProtectionNeedAssessment inserts a new Schutzbedarfsfeststellung record.
func (r *Repository) CreateProtectionNeedAssessment(ctx context.Context, orgID string, in CreateProtectionNeedInput) (ProtectionNeedAssessment, error) {
	row, err := r.q.CreateCKProtectionNeedAssessment(ctx, db.CreateCKProtectionNeedAssessmentParams{
		OrgID:      orgID,
		Name:       in.Name,
		ObjectType: in.ObjectType,
		ObjectName: in.ObjectName,
	})
	if err != nil {
		return ProtectionNeedAssessment{}, fmt.Errorf("create protection need assessment: %w", err)
	}
	return protectionNeedFromRow(row), nil
}

// ListProtectionNeedAssessments returns all assessments for an organisation.
func (r *Repository) ListProtectionNeedAssessments(ctx context.Context, orgID string) ([]ProtectionNeedAssessment, error) {
	rows, err := r.q.ListCKProtectionNeedAssessments(ctx, orgID)
	if err != nil {
		return nil, fmt.Errorf("list protection need assessments: %w", err)
	}
	out := make([]ProtectionNeedAssessment, len(rows))
	for i, row := range rows {
		out[i] = protectionNeedFromRow(row)
	}
	return out, nil
}

// GetProtectionNeedAssessment returns a single assessment by ID within an organisation.
func (r *Repository) GetProtectionNeedAssessment(ctx context.Context, orgID, id string) (ProtectionNeedAssessment, error) {
	row, err := r.q.GetCKProtectionNeedAssessment(ctx, db.GetCKProtectionNeedAssessmentParams{ID: id, OrgID: orgID})
	if err != nil {
		return ProtectionNeedAssessment{}, fmt.Errorf("get protection need assessment: %w", err)
	}
	return protectionNeedFromRow(row), nil
}

// UpdateProtectionNeedAssessment sets the C/I/A ratings and recomputes the overall level.
func (r *Repository) UpdateProtectionNeedAssessment(ctx context.Context, orgID, id string, in UpdateProtectionNeedInput) (ProtectionNeedAssessment, error) {
	overall := CalculateOverallProtectionNeed(in.Confidentiality, in.Integrity, in.Availability)
	row, err := r.q.UpdateCKProtectionNeedAssessment(ctx, db.UpdateCKProtectionNeedAssessmentParams{
		ID:              id,
		OrgID:           orgID,
		Confidentiality: in.Confidentiality,
		Integrity:       in.Integrity,
		Availability:    in.Availability,
		Overall:         overall,
	})
	if err != nil {
		return ProtectionNeedAssessment{}, fmt.Errorf("update protection need assessment: %w", err)
	}
	return protectionNeedFromRow(row), nil
}

// FinalizeProtectionNeedAssessment sets the assessment status to 'finalized'.
func (r *Repository) FinalizeProtectionNeedAssessment(ctx context.Context, orgID, id string) (ProtectionNeedAssessment, error) {
	row, err := r.q.FinalizeCKProtectionNeedAssessment(ctx, db.FinalizeCKProtectionNeedAssessmentParams{ID: id, OrgID: orgID})
	if err != nil {
		return ProtectionNeedAssessment{}, fmt.Errorf("finalize protection need assessment: %w", err)
	}
	return protectionNeedFromRow(row), nil
}

// DeleteProtectionNeedAssessment removes a protection need assessment record.
func (r *Repository) DeleteProtectionNeedAssessment(ctx context.Context, orgID, id string) error {
	n, err := r.q.DeleteCKProtectionNeedAssessment(ctx, db.DeleteCKProtectionNeedAssessmentParams{ID: id, OrgID: orgID})
	if err != nil {
		return fmt.Errorf("delete protection need assessment: %w", err)
	}
	if n == 0 {
		return fmt.Errorf("protection need assessment not found")
	}
	return nil
}

// protectionNeedFromRow maps a db.CkProtectionNeedAssessments row to the domain model.
func protectionNeedFromRow(row db.CkProtectionNeedAssessments) ProtectionNeedAssessment {
	return ProtectionNeedAssessment{
		ID:              row.ID,
		OrgID:           row.OrgID,
		Name:            row.Name,
		ObjectType:      row.ObjectType,
		ObjectName:      row.ObjectName,
		Confidentiality: row.Confidentiality,
		Integrity:       row.Integrity,
		Availability:    row.Availability,
		Overall:         row.Overall,
		Status:          row.Status,
		FinalizedAt:     ckTsToTimePtr(row.FinalizedAt),
		CreatedAt:       ckTsToTime(row.CreatedAt),
		UpdatedAt:       ckTsToTime(row.UpdatedAt),
	}
}
