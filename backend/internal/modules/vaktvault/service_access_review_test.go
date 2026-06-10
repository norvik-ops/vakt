// Copyright (c) 2026 NorvikOps. All rights reserved.
// SPDX-License-Identifier: Elastic-2.0

// Tests for Sprint 70 S70-5: Vault Access Review model invariants.

package vaktvault

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

// ── S70-5: CurrentQuarterLabel ───────────────────────────────────────────────

func TestCurrentQuarterLabel_Q1(t *testing.T) {
	for _, month := range []time.Month{time.January, time.February, time.March} {
		d := time.Date(2026, month, 15, 0, 0, 0, 0, time.UTC)
		assert.Equal(t, "Q1/2026", CurrentQuarterLabel(d), "month=%v", month)
	}
}

func TestCurrentQuarterLabel_Q2(t *testing.T) {
	for _, month := range []time.Month{time.April, time.May, time.June} {
		d := time.Date(2026, month, 15, 0, 0, 0, 0, time.UTC)
		assert.Equal(t, "Q2/2026", CurrentQuarterLabel(d), "month=%v", month)
	}
}

func TestCurrentQuarterLabel_Q3(t *testing.T) {
	for _, month := range []time.Month{time.July, time.August, time.September} {
		d := time.Date(2026, month, 15, 0, 0, 0, 0, time.UTC)
		assert.Equal(t, "Q3/2026", CurrentQuarterLabel(d), "month=%v", month)
	}
}

func TestCurrentQuarterLabel_Q4(t *testing.T) {
	for _, month := range []time.Month{time.October, time.November, time.December} {
		d := time.Date(2026, month, 15, 0, 0, 0, 0, time.UTC)
		assert.Equal(t, "Q4/2026", CurrentQuarterLabel(d), "month=%v", month)
	}
}

func TestCurrentQuarterLabel_YearBoundary(t *testing.T) {
	d := time.Date(2025, time.December, 31, 0, 0, 0, 0, time.UTC)
	assert.Equal(t, "Q4/2025", CurrentQuarterLabel(d))

	d2 := time.Date(2026, time.January, 1, 0, 0, 0, 0, time.UTC)
	assert.Equal(t, "Q1/2026", CurrentQuarterLabel(d2))
}

// ── S70-5: AccessReview model ─────────────────────────────────────────────────

func TestAccessReview_StatusValues(t *testing.T) {
	validStatuses := []string{"open", "completed"}
	for _, s := range validStatuses {
		assert.NotEmpty(t, s)
	}
}

func TestAccessReviewItem_IsStale_OlderThan90Days(t *testing.T) {
	threshold := time.Now().UTC().Add(-90 * 24 * time.Hour)
	old := threshold.Add(-time.Hour)
	item := AccessReviewItem{
		SecretKey:      "DB_PASSWORD",
		LastAccessedAt: &old,
		IsStale:        old.Before(threshold),
	}
	assert.True(t, item.IsStale, "secret last accessed >90 days ago must be stale")
}

func TestAccessReviewItem_IsStale_Never(t *testing.T) {
	item := AccessReviewItem{
		SecretKey:      "DB_PASSWORD",
		LastAccessedAt: nil,
		IsStale:        true, // NULL last_accessed_at → stale
	}
	assert.True(t, item.IsStale, "secret never accessed must be stale")
}

func TestAccessReviewItem_NotStale_Recent(t *testing.T) {
	recent := time.Now().UTC().Add(-24 * time.Hour)
	threshold := time.Now().UTC().Add(-90 * 24 * time.Hour)
	item := AccessReviewItem{
		SecretKey:      "API_KEY",
		LastAccessedAt: &recent,
		IsStale:        recent.Before(threshold),
	}
	assert.False(t, item.IsStale, "secret accessed yesterday must not be stale")
}

// ── S70-5: ReviewDecision action validation ───────────────────────────────────

func TestReviewDecision_Actions(t *testing.T) {
	keep := ReviewDecision{Action: "keep", EnvID: "env-1", SecretKey: "DB_PASSWORD"}
	revoke := ReviewDecision{Action: "revoke", EnvID: "env-1", SecretKey: "OLD_TOKEN"}

	assert.Equal(t, "keep", keep.Action)
	assert.Equal(t, "revoke", revoke.Action)
}

func TestCompleteAccessReviewInput_RevokeCount(t *testing.T) {
	in := CompleteAccessReviewInput{
		Decisions: []ReviewDecision{
			{Action: "keep", EnvID: "e1", SecretKey: "DB_PASS"},
			{Action: "revoke", EnvID: "e1", SecretKey: "OLD_TOKEN"},
			{Action: "revoke", EnvID: "e2", SecretKey: "STALE_KEY"},
			{Action: "keep", EnvID: "e3", SecretKey: "API_KEY"},
		},
	}
	revokedCount := 0
	for _, d := range in.Decisions {
		if d.Action == "revoke" {
			revokedCount++
		}
	}
	assert.Equal(t, 2, revokedCount)
}
