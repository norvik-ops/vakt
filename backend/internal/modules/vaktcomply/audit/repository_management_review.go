// Copyright (c) 2026 NorvikOps. All rights reserved.
// SPDX-License-Identifier: Elastic-2.0

package audit

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
)

const managementReviewSelectCols = `
SELECT id::text, org_id::text, review_date::text, review_type, participant_ids,
       status, audit_findings_summary, incident_summary, risk_status_summary,
       previous_actions_status, kpi_snapshot, context_changes, customer_feedback,
       improvement_decisions, resource_decisions, isms_changes,
       next_review_date::text, approved_by::text, approved_at, created_by::text, created_at, updated_at
FROM ck_management_reviews`

// managementReviewReturningCols is the RETURNING clause for UPDATE/INSERT so the
// mutated row is read back in ONE statement. S124-7 (N2): the previous
// `WITH upd AS (UPDATE … RETURNING id) SELECT … JOIN upd` pattern is broken —
// every WITH sub-statement and the outer SELECT share one snapshot, so the outer
// scan reads the PRE-update row (Approve returned status:"draft", approved_by:null
// despite persisting correctly). RETURNING sees the post-update values directly.
// Same column order as scanManagementReview / managementReviewSelectCols.
const managementReviewReturningCols = ` RETURNING
       id::text, org_id::text, review_date::text, review_type, participant_ids,
       status, audit_findings_summary, incident_summary, risk_status_summary,
       previous_actions_status, kpi_snapshot, context_changes, customer_feedback,
       improvement_decisions, resource_decisions, isms_changes,
       next_review_date::text, approved_by::text, approved_at, created_by::text, created_at, updated_at`

// scanManagementReview scans a single row into a ManagementReview.
func scanManagementReview(row pgx.Row) (ManagementReview, error) {
	var mr ManagementReview
	var participantIDs []byte
	var kpiSnapshot []byte
	var improvementDecisions []byte
	var nextReviewDate *string
	var approvedBy pgtype.Text
	var approvedAt pgtype.Timestamptz
	var createdAt pgtype.Timestamptz
	var updatedAt pgtype.Timestamptz

	err := row.Scan(
		&mr.ID,
		&mr.OrgID,
		&mr.ReviewDate,
		&mr.ReviewType,
		&participantIDs,
		&mr.Status,
		&mr.AuditFindingsSummary,
		&mr.IncidentSummary,
		&mr.RiskStatusSummary,
		&mr.PreviousActionsStatus,
		&kpiSnapshot,
		&mr.ContextChanges,
		&mr.CustomerFeedback,
		&improvementDecisions,
		&mr.ResourceDecisions,
		&mr.ISMSChanges,
		&nextReviewDate,
		&approvedBy,
		&approvedAt,
		&mr.CreatedBy,
		&createdAt,
		&updatedAt,
	)
	if err != nil {
		return ManagementReview{}, err
	}

	if len(participantIDs) > 0 {
		mr.ParticipantIDs = json.RawMessage(participantIDs)
	} else {
		mr.ParticipantIDs = json.RawMessage("[]")
	}

	if len(kpiSnapshot) > 0 {
		mr.KPISnapshot = json.RawMessage(kpiSnapshot)
	}

	if len(improvementDecisions) > 0 {
		mr.ImprovementDecisions = json.RawMessage(improvementDecisions)
	} else {
		mr.ImprovementDecisions = json.RawMessage("[]")
	}

	mr.NextReviewDate = nextReviewDate

	if approvedBy.Valid {
		s := approvedBy.String
		mr.ApprovedBy = &s
	}
	mr.ApprovedAt = ckTsToTimePtr(approvedAt)
	mr.CreatedAt = ckTsToTime(createdAt)
	mr.UpdatedAt = ckTsToTime(updatedAt)

	return mr, nil
}

// CreateManagementReview inserts a new management review record.
func (r *Repository) CreateManagementReview(ctx context.Context, orgID, userID string, in CreateManagementReviewInput) (ManagementReview, error) {
	participantIDs := in.ParticipantIDs
	if len(participantIDs) == 0 {
		participantIDs = json.RawMessage("[]")
	}

	// A data-modifying CTE cannot see its own inserted rows from an outer scan of
	// the same table (all sub-statements share one snapshot), so the previous
	// "WITH ins AS (INSERT…) SELECT … FROM ck_management_reviews JOIN ins" pattern
	// always returned zero rows → pgx.ErrNoRows → 500 (and each retry inserted a
	// duplicate). RETURNING reads the freshly inserted row directly, defaults included.
	q := `INSERT INTO ck_management_reviews (org_id, review_date, review_type, participant_ids, created_by)
    VALUES ($1, $2::date, $3, $4, $5::uuid)
    RETURNING id::text, org_id::text, review_date::text, review_type, participant_ids,
       status, audit_findings_summary, incident_summary, risk_status_summary,
       previous_actions_status, kpi_snapshot, context_changes, customer_feedback,
       improvement_decisions, resource_decisions, isms_changes,
       next_review_date::text, approved_by::text, approved_at, created_by::text, created_at, updated_at`

	row := r.db.QueryRow(ctx, q, orgID, in.ReviewDate, in.ReviewType, []byte(participantIDs), userID)
	mr, err := scanManagementReview(row)
	if err != nil {
		return ManagementReview{}, fmt.Errorf("create management review: %w", err)
	}
	return mr, nil
}

// GetManagementReview returns a single management review by org + ID.
func (r *Repository) GetManagementReview(ctx context.Context, orgID, id string) (ManagementReview, error) {
	q := managementReviewSelectCols + `
WHERE org_id = $1 AND id = $2::uuid`

	row := r.db.QueryRow(ctx, q, orgID, id)
	mr, err := scanManagementReview(row)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return ManagementReview{}, fmt.Errorf("management review not found")
		}
		return ManagementReview{}, fmt.Errorf("get management review: %w", err)
	}
	return mr, nil
}

// ListManagementReviews returns all management reviews for an organisation, newest first.
func (r *Repository) ListManagementReviews(ctx context.Context, orgID string) ([]ManagementReview, error) {
	q := managementReviewSelectCols + `
WHERE org_id = $1
ORDER BY review_date DESC`

	rows, err := r.db.Query(ctx, q, orgID)
	if err != nil {
		return nil, fmt.Errorf("list management reviews: %w", err)
	}
	defer rows.Close()

	var out []ManagementReview
	for rows.Next() {
		mr, err := scanManagementReview(rows)
		if err != nil {
			return nil, fmt.Errorf("scan management review: %w", err)
		}
		out = append(out, mr)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate management reviews: %w", err)
	}
	return out, nil
}

// UpdateManagementReviewInputs sets the input-phase text fields of a management review.
func (r *Repository) UpdateManagementReviewInputs(ctx context.Context, orgID, id string, in UpdateManagementReviewInputsInput) (ManagementReview, error) {
	kpiSnapshot := []byte(in.KPISnapshot)
	if len(kpiSnapshot) == 0 {
		kpiSnapshot = nil
	}

	q := `UPDATE ck_management_reviews SET
        audit_findings_summary  = $3,
        incident_summary        = $4,
        risk_status_summary     = $5,
        previous_actions_status = $6,
        kpi_snapshot            = $7,
        context_changes         = $8,
        customer_feedback       = $9,
        updated_at              = NOW()
    WHERE org_id = $1 AND id = $2::uuid` + managementReviewReturningCols

	row := r.db.QueryRow(ctx, q, orgID, id,
		in.AuditFindingsSummary, in.IncidentSummary, in.RiskStatusSummary,
		in.PreviousActionsStatus, kpiSnapshot, in.ContextChanges, in.CustomerFeedback,
	)
	mr, err := scanManagementReview(row)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return ManagementReview{}, fmt.Errorf("management review not found")
		}
		return ManagementReview{}, fmt.Errorf("update management review inputs: %w", err)
	}
	return mr, nil
}

// UpdateManagementReviewOutputs sets the output-phase fields of a management review.
func (r *Repository) UpdateManagementReviewOutputs(ctx context.Context, orgID, id string, in UpdateManagementReviewOutputsInput) (ManagementReview, error) {
	improvementDecisions := []byte(in.ImprovementDecisions)
	if len(improvementDecisions) == 0 {
		improvementDecisions = []byte("[]")
	}

	var nextReviewDate *string
	if in.NextReviewDate != nil && *in.NextReviewDate != "" {
		nextReviewDate = in.NextReviewDate
	}

	q := `UPDATE ck_management_reviews SET
        improvement_decisions = $3,
        resource_decisions    = $4,
        isms_changes          = $5,
        next_review_date      = $6::date,
        updated_at            = NOW()
    WHERE org_id = $1 AND id = $2::uuid` + managementReviewReturningCols

	row := r.db.QueryRow(ctx, q, orgID, id,
		improvementDecisions, in.ResourceDecisions, in.ISMSChanges, nextReviewDate,
	)
	mr, err := scanManagementReview(row)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return ManagementReview{}, fmt.Errorf("management review not found")
		}
		return ManagementReview{}, fmt.Errorf("update management review outputs: %w", err)
	}
	return mr, nil
}

// ApproveManagementReview sets status=approved and records the approver.
func (r *Repository) ApproveManagementReview(ctx context.Context, orgID, id, approverID string) (ManagementReview, error) {
	now := time.Now().UTC()

	q := `UPDATE ck_management_reviews SET
        status      = 'approved',
        approved_by = $3::uuid,
        approved_at = $4,
        updated_at  = NOW()
    WHERE org_id = $1 AND id = $2::uuid` + managementReviewReturningCols

	row := r.db.QueryRow(ctx, q, orgID, id, approverID, now)
	mr, err := scanManagementReview(row)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return ManagementReview{}, fmt.Errorf("management review not found")
		}
		return ManagementReview{}, fmt.Errorf("approve management review: %w", err)
	}
	return mr, nil
}

// GetLastManagementReview returns the most recent review for an organisation, or nil if none exist.
func (r *Repository) GetLastManagementReview(ctx context.Context, orgID string) (*ManagementReview, error) {
	q := managementReviewSelectCols + `
WHERE org_id = $1
ORDER BY review_date DESC
LIMIT 1`

	row := r.db.QueryRow(ctx, q, orgID)
	mr, err := scanManagementReview(row)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, nil
		}
		return nil, fmt.Errorf("get last management review: %w", err)
	}
	return &mr, nil
}
