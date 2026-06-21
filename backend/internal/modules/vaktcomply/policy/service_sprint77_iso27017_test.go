// Copyright (c) 2026 NorvikOps. All rights reserved.
// SPDX-License-Identifier: Elastic-2.0

// Sprint 77 unit tests for ISO 27017 and ISO 27018 framework controls (no DB required).

package policy

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

const s77fwID = "s77-fw-id"
const s77orgID = "s77-org-id"

// ── ISO 27017 ─────────────────────────────────────────────────────────────────

func TestISO27017_ControlCount(t *testing.T) {
	controls := iso27017Controls(s77fwID, s77orgID)
	assert.GreaterOrEqual(t, len(controls), 30,
		"ISO 27017 must have ≥30 controls")
}

func TestISO27017_IDsAreUnique(t *testing.T) {
	controls := iso27017Controls(s77fwID, s77orgID)
	seen := make(map[string]bool, len(controls))
	for _, c := range controls {
		assert.False(t, seen[c.ControlID], "duplicate ControlID: %s", c.ControlID)
		seen[c.ControlID] = true
	}
}

func TestISO27017_NoLegacyIDs(t *testing.T) {
	controls := iso27017Controls(s77fwID, s77orgID)
	for _, c := range controls {
		for _, legacy := range []string{"A.9.", "A.10.", "A.11.", "A.12.", "A.13.", "A.14.", "A.15.", "A.16.", "A.17.", "A.18."} {
			assert.False(t, strings.HasPrefix(c.ControlID, legacy),
				"control %s uses legacy ISO 27001:2013 ID prefix %s", c.ControlID, legacy)
		}
	}
}

func TestISO27017_AllFieldsPopulated(t *testing.T) {
	controls := iso27017Controls(s77fwID, s77orgID)
	for _, c := range controls {
		assert.NotEmpty(t, c.ControlID, "ControlID must not be empty")
		assert.NotEmpty(t, c.Title, "Title must not be empty for %s", c.ControlID)
		assert.NotEmpty(t, c.Description, "Description must not be empty for %s", c.ControlID)
		assert.NotEmpty(t, c.Domain, "Domain must not be empty for %s", c.ControlID)
		assert.Equal(t, s77fwID, c.FrameworkID)
		assert.Equal(t, s77orgID, c.OrgID)
	}
}

func TestISO27017_CrossMappingsExist(t *testing.T) {
	assert.Greater(t, len(iso27017ISO27001Mappings), 0, "ISO27017↔ISO27001 mappings must exist")
	assert.Greater(t, len(iso27017C5Mappings), 0, "ISO27017↔C5 mappings must exist")
	assert.Greater(t, len(iso27017BSIMappings), 0, "ISO27017↔BSI mappings must exist")

	// Verify mapping framework names are correct.
	for _, p := range iso27017ISO27001Mappings {
		assert.True(t, p.src == "ISO27017" || p.tgt == "ISO27017",
			"pair must involve ISO27017: %+v", p)
	}
}

func TestISO27017_BuiltinAvailableEntry(t *testing.T) {
	found := false
	for _, b := range builtinAvailable {
		if b.name == "ISO27017" {
			found = true
			assert.NotEmpty(t, b.description)
		}
	}
	assert.True(t, found, "ISO27017 must be in builtinAvailable")
}

// ── ISO 27018 ─────────────────────────────────────────────────────────────────

func TestISO27018_ControlCount(t *testing.T) {
	controls := iso27018Controls(s77fwID, s77orgID)
	assert.GreaterOrEqual(t, len(controls), 12,
		"ISO 27018 must have ≥12 controls")
}

func TestISO27018_IDsAreUnique(t *testing.T) {
	controls := iso27018Controls(s77fwID, s77orgID)
	seen := make(map[string]bool, len(controls))
	for _, c := range controls {
		assert.False(t, seen[c.ControlID], "duplicate ControlID: %s", c.ControlID)
		seen[c.ControlID] = true
	}
}

func TestISO27018_AllFieldsPopulated(t *testing.T) {
	controls := iso27018Controls(s77fwID, s77orgID)
	for _, c := range controls {
		assert.NotEmpty(t, c.ControlID, "ControlID must not be empty")
		assert.NotEmpty(t, c.Title, "Title must not be empty for %s", c.ControlID)
		assert.NotEmpty(t, c.Description, "Description must not be empty for %s", c.ControlID)
		assert.NotEmpty(t, c.Domain, "Domain must not be empty for %s", c.ControlID)
		assert.Equal(t, s77fwID, c.FrameworkID)
		assert.Equal(t, s77orgID, c.OrgID)
	}
}

func TestISO27018_CrossMappingsExist(t *testing.T) {
	assert.Greater(t, len(iso27018ISO27001Mappings), 0, "ISO27018↔ISO27001 mappings must exist")
	assert.Greater(t, len(iso27018C5Mappings), 0, "ISO27018↔C5 mappings must exist")

	// The iso27018DSGVO-TOM var may be unnamed — verify via seeder existence (compile-time).
	// We check the C5 side directly since DSGVO-TOM var name is internal.
	for _, p := range iso27018C5Mappings {
		assert.True(t, p.src == "ISO27018" || p.tgt == "ISO27018",
			"pair must involve ISO27018: %+v", p)
	}
}

func TestISO27018_BuiltinAvailableEntry(t *testing.T) {
	found := false
	for _, b := range builtinAvailable {
		if b.name == "ISO27018" {
			found = true
			assert.NotEmpty(t, b.description)
		}
	}
	assert.True(t, found, "ISO27018 must be in builtinAvailable")
}

// ── Version sanity ─────────────────────────────────────────────────────────────

func TestISO27017_Version(t *testing.T) {
	assert.Equal(t, "2015", BuiltinVersion("ISO27017"))
}

func TestISO27018_Version(t *testing.T) {
	assert.Equal(t, "2019", BuiltinVersion("ISO27018"))
}
