// Copyright (c) 2026 NorvikOps. All rights reserved.
// SPDX-License-Identifier: Elastic-2.0

package policy

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/matharnica/vakt/internal/db"
)

// R1-KW-6: TallyReadiness is what the framework overview (ListFrameworks) and
// the dashboard read. It must give exactly the number ComputeReadinessReport
// gives the framework detail for the same controls (ADR-0086).
func TestTallyReadiness_MatchesComputeReadinessReport(t *testing.T) {
	rows := []db.ListCKFrameworkReadinessRowsRow{
		{FrameworkID: "fw", ManualStatus: "implemented"},                      // covered
		{FrameworkID: "fw", EvidenceCount: 2},                                 // covered by evidence
		{FrameworkID: "fw", EvidenceCount: 1},                                 // partial
		{FrameworkID: "fw", ManualStatus: "in_progress"},                      // partial
		{FrameworkID: "fw", NotApplicable: true, ManualStatus: "implemented"}, // out of the denominator
		{FrameworkID: "fw"},                                                   // missing
		{FrameworkID: "fw"},                                                   // missing
		{FrameworkID: "na-only", NotApplicable: true},
	}

	got := TallyReadiness(rows)

	require.Equal(t, FrameworkReadiness{Covered: 2, Partial: 2, Applicable: 6, Score: 50}, got["fw"])
	require.Equal(t, FrameworkReadiness{}, got["na-only"], "nur nicht-anwendbare Kontrollen: 0 %, kein Teilen durch null")

	controls := []Control{
		{ID: "1", ManualStatus: "implemented"},
		{ID: "2"},
		{ID: "3"},
		{ID: "4", ManualStatus: "in_progress"},
		{ID: "5", NotApplicable: true, ManualStatus: "implemented"},
		{ID: "6"},
		{ID: "7"},
	}
	report := ComputeReadinessReport(&Framework{ID: "fw"}, controls, map[string]int{"2": 2, "3": 1})
	require.InDelta(t, report.ReadinessScore, got["fw"].Score, 0.0001, "Übersicht/Dashboard ≠ Detail")
}

// Subreview PR #182: die Dashboard-Gesamtzahl wird nach ADR-0086 gepoolt —
// Summen über alle Frameworks, dann einmal ReadinessScore. Kein Mittelwert der
// Framework-Prozente, nicht anwendbare Kontrollen aus dem Nenner, Teil zählt halb.
func TestPoolReadiness_PoolsSumsNotPercentages(t *testing.T) {
	byFW := TallyReadiness([]db.ListCKFrameworkReadinessRowsRow{
		{FrameworkID: "a", ManualStatus: "implemented"},
		{FrameworkID: "a", ManualStatus: "in_progress"},
		{FrameworkID: "a", NotApplicable: true},
		{FrameworkID: "a"},
		{FrameworkID: "b"},
		{FrameworkID: "b"},
		{FrameworkID: "b"},
	})
	got := PoolReadiness(byFW)
	require.Equal(t, 1, got.Covered)
	require.Equal(t, 1, got.Partial)
	require.Equal(t, 6, got.Applicable)
	require.InDelta(t, 1.5/6*100, got.Score, 0.0001)
	require.Equal(t, FrameworkReadiness{}, PoolReadiness(nil), "keine Frameworks: 0, kein Teilen durch null")
}
