//go:build integration

// Copyright (c) 2026 NorvikOps. All rights reserved.
// SPDX-License-Identifier: Elastic-2.0

package integration_test

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	audit "github.com/matharnica/vakt/internal/modules/vaktcomply/audit"
)

// TestCreateManagementReview_ReturnsInsertedRow is the regression guard for the
// born-broken CreateManagementReview query. The original SQL was:
//
//	WITH ins AS (INSERT INTO ck_management_reviews (...) RETURNING id)
//	SELECT ... FROM ck_management_reviews mr JOIN ins ON mr.id = ins.id
//
// A data-modifying CTE and the outer table scan share one snapshot, so the outer
// scan of ck_management_reviews could not see the row the CTE had just inserted —
// the JOIN returned zero rows, scanManagementReview got pgx.ErrNoRows, and the
// handler mapped that to HTTP 500. Worse, the row WAS inserted, so every retry
// created a duplicate. A validation-gated empty-body sweep never reached the
// INSERT (review_date is required → 422 first); only a happy-path create with a
// real body triggered it.
//
// The fix reads the freshly inserted row via INSERT ... RETURNING directly. This
// test asserts the create returns the row with all defaults populated.
func TestCreateManagementReview_ReturnsInsertedRow(t *testing.T) {
	pool, orgID, cleanup := bootPostgresWithOrg(t)
	defer cleanup()
	ctx := context.Background()

	var userID string
	require.NoError(t, pool.QueryRow(ctx, `
		INSERT INTO users (email) VALUES ('mr-author@example.org')
		RETURNING id::text`).Scan(&userID))

	repo := audit.NewRepository(pool)

	mr, err := repo.CreateManagementReview(ctx, orgID, userID, audit.CreateManagementReviewInput{
		ReviewDate:     "2026-07-11",
		ReviewType:     "annual",
		ParticipantIDs: json.RawMessage("[]"),
	})
	require.NoError(t, err, "create must return the inserted row, not pgx.ErrNoRows")

	assert.NotEmpty(t, mr.ID, "returned review must carry its generated id")
	assert.Equal(t, orgID, mr.OrgID)
	assert.Equal(t, "2026-07-11", mr.ReviewDate)
	assert.Equal(t, "annual", mr.ReviewType)
	assert.Equal(t, "draft", mr.Status, "status default must be applied and visible in RETURNING")
	assert.Equal(t, userID, mr.CreatedBy)

	// And it must have inserted exactly one row — not zero (old bug) or a duplicate.
	var n int
	require.NoError(t, pool.QueryRow(ctx,
		`SELECT count(*) FROM ck_management_reviews WHERE org_id = $1::uuid`, orgID).Scan(&n))
	assert.Equal(t, 1, n, "exactly one row must exist after a single create")
}
