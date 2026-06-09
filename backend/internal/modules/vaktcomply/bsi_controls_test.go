// Copyright (c) 2026 NorvikOps. All rights reserved.
// SPDX-License-Identifier: Elastic-2.0

package vaktcomply

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

// minBSIControls is the floor we promise to ship as the "BSI baseline" in
// the customer-facing materials.  Sprint 59 closed the audit gap (7 stubs
// → 34 fully described controls).  If a future change drops below this
// number, the test red-lights the regression.
const minBSIControls = 50

// requiredBSILayers lists every BSI-Grundschutz-Kompendium layer that the
// baseline must cover. The check uses the BSI-XXX.* ID prefix as the
// layer identifier — it is the canonical ordering from the Standard
// 200-2 / Kompendium 2023.
var requiredBSILayers = []string{
	"BSI-ISMS.", // Sicherheitsmanagement
	"BSI-ORP.",  // Organisation und Personal
	"BSI-CON.",  // Konzeption und Vorgehensweise
	"BSI-OPS.",  // Betrieb
	"BSI-DER.",  // Detektion und Reaktion
	"BSI-APP.",  // Anwendungen
	"BSI-SYS.",  // IT-Systeme
	"BSI-IND.",  // Industrielle IT
	"BSI-NET.",  // Netze und Kommunikation
	"BSI-INF.",  // Infrastruktur
}

// TestBSIControls_BaselineSize closes audit finding F-BSI: the previous
// implementation shipped 7 stub controls with empty descriptions, which
// disqualified Vakt from BSI Grundschutz Procurement. This test pins the
// modern baseline so a future refactor cannot silently regress.
func TestBSIControls_BaselineSize(t *testing.T) {
	ctrls := bsiControls("framework-id", "org-id")
	assert.GreaterOrEqual(t, len(ctrls), minBSIControls,
		"BSI baseline must keep at least %d controls (got %d) — audit-relevant", minBSIControls, len(ctrls))
}

// TestBSIControls_AllLayersCovered ensures the baseline spans every BSI
// Kompendium layer. A baseline that misses a whole layer (e.g. no IND.*
// controls because OT is out of scope for the customer demographic) would
// be a marketing-vs-reality drift.
func TestBSIControls_AllLayersCovered(t *testing.T) {
	ctrls := bsiControls("framework-id", "org-id")
	seen := make(map[string]bool, len(requiredBSILayers))
	for _, ctrl := range ctrls {
		for _, layer := range requiredBSILayers {
			if strings.HasPrefix(ctrl.ControlID, layer) {
				seen[layer] = true
			}
		}
	}
	for _, layer := range requiredBSILayers {
		assert.True(t, seen[layer], "layer %s missing from BSI baseline", layer)
	}
}

// TestBSIControls_EveryControlHasDescription is the regression for the
// audit's "7 stub controls without description" finding. Every entry
// must carry a non-empty Description so the Compliance-UI can render
// meaningful guidance.
func TestBSIControls_EveryControlHasDescription(t *testing.T) {
	ctrls := bsiControls("framework-id", "org-id")
	for _, c := range ctrls {
		assert.NotEmpty(t, c.Description, "control %s must have a non-empty Description", c.ControlID)
		assert.GreaterOrEqual(t, len(c.Description), 60,
			"control %s description is suspiciously short (%d chars) — should read like a checklist item", c.ControlID, len(c.Description))
	}
}

// TestBSIControls_HasTitleDomainAndEvidenceType pins the structural
// completeness of every control — these fields drive the dashboard
// rendering and per-domain progress calculation. Empty values would
// show up as broken-looking UI rows.
func TestBSIControls_HasTitleDomainAndEvidenceType(t *testing.T) {
	ctrls := bsiControls("framework-id", "org-id")
	validEvType := map[string]bool{"manual": true, "automated": true, "third_party": true}
	for _, c := range ctrls {
		assert.NotEmpty(t, c.Title, "control %s missing Title", c.ControlID)
		assert.NotEmpty(t, c.Domain, "control %s missing Domain", c.ControlID)
		assert.True(t, validEvType[c.EvidenceType],
			"control %s has invalid EvidenceType %q", c.ControlID, c.EvidenceType)
		assert.True(t, c.Weight >= 1 && c.Weight <= 3,
			"control %s has Weight %d outside 1..3", c.ControlID, c.Weight)
	}
}

// TestBSIControls_IDsAreUnique guards against accidental duplicates when
// new layers are added — Postgres' UNIQUE(framework_id, control_id) would
// otherwise fail at install time with a confusing error.
func TestBSIControls_IDsAreUnique(t *testing.T) {
	ctrls := bsiControls("framework-id", "org-id")
	seen := make(map[string]bool, len(ctrls))
	for _, c := range ctrls {
		assert.False(t, seen[c.ControlID], "control_id %s appears twice", c.ControlID)
		seen[c.ControlID] = true
	}
}
