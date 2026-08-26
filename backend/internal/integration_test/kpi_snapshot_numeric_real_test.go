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

	"github.com/matharnica/vakt/internal/modules/vaktcomply/reporting"
)

// TestUpsertKPISnapshot_PersistsFloatKPIs (R1-20-A2) proves that the seven
// float-valued KPI columns actually reach the database as numbers.
//
// They did not. The write path built its numeric argument via
// pgtype.Numeric.Scan(float64) — an operation pgx v5 does NOT support: Scan
// accepts only nil/string and returns "cannot scan float64". That error was
// discarded and an empty (invalid → NULL) Numeric written instead, so every one
// of the seven float KPIs — compliance score, residual risk, MTTR, evidence
// coverage, finding-SLA compliance, suppliers-overdue %, phishing click rate —
// was silently persisted as NULL. The KPI PDF then printed "n/a" for a score the
// same SQL computes live as 0.00 or 2.34. This also corrects R1-27-V03: the
// finding-SLA metric is not constantly 100 %, it is constantly NULL, and the
// cause is here in the shared write helper, not in the SLA calculation.
//
// The assertion is deliberately made at the raw column level (NOT via a
// round-trip through GetLatestKPISnapshot, which reads NULL back as a benign nil
// *float64 and would hide a persisted NULL). A separate round-trip check confirms
// the values also decode back correctly.
//
// Red-on-revert: restore the old Scan(*v) helper and all seven columns come back
// NULL — every column assertion below fails.
func TestUpsertKPISnapshot_PersistsFloatKPIs(t *testing.T) {
	if testing.Short() {
		t.Skip("integration: -short mode")
	}
	pool, orgID, cleanup := bootPostgresWithOrg(t)
	defer cleanup()
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	defer cancel()

	repo := reporting.NewRepository(pool)

	// Distinct, non-round values so a mix-up between columns would show, plus one
	// deliberate 0.0 (compliance score): a valid measurement of zero must persist
	// as 0.0, never as NULL — that distinction is the whole point of A2.
	f := func(v float64) *float64 { return &v }
	i := func(v int) *int { return &v }
	snap := reporting.KPISnapshot{
		SnapshotDate:          "2026-07-28",
		ComplianceScore:       f(0.0),   // valid zero, must NOT become NULL
		OpenCriticalControls:  i(3),     // int columns were never broken
		OpenHighRisks:         i(7),     //
		ResidualRiskAvg:       f(2.34),  // the live value the PDF failed to print
		OpenIncidents:         i(2),     //
		IncidentMTTRDays:      f(11.5),  //
		EvidenceCoverage:      f(87.25), //
		ExpiringEvidenceCount: i(4),     //
		FindingSLACompliance:  f(93.75), // R1-27-V03: constant NULL, not constant 100
		OpenMajorNCs:          i(1),     //
		SuppliersOverduePct:   f(12.5),  //
		PhishingClickRate:     f(50.0),  //
	}

	require.NoError(t, repo.UpsertKPISnapshot(ctx, orgID, snap))

	// Read the raw columns straight from the table. Each `expected` is the exact
	// value we wrote; a NULL (the old bug) makes the pointer nil and fails the
	// require.NotNil before the value is ever compared.
	type col struct {
		name     string
		expected float64
	}
	cols := []col{
		{"kpi_compliance_score", 0.0},
		{"kpi_residual_risk_avg", 2.34},
		{"kpi_incident_mttr_days", 11.5},
		{"kpi_evidence_coverage", 87.25},
		{"kpi_finding_sla_compliance", 93.75},
		{"kpi_suppliers_overdue_pct", 12.5},
		{"kpi_phishing_click_rate", 50.0},
	}
	require.Len(t, cols, 7, "all seven float KPIs are covered")

	for _, c := range cols {
		var got *float64
		require.NoError(t, pool.QueryRow(ctx,
			`SELECT `+c.name+`::float8 FROM ck_isms_kpi_snapshots WHERE org_id = $1::uuid`,
			orgID).Scan(&got),
			"select %s", c.name)
		require.NotNilf(t, got, "%s persisted as NULL — the float KPI never reached the DB (R1-20-A2)", c.name)
		assert.InDeltaf(t, c.expected, *got, 0.001, "%s round-trips to its written value", c.name)
	}

	// The public read path must also see the values (not just the raw columns).
	got, err := repo.GetLatestKPISnapshot(ctx, orgID)
	require.NoError(t, err)
	require.NotNil(t, got)
	require.NotNil(t, got.ComplianceScore)
	assert.InDelta(t, 0.0, *got.ComplianceScore, 0.001)
	require.NotNil(t, got.FindingSLACompliance)
	assert.InDelta(t, 93.75, *got.FindingSLACompliance, 0.001)
	require.NotNil(t, got.PhishingClickRate)
	assert.InDelta(t, 50.0, *got.PhishingClickRate, 0.001)
}
