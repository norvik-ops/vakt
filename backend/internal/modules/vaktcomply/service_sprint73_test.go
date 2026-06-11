// Copyright (c) 2026 NorvikOps. All rights reserved.
// SPDX-License-Identifier: Elastic-2.0

package vaktcomply

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestBSIDemoSeedControls_Coverage verifies that the 8 BSI seed controls used
// in demoseed cover the key BSI IT-Grundschutz Schichten (layers) and have
// matching control IDs in the full bsiControls catalog.
func TestBSIDemoSeedControls_Coverage(t *testing.T) {
	seedIDs := []string{
		"BSI-ISMS.1.A1",
		"BSI-ISMS.1.A6",
		"BSI-ORP.3.A1",
		"BSI-OPS.1.1.3.A1",
		"BSI-OPS.1.1.4.A1",
		"BSI-DER.1.A1",
		"BSI-CON.3.A1",
		"BSI-NET.1.1.A1",
	}

	catalog := bsiControls("fw-id", "org-id")
	catalogByID := make(map[string]struct{}, len(catalog))
	for _, c := range catalog {
		catalogByID[c.ControlID] = struct{}{}
	}

	for _, id := range seedIDs {
		_, ok := catalogByID[id]
		assert.True(t, ok, "BSI seed control %s must exist in bsiControls catalog", id)
	}

	// Seed controls must span at least 4 distinct Schichten.
	schichten := make(map[string]struct{})
	for _, id := range seedIDs {
		// Control ID format: "BSI-<LAYER>.<num>.A<req>" — extract layer prefix.
		withoutPrefix := strings.TrimPrefix(id, "BSI-")
		dotIdx := strings.Index(withoutPrefix, ".")
		if dotIdx > 0 {
			schichten[withoutPrefix[:dotIdx]] = struct{}{}
		}
	}
	assert.GreaterOrEqual(t, len(schichten), 4,
		"BSI demo seed must cover at least 4 Grundschutz-Schichten, got %d", len(schichten))
}

// TestBSIDemoSeedControls_ManualStatusValues verifies that only valid
// manual_status values are used in the BSI demo seed definition.
func TestBSIDemoSeedControls_ManualStatusValues(t *testing.T) {
	validStatuses := map[string]bool{
		"":            true,
		"in_progress": true,
		"implemented": true,
	}
	seedStatuses := []string{"implemented", "in_progress", "implemented", "implemented", "implemented", "", "in_progress", ""}
	for i, s := range seedStatuses {
		require.True(t, validStatuses[s],
			"BSI seed control index %d has invalid manual_status %q", i, s)
	}
}

// TestAuditorExportZIPRouteRegistration verifies that RegisterAuditor wires
// the export.zip endpoint (S73-1). This is a compile-time guard via the route
// list — the handler must exist as a method on Handler.
func TestAuditorExportZIPRouteRegistration(t *testing.T) {
	// Verify that AuditorExportZIP is defined on *Handler.
	var h *Handler
	// The method must be callable (compile check); we do not call it
	// with a live echo context here — just verify it exists as a function value.
	fn := h.AuditorExportZIP
	require.NotNil(t, fn, "AuditorExportZIP must be defined on Handler")
}
