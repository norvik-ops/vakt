// Copyright (c) 2026 NorvikOps. All rights reserved.
// SPDX-License-Identifier: Elastic-2.0

package policy

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// The SoA shows the status of its control, with the rule of the control page
// (ResolveStatus): manual status first, otherwise the evidence count.
func TestSoAStatusFromControl_FollowsResolveStatus(t *testing.T) {
	cases := []struct {
		name     string
		na       bool
		manual   string
		evidence int
		want     string
	}{
		{"manuell umgesetzt", false, "implemented", 0, "implemented"},
		{"manuell in Bearbeitung", false, "in_progress", 0, "partial"},
		{"NIS2-Dialekt partial", false, "partial", 0, "partial"},
		// A.5.2 aus dem UX-Durchgang: ein Nachweis, kein Status -> "In Bearbeitung".
		{"ein Nachweis ohne Status", false, "", 1, "partial"},
		{"zwei Nachweise ohne Status", false, "", 2, "implemented"},
		{"nichts", false, "", 0, "not_started"},
		{"Kontrolle nicht anwendbar", true, "implemented", 3, "not_started"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			assert.Equal(t, c.want, SoAStatusFromControl(c.na, c.manual, c.evidence))
		})
	}
}

func TestResolveSoAEntry_DraftReadsControl(t *testing.T) {
	e := SoADedicatedEntry{ControlRef: "A.5.2", ControlName: "Information security roles and responsibilities", ImplementationStatus: "not_started"}
	resolveSoAEntry(&e, false, &soaControlLink{
		ID: "ctrl-1", FrameworkID: "fw-1", Title: "Rollen und Verantwortlichkeiten", EvidenceCount: 1,
	})
	assert.Equal(t, "partial", e.ImplementationStatus, "Entwurf zeigt den Status der Kontrolle, nicht das alte SoA-Feld")
	assert.Equal(t, "Rollen und Verantwortlichkeiten", e.ControlName, "Entwurf zeigt den Titel der Kontrolle")
	assert.Equal(t, SoAStatusSourceControl, e.StatusSource)
	require.NotNil(t, e.LinkedControlID)
	assert.Equal(t, "ctrl-1", *e.LinkedControlID)
	require.NotNil(t, e.LinkedFrameworkID)
	assert.Equal(t, "fw-1", *e.LinkedFrameworkID)
}

func TestResolveSoAEntry_ApprovedKeepsSnapshot(t *testing.T) {
	e := SoADedicatedEntry{ControlRef: "A.5.2", ControlName: "Stand bei Freigabe", ImplementationStatus: "planned"}
	resolveSoAEntry(&e, true, &soaControlLink{
		ID: "ctrl-1", FrameworkID: "fw-1", Title: "Neuer Titel", ManualStatus: "implemented",
	})
	assert.Equal(t, "planned", e.ImplementationStatus, "eingefrorene Version darf sich nicht ändern")
	assert.Equal(t, "Stand bei Freigabe", e.ControlName)
	assert.Equal(t, SoAStatusSourceSnapshot, e.StatusSource)
	require.NotNil(t, e.LinkedControlID, "Link zur Kontrolle bleibt für die Navigation")
}

func TestResolveSoAEntry_DraftWithoutControlKeepsStored(t *testing.T) {
	e := SoADedicatedEntry{ControlRef: "A.8.9", ControlName: "Configuration management", ImplementationStatus: "not_started"}
	resolveSoAEntry(&e, false, nil)
	assert.Equal(t, "not_started", e.ImplementationStatus)
	assert.Equal(t, "Configuration management", e.ControlName)
	assert.Equal(t, SoAStatusSourceSoA, e.StatusSource)
	assert.Nil(t, e.LinkedControlID)
}

// summarizeSoA follows the shared counting rule (ReadinessScore): implemented
// counts fully, partial half; excluded entries are outside the denominator.
func TestSummarizeSoA_CountingRule(t *testing.T) {
	s := summarizeSoA([]SoADedicatedEntry{
		{Applicable: true, ImplementationStatus: "implemented"},
		{Applicable: true, ImplementationStatus: "partial"},
		{Applicable: true, ImplementationStatus: "not_started"},
		{Applicable: true, ImplementationStatus: "planned"},
		{Applicable: false, ImplementationStatus: "implemented"},
	})
	assert.Equal(t, 4, s.ApplicableCount)
	assert.Equal(t, 1, s.ExcludedCount)
	assert.Equal(t, 1, s.ImplementedCount)
	assert.Equal(t, 1, s.PartialCount)
	assert.Equal(t, 1, s.PlannedCount)
	assert.Equal(t, 1, s.NotStartedCount)
	assert.InDelta(t, ReadinessScore(1, 1, 4), s.ImplementationPct, 0.0001)
	assert.InDelta(t, 37.5, s.ImplementationPct, 0.0001)
}

func TestSummarizeSoA_Empty(t *testing.T) {
	s := summarizeSoA(nil)
	assert.Equal(t, 0, s.ApplicableCount)
	assert.Equal(t, 0.0, s.ImplementationPct)
}
