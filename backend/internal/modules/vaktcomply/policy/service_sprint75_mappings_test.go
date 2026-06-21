// Copyright (c) 2026 NorvikOps. All rights reserved.
// SPDX-License-Identifier: Elastic-2.0

// Sprint 75 unit tests for framework mapping variables (no DB required).

package policy

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

// ── S75-4: Specific pair assertions (AC4) + semantic regression guards ────────

// TestDSGVOTOMBSI_AC4_TOM2_EquivalentORP4 verifies story S75-4 AC4:
// TOM-2 (Zugangskontrolle) must have an equivalent mapping to BSI-ORP.4.A1 (IAM).
func TestDSGVOTOMBSI_AC4_TOM2_EquivalentORP4(t *testing.T) {
	found := false
	for _, p := range dsgvoTOMBSIMappings {
		if p.srcCode == "TOM-2" && p.tgtCode == "BSI-ORP.4.A1" && p.mtype == "equivalent" {
			found = true
		}
	}
	assert.True(t, found, "S75-4 AC4: dsgvoTOMBSIMappings must contain TOM-2→BSI-ORP.4.A1 equivalent")
}

// TestDSGVOTOMNIS2_SemanticRegression_TOM4_ToTLS guards against re-introducing the
// bug where TOM-4 (Weitergabekontrolle) was incorrectly mapped to NIS2-G.1
// (Cyberhygiene/Schulungen) instead of NIS2-H.4 (Verschlüsselung übertragener Daten / TLS).
func TestDSGVOTOMNIS2_SemanticRegression_TOM4_ToTLS(t *testing.T) {
	hasTLSmapping := false
	hasWrongCyberhygiene := false
	for _, p := range dsgvoTOMNIS2Mappings {
		if p.srcCode == "TOM-4" {
			if p.tgtCode == "NIS2-H.4" {
				hasTLSmapping = true
			}
			if p.tgtCode == "NIS2-G.1" {
				hasWrongCyberhygiene = true
			}
		}
	}
	assert.True(t, hasTLSmapping, "TOM-4 (Weitergabekontrolle) must map to NIS2-H.4 (TLS)")
	assert.False(t, hasWrongCyberhygiene, "TOM-4 must NOT map to NIS2-G.1 (Cyberhygiene — wrong thematic area)")
}

// TestDSGVOTOMNIS2_SemanticRegression_TOM1_ToPhysical guards against TOM-1 (Zutrittskontrolle)
// being mapped to NIS2-H.1 (Kryptographierichtlinie) — a thematic area mismatch.
func TestDSGVOTOMNIS2_SemanticRegression_TOM1_ToPhysical(t *testing.T) {
	hasPhysical := false
	hasWrongCrypto := false
	for _, p := range dsgvoTOMNIS2Mappings {
		if p.srcCode == "TOM-1" {
			if p.tgtCode == "NIS2-I.10" {
				hasPhysical = true
			}
			if p.tgtCode == "NIS2-H.1" || p.tgtCode == "NIS2-H.2" {
				hasWrongCrypto = true
			}
		}
	}
	assert.True(t, hasPhysical, "TOM-1 (Zutrittskontrolle) must map to NIS2-I.10 (Physische Sicherheitsmaßnahmen)")
	assert.False(t, hasWrongCrypto, "TOM-1 must NOT map to NIS2-H.x (Kryptographie — wrong thematic area)")
}

// TestNIS2BSIExtended_SemanticRegression_H_to_Crypto guards against NIS2-H.x (Kryptographie)
// being mapped to BSI-INF.x (Gebäude/Rechenzentrum) — a critical thematic area confusion.
func TestNIS2BSIExtended_SemanticRegression_H_to_Crypto(t *testing.T) {
	for _, p := range nis2BSIExtendedMappings {
		if strings.HasPrefix(p.srcCode, "NIS2-H.") {
			assert.True(t, strings.HasPrefix(p.tgtCode, "BSI-CON."),
				"NIS2-H.x (Kryptographie) must map to BSI-CON.x, got %s→%s", p.srcCode, p.tgtCode)
		}
	}
}

// TestNIS2BSIExtended_SemanticRegression_D_to_SupplyChain guards against NIS2-D.x (Supply-Chain)
// being mapped to BSI-DER.4.A1 (Notfallmanagement) — a category mismatch.
func TestNIS2BSIExtended_SemanticRegression_D_to_SupplyChain(t *testing.T) {
	for _, p := range nis2BSIExtendedMappings {
		if strings.HasPrefix(p.srcCode, "NIS2-D.") {
			assert.True(t, strings.HasPrefix(p.tgtCode, "BSI-OPS.2."),
				"NIS2-D.x (Supply-Chain) must map to BSI-OPS.2.x, got %s→%s", p.srcCode, p.tgtCode)
		}
	}
}

// TestNIS2BSIExtended_SemanticRegression_G_to_Training guards against NIS2-G.x (Cyberhygiene/Schulungen)
// being mapped to BSI-CON.1.A1 (Kryptokonzept) — a content mismatch.
func TestNIS2BSIExtended_SemanticRegression_G_to_Training(t *testing.T) {
	for _, p := range nis2BSIExtendedMappings {
		if strings.HasPrefix(p.srcCode, "NIS2-G.") {
			assert.True(t, strings.HasPrefix(p.tgtCode, "BSI-ORP.3."),
				"NIS2-G.x (Cyberhygiene/Schulungen) must map to BSI-ORP.3.x (Schulung), got %s→%s", p.srcCode, p.tgtCode)
		}
	}
}
