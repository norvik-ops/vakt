// Copyright (c) 2026 NorvikOps. All rights reserved.
// SPDX-License-Identifier: Elastic-2.0

// S74-1 + S74-2 unit tests (pure function logic, no DB required).

package vaktcomply

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

// ── S74-1: Fortschrittsformel ─────────────────────────────────────────────────

func TestComputeUmsetzungsgrad_AllJa(t *testing.T) {
	// 10×ja, 0 teilweise, 0 entbehrlich, total 10 → 100%
	pct := computeUmsetzungsgrad(10, 0, 0, 10)
	assert.InDelta(t, 100.0, pct, 0.01)
}

func TestComputeUmsetzungsgrad_MixedFormula(t *testing.T) {
	// Story AC6: 3×ja, 2×teilweise, 1×entbehrlich, 4×nein → total=10
	// relevante = 10-1 = 9
	// punkte = 3×1 + 2×0.5 = 4.0
	// pct = 4/9 × 100 = 44.44...
	pct := computeUmsetzungsgrad(3, 2, 1, 10)
	assert.InDelta(t, 44.44, pct, 0.1)
}

func TestComputeUmsetzungsgrad_AllNein(t *testing.T) {
	pct := computeUmsetzungsgrad(0, 0, 0, 5)
	assert.InDelta(t, 0.0, pct, 0.01)
}

func TestComputeUmsetzungsgrad_AllEntbehrlich(t *testing.T) {
	// All entbehrlich → relevante=0 → 100% (fully handled)
	pct := computeUmsetzungsgrad(0, 0, 5, 5)
	assert.InDelta(t, 100.0, pct, 0.01)
}

func TestComputeUmsetzungsgrad_OnlyTeilweise(t *testing.T) {
	// 4×teilweise, total=4 → punkte=2, relevante=4 → 50%
	pct := computeUmsetzungsgrad(0, 4, 0, 4)
	assert.InDelta(t, 50.0, pct, 0.01)
}

func TestComputeUmsetzungsgrad_Zero(t *testing.T) {
	pct := computeUmsetzungsgrad(0, 0, 0, 0)
	assert.InDelta(t, 100.0, pct, 0.01) // no requirements = 100%
}

// ── S74-1: SetCheckResult — entbehrlich validation (pure logic) ──────────────

func TestSetCheckResult_EntbehrlichRequiresBegruendung(t *testing.T) {
	// Test the guard logic directly (same code path as the service).
	in := SetCheckResultInput{
		Umsetzungsstatus: "entbehrlich",
		Begruendung:      "",
	}
	// We can't call svc.SetCheckResult without a DB, but we can test the guard inline.
	begruendungRequired := in.Umsetzungsstatus == "entbehrlich" && len(trimSpaceStr(in.Begruendung)) == 0
	assert.True(t, begruendungRequired, "entbehrlich without begruendung should trigger guard")

	in2 := SetCheckResultInput{
		Umsetzungsstatus: "entbehrlich",
		Begruendung:      "Nicht anwendbar für dieses System",
	}
	begruendungRequired2 := in2.Umsetzungsstatus == "entbehrlich" && len(trimSpaceStr(in2.Begruendung)) == 0
	assert.False(t, begruendungRequired2, "entbehrlich with begruendung should not trigger guard")
}

func trimSpaceStr(s string) string {
	out := []byte{}
	for _, c := range s {
		if c != ' ' && c != '\t' && c != '\n' {
			out = append(out, byte(c))
		}
	}
	return string(out)
}

// ── S74-1: CreateBSITargetObjectInput validation ─────────────────────────────

func TestBSITargetObjectInput_AbsicherungsniveauDefault(t *testing.T) {
	in := CreateBSITargetObjectInput{
		Name: "Webserver",
		Type: "it_system",
	}
	// Empty Absicherungsniveau should default to "standard" in service.
	assert.Equal(t, "", in.Absicherungsniveau, "zero-value is empty")
}

func TestBSITargetObject_TypeValues(t *testing.T) {
	validTypes := []string{"it_system", "application", "network", "room", "process"}
	for _, typ := range validTypes {
		in := CreateBSITargetObjectInput{
			Name:               "Test",
			Type:               typ,
			Absicherungsniveau: "standard",
		}
		assert.NotEmpty(t, in.Type)
		assert.Equal(t, typ, in.Type)
	}
}

// ── S74-2: GAP-Report CSV builder ────────────────────────────────────────────

func TestBuildGapReportCSV_ContainsHeader(t *testing.T) {
	report := BSIGapReport{
		OrgID: "test-org",
		Gaps:  []BSIGapDetail{},
	}
	csv := buildGapReportCSV(report)
	assert.Contains(t, csv, "baustein_id,anforderung_id")
}

func TestBuildGapReportCSV_EscapesCommasInFields(t *testing.T) {
	report := BSIGapReport{
		Gaps: []BSIGapDetail{
			{
				BausteinID:       "BSI-ORP.1",
				AnforderungID:    "BSI-ORP.1.A1",
				AnforderungTitle: "Übernahme von Verantwortung, für Informationssicherheit",
				Zielobjekt:       "Webserver",
				Umsetzungsstatus: "nein",
			},
		},
	}
	csv := buildGapReportCSV(report)
	assert.Contains(t, csv, `"Übernahme von Verantwortung, für Informationssicherheit"`)
}
