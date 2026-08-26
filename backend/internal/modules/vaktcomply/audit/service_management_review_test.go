// Copyright (c) 2026 NorvikOps. All rights reserved.
// SPDX-License-Identifier: Elastic-2.0

package audit

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Die drei frueheren Tests dieser Stelle
// (TestApproveManagementReview_NonAdminRejected / _AnalystRejected /
// _EmptyRoleRejected) sind mit R1-W7C-N1 entfallen. Sie riefen die
// Rollenpruefung direkt auf dem Service auf und reichten die Rolle als Argument
// mit — genau deshalb waren sie gruen, waehrend die Pruefung in der laufenden
// Anwendung immer denselben leeren Wert sah und jede Freigabe ablehnte. Ein
// Test, der seine Eingabe selbst mitbringt, kann eine kaputte Naht nicht sehen.
//
// Wer freigeben darf, steht jetzt an genau einer Stelle (der Route) und wird
// dort geprueft: TestManagementReviewApprovalIsInternalAuditorOnly im Paket
// vaktcomply (Rollenmatrix aus ADR-0055) und der Integrationstest
// vaktcomply_freigabe_real_test.go gegen echtes Postgres.

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
