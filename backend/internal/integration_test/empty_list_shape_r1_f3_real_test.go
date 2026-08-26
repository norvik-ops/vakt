//go:build integration

// Copyright (c) 2026 NorvikOps. All rights reserved.
// SPDX-License-Identifier: Elastic-2.0

package integration_test

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/matharnica/vakt/internal/modules/vaktcomply/audit"
)

// TestR1_F3_AuditEmptyListsSerializeAsArrayNotNull is the regression guard for
// R1-F3-01 (HIGH, SHAPE_MISMATCH), found by the review of Batch-1 fix A.
//
// GET /vaktcomply/audit-program/:id/findings returned `200 null` on the empty
// state: repository_program.go used the nil-able idiom `var findings []T`, the
// service passes the slice through, and handler ListAuditFindings does
// `c.JSON(200, findings)` with no nil-guard. The frontend AuditProgramPage uses
// `const { data: findings = [] }`, whose default only fires on `undefined`, not
// `null` → `findings.length`/`findings.map` throw a TypeError and the
// ErrorBoundary tears AuditProgramPage to its error page. The trigger is the
// normal state: expanding a freshly created audit that has no findings yet.
//
// This is the same class as R1-18-D1 but in the vaktcomply/audit subpackage,
// which was outside that fix's file ownership. The fix normalises each source to
// `make([]T, 0)` at the repository boundary, closing the leak where the nil is
// born so the contract holds for any consumer, not just the current handler.
//
// The variant sweep (I4) over the audit package pinned two same-idiom siblings —
// ListAuditPlans (bare-array GET, no live FE consumer yet) and
// ListManagementReviews (bare-array GET, currently masked by the FE's `!reviews`
// guard). Both are bare top-level array GETs in owned files, so they are
// normalised and pinned here too. ListAuditRecords (make) and
// ListAuditProgramAudits (`[]T{}`) were already non-nil and are left untouched.
//
// Red-on-revert: reverting any source back to `var out []T` makes the matching
// assertion fail (a nil slice on zero rows), verified against real Postgres.
func TestR1_F3_AuditEmptyListsSerializeAsArrayNotNull(t *testing.T) {
	pool, orgID, cleanup := bootPostgresWithOrg(t)
	defer cleanup()
	ctx := context.Background()

	repo := audit.NewRepository(pool)

	// The reported live crash: findings for an audit with zero findings. A random
	// audit id yields zero rows — exactly the empty state the page hits.
	findings, err := repo.ListAuditFindings(ctx, orgID, uuid.NewString())
	require.NoError(t, err)
	assert.NotNil(t, findings, "ListAuditFindings must return [] not nil on the empty state")
	assert.Len(t, findings, 0)

	// Same-class variant: bare-array GET /vaktcomply/audit-plans.
	plans, err := repo.ListAuditPlans(ctx, orgID)
	require.NoError(t, err)
	assert.NotNil(t, plans, "ListAuditPlans must return [] not nil on the empty state")
	assert.Len(t, plans, 0)

	// Same-class variant: bare-array GET /vaktcomply/management-reviews.
	reviews, err := repo.ListManagementReviews(ctx, orgID)
	require.NoError(t, err)
	assert.NotNil(t, reviews, "ListManagementReviews must return [] not nil on the empty state")
	assert.Len(t, reviews, 0)
}
