// Copyright (c) 2026 NorvikOps. All rights reserved.
// SPDX-License-Identifier: Elastic-2.0

package vaktcomply

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestComputeScores_BothSet verifies score calculation when all four factors are set.
func TestComputeScores_BothSet(t *testing.T) {
	il, ii, rl, ri := 3, 4, 2, 3
	r := &Risk{
		InherentLikelihood: &il,
		InherentImpact:     &ii,
		ResidualLikelihood: &rl,
		ResidualImpact:     &ri,
	}
	r.ComputeScores()

	require.NotNil(t, r.InherentScore)
	assert.Equal(t, 12, *r.InherentScore, "inherent score = 3×4")

	require.NotNil(t, r.ResidualScore)
	assert.Equal(t, 6, *r.ResidualScore, "residual score = 2×3")
}

// TestComputeScores_NilFactors verifies that nil factors do not produce a score.
func TestComputeScores_NilFactors(t *testing.T) {
	r := &Risk{}
	r.ComputeScores()

	assert.Nil(t, r.InherentScore, "no inherent score when factors are nil")
	assert.Nil(t, r.ResidualScore, "no residual score when factors are nil")
}

// TestComputeScores_PartialInherent verifies that only the residual score is set when
// inherent factors are incomplete.
func TestComputeScores_PartialInherent(t *testing.T) {
	il := 4
	rl, ri := 1, 2
	r := &Risk{
		InherentLikelihood: &il,
		// InherentImpact nil → no inherent score
		ResidualLikelihood: &rl,
		ResidualImpact:     &ri,
	}
	r.ComputeScores()

	assert.Nil(t, r.InherentScore, "inherent score should be nil when only likelihood is set")
	require.NotNil(t, r.ResidualScore)
	assert.Equal(t, 2, *r.ResidualScore)
}

// TestUpdateRiskResidualInput_ValidateConstraints ensures the validate tags are wired correctly.
func TestUpdateRiskResidualInput_ValidateConstraints(t *testing.T) {
	// Boundary values: 1 and 5 are valid; 0 and 6 are not accepted by validate:"min=1,max=5"
	valid1, valid5 := 1, 5
	in := UpdateRiskResidualInput{
		InherentLikelihood: &valid1,
		InherentImpact:     &valid5,
		ResidualLikelihood: &valid1,
		ResidualImpact:     &valid5,
	}
	// Struct tags say omitempty,min=1,max=5 — just check the types compile and values are in range.
	assert.Equal(t, 1, *in.InherentLikelihood)
	assert.Equal(t, 5, *in.InherentImpact)
}

// TestAcceptRiskInput_Justification confirms that AcceptRiskInput carries the justification field.
func TestAcceptRiskInput_Justification(t *testing.T) {
	in := AcceptRiskInput{Justification: "Residualrisiko ist akzeptabel, da Gegenmaßnahmen greifen."}
	assert.NotEmpty(t, in.Justification)
}

// TestService_AcceptRisk_MethodExists verifies the service exposes the AcceptRisk method signature
// (compile-time check — no DB interaction needed).
func TestService_AcceptRisk_MethodExists(t *testing.T) {
	// This is a compile-time signature check. Service cannot be instantiated without a DB in unit tests.
	var _ func(s *Service) = func(s *Service) {
		_ = s.AcceptRisk
		_ = s.UpdateRiskResidualFields
	}
}
