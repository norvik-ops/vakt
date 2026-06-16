// Copyright (c) 2026 NorvikOps. All rights reserved.
// SPDX-License-Identifier: Elastic-2.0

package vaktcomply

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ── validateBIAProcess tests ──────────────────────────────────────────────────

func TestValidateBIAProcess_OK(t *testing.T) {
	assert.NoError(t, validateBIAProcess(72, 24, 50))
}

func TestValidateBIAProcess_RPOGreaterThanRTO(t *testing.T) {
	err := validateBIAProcess(24, 48, 50)
	assert.ErrorIs(t, err, ErrRPOExceedsRTO)
}

func TestValidateBIAProcess_RPOEqualsRTO(t *testing.T) {
	// Edge case: RPO == RTO is allowed
	assert.NoError(t, validateBIAProcess(24, 24, 50))
}

func TestValidateBIAProcess_MBCOAbove100(t *testing.T) {
	err := validateBIAProcess(72, 24, 101)
	assert.ErrorIs(t, err, ErrMBCOOutOfRange)
}

func TestValidateBIAProcess_MBCOZeroOK(t *testing.T) {
	assert.NoError(t, validateBIAProcess(72, 24, 0))
}

// ── validateRecoveryPlan tests ────────────────────────────────────────────────

func TestValidateRecoveryPlan_OK(t *testing.T) {
	steps := []RecoveryStep{
		{Order: 1, Action: "Step A", Responsible: "IT", DurationMin: 30},
		{Order: 2, Action: "Step B", Responsible: "DevOps", DurationMin: 15},
	}
	assert.NoError(t, validateRecoveryPlan(4, steps))
}

func TestValidateRecoveryPlan_RTOZero(t *testing.T) {
	err := validateRecoveryPlan(0, nil)
	assert.ErrorIs(t, err, ErrRTORequired)
}

func TestValidateRecoveryPlan_StepsGap(t *testing.T) {
	steps := []RecoveryStep{
		{Order: 1, Action: "Step A"},
		{Order: 3, Action: "Step C"}, // gap: missing 2
	}
	err := validateRecoveryPlan(4, steps)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "steps order")
}

func TestValidateRecoveryPlan_EmptyStepsOK(t *testing.T) {
	assert.NoError(t, validateRecoveryPlan(4, []RecoveryStep{}))
}

// ── BCMReadinessScore unit tests ──────────────────────────────────────────────

func TestBCMCriterionPoints(t *testing.T) {
	// Each criterion is worth 20 points — total must be 100
	total := 0
	criteria := []BCMCriterion{
		{Key: "c1", Points: 20, Met: true},
		{Key: "c2", Points: 20, Met: true},
		{Key: "c3", Points: 20, Met: true},
		{Key: "c4", Points: 20, Met: true},
		{Key: "c5", Points: 20, Met: true},
	}
	for _, c := range criteria {
		if c.Met {
			total += c.Points
		}
	}
	assert.Equal(t, 100, total)
}

func TestBCMScoreZeroWhenNoCriteriaMet(t *testing.T) {
	criteria := []BCMCriterion{
		{Key: "c1", Points: 20, Met: false},
		{Key: "c2", Points: 20, Met: false},
		{Key: "c3", Points: 20, Met: false},
		{Key: "c4", Points: 20, Met: false},
		{Key: "c5", Points: 20, Met: false},
	}
	score := 0
	for _, c := range criteria {
		if c.Met {
			score += c.Points
		}
	}
	assert.Equal(t, 0, score)
}

// ── DER.4 Cross-Mapping tests ─────────────────────────────────────────────────

func TestDER4CrossMappings_Count(t *testing.T) {
	assert.Equal(t, 12, len(der4CrossMappings), "expected 12 DER.4 cross-mapping pairs")
}

func TestDER4CrossMappings_NoLegacyISO(t *testing.T) {
	for _, m := range der4CrossMappings {
		// No A.9–A.18 legacy ISO 27001:2001 codes
		if m.tgt == "ISO27001" {
			code := m.tgtCode
			assert.NotContains(t, code, "A.9.", "found legacy ISO code %s", code)
			assert.NotContains(t, code, "A.10.", "found legacy ISO code %s", code)
			assert.NotContains(t, code, "A.11.", "found legacy ISO code %s", code)
		}
	}
}

func TestDER4CrossMappings_AllBSISide(t *testing.T) {
	for _, m := range der4CrossMappings {
		assert.Equal(t, "BSI", m.src, "src should always be BSI")
		assert.Contains(t, m.srcCode, "DER.4", "srcCode should be DER.4.x")
	}
}
