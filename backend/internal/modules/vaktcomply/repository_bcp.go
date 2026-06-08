// Copyright (c) 2026 NorvikOps. All rights reserved.
// SPDX-License-Identifier: Elastic-2.0

package vaktcomply

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5/pgtype"

	db "github.com/matharnica/vakt/internal/db"
)

// CreateBCPPlan inserts a new BCP plan for the given organisation.
func (r *Repository) CreateBCPPlan(ctx context.Context, orgID string, in CreateBCPPlanInput) (BCPPlan, error) {
	status := "draft"
	if in.Status != "" {
		status = in.Status
	}
	version := "1.0"
	if in.Version != "" {
		version = in.Version
	}
	row, err := r.q.CreateCKBCPPlan(ctx, db.CreateCKBCPPlanParams{
		OrgID:   orgID,
		Title:   in.Title,
		Scope:   in.Scope,
		Version: version,
		Status:  status,
		Owner:   in.Owner,
	})
	if err != nil {
		return BCPPlan{}, fmt.Errorf("create bcp plan: %w", err)
	}
	return bcpPlanFromRow(row), nil
}

// ListBCPPlans returns all BCP plans for an organisation.
func (r *Repository) ListBCPPlans(ctx context.Context, orgID string) ([]BCPPlan, error) {
	rows, err := r.q.ListCKBCPPlans(ctx, orgID)
	if err != nil {
		return nil, fmt.Errorf("list bcp plans: %w", err)
	}
	out := make([]BCPPlan, len(rows))
	for i, row := range rows {
		out[i] = bcpPlanFromRow(row)
	}
	return out, nil
}

// GetBCPPlan returns a single BCP plan by ID within an organisation.
func (r *Repository) GetBCPPlan(ctx context.Context, orgID, id string) (BCPPlan, error) {
	row, err := r.q.GetCKBCPPlan(ctx, db.GetCKBCPPlanParams{ID: id, OrgID: orgID})
	if err != nil {
		return BCPPlan{}, fmt.Errorf("get bcp plan: %w", err)
	}
	return bcpPlanFromRow(row), nil
}

// UpdateBCPPlan updates an existing BCP plan.
func (r *Repository) UpdateBCPPlan(ctx context.Context, orgID, id string, in UpdateBCPPlanInput) (BCPPlan, error) {
	row, err := r.q.UpdateCKBCPPlan(ctx, db.UpdateCKBCPPlanParams{
		ID:      id,
		OrgID:   orgID,
		Title:   in.Title,
		Scope:   in.Scope,
		Version: in.Version,
		Status:  in.Status,
		Owner:   in.Owner,
	})
	if err != nil {
		return BCPPlan{}, fmt.Errorf("update bcp plan: %w", err)
	}
	return bcpPlanFromRow(row), nil
}

// DeleteBCPPlan removes a BCP plan and returns an error if not found.
func (r *Repository) DeleteBCPPlan(ctx context.Context, orgID, id string) error {
	n, err := r.q.DeleteCKBCPPlan(ctx, db.DeleteCKBCPPlanParams{ID: id, OrgID: orgID})
	if err != nil {
		return fmt.Errorf("delete bcp plan: %w", err)
	}
	if n == 0 {
		return fmt.Errorf("bcp plan not found")
	}
	return nil
}

// AddBCPTest logs a test result against a BCP plan.
func (r *Repository) AddBCPTest(ctx context.Context, orgID, planID string, in CreateBCPTestInput) (BCPTest, error) {
	var testDate pgtype.Date
	if err := testDate.Scan(in.TestDate); err != nil {
		return BCPTest{}, fmt.Errorf("parse test_date: %w", err)
	}
	row, err := r.q.CreateCKBCPTest(ctx, db.CreateCKBCPTestParams{
		OrgID:    orgID,
		PlanID:   planID,
		TestDate: testDate,
		TestType: in.TestType,
		Outcome:  in.Outcome,
		Findings: in.Findings,
	})
	if err != nil {
		return BCPTest{}, fmt.Errorf("add bcp test: %w", err)
	}
	return bcpTestFromRow(row), nil
}

// ListBCPTests returns all test records for a BCP plan.
func (r *Repository) ListBCPTests(ctx context.Context, orgID, planID string) ([]BCPTest, error) {
	rows, err := r.q.ListCKBCPTests(ctx, db.ListCKBCPTestsParams{PlanID: planID, OrgID: orgID})
	if err != nil {
		return nil, fmt.Errorf("list bcp tests: %w", err)
	}
	out := make([]BCPTest, len(rows))
	for i, row := range rows {
		out[i] = bcpTestFromRow(row)
	}
	return out, nil
}

// GetLatestBCPTest returns the most recent test record for a plan, or nil if none exists.
func (r *Repository) GetLatestBCPTest(ctx context.Context, orgID, planID string) (*BCPTest, error) {
	row, err := r.q.GetLatestCKBCPTest(ctx, db.GetLatestCKBCPTestParams{PlanID: planID, OrgID: orgID})
	if err != nil {
		return nil, fmt.Errorf("get latest bcp test: %w", err)
	}
	t := bcpTestFromRow(row)
	return &t, nil
}

// bcpPlanFromRow maps a db.CkBcpPlans row to the BCPPlan domain model.
func bcpPlanFromRow(row db.CkBcpPlans) BCPPlan {
	return BCPPlan{
		ID:        row.ID,
		OrgID:     row.OrgID,
		Title:     row.Title,
		Scope:     row.Scope,
		Version:   row.Version,
		Status:    row.Status,
		Owner:     row.Owner,
		CreatedAt: ckTsToTime(row.CreatedAt),
		UpdatedAt: ckTsToTime(row.UpdatedAt),
	}
}

// bcpTestFromRow maps a db.CkBcpTests row to the BCPTest domain model.
func bcpTestFromRow(row db.CkBcpTests) BCPTest {
	var dateStr string
	if row.TestDate.Valid {
		dateStr = row.TestDate.Time.Format("2006-01-02")
	}
	return BCPTest{
		ID:        row.ID,
		OrgID:     row.OrgID,
		PlanID:    row.PlanID,
		TestDate:  dateStr,
		TestType:  row.TestType,
		Outcome:   row.Outcome,
		Findings:  row.Findings,
		CreatedAt: ckTsToTime(row.CreatedAt),
	}
}
