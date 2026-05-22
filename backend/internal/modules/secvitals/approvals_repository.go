// Copyright (c) 2026 NorvikOps. All rights reserved.
// SPDX-License-Identifier: Elastic-2.0

package secvitals

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"

	"github.com/matharnica/vakt/internal/db"
)

// Approval represents a pending or resolved control status change approval request.
type Approval struct {
	ID              string     `json:"id"`
	OrgID           string     `json:"org_id"`
	ControlID       string     `json:"control_id"`
	RequestedBy     string     `json:"requested_by"`
	RequestedStatus string     `json:"requested_status"`
	CurrentStatus   string     `json:"current_status"`
	Comment         string     `json:"comment,omitempty"`
	Status          string     `json:"status"` // pending | approved | rejected
	ReviewedBy      string     `json:"reviewed_by,omitempty"`
	ReviewedAt      *time.Time `json:"reviewed_at,omitempty"`
	ReviewComment   string     `json:"review_comment,omitempty"`
	CreatedAt       time.Time  `json:"created_at"`
}

// ApprovalWithDetails enriches an Approval with control title and requester name.
type ApprovalWithDetails struct {
	Approval
	ControlTitle   string `json:"control_title"`
	ControlRef     string `json:"control_ref"`
	RequesterName  string `json:"requester_name"`
	RequesterEmail string `json:"requester_email"`
}

func approvalFromCk(r db.CkControlApprovals) Approval {
	return Approval{
		ID:              r.ID,
		OrgID:           r.OrgID,
		ControlID:       r.ControlID,
		RequestedBy:     r.RequestedBy,
		RequestedStatus: r.RequestedStatus,
		CurrentStatus:   r.CurrentStatus,
		Comment:         r.Comment.String,
		Status:          r.Status,
		ReviewedBy:      uuidStringFromPgtype(r.ReviewedBy),
		ReviewedAt:      ckTsToTimePtr(r.ReviewedAt),
		ReviewComment:   r.ReviewComment.String,
		CreatedAt:       ckTsToTime(r.CreatedAt),
	}
}

// CreateApprovalRequest inserts a new pending approval request.
func (r *Repository) CreateApprovalRequest(
	ctx context.Context,
	orgID, controlID, requestedBy, requestedStatus, currentStatus, comment string,
) (*Approval, error) {
	row, err := r.q.CreateCKApprovalRequest(ctx, db.CreateCKApprovalRequestParams{
		OrgID:           orgID,
		ControlID:       controlID,
		RequestedBy:     requestedBy,
		RequestedStatus: requestedStatus,
		CurrentStatus:   currentStatus,
		Comment:         comment,
	})
	if err != nil {
		return nil, fmt.Errorf("create approval request: %w", err)
	}
	a := approvalFromCk(db.CkControlApprovals(row))
	return &a, nil
}

// ListPendingApprovals returns all pending approvals for an org, joined with control and user info.
func (r *Repository) ListPendingApprovals(ctx context.Context, orgID string) ([]ApprovalWithDetails, error) {
	rows, err := r.q.ListCKPendingApprovals(ctx, orgID)
	if err != nil {
		return nil, fmt.Errorf("list pending approvals: %w", err)
	}
	out := make([]ApprovalWithDetails, 0, len(rows))
	for _, row := range rows {
		out = append(out, ApprovalWithDetails{
			Approval: Approval{
				ID:              row.ID,
				OrgID:           row.OrgID,
				ControlID:       row.ControlID,
				RequestedBy:     row.RequestedBy,
				RequestedStatus: row.RequestedStatus,
				CurrentStatus:   row.CurrentStatus,
				Comment:         row.Comment,
				Status:          row.Status,
				ReviewedBy:      uuidStringFromPgtype(row.ReviewedBy),
				ReviewedAt:      ckTsToTimePtr(row.ReviewedAt),
				ReviewComment:   row.ReviewComment,
				CreatedAt:       ckTsToTime(row.CreatedAt),
			},
			ControlTitle:   row.ControlTitle,
			ControlRef:     row.ControlRef,
			RequesterName:  row.RequesterName,
			RequesterEmail: row.RequesterEmail,
		})
	}
	return out, nil
}

// GetApproval returns a single approval request by ID within an org.
func (r *Repository) GetApproval(ctx context.Context, orgID, approvalID string) (*Approval, error) {
	row, err := r.q.GetCKApproval(ctx, db.GetCKApprovalParams{ID: approvalID, OrgID: orgID})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, fmt.Errorf("approval not found")
		}
		return nil, fmt.Errorf("get approval: %w", err)
	}
	a := approvalFromCk(db.CkControlApprovals(row))
	return &a, nil
}

// ReviewApproval marks an approval as approved or rejected and optionally updates the control status.
func (r *Repository) ReviewApproval(
	ctx context.Context,
	orgID, approvalID, reviewerID string,
	approve bool,
	comment string,
) error {
	newStatus := "rejected"
	if approve {
		newStatus = "approved"
	}
	now := time.Now().UTC()

	tx, err := r.db.Begin(ctx)
	if err != nil {
		return fmt.Errorf("begin tx: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }() // no-op when Commit succeeded
	qtx := r.q.WithTx(tx)

	reviewRow, err := qtx.ReviewCKApproval(ctx, db.ReviewCKApprovalParams{
		ID:         approvalID,
		OrgID:      orgID,
		Status:     newStatus,
		ReviewedBy: ckOptUUIDFromStr(reviewerID),
		ReviewedAt: pgtype.Timestamptz{Time: now, Valid: true},
		Comment:    comment,
	})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return fmt.Errorf("approval not found or already reviewed")
		}
		return fmt.Errorf("update approval: %w", err)
	}

	if approve {
		var notApplicable bool
		var manualStatus string
		switch reviewRow.RequestedStatus {
		case "not_applicable":
			notApplicable = true
			manualStatus = ""
		case "missing":
			notApplicable = false
			manualStatus = ""
		default:
			notApplicable = false
			manualStatus = reviewRow.RequestedStatus
		}
		if err := qtx.ApplyCKApprovedControlStatus(ctx, db.ApplyCKApprovedControlStatusParams{
			ID:            reviewRow.ControlID,
			OrgID:         orgID,
			NotApplicable: notApplicable,
			ManualStatus:  ckOptText(manualStatus),
		}); err != nil {
			return fmt.Errorf("update control status: %w", err)
		}
	}

	return tx.Commit(ctx)
}

// OrgApprovalRequired returns whether the organisation requires approval for control status changes.
func (r *Repository) OrgApprovalRequired(ctx context.Context, orgID string) (bool, error) {
	required, err := r.q.GetCKOrgApprovalRequired(ctx, orgID)
	if err != nil {
		return false, fmt.Errorf("get org approval_required: %w", err)
	}
	return required, nil
}

// SetOrgApprovalRequired updates the approval_required flag for an organisation.
func (r *Repository) SetOrgApprovalRequired(ctx context.Context, orgID string, required bool) error {
	n, err := r.q.SetCKOrgApprovalRequired(ctx, db.SetCKOrgApprovalRequiredParams{
		ID:               orgID,
		ApprovalRequired: required,
	})
	if err != nil {
		return fmt.Errorf("set org approval_required: %w", err)
	}
	if n == 0 {
		return fmt.Errorf("org not found: %s", orgID)
	}
	return nil
}

// CountPendingApprovals returns the number of pending approvals for an org.
func (r *Repository) CountPendingApprovals(ctx context.Context, orgID string) (int, error) {
	n, err := r.q.CountCKPendingApprovals(ctx, orgID)
	if err != nil {
		return 0, fmt.Errorf("count pending approvals: %w", err)
	}
	return int(n), nil
}

// GetOrgMemberRole returns the role name of a user in an organisation.
// Returns pgx.ErrNoRows if the user is not a member.
func (r *Repository) GetOrgMemberRole(ctx context.Context, userID, orgID string) (string, error) {
	return r.q.GetCKOrgMemberRole(ctx, db.GetCKOrgMemberRoleParams{UserID: userID, OrgID: orgID})
}
