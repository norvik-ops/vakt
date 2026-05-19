// Copyright (c) 2026 NorvikOps. All rights reserved.
// SPDX-License-Identifier: Elastic-2.0

package secvitals

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
)

// --- Access Review Campaigns ---

// ListAccessReviewCampaigns returns all campaigns for an organisation ordered by created_at DESC.
func (r *Repository) ListAccessReviewCampaigns(ctx context.Context, orgID string) ([]AccessReviewCampaign, error) {
	rows, err := r.db.Query(ctx, `
		SELECT id::text, org_id::text, title, COALESCE(description,''),
		       status, reviewer_email, COALESCE(scope,''),
		       due_date, completed_at, COALESCE(created_by,''),
		       created_at, updated_at
		FROM ck_access_review_campaigns
		WHERE org_id = $1::uuid
		ORDER BY created_at DESC`,
		orgID,
	)
	if err != nil {
		return nil, fmt.Errorf("list access review campaigns: %w", err)
	}
	defer rows.Close()

	var campaigns []AccessReviewCampaign
	for rows.Next() {
		var c AccessReviewCampaign
		if err := rows.Scan(
			&c.ID, &c.OrgID, &c.Title, &c.Description,
			&c.Status, &c.ReviewerEmail, &c.Scope,
			&c.DueDate, &c.CompletedAt, &c.CreatedBy,
			&c.CreatedAt, &c.UpdatedAt,
		); err != nil {
			return nil, fmt.Errorf("scan access review campaign: %w", err)
		}
		campaigns = append(campaigns, c)
	}
	return campaigns, rows.Err()
}

// GetAccessReviewCampaign returns a single campaign by ID. Returns ErrNotFound if absent.
func (r *Repository) GetAccessReviewCampaign(ctx context.Context, orgID, id string) (*AccessReviewCampaign, error) {
	var c AccessReviewCampaign
	err := r.db.QueryRow(ctx, `
		SELECT id::text, org_id::text, title, COALESCE(description,''),
		       status, reviewer_email, COALESCE(scope,''),
		       due_date, completed_at, COALESCE(created_by,''),
		       created_at, updated_at
		FROM ck_access_review_campaigns
		WHERE id = $1::uuid AND org_id = $2::uuid`,
		id, orgID,
	).Scan(
		&c.ID, &c.OrgID, &c.Title, &c.Description,
		&c.Status, &c.ReviewerEmail, &c.Scope,
		&c.DueDate, &c.CompletedAt, &c.CreatedBy,
		&c.CreatedAt, &c.UpdatedAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, fmt.Errorf("get access review campaign: %w", err)
	}
	return &c, nil
}

// CreateAccessReviewCampaign inserts a new campaign and returns the created record.
func (r *Repository) CreateAccessReviewCampaign(ctx context.Context, orgID string, in CreateAccessReviewCampaignInput) (*AccessReviewCampaign, error) {
	var dueDate *time.Time
	if in.DueDate != nil && *in.DueDate != "" {
		t, err := time.Parse(time.RFC3339, *in.DueDate)
		if err != nil {
			// Try date-only format
			t, err = time.Parse("2006-01-02", *in.DueDate)
			if err != nil {
				return nil, fmt.Errorf("invalid due_date format: %w", err)
			}
		}
		dueDate = &t
	}

	var c AccessReviewCampaign
	err := r.db.QueryRow(ctx, `
		INSERT INTO ck_access_review_campaigns
		  (org_id, title, description, reviewer_email, scope, due_date)
		VALUES ($1::uuid, $2, $3, $4, $5, $6)
		RETURNING id::text, org_id::text, title, COALESCE(description,''),
		          status, reviewer_email, COALESCE(scope,''),
		          due_date, completed_at, COALESCE(created_by,''),
		          created_at, updated_at`,
		orgID, in.Title, in.Description, in.ReviewerEmail, in.Scope, dueDate,
	).Scan(
		&c.ID, &c.OrgID, &c.Title, &c.Description,
		&c.Status, &c.ReviewerEmail, &c.Scope,
		&c.DueDate, &c.CompletedAt, &c.CreatedBy,
		&c.CreatedAt, &c.UpdatedAt,
	)
	if err != nil {
		return nil, fmt.Errorf("create access review campaign: %w", err)
	}
	return &c, nil
}

// UpdateAccessReviewCampaign applies updates to an existing campaign and returns the updated record.
func (r *Repository) UpdateAccessReviewCampaign(ctx context.Context, orgID, id string, in UpdateAccessReviewCampaignInput) (*AccessReviewCampaign, error) {
	cur, err := r.GetAccessReviewCampaign(ctx, orgID, id)
	if err != nil {
		return nil, err
	}

	title := cur.Title
	description := cur.Description
	reviewerEmail := cur.ReviewerEmail
	scope := cur.Scope
	status := cur.Status
	var dueDate *time.Time

	// Carry over existing due_date
	dueDate = cur.DueDate

	if in.Title != "" {
		title = in.Title
	}
	if in.Description != "" {
		description = in.Description
	}
	if in.ReviewerEmail != "" {
		reviewerEmail = in.ReviewerEmail
	}
	if in.Scope != "" {
		scope = in.Scope
	}
	if in.Status != "" {
		status = in.Status
	}
	if in.DueDate != nil {
		if *in.DueDate == "" {
			dueDate = nil
		} else {
			t, err := time.Parse(time.RFC3339, *in.DueDate)
			if err != nil {
				t, err = time.Parse("2006-01-02", *in.DueDate)
				if err != nil {
					return nil, fmt.Errorf("invalid due_date format: %w", err)
				}
			}
			dueDate = &t
		}
	}

	// Set completed_at when transitioning to completed
	var completedAt *time.Time
	if status == "completed" && cur.Status != "completed" {
		now := time.Now().UTC()
		completedAt = &now
	} else {
		completedAt = cur.CompletedAt
	}

	var c AccessReviewCampaign
	err = r.db.QueryRow(ctx, `
		UPDATE ck_access_review_campaigns
		SET title = $1, description = $2, reviewer_email = $3, scope = $4,
		    status = $5, due_date = $6, completed_at = $7, updated_at = NOW()
		WHERE id = $8::uuid AND org_id = $9::uuid
		RETURNING id::text, org_id::text, title, COALESCE(description,''),
		          status, reviewer_email, COALESCE(scope,''),
		          due_date, completed_at, COALESCE(created_by,''),
		          created_at, updated_at`,
		title, description, reviewerEmail, scope,
		status, dueDate, completedAt,
		id, orgID,
	).Scan(
		&c.ID, &c.OrgID, &c.Title, &c.Description,
		&c.Status, &c.ReviewerEmail, &c.Scope,
		&c.DueDate, &c.CompletedAt, &c.CreatedBy,
		&c.CreatedAt, &c.UpdatedAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, fmt.Errorf("update access review campaign: %w", err)
	}
	return &c, nil
}

// DeleteAccessReviewCampaign removes a campaign (cascades to items).
func (r *Repository) DeleteAccessReviewCampaign(ctx context.Context, orgID, id string) error {
	tag, err := r.db.Exec(ctx, `
		DELETE FROM ck_access_review_campaigns WHERE id = $1::uuid AND org_id = $2::uuid`,
		id, orgID,
	)
	if err != nil {
		return fmt.Errorf("delete access review campaign: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

// --- Access Review Items ---

// ListAccessReviewItems returns all items for a campaign.
func (r *Repository) ListAccessReviewItems(ctx context.Context, orgID, campaignID string) ([]AccessReviewItem, error) {
	rows, err := r.db.Query(ctx, `
		SELECT id::text, campaign_id::text, org_id::text,
		       user_email, access_level, decision,
		       COALESCE(reviewer_comment,''), decided_at, created_at
		FROM ck_access_review_items
		WHERE campaign_id = $1::uuid AND org_id = $2::uuid
		ORDER BY created_at ASC`,
		campaignID, orgID,
	)
	if err != nil {
		return nil, fmt.Errorf("list access review items: %w", err)
	}
	defer rows.Close()

	var items []AccessReviewItem
	for rows.Next() {
		var it AccessReviewItem
		if err := rows.Scan(
			&it.ID, &it.CampaignID, &it.OrgID,
			&it.UserEmail, &it.AccessLevel, &it.Decision,
			&it.ReviewerComment, &it.DecidedAt, &it.CreatedAt,
		); err != nil {
			return nil, fmt.Errorf("scan access review item: %w", err)
		}
		items = append(items, it)
	}
	return items, rows.Err()
}

// CreateAccessReviewItem inserts a new review item and returns the created record.
func (r *Repository) CreateAccessReviewItem(ctx context.Context, orgID string, in CreateAccessReviewItemInput) (*AccessReviewItem, error) {
	var it AccessReviewItem
	err := r.db.QueryRow(ctx, `
		INSERT INTO ck_access_review_items
		  (campaign_id, org_id, user_email, access_level)
		VALUES ($1::uuid, $2::uuid, $3, $4)
		RETURNING id::text, campaign_id::text, org_id::text,
		          user_email, access_level, decision,
		          COALESCE(reviewer_comment,''), decided_at, created_at`,
		in.CampaignID, orgID, in.UserEmail, in.AccessLevel,
	).Scan(
		&it.ID, &it.CampaignID, &it.OrgID,
		&it.UserEmail, &it.AccessLevel, &it.Decision,
		&it.ReviewerComment, &it.DecidedAt, &it.CreatedAt,
	)
	if err != nil {
		return nil, fmt.Errorf("create access review item: %w", err)
	}
	return &it, nil
}

// UpdateAccessReviewItem applies a decision to a review item.
func (r *Repository) UpdateAccessReviewItem(ctx context.Context, orgID, id string, in UpdateAccessReviewItemInput) (*AccessReviewItem, error) {
	var decidedAt *time.Time
	if in.Decision == "approved" || in.Decision == "revoked" {
		now := time.Now().UTC()
		decidedAt = &now
	}

	var it AccessReviewItem
	err := r.db.QueryRow(ctx, `
		UPDATE ck_access_review_items
		SET decision = COALESCE(NULLIF($1,''), decision),
		    reviewer_comment = $2,
		    decided_at = CASE WHEN $1 IN ('approved','revoked') THEN $3 ELSE decided_at END
		WHERE id = $4::uuid AND org_id = $5::uuid
		RETURNING id::text, campaign_id::text, org_id::text,
		          user_email, access_level, decision,
		          COALESCE(reviewer_comment,''), decided_at, created_at`,
		in.Decision, in.ReviewerComment, decidedAt, id, orgID,
	).Scan(
		&it.ID, &it.CampaignID, &it.OrgID,
		&it.UserEmail, &it.AccessLevel, &it.Decision,
		&it.ReviewerComment, &it.DecidedAt, &it.CreatedAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, fmt.Errorf("update access review item: %w", err)
	}
	return &it, nil
}
