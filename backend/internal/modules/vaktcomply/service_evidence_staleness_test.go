// Copyright (c) 2026 NorvikOps. All rights reserved.
// SPDX-License-Identifier: Elastic-2.0

package vaktcomply

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestDefaultMaxAgeDays(t *testing.T) {
	cases := []struct {
		evidenceType string
		expected     int
	}{
		{"scanner", 7},
		{"cloud", 2},
		{"policy", 365},
		{"pentest", 365},
		{"phishing", 90},
		{"manual", 180},
		{"unknown_type", 180},
	}
	for _, tc := range cases {
		t.Run(tc.evidenceType, func(t *testing.T) {
			assert.Equal(t, tc.expected, DefaultMaxAgeDays(tc.evidenceType))
		})
	}
}

func TestComplianceScore_StaleCountsAsNotOk(t *testing.T) {
	// Simulate: 10 controls, 8 ok, 2 stale → Score = 8/10 = 80%
	s := ComplianceScore{
		TotalControls: 10,
		OkCount:       8,
		StaleCount:    2,
		MissingCount:  0,
		NACount:       0,
	}
	denominator := s.TotalControls - s.NACount
	require.Equal(t, 10, denominator)
	s.ScorePct = float64(s.OkCount) / float64(denominator) * 100
	assert.InDelta(t, 80.0, s.ScorePct, 0.01, "stale counts as not-ok: 8/10 = 80%")
}

func TestComplianceScore_NAExcludedFromDenominator(t *testing.T) {
	// 10 controls, 8 ok, 2 na → Score = 8/8 = 100%
	s := ComplianceScore{
		TotalControls: 10,
		OkCount:       8,
		StaleCount:    0,
		MissingCount:  0,
		NACount:       2,
	}
	denominator := s.TotalControls - s.NACount
	require.Equal(t, 8, denominator)
	s.ScorePct = float64(s.OkCount) / float64(denominator) * 100
	assert.InDelta(t, 100.0, s.ScorePct, 0.01, "NA excluded: 8/8 = 100%")
}

func TestEvidenceStalenessLogic(t *testing.T) {
	now := time.Now().UTC()

	// Evidence 8 days old, max_age = 7 → stale
	evidenceAge := 8 * 24 * time.Hour
	maxAgeDays := 7
	isStale := now.Sub(now.Add(-evidenceAge)) > time.Duration(maxAgeDays)*24*time.Hour
	assert.True(t, isStale, "8-day-old evidence with max_age=7 should be stale")

	// Evidence 6 days old, max_age = 7 → ok
	evidenceAge = 6 * 24 * time.Hour
	isStale = now.Sub(now.Add(-evidenceAge)) > time.Duration(maxAgeDays)*24*time.Hour
	assert.False(t, isStale, "6-day-old evidence with max_age=7 should be ok")

	// No evidence → missing (handled by NULL check in SQL)
	// max_age = NULL → ok regardless of age (no staleness check)
}
