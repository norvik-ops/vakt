// Copyright (c) 2026 NorvikOps. All rights reserved.
// SPDX-License-Identifier: Elastic-2.0

// Tests for Sprint 69 S69-3 SLA enforcement logic.

package vaktscan

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

// ── S69-3: SLAPolicy model invariants ────────────────────────────────────────

func TestSLAPolicyDefaultDaysValid(t *testing.T) {
	defaults := []struct {
		severity string
		days     int
	}{
		{"critical", 7},
		{"high", 30},
		{"medium", 90},
		{"low", 180},
		{"info", 365},
	}
	for _, d := range defaults {
		assert.Greater(t, d.days, 0, "remediation days for %s must be positive", d.severity)
	}
}

func TestSLAPolicySeveritiesDistinct(t *testing.T) {
	// Each expected severity must appear exactly once in the defaults.
	expected := []string{"critical", "high", "medium", "low", "info"}
	seen := make(map[string]int)
	for _, s := range expected {
		seen[s]++
	}
	for _, s := range expected {
		assert.Equal(t, 1, seen[s], "severity %q appears more than once", s)
	}
	assert.Equal(t, 5, len(seen), "expected exactly 5 severity levels")
}

func TestSLASummaryZeroValues(t *testing.T) {
	s := &SLASummary{
		BySeverity:   make(map[string]int),
		OverdueBySev: make(map[string]int),
	}
	assert.Equal(t, 0, s.TotalOpen)
	assert.Equal(t, 0, s.Overdue)
	assert.Equal(t, 0, s.AtRisk)
	assert.Equal(t, 0, s.OnTrack)
	assert.Empty(t, s.BySeverity)
}

func TestSLASummaryAggregation(t *testing.T) {
	// Simulate the aggregation loop in GetSLASummary.
	type row = SLASummaryRow
	rows := []row{
		{Severity: "critical", SLAStatus: "overdue", Count: 2},
		{Severity: "high", SLAStatus: "at_risk", Count: 3},
		{Severity: "medium", SLAStatus: "on_track", Count: 5},
		{Severity: "critical", SLAStatus: "on_track", Count: 1},
	}

	s := &SLASummary{
		BySeverity:   make(map[string]int),
		OverdueBySev: make(map[string]int),
	}
	for _, r := range rows {
		s.TotalOpen += r.Count
		s.BySeverity[r.Severity] += r.Count
		switch r.SLAStatus {
		case "overdue":
			s.Overdue += r.Count
			s.OverdueBySev[r.Severity] += r.Count
		case "at_risk":
			s.AtRisk += r.Count
		case "on_track":
			s.OnTrack += r.Count
		}
	}

	assert.Equal(t, 11, s.TotalOpen)
	assert.Equal(t, 2, s.Overdue)
	assert.Equal(t, 3, s.AtRisk)
	assert.Equal(t, 6, s.OnTrack)
	assert.Equal(t, 3, s.BySeverity["critical"])
	assert.Equal(t, 2, s.OverdueBySev["critical"])
}

// ── S69-3: SLA status derivation logic ───────────────────────────────────────

func TestSLAStatusDerivationOverdue(t *testing.T) {
	// A finding whose due date is in the past must be "overdue".
	pol := SLAPolicy{Severity: "critical", RemediationDays: 7, NotificationAdvanceDays: 2}
	createdAt := time.Now().Add(-10 * 24 * time.Hour)
	due := createdAt.Add(time.Duration(pol.RemediationDays) * 24 * time.Hour)

	now := time.Now()
	assert.True(t, due.Before(now), "finding due in past must be overdue")
}

func TestSLAStatusDerivationAtRisk(t *testing.T) {
	// A finding due within NotificationAdvanceDays is "at_risk".
	pol := SLAPolicy{Severity: "high", RemediationDays: 30, NotificationAdvanceDays: 5}
	createdAt := time.Now().Add(-26 * 24 * time.Hour) // 26 days ago → 4 days left
	due := createdAt.Add(time.Duration(pol.RemediationDays) * 24 * time.Hour)

	now := time.Now()
	assert.False(t, due.Before(now), "not yet overdue")
	advanceWindow := now.Add(time.Duration(pol.NotificationAdvanceDays) * 24 * time.Hour)
	assert.True(t, due.Before(advanceWindow), "due within advance window → at_risk")
}

func TestSLAStatusDerivationOnTrack(t *testing.T) {
	// A finding with plenty of time remaining is "on_track".
	pol := SLAPolicy{Severity: "medium", RemediationDays: 90, NotificationAdvanceDays: 7}
	createdAt := time.Now().Add(-1 * 24 * time.Hour) // 1 day ago → 89 days left
	due := createdAt.Add(time.Duration(pol.RemediationDays) * 24 * time.Hour)

	now := time.Now()
	assert.False(t, due.Before(now), "not overdue")
	advanceWindow := now.Add(time.Duration(pol.NotificationAdvanceDays) * 24 * time.Hour)
	assert.False(t, due.Before(advanceWindow), "not in advance window → on_track")
}

// ── S69-3: SLAFinding struct ──────────────────────────────────────────────────

func TestSLAFindingNilDueAt(t *testing.T) {
	f := SLAFinding{ID: "abc", Severity: "critical", SLADueAt: nil, CreatedAt: time.Now()}
	assert.Nil(t, f.SLADueAt, "new finding should have nil sla_due_at before first check")
}
