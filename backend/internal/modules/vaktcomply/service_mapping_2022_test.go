// Copyright (c) 2026 NorvikOps. All rights reserved.
// SPDX-License-Identifier: Elastic-2.0

package vaktcomply

import (
	"fmt"
	"regexp"
	"strings"
	"testing"
)

// iso2013Pattern matches ISO 27001:2013-only control IDs that no longer exist
// after migration 191. These chapters were restructured or renamed in 2022:
// A.9.x (access control), A.10.x (cryptography), A.11.x (physical),
// A.12.x (operations), A.13.x (comms), A.14.x (dev), A.15.x (suppliers),
// A.16.x (incidents), A.17.x (BCM), A.18.x (compliance).
// Also any three-level ID (A.x.y.z) which are sub-controls merged in 2022.
var iso2013Pattern = regexp.MustCompile(
	`\bA\.(9|10|11|12|13|14|15|16|17|18)\.\d+` + // 2013-only chapter ranges
		`|\bA\.\d+\.\d+\.\d+`, // three-level sub-control (A.x.y.z)
)

// TestMappingSeedsUseOnly2022IDs verifies that all static mapping tables reference
// ISO 27001:2022 Annex A control IDs only (A.5.x–A.8.x, no 2013-legacy IDs).
func TestMappingSeedsUseOnly2022IDs(t *testing.T) {
	t.Run("craMappings", func(t *testing.T) {
		for _, p := range craMappings {
			checkNotLegacy(t, p.tgtCode)
		}
	})

	t.Run("euAIActISO27001Mappings", func(t *testing.T) {
		for _, p := range euAIActISO27001Mappings {
			checkNotLegacy(t, p.tgtCode)
		}
	})

	t.Run("iso42001ISO27001Mappings", func(t *testing.T) {
		for _, p := range iso42001ISO27001Mappings {
			checkNotLegacy(t, p.tgtCode)
		}
	})

	t.Run("tisaxToISO27001Mappings", func(t *testing.T) {
		for tisax, iso := range tisaxToISO27001Mappings {
			if iso2013Pattern.MatchString(iso) {
				t.Errorf("tisaxToISO27001Mappings[%q] = %q contains legacy ISO 27001:2013 ID", tisax, iso)
			}
		}
	})

	t.Run("buildPrerequisiteChains_ISO27001", func(t *testing.T) {
		for _, ch := range buildPrerequisiteChains() {
			if ch.ControlFW == "ISO27001" {
				checkNotLegacy(t, ch.ControlCode)
			}
			if ch.PrereqFW == "ISO27001" {
				checkNotLegacy(t, ch.PrereqCode)
			}
		}
	})

	t.Run("dsgvoTOMISO27001Mappings", func(t *testing.T) {
		for _, p := range dsgvoTOMISO27001Mappings {
			checkNotLegacy(t, p.tgtCode)
		}
	})

	t.Run("tisaxISO27001Mappings", func(t *testing.T) {
		for _, p := range tisaxISO27001Mappings {
			checkNotLegacy(t, p.tgtCode)
		}
	})

	// S75 — new mapping variables that reference ISO 27001:2022 IDs.
	t.Run("iso27001BSIMappings_ISO_codes", func(t *testing.T) {
		for _, p := range iso27001BSIMappings {
			if p.src == "ISO27001" {
				checkNotLegacy(t, p.srcCode)
			}
		}
	})

	t.Run("cisISO27001Mappings_ISO_codes", func(t *testing.T) {
		for _, p := range cisISO27001Mappings {
			if p.tgt == "ISO27001" {
				checkNotLegacy(t, p.tgtCode)
			}
		}
	})
}

func checkNotLegacy(t *testing.T, id string) {
	t.Helper()
	if iso2013Pattern.MatchString(id) {
		t.Errorf("legacy ISO 27001:2013 ID %q found in mapping seed — update to 2022 Annex A ID", id)
	}
}

// TestBISGMappingCoverage verifies that the NIS2 ↔ ISO 27001 seeder covers all
// ten §30 BISG requirements (= NIS2 Art. 21 §2 sub-clauses a–j) with at least
// one ISO 27001:2022 Annex A mapping. Source: BSI Onepager NIS-2 und ISO 27001.
func TestBISGMappingCoverage(t *testing.T) {
	// The NIS2 control prefixes that correspond to §30 Nr. 1–10.
	// Nr. 1→A, Nr. 2→B, Nr. 3→C, Nr. 4→D, Nr. 5→E, Nr. 7→G, Nr. 8→H,
	// Nr. 9 (assets→A.8, HR→A.6, access→F.1), Nr. 10 (MFA→F.1, comms→E.8).
	required := []string{
		"NIS2-A.1", // §30 Nr. 1: Risikoanalyse
		"NIS2-B.1", // §30 Nr. 2: Incident Response
		"NIS2-B.5", // §30 Nr. 2: 24h-Meldung
		"NIS2-C.1", // §30 Nr. 3: BCM Richtlinie
		"NIS2-C.4", // §30 Nr. 3: BCM Backup
		"NIS2-D.1", // §30 Nr. 4: Sichere Lieferkette
		"NIS2-E.3", // §30 Nr. 5: Schwachstellenmanagement
		"NIS2-E.4", // §30 Nr. 5: Patch-Management
		"NIS2-G.2", // §30 Nr. 7: Schulungen
		"NIS2-H.1", // §30 Nr. 8: Kryptografie
		"NIS2-F.1", // §30 Nr. 9+10: Zugriffskontrolle / MFA
		"NIS2-A.8", // §30 Nr. 9: Asset-Management
		"NIS2-A.6", // §30 Nr. 9: Personalsicherheit
		"NIS2-E.8", // §30 Nr. 10: Netzwerksicherheit
	}

	const iso, nis2 = "ISO27001", "NIS2"

	covered := make(map[string][]string) // nis2Code → []isoCode

	// Inline the same entries as SeedFrameworkMappings to verify completeness.
	entries := isoNIS2Entries()
	for _, e := range entries {
		if e[1] == nis2 {
			covered[e[3]] = append(covered[e[3]], e[0]) // [isoID, ..., nis2ID, ...]
		}
		if e[3] == nis2 {
			covered[e[1]] = append(covered[e[1]], e[0])
		}
	}
	_ = iso

	for _, nis2Code := range required {
		isos := covered[nis2Code]
		if len(isos) == 0 {
			t.Errorf("§30 BISG: no ISO 27001:2022 mapping found for %s", nis2Code)
		} else {
			for _, iso2022 := range isos {
				if iso2013Pattern.MatchString(iso2022) {
					t.Errorf("%s mapping %q is a legacy 2013 ISO ID", nis2Code, iso2022)
				}
			}
		}
	}
}

// isoNIS2Entries returns the [isoID, "ISO27001", "NIS2", nis2ID] tuples from
// SeedFrameworkMappings as a string-slice for test introspection without DB calls.
func isoNIS2Entries() [][4]string {
	type entry struct {
		srcFW, srcCode, tgtFW, tgtCode, mtype string
	}
	const iso, nis2 = "ISO27001", "NIS2"

	raw := []entry{
		{iso, "A.5.1", nis2, "NIS2-A.1", "equivalent"},
		{iso, "A.5.2", nis2, "NIS2-A.1", "partial"},
		{iso, "A.5.3", nis2, "NIS2-A.1", "partial"},
		{iso, "A.5.4", nis2, "NIS2-A.1", "partial"},
		{iso, "A.5.7", nis2, "NIS2-A.1", "partial"},
		{iso, "A.5.31", nis2, "NIS2-A.1", "partial"},
		{iso, "A.5.35", nis2, "NIS2-A.1", "partial"},
		{iso, "A.5.36", nis2, "NIS2-A.1", "equivalent"},
		{iso, "A.8.34", nis2, "NIS2-A.1", "partial"},

		{iso, "A.5.24", nis2, "NIS2-B.1", "equivalent"},
		{iso, "A.5.25", nis2, "NIS2-B.1", "partial"},
		{iso, "A.5.26", nis2, "NIS2-B.1", "partial"},
		{iso, "A.5.27", nis2, "NIS2-B.1", "partial"},
		{iso, "A.5.28", nis2, "NIS2-B.1", "partial"},
		{iso, "A.6.8", nis2, "NIS2-B.1", "partial"},
		{iso, "A.8.15", nis2, "NIS2-B.1", "partial"},
		{iso, "A.8.16", nis2, "NIS2-B.1", "partial"},
		{iso, "A.8.17", nis2, "NIS2-B.1", "partial"},
		{iso, "A.6.8", nis2, "NIS2-B.5", "equivalent"},
		{iso, "A.5.24", nis2, "NIS2-B.5", "partial"},

		{iso, "A.5.29", nis2, "NIS2-C.1", "equivalent"},
		{iso, "A.5.30", nis2, "NIS2-C.1", "equivalent"},
		{iso, "A.5.26", nis2, "NIS2-C.1", "partial"},
		{iso, "A.7.11", nis2, "NIS2-C.1", "partial"},
		{iso, "A.8.14", nis2, "NIS2-C.1", "partial"},
		{iso, "A.8.13", nis2, "NIS2-C.4", "equivalent"},

		{iso, "A.5.19", nis2, "NIS2-D.1", "equivalent"},
		{iso, "A.5.20", nis2, "NIS2-D.1", "equivalent"},
		{iso, "A.5.21", nis2, "NIS2-D.1", "partial"},
		{iso, "A.5.22", nis2, "NIS2-D.1", "partial"},
		{iso, "A.8.30", nis2, "NIS2-D.1", "partial"},

		{iso, "A.8.8", nis2, "NIS2-E.3", "equivalent"},
		{iso, "A.8.9", nis2, "NIS2-E.3", "partial"},
		{iso, "A.8.7", nis2, "NIS2-E.3", "partial"},
		{iso, "A.5.23", nis2, "NIS2-E.3", "partial"},
		{iso, "A.5.21", nis2, "NIS2-E.3", "partial"},
		{iso, "A.7.3", nis2, "NIS2-E.3", "partial"},
		{iso, "A.7.5", nis2, "NIS2-E.3", "partial"},
		{iso, "A.7.13", nis2, "NIS2-E.3", "partial"},
		{iso, "A.8.16", nis2, "NIS2-E.3", "partial"},
		{iso, "A.8.20", nis2, "NIS2-E.3", "partial"},
		{iso, "A.8.22", nis2, "NIS2-E.3", "partial"},
		{iso, "A.8.25", nis2, "NIS2-E.3", "partial"},
		{iso, "A.8.29", nis2, "NIS2-E.3", "partial"},
		{iso, "A.8.31", nis2, "NIS2-E.3", "partial"},
		{iso, "A.8.33", nis2, "NIS2-E.3", "partial"},
		{iso, "A.8.34", nis2, "NIS2-E.3", "partial"},
		{iso, "A.8.8", nis2, "NIS2-E.4", "equivalent"},
		{iso, "A.8.32", nis2, "NIS2-E.4", "partial"},

		{iso, "A.6.3", nis2, "NIS2-G.2", "equivalent"},
		{iso, "A.8.7", nis2, "NIS2-G.2", "partial"},

		{iso, "A.8.24", nis2, "NIS2-H.1", "equivalent"},
		{iso, "A.5.31", nis2, "NIS2-H.1", "partial"},

		{iso, "A.5.9", nis2, "NIS2-A.8", "equivalent"},
		{iso, "A.5.10", nis2, "NIS2-A.8", "partial"},
		{iso, "A.5.11", nis2, "NIS2-A.8", "partial"},
		{iso, "A.5.12", nis2, "NIS2-A.8", "partial"},
		{iso, "A.5.13", nis2, "NIS2-A.8", "partial"},
		{iso, "A.5.14", nis2, "NIS2-A.8", "partial"},
		{iso, "A.7.10", nis2, "NIS2-A.8", "partial"},
		{iso, "A.6.1", nis2, "NIS2-A.6", "equivalent"},
		{iso, "A.6.2", nis2, "NIS2-A.6", "partial"},
		{iso, "A.6.4", nis2, "NIS2-A.6", "partial"},
		{iso, "A.6.5", nis2, "NIS2-A.6", "partial"},
		{iso, "A.7.1", nis2, "NIS2-A.6", "partial"},
		{iso, "A.7.4", nis2, "NIS2-A.6", "partial"},
		{iso, "A.7.7", nis2, "NIS2-A.6", "partial"},
		{iso, "A.5.15", nis2, "NIS2-F.1", "equivalent"},
		{iso, "A.5.16", nis2, "NIS2-F.1", "equivalent"},
		{iso, "A.5.17", nis2, "NIS2-F.1", "partial"},
		{iso, "A.5.18", nis2, "NIS2-F.1", "partial"},
		{iso, "A.5.28", nis2, "NIS2-F.1", "partial"},
		{iso, "A.8.2", nis2, "NIS2-F.1", "partial"},
		{iso, "A.8.3", nis2, "NIS2-F.1", "partial"},
		{iso, "A.8.18", nis2, "NIS2-F.1", "partial"},
		{iso, "A.8.21", nis2, "NIS2-F.1", "partial"},
		{iso, "A.8.5", nis2, "NIS2-F.1", "equivalent"},
		{iso, "A.8.20", nis2, "NIS2-E.8", "equivalent"},
		{iso, "A.8.21", nis2, "NIS2-E.8", "equivalent"},
		{iso, "A.8.22", nis2, "NIS2-E.8", "partial"},
		{iso, "A.7.2", nis2, "NIS2-E.8", "partial"},

		// ENISA TIG v1.2 additions — Req. 6.x Secure Development → NIS2-E.1
		{iso, "A.8.26", nis2, "NIS2-E.1", "equivalent"},
		{iso, "A.8.27", nis2, "NIS2-E.1", "partial"},
		{iso, "A.8.28", nis2, "NIS2-E.1", "partial"},
		{iso, "A.5.8", nis2, "NIS2-E.1", "partial"},
		// ENISA TIG v1.2 — Req. 13.x Physical Security → NIS2-E.3
		{iso, "A.7.8", nis2, "NIS2-E.3", "partial"},
		{iso, "A.7.12", nis2, "NIS2-E.3", "partial"},
	}

	result := make([][4]string, len(raw))
	for i, e := range raw {
		result[i] = [4]string{e.srcCode, e.tgtFW, e.srcFW, e.tgtCode}
	}
	return result
}

// TestTISAXMappingsUse2022IDs verifies TISAX → ISO mapping uses only 2022 IDs.
func TestTISAXMappingsUse2022IDs(t *testing.T) {
	for tisax, iso := range tisaxToISO27001Mappings {
		if iso2013Pattern.MatchString(iso) {
			t.Errorf("tisaxToISO27001Mappings[%q] = %q contains legacy ISO 27001:2013 ID", tisax, iso)
		}
		// Verify 2022 range: A.5.x–A.8.x
		if !strings.HasPrefix(iso, "A.5.") && !strings.HasPrefix(iso, "A.6.") &&
			!strings.HasPrefix(iso, "A.7.") && !strings.HasPrefix(iso, "A.8.") {
			t.Errorf("tisaxToISO27001Mappings[%q] = %q is not in ISO 27001:2022 range A.5–A.8", tisax, iso)
		}
	}
}

// ── C5:2026 mapping invariants ────────────────────────────────────────────────

func TestC5ISO27001MappingsUseCodes(t *testing.T) {
	c5re := regexp.MustCompile(`^C5-[A-Z]+-\d+$`)
	for _, p := range c5ISO27001Mappings {
		if p.src == "C5" {
			if !c5re.MatchString(p.srcCode) {
				t.Errorf("c5ISO27001Mappings: invalid C5 code %q", p.srcCode)
			}
			checkNotLegacy(t, p.tgtCode)
		}
		if p.tgt == "C5" {
			if !c5re.MatchString(p.tgtCode) {
				t.Errorf("c5ISO27001Mappings: invalid C5 code %q", p.tgtCode)
			}
		}
	}
}

func TestC5ISO27001MappingsDomainCoverage(t *testing.T) {
	domains := map[string]bool{}
	domainRE := regexp.MustCompile(`^C5-([A-Z]+)-`)
	for _, p := range c5ISO27001Mappings {
		if m := domainRE.FindStringSubmatch(p.srcCode); m != nil {
			domains[m[1]] = true
		}
	}
	required := []string{"OIS", "SP", "HR", "AM", "PS", "OPS", "IAM", "CRY", "COS", "DEV", "SSO", "SIM", "BCM", "COM"}
	for _, d := range required {
		if !domains[d] {
			t.Errorf("c5ISO27001Mappings: domain %q has no entries", d)
		}
	}
}

func TestC5NIS2MappingsValid(t *testing.T) {
	nis2re := regexp.MustCompile(`^NIS2-[A-Z]\.\d+$`)
	for _, p := range c5NIS2Mappings {
		if p.src != "C5" || p.tgt != "NIS2" {
			t.Errorf("c5NIS2Mappings: unexpected direction %s→%s", p.src, p.tgt)
		}
		if !nis2re.MatchString(p.tgtCode) {
			t.Errorf("c5NIS2Mappings: invalid NIS2 code %q", p.tgtCode)
		}
	}
}

func TestC5NIS2MappingsCoversKeyAreas(t *testing.T) {
	covered := map[string]bool{}
	for _, p := range c5NIS2Mappings {
		covered[p.tgtCode] = true
	}
	required := []string{"NIS2-A.1", "NIS2-B.1", "NIS2-C.1", "NIS2-D.1", "NIS2-E.3", "NIS2-H.1", "NIS2-F.1"}
	for _, code := range required {
		if !covered[code] {
			t.Errorf("c5NIS2Mappings: no mapping for key NIS2 requirement %s", code)
		}
	}
}

// ── KRITIS-DachG mapping invariants ──────────────────────────────────────────

func TestKRITISISO27001MappingsUseCodes(t *testing.T) {
	kritisRE := regexp.MustCompile(`^KRITIS-DG\.\d+$`)
	for _, p := range kritisISO27001Mappings {
		if p.src == "KRITIS" {
			if !kritisRE.MatchString(p.srcCode) {
				t.Errorf("kritisISO27001Mappings: invalid KRITIS code %q", p.srcCode)
			}
			checkNotLegacy(t, p.tgtCode)
		}
	}
}

func TestKRITISISO27001MappingsCoreRequirementsCovered(t *testing.T) {
	covered := map[string]bool{}
	for _, p := range kritisISO27001Mappings {
		covered[p.srcCode] = true
	}
	// BGBl. 2026 I Nr. 66 core requirements
	required := []string{
		"KRITIS-DG.3",  // ISMS / Risikoanalyse
		"KRITIS-DG.4",  // Lieferkette
		"KRITIS-DG.6",  // Zugriffskontrolle
		"KRITIS-DG.11", // Kryptographie
		"KRITIS-DG.13", // Schwachstellenmanagement
		"KRITIS-DG.14", // Vorfallsmanagement
		"KRITIS-DG.15", // Meldepflichten
		"KRITIS-DG.16", // Schulungen
		"KRITIS-DG.18", // Resilienzplan
	}
	for _, code := range required {
		if !covered[code] {
			t.Errorf("kritisISO27001Mappings: missing required KRITIS requirement %s", code)
		}
	}
}

func TestKRITISNIS2MappingsValid(t *testing.T) {
	kritisRE := regexp.MustCompile(`^KRITIS-DG\.\d+$`)
	nis2re := regexp.MustCompile(`^NIS2-[A-Z]\.\d+$`)
	for _, p := range kritisNIS2Mappings {
		if !kritisRE.MatchString(p.srcCode) {
			t.Errorf("kritisNIS2Mappings: invalid KRITIS code %q", p.srcCode)
		}
		if !nis2re.MatchString(p.tgtCode) {
			t.Errorf("kritisNIS2Mappings: invalid NIS2 code %q", p.tgtCode)
		}
	}
}

// ── NIS2 ↔ DORA Simplified mapping invariants ────────────────────────────────

func TestNIS2DORASimplifiedMappingsValid(t *testing.T) {
	doraSimplRE := regexp.MustCompile(`^DORA-S\.\d+$`)
	nis2re := regexp.MustCompile(`^NIS2-[A-Z]\.\d+$`)
	for _, p := range nis2DORASimplifiedMappings {
		if p.src != "NIS2" || p.tgt != "DORA" {
			t.Errorf("nis2DORASimplifiedMappings: unexpected direction %s→%s", p.src, p.tgt)
		}
		if !nis2re.MatchString(p.srcCode) {
			t.Errorf("nis2DORASimplifiedMappings: invalid NIS2 code %q", p.srcCode)
		}
		if !doraSimplRE.MatchString(p.tgtCode) {
			t.Errorf("nis2DORASimplifiedMappings: invalid DORA-S code %q — expected DORA-S.N format", p.tgtCode)
		}
	}
}

func TestNIS2DORASimplifiedMappingsCoversAll15Controls(t *testing.T) {
	covered := map[string]bool{}
	for _, p := range nis2DORASimplifiedMappings {
		covered[p.tgtCode] = true
	}
	for i := 1; i <= 15; i++ {
		code := fmt.Sprintf("DORA-S.%d", i)
		if !covered[code] {
			t.Errorf("nis2DORASimplifiedMappings: DORA Art.16 simplified control %s has no NIS2 mapping", code)
		}
	}
}

// ── prEN 18286 mapping invariants ────────────────────────────────────────────

func TestPREN18286ISO42001MappingsValid(t *testing.T) {
	// Codes follow clause numbering (4.1, 10.1) or Annex refs (A.1, A4.2)
	prenRE := regexp.MustCompile(`^18286-[A-Z0-9]+\.\d+$`)
	iso42re := regexp.MustCompile(`^42001-[A-Z0-9]+\.\d+$`)
	for _, p := range prEN18286ISO42001Mappings {
		if p.src != "PREN18286" || p.tgt != "ISO42001" {
			t.Errorf("prEN18286ISO42001Mappings: unexpected direction %s→%s", p.src, p.tgt)
		}
		if !prenRE.MatchString(p.srcCode) {
			t.Errorf("prEN18286ISO42001Mappings: invalid prEN18286 code %q", p.srcCode)
		}
		if !iso42re.MatchString(p.tgtCode) {
			t.Errorf("prEN18286ISO42001Mappings: invalid ISO 42001 code %q", p.tgtCode)
		}
	}
}

func TestPREN18286MappingsNonEmpty(t *testing.T) {
	if len(prEN18286ISO42001Mappings) == 0 {
		t.Error("prEN18286ISO42001Mappings must not be empty")
	}
}

func TestPREN18286EUAIActMappingsValid(t *testing.T) {
	// prEN 18286 Annex ZA maps directly to EU AI Act Art. 9–18.
	// EU AI Act internal codes: AIACT-1.x (Art.9), AIACT-2.x (Art.10),
	// AIACT-3.x (Art.11), AIACT-4.1 (Art.12), AIACT-5.x (Art.13),
	// AIACT-6.x (Art.14), AIACT-9.x (Art.17), AIACT-10.x (Art.18).
	aiactRE := regexp.MustCompile(`^AIACT-\d+\.\d+$`)
	prenRE := regexp.MustCompile(`^18286-[A-Z0-9]+\.\d+$`)
	for _, p := range prEN18286EUAIActMappings {
		if p.src != "PREN18286" || p.tgt != "EUAIACT" {
			t.Errorf("prEN18286EUAIActMappings: unexpected direction %s→%s", p.src, p.tgt)
		}
		if !prenRE.MatchString(p.srcCode) {
			t.Errorf("prEN18286EUAIActMappings: invalid prEN18286 code %q", p.srcCode)
		}
		if !aiactRE.MatchString(p.tgtCode) {
			t.Errorf("prEN18286EUAIActMappings: invalid EU AI Act code %q", p.tgtCode)
		}
	}
}

func TestPREN18286EUAIActMappingsCoversQMS(t *testing.T) {
	// Art. 17 QMS (AIACT-9.x) is the primary purpose of prEN 18286.
	covered := map[string]bool{}
	for _, p := range prEN18286EUAIActMappings {
		covered[p.tgtCode] = true
	}
	if !covered["AIACT-9.1"] {
		t.Error("prEN18286EUAIActMappings: missing Art.17 QMS mapping to AIACT-9.1")
	}
}

// ── New seeder coverage tests ─────────────────────────────────────────────────

func TestKRITISDORAMappingsValid(t *testing.T) {
	kritisRE := regexp.MustCompile(`^KRITIS-DG\.\d+$`)
	doraRE := regexp.MustCompile(`^DORA-\d+\.\d+$`)
	for _, p := range kritisDORAMappings {
		if !kritisRE.MatchString(p.srcCode) {
			t.Errorf("kritisDORAMappings: invalid KRITIS code %q", p.srcCode)
		}
		if !doraRE.MatchString(p.tgtCode) {
			t.Errorf("kritisDORAMappings: invalid DORA code %q", p.tgtCode)
		}
	}
}

func TestBSIKRITISMappingsValid(t *testing.T) {
	bsiRE := regexp.MustCompile(`^BSI-[A-Z]+`)
	kritisRE := regexp.MustCompile(`^KRITIS-DG\.\d+$`)
	for _, p := range bsiKRITISMappings {
		if !bsiRE.MatchString(p.srcCode) {
			t.Errorf("bsiKRITISMappings: invalid BSI code %q", p.srcCode)
		}
		if !kritisRE.MatchString(p.tgtCode) {
			t.Errorf("bsiKRITISMappings: invalid KRITIS code %q", p.tgtCode)
		}
	}
}

func TestC5BSIMappingsValid(t *testing.T) {
	c5re := regexp.MustCompile(`^C5-[A-Z]+-\d+$`)
	bsiRE := regexp.MustCompile(`^BSI-[A-Z]+`)
	for _, p := range c5BSIMappings {
		if p.src == "C5" {
			if !c5re.MatchString(p.srcCode) {
				t.Errorf("c5BSIMappings: invalid C5 code %q", p.srcCode)
			}
			if !bsiRE.MatchString(p.tgtCode) {
				t.Errorf("c5BSIMappings: invalid BSI code %q", p.tgtCode)
			}
		}
	}
}

func TestTISAXNIS2MappingsValid(t *testing.T) {
	tisaxRE := regexp.MustCompile(`^TISAX-\d+\.\d+\.\d+$`)
	nis2RE := regexp.MustCompile(`^NIS2-[A-Z]\.\d+$`)
	for _, p := range tisaxNIS2Mappings {
		if !tisaxRE.MatchString(p.srcCode) {
			t.Errorf("tisaxNIS2Mappings: invalid TISAX code %q", p.srcCode)
		}
		if !nis2RE.MatchString(p.tgtCode) {
			t.Errorf("tisaxNIS2Mappings: invalid NIS2 code %q", p.tgtCode)
		}
	}
}

func TestTISAXNIS2MappingsCoversArt21(t *testing.T) {
	covered := map[string]bool{}
	for _, p := range tisaxNIS2Mappings {
		covered[p.tgtCode] = true
	}
	// NIS2 Art. 21 core areas that TISAX covers per ENX expert opinion
	required := []string{"NIS2-A.1", "NIS2-B.1", "NIS2-C.1", "NIS2-F.1", "NIS2-G.2", "NIS2-H.1"}
	for _, code := range required {
		if !covered[code] {
			t.Errorf("tisaxNIS2Mappings: missing NIS2 Art.21 coverage for %s", code)
		}
	}
}

// ── New mapping pair tests (Sprint 72 additions) ──────────────────────────────

func TestISO27017C5MappingsValid(t *testing.T) {
	isoRE := regexp.MustCompile(`^27017-`)
	c5RE := regexp.MustCompile(`^C5-[A-Z]+-\d+$`)
	for _, p := range iso27017C5Mappings {
		if !isoRE.MatchString(p.srcCode) {
			t.Errorf("iso27017C5Mappings: invalid ISO27017 srcCode %q", p.srcCode)
		}
		if !c5RE.MatchString(p.tgtCode) {
			t.Errorf("iso27017C5Mappings: invalid C5 tgtCode %q", p.tgtCode)
		}
	}
}

func TestISO27017C5MappingsNonEmpty(t *testing.T) {
	if len(iso27017C5Mappings) < 20 {
		t.Errorf("iso27017C5Mappings: expected ≥20 entries, got %d", len(iso27017C5Mappings))
	}
}

func TestISO27017C5MappingsCoversKeyDomains(t *testing.T) {
	domains := map[string]bool{}
	for _, p := range iso27017C5Mappings {
		// extract domain prefix: C5-OIS-01 → OIS
		parts := strings.SplitN(p.tgtCode, "-", 3)
		if len(parts) == 3 {
			domains[parts[1]] = true
		}
	}
	for _, d := range []string{"IAM", "CRY", "OPS", "COS", "SSO"} {
		if !domains[d] {
			t.Errorf("iso27017C5Mappings: missing C5 domain %s", d)
		}
	}
}

func TestISO27018DsgvoTOMMappingsValid(t *testing.T) {
	isoRE := regexp.MustCompile(`^27018-A\.\d+\.\d+$`)
	tomRE := regexp.MustCompile(`^TOM-\d+$`)
	for _, p := range iso27018DsgvoTOMMappings {
		if !isoRE.MatchString(p.srcCode) {
			t.Errorf("iso27018DsgvoTOMMappings: invalid ISO27018 srcCode %q", p.srcCode)
		}
		if !tomRE.MatchString(p.tgtCode) {
			t.Errorf("iso27018DsgvoTOMMappings: invalid DSGVO-TOM tgtCode %q", p.tgtCode)
		}
	}
}

func TestISO27018DsgvoTOMMappingsCoversAllTOMs(t *testing.T) {
	covered := map[string]bool{}
	for _, p := range iso27018DsgvoTOMMappings {
		covered[p.tgtCode] = true
	}
	// All 8 classic DSGVO §64 TOMs must be covered
	for _, tom := range []string{"TOM-2", "TOM-3", "TOM-4", "TOM-5", "TOM-6", "TOM-8", "TOM-10", "TOM-13"} {
		if !covered[tom] {
			t.Errorf("iso27018DsgvoTOMMappings: missing coverage for %s", tom)
		}
	}
}

func TestDsgvoTOMISO27001MappingsValid(t *testing.T) {
	tomRE := regexp.MustCompile(`^TOM-\d+$`)
	isoRE := regexp.MustCompile(`^A\.[5-8]\.\d+$`)
	for _, p := range dsgvoTOMISO27001Mappings {
		if !tomRE.MatchString(p.srcCode) {
			t.Errorf("dsgvoTOMISO27001Mappings: invalid TOM srcCode %q", p.srcCode)
		}
		if !isoRE.MatchString(p.tgtCode) {
			t.Errorf("dsgvoTOMISO27001Mappings: invalid ISO27001 tgtCode %q", p.tgtCode)
		}
	}
}

func TestDsgvoTOMISO27001MappingsCoversAll13TOMs(t *testing.T) {
	covered := map[string]bool{}
	for _, p := range dsgvoTOMISO27001Mappings {
		covered[p.srcCode] = true
	}
	for i := 1; i <= 13; i++ {
		tom := fmt.Sprintf("TOM-%d", i)
		if !covered[tom] {
			t.Errorf("dsgvoTOMISO27001Mappings: TOM %s has no ISO27001 mapping", tom)
		}
	}
}

func TestTISAXISO27001MappingsValid(t *testing.T) {
	tisaxRE := regexp.MustCompile(`^TISAX-\d+\.\d+\.\d+$`)
	isoRE := regexp.MustCompile(`^A\.[5-8]\.\d+$`)
	for _, p := range tisaxISO27001Mappings {
		if !tisaxRE.MatchString(p.srcCode) {
			t.Errorf("tisaxISO27001Mappings: invalid TISAX code %q", p.srcCode)
		}
		if !isoRE.MatchString(p.tgtCode) {
			t.Errorf("tisaxISO27001Mappings: invalid ISO27001 code %q (must be A.5-8.x)", p.tgtCode)
		}
	}
}

func TestTISAXISO27001MappingsCoversKeyClauses(t *testing.T) {
	covered := map[string]bool{}
	for _, p := range tisaxISO27001Mappings {
		covered[p.srcCode] = true
	}
	// VDA ISA 6.0 core clauses must all have ISO27001 mappings
	required := []string{
		"TISAX-1.1.1", "TISAX-5.1.1", "TISAX-6.1.1", "TISAX-8.1.6",
		"TISAX-12.1.1", "TISAX-13.1.1", "TISAX-14.1.1",
	}
	for _, code := range required {
		if !covered[code] {
			t.Errorf("tisaxISO27001Mappings: missing mapping for %s", code)
		}
	}
}

func TestBSIDORAMappingsValid(t *testing.T) {
	bsiRE := regexp.MustCompile(`^BSI-[A-Z]`)
	doraRE := regexp.MustCompile(`^DORA-\d+\.\d+$`)
	for _, p := range bsiDORAMappings {
		if !bsiRE.MatchString(p.srcCode) {
			t.Errorf("bsiDORAMappings: invalid BSI code %q", p.srcCode)
		}
		if !doraRE.MatchString(p.tgtCode) {
			t.Errorf("bsiDORAMappings: invalid DORA code %q", p.tgtCode)
		}
	}
}

func TestBSIDORAMappingsCoversAllDORAPillars(t *testing.T) {
	covered := map[string]bool{}
	for _, p := range bsiDORAMappings {
		// extract pillar prefix: DORA-1.1 → "1"
		parts := strings.SplitN(p.tgtCode, "-", 2)
		if len(parts) == 2 {
			pillar := strings.SplitN(parts[1], ".", 2)[0]
			covered[pillar] = true
		}
	}
	for _, pillar := range []string{"1", "2", "3", "4"} {
		if !covered[pillar] {
			t.Errorf("bsiDORAMappings: DORA pillar %s not covered", pillar)
		}
	}
}

func TestISO27017BSIMappingsValid(t *testing.T) {
	isoRE := regexp.MustCompile(`^27017-`)
	bsiRE := regexp.MustCompile(`^BSI-[A-Z]`)
	for _, p := range iso27017BSIMappings {
		if !isoRE.MatchString(p.srcCode) {
			t.Errorf("iso27017BSIMappings: invalid ISO27017 code %q", p.srcCode)
		}
		if !bsiRE.MatchString(p.tgtCode) {
			t.Errorf("iso27017BSIMappings: invalid BSI code %q", p.tgtCode)
		}
	}
}

func TestISO27017BSIMappingsCoversCloudBausteine(t *testing.T) {
	covered := map[string]bool{}
	for _, p := range iso27017BSIMappings {
		covered[p.tgtCode] = true
	}
	// Cloud-relevant BSI Bausteine must be covered
	for _, code := range []string{"BSI-OPS.2.3", "BSI-CON.1", "BSI-NET.1.1", "BSI-SYS.1.6"} {
		if !covered[code] {
			t.Errorf("iso27017BSIMappings: missing cloud Baustein %s", code)
		}
	}
}

func TestISO27018C5MappingsValid(t *testing.T) {
	isoRE := regexp.MustCompile(`^27018-A\.\d+\.\d+$`)
	c5RE := regexp.MustCompile(`^C5-[A-Z]+-\d+$`)
	for _, p := range iso27018C5Mappings {
		if !isoRE.MatchString(p.srcCode) {
			t.Errorf("iso27018C5Mappings: invalid ISO27018 code %q", p.srcCode)
		}
		if !c5RE.MatchString(p.tgtCode) {
			t.Errorf("iso27018C5Mappings: invalid C5 code %q", p.tgtCode)
		}
	}
}

func TestISO27018C5MappingsCoversDataProtectionDomains(t *testing.T) {
	domains := map[string]bool{}
	for _, p := range iso27018C5Mappings {
		parts := strings.SplitN(p.tgtCode, "-", 3)
		if len(parts) == 3 {
			domains[parts[1]] = true
		}
	}
	for _, d := range []string{"COM", "SSO", "CRY", "IAM", "INQ"} {
		if !domains[d] {
			t.Errorf("iso27018C5Mappings: missing C5 domain %s", d)
		}
	}
}

func TestISO27017DORAMappingsValid(t *testing.T) {
	isoRE := regexp.MustCompile(`^27017-`)
	doraRE := regexp.MustCompile(`^DORA-\d+\.\d+$`)
	for _, p := range iso27017DORAMappings {
		if !isoRE.MatchString(p.srcCode) {
			t.Errorf("iso27017DORAMappings: invalid ISO27017 code %q", p.srcCode)
		}
		if !doraRE.MatchString(p.tgtCode) {
			t.Errorf("iso27017DORAMappings: invalid DORA code %q", p.tgtCode)
		}
	}
}

func TestISO27017DORAMappingsCoversTPICT(t *testing.T) {
	covered := map[string]bool{}
	for _, p := range iso27017DORAMappings {
		covered[p.tgtCode] = true
	}
	// DORA Art. 28 TPICT must be covered (4.1, 4.2)
	for _, code := range []string{"DORA-4.1", "DORA-4.2", "DORA-1.4", "DORA-1.5"} {
		if !covered[code] {
			t.Errorf("iso27017DORAMappings: missing DORA TPICT coverage for %s", code)
		}
	}
}

func TestISO27017KRITISMappingsValid(t *testing.T) {
	isoRE := regexp.MustCompile(`^27017-`)
	kritisRE := regexp.MustCompile(`^KRITIS-DG\.\d+$`)
	for _, p := range iso27017KRITISMappings {
		if !isoRE.MatchString(p.srcCode) {
			t.Errorf("iso27017KRITISMappings: invalid ISO27017 code %q", p.srcCode)
		}
		if !kritisRE.MatchString(p.tgtCode) {
			t.Errorf("iso27017KRITISMappings: invalid KRITIS code %q", p.tgtCode)
		}
	}
}

func TestISO27017KRITISMappingsCoversCloudParagraphs(t *testing.T) {
	covered := map[string]bool{}
	for _, p := range iso27017KRITISMappings {
		covered[p.tgtCode] = true
	}
	// KRITIS-DachG §12-14 relevant requirements for cloud
	for _, code := range []string{"KRITIS-DG.3", "KRITIS-DG.4", "KRITIS-DG.9", "KRITIS-DG.11", "KRITIS-DG.12"} {
		if !covered[code] {
			t.Errorf("iso27017KRITISMappings: missing KRITIS cloud coverage for %s", code)
		}
	}
}

func TestISO42001NIS2MappingsValid(t *testing.T) {
	iso42001RE := regexp.MustCompile(`^42001-`)
	nis2RE := regexp.MustCompile(`^NIS2-[A-Z]\.\d+$`)
	for _, p := range iso42001NIS2Mappings {
		if !iso42001RE.MatchString(p.srcCode) {
			t.Errorf("iso42001NIS2Mappings: invalid ISO42001 code %q", p.srcCode)
		}
		if !nis2RE.MatchString(p.tgtCode) {
			t.Errorf("iso42001NIS2Mappings: invalid NIS2 code %q", p.tgtCode)
		}
	}
}

func TestISO42001NIS2MappingsCoversAIRiskAndGovernance(t *testing.T) {
	covered := map[string]bool{}
	for _, p := range iso42001NIS2Mappings {
		covered[p.tgtCode] = true
	}
	for _, code := range []string{"NIS2-A.1", "NIS2-A.3", "NIS2-G.2", "NIS2-E.3"} {
		if !covered[code] {
			t.Errorf("iso42001NIS2Mappings: missing NIS2 AI coverage for %s", code)
		}
	}
}

func TestCRAC5MappingsValid(t *testing.T) {
	craRE := regexp.MustCompile(`^CRA-\d+\.\d+$`)
	c5RE := regexp.MustCompile(`^C5-[A-Z]+-\d+$`)
	for _, p := range craC5Mappings {
		if !craRE.MatchString(p.srcCode) {
			t.Errorf("craC5Mappings: invalid CRA code %q", p.srcCode)
		}
		if !c5RE.MatchString(p.tgtCode) {
			t.Errorf("craC5Mappings: invalid C5 code %q", p.tgtCode)
		}
	}
}

func TestCRAC5MappingsCoversProductSecurityDomains(t *testing.T) {
	domains := map[string]bool{}
	for _, p := range craC5Mappings {
		parts := strings.SplitN(p.tgtCode, "-", 3)
		if len(parts) == 3 {
			domains[parts[1]] = true
		}
	}
	for _, d := range []string{"PSS", "DEV", "OPS", "CRY"} {
		if !domains[d] {
			t.Errorf("craC5Mappings: missing product security C5 domain %s", d)
		}
	}
}

func TestCRABSIMappingsValid(t *testing.T) {
	craRE := regexp.MustCompile(`^CRA-\d+\.\d+$`)
	bsiRE := regexp.MustCompile(`^BSI-[A-Z]`)
	for _, p := range craBSIMappings {
		if !craRE.MatchString(p.srcCode) {
			t.Errorf("craBSIMappings: invalid CRA code %q", p.srcCode)
		}
		if !bsiRE.MatchString(p.tgtCode) {
			t.Errorf("craBSIMappings: invalid BSI code %q", p.tgtCode)
		}
	}
}

func TestCRABSIMappingsCoversSecureDev(t *testing.T) {
	covered := map[string]bool{}
	for _, p := range craBSIMappings {
		covered[p.tgtCode] = true
	}
	// BSI TR-03183 Module H core Bausteine
	for _, code := range []string{"BSI-CON.8", "BSI-OPS.1.1.2", "BSI-CON.1", "BSI-DER.3.2"} {
		if !covered[code] {
			t.Errorf("craBSIMappings: missing BSI TR-03183 Baustein %s", code)
		}
	}
}

func TestTISAXC5MappingsValid(t *testing.T) {
	tisaxRE := regexp.MustCompile(`^TISAX-\d+\.\d+\.\d+$`)
	c5RE := regexp.MustCompile(`^C5-[A-Z]+-\d+$`)
	for _, p := range tisaxC5Mappings {
		if !tisaxRE.MatchString(p.srcCode) {
			t.Errorf("tisaxC5Mappings: invalid TISAX code %q", p.srcCode)
		}
		if !c5RE.MatchString(p.tgtCode) {
			t.Errorf("tisaxC5Mappings: invalid C5 code %q", p.tgtCode)
		}
	}
}

func TestTISAXC5MappingsCoversCoreDomains(t *testing.T) {
	domains := map[string]bool{}
	for _, p := range tisaxC5Mappings {
		parts := strings.SplitN(p.tgtCode, "-", 3)
		if len(parts) == 3 {
			domains[parts[1]] = true
		}
	}
	for _, d := range []string{"OIS", "IAM", "CRY", "OPS", "SIM"} {
		if !domains[d] {
			t.Errorf("tisaxC5Mappings: missing C5 domain %s", d)
		}
	}
}

func TestEUAIActNIS2MappingsValid(t *testing.T) {
	aiactRE := regexp.MustCompile(`^AIACT-\d+\.\d+$`)
	nis2RE := regexp.MustCompile(`^NIS2-[A-Z]\.\d+$`)
	for _, p := range euAIActNIS2Mappings {
		if !aiactRE.MatchString(p.srcCode) {
			t.Errorf("euAIActNIS2Mappings: invalid AIACT code %q", p.srcCode)
		}
		if !nis2RE.MatchString(p.tgtCode) {
			t.Errorf("euAIActNIS2Mappings: invalid NIS2 code %q", p.tgtCode)
		}
	}
}

func TestEUAIActNIS2MappingsCoversRiskAndGovernance(t *testing.T) {
	covered := map[string]bool{}
	for _, p := range euAIActNIS2Mappings {
		covered[p.tgtCode] = true
	}
	// ENISA TIG AI Act core areas in NIS2
	for _, code := range []string{"NIS2-A.1", "NIS2-A.3", "NIS2-E.3", "NIS2-F.3"} {
		if !covered[code] {
			t.Errorf("euAIActNIS2Mappings: missing AI Act NIS2 coverage for %s", code)
		}
	}
}
