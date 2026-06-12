// Copyright (c) 2026 NorvikOps. All rights reserved.
// SPDX-License-Identifier: Elastic-2.0

package xlsxexport

import (
	"bytes"
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
