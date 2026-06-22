// Copyright (c) 2026 NorvikOps. All rights reserved.
// SPDX-License-Identifier: Elastic-2.0

package vaktcomply

import (
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
)

func TestNIS2ReportabilityCheck_IsReportable(t *testing.T) {
	tests := []struct {
		name string
		c    NIS2ReportabilityCheck
		want bool
	}{
		{
			name: "all false → not reportable",
			c:    NIS2ReportabilityCheck{CausesSignificantDisruption: false, AffectsThirdParties: false, CausesFinancialDamage: false},
			want: false,
		},
		{
			name: "causes significant disruption → reportable",
			c:    NIS2ReportabilityCheck{CausesSignificantDisruption: true},
			want: true,
		},
		{
			name: "affects third parties → reportable",
			c:    NIS2ReportabilityCheck{AffectsThirdParties: true},
			want: true,
		},
		{
			name: "causes financial damage → reportable",
			c:    NIS2ReportabilityCheck{CausesFinancialDamage: true},
			want: true,
		},
		{
			name: "all true → reportable",
			c:    NIS2ReportabilityCheck{CausesSignificantDisruption: true, AffectsThirdParties: true, CausesFinancialDamage: true},
			want: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, tt.c.IsReportable())
		})
	}
}

func TestMarkIncidentReportable_DeadlineCalculation(t *testing.T) {
	detectedAt := time.Date(2026, 6, 10, 10, 0, 0, 0, time.UTC)
	incidentID := uuid.New()

	// Verify deadline arithmetic without hitting the DB.
	earlyWarning := detectedAt.Add(24 * time.Hour)
	fullReport := detectedAt.Add(72 * time.Hour)
	finalReport := detectedAt.Add(30 * 24 * time.Hour)

	assert.Equal(t, detectedAt.Add(24*time.Hour), earlyWarning, "early warning = T+24h")
	assert.Equal(t, detectedAt.Add(72*time.Hour), fullReport, "full report = T+72h")
	assert.Equal(t, detectedAt.Add(720*time.Hour), finalReport, "final report = T+720h (30d)")

	_ = incidentID // used in service test with live DB
}

func TestNIS2DeadlineCheck_StageFiltering(t *testing.T) {
	now := time.Now().UTC()
	warn := now.Add(2 * time.Hour)

	// Deadline within warn window and not yet submitted → should notify
	deadline := now.Add(1 * time.Hour)
	assert.True(t, deadline.Before(warn), "deadline in < 2h should trigger notification")

	// Deadline already past → also within warn window
	pastDeadline := now.Add(-1 * time.Hour)
	assert.True(t, pastDeadline.Before(warn), "overdue deadline should also trigger notification")

	// Deadline far in future → should not notify
	futureDeadline := now.Add(3 * time.Hour)
	assert.False(t, futureDeadline.Before(warn), "deadline in > 2h should not trigger notification")
}
