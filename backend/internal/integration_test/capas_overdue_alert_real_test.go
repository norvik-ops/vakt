//go:build integration

// Copyright (c) 2026 NorvikOps. All rights reserved.
// SPDX-License-Identifier: Elastic-2.0

package integration_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/matharnica/vakt/internal/modules/vaktcomply"
)

// TestListOverdueEffectivenessChecks_SchemaMatches is a regression test for the
// 2026-06-14 incident: ListOverdueEffectivenessChecks selected the column
// ck_capas.created_by, which exists in no migration, so the daily
// effectiveness-check-overdue alert worker job failed every run with
//   ERROR: column "created_by" does not exist
//
// Booting a real Postgres and running the query against the migrated schema
// would have caught this at CI time. The test seeds one matching (overdue,
// unconfirmed, major_nc) CAPA and two non-matching ones, then asserts the
// query succeeds and returns exactly the overdue row.
func TestListOverdueEffectivenessChecks_SchemaMatches(t *testing.T) {
	if testing.Short() {
		t.Skip("integration: -short mode")
	}
	ctx := context.Background()
	pool, teardown := bootPostgres(t)
	defer teardown()

	var orgID string
	require.NoError(t, pool.QueryRow(ctx, `
		INSERT INTO organizations (name, slug) VALUES ('Acme', 'acme')
		RETURNING id::text
	`).Scan(&orgID))

	// Seed three CAPAs:
	//  - overdue major_nc, effectiveness not confirmed  -> MUST be returned
	//  - overdue major_nc, but effectiveness confirmed   -> excluded (confirmed)
	//  - future check date, major_nc, not confirmed      -> excluded (not overdue)
	var overdueID string
	require.NoError(t, pool.QueryRow(ctx, `
		INSERT INTO ck_capas (org_id, source_type, title, nc_classification,
		                      effectiveness_check_date, effectiveness_confirmed)
		VALUES ($1::uuid, 'manual', 'Overdue major NC', 'major_nc',
		        CURRENT_DATE - INTERVAL '3 days', NULL)
		RETURNING id::text
	`, orgID).Scan(&overdueID))

	_, err := pool.Exec(ctx, `
		INSERT INTO ck_capas (org_id, source_type, title, nc_classification,
		                      effectiveness_check_date, effectiveness_confirmed)
		VALUES
			($1::uuid, 'manual', 'Already confirmed', 'major_nc',
			 CURRENT_DATE - INTERVAL '3 days', TRUE),
			($1::uuid, 'manual', 'Not yet due',       'major_nc',
			 CURRENT_DATE + INTERVAL '7 days', NULL)
	`, orgID)
	require.NoError(t, err)

	repo := vaktcomply.NewRepository(pool)
	items, err := repo.ListOverdueEffectivenessChecks(ctx)
	require.NoError(t, err, "query must match the ck_capas schema (regression: created_by)")

	require.Len(t, items, 1, "only the overdue, unconfirmed major_nc CAPA should be returned")
	require.Equal(t, orgID, items[0].OrgID)
	require.Equal(t, overdueID, items[0].CAPAID)
}
