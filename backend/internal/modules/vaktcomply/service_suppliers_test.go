// Copyright (c) 2026 NorvikOps. All rights reserved.
// SPDX-License-Identifier: Elastic-2.0

package vaktcomply

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ── parseSupplierCSVRows — edge cases not covered in service_test.go ─────────

func TestParseSupplierCSVRows_AllValidCriticalities(t *testing.T) {
	for _, crit := range []string{"low", "medium", "high", "critical", "standard", "important"} {
		csv := "name,criticality\nAcme," + crit + "\n"
		rows, err := parseSupplierCSVRows(csv)
		require.NoError(t, err, "criticality=%s", crit)
		require.Len(t, rows, 1, "criticality=%s", crit)
		assert.Equal(t, crit, rows[0].Criticality)
	}
}

func TestParseSupplierCSVRows_BoolTrueVariants(t *testing.T) {
	for _, val := range []string{"True", "TRUE", "1"} {
		csv := "name,nis2_relevant\nAcme," + val + "\n"
		rows, err := parseSupplierCSVRows(csv)
		require.NoError(t, err)
		require.Len(t, rows, 1)
		assert.True(t, rows[0].NIS2Relevant, "input=%q should parse as true", val)
	}
}

// ── computeStatus — branches not covered in service_test.go ─────────────────

func TestComputeStatus_InProgressAssessment(t *testing.T) {
	supplier := Supplier{ID: "s1"}
	assessments := []Assessment{{ID: "a1", Status: "in_progress"}}
	st := computeStatus(supplier, assessments, nil, time.Now())
	assert.Equal(t, "yellow", st.Status)
	assert.Equal(t, "assessment_pending", st.Details["reason"])
}

func TestComputeStatus_SubmittedAwaitingReview(t *testing.T) {
	supplier := Supplier{ID: "s1"}
	assessments := []Assessment{{ID: "a1", Status: "submitted"}}
	st := computeStatus(supplier, assessments, nil, time.Now())
	assert.Equal(t, "yellow", st.Status)
	assert.Equal(t, "awaiting_review", st.Details["reason"])
}

func TestComputeStatus_ReviewedNoAnswers_Fallback(t *testing.T) {
	supplier := Supplier{ID: "s1"}
	assessments := []Assessment{{ID: "a1", Status: "reviewed"}}
	st := computeStatus(supplier, assessments, nil, time.Now())
	// reviewed but empty answers → falls through to fallback yellow
	assert.Equal(t, "yellow", st.Status)
}
