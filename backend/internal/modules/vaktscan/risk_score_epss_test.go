// Copyright (c) 2026 NorvikOps. All rights reserved.
// SPDX-License-Identifier: Elastic-2.0

package vaktscan

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

// TestComputeRiskScoreReflectsEPSS pins the property FIX 1(b) (R1-36b-SC02)
// relies on: after an EPSS refresh, recomputing risk_score with ComputeRiskScore
// moves the score in step with the fresh percentile. The DB wiring in
// UpdateEPSSScores (RETURNING → ComputeRiskScore → UPDATE risk_score) is
// DB-coupled and marked Live-REVERIFY; this test locks the pure calculation the
// wiring depends on.
//
// It is non-tautological: the expected values are derived by hand from the
// documented formula (cvss * (1 + epss_percentile) * criticality_multiplier),
// not read back from ComputeRiskScore.
func TestComputeRiskScoreReflectsEPSS(t *testing.T) {
	cvss := 8.0

	// Before enrichment: no EPSS percentile → multiplier 1.0.
	before := Finding{Severity: "high", CVSSScore: &cvss}
	ComputeRiskScore(&before)
	assert.NotNil(t, before.RiskScore)
	assert.InDelta(t, 8.0*1.0*1.5, *before.RiskScore, 1e-9) // 12.0

	// After enrichment: percentile 0.9 → multiplier 1.9. Same finding, fresh EPSS.
	pct := 0.9
	after := Finding{Severity: "high", CVSSScore: &cvss, EPSSPercentile: &pct}
	ComputeRiskScore(&after)
	assert.NotNil(t, after.RiskScore)
	assert.InDelta(t, 8.0*1.9*1.5, *after.RiskScore, 1e-9) // 22.8

	// The whole point of the recompute: a stale risk_score would still read 12.0.
	assert.Greater(t, *after.RiskScore, *before.RiskScore,
		"risk_score must rise with the EPSS percentile, otherwise the recompute is pointless")
}

// TestComputeRiskScoreNilCVSSDefault guards the documented default (nil CVSS →
// 5.0) that the recompute inherits, so a NULL cvss_score row still gets a
// non-nil risk_score after EPSS enrichment.
func TestComputeRiskScoreNilCVSSDefault(t *testing.T) {
	pct := 0.5
	f := Finding{Severity: "medium", EPSSPercentile: &pct} // CVSSScore nil
	ComputeRiskScore(&f)
	assert.NotNil(t, f.RiskScore)
	assert.InDelta(t, 5.0*1.5*1.0, *f.RiskScore, 1e-9) // 7.5
}
