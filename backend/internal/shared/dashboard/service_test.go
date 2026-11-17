// Copyright (c) 2026 NorvikOps. All rights reserved.
// SPDX-License-Identifier: Elastic-2.0

package dashboard

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
)

// ---------------------------------------------------------------------------
// defaultScoreConfig
// ---------------------------------------------------------------------------

func TestDefaultScoreConfig_Values(t *testing.T) {
	cfg := defaultScoreConfig()
	assert.Equal(t, 70, cfg.BaseScore)
	assert.Equal(t, 5, cfg.CritPenalty)
	assert.Equal(t, 30, cfg.CritPenaltyCap)
	assert.Equal(t, 2, cfg.HighPenalty)
	assert.Equal(t, 10, cfg.HighPenaltyCap)
	assert.Equal(t, 20, cfg.BreachPenalty)
	assert.Equal(t, 20, cfg.BreachPenaltyCap)
	assert.Equal(t, 10, cfg.FwBonus)
	assert.Equal(t, 30, cfg.FwBonusCap)
}

// ---------------------------------------------------------------------------
// ScoreConfig validation (mirrors UpdateScoreConfig handler logic)
// ---------------------------------------------------------------------------

func isValidScoreConfig(cfg ScoreConfig) bool {
	fields := []int{
		cfg.BaseScore, cfg.CritPenalty, cfg.CritPenaltyCap,
		cfg.HighPenalty, cfg.HighPenaltyCap,
		cfg.BreachPenalty, cfg.BreachPenaltyCap,
		cfg.FwBonus, cfg.FwBonusCap,
	}
	for _, v := range fields {
		if v < 1 || v > 100 {
			return false
		}
	}
	return true
}

func TestScoreConfigValidation_DefaultIsValid(t *testing.T) {
	assert.True(t, isValidScoreConfig(defaultScoreConfig()))
}

func TestScoreConfigValidation_ZeroValueFails(t *testing.T) {
	assert.False(t, isValidScoreConfig(ScoreConfig{}))
}

func TestScoreConfigValidation_OverHundredFails(t *testing.T) {
	cfg := defaultScoreConfig()
	cfg.BaseScore = 101
	assert.False(t, isValidScoreConfig(cfg))
}

func TestScoreConfigValidation_ExactBoundaries(t *testing.T) {
	low := ScoreConfig{
		BaseScore: 1, CritPenalty: 1, CritPenaltyCap: 1,
		HighPenalty: 1, HighPenaltyCap: 1,
		BreachPenalty: 1, BreachPenaltyCap: 1,
		FwBonus: 1, FwBonusCap: 1,
	}
	assert.True(t, isValidScoreConfig(low), "all-ones config should be valid (lower boundary)")

	high := ScoreConfig{
		BaseScore: 100, CritPenalty: 100, CritPenaltyCap: 100,
		HighPenalty: 100, HighPenaltyCap: 100,
		BreachPenalty: 100, BreachPenaltyCap: 100,
		FwBonus: 100, FwBonusCap: 100,
	}
	assert.True(t, isValidScoreConfig(high), "all-100 config should be valid (upper boundary)")
}

// ---------------------------------------------------------------------------
// aggregateCacheKey
// ---------------------------------------------------------------------------

func TestAggregateCacheKey_Format(t *testing.T) {
	orgID := "test-org-uuid"
	// v2: payload gained overall_readiness — an old cached payload must not be served.
	assert.Equal(t, fmt.Sprintf("dashboard:aggregate:v2:%s", orgID), aggregateCacheKey(orgID))
}

func TestAggregateCacheKey_DifferentOrgs(t *testing.T) {
	assert.NotEqual(t, aggregateCacheKey("org-a"), aggregateCacheKey("org-b"))
}

func TestAggregateCacheKey_EmptyOrg(t *testing.T) {
	assert.Equal(t, "dashboard:aggregate:v2:", aggregateCacheKey(""))
}
