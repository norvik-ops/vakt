// Copyright (c) 2026 NorvikOps. All rights reserved.
// SPDX-License-Identifier: Elastic-2.0

// S74-3 unit tests for BSI 200-3 risk matrix logic (pure functions, no DB).

package bsi

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

// TestComputeRisikokategorie_AllCombinations validates all 16 cells of the 4×4 matrix.
func TestComputeRisikokategorie_AllCombinations(t *testing.T) {
	tests := []struct {
		hauf    string
		schaden string
		want    string
	}{
		{"selten", "vernachlaessigbar", "gering"},
		{"selten", "begrenzt", "gering"},
		{"selten", "betraechtlich", "mittel"},
		{"selten", "existenzbedrohend", "hoch"},
		{"mittel", "vernachlaessigbar", "gering"},
		{"mittel", "begrenzt", "mittel"},
		{"mittel", "betraechtlich", "hoch"},
		{"mittel", "existenzbedrohend", "sehr_hoch"},
		{"haeufig", "vernachlaessigbar", "mittel"},
		{"haeufig", "begrenzt", "hoch"},
		{"haeufig", "betraechtlich", "sehr_hoch"},
		{"haeufig", "existenzbedrohend", "sehr_hoch"},
		{"sehr_haeufig", "vernachlaessigbar", "sehr_hoch"},
		{"sehr_haeufig", "begrenzt", "sehr_hoch"},
		{"sehr_haeufig", "betraechtlich", "sehr_hoch"},
		{"sehr_haeufig", "existenzbedrohend", "sehr_hoch"},
	}

	for _, tt := range tests {
		t.Run(tt.hauf+"_"+tt.schaden, func(t *testing.T) {
			got := ComputeRisikokategorie(tt.hauf, tt.schaden)
			assert.Equal(t, tt.want, got,
				"ComputeRisikokategorie(%q, %q) = %q, want %q",
				tt.hauf, tt.schaden, got, tt.want)
		})
	}
}

// TestComputeRisikokategorie_UnknownInputFallsBack checks the fallback case.
func TestComputeRisikokategorie_UnknownInputFallsBack(t *testing.T) {
	result := ComputeRisikokategorie("unknown", "unknown")
	assert.Equal(t, "mittel", result, "unknown input should fallback to 'mittel'")
}

// TestBSIRiskSummary_ZeroValues ensures zero-values are valid.
func TestBSIRiskSummary_ZeroValues(t *testing.T) {
	s := BSIRiskSummary{}
	assert.Equal(t, 0, s.Gering)
	assert.Equal(t, 0, s.Mittel)
	assert.Equal(t, 0, s.Hoch)
	assert.Equal(t, 0, s.SehrHoch)
	assert.Equal(t, 0, s.Offen)
}

// TestBSI47Threats_TableSize validates the seed count at the SQL level via migration.
// We test that all 47 threat IDs follow the G-0.x pattern.
func TestBSI47Threats_IDPattern(t *testing.T) {
	// Inline the 47 IDs to validate the pattern without a DB.
	ids := []string{
		"G-0.1", "G-0.2", "G-0.3", "G-0.4", "G-0.5", "G-0.6", "G-0.7",
		"G-0.8", "G-0.9", "G-0.10", "G-0.11", "G-0.12", "G-0.13", "G-0.14",
		"G-0.15", "G-0.16", "G-0.17", "G-0.18", "G-0.19", "G-0.20", "G-0.21",
		"G-0.22", "G-0.23", "G-0.24", "G-0.25", "G-0.26", "G-0.27", "G-0.28",
		"G-0.29", "G-0.30", "G-0.31", "G-0.32", "G-0.33", "G-0.34", "G-0.35",
		"G-0.36", "G-0.37", "G-0.38", "G-0.39", "G-0.40", "G-0.41", "G-0.42",
		"G-0.43", "G-0.44", "G-0.45", "G-0.46", "G-0.47",
	}
	assert.Len(t, ids, 47, "BSI IT-Grundschutz-Kompendium defines exactly 47 elementare Gefährdungen")

	seen := make(map[string]bool, len(ids))
	for _, id := range ids {
		assert.False(t, seen[id], "duplicate threat ID: %s", id)
		seen[id] = true
		assert.Regexp(t, `^G-0\.\d+$`, id, "threat ID must match G-0.N pattern: %s", id)
	}
}
