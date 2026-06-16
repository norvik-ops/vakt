// Copyright (c) 2026 NorvikOps. All rights reserved.
// SPDX-License-Identifier: Elastic-2.0

// S89-6: report renderers for the two most-requested auditor documents — the
// Statement of Applicability and the risk register — as editable .docx.

package docxexport

import (
	"fmt"
	"strconv"
	"time"
)

// SoARow is one Statement-of-Applicability entry.
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

// SoASummary holds aggregate SoA stats.
type SoASummary struct {
	ApplicableCount   int
	ExcludedCount     int
	ImplementedCount  int
	ImplementationPct float64
}

// RiskRow is one risk-register entry.
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

func yesNo(b bool) string {
	if b {
		return "Ja"
	}
	return "Nein"
}

func fmtDate(t *time.Time) string {
	if t == nil || t.IsZero() {
		return ""
	}
	return t.Format("2006-01-02")
}

// RenderSoA produces a .docx Statement of Applicability.
func RenderSoA(rows []SoARow, summary SoASummary) ([]byte, error) {
	d := New()
	d.Heading("Statement of Applicability (SoA) — ISO/IEC 27001:2022")
	d.Paragraph(fmt.Sprintf("Erstellt: %s", time.Now().UTC().Format("2006-01-02")))
	d.Paragraph(fmt.Sprintf(
		"Anwendbare Controls: %d · Ausgeschlossen: %d · Umgesetzt: %d (%.0f%%)",
		summary.ApplicableCount, summary.ExcludedCount, summary.ImplementedCount, summary.ImplementationPct))
	d.Paragraph("")

	headers := []string{"Control", "Titel", "Klausel", "Anwendbar", "Status", "Verantwortlich", "Begründung"}
	tableRows := make([][]string, len(rows))
	for i, r := range rows {
		tableRows[i] = []string{
			r.ControlRef, r.ControlName, r.ControlGroup, yesNo(r.Applicable),
			r.ImplementationStatus, r.Owner, r.Justification,
		}
	}
	d.Table(headers, tableRows)
	return d.Bytes()
}

// RenderRisiken produces a .docx risk register.
func RenderRisiken(rows []RiskRow) ([]byte, error) {
	d := New()
	d.Heading("Risikoregister")
	d.Paragraph(fmt.Sprintf("Erstellt: %s · %d Risiken", time.Now().UTC().Format("2006-01-02"), len(rows)))
	d.Paragraph("")

	headers := []string{"Titel", "Kategorie", "Eintritt", "Auswirkung", "Score", "Behandlung", "Status", "Verantwortlich", "Fällig", "Restrisiko"}
	tableRows := make([][]string, len(rows))
	for i, r := range rows {
		residual := ""
		if r.ResidualScore != nil {
			residual = strconv.Itoa(*r.ResidualScore)
		}
		tableRows[i] = []string{
			r.Title, r.Category, strconv.Itoa(r.Likelihood), strconv.Itoa(r.Impact),
			strconv.Itoa(r.RiskScore), r.Treatment, r.Status, r.Owner, fmtDate(r.DueDate), residual,
		}
	}
	d.Table(headers, tableRows)
	return d.Bytes()
}
