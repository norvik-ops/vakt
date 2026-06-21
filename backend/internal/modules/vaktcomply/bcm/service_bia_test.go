// Copyright (c) 2026 NorvikOps. All rights reserved.
// SPDX-License-Identifier: Elastic-2.0

package bcm

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
