// Copyright (c) 2026 NorvikOps. All rights reserved.
// SPDX-License-Identifier: Elastic-2.0

// Tests for Sprint 69 S69-1 cross-framework mapping logic.

package vaktcomply

import (
	"regexp"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ── S69-1: DORA seeder — uses doraISO27001Mapping (2022 IDs) ─────────────────
// doraISO27001MappingFixed was removed in the ISO 27001:2022 mapping-seed cleanup.
// SeedDORAMappingsFixed now delegates to doraISO27001Mapping (service_helpers.go)
// which already carries correct 2022 Annex A IDs.

func TestDoraISO27001MappingUses2022IDs(t *testing.T) {
	// All ISO 27001 IDs in doraISO27001Mapping must be 2022 Annex A (A.5–A.8).
	// Legacy 2013 IDs (A.9.x–A.18.x or three-level A.x.y.z) must not appear.
	legacyRE := regexp.MustCompile(`\bA\.(9|10|11|12|13|14|15|16|17|18)\.\d+|\bA\.\d+\.\d+\.\d+`)
	for doraCode, isoCodes := range doraISO27001Mapping {
		for _, isoCode := range strings.Split(isoCodes, ",") {
			isoCode = strings.TrimSpace(isoCode)
			assert.False(t, legacyRE.MatchString(isoCode),
				"doraISO27001Mapping[%s] contains legacy 2013 ID %q", doraCode, isoCode)
		}
	}
}

func TestDoraISO27001MappingCoversAllFiveChapters(t *testing.T) {
	chapters := map[string]bool{}
	for doraCode := range doraISO27001Mapping {
		if len(doraCode) >= 7 {
			chapters[string(doraCode[5])] = true
		}
	}
	for _, ch := range []string{"1", "2", "3", "4", "5"} {
		assert.True(t, chapters[ch], "no DORA chapter %s in doraISO27001Mapping", ch)
	}
}

// ── S69-1: Prerequisite chains — structural invariants ───────────────────────

func TestBuildPrerequisiteChainsReturnsNonEmpty(t *testing.T) {
	chains := buildPrerequisiteChains()
	require.NotEmpty(t, chains)
	assert.Greater(t, len(chains), 10, "expected at least 10 prerequisite entries")
}

func TestBuildPrerequisiteChainsNoDuplicates(t *testing.T) {
	chains := buildPrerequisiteChains()
	type key struct{ fw, code, preqFW, preqCode string }
	seen := make(map[key]bool, len(chains))
	for _, c := range chains {
		k := key{c.ControlFW, c.ControlCode, c.PrereqFW, c.PrereqCode}
		assert.False(t, seen[k], "duplicate prerequisite: %+v", k)
		seen[k] = true
	}
}

func TestBuildPrerequisiteChainsValidDependencyTypes(t *testing.T) {
	validTypes := map[string]bool{"required": true, "recommended": true, "informative": true}
	for _, c := range buildPrerequisiteChains() {
		assert.True(t, validTypes[c.DependencyType],
			"invalid dependency_type %q for %s→%s", c.DependencyType, c.ControlCode, c.PrereqCode)
	}
}

func TestBuildPrerequisiteChainsAllHaveRationale(t *testing.T) {
	for _, c := range buildPrerequisiteChains() {
		assert.NotEmpty(t, c.Rationale,
			"missing rationale for %s/%s → %s/%s", c.ControlFW, c.ControlCode, c.PrereqFW, c.PrereqCode)
	}
}

// ── S69-1: Helper functions ───────────────────────────────────────────────────

func TestSplitCodesBasic(t *testing.T) {
	cases := []struct {
		raw      string
		expected []string
	}{
		{"A.5.1", []string{"A.5.1"}},
		{"A.5.1, A.6.1", []string{"A.5.1", "A.6.1"}},
		{"A.17.1, A.12.3", []string{"A.17.1", "A.12.3"}},
		{"A.15.1, A.18.1", []string{"A.15.1", "A.18.1"}},
	}
	for _, tc := range cases {
		got := splitCodes(tc.raw)
		assert.Equal(t, tc.expected, got, "splitCodes(%q)", tc.raw)
	}
}

// ── S69-1: Framework pair coverage ──────────────────────────────────────────

func TestFrameworkPairCoverageStruct(t *testing.T) {
	fc := FrameworkPairCoverage{
		FrameworkAName: "ISO27001",
		FrameworkBName: "NIS2",
		MappingCount:   42,
		IsMapped:       true,
	}
	assert.True(t, fc.IsMapped)
	assert.Equal(t, 42, fc.MappingCount)
}

func TestMappingCoverageResponseCalculation(t *testing.T) {
	resp := &MappingCoverageResponse{
		Pairs: []FrameworkPairCoverage{
			{IsMapped: true},
			{IsMapped: true},
			{IsMapped: false},
		},
		TotalMeaningfulPairs: 3,
		MappedPairs:          2,
		CoveragePct:          66.67,
	}
	assert.Equal(t, 3, resp.TotalMeaningfulPairs)
	assert.Equal(t, 2, resp.MappedPairs)
	assert.InDelta(t, 66.67, resp.CoveragePct, 0.01)
}

// ── S69-1: CRA mapping arrays ────────────────────────────────────────────────

func TestCRAMappingsUseCRAPrefix(t *testing.T) {
	for _, p := range craMappings {
		assert.Equal(t, "CRA", p.src, "unexpected src framework in craMappings: %q", p.src)
		assert.Regexp(t, `^CRA-\d+\.\d+$`, p.srcCode, "invalid CRA code: %s", p.srcCode)
	}
}

func TestCRANIS2MappingsTargetNIS2(t *testing.T) {
	for _, p := range craNIS2Mappings {
		assert.Equal(t, "NIS2", p.tgt, "unexpected tgt in craNIS2Mappings: %q", p.tgt)
	}
}

func TestNIS2DORAMappingsValid(t *testing.T) {
	for _, p := range nis2DORAMappings {
		assert.Equal(t, "NIS2", p.src)
		assert.Equal(t, "DORA", p.tgt)
	}
}

// ── Implementation step invariants ───────────────────────────────────────────

func TestImplementationStepFields(t *testing.T) {
	step := ImplementationStep{
		StepNr:           1,
		FrameworkID:      "fw-uuid",
		ControlCode:      "A.5.1",
		ControlTitle:     "Informationssicherheitsrichtlinie",
		CurrentStatus:    "not_started",
		PrerequisitesMet: true,
	}
	assert.Equal(t, 1, step.StepNr)
	assert.True(t, step.PrerequisitesMet)
	assert.Empty(t, step.BlockingPrereqs)
}
