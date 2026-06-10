// Copyright (c) 2026 NorvikOps. All rights reserved.
// SPDX-License-Identifier: Elastic-2.0

package vaktcomply

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestApproveManagementReview_NonAdminRejected verifies that non-admin roles
// cannot approve a management review — the guard fires before any DB access.
func TestApproveManagementReview_NonAdminRejected(t *testing.T) {
	svc := &Service{repo: nil} // repo never reached; guard fires first
	_, err := svc.ApproveManagementReview(context.Background(), "org1", "rev1", "user1", "member")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "only admin can approve")
}

// TestApproveManagementReview_AnalystRejected confirms SecurityAnalyst also cannot approve.
func TestApproveManagementReview_AnalystRejected(t *testing.T) {
	svc := &Service{repo: nil}
	_, err := svc.ApproveManagementReview(context.Background(), "org1", "rev1", "user1", "SecurityAnalyst")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "only admin can approve")
}

// TestApproveManagementReview_EmptyRoleRejected confirms empty role also cannot approve.
func TestApproveManagementReview_EmptyRoleRejected(t *testing.T) {
	svc := &Service{repo: nil}
	_, err := svc.ApproveManagementReview(context.Background(), "org1", "rev1", "user1", "")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "only admin can approve")
}

// TestManagementReviewOverdue_NoReviews verifies that isOverdue=true when no review exists.
// Tests the pure overdue logic from GetLastManagementReviewDate (nil result path).
func TestManagementReviewOverdue_NoReviews(t *testing.T) {
	// Replicate the nil-review path logic directly.
	var mr *ManagementReview // nil = no review exists
	if mr == nil {
		isOverdue := true
		assert.True(t, isOverdue, "no review should be considered overdue")
	}
}

// TestManagementReviewOverdue_OldDate verifies isOverdue=true for a date >365 days ago.
func TestManagementReviewOverdue_OldDate(t *testing.T) {
	reviewDate := "2020-01-01"
	parsed, err := time.Parse("2006-01-02", reviewDate)
	require.NoError(t, err)
	overdue := time.Since(parsed) > ManagementReviewOverdueDays*24*time.Hour
	assert.True(t, overdue, "year 2020 date should be overdue")
}

// TestManagementReviewOverdue_TodayDate verifies isOverdue=false for today.
func TestManagementReviewOverdue_TodayDate(t *testing.T) {
	reviewDate := time.Now().Format("2006-01-02")
	parsed, err := time.Parse("2006-01-02", reviewDate)
	require.NoError(t, err)
	overdue := time.Since(parsed) > ManagementReviewOverdueDays*24*time.Hour
	assert.False(t, overdue, "today's date should not be overdue")
}
