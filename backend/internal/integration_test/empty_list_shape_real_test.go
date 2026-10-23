//go:build integration

// Copyright (c) 2026 NorvikOps. All rights reserved.
// SPDX-License-Identifier: Elastic-2.0

package integration_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	auditpkg "github.com/matharnica/vakt/internal/modules/vaktcomply/audit"
)

// TestEmptyListsSerializeAsArrayNotNull is the regression guard for S131-D3
// (R-C09/R-H26/R-H27, D18-01/02/03): list repositories that used `var x []T`
// returned a nil slice on the empty state, which serialises to JSON `null`, not
// `[]`. The frontend's react-query `= []` destructuring default catches undefined
// but NOT null, so `audits.length` / `report.campaigns.length` crashed the whole
// page for every new customer. The fix initialises to `[]T{}`; this test asserts
// the empty-DB result is a non-nil empty slice. (The second confirmed case, the
// Aware training report, left with the archived module.)
func TestEmptyListsSerializeAsArrayNotNull(t *testing.T) {
	pool, orgID, cleanup := bootPostgresWithOrg(t)
	defer cleanup()
	ctx := context.Background()

	// D18-02: audit program — empty org must yield [] not nil.
	auditRepo := auditpkg.NewRepository(pool)
	audits, err := auditRepo.ListAuditProgramAudits(ctx, orgID)
	require.NoError(t, err)
	assert.NotNil(t, audits, "ListAuditProgramAudits must return [] not nil on the empty state")
	assert.Len(t, audits, 0)
}
