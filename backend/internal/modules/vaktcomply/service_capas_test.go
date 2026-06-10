// Copyright (c) 2026 NorvikOps. All rights reserved.
// SPDX-License-Identifier: Elastic-2.0

package vaktcomply

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

// TestEffectivenessCheckInputZeroValue verifies that a zero-value
// EffectivenessCheckInput is correctly initialised (confirmed defaults to false).
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
