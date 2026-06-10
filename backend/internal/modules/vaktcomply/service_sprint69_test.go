// Copyright (c) 2026 NorvikOps. All rights reserved.
// SPDX-License-Identifier: Elastic-2.0

// Tests for Sprint 69 S69-1 cross-framework mapping logic.

package vaktcomply

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ── S69-1: DORA hotfix — control ID correctness ──────────────────────────────

func TestDoraISO27001MappingFixedUsesCorrectControlIDs(t *testing.T) {
	// All values in doraISO27001MappingFixed must use 2013-style IDs (A.X.Y format).
	// 2022-style IDs like "A.5.30" or "A.8.6" must not appear.
	badPatterns := []string{"A.5.30", "A.5.31", "A.5.34", "A.8.6", "A.8.9", "A.8.16"}
	for doraCode, isoCodes := range doraISO27001MappingFixed {
		for _, bad := range badPatterns {
			assert.NotContains(t, isoCodes, bad,
				"DORA mapping %s references non-existent 2022-only code %s", doraCode, bad)
		}
	}
}

func TestDoraISO27001MappingFixedAllDoraCodesValid(t *testing.T) {
	// DORA codes must follow DORA-X.Y pattern.
	for doraCode := range doraISO27001MappingFixed {
		assert.Regexp(t, `^DORA-\d+\.\d+$`, doraCode,
			"invalid DORA control code: %s", doraCode)
	}
}

func TestDoraISO27001MappingFixedCoversAllFiveChapters(t *testing.T) {
	chapters := map[string]bool{}
	for doraCode := range doraISO27001MappingFixed {
		if len(doraCode) >= 7 {
			chapters[string(doraCode[5])] = true // "DORA-X.Y" → index 5 is X
		}
	}
	for _, ch := range []string{"1", "2", "3", "4", "5"} {
		assert.True(t, chapters[ch], "no DORA chapter %s in fixed mapping", ch)
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
