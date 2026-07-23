//go:build integration

// Copyright (c) 2026 NorvikOps. All rights reserved.
// SPDX-License-Identifier: Elastic-2.0

package integration_test

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/matharnica/vakt/internal/modules/vaktaware"
	auditpkg "github.com/matharnica/vakt/internal/modules/vaktcomply/audit"
)

// TestEmptyListsSerializeAsArrayNotNull is the regression guard for S131-D3
// (R-C09/R-H26/R-H27, D18-01/02/03): list repositories that used `var x []T`
// returned a nil slice on the empty state, which serialises to JSON `null`, not
// `[]`. The frontend's react-query `= []` destructuring default catches undefined
// but NOT null, so `audits.length` / `report.campaigns.length` crashed the whole
// page for every new customer. The fix initialises to `[]T{}`; this test asserts
// the empty-DB result is a non-nil empty slice for the two confirmed-live crashes.
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

	// D18-03: training-report campaign summaries — empty org must yield [] not nil.
	awareRepo := vaktaware.NewRepository(pool)
	from := time.Date(2020, 1, 1, 0, 0, 0, 0, time.UTC)
	to := time.Date(2030, 1, 1, 0, 0, 0, 0, time.UTC)
	campaigns, err := awareRepo.ListCampaignSummariesForReport(ctx, orgID, from, to)
	require.NoError(t, err)
	assert.NotNil(t, campaigns, "ListCampaignSummariesForReport must return [] not nil on the empty state")
	assert.Len(t, campaigns, 0)
}
