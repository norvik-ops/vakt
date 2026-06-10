// Copyright (c) 2026 NorvikOps. All rights reserved.
// SPDX-License-Identifier: Elastic-2.0

// S70-5: Vault Access Review — quarterly review of who can access which secrets.

package vaktvault

import (
	"context"
	"fmt"
	"time"
)

// AccessReview is one quarterly review run for a Vault organisation.
type AccessReview struct {
	ID             string     `json:"id"`
	OrgID          string     `json:"org_id"`
	PeriodLabel    string     `json:"period_label"` // e.g. "Q2/2026"
	Status         string     `json:"status"`       // open | completed
	ReviewedBy     *string    `json:"reviewed_by,omitempty"`
	CompletedAt    *time.Time `json:"completed_at,omitempty"`
	TotalEntries   int        `json:"total_entries"`
	StaleEntries   int        `json:"stale_entries"`
	RevokedEntries int        `json:"revoked_entries"`
	CreatedAt      time.Time  `json:"created_at"`
}

// AccessReviewItem describes one secret's access status within a review.
type AccessReviewItem struct {
	SecretKey      string     `json:"secret_key"`
	EnvID          string     `json:"env_id"`
	ProjectName    string     `json:"project_name,omitempty"`
	LastAccessedAt *time.Time `json:"last_accessed_at,omitempty"`
	IsStale        bool       `json:"is_stale"` // no access in >90 days
}

// ReviewDecision is one keep/revoke decision for completing a review.
type ReviewDecision struct {
	EnvID     string `json:"env_id" validate:"required"`
	SecretKey string `json:"secret_key" validate:"required"`
	Action    string `json:"action" validate:"required,oneof=keep revoke"`
}

// CompleteAccessReviewInput is the request body for POST /vault/access-reviews/:id/complete.
type CompleteAccessReviewInput struct {
	Decisions []ReviewDecision `json:"decisions"`
}

// CurrentQuarterLabel returns a label like "Q2/2026" for the current date.
func CurrentQuarterLabel(t time.Time) string {
	q := (int(t.Month())-1)/3 + 1
	return fmt.Sprintf("Q%d/%d", q, t.Year())
}

// CreateAccessReview starts a new quarterly access review.
func (s *Service) CreateAccessReview(ctx context.Context, orgID string) (*AccessReview, error) {
	label := CurrentQuarterLabel(time.Now().UTC())

	// Count current secrets as the total entries baseline
	var total int
	if err := s.db.QueryRow(ctx, `
		SELECT COUNT(DISTINCT sk.key)
		FROM so_secrets sk
		JOIN so_environments e ON e.id = sk.env_id
		JOIN so_projects p ON p.id = e.project_id
		WHERE p.org_id = $1`, orgID,
	).Scan(&total); err != nil {
		total = 0
	}

	// Count secrets not accessed in >90 days
	var stale int
	if err := s.db.QueryRow(ctx, `
		SELECT COUNT(DISTINCT sk.key)
		FROM so_secrets sk
		JOIN so_environments e ON e.id = sk.env_id
		JOIN so_projects p ON p.id = e.project_id
		WHERE p.org_id = $1
		  AND (sk.last_accessed_at IS NULL OR sk.last_accessed_at < NOW() - INTERVAL '90 days')`,
		orgID,
	).Scan(&stale); err != nil {
		stale = 0
	}

	row := s.db.QueryRow(ctx, `
		INSERT INTO so_access_reviews (org_id, period_label, total_entries, stale_entries)
		VALUES ($1, $2, $3, $4)
		RETURNING id, org_id, period_label, status, reviewed_by, completed_at, total_entries, stale_entries, revoked_entries, created_at`,
		orgID, label, total, stale,
	)
	var r AccessReview
	if err := row.Scan(&r.ID, &r.OrgID, &r.PeriodLabel, &r.Status, &r.ReviewedBy, &r.CompletedAt, &r.TotalEntries, &r.StaleEntries, &r.RevokedEntries, &r.CreatedAt); err != nil {
		return nil, fmt.Errorf("create access review: %w", err)
	}
	return &r, nil
}

// ListAccessReviews returns all access reviews for the organisation.
func (s *Service) ListAccessReviews(ctx context.Context, orgID string) ([]AccessReview, error) {
	rows, err := s.db.Query(ctx, `
		SELECT id, org_id, period_label, status, reviewed_by, completed_at, total_entries, stale_entries, revoked_entries, created_at
		FROM so_access_reviews WHERE org_id = $1 ORDER BY created_at DESC`,
		orgID,
	)
	if err != nil {
		return nil, fmt.Errorf("list access reviews: %w", err)
	}
	defer rows.Close()
	var out []AccessReview
	for rows.Next() {
		var r AccessReview
		if err := rows.Scan(&r.ID, &r.OrgID, &r.PeriodLabel, &r.Status, &r.ReviewedBy, &r.CompletedAt, &r.TotalEntries, &r.StaleEntries, &r.RevokedEntries, &r.CreatedAt); err != nil {
			return nil, fmt.Errorf("scan access review: %w", err)
		}
		out = append(out, r)
	}
	return out, nil
}

// GetAccessReview returns a single access review with its stale items.
func (s *Service) GetAccessReview(ctx context.Context, orgID, reviewID string) (*AccessReview, []AccessReviewItem, error) {
	row := s.db.QueryRow(ctx, `
		SELECT id, org_id, period_label, status, reviewed_by, completed_at, total_entries, stale_entries, revoked_entries, created_at
		FROM so_access_reviews WHERE org_id = $1 AND id = $2`,
		orgID, reviewID,
	)
	var r AccessReview
	if err := row.Scan(&r.ID, &r.OrgID, &r.PeriodLabel, &r.Status, &r.ReviewedBy, &r.CompletedAt, &r.TotalEntries, &r.StaleEntries, &r.RevokedEntries, &r.CreatedAt); err != nil {
		return nil, nil, fmt.Errorf("access review not found: %w", err)
	}

	// Load stale items
	itemRows, err := s.db.Query(ctx, `
		SELECT sk.key, sk.env_id, p.name as project_name, sk.last_accessed_at,
			(sk.last_accessed_at IS NULL OR sk.last_accessed_at < NOW() - INTERVAL '90 days') as is_stale
		FROM so_secrets sk
		JOIN so_environments e ON e.id = sk.env_id
		JOIN so_projects p ON p.id = e.project_id
		WHERE p.org_id = $1
		ORDER BY is_stale DESC, sk.last_accessed_at ASC NULLS FIRST`,
		orgID,
	)
	if err != nil {
		return &r, nil, nil
	}
	defer itemRows.Close()
	var items []AccessReviewItem
	for itemRows.Next() {
		var item AccessReviewItem
		if err := itemRows.Scan(&item.SecretKey, &item.EnvID, &item.ProjectName, &item.LastAccessedAt, &item.IsStale); err != nil {
			continue
		}
		items = append(items, item)
	}
	return &r, items, nil
}

// CompleteAccessReview finalises a review, applying revoke decisions.
func (s *Service) CompleteAccessReview(ctx context.Context, orgID, reviewID, reviewerID string, decisions []ReviewDecision) (*AccessReview, error) {
	revokedCount := 0
	for _, d := range decisions {
		if d.Action != "revoke" {
			continue
		}
		// Delete the secret
		ct, err := s.db.Exec(ctx, `
			DELETE FROM so_secrets sk
			USING so_environments e
			JOIN so_projects p ON p.id = e.project_id
			WHERE sk.env_id = e.id AND sk.key = $1 AND e.id = $2 AND p.org_id = $3`,
			d.SecretKey, d.EnvID, orgID,
		)
		if err == nil && ct.RowsAffected() > 0 {
			revokedCount++
		}
	}

	row := s.db.QueryRow(ctx, `
		UPDATE so_access_reviews SET
			status          = 'completed',
			reviewed_by     = $3,
			completed_at    = NOW(),
			revoked_entries = revoked_entries + $4,
			updated_at      = NOW()
		WHERE org_id = $1 AND id = $2
		RETURNING id, org_id, period_label, status, reviewed_by, completed_at, total_entries, stale_entries, revoked_entries, created_at`,
		orgID, reviewID, reviewerID, revokedCount,
	)
	var r AccessReview
	if err := row.Scan(&r.ID, &r.OrgID, &r.PeriodLabel, &r.Status, &r.ReviewedBy, &r.CompletedAt, &r.TotalEntries, &r.StaleEntries, &r.RevokedEntries, &r.CreatedAt); err != nil {
		return nil, fmt.Errorf("complete access review: %w", err)
	}
	return &r, nil
}
