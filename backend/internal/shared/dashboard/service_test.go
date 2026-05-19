// Copyright (c) 2026 NorvikOps. All rights reserved.
// SPDX-License-Identifier: Elastic-2.0

package dashboard

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
)

// calculateScore is a test helper that wraps ComputeScore with individual int
// arguments for readability in table-driven tests.
func calculateScore(cfg ScoreConfig, critCount, highCount, breachCount, fwCount int) int {
	score, _ := ComputeScore(ScoreInputs{
		Cfg:         cfg,
		CritCount:   int64(critCount),
		HighCount:   int64(highCount),
		BreachCount: int64(breachCount),
		FwCount:     int64(fwCount),
	})
	return score
}

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
// ComputeScore — happy paths
// ---------------------------------------------------------------------------

func TestCalculateScore_NoFindingsNoFrameworks(t *testing.T) {
	score := calculateScore(defaultScoreConfig(), 0, 0, 0, 0)
	assert.Equal(t, 70, score) // base=70, no penalties, no bonus
}

func TestCalculateScore_WithFrameworkBonus(t *testing.T) {
	// 3 frameworks: bonus = 3*10 = 30, capped at 30 → 70+30 = 100
	score := calculateScore(defaultScoreConfig(), 0, 0, 0, 3)
	assert.Equal(t, 100, score)
}

func TestCalculateScore_BonusCapEnforced(t *testing.T) {
	// 10 frameworks: bonus = 10*10 = 100, capped at 30 → 70+30 = 100
	score := calculateScore(defaultScoreConfig(), 0, 0, 0, 10)
	assert.Equal(t, 100, score)
}

func TestCalculateScore_CriticalPenaltyCapped(t *testing.T) {
	// 100 criticals: penalty = min(100*5, 30) = 30 → 70-30 = 40
	score := calculateScore(defaultScoreConfig(), 100, 0, 0, 0)
	assert.Equal(t, 40, score)
}

func TestCalculateScore_HighPenaltyCapped(t *testing.T) {
	// 100 highs: penalty = min(100*2, 10) = 10 → 70-10 = 60
	score := calculateScore(defaultScoreConfig(), 0, 100, 0, 0)
	assert.Equal(t, 60, score)
}

func TestCalculateScore_BreachPenaltyCapped(t *testing.T) {
	// 10 breaches: penalty = min(10*20, 20) = 20 → 70-20 = 50
	score := calculateScore(defaultScoreConfig(), 0, 0, 10, 0)
	assert.Equal(t, 50, score)
}

func TestCalculateScore_ClampedToZero(t *testing.T) {
	cfg := defaultScoreConfig()
	cfg.BaseScore = 1
	cfg.CritPenaltyCap = 50
	cfg.HighPenaltyCap = 50
	cfg.BreachPenaltyCap = 50
	score := calculateScore(cfg, 100, 100, 100, 0)
	assert.Equal(t, 0, score, "score should be clamped to 0, never negative")
}

func TestCalculateScore_ClampedToHundred(t *testing.T) {
	cfg := defaultScoreConfig()
	cfg.BaseScore = 100
	cfg.FwBonus = 50
	cfg.FwBonusCap = 100
	score := calculateScore(cfg, 0, 0, 0, 10)
	assert.Equal(t, 100, score, "score should be clamped to 100")
}

// ---------------------------------------------------------------------------
// ComputeScore — components map is populated correctly
// ---------------------------------------------------------------------------

func TestComputeScore_ComponentsMap(t *testing.T) {
	_, components := ComputeScore(ScoreInputs{
		Cfg:         defaultScoreConfig(),
		CritCount:   3,
		HighCount:   5,
		BreachCount: 1,
		FwCount:     2,
	})
	assert.Equal(t, int64(3), components["critical_findings"])
	assert.Equal(t, int64(5), components["high_findings"])
	assert.Equal(t, int64(1), components["open_breaches"])
	assert.Equal(t, int64(2), components["active_frameworks"])
}

// ---------------------------------------------------------------------------
// ComputeScore — table-driven tests
// ---------------------------------------------------------------------------

func TestCalculateScore_Table(t *testing.T) {
	cfg := defaultScoreConfig()

	tests := []struct {
		name      string
		crit      int
		high      int
		breach    int
		fw        int
		wantScore int
	}{
		{"clean slate", 0, 0, 0, 0, 70},
		{"single critical", 1, 0, 0, 0, 65},  // 70-5
		{"single high", 0, 1, 0, 0, 68},       // 70-2
		{"one open breach", 0, 0, 1, 0, 50},   // 70-20
		{"one framework", 0, 0, 0, 1, 80},     // 70+10
		{
			// crit_pen=min(15,30)=15; high_pen=min(10,10)=10; breach_pen=min(20,20)=20; fw_bonus=min(20,30)=20
			// 70-15-10-20+20 = 45
			name: "realistic mixed", crit: 3, high: 5, breach: 1, fw: 2, wantScore: 45,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.wantScore, calculateScore(cfg, tt.crit, tt.high, tt.breach, tt.fw))
		})
	}
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
	assert.Equal(t, fmt.Sprintf("dashboard:aggregate:%s", orgID), aggregateCacheKey(orgID))
}

func TestAggregateCacheKey_DifferentOrgs(t *testing.T) {
	assert.NotEqual(t, aggregateCacheKey("org-a"), aggregateCacheKey("org-b"))
}

func TestAggregateCacheKey_EmptyOrg(t *testing.T) {
	assert.Equal(t, "dashboard:aggregate:", aggregateCacheKey(""))
}
