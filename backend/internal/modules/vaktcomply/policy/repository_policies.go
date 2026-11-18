// Copyright (c) 2026 NorvikOps. All rights reserved.
// SPDX-License-Identifier: Elastic-2.0

package policy

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"

	"github.com/matharnica/vakt/internal/db"
)

// --- Policy Management (FR-CK14) ---

func (r *Repository) ListPolicies(ctx context.Context, orgID string) ([]Policy, error) {
	rows, err := r.q.ListCKPolicies(ctx, orgID)
	if err != nil {
		return nil, fmt.Errorf("list policies: %w", err)
	}
	out := make([]Policy, 0, len(rows))
	for _, row := range rows {
		out = append(out, policyFromFields(policyFields{
			ID: row.ID, OrgID: row.OrgID, Title: row.Title,
			Description: row.Description, Category: row.Category,
			Status: row.Status, Version: row.Version,
			EffectiveDate: row.EffectiveDate, ReviewDate: row.ReviewDate,
			Owner: row.Owner, CreatedAt: row.CreatedAt, UpdatedAt: row.UpdatedAt,
			VersionNum: row.VersionNum, VersionNote: row.VersionNote,
			LastUpdatedBy: row.LastUpdatedBy,
			ReviewedAt:    row.ReviewedAt, NextReviewDue: row.NextReviewDue,
			ApprovedBy: row.ApprovedBy, ApprovedByName: row.ApprovedByName, ApprovedAt: row.ApprovedAt,
		}))
	}
	return out, nil
}

func (r *Repository) GetPolicy(ctx context.Context, orgID, id string) (*Policy, error) {
	return getPolicy(ctx, r.q, orgID, id)
}

// getPolicy reads one policy including the approver name (join on users). It
// takes the querier so UpdatePolicy can re-read inside its own transaction.
func getPolicy(ctx context.Context, q *db.Queries, orgID, id string) (*Policy, error) {
	row, err := q.GetCKPolicy(ctx, db.GetCKPolicyParams{ID: id, OrgID: orgID})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, fmt.Errorf("get policy: %w", err)
	}
	p := policyFromFields(policyFields{
		ID: row.ID, OrgID: row.OrgID, Title: row.Title,
		Description: row.Description, Category: row.Category,
		Status: row.Status, Version: row.Version,
		EffectiveDate: row.EffectiveDate, ReviewDate: row.ReviewDate,
		Owner: row.Owner, CreatedAt: row.CreatedAt, UpdatedAt: row.UpdatedAt,
		VersionNum: row.VersionNum, VersionNote: row.VersionNote,
		LastUpdatedBy: row.LastUpdatedBy,
		ReviewedAt:    row.ReviewedAt, NextReviewDue: row.NextReviewDue,
		ApprovedBy: row.ApprovedBy, ApprovedByName: row.ApprovedByName, ApprovedAt: row.ApprovedAt,
	})
	return &p, nil
}

// UpdatePolicy snapshots the current policy version into ck_policy_versions, then increments
// version_num and applies the update fields. All steps run in a single transaction.
//
// decide receives the status the row had before this update (read under a row
// lock) and returns the approval change to apply. The decision itself is domain
// logic and lives in the service (approvalTransition); the repository only makes
// sure it is taken on the real previous state and written atomically with it.
func (r *Repository) UpdatePolicy(ctx context.Context, orgID, id string, in UpdatePolicyInput,
	decide func(prevStatus string) (approvalChange, error),
) (*Policy, error) {
	versionLabel := in.Version
	if versionLabel == "" {
		versionLabel = "1.0"
	}
	versionNote := ""
	if in.VersionNote != nil {
		versionNote = *in.VersionNote
	}
	updatedBy := ""
	if in.UpdatedBy != nil {
		updatedBy = *in.UpdatedBy
	}

	tx, err := r.db.Begin(ctx)
	if err != nil {
		return nil, fmt.Errorf("begin tx: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }() // no-op when Commit succeeded
	qtx := r.q.WithTx(tx)

	prevStatus, err := qtx.GetCKPolicyStatusForUpdate(ctx, db.GetCKPolicyStatusForUpdateParams{ID: id, OrgID: orgID})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, fmt.Errorf("lock policy: %w", err)
	}
	change, err := decide(prevStatus)
	if err != nil {
		return nil, err
	}

	// Snapshot the current state into ck_policy_versions before updating.
	if err := qtx.SnapshotCKPolicyVersion(ctx, db.SnapshotCKPolicyVersionParams{ID: id, OrgID: orgID}); err != nil {
		return nil, fmt.Errorf("snapshot policy version: %w", err)
	}

	approvedAt := pgtype.Timestamptz{}
	if change.At != nil {
		approvedAt = pgtype.Timestamptz{Time: *change.At, Valid: true}
	}
	if _, err := qtx.UpdateCKPolicy(ctx, db.UpdateCKPolicyParams{
		ID:            id,
		OrgID:         orgID,
		Title:         in.Title,
		Description:   in.Description,
		Category:      in.Category,
		Status:        in.Status,
		Version:       versionLabel,
		EffectiveDate: policyDateFromTimePtr(in.EffectiveDate),
		ReviewDate:    policyDateFromTimePtr(in.ReviewDate),
		Owner:         in.Owner,
		VersionNote:   versionNote,
		LastUpdatedBy: updatedBy,
		RefreshReview: updatedBy != "",
		NextReviewDue: ckOptDatePtr(in.NextReviewDue),
		SetApproval:   change.Touch,
		ApprovedBy:    change.By,
		ApprovedAt:    approvedAt,
	}); err != nil {
		return nil, fmt.Errorf("update policy: %w", err)
	}

	// Re-read inside the transaction: the response carries the approver name,
	// which only the join on users can supply.
	p, err := getPolicy(ctx, qtx, orgID, id)
	if err != nil {
		return nil, err
	}
	if err := tx.Commit(ctx); err != nil {
		return nil, fmt.Errorf("commit policy update: %w", err)
	}
	return p, nil
}

func (r *Repository) CreatePolicy(ctx context.Context, orgID string, in CreatePolicyInput) (*Policy, error) {
	version := in.Version
	if version == "" {
		version = "1.0"
	}
	row, err := r.q.CreateCKPolicy(ctx, db.CreateCKPolicyParams{
		OrgID:         orgID,
		Title:         in.Title,
		Description:   in.Description,
		Category:      in.Category,
		Version:       version,
		EffectiveDate: policyDateFromTimePtr(in.EffectiveDate),
		ReviewDate:    policyDateFromTimePtr(in.ReviewDate),
		Owner:         in.Owner,
	})
	if err != nil {
		return nil, fmt.Errorf("create policy: %w", err)
	}
	p := policyFromFields(policyFields{
		ID: row.ID, OrgID: row.OrgID, Title: row.Title,
		Description: row.Description, Category: row.Category,
		Status: row.Status, Version: row.Version,
		EffectiveDate: row.EffectiveDate, ReviewDate: row.ReviewDate,
		Owner: row.Owner, CreatedAt: row.CreatedAt, UpdatedAt: row.UpdatedAt,
		VersionNum: row.VersionNum, VersionNote: row.VersionNote,
		LastUpdatedBy: row.LastUpdatedBy,
		ReviewedAt:    row.ReviewedAt, NextReviewDue: row.NextReviewDue,
	})
	return &p, nil
}

// ListPolicyVersions returns all historical version snapshots for a policy, newest first.
func (r *Repository) ListPolicyVersions(ctx context.Context, orgID, policyID string) ([]PolicyVersion, error) {
	rows, err := r.q.ListCKPolicyVersions(ctx, db.ListCKPolicyVersionsParams{PolicyID: policyID, OrgID: orgID})
	if err != nil {
		return nil, fmt.Errorf("list policy versions: %w", err)
	}
	versions := make([]PolicyVersion, 0, len(rows))
	for _, row := range rows {
		versions = append(versions, PolicyVersion{
			ID: row.ID, OrgID: row.OrgID, PolicyID: row.PolicyID, Version: int(row.Version),
			Title: row.Title, Content: row.Content, Status: row.Status,
			VersionNote: row.VersionNote, UpdatedBy: row.UpdatedBy,
			CreatedAt: ckTsToTime(row.CreatedAt),
		})
	}
	return versions, nil
}

// GetPolicyVersion returns a single historical version snapshot.
func (r *Repository) GetPolicyVersion(ctx context.Context, orgID, policyID string, version int) (PolicyVersion, error) {
	row, err := r.q.GetCKPolicyVersion(ctx, db.GetCKPolicyVersionParams{
		PolicyID: policyID, OrgID: orgID, Version: int32(version),
	})
	if err != nil {
		return PolicyVersion{}, fmt.Errorf("get policy version: %w", err)
	}
	return PolicyVersion{
		ID: row.ID, OrgID: row.OrgID, PolicyID: row.PolicyID, Version: int(row.Version),
		Title: row.Title, Content: row.Content, Status: row.Status,
		VersionNote: row.VersionNote, UpdatedBy: row.UpdatedBy,
		CreatedAt: ckTsToTime(row.CreatedAt),
	}, nil
}

// ListPoliciesPaged returns a page of policies plus the total count.
func (r *Repository) ListPoliciesPaged(ctx context.Context, orgID string, offset, limit int) ([]Policy, int, error) {
	total, err := r.q.CountCKPolicies(ctx, orgID)
	if err != nil {
		return nil, 0, fmt.Errorf("count policies: %w", err)
	}
	rows, err := r.q.ListCKPoliciesPaged(ctx, db.ListCKPoliciesPagedParams{
		OrgID:  orgID,
		Limit:  int32(limit),
		Offset: int32(offset),
	})
	if err != nil {
		return nil, 0, fmt.Errorf("list policies paged: %w", err)
	}
	policies := make([]Policy, 0, len(rows))
	for _, row := range rows {
		policies = append(policies, policyFromFields(policyFields{
			ID: row.ID, OrgID: row.OrgID, Title: row.Title,
			Description: row.Description, Category: row.Category,
			Status: row.Status, Version: row.Version,
			EffectiveDate: row.EffectiveDate, ReviewDate: row.ReviewDate,
			Owner: row.Owner, CreatedAt: row.CreatedAt, UpdatedAt: row.UpdatedAt,
			VersionNum: row.VersionNum, VersionNote: row.VersionNote,
			LastUpdatedBy: row.LastUpdatedBy,
			ReviewedAt:    row.ReviewedAt, NextReviewDue: row.NextReviewDue,
			ApprovedBy: row.ApprovedBy, ApprovedByName: row.ApprovedByName, ApprovedAt: row.ApprovedAt,
		}))
	}
	return policies, int(total), nil
}
