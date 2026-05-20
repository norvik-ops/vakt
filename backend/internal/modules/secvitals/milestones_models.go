// Copyright (c) 2026 NorvikOps. All rights reserved.
// SPDX-License-Identifier: Elastic-2.0

package secvitals

import "time"

// AuditMilestone represents a certification target, audit event, or deadline on the org calendar.
type AuditMilestone struct {
	ID            string    `json:"id"`
	OrgID         string    `json:"org_id"`
	FrameworkID   *string   `json:"framework_id,omitempty"`
	Title         string    `json:"title"`
	Description   string    `json:"description,omitempty"`
	MilestoneDate string    `json:"milestone_date"` // DATE — stored as YYYY-MM-DD string
	MilestoneType string    `json:"milestone_type"`
	Status        string    `json:"status"`
	CreatedBy     *string   `json:"created_by,omitempty"`
	CreatedAt     time.Time `json:"created_at"`
	UpdatedAt     time.Time `json:"updated_at"`
	// DaysRemaining is computed server-side for convenience.
	DaysRemaining *int `json:"days_remaining,omitempty"`
}

// CreateMilestoneInput holds validated input for creating an audit milestone.
type CreateMilestoneInput struct {
	FrameworkID   string `json:"framework_id"    validate:"omitempty,uuid"`
	Title         string `json:"title"           validate:"required,max=255"`
	Description   string `json:"description"     validate:"max=2000"`
	MilestoneDate string `json:"milestone_date"  validate:"required"`
	MilestoneType string `json:"milestone_type"  validate:"required,oneof=internal_audit external_audit certification_target review_deadline training_deadline custom"`
}

// UpdateMilestoneInput holds validated input for updating an audit milestone.
type UpdateMilestoneInput struct {
	Title         *string `json:"title"           validate:"omitempty,max=255"`
	Description   *string `json:"description"     validate:"omitempty,max=2000"`
	MilestoneDate *string `json:"milestone_date"`
	MilestoneType *string `json:"milestone_type"  validate:"omitempty,oneof=internal_audit external_audit certification_target review_deadline training_deadline custom"`
	Status        *string `json:"status"          validate:"omitempty,oneof=upcoming completed missed cancelled"`
}
