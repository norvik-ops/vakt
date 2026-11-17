// Copyright (c) 2026 NorvikOps. All rights reserved.
// SPDX-License-Identifier: Elastic-2.0

package policy

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/matharnica/vakt/internal/shared/pdfutil/pdftest"
)

// isoAnnexAEntries baut 93 SoA-Zeilen in der Gruppierung von ISO 27001:2022
// Annex A (37 / 8 / 14 / 34) — genug für mehrere Tabellenseiten.
func isoAnnexAEntries() []SoADedicatedEntry {
	var entries []SoADedicatedEntry
	for _, g := range []struct {
		group string
		count int
	}{{"5", 37}, {"6", 8}, {"7", 14}, {"8", 34}} {
		for i := 1; i <= g.count; i++ {
			entries = append(entries, SoADedicatedEntry{
				ControlRef:           fmt.Sprintf("A.%s.%d", g.group, i),
				ControlName:          fmt.Sprintf("Kontrolle %s.%d", g.group, i),
				ControlGroup:         g.group,
				Applicable:           true,
				Justification:        "Begründung",
				ImplementationStatus: "implemented",
			})
		}
	}
	return entries
}

// Nach einem Seitenumbruch zeichnet buildSoAPDF den Tabellenkopf neu. Der Kopf
// setzt weiße Schrift; die Kontrollzeilen danach müssen wieder dunkel stehen.
// Vorher standen bei ISO 27001 33 von 93 Kontrollen weiß auf weiß im PDF.
func TestBuildSoAPDF_TabellentextNachSeitenumbruchSichtbar(t *testing.T) {
	entries := isoAnnexAEntries()
	pdf, err := buildSoAPDF(entries, &SoASummary{Status: "draft", ApplicableCount: len(entries)}, 1)
	require.NoError(t, err)

	runs := pdftest.TextRuns(t, pdf)
	refColor := map[string]pdftest.TextRun{}
	for _, r := range runs {
		refColor[r.Text] = r
	}

	pages := map[int]bool{}
	for _, e := range entries {
		run, ok := refColor[e.ControlRef]
		require.True(t, ok, "Kontrolle %s fehlt im PDF-Text", e.ControlRef)
		pages[run.Page] = true
		assert.NotEqual(t, pdftest.White, run.Color,
			"Kontrolle %s auf Seite %d ist weiß gesetzt und damit unsichtbar", e.ControlRef, run.Page)
	}
	require.Greater(t, len(pages), 1, "der Test braucht Kontrollen auf mehr als einer Tabellenseite")
}
