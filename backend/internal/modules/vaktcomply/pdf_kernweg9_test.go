// Copyright (c) 2026 NorvikOps. All rights reserved.
// SPDX-License-Identifier: Elastic-2.0

package vaktcomply

import (
	"fmt"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/matharnica/vakt/internal/modules/vaktcomply/policy"
	"github.com/matharnica/vakt/internal/shared/pdfutil/pdftest"
)

// Die Entwurfs-SoA (kein kanonisches SoA initialisiert, oder kein ISO-Framework)
// zeichnet je Domäne einen Tabellenkopf in weißer Schrift. Die erste Kontrolle
// unter jedem Kopf erbte diese Farbe und stand weiß auf hellem Grund.
func TestGenerateSoAPDF_ErsteZeileNachTabellenkopfSichtbar(t *testing.T) {
	var rows []SoARow
	for _, d := range []string{"A.5 Organisatorisch", "A.8 Technologisch"} {
		for i := 1; i <= 3; i++ {
			rows = append(rows, SoARow{
				ControlID:  fmt.Sprintf("%s.%d", strings.Fields(d)[0], i),
				Title:      fmt.Sprintf("Kontrolle %s.%d", strings.Fields(d)[0], i),
				Domain:     d,
				Applicable: true,
			})
		}
	}
	pdf, err := GenerateSoAPDF(rows, "ISO 27001", "Test GmbH", time.Now(), true)
	require.NoError(t, err)

	colors := map[string]string{}
	for _, r := range pdftest.TextRuns(t, pdf) {
		colors[r.Text] = r.Color
	}
	for _, row := range rows {
		for _, text := range []string{row.ControlID, row.Title} {
			c, ok := colors[text]
			require.True(t, ok, "%q fehlt im PDF-Text", text)
			assert.NotEqual(t, pdftest.White, c, "%q ist weiß gesetzt und damit unsichtbar", text)
		}
	}
}

// Seite 1 der Compliance-Übersicht: „Abgedeckt“ im Kopf und die Summe der
// Domänen-Spalte „Abgedeckt“ müssen bei teilweise umgesetzten Kontrollen
// übereinstimmen — beide kommen aus ComputeReadinessReport.
func TestGenerateFrameworkPDF_KopfUndDomaenenTabelleZaehlenGleich(t *testing.T) {
	fw := &policy.Framework{ID: "fw-1", Name: "ISO 27001"}
	controls := []policy.Control{
		{ID: "c-1", Domain: "Organisation", ManualStatus: "implemented"},
		{ID: "c-2", Domain: "Organisation", ManualStatus: "partial"},
		{ID: "c-3", Domain: "Organisation"},
		{ID: "c-4", Domain: "Technik"},
	}
	report := policy.ComputeReadinessReport(fw, controls, map[string]int{})

	pdf, err := GenerateFrameworkPDF(report, nil, "Test GmbH")
	require.NoError(t, err)
	runs := pdftest.TextRuns(t, pdf)

	head := -1
	for _, r := range runs {
		if v, ok := strings.CutPrefix(r.Text, "Abgedeckt: "); ok {
			head, err = strconv.Atoi(v)
			require.NoError(t, err)
		}
	}
	require.Equal(t, 1, head, "Kopf „Abgedeckt“ fehlt oder zählt falsch")

	// Domänen-Zeilen: Name, Controls, Abgedeckt, Bereitschaft.
	sum := 0
	for i, r := range runs {
		if (r.Text == "Organisation" || r.Text == "Technik") && i+3 < len(runs) {
			n, err := strconv.Atoi(runs[i+2].Text)
			require.NoError(t, err, "Spalte „Abgedeckt“ nach %q nicht lesbar", r.Text)
			sum += n
			if r.Text == "Organisation" {
				assert.Equal(t, "50%", runs[i+3].Text, "1 umgesetzt + 1 teilweise von 3 = 50 %")
			}
		}
	}
	assert.Equal(t, head, sum, "Domänen-Spalte „Abgedeckt“ summiert sich anders als der Kopf")
}
