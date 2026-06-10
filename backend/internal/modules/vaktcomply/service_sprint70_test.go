// Copyright (c) 2026 NorvikOps. All rights reserved.
// SPDX-License-Identifier: Elastic-2.0

// Tests for Sprint 70 S70-1 (ISO 27001:2022) and S70-2 (NIS2 enrichment).

package vaktcomply

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ── S70-1: ISO 27001:2022 Control Library ────────────────────────────────────

func TestISO27001Controls_Count(t *testing.T) {
	controls := iso27001Controls("fw-1", "org-1")
	assert.Equal(t, 93, len(controls), "ISO 27001:2022 must have exactly 93 Annex A controls")
}

func TestISO27001Controls_GroupDistribution(t *testing.T) {
	controls := iso27001Controls("fw-1", "org-1")
	byGroup := map[string]int{}
	for _, c := range controls {
		prefix := strings.Split(c.ControlID, ".")[0] // "A"
		if len(c.ControlID) >= 3 {
			prefix = c.ControlID[:3] // "A.5", "A.6", "A.7", "A.8"
		}
		byGroup[prefix]++
	}
	assert.Equal(t, 37, byGroup["A.5"], "A.5 Organisational must have 37 controls")
	assert.Equal(t, 8, byGroup["A.6"], "A.6 People must have 8 controls")
	assert.Equal(t, 14, byGroup["A.7"], "A.7 Physical must have 14 controls")
	assert.Equal(t, 34, byGroup["A.8"], "A.8 Technological must have 34 controls")
}

func TestISO27001Controls_NoLegacyCodes(t *testing.T) {
	controls := iso27001Controls("fw-1", "org-1")
	for _, c := range controls {
		assert.False(t, strings.HasPrefix(c.ControlID, "A2022-"),
			"control %s must not use deprecated A2022- prefix", c.ControlID)
		// No 2013-style groups (A.9–A.18)
		for _, oldGroup := range []string{"A.9.", "A.10.", "A.11.", "A.12.", "A.13.", "A.14.", "A.15.", "A.16.", "A.17.", "A.18."} {
			assert.False(t, strings.HasPrefix(c.ControlID, oldGroup),
				"control %s must not use 2013-era group %s", c.ControlID, oldGroup)
		}
	}
}

func TestISO27001Controls_UniqueIDs(t *testing.T) {
	controls := iso27001Controls("fw-1", "org-1")
	seen := map[string]bool{}
	for _, c := range controls {
		require.False(t, seen[c.ControlID], "duplicate control ID: %s", c.ControlID)
		seen[c.ControlID] = true
	}
}

func TestISO27001Controls_AllHaveTitle(t *testing.T) {
	controls := iso27001Controls("fw-1", "org-1")
	for _, c := range controls {
		assert.NotEmpty(t, c.Title, "control %s must have a title", c.ControlID)
	}
}

// ── S70-2: NIS2 Implementing Regulation enrichment ───────────────────────────

func TestNIS2Controls_AllHaveThematicArea(t *testing.T) {
	controls := nis2Controls("fw-2", "org-1")
	for _, c := range controls {
		if _, ok := nis2ControlMeta[c.ControlID]; ok {
			assert.NotEmpty(t, c.ThematicArea,
				"NIS2 control %s must have a thematic_area when meta exists", c.ControlID)
		}
	}
}

func TestNIS2Controls_MetaLookupComplete(t *testing.T) {
	controls := nis2Controls("fw-2", "org-1")
	unmapped := 0
	for _, c := range controls {
		if c.ThematicArea == "" {
			unmapped++
		}
	}
	// Allow at most 5% without meta (older controls pre-dating the regulation)
	threshold := len(controls) / 20
	assert.LessOrEqual(t, unmapped, threshold,
		"too many NIS2 controls missing thematic_area: %d/%d", unmapped, len(controls))
}

func TestNIS2Controls_ScopeFilterAll(t *testing.T) {
	controls := nis2Controls("fw-2", "org-1")
	for _, c := range controls {
		if _, ok := nis2ControlMeta[c.ControlID]; ok {
			assert.NotEmpty(t, c.ApplicabilityScope,
				"NIS2 control %s with meta must have applicability_scope", c.ControlID)
		}
	}
}

func TestFilterControlsByScope_All(t *testing.T) {
	controls := []Control{
		{ControlID: "NIS2-1", ApplicabilityScope: []string{"all"}},
		{ControlID: "NIS2-2", ApplicabilityScope: []string{"msp", "cloud"}},
		{ControlID: "NIS2-3", ApplicabilityScope: []string{"dns"}},
	}
	result := filterControlsByScope(controls, "msp")
	// "all" and "msp" match; "dns" does not
	assert.Equal(t, 2, len(result))
	assert.Equal(t, "NIS2-1", result[0].ControlID)
	assert.Equal(t, "NIS2-2", result[1].ControlID)
}

func TestFilterControlsByScope_NoMatch(t *testing.T) {
	controls := []Control{
		{ControlID: "NIS2-1", ApplicabilityScope: []string{"dns"}},
		{ControlID: "NIS2-2", ApplicabilityScope: []string{"cloud"}},
	}
	result := filterControlsByScope(controls, "msp")
	assert.Empty(t, result)
}

func TestFilterControlsByScope_AllScopeAlwaysIncluded(t *testing.T) {
	controls := []Control{
		{ControlID: "NIS2-1", ApplicabilityScope: []string{"all"}},
		{ControlID: "NIS2-2", ApplicabilityScope: []string{"all"}},
	}
	for _, scope := range []string{"msp", "cloud", "dns", "anyone"} {
		result := filterControlsByScope(controls, scope)
		assert.Equal(t, 2, len(result), "scope=%s: 'all' controls must always pass filter", scope)
	}
}
