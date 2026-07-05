package vaktcomply

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/matharnica/vakt/internal/db"
)

func campaignFromCk(r db.CkAccessReviewCampaigns) AccessReviewCampaign {
	return AccessReviewCampaign{
		ID:            r.ID,
		OrgID:         r.OrgID,
		Title:         r.Title,
		Description:   r.Description.String,
		Status:        r.Status,
		ReviewerEmail: r.ReviewerEmail,
		Scope:         r.Scope.String,
		DueDate:       ckTsToTimePtr(r.DueDate),
		CompletedAt:   ckTsToTimePtr(r.CompletedAt),
		CreatedBy:     r.CreatedBy.String,
		CreatedAt:     ckTsToTime(r.CreatedAt),
		UpdatedAt:     ckTsToTime(r.UpdatedAt),
	}
}

// parseAccessReviewDueDate accepts RFC3339 or YYYY-MM-DD.
func parseAccessReviewDueDate(s string) (*time.Time, error) {
	if s == "" {
		return nil, nil
	}
	if t, err := time.Parse(time.RFC3339, s); err == nil {
		return &t, nil
	}
	if t, err := time.Parse("2006-01-02", s); err == nil {
		return &t, nil
	}
	return nil, fmt.Errorf("invalid due_date format: %s", s)
}

// ListAccessReviewCampaigns returns all campaigns for an organisation ordered by created_at DESC.
func (r *Repository) ListAccessReviewCampaigns(ctx context.Context, orgID string) ([]AccessReviewCampaign, error) {
	rows, err := r.q.ListCKAccessReviewCampaigns(ctx, orgID)
	if err != nil {
		return nil, fmt.Errorf("list access review campaigns: %w", err)
	}
	out := make([]AccessReviewCampaign, 0, len(rows))
	for _, row := range rows {
		out = append(out, campaignFromCk(db.CkAccessReviewCampaigns(row)))
	}
	return out, nil
}

// GetAccessReviewCampaign returns a single campaign by ID. Returns ErrNotFound if absent.
func (r *Repository) GetAccessReviewCampaign(ctx context.Context, orgID, id string) (*AccessReviewCampaign, error) {
	row, err := r.q.GetCKAccessReviewCampaign(ctx, db.GetCKAccessReviewCampaignParams{ID: id, OrgID: orgID})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, fmt.Errorf("get access review campaign: %w", err)
	}
	c := campaignFromCk(db.CkAccessReviewCampaigns(row))
	return &c, nil
}

// CreateAccessReviewCampaign inserts a new campaign and returns the created record.
func (r *Repository) CreateAccessReviewCampaign(ctx context.Context, orgID string, in CreateAccessReviewCampaignInput) (*AccessReviewCampaign, error) {
	var dueDate *time.Time
	if in.DueDate != nil {
		t, err := parseAccessReviewDueDate(*in.DueDate)
		if err != nil {
			return nil, err
		}
		dueDate = t
	}
	row, err := r.q.CreateCKAccessReviewCampaign(ctx, db.CreateCKAccessReviewCampaignParams{
		OrgID:         orgID,
		Title:         in.Title,
		Description:   ckOptText(in.Description),
		ReviewerEmail: in.ReviewerEmail,
		Scope:         ckOptText(in.Scope),
		DueDate:       ckOptTsPtr(dueDate),
	})
	if err != nil {
		return nil, fmt.Errorf("create access review campaign: %w", err)
	}
	c := campaignFromCk(db.CkAccessReviewCampaigns(row))
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
	dueDate := cur.DueDate

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
			t, err := parseAccessReviewDueDate(*in.DueDate)
			if err != nil {
				return nil, err
			}
			dueDate = t
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

	row, err := r.q.UpdateCKAccessReviewCampaign(ctx, db.UpdateCKAccessReviewCampaignParams{
		Title:         title,
		Description:   ckOptText(description),
		ReviewerEmail: reviewerEmail,
		Scope:         ckOptText(scope),
		Status:        status,
		DueDate:       ckOptTsPtr(dueDate),
		CompletedAt:   ckOptTsPtr(completedAt),
		ID:            id,
		OrgID:         orgID,
	})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, fmt.Errorf("update access review campaign: %w", err)
	}
	c := campaignFromCk(db.CkAccessReviewCampaigns(row))
	return &c, nil
}

// DeleteAccessReviewCampaign removes a campaign (cascades to items).
func (r *Repository) DeleteAccessReviewCampaign(ctx context.Context, orgID, id string) error {
	n, err := r.q.DeleteCKAccessReviewCampaign(ctx, db.DeleteCKAccessReviewCampaignParams{ID: id, OrgID: orgID})
	if err != nil {
		return fmt.Errorf("delete access review campaign: %w", err)
	}
	if n == 0 {
		return ErrNotFound
	}
	return nil
}

// --- Access Review Items ---

func itemFromCk(r db.CkAccessReviewItems) AccessReviewItem {
	return AccessReviewItem{
		ID:              r.ID,
		CampaignID:      r.CampaignID,
		OrgID:           r.OrgID,
		UserEmail:       r.UserEmail,
		AccessLevel:     r.AccessLevel,
		Decision:        r.Decision,
		ReviewerComment: r.ReviewerComment.String,
		DecidedAt:       ckTsToTimePtr(r.DecidedAt),
		CreatedAt:       ckTsToTime(r.CreatedAt),
	}
}

// ListAccessReviewItems returns all items for a campaign.
func (r *Repository) ListAccessReviewItems(ctx context.Context, orgID, campaignID string) ([]AccessReviewItem, error) {
	rows, err := r.q.ListCKAccessReviewItems(ctx, db.ListCKAccessReviewItemsParams{
		CampaignID: campaignID,
		OrgID:      orgID,
	})
	if err != nil {
		return nil, fmt.Errorf("list access review items: %w", err)
	}
	out := make([]AccessReviewItem, 0, len(rows))
	for _, row := range rows {
		out = append(out, itemFromCk(db.CkAccessReviewItems(row)))
	}
	return out, nil
}

// CreateAccessReviewItem inserts a new review item and returns the created record.
func (r *Repository) CreateAccessReviewItem(ctx context.Context, orgID string, in CreateAccessReviewItemInput) (*AccessReviewItem, error) {
	row, err := r.q.CreateCKAccessReviewItem(ctx, db.CreateCKAccessReviewItemParams{
		CampaignID:  in.CampaignID,
		OrgID:       orgID,
		UserEmail:   in.UserEmail,
		AccessLevel: in.AccessLevel,
	})
	if err != nil {
		return nil, fmt.Errorf("create access review item: %w", err)
	}
	it := itemFromCk(db.CkAccessReviewItems(row))
	return &it, nil
}

// UpdateAccessReviewItem applies a decision to a review item.
func (r *Repository) UpdateAccessReviewItem(ctx context.Context, orgID, id string, in UpdateAccessReviewItemInput) (*AccessReviewItem, error) {
	var decidedAt pgtype.Timestamptz
	if in.Decision == "approved" || in.Decision == "revoked" {
		decidedAt = pgtype.Timestamptz{Time: time.Now().UTC(), Valid: true}
	}
	row, err := r.q.UpdateCKAccessReviewItem(ctx, db.UpdateCKAccessReviewItemParams{
		Decision:        in.Decision,
		ReviewerComment: in.ReviewerComment,
		DecidedAt:       decidedAt,
		ID:              id,
		OrgID:           orgID,
	})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, fmt.Errorf("update access review item: %w", err)
	}
	it := itemFromCk(db.CkAccessReviewItems(row))
	return &it, nil
}
