// Copyright (c) 2026 NorvikOps. All rights reserved.
// SPDX-License-Identifier: Elastic-2.0

package audit

import (
	"context"
	"fmt"
	"time"
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

// ApproveManagementReview approves a management review. Only admin users may approve.
func (s *Service) ApproveManagementReview(ctx context.Context, orgID, id, approverID, userRole string) (ManagementReview, error) {
	if userRole != "admin" {
		return ManagementReview{}, fmt.Errorf("only admin can approve")
	}
	return s.repo.ApproveManagementReview(ctx, orgID, id, approverID)
}

// GetLastManagementReview returns the most recent management review, or nil if none exist.
func (s *Service) GetLastManagementReview(ctx context.Context, orgID string) (*ManagementReview, error) {
	return s.repo.GetLastManagementReview(ctx, orgID)
}

// GetLastManagementReviewDate returns the date of the last review, whether it is overdue, and any error.
// isOverdue is true when no review exists or the last review is older than ManagementReviewOverdueDays.
func (s *Service) GetLastManagementReviewDate(ctx context.Context, orgID string) (dateStr string, isOverdue bool, err error) {
	mr, err := s.repo.GetLastManagementReview(ctx, orgID)
	if err != nil {
		return "", false, err
	}
	if mr == nil {
		return "", true, nil
	}
	t, parseErr := time.Parse("2006-01-02", mr.ReviewDate)
	if parseErr != nil {
		return mr.ReviewDate, true, nil
	}
	overdue := time.Since(t) > ManagementReviewOverdueDays*24*time.Hour
	return mr.ReviewDate, overdue, nil
}
