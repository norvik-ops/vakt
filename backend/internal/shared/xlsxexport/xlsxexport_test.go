// Copyright (c) 2026 NorvikOps. All rights reserved.
// SPDX-License-Identifier: Elastic-2.0

package xlsxexport

import (
	"bytes"
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/xuri/excelize/v2"
)

func sampleSoARows() []SoARow {
	return []SoARow{
		{
			ControlRef:           "A.5.1",
			ControlName:          "Policies for information security",
			ControlGroup:         "5 Organizational",
			Applicable:           true,
			Justification:        "ISMS in place",
			ImplementationStatus: "implemented",
			Owner:                "CISO",
			UpdatedAt:            time.Date(2026, 3, 15, 0, 0, 0, 0, time.UTC),
		},
		{
			ControlRef:           "A.5.2",
			ControlName:          "Information security roles",
			ControlGroup:         "5 Organizational",
			Applicable:           false,
			Justification:        "Not applicable for our size",
			ImplementationStatus: "not_applicable",
			Owner:                "",
			UpdatedAt:            time.Date(2026, 3, 15, 0, 0, 0, 0, time.UTC),
		},
	}
}

func sampleSoASummary() SoASummary {
	return SoASummary{
		ApplicableCount:   1,
		ExcludedCount:     1,
		ImplementedCount:  1,
		PartialCount:      0,
		PlannedCount:      0,
		NotStartedCount:   0,
		ImplementationPct: 100.0,
	}
}

func sampleRiskRows() []RiskRow {
	score := 12
	return []RiskRow{
		{
			ID:            "00000000-0000-0000-0000-000000000001",
			Title:         "Phishing attack",
			Category:      "external",
			Likelihood:    3,
			Impact:        4,
			RiskScore:     12,
			Treatment:     "mitigate",
			Status:        "open",
			Owner:         "IT-Team",
			DueDate:       nil,
			ResidualScore: &score,
		},
		{
			ID:            "00000000-0000-0000-0000-000000000002",
			Title:         "Data loss",
			Category:      "internal",
			Likelihood:    2,
			Impact:        5,
			RiskScore:     10,
			Treatment:     "transfer",
			Status:        "mitigated",
			Owner:         "SecOps",
			DueDate:       nil,
			ResidualScore: nil,
		},
	}
}

func TestRenderSoA_ReturnsValidXLSX(t *testing.T) {
	data, err := RenderSoA(sampleSoARows(), sampleSoASummary())
	require.NoError(t, err)
	require.NotEmpty(t, data)

	f, err := excelize.OpenReader(bytes.NewReader(data))
	require.NoError(t, err)
	defer f.Close()

	sheets := f.GetSheetList()
	assert.Contains(t, sheets, "SoA")
	assert.Contains(t, sheets, "Zusammenfassung")
}

func TestRenderSoA_SheetHasCorrectHeaders(t *testing.T) {
	data, err := RenderSoA(sampleSoARows(), sampleSoASummary())
	require.NoError(t, err)

	f, err := excelize.OpenReader(bytes.NewReader(data))
	require.NoError(t, err)
	defer f.Close()

	cell, _ := f.GetCellValue("SoA", "A1")
	assert.Equal(t, "Control ID", cell)
	cell, _ = f.GetCellValue("SoA", "D1")
	assert.Equal(t, "Anwendbar", cell)
}

func TestRenderSoA_DataRowsPresent(t *testing.T) {
	data, err := RenderSoA(sampleSoARows(), sampleSoASummary())
	require.NoError(t, err)

	f, err := excelize.OpenReader(bytes.NewReader(data))
	require.NoError(t, err)
	defer f.Close()

	// Row 2 = first data row
	cell, _ := f.GetCellValue("SoA", "A2")
	assert.Equal(t, "A.5.1", cell)
	cell, _ = f.GetCellValue("SoA", "D2")
	assert.Equal(t, "Ja", cell)

	cell, _ = f.GetCellValue("SoA", "D3")
	assert.Equal(t, "Nein", cell)
}

func TestRenderSoA_SummarySheetHasData(t *testing.T) {
	data, err := RenderSoA(sampleSoARows(), sampleSoASummary())
	require.NoError(t, err)

	f, err := excelize.OpenReader(bytes.NewReader(data))
	require.NoError(t, err)
	defer f.Close()

	cell, _ := f.GetCellValue("Zusammenfassung", "A1")
	assert.Contains(t, cell, "Zusammenfassung")
}

func TestRenderRisiken_ReturnsValidXLSX(t *testing.T) {
	data, err := RenderRisiken(sampleRiskRows())
	require.NoError(t, err)
	require.NotEmpty(t, data)

	f, err := excelize.OpenReader(bytes.NewReader(data))
	require.NoError(t, err)
	defer f.Close()

	sheets := f.GetSheetList()
	assert.Contains(t, sheets, "Risiken")
	assert.Contains(t, sheets, "Matrix")
}

func TestRenderRisiken_SheetHasCorrectHeaders(t *testing.T) {
	data, err := RenderRisiken(sampleRiskRows())
	require.NoError(t, err)

	f, err := excelize.OpenReader(bytes.NewReader(data))
	require.NoError(t, err)
	defer f.Close()

	cell, _ := f.GetCellValue("Risiken", "A1")
	assert.Equal(t, "Risk ID", cell)
	cell, _ = f.GetCellValue("Risiken", "F1")
	assert.Equal(t, "Risikostufe", cell)
}

func TestRenderRisiken_DataRowsPresent(t *testing.T) {
	data, err := RenderRisiken(sampleRiskRows())
	require.NoError(t, err)

	f, err := excelize.OpenReader(bytes.NewReader(data))
	require.NoError(t, err)
	defer f.Close()

	cell, _ := f.GetCellValue("Risiken", "B2")
	assert.Equal(t, "Phishing attack", cell)
}

func TestRenderRisiken_MatrixSheetPresent(t *testing.T) {
	data, err := RenderRisiken(sampleRiskRows())
	require.NoError(t, err)

	f, err := excelize.OpenReader(bytes.NewReader(data))
	require.NoError(t, err)
	defer f.Close()

	cell, _ := f.GetCellValue("Matrix", "A1")
	assert.Contains(t, cell, "Risikomatrix")
}

func TestRenderSoA_EmptyRows(t *testing.T) {
	data, err := RenderSoA([]SoARow{}, SoASummary{})
	require.NoError(t, err)
	assert.NotEmpty(t, data)
}

func TestRenderRisiken_EmptyRows(t *testing.T) {
	data, err := RenderRisiken([]RiskRow{})
	require.NoError(t, err)
	assert.NotEmpty(t, data)
}

func TestColName(t *testing.T) {
	assert.Equal(t, "A", colName(1))
	assert.Equal(t, "Z", colName(26))
	assert.Equal(t, "AA", colName(27))
	assert.Equal(t, "AZ", colName(52))
}

// Die Risiko-Anzahl muss in der Zelle ihres eigenen Scores stehen. Vorher
// landete sie eine Zeile zu hoch — ein Risiko mit W=5 überschrieb die
// Achsen-Kopfzeile, eines mit W=1 stand bei W=2.
func TestRenderRisiken_MatrixCountsInOwnCell(t *testing.T) {
	rows := []RiskRow{
		{ID: "00000001-aaaa-bbbb-cccc-000000000000", Title: "a", Likelihood: 5, Impact: 5},
		{ID: "00000002-aaaa-bbbb-cccc-000000000000", Title: "b", Likelihood: 5, Impact: 5},
		{ID: "00000003-aaaa-bbbb-cccc-000000000000", Title: "c", Likelihood: 1, Impact: 3},
		{ID: "00000004-aaaa-bbbb-cccc-000000000000", Title: "d", Likelihood: 0, Impact: 9}, // ausserhalb der Matrix: ignorieren
	}
	data, err := RenderRisiken(rows)
	require.NoError(t, err)

	f, err := excelize.OpenReader(bytes.NewReader(data))
	require.NoError(t, err)
	defer f.Close()

	get := func(cell string) string {
		v, _ := f.GetCellValue("Matrix", cell)
		return v
	}
	// W=5 steht in Zeile 4, A=5 in Spalte F.
	assert.Equal(t, "25 (2×)", get("F4"))
	// W=1 steht in Zeile 8, A=3 in Spalte D.
	assert.Equal(t, "3 (1×)", get("D8"))
	// Achsen-Kopfzeile und Nachbarzellen bleiben unberührt.
	assert.Equal(t, "A=5", get("F3"))
	assert.Equal(t, "6", get("D7")) // W=2 × A=3, ohne Anzahl
}

// fillOf returns the pattern fill colour of a cell (upper case, no '#').
func fillOf(t *testing.T, f *excelize.File, sheet, cell string) string {
	t.Helper()
	id, err := f.GetCellStyle(sheet, cell)
	require.NoError(t, err)
	st, err := f.GetStyle(id)
	require.NoError(t, err)
	if len(st.Fill.Color) == 0 {
		return ""
	}
	return st.Fill.Color[0]
}

// R1-KW-17: the XLSX export colours the score with the same four levels as the
// risk register (15/10/5). Before the fix it cut at 15/8, so score 8 (Mittel)
// and score 10 (Hoch) shared one colour.
func TestRenderRisiken_LevelColoursFollowRiskRegister(t *testing.T) {
	// likelihood × impact as a user would enter them: 4×5=20, 2×5=10, 2×4=8, 1×4=4.
	pairs := [][2]int{{4, 5}, {2, 5}, {2, 4}, {1, 4}}
	rows := make([]RiskRow, 0, len(pairs))
	for i, p := range pairs {
		rows = append(rows, RiskRow{ID: fmt.Sprintf("0000000%d-aaaa", i), Title: "R", Likelihood: p[0], Impact: p[1], RiskScore: p[0] * p[1]})
	}
	data, err := RenderRisiken(rows)
	require.NoError(t, err)
	f, err := excelize.OpenReader(bytes.NewReader(data))
	require.NoError(t, err)
	defer f.Close()

	// Column F = score ("Risikostufe"), rows 2..5.
	assert.Equal(t, "FEE2E2", fillOf(t, f, "Risiken", "F2"), "score 20 = Kritisch")
	assert.Equal(t, "FFEDD5", fillOf(t, f, "Risiken", "F3"), "score 10 = Hoch")
	assert.Equal(t, "FEF3C7", fillOf(t, f, "Risiken", "F4"), "score 8 = Mittel")
	assert.Equal(t, "DCFCE7", fillOf(t, f, "Risiken", "F5"), "score 4 = Niedrig")

	// Matrix sheet: row 4 = W=5 … row 8 = W=1; column B = A=1 … F = A=5.
	assert.Equal(t, "FCA5A5", fillOf(t, f, "Matrix", "F6"), "matrix W=3×A=5=15 Kritisch")
	assert.Equal(t, "FDBA74", fillOf(t, f, "Matrix", "F7"), "matrix W=2×A=5=10 Hoch")
	assert.Equal(t, "FDE68A", fillOf(t, f, "Matrix", "E7"), "matrix W=2×A=4=8 Mittel")
	assert.Equal(t, "FDE68A", fillOf(t, f, "Matrix", "F8"), "matrix W=1×A=5=5 Mittel")
	assert.Equal(t, "86EFAC", fillOf(t, f, "Matrix", "E8"), "matrix W=1×A=4=4 Niedrig")
}
