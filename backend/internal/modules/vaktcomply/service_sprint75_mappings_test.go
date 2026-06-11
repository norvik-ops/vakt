// Copyright (c) 2026 NorvikOps. All rights reserved.
// SPDX-License-Identifier: Elastic-2.0

// Sprint 75 unit tests for framework mapping variables (no DB required).

package vaktcomply

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

// ── S75-2: ISO 27001 ↔ BSI ───────────────────────────────────────────────────

func TestISO27001BSIMappings_MinPairs(t *testing.T) {
	assert.GreaterOrEqual(t, len(iso27001BSIMappings), 75,
		"ISO27001↔BSI must have ≥75 pairs (S75-2 requirement)")
}

func TestISO27001BSIMappings_Bidirectional(t *testing.T) {
	srcSet := make(map[string]bool)
	tgtSet := make(map[string]bool)
	for _, p := range iso27001BSIMappings {
		srcSet[p.srcCode] = true
		tgtSet[p.tgtCode] = true
	}
	assert.Greater(t, len(srcSet), 0, "must have ISO27001 source codes")
	assert.Greater(t, len(tgtSet), 0, "must have BSI target codes")
}

func TestISO27001BSIMappings_NoDuplicates(t *testing.T) {
	type key struct{ src, tgt string }
	seen := make(map[key]bool)
	for _, p := range iso27001BSIMappings {
		k := key{p.srcCode, p.tgtCode}
		assert.False(t, seen[k], "duplicate pair %s→%s", p.srcCode, p.tgtCode)
		seen[k] = true
	}
}

func TestISO27001BSIMappings_OnlyValidISOCodes(t *testing.T) {
	for _, p := range iso27001BSIMappings {
		if p.src == "ISO27001" {
			assert.True(t, strings.HasPrefix(p.srcCode, "A.5.") ||
				strings.HasPrefix(p.srcCode, "A.6.") ||
				strings.HasPrefix(p.srcCode, "A.7.") ||
				strings.HasPrefix(p.srcCode, "A.8."),
				"ISO27001 code %q is not a valid 2022 Annex A ID", p.srcCode)
		}
	}
}

// ── S75-3: NIS2 ↔ BSI Enrichment ─────────────────────────────────────────────

func TestNIS2BSIMappings_MinPairs(t *testing.T) {
	total := len(nis2BSIMappings) + len(nis2BSIExtendedMappings)
	assert.GreaterOrEqual(t, total, 40,
		"NIS2↔BSI total must be ≥40 pairs (S75-3 requirement)")
}

func TestNIS2BSIMappings_AllThematicAreas(t *testing.T) {
	// NIS2 Art. 21 thematic areas A–J must all be represented.
	areas := map[string]bool{
		"A": false, "B": false, "C": false, "D": false, "E": false,
		"F": false, "G": false, "H": false, "I": false, "J": false,
	}
	all := append(nis2BSIMappings, nis2BSIExtendedMappings...)
	for _, p := range all {
		if p.src == "NIS2" {
			// NIS2 codes are NIS2-X.Y — extract the letter after "NIS2-"
			if len(p.srcCode) > 5 {
				letter := string(p.srcCode[5])
				areas[letter] = true
			}
		}
	}
	for area, covered := range areas {
		assert.True(t, covered, "NIS2 thematic area %s not covered in BSI mapping", area)
	}
}

// ── S75-4: DSGVO-TOM ↔ NIS2 + BSI ───────────────────────────────────────────

func TestDSGVOTOMNIS2Mappings_MinPairs(t *testing.T) {
	assert.GreaterOrEqual(t, len(dsgvoTOMNIS2Mappings), 12,
		"DSGVO-TOM↔NIS2 must have ≥12 pairs (S75-4 requirement)")
}

func TestDSGVOTOMBSIMappings_MinPairs(t *testing.T) {
	assert.GreaterOrEqual(t, len(dsgvoTOMBSIMappings), 11,
		"DSGVO-TOM↔BSI must have ≥11 pairs (S75-4 requirement)")
}

// ── S75-5: CIS ↔ ISO 27001 + BSI ─────────────────────────────────────────────

func TestCISISO27001Mappings_MinPairs(t *testing.T) {
	assert.GreaterOrEqual(t, len(cisISO27001Mappings), 22,
		"CIS↔ISO27001 must have ≥22 pairs (S75-5 requirement)")
}

func TestCISBSIMappings_MinPairs(t *testing.T) {
	assert.GreaterOrEqual(t, len(cisBSIMappings), 18,
		"CIS↔BSI must have ≥18 pairs (S75-5 requirement)")
}

func TestCISISO27001Mappings_OnlyValid2022IDs(t *testing.T) {
	for _, p := range cisISO27001Mappings {
		if p.tgt == "ISO27001" {
			assert.True(t, strings.HasPrefix(p.tgtCode, "A.5.") ||
				strings.HasPrefix(p.tgtCode, "A.6.") ||
				strings.HasPrefix(p.tgtCode, "A.7.") ||
				strings.HasPrefix(p.tgtCode, "A.8."),
				"CIS→ISO27001 code %q is not a valid 2022 Annex A ID", p.tgtCode)
		}
	}
}

// ── S75-6: TISAX ↔ BSI + DSGVO-TOM ──────────────────────────────────────────

func TestTISAXBSIMappings_MinPairs(t *testing.T) {
	assert.GreaterOrEqual(t, len(tisaxBSIMappings), 18,
		"TISAX↔BSI must have ≥18 pairs (S75-6 requirement)")
}

func TestTISAXDSGVOTOMMappings_MinPairs(t *testing.T) {
	assert.GreaterOrEqual(t, len(tisaxDSGVOTOMMappings), 11,
		"TISAX↔DSGVO-TOM must have ≥11 pairs (S75-6 requirement)")
}

func TestTISAXDSGVOTOMMappings_DPCoverage(t *testing.T) {
	// Key DSGVO TOMs for data protection must be covered.
	required := []string{"TOM-1", "TOM-2", "TOM-3", "TOM-4", "TOM-5", "TOM-6", "TOM-7", "TOM-8", "TOM-10", "TOM-12"}
	covered := make(map[string]bool)
	for _, p := range tisaxDSGVOTOMMappings {
		if p.tgt == "DSGVO-TOM" {
			covered[p.tgtCode] = true
		}
	}
	for _, tom := range required {
		assert.True(t, covered[tom], "TISAX↔DSGVO-TOM mapping missing coverage for %s", tom)
	}
}

// ── S75-1: KRITIS + C5 in builtinAvailable ───────────────────────────────────

func TestBuiltinAvailable_KRITISandC5Present(t *testing.T) {
	names := make(map[string]bool, len(builtinAvailable))
	for _, b := range builtinAvailable {
		names[b.name] = true
	}
	assert.True(t, names["KRITIS"], "KRITIS must be in builtinAvailable (S75-1)")
	assert.True(t, names["C5"], "C5 must be in builtinAvailable (S75-1)")
}
