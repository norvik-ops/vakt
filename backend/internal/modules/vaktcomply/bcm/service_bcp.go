// Copyright (c) 2026 NorvikOps. All rights reserved.
// SPDX-License-Identifier: Elastic-2.0

package bcm

import (
	"context"
	"time"
)

// CreateBCPPlan creates a new BCP plan for the organisation.
func (s *Service) CreateBCPPlan(ctx context.Context, orgID string, in CreateBCPPlanInput) (BCPPlan, error) {
	return s.repo.CreateBCPPlan(ctx, orgID, in)
}

// ListBCPPlans returns all BCP plans for the organisation.
func (s *Service) ListBCPPlans(ctx context.Context, orgID string) ([]BCPPlan, error) {
	return s.repo.ListBCPPlans(ctx, orgID)
}

// GetBCPPlan returns a single BCP plan by ID.
func (s *Service) GetBCPPlan(ctx context.Context, orgID, id string) (BCPPlan, error) {
	return s.repo.GetBCPPlan(ctx, orgID, id)
}

// UpdateBCPPlan updates an existing BCP plan.
func (s *Service) UpdateBCPPlan(ctx context.Context, orgID, id string, in UpdateBCPPlanInput) (BCPPlan, error) {
	return s.repo.UpdateBCPPlan(ctx, orgID, id, in)
}

// DeleteBCPPlan removes a BCP plan.
func (s *Service) DeleteBCPPlan(ctx context.Context, orgID, id string) error {
	return s.repo.DeleteBCPPlan(ctx, orgID, id)
}

// AddBCPTest logs a new test result for the given plan.
// It verifies the plan belongs to the organisation before inserting.
func (s *Service) AddBCPTest(ctx context.Context, orgID, planID string, in CreateBCPTestInput) (BCPTest, error) {
	if _, err := s.repo.GetBCPPlan(ctx, orgID, planID); err != nil {
		return BCPTest{}, err
	}
	return s.repo.AddBCPTest(ctx, orgID, planID, in)
}

// ListBCPTests returns all test records for a BCP plan.
// It verifies the plan belongs to the organisation before querying.
func (s *Service) ListBCPTests(ctx context.Context, orgID, planID string) ([]BCPTest, error) {
	if _, err := s.repo.GetBCPPlan(ctx, orgID, planID); err != nil {
		return nil, err
	}
	return s.repo.ListBCPTests(ctx, orgID, planID)
}

// BCPTestStaleDays is the number of days after which a BCDR test is considered stale.
const BCPTestStaleDays = 365

// BCPTestIsStale returns true if no test date is provided or the given date is
// older than BCPTestStaleDays (12 months).
func BCPTestIsStale(latestDate string) bool {
	if latestDate == "" {
		return true
	}
	t, err := time.Parse("2006-01-02", latestDate)
	if err != nil {
		return true
	}
	return time.Since(t) > BCPTestStaleDays*24*time.Hour
}
