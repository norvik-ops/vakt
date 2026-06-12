// Copyright (c) 2026 NorvikOps. All rights reserved.
// SPDX-License-Identifier: Elastic-2.0

// Package xlsxexport produces audit-ready XLSX files using excelize.
// Functions are pure data → bytes; no HTTP or DB coupling.
package xlsxexport

import (
	"fmt"
	"time"

	"github.com/xuri/excelize/v2"
)

// SoARow is one entry in the Statement of Applicability export.
type SoARow struct {
	ControlRef           string
	ControlName          string
	ControlGroup         string
	Applicable           bool
	Justification        string
	ImplementationStatus string
	Owner                string
	UpdatedAt            time.Time
}

// SoASummary holds aggregate stats for the summary sheet.
type SoASummary struct {
	ApplicableCount   int
	ExcludedCount     int
	ImplementedCount  int
	PartialCount      int
	PlannedCount      int
	NotStartedCount   int
	ImplementationPct float64
}

// RiskRow is one entry in the risk register export.
type RiskRow struct {
	ID            string
	Title         string
	Category      string
	Likelihood    int
	Impact        int
	RiskScore     int
	Treatment     string
	Status        string
	Owner         string
	DueDate       *time.Time
	ResidualScore *int
}

// RenderSoA produces a two-sheet XLSX for the Statement of Applicability.
func RenderSoA(rows []SoARow, summary SoASummary) ([]byte, error) {
	f := excelize.NewFile()
	defer f.Close()

	// ── Sheet 1: SoA ──────────────────────────────────────────────────────────
	sheet := "SoA"
	f.SetSheetName("Sheet1", sheet)

	headers := []string{
		"Control ID", "Titel", "Klausel", "Anwendbar",
		"Begründung", "Implementierungsstatus", "Verantwortlicher", "Letzte Änderung",
	}
	widths := []float64{14, 42, 22, 10, 40, 22, 22, 16}

	headerStyle, err := f.NewStyle(&excelize.Style{
		Font:      &excelize.Font{Bold: true, Color: "FFFFFF"},
		Fill:      excelize.Fill{Type: "pattern", Color: []string{"1E3A5F"}, Pattern: 1},
		Alignment: &excelize.Alignment{Horizontal: "center", Vertical: "center"},
		Border: []excelize.Border{
			{Type: "bottom", Color: "FFFFFF", Style: 1},
		},
	})
	if err != nil {
		return nil, err
	}

	for i, h := range headers {
		cell := colName(i+1) + "1"
		_ = f.SetCellValue(sheet, cell, h)
		_ = f.SetCellStyle(sheet, cell, cell, headerStyle)
		_ = f.SetColWidth(sheet, colName(i+1), colName(i+1), widths[i])
	}
	_ = f.SetRowHeight(sheet, 1, 18)
	_ = f.SetPanes(sheet, &excelize.Panes{Freeze: true, YSplit: 1, TopLeftCell: "A2", ActivePane: "bottomLeft"})
	_ = f.AutoFilter(sheet, "A1:H1", []excelize.AutoFilterOptions{})

	yesStyle, _ := f.NewStyle(&excelize.Style{Font: &excelize.Font{Color: "1A6B2A"}})
	noStyle, _ := f.NewStyle(&excelize.Style{Font: &excelize.Font{Color: "B91C1C"}})

	for i, r := range rows {
		row := i + 2
		applicable := "Nein"
		cellStyle := noStyle
		if r.Applicable {
			applicable = "Ja"
			cellStyle = yesStyle
		}
		_ = f.SetCellValue(sheet, colName(1)+fmt.Sprint(row), r.ControlRef)
		_ = f.SetCellValue(sheet, colName(2)+fmt.Sprint(row), r.ControlName)
		_ = f.SetCellValue(sheet, colName(3)+fmt.Sprint(row), r.ControlGroup)
		_ = f.SetCellValue(sheet, colName(4)+fmt.Sprint(row), applicable)
		_ = f.SetCellStyle(sheet, colName(4)+fmt.Sprint(row), colName(4)+fmt.Sprint(row), cellStyle)
		_ = f.SetCellValue(sheet, colName(5)+fmt.Sprint(row), r.Justification)
		_ = f.SetCellValue(sheet, colName(6)+fmt.Sprint(row), r.ImplementationStatus)
		_ = f.SetCellValue(sheet, colName(7)+fmt.Sprint(row), r.Owner)
		_ = f.SetCellValue(sheet, colName(8)+fmt.Sprint(row), r.UpdatedAt.UTC().Format("2006-01-02"))
	}

	// ── Sheet 2: Zusammenfassung ──────────────────────────────────────────────
	sumSheet := "Zusammenfassung"
	_, _ = f.NewSheet(sumSheet)

	titleStyle, _ := f.NewStyle(&excelize.Style{Font: &excelize.Font{Bold: true, Size: 13}})
	labelStyle, _ := f.NewStyle(&excelize.Style{Font: &excelize.Font{Bold: true}})

	_ = f.SetCellValue(sumSheet, "A1", "Statement of Applicability — Zusammenfassung")
	_ = f.SetCellStyle(sumSheet, "A1", "A1", titleStyle)
	_ = f.SetColWidth(sumSheet, "A", "A", 36)
	_ = f.SetColWidth(sumSheet, "B", "B", 14)

	summaryRows := [][]interface{}{
		{"Anwendbare Controls", summary.ApplicableCount},
		{"Ausgeschlossene Controls", summary.ExcludedCount},
		{"Implementiert", summary.ImplementedCount},
		{"Teilweise implementiert", summary.PartialCount},
		{"Geplant", summary.PlannedCount},
		{"Nicht begonnen", summary.NotStartedCount},
		{"Implementierungsgrad", fmt.Sprintf("%.1f %%", summary.ImplementationPct)},
	}
	for i, sr := range summaryRows {
		row := i + 3
		_ = f.SetCellValue(sumSheet, fmt.Sprintf("A%d", row), sr[0])
		_ = f.SetCellStyle(sumSheet, fmt.Sprintf("A%d", row), fmt.Sprintf("A%d", row), labelStyle)
		_ = f.SetCellValue(sumSheet, fmt.Sprintf("B%d", row), sr[1])
	}

	buf, err := f.WriteToBuffer()
	if err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

// RenderRisiken produces a two-sheet XLSX for the risk register.
func RenderRisiken(rows []RiskRow) ([]byte, error) {
	f := excelize.NewFile()
	defer f.Close()

	// ── Sheet 1: Risiken ──────────────────────────────────────────────────────
	sheet := "Risiken"
	f.SetSheetName("Sheet1", sheet)

	headers := []string{
		"Risk ID", "Titel", "Kategorie", "Wahrscheinlichkeit",
		"Auswirkung", "Risikostufe", "Behandlung", "Status",
		"Verantwortlicher", "Fälligkeitsdatum", "Restrisiko",
	}
	widths := []float64{10, 36, 16, 18, 12, 12, 14, 12, 20, 16, 12}

	headerStyle, err := f.NewStyle(&excelize.Style{
		Font:      &excelize.Font{Bold: true, Color: "FFFFFF"},
		Fill:      excelize.Fill{Type: "pattern", Color: []string{"1E3A5F"}, Pattern: 1},
		Alignment: &excelize.Alignment{Horizontal: "center", Vertical: "center"},
	})
	if err != nil {
		return nil, err
	}
	for i, h := range headers {
		cell := colName(i+1) + "1"
		_ = f.SetCellValue(sheet, cell, h)
		_ = f.SetCellStyle(sheet, cell, cell, headerStyle)
		_ = f.SetColWidth(sheet, colName(i+1), colName(i+1), widths[i])
	}
	_ = f.SetRowHeight(sheet, 1, 18)
	_ = f.SetPanes(sheet, &excelize.Panes{Freeze: true, YSplit: 1, TopLeftCell: "A2", ActivePane: "bottomLeft"})
	_ = f.AutoFilter(sheet, "A1:K1", []excelize.AutoFilterOptions{})

	highStyle, _ := f.NewStyle(&excelize.Style{Fill: excelize.Fill{Type: "pattern", Color: []string{"FEE2E2"}, Pattern: 1}})
	medStyle, _ := f.NewStyle(&excelize.Style{Fill: excelize.Fill{Type: "pattern", Color: []string{"FEF3C7"}, Pattern: 1}})
	lowStyle, _ := f.NewStyle(&excelize.Style{Fill: excelize.Fill{Type: "pattern", Color: []string{"DCFCE7"}, Pattern: 1}})

	for i, r := range rows {
		row := i + 2
		dueDate := ""
		if r.DueDate != nil {
			dueDate = r.DueDate.UTC().Format("2006-01-02")
		}
		residual := ""
		if r.ResidualScore != nil {
			residual = fmt.Sprint(*r.ResidualScore)
		}
		_ = f.SetCellValue(sheet, colName(1)+fmt.Sprint(row), r.ID[:8]) // short ID
		_ = f.SetCellValue(sheet, colName(2)+fmt.Sprint(row), r.Title)
		_ = f.SetCellValue(sheet, colName(3)+fmt.Sprint(row), r.Category)
		_ = f.SetCellValue(sheet, colName(4)+fmt.Sprint(row), r.Likelihood)
		_ = f.SetCellValue(sheet, colName(5)+fmt.Sprint(row), r.Impact)
		_ = f.SetCellValue(sheet, colName(6)+fmt.Sprint(row), r.RiskScore)
		_ = f.SetCellValue(sheet, colName(7)+fmt.Sprint(row), r.Treatment)
		_ = f.SetCellValue(sheet, colName(8)+fmt.Sprint(row), r.Status)
		_ = f.SetCellValue(sheet, colName(9)+fmt.Sprint(row), r.Owner)
		_ = f.SetCellValue(sheet, colName(10)+fmt.Sprint(row), dueDate)
		_ = f.SetCellValue(sheet, colName(11)+fmt.Sprint(row), residual)

		// Color-code the score cell by risk level.
		scoreCell := colName(6) + fmt.Sprint(row)
		switch {
		case r.RiskScore >= 15:
			_ = f.SetCellStyle(sheet, scoreCell, scoreCell, highStyle)
		case r.RiskScore >= 8:
			_ = f.SetCellStyle(sheet, scoreCell, scoreCell, medStyle)
		default:
			_ = f.SetCellStyle(sheet, scoreCell, scoreCell, lowStyle)
		}
	}

	// ── Sheet 2: Matrix (5×5 heatmap) ────────────────────────────────────────
	matSheet := "Matrix"
	_, _ = f.NewSheet(matSheet)

	titleStyle, _ := f.NewStyle(&excelize.Style{Font: &excelize.Font{Bold: true, Size: 13}})
	_ = f.SetCellValue(matSheet, "A1", "Risikomatrix (Wahrscheinlichkeit × Auswirkung)")
	_ = f.SetCellStyle(matSheet, "A1", "A1", titleStyle)

	axisStyle, _ := f.NewStyle(&excelize.Style{Font: &excelize.Font{Bold: true}, Alignment: &excelize.Alignment{Horizontal: "center"}})

	// Column headers (Impact 1–5)
	for col := 1; col <= 5; col++ {
		cell := colName(col+1) + "3"
		_ = f.SetCellValue(matSheet, cell, fmt.Sprintf("A=%d", col))
		_ = f.SetCellStyle(matSheet, cell, cell, axisStyle)
		_ = f.SetColWidth(matSheet, colName(col+1), colName(col+1), 10)
	}
	_ = f.SetCellValue(matSheet, "A3", "W \\ A")
	_ = f.SetCellStyle(matSheet, "A3", "A3", axisStyle)

	// Row headers (Likelihood 5–1, top = high)
	for rowOff := 0; rowOff < 5; rowOff++ {
		likelihood := 5 - rowOff
		row := rowOff + 4
		_ = f.SetCellValue(matSheet, fmt.Sprintf("A%d", row), fmt.Sprintf("W=%d", likelihood))
		_ = f.SetCellStyle(matSheet, fmt.Sprintf("A%d", row), fmt.Sprintf("A%d", row), axisStyle)
		_ = f.SetRowHeight(matSheet, row, 22)

		for colOff := 0; colOff < 5; colOff++ {
			impact := colOff + 1
			score := likelihood * impact
			cell := colName(colOff+2) + fmt.Sprint(row)
			_ = f.SetCellValue(matSheet, cell, score)

			var fillColor string
			switch {
			case score >= 15:
				fillColor = "FCA5A5" // red
			case score >= 8:
				fillColor = "FDE68A" // amber
			default:
				fillColor = "86EFAC" // green
			}
			cellStyle, _ := f.NewStyle(&excelize.Style{
				Fill:      excelize.Fill{Type: "pattern", Color: []string{fillColor}, Pattern: 1},
				Alignment: &excelize.Alignment{Horizontal: "center"},
				Border: []excelize.Border{
					{Type: "left", Color: "CCCCCC", Style: 1},
					{Type: "top", Color: "CCCCCC", Style: 1},
				},
			})
			_ = f.SetCellStyle(matSheet, cell, cell, cellStyle)
		}
	}

	// Count risks per cell and annotate.
	type cellKey struct{ l, i int }
	counts := make(map[cellKey]int)
	for _, r := range rows {
		counts[cellKey{r.Likelihood, r.Impact}]++
	}
	for k, cnt := range counts {
		rowOff := 5 - k.l // likelihood 5 → row 4, likelihood 1 → row 8
		cell := colName(k.i+1) + fmt.Sprint(rowOff+3)
		existing, _ := f.GetCellValue(matSheet, cell)
		_ = f.SetCellValue(matSheet, cell, fmt.Sprintf("%s (%d×)", existing, cnt))
	}

	buf, err := f.WriteToBuffer()
	if err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

// colName converts a 1-based column index to an Excel column name (A, B, ..., Z, AA, ...).
func colName(col int) string {
	name := ""
	for col > 0 {
		col--
		name = string(rune('A'+col%26)) + name
		col /= 26
	}
	return name
}
