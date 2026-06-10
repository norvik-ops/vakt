// Copyright (c) 2026 NorvikOps. All rights reserved.
// SPDX-License-Identifier: Elastic-2.0

package vaktcomply

import (
	"context"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgtype"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestKPISnapshotStructFields verifies the KPISnapshot struct can be instantiated
// with all fields — this is a compile-time completeness check.
func TestKPISnapshotStructFields(t *testing.T) {
	score := 95.5
	count := 3
	snap := KPISnapshot{
		ID:                    "id-1",
		OrgID:                 "org-1",
		SnapshotDate:          "2026-06-09",
		ComplianceScore:       &score,
		OpenCriticalControls:  &count,
		OpenHighRisks:         &count,
		ResidualRiskAvg:       &score,
		OpenIncidents:         &count,
		IncidentMTTRDays:      &score,
		EvidenceCoverage:      &score,
		ExpiringEvidenceCount: &count,
		FindingSLACompliance:  nil,
		OpenMajorNCs:          nil,
		SuppliersOverduePct:   nil,
		PhishingClickRate:     nil,
		CreatedAt:             time.Now(),
	}
	assert.Equal(t, "org-1", snap.OrgID)
	assert.Equal(t, 95.5, *snap.ComplianceScore)
	assert.Nil(t, snap.FindingSLACompliance)
}

// TestKPIDashboardStructFields verifies KPIDashboard bundles correctly.
func TestKPIDashboardStructFields(t *testing.T) {
	dash := KPIDashboard{
		Current: nil,
		History: []KPISnapshot{},
	}
	assert.Nil(t, dash.Current)
	assert.Empty(t, dash.History)
}

// TestNumericToFloat64PtrNilOnInvalid verifies the helper returns nil for an
// invalid (NULL) pgtype.Numeric — avoids panics when the DB returns NULL.
func TestNumericToFloat64PtrNilOnInvalid(t *testing.T) {
	var n pgtype.Numeric // zero value → Valid = false
	result := numericToFloat64Ptr(n)
	require.Nil(t, result, "expected nil for invalid Numeric")
}

// TestNumericToFloat64PtrValue verifies the helper correctly converts a valid
// pgtype.Numeric to a *float64.
func TestNumericToFloat64PtrValue(t *testing.T) {
	var n pgtype.Numeric
	// pgtype.Numeric.Scan accepts a string representation of the number.
	require.NoError(t, n.Scan("42.5"))
	result := numericToFloat64Ptr(n)
	require.NotNil(t, result)
	assert.InDelta(t, 42.5, *result, 0.001)
}

// TestCalculateKPIsForOrgNilDB verifies that CalculateKPIsForOrg does not panic
// when passed a nil DB pool — all sub-calculators must guard against nil.
func TestCalculateKPIsForOrgNilDB(t *testing.T) {
	ctx := context.Background()
	snap := CalculateKPIsForOrg(ctx, nil, "org-nil")

	today := time.Now().Format("2006-01-02")
	assert.Equal(t, today, snap.SnapshotDate, "snapshot_date should always be today")

	// All computed KPIs must be nil when db is nil.
	assert.Nil(t, snap.ComplianceScore)
	assert.Nil(t, snap.OpenCriticalControls)
	assert.Nil(t, snap.OpenHighRisks)
	assert.Nil(t, snap.ResidualRiskAvg)
	assert.Nil(t, snap.OpenIncidents)
	assert.Nil(t, snap.IncidentMTTRDays)
	assert.Nil(t, snap.EvidenceCoverage)
	assert.Nil(t, snap.ExpiringEvidenceCount)
	// Statically deferred KPIs are always nil regardless.
	assert.Nil(t, snap.FindingSLACompliance)
	assert.Nil(t, snap.OpenMajorNCs)
	assert.Nil(t, snap.SuppliersOverduePct)
	assert.Nil(t, snap.PhishingClickRate)
}
