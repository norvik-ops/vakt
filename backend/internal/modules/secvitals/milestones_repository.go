// Copyright (c) 2026 NorvikOps. All rights reserved.
// SPDX-License-Identifier: Elastic-2.0

package secvitals

import (
	"context"
	"fmt"
	"math"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"

	"github.com/matharnica/vakt/internal/db"
)


// ── helpers ──────────────────────────────────────────────────────────────────

// uuidStringPtr returns the UUID as *string (nil when invalid).
// Shared with repository.go via same package — uses uuidStringFromPgtype.
func uuidStringPtr(u pgtype.UUID) *string {
	if !u.Valid {
		return nil
	}
	s := uuidStringFromPgtype(u)
	return &s
}

// milestoneFromRow maps a sqlc milestone row (shared column layout) to the
// AuditMilestone domain type. today is pre-computed by the caller.
func milestoneFromRow(
	id, orgID string,
	frameworkID pgtype.UUID,
	title string,
	description pgtype.Text,
	milestoneDate, milestoneType, status string,
	createdBy pgtype.UUID,
	createdAt, updatedAt pgtype.Timestamptz,
	today time.Time,
) AuditMilestone {
	return AuditMilestone{
		ID:            id,
		OrgID:         orgID,
		FrameworkID:   uuidStringPtr(frameworkID),
		Title:         title,
		Description:   description.String,
		MilestoneDate: milestoneDate,
		MilestoneType: milestoneType,
		Status:        status,
		CreatedBy:     uuidStringPtr(createdBy),
		CreatedAt:     ckTsToTime(createdAt),
		UpdatedAt:     ckTsToTime(updatedAt),
		DaysRemaining: computeDaysRemaining(milestoneDate, today),
	}
}

// milestoneFromListRow maps a ListCKMilestonesRow to AuditMilestone.
func milestoneFromListRow(r db.ListCKMilestonesRow, today time.Time) AuditMilestone {
	return milestoneFromRow(
		r.ID, r.OrgID, r.FrameworkID,
		r.Title, r.Description,
		r.MilestoneDate, r.MilestoneType, r.Status,
		r.CreatedBy, r.CreatedAt, r.UpdatedAt,
		today,
	)
}

// milestoneFromGetRow maps a GetCKMilestoneRow to AuditMilestone.
func milestoneFromGetRow(r db.GetCKMilestoneRow, today time.Time) AuditMilestone {
	return milestoneFromRow(
		r.ID, r.OrgID, r.FrameworkID,
		r.Title, r.Description,
		r.MilestoneDate, r.MilestoneType, r.Status,
		r.CreatedBy, r.CreatedAt, r.UpdatedAt,
		today,
	)
}

// milestoneFromCreateRow maps a CreateCKMilestoneRow to AuditMilestone.
func milestoneFromCreateRow(r db.CreateCKMilestoneRow, today time.Time) AuditMilestone {
	return milestoneFromRow(
		r.ID, r.OrgID, r.FrameworkID,
		r.Title, r.Description,
		r.MilestoneDate, r.MilestoneType, r.Status,
		r.CreatedBy, r.CreatedAt, r.UpdatedAt,
		today,
	)
}

// milestoneFromUpdateRow maps an UpdateCKMilestoneRow to AuditMilestone.
func milestoneFromUpdateRow(r db.UpdateCKMilestoneRow, today time.Time) AuditMilestone {
	return milestoneFromRow(
		r.ID, r.OrgID, r.FrameworkID,
		r.Title, r.Description,
		r.MilestoneDate, r.MilestoneType, r.Status,
		r.CreatedBy, r.CreatedAt, r.UpdatedAt,
		today,
	)
}

// milestoneFromNextRow maps a NextCKMilestoneRow to AuditMilestone.
func milestoneFromNextRow(r db.NextCKMilestoneRow, today time.Time) AuditMilestone {
	return milestoneFromRow(
		r.ID, r.OrgID, r.FrameworkID,
		r.Title, r.Description,
		r.MilestoneDate, r.MilestoneType, r.Status,
		r.CreatedBy, r.CreatedAt, r.UpdatedAt,
		today,
	)
}

// parseDateArg converts a YYYY-MM-DD string to pgtype.Date (invalid on empty).
func parseDateArg(s string) pgtype.Date {
	if s == "" {
		return pgtype.Date{}
	}
	t, err := time.Parse("2006-01-02", s)
	if err != nil {
		return pgtype.Date{}
	}
	return pgtype.Date{Time: t, Valid: true}
}

// ── Repository methods ────────────────────────────────────────────────────────

// ListMilestones returns all milestones for an org ordered by milestone_date ASC.
// If statusFilter is non-empty only that status is returned.
func (r *Repository) ListMilestones(ctx context.Context, orgID, statusFilter string) ([]AuditMilestone, error) {
	rows, err := r.q.ListCKMilestones(ctx, db.ListCKMilestonesParams{
		OrgID:  orgID,
		Status: ckOptText(statusFilter),
	})
	if err != nil {
		return nil, fmt.Errorf("list milestones: %w", err)
	}
	today := time.Now().UTC().Truncate(24 * time.Hour)
	milestones := make([]AuditMilestone, 0, len(rows))
	for _, row := range rows {
		milestones = append(milestones, milestoneFromListRow(row, today))
	}
	return milestones, nil
}

// GetMilestone retrieves a single milestone by ID.
func (r *Repository) GetMilestone(ctx context.Context, orgID, milestoneID string) (*AuditMilestone, error) {
	row, err := r.q.GetCKMilestone(ctx, db.GetCKMilestoneParams{ID: milestoneID, OrgID: orgID})
	if err != nil {
		return nil, fmt.Errorf("get milestone: %w", err)
	}
	today := time.Now().UTC().Truncate(24 * time.Hour)
	m := milestoneFromGetRow(row, today)
	return &m, nil
}

// CreateMilestone inserts a new milestone.
func (r *Repository) CreateMilestone(ctx context.Context, orgID, createdBy string, in CreateMilestoneInput) (*AuditMilestone, error) {
	row, err := r.q.CreateCKMilestone(ctx, db.CreateCKMilestoneParams{
		OrgID:         orgID,
		FrameworkID:   ckOptUUIDFromStr(in.FrameworkID),
		Title:         in.Title,
		Description:   ckOptText(in.Description),
		MilestoneDate: parseDateArg(in.MilestoneDate),
		MilestoneType: in.MilestoneType,
		CreatedBy:     ckOptUUIDFromStr(createdBy),
	})
	if err != nil {
		return nil, fmt.Errorf("create milestone: %w", err)
	}
	today := time.Now().UTC().Truncate(24 * time.Hour)
	m := milestoneFromCreateRow(row, today)
	return &m, nil
}

// UpdateMilestone applies a partial update to an existing milestone.
func (r *Repository) UpdateMilestone(ctx context.Context, orgID, milestoneID string, in UpdateMilestoneInput) (*AuditMilestone, error) {
	// Fetch current to merge
	cur, err := r.GetMilestone(ctx, orgID, milestoneID)
	if err != nil {
		return nil, err
	}

	title := cur.Title
	description := cur.Description
	milestoneDate := cur.MilestoneDate
	milestoneType := cur.MilestoneType
	status := cur.Status

	if in.Title != nil {
		title = *in.Title
	}
	if in.Description != nil {
		description = *in.Description
	}
	if in.MilestoneDate != nil {
		milestoneDate = *in.MilestoneDate
	}
	if in.MilestoneType != nil {
		milestoneType = *in.MilestoneType
	}
	if in.Status != nil {
		status = *in.Status
	}

	row, err := r.q.UpdateCKMilestone(ctx, db.UpdateCKMilestoneParams{
		Title:         title,
		Description:   ckOptText(description),
		MilestoneDate: parseDateArg(milestoneDate),
		MilestoneType: milestoneType,
		Status:        status,
		ID:            milestoneID,
		OrgID:         orgID,
	})
	if err != nil {
		return nil, fmt.Errorf("update milestone: %w", err)
	}
	today := time.Now().UTC().Truncate(24 * time.Hour)
	m := milestoneFromUpdateRow(row, today)
	return &m, nil
}

// DeleteMilestone removes a milestone.
func (r *Repository) DeleteMilestone(ctx context.Context, orgID, milestoneID string) error {
	n, err := r.q.DeleteCKMilestone(ctx, db.DeleteCKMilestoneParams{ID: milestoneID, OrgID: orgID})
	if err != nil {
		return fmt.Errorf("delete milestone: %w", err)
	}
	if n == 0 {
		return fmt.Errorf("milestone not found")
	}
	return nil
}

// NextMilestone returns the nearest upcoming milestone or nil if none exist.
func (r *Repository) NextMilestone(ctx context.Context, orgID string) (*AuditMilestone, error) {
	row, err := r.q.NextCKMilestone(ctx, orgID)
	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, err // caller checks pgx.ErrNoRows
		}
		return nil, err
	}
	today := time.Now().UTC().Truncate(24 * time.Hour)
	m := milestoneFromNextRow(row, today)
	return &m, nil
}

// computeDaysRemaining returns a pointer to the number of days between today and the milestone date.
// Negative values mean the milestone is overdue.
func computeDaysRemaining(dateStr string, today time.Time) *int {
	t, err := time.Parse("2006-01-02", dateStr)
	if err != nil {
		return nil
	}
	days := int(math.Round(t.Sub(today).Hours() / 24))
	return &days
}
