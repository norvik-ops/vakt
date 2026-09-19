// Copyright (c) 2026 NorvikOps. All rights reserved.
// SPDX-License-Identifier: Elastic-2.0

package audit

import (
	"context"
)

// ManagementReviewOverdueDays is the number of days after which a management review is considered overdue.
const ManagementReviewOverdueDays = 365

// CreateManagementReview creates a new management review for the organisation.
func (s *Service) CreateManagementReview(ctx context.Context, orgID, userID string, in CreateManagementReviewInput) (ManagementReview, error) {
	return s.repo.CreateManagementReview(ctx, orgID, userID, in)
}

// GetManagementReview returns a single management review by ID.
func (s *Service) GetManagementReview(ctx context.Context, orgID, id string) (ManagementReview, error) {
	return s.repo.GetManagementReview(ctx, orgID, id)
}

// ListManagementReviews returns all management reviews for the organisation.
func (s *Service) ListManagementReviews(ctx context.Context, orgID string) ([]ManagementReview, error) {
	return s.repo.ListManagementReviews(ctx, orgID)
}

// UpdateManagementReviewInputs updates the input-phase fields of a management review.
func (s *Service) UpdateManagementReviewInputs(ctx context.Context, orgID, id string, in UpdateManagementReviewInputsInput) (ManagementReview, error) {
	return s.repo.UpdateManagementReviewInputs(ctx, orgID, id, in)
}

// UpdateManagementReviewOutputs updates the output-phase fields of a management review.
func (s *Service) UpdateManagementReviewOutputs(ctx context.Context, orgID, id string, in UpdateManagementReviewOutputsInput) (ManagementReview, error) {
	return s.repo.UpdateManagementReviewOutputs(ctx, orgID, id, in)
}

// ApproveManagementReview approves a management review.
//
// Wer freigeben darf, entscheidet ausschliesslich die Route:
// `auth.RequireRole("InternalAuditor")` in routes.go, so wie es ADR-0055
// festgelegt hat (Rollenmatrix dort: Management-Review freigeben — Admin ❌,
// SecurityAnalyst ❌, InternalAuditor ✅). Die frueher hier stehende Pruefung
// `userRole != "admin"` sagte das Gegenteil und war ausserdem unerreichbar:
// sie las `c.Get("role")`, einen Schluessel, den niemand setzt.
func (s *Service) ApproveManagementReview(ctx context.Context, orgID, id, approverID string) (ManagementReview, error) {
	return s.repo.ApproveManagementReview(ctx, orgID, id, approverID)
}

// GetLastManagementReview returns the most recent management review, or nil if none exist.
func (s *Service) GetLastManagementReview(ctx context.Context, orgID string) (*ManagementReview, error) {
	return s.repo.GetLastManagementReview(ctx, orgID)
}
