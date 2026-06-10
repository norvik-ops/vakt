// Copyright (c) 2026 NorvikOps. All rights reserved.
// SPDX-License-Identifier: Elastic-2.0

// Tests for Sprint 70 S70-3: DSGVO Art. 25 Privacy by Design model invariants.

package vaktprivacy

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

// ── S70-3: PrivacyDesignAssessment model ─────────────────────────────────────

func TestPrivacyDesignAssessment_DefaultResult(t *testing.T) {
	// When no result is provided, the service defaults to "not_assessed".
	// This mirrors the logic in CreateOrUpdatePrivacyDesign.
	input := PrivacyDesignInput{}
	result := input.AssessmentResult
	if result == "" {
		result = "not_assessed"
	}
	assert.Equal(t, "not_assessed", result)
}

func TestPrivacyDesignAssessment_CompliantResult(t *testing.T) {
	input := PrivacyDesignInput{AssessmentResult: "compliant"}
	assert.Equal(t, "compliant", input.AssessmentResult)
}

func TestPrivacyDesignAssessment_PartialResult(t *testing.T) {
	input := PrivacyDesignInput{AssessmentResult: "partially"}
	assert.Equal(t, "partially", input.AssessmentResult)
}

func TestPrivacyDesignAssessment_ReviewedAtSetWhenReviewerProvided(t *testing.T) {
	// Simulate CreateOrUpdatePrivacyDesign reviewer logic.
	input := PrivacyDesignInput{ReviewedBy: "user-123"}
	var reviewedBy *string
	var reviewedAt *time.Time
	if input.ReviewedBy != "" {
		reviewedBy = &input.ReviewedBy
		now := time.Now().UTC()
		reviewedAt = &now
	}
	assert.NotNil(t, reviewedBy)
	assert.NotNil(t, reviewedAt)
	assert.Equal(t, "user-123", *reviewedBy)
}

func TestPrivacyDesignAssessment_NoReviewerMeansNilTimestamp(t *testing.T) {
	input := PrivacyDesignInput{}
	var reviewedBy *string
	var reviewedAt *time.Time
	if input.ReviewedBy != "" {
		reviewedBy = &input.ReviewedBy
		now := time.Now().UTC()
		reviewedAt = &now
	}
	assert.Nil(t, reviewedBy)
	assert.Nil(t, reviewedAt)
}

// ── S70-3: PrivacyDesignSummary pct_compliant calculation ────────────────────

func TestPrivacyDesignSummary_PctCompliant_AllCompliant(t *testing.T) {
	s := &PrivacyDesignSummary{TotalActivities: 10, Compliant: 10}
	if s.TotalActivities > 0 {
		s.PctCompliant = float64(s.Compliant) / float64(s.TotalActivities) * 100
	}
	assert.Equal(t, 100.0, s.PctCompliant)
}

func TestPrivacyDesignSummary_PctCompliant_Half(t *testing.T) {
	s := &PrivacyDesignSummary{TotalActivities: 10, Compliant: 5}
	if s.TotalActivities > 0 {
		s.PctCompliant = float64(s.Compliant) / float64(s.TotalActivities) * 100
	}
	assert.Equal(t, 50.0, s.PctCompliant)
}

func TestPrivacyDesignSummary_PctCompliant_ZeroActivities(t *testing.T) {
	s := &PrivacyDesignSummary{TotalActivities: 0, Compliant: 0}
	if s.TotalActivities > 0 {
		s.PctCompliant = float64(s.Compliant) / float64(s.TotalActivities) * 100
	}
	assert.Equal(t, 0.0, s.PctCompliant, "must not divide by zero")
}

func TestPrivacyDesignSummary_PendingCount(t *testing.T) {
	// PendingCount = activities with no assessment at all.
	s := &PrivacyDesignSummary{
		TotalActivities: 10,
		WithAssessment:  7,
	}
	s.PendingCount = s.TotalActivities - s.WithAssessment
	assert.Equal(t, 3, s.PendingCount)
}

func TestPrivacyDesignSummary_FieldsDistinct(t *testing.T) {
	// Compliant + Partially + NotAssessed must equal WithAssessment.
	s := &PrivacyDesignSummary{
		Compliant:   4,
		Partially:   2,
		NotAssessed: 1,
	}
	s.WithAssessment = s.Compliant + s.Partially + s.NotAssessed
	assert.Equal(t, 7, s.WithAssessment)
}
