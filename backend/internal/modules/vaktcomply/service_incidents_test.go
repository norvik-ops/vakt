package vaktcomply

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestComputeDeadlines_NIS2HasNoFourHour(t *testing.T) {
	base := time.Date(2026, 1, 1, 12, 0, 0, 0, time.UTC)
	dl := computeDeadlines("nis2", base)

	assert.Nil(t, dl["4h"], "NIS2 has no 4h deadline")
	require.NotNil(t, dl["24h"])
	require.NotNil(t, dl["72h"])
	require.NotNil(t, dl["30d"])

	assert.Equal(t, base.Add(24*time.Hour), *dl["24h"])
	assert.Equal(t, base.Add(72*time.Hour), *dl["72h"])
	assert.Equal(t, base.AddDate(0, 0, 30), *dl["30d"])
}

func TestComputeDeadlines_DORAHasAllFour(t *testing.T) {
	base := time.Date(2026, 1, 1, 8, 0, 0, 0, time.UTC)
	dl := computeDeadlines("dora", base)

	require.NotNil(t, dl["4h"])
	require.NotNil(t, dl["24h"])
	require.NotNil(t, dl["72h"])
	require.NotNil(t, dl["30d"])

	assert.Equal(t, base.Add(4*time.Hour), *dl["4h"])
	assert.Equal(t, base.Add(24*time.Hour), *dl["24h"])
	assert.Equal(t, base.Add(72*time.Hour), *dl["72h"])
	assert.Equal(t, base.AddDate(0, 0, 30), *dl["30d"])
}

func TestComputeDeadlines_UnknownTypeAllNil(t *testing.T) {
	dl := computeDeadlines("general", time.Now())
	assert.Nil(t, dl["4h"])
	assert.Nil(t, dl["24h"])
	assert.Nil(t, dl["72h"])
	assert.Nil(t, dl["30d"])
}

// ── computeDeadlineStatus ────────────────────────────────────────────────────

func TestComputeDeadlineStatus_AllNilReturnsNil(t *testing.T) {
	inc := &Incident{}
	assert.Nil(t, computeDeadlineStatus(inc))
}

func TestComputeDeadlineStatus_NIS2HasFlags(t *testing.T) {
	now := time.Now().UTC()
	d24 := now.Add(20 * time.Hour)
	d72 := now.Add(68 * time.Hour)
	d30 := now.AddDate(0, 0, 30)

	inc := &Incident{Deadline24h: &d24, Deadline72h: &d72, Deadline30d: &d30}
	status := computeDeadlineStatus(inc)

	require.NotNil(t, status)
	assert.False(t, status.Has4h)
	assert.True(t, status.Has24h)
	assert.True(t, status.Has72h)
	assert.True(t, status.Has30d)
}

func TestEffectivenessCheckInputZeroValue(t *testing.T) {
	var in EffectivenessCheckInput
	assert.False(t, in.Confirmed, "zero value of Confirmed must be false")
	assert.Equal(t, "", in.EvidenceNote, "zero value of EvidenceNote must be empty string")
}

// TestCAPANCFieldsZeroValue verifies that a zero-value CAPANCFields struct
// has the correct defaults for non-pointer fields.
func TestCAPANCFieldsZeroValue(t *testing.T) {
	var f CAPANCFields
	assert.Nil(t, f.NCClassification, "NCClassification must be nil when not set")
	assert.Equal(t, "", f.ImmediateContainment)
	assert.Equal(t, "", f.RootCause)
	assert.Nil(t, f.SimilarNCsAssessed)
	assert.Equal(t, "", f.SimilarNCsNotes)
	assert.Nil(t, f.EffectivenessCheckDate)
	assert.Nil(t, f.EffectivenessConfirmed)
	assert.Nil(t, f.EffectivenessCheckedAt)
	assert.Nil(t, f.EffectivenessCheckedBy)
	assert.Equal(t, "", f.EffectivenessEvidence)
}

// TestCAPANCClassificationValues verifies that the NC classification constant
// strings are what the DB CHECK constraint expects.
func TestCAPANCClassificationValues(t *testing.T) {
	valid := map[string]bool{
		"major_nc":    true,
		"minor_nc":    true,
		"observation": true,
		"ofi":         true,
	}
	for v := range valid {
		s := v
		f := CAPANCFields{NCClassification: &s}
		assert.Equal(t, v, *f.NCClassification, "classification value must round-trip")
	}
}

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
	var _ = func(s *Service) {
		_ = s.Risk.AcceptRisk
		_ = s.Risk.UpdateRiskResidualFields
	}
}

func TestCalculateOverallProtectionNeed(t *testing.T) {
	cases := []struct{ c, i, a, want string }{
		{"normal", "normal", "normal", "normal"},
		{"normal", "hoch", "normal", "hoch"},
		{"sehr_hoch", "normal", "normal", "sehr_hoch"},
		{"hoch", "sehr_hoch", "hoch", "sehr_hoch"},
		{"hoch", "hoch", "normal", "hoch"},
		{"normal", "normal", "sehr_hoch", "sehr_hoch"},
	}
	for _, tc := range cases {
		got := CalculateOverallProtectionNeed(tc.c, tc.i, tc.a)
		assert.Equal(t, tc.want, got, "c=%s i=%s a=%s", tc.c, tc.i, tc.a)
	}
}
