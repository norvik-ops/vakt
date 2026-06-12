// Copyright (c) 2026 NorvikOps. All rights reserved.
// SPDX-License-Identifier: Elastic-2.0

package vaktcomply

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestLoadBSICatalog_ParseOK(t *testing.T) {
	cat, err := loadBSICatalog()
	require.NoError(t, err)
	require.NotNil(t, cat)
	assert.Equal(t, "2023", cat.Edition)
	assert.NotEmpty(t, cat.Schichten)
}

func TestBSICatalog_111Bausteine(t *testing.T) {
	cat, err := loadBSICatalog()
	require.NoError(t, err)

	total := 0
	for _, s := range cat.Schichten {
		total += len(s.Bausteine)
	}
	assert.Equal(t, 111, total, "catalog must contain exactly 111 Bausteine")
}

func TestBSICatalog_NoDuplicateIDs(t *testing.T) {
	cat, err := loadBSICatalog()
	require.NoError(t, err)

	seen := make(map[string]bool)
	for _, schicht := range cat.Schichten {
		for _, baustein := range schicht.Bausteine {
			for _, anf := range baustein.Anforderungen {
				assert.False(t, seen[anf.ID], "duplicate anforderung ID: %s", anf.ID)
				seen[anf.ID] = true
			}
		}
	}
}

func TestBSICatalog_ValidStufen(t *testing.T) {
	cat, err := loadBSICatalog()
	require.NoError(t, err)

	validStufen := map[string]bool{"basis": true, "standard": true, "erhoeht": true}
	for _, schicht := range cat.Schichten {
		for _, baustein := range schicht.Bausteine {
			for _, anf := range baustein.Anforderungen {
				assert.True(t, validStufen[anf.Stufe], "invalid stufe %q for %s", anf.Stufe, anf.ID)
				assert.NotEmpty(t, anf.Stufe, "missing stufe for %s", anf.ID)
			}
		}
	}
}

func TestBSICatalog_IDsHaveBSIPrefix(t *testing.T) {
	cat, err := loadBSICatalog()
	require.NoError(t, err)

	for _, schicht := range cat.Schichten {
		for _, baustein := range schicht.Bausteine {
			for _, anf := range baustein.Anforderungen {
				assert.True(t, strings.HasPrefix(anf.ID, "BSI-"), "ID must start with BSI-: %s", anf.ID)
			}
		}
	}
}

func TestBSICatalog_ExistingIDsPreserved(t *testing.T) {
	// The 80 original IDs from S74 must remain unchanged (91 ISO↔BSI mappings from S75 depend on them).
	required := []string{
		"BSI-ISMS.1.A1", "BSI-ISMS.1.A2", "BSI-ISMS.1.A4", "BSI-ISMS.1.A5",
		"BSI-ISMS.1.A6", "BSI-ISMS.1.A9", "BSI-ISMS.1.A10",
		"BSI-ORP.1.A1", "BSI-ORP.2.A1", "BSI-ORP.2.A2", "BSI-ORP.2.A3", "BSI-ORP.2.A4",
		"BSI-ORP.3.A1", "BSI-ORP.4.A1", "BSI-ORP.5.A1",
		"BSI-CON.1.A1", "BSI-CON.2.A1", "BSI-CON.3.A1", "BSI-CON.4.A1", "BSI-CON.5.A1",
		"BSI-CON.6.A1", "BSI-CON.7.A1", "BSI-CON.8.A1", "BSI-CON.9.A1", "BSI-CON.10.A1",
		"BSI-OPS.1.1.2.A1", "BSI-OPS.1.1.3.A1", "BSI-OPS.1.1.4.A1", "BSI-OPS.1.1.4.A2",
		"BSI-OPS.1.1.5.A1", "BSI-OPS.1.1.6.A1", "BSI-OPS.1.1.7.A1", "BSI-OPS.1.2.5.A1",
		"BSI-OPS.2.2.A1", "BSI-OPS.2.4.A1",
		"BSI-DER.1.A1", "BSI-DER.1.A2", "BSI-DER.2.1.A1", "BSI-DER.2.2.A1", "BSI-DER.3.2.A1",
		"BSI-BCM.1.A1", "BSI-BCM.1.A2", "BSI-BCM.2.A1",
		"BSI-APP.1.1.A1", "BSI-APP.1.2.A1", "BSI-APP.1.4.A1",
		"BSI-APP.2.1.A1", "BSI-APP.2.3.A1",
		"BSI-APP.3.1.A1", "BSI-APP.3.2.A1",
		"BSI-APP.4.4.A1", "BSI-APP.4.4.A2", "BSI-APP.5.1.A1", "BSI-APP.5.3.A1",
		"BSI-SYS.1.1.A1", "BSI-SYS.1.2.A1", "BSI-SYS.1.3.A1", "BSI-SYS.1.5.A1",
		"BSI-SYS.1.6.A1", "BSI-SYS.2.1.A1", "BSI-SYS.2.2.3.A1",
		"BSI-SYS.3.1.A1", "BSI-SYS.3.2.A1", "BSI-SYS.4.1.A1", "BSI-SYS.4.5.A1",
		"BSI-IND.1.A1",
		"BSI-NET.1.1.A1", "BSI-NET.1.2.A1", "BSI-NET.3.1.A1", "BSI-NET.3.2.A1",
		"BSI-NET.4.1.A1", "BSI-NET.4.5.A1",
		"BSI-INF.1.A1", "BSI-INF.2.A1", "BSI-INF.3.A1", "BSI-INF.5.A1",
		"BSI-INF.7.A1", "BSI-INF.8.A1", "BSI-INF.10.A1",
		"BSI-DER.4.A1",
	}

	levels := bsiRequirementLevels()
	for _, id := range required {
		_, ok := levels[id]
		assert.True(t, ok, "existing ID missing from catalog: %s", id)
	}
}

func TestBSIControls_CountMatchesCatalog(t *testing.T) {
	controls := bsiControls("fw-test", "org-test")
	require.NotEmpty(t, controls)

	cat, err := loadBSICatalog()
	require.NoError(t, err)

	var anfCount int
	for _, s := range cat.Schichten {
		for _, b := range s.Bausteine {
			anfCount += len(b.Anforderungen)
		}
	}
	assert.Equal(t, anfCount, len(controls), "bsiControls count must match catalog anforderungen count")
}

func TestKompendiumProvider_Controls_CountMatchesCatalog(t *testing.T) {
	p := KompendiumProvider{}
	controls, err := p.Controls("fw-test", "org-test")
	require.NoError(t, err)
	require.NotEmpty(t, controls)

	cat, err := loadBSICatalog()
	require.NoError(t, err)

	var anfCount int
	for _, s := range cat.Schichten {
		for _, b := range s.Bausteine {
			anfCount += len(b.Anforderungen)
		}
	}
	assert.Equal(t, anfCount, len(controls), "KompendiumProvider.Controls count must match catalog anforderungen")
}

func TestKompendiumProvider_Metadata_Edition(t *testing.T) {
	p := KompendiumProvider{}
	meta := p.Metadata()
	assert.Equal(t, "2023", meta.Edition)
	assert.Equal(t, "BSI IT-Grundschutz Kompendium", meta.Source)
}

func TestGsppProvider_Controls_ReturnsError(t *testing.T) {
	p := gsppProvider{}
	controls, err := p.Controls("fw-test", "org-test")
	require.Error(t, err)
	assert.Nil(t, controls)
	assert.Contains(t, err.Error(), "it-sa")
}

func TestGsppProvider_Metadata(t *testing.T) {
	p := gsppProvider{}
	meta := p.Metadata()
	assert.Equal(t, "BSI Grundschutz++", meta.Source)
	assert.Equal(t, "draft", meta.Edition)
}

func TestCatalogRegistry_BSIAndGSPPRegistered(t *testing.T) {
	bsiProv, ok := catalogRegistry["BSI"]
	require.True(t, ok, "BSI must be in catalogRegistry")
	assert.IsType(t, KompendiumProvider{}, bsiProv)

	gsppProv, ok := catalogRegistry["GSPP"]
	require.True(t, ok, "GSPP must be in catalogRegistry")
	assert.IsType(t, gsppProvider{}, gsppProv)
}

func TestBSIRequirementLevels_AllHaveLevel(t *testing.T) {
	levels := bsiRequirementLevels()
	require.NotEmpty(t, levels)
	for id, stufe := range levels {
		assert.NotEmpty(t, stufe, "empty stufe for %s", id)
	}
}

func TestKompendiumScorer_ScoreFiltered_BasisOnly_CatalogTest(t *testing.T) {
	rows := []BSICheckResult{
		{RequirementLevel: "basis", Umsetzungsstatus: "ja"},
		{RequirementLevel: "basis", Umsetzungsstatus: "nein"},
		{RequirementLevel: "standard", Umsetzungsstatus: "nein"},
		{RequirementLevel: "erhoeht", Umsetzungsstatus: "nein"},
	}
	// Basis-Absicherung: only the 2 basis rows count
	pct := KompendiumScorer{}.ScoreFiltered(rows, "basis")
	// ja=1, teilweise=0, entbehrlich=0, total=2 → 1/2 * 100 = 50
	assert.InDelta(t, 50.0, pct, 0.01)
}

func TestKompendiumScorer_ScoreFiltered_StandardIncludesBasis_CatalogTest(t *testing.T) {
	rows := []BSICheckResult{
		{RequirementLevel: "basis", Umsetzungsstatus: "ja"},
		{RequirementLevel: "standard", Umsetzungsstatus: "ja"},
		{RequirementLevel: "erhoeht", Umsetzungsstatus: "nein"},
	}
	// Standard: basis + standard = 2 rows, both ja; erhoeht excluded
	pct := KompendiumScorer{}.ScoreFiltered(rows, "standard")
	assert.InDelta(t, 100.0, pct, 0.01)
}

func TestKompendiumScorer_ScoreFiltered_KernSameAsStandard_CatalogTest(t *testing.T) {
	rows := []BSICheckResult{
		{RequirementLevel: "basis", Umsetzungsstatus: "ja"},
		{RequirementLevel: "standard", Umsetzungsstatus: "nein"},
		{RequirementLevel: "erhoeht", Umsetzungsstatus: "nein"},
	}
	pctStandard := KompendiumScorer{}.ScoreFiltered(rows, "standard")
	pctKern := KompendiumScorer{}.ScoreFiltered(rows, "kern")
	assert.Equal(t, pctStandard, pctKern, "kern and standard should produce the same result per BSI 200-2 §8.3")
}
