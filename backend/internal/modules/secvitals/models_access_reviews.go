// Copyright (c) 2026 NorvikOps. All rights reserved.
// SPDX-License-Identifier: Elastic-2.0

package secvitals

import "time"

// AccessReviewCampaign represents a periodic attestation campaign for user access.
type AccessReviewCampaign struct {
	ID            string     `json:"id"`
	OrgID         string     `json:"org_id"`
	Title         string     `json:"title"`
	Description   string     `json:"description,omitempty"`
	Status        string     `json:"status"`
	ReviewerEmail string     `json:"reviewer_email"`
	Scope         string     `json:"scope,omitempty"`
	DueDate       *time.Time `json:"due_date,omitempty"`
	CompletedAt   *time.Time `json:"completed_at,omitempty"`
	CreatedBy     string     `json:"created_by,omitempty"`
	CreatedAt     time.Time  `json:"created_at"`
	UpdatedAt     time.Time  `json:"updated_at"`
}

// AccessReviewItem represents a single access attestation entry within a campaign.
type AccessReviewItem struct {
	ID              string     `json:"id"`
	CampaignID      string     `json:"campaign_id"`
	OrgID           string     `json:"org_id"`
	UserEmail       string     `json:"user_email"`
	AccessLevel     string     `json:"access_level"`
	Decision        string     `json:"decision"`
	ReviewerComment string     `json:"reviewer_comment,omitempty"`
	DecidedAt       *time.Time `json:"decided_at,omitempty"`
	CreatedAt       time.Time  `json:"created_at"`
}

// CreateAccessReviewCampaignInput holds validated input for creating a campaign.
type CreateAccessReviewCampaignInput struct {
	Title         string  `json:"title"          validate:"required,max=255"`
	Description   string  `json:"description"    validate:"max=2000"`
	ReviewerEmail string  `json:"reviewer_email" validate:"required,email"`
	Scope         string  `json:"scope"          validate:"max=500"`
	DueDate       *string `json:"due_date"`
}

// UpdateAccessReviewCampaignInput holds validated input for updating a campaign.
type UpdateAccessReviewCampaignInput struct {
	Title         string  `json:"title"`
	Description   string  `json:"description"`
	ReviewerEmail string  `json:"reviewer_email"`
	Scope         string  `json:"scope"`
	DueDate       *string `json:"due_date"`
	Status        string  `json:"status" validate:"omitempty,oneof=draft active completed cancelled"`
}

// CreateAccessReviewItemInput holds validated input for creating a review item.
type CreateAccessReviewItemInput struct {
	CampaignID  string `json:"campaign_id"  validate:"required,uuid"`
	UserEmail   string `json:"user_email"   validate:"required,email"`
	AccessLevel string `json:"access_level" validate:"required,max=100"`
}

// UpdateAccessReviewItemInput holds validated input for updating a review item decision.
type UpdateAccessReviewItemInput struct {
	Decision        string `json:"decision"         validate:"omitempty,oneof=approved revoked pending"`
	ReviewerComment string `json:"reviewer_comment" validate:"max=2000"`
}
