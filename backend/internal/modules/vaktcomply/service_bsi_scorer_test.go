// Copyright (c) 2026 NorvikOps. All rights reserved.
// SPDX-License-Identifier: Elastic-2.0

package vaktcomply

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

// dummyScorer proves the interface is exchangeable: always returns fixed values.
type dummyScorer struct{ fixed float64 }

func (d dummyScorer) Score(_, _, _, _ int) float64                              { return d.fixed }
func (d dummyScorer) ScoreFiltered(_ []BSICheckResult, _ string) float64        { return d.fixed }

var _ ComplianceScorer = KompendiumScorer{}  // compile-time interface check
var _ ComplianceScorer = dummyScorer{}       // compile-time interface check

func TestKompendiumScorer_Score(t *testing.T) {
	s := KompendiumScorer{}

	assert.InDelta(t, 100.0, s.Score(0, 0, 5, 5), 0.01, "all entbehrlich → 100%")
	assert.InDelta(t, 100.0, s.Score(2, 0, 0, 2), 0.01, "all ja → 100%")
	assert.InDelta(t, 50.0, s.Score(1, 0, 0, 2), 0.01, "half ja")
	assert.InDelta(t, 25.0, s.Score(0, 1, 0, 2), 0.01, "one teilweise of 2 → 0.5/2 = 25%")
	assert.InDelta(t, 75.0, s.Score(1, 1, 0, 2), 0.01, "ja + teilweise of 2 → 1.5/2 = 75%")
	assert.InDelta(t, 100.0, s.Score(0, 0, 0, 0), 0.01, "no rows → 100% (all entbehrlich edge)")
}

func TestKompendiumScorer_ScoreFiltered_BasisOnly(t *testing.T) {
	rows := []BSICheckResult{
		{RequirementLevel: "basis", Umsetzungsstatus: "ja"},
		{RequirementLevel: "basis", Umsetzungsstatus: "nein"},
		{RequirementLevel: "standard", Umsetzungsstatus: "nein"},
		{RequirementLevel: "erhoeht", Umsetzungsstatus: "nein"},
	}
	pct := KompendiumScorer{}.ScoreFiltered(rows, "basis")
	// ja=1, total=2 → 50%
	assert.InDelta(t, 50.0, pct, 0.01)
}

func TestKompendiumScorer_ScoreFiltered_StandardIncludesBasis(t *testing.T) {
	rows := []BSICheckResult{
		{RequirementLevel: "basis", Umsetzungsstatus: "ja"},
		{RequirementLevel: "standard", Umsetzungsstatus: "ja"},
		{RequirementLevel: "erhoeht", Umsetzungsstatus: "nein"},
	}
	pct := KompendiumScorer{}.ScoreFiltered(rows, "standard")
	assert.InDelta(t, 100.0, pct, 0.01)
}

func TestKompendiumScorer_ScoreFiltered_KernSameAsStandard(t *testing.T) {
	rows := []BSICheckResult{
		{RequirementLevel: "basis", Umsetzungsstatus: "ja"},
		{RequirementLevel: "standard", Umsetzungsstatus: "nein"},
		{RequirementLevel: "erhoeht", Umsetzungsstatus: "nein"},
	}
	pctStandard := KompendiumScorer{}.ScoreFiltered(rows, "standard")
	pctKern := KompendiumScorer{}.ScoreFiltered(rows, "kern")
	assert.Equal(t, pctStandard, pctKern, "kern and standard produce same result per BSI 200-2 §8.3")
}

func TestDummyScorer_Exchangeable(t *testing.T) {
	var s ComplianceScorer = dummyScorer{fixed: 42.0}
	assert.InDelta(t, 42.0, s.Score(1, 2, 3, 4), 0.01)
	assert.InDelta(t, 42.0, s.ScoreFiltered(nil, "basis"), 0.01)
}
