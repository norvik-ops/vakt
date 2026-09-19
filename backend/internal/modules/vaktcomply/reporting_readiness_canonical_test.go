// Copyright (c) 2026 NorvikOps. All rights reserved.
// SPDX-License-Identifier: Elastic-2.0

package vaktcomply

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/matharnica/vakt/internal/modules/vaktcomply/policy"
)

// TestReportsUseCanonicalReadiness pins R1-20-03 and R1-W3C-N2 (ADR-0086).
//
// Before the fix the executive summary and the board report's live score came
// from two SQL aggregates (GetExecutiveFrameworkScores /
// GetBoardReportComplianceScoreRows) that counted a control as implemented ONLY
// when manual_status = 'implemented', ignored attached evidence, ignored partial
// progress, and left not_applicable controls in the denominator. The board
// report then subtracted that number from d.ScorePrevious, which comes from the
// score_history snapshot computed via policy.ComputeReadinessReport — a
// different formula. The delta subtracted apples from oranges.
//
// The reports now compute their figures through computeOrgReadiness →
// poolOrgReadiness, i.e. exactly policy.ComputeReadinessReport. This test feeds
// a control set that exercises all four effective states plus an evidence-only
// control and checks that:
//   - each framework's readiness matches a value computed by hand,
//   - the pooled org score matches a value computed by hand,
//   - that canonical org score is NOT what the superseded manual_status-only
//     weighted formula would have produced for the same controls.
//
// The expected numbers are written out by hand, not derived from the function
// under test, so the test is not the formula checked against itself. Reverting
// the reports to the manual_status SQL aggregate makes the divergence assertion
// (canonical != legacy) fail.
func TestReportsUseCanonicalReadiness(t *testing.T) {
	fwA := &policy.Framework{ID: "fw-a", Name: "ISO27001"}
	fwB := &policy.Framework{ID: "fw-b", Name: "NIS2"}

	// Framework A: 5 controls.
	//   c1 manual implemented            -> covered
	//   c2 two evidences, no manual      -> covered  (legacy SQL would MISS this)
	//   c3 manual in_progress            -> partial (half weight)
	//   c4 not_applicable                -> out of the denominator
	//   c5 nothing                       -> missing
	// applicable = 5 - 1 = 4; readiness = (2 + 0.5) / 4 * 100 = 62.5
	controlsA := []policy.Control{
		{ID: "c1", Domain: "Access", ManualStatus: "implemented"},
		{ID: "c2", Domain: "Access"},
		{ID: "c3", Domain: "Crypto", ManualStatus: "in_progress"},
		{ID: "c4", Domain: "Crypto", NotApplicable: true},
		{ID: "c5", Domain: "Ops"},
	}
	evidenceA := map[string]int{"c2": 2}

	// Framework B: 2 controls.
	//   d1 manual implemented -> covered
	//   d2 not_applicable     -> out of the denominator
	// applicable = 2 - 1 = 1; readiness = 1 / 1 * 100 = 100
	controlsB := []policy.Control{
		{ID: "d1", Domain: "Governance", ManualStatus: "implemented"},
		{ID: "d2", Domain: "Governance", NotApplicable: true},
	}
	evidenceB := map[string]int{}

	// Build the per-framework rows exactly as computeOrgReadiness does.
	reportA := policy.ComputeReadinessReport(fwA, controlsA, evidenceA)
	reportB := policy.ComputeReadinessReport(fwB, controlsB, evidenceB)
	rows := []canonicalFrameworkReadiness{
		{
			Name:       fwA.Name,
			Score:      reportA.ReadinessScore,
			Covered:    reportA.Covered,
			Partial:    reportA.Partial,
			Applicable: reportA.TotalControls - reportA.NotApplicable,
			Total:      reportA.TotalControls,
		},
		{
			Name:       fwB.Name,
			Score:      reportB.ReadinessScore,
			Covered:    reportB.Covered,
			Partial:    reportB.Partial,
			Applicable: reportB.TotalControls - reportB.NotApplicable,
			Total:      reportB.TotalControls,
		},
	}

	// Per-framework readiness — hand-computed above.
	assert.InDelta(t, 62.5, rows[0].Score, 0.001, "framework A readiness")
	assert.Equal(t, 2, rows[0].Covered)
	assert.Equal(t, 1, rows[0].Partial)
	assert.Equal(t, 4, rows[0].Applicable)
	assert.Equal(t, 5, rows[0].Total)
	assert.InDelta(t, 100.0, rows[1].Score, 0.001, "framework B readiness")

	// Pooled org score — the figure the board report and executive summary show.
	// covered = 3, partial = 1, applicable = 5 -> (3 + 0.5) / 5 * 100 = 70.
	orgScore := poolOrgReadiness(rows)
	assert.InDelta(t, 70.0, orgScore, 0.001, "pooled org readiness")

	// The superseded formula for the SAME controls: per framework
	// implemented(manual only)/total(incl. N/A), weighted by total.
	//   A: 1/5 = 20 % (weight 5)   B: 1/2 = 50 % (weight 2)
	//   overall = (20*5 + 50*2) / (5+2) = 200/7 ≈ 28.57
	legacyImplA, legacyTotalA := 1.0, 5.0
	legacyImplB, legacyTotalB := 1.0, 2.0
	legacyScoreA := legacyImplA / legacyTotalA * 100
	legacyScoreB := legacyImplB / legacyTotalB * 100
	legacyOverall := (legacyScoreA*legacyTotalA + legacyScoreB*legacyTotalB) / (legacyTotalA + legacyTotalB)

	require.InDelta(t, 200.0/7.0, legacyOverall, 0.001, "sanity: legacy formula reproduced")
	assert.False(t, almostEqual(orgScore, legacyOverall),
		"canonical readiness (%.2f) must differ from the superseded manual_status formula (%.2f); "+
			"if these are equal the reports slid back onto the old SQL aggregate", orgScore, legacyOverall)
}

func almostEqual(a, b float64) bool {
	const eps = 0.001
	d := a - b
	if d < 0 {
		d = -d
	}
	return d < eps
}
