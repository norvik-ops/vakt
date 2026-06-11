// Copyright (c) 2026 NorvikOps. All rights reserved.
// SPDX-License-Identifier: Elastic-2.0

package vaktcomply

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const (
	testFWID  = "fw-id"
	testOrgID = "org-id"
)

// TestDORASimplified_ExactlyFifteenControls (C1) — activating the simplified variant
// must produce exactly 15 DORA-S.* controls and zero DORA-1.x controls.
func TestDORASimplified_ExactlyFifteenControls(t *testing.T) {
	controls := doraSimplifiedControls(testFWID, testOrgID)

	require.Len(t, controls, 15, "simplified DORA must have exactly 15 controls (RTS EU 2024/1774 Art. 3–10)")

	for _, c := range controls {
		assert.True(t, strings.HasPrefix(c.ControlID, "DORA-S."),
			"simplified control %q must use DORA-S.* prefix", c.ControlID)
	}
}

// TestDORASimplified_NoFullControls (C1) — builtinControls with variant="simplified"
// must return no DORA-1.x through DORA-5.x controls.
func TestDORASimplified_NoFullControls(t *testing.T) {
	controls := builtinControls(testFWID, testOrgID, "DORA", "simplified")

	for _, c := range controls {
		assert.False(t,
			strings.HasPrefix(c.ControlID, "DORA-1.") ||
				strings.HasPrefix(c.ControlID, "DORA-2.") ||
				strings.HasPrefix(c.ControlID, "DORA-3.") ||
				strings.HasPrefix(c.ControlID, "DORA-4.") ||
				strings.HasPrefix(c.ControlID, "DORA-5."),
			"simplified variant must not include full-framework control %q", c.ControlID)
	}
}

// TestDORAFull_NoSimplifiedControls (C2 companion) — builtinControls with variant="full"
// must not contain any DORA-S.* controls.
func TestDORAFull_NoSimplifiedControls(t *testing.T) {
	controls := builtinControls(testFWID, testOrgID, "DORA", "full")

	for _, c := range controls {
		assert.False(t, strings.HasPrefix(c.ControlID, "DORA-S."),
			"full variant must not include simplified control %q", c.ControlID)
	}
}

// TestDORASimplified_IDsAreUnique guards against accidental duplicate IDs.
// PostgreSQL UNIQUE(framework_id, control_id) would surface this as a confusing error.
func TestDORASimplified_IDsAreUnique(t *testing.T) {
	controls := doraSimplifiedControls(testFWID, testOrgID)
	seen := make(map[string]bool, len(controls))
	for _, c := range controls {
		assert.False(t, seen[c.ControlID], "control_id %s appears twice in simplified DORA", c.ControlID)
		seen[c.ControlID] = true
	}
}

// TestDORASimplified_EveryControlHasRequiredFields verifies structural completeness.
func TestDORASimplified_EveryControlHasRequiredFields(t *testing.T) {
	controls := doraSimplifiedControls(testFWID, testOrgID)
	validEvType := map[string]bool{"manual": true, "automated": true, "third_party": true}

	for _, c := range controls {
		assert.NotEmpty(t, c.Title, "control %s missing Title", c.ControlID)
		assert.NotEmpty(t, c.Domain, "control %s missing Domain", c.ControlID)
		assert.NotEmpty(t, c.Description, "control %s missing Description", c.ControlID)
		assert.True(t, validEvType[c.EvidenceType],
			"control %s has invalid EvidenceType %q", c.ControlID, c.EvidenceType)
		assert.True(t, c.Weight >= 1 && c.Weight <= 3,
			"control %s has Weight %d outside 1..3", c.ControlID, c.Weight)
		assert.Equal(t, testFWID, c.FrameworkID)
		assert.Equal(t, testOrgID, c.OrgID)
	}
}

// TestDORAVariant_FilterQueryParam (C3) — builtinControls dispatches correctly
// based on the variant string, simulating what the ?framework_variant= query
// param would trigger in the service layer.
func TestDORAVariant_FilterQueryParam(t *testing.T) {
	simplified := builtinControls(testFWID, testOrgID, "DORA", "simplified")
	full := builtinControls(testFWID, testOrgID, "DORA", "full")
	defaultFull := builtinControls(testFWID, testOrgID, "DORA", "")

	// Simplified must be exactly 15 DORA-S.* controls.
	assert.Len(t, simplified, 15)

	// Full must have no DORA-S.* controls.
	for _, c := range full {
		assert.False(t, strings.HasPrefix(c.ControlID, "DORA-S."),
			"full variant must not have simplified control %q", c.ControlID)
	}

	// Empty variant string must default to full.
	assert.Equal(t, len(full), len(defaultFull),
		"empty variant must default to full framework")
}
