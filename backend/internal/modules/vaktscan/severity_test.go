// Copyright (c) 2026 NorvikOps. All rights reserved.
// SPDX-License-Identifier: Elastic-2.0

package vaktscan

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestNormalizeSeverity haelt die Entscheidung fest, die dieser Fix getroffen hat:
// „unbewertet" ist ein eigener Zustand und nicht dasselbe wie „info".
func TestNormalizeSeverity(t *testing.T) {
	cases := []struct {
		name         string
		raw          string
		want         string
		unrecognised bool
		warum        string
	}{
		// Die Grade, die Trivy und Nuclei tatsaechlich liefern.
		{"trivy CRITICAL", "CRITICAL", SeverityCritical, false, "Trivy schreibt gross, gespeichert wird klein"},
		{"trivy HIGH", "HIGH", SeverityHigh, false, ""},
		{"trivy MEDIUM", "MEDIUM", SeverityMedium, false, ""},
		{"trivy LOW", "LOW", SeverityLow, false, ""},
		{"nuclei info", "info", SeverityInfo, false, ""},

		// Der Wert, an dem der Stapel gestorben ist.
		{
			"trivy UNKNOWN", "UNKNOWN", SeverityUnknown, false,
			"steht in Trivys Vorgabemenge — regulaere Ausgabe, kein Ausreisser",
		},
		{"nuclei unknown", "unknown", SeverityUnknown, false, ""},

		// Gar keine Angabe ist keine Bewertung.
		{
			"leer", "", SeverityUnknown, false,
			"vorher `info` — das behauptete eine Bewertung, die nie stattgefunden hat",
		},
		{"nur Leerzeichen", "   ", SeverityUnknown, false, ""},

		// CycloneDX: `none` ist der umgekehrte Fall.
		{
			"cyclonedx none", "none", SeverityInfo, false,
			"CVSS-Rating None = Score 0.0: bewertet, Ergebnis 'kein Effekt' — das IST `info`",
		},
		{"informational", "Informational", SeverityInfo, false, ""},

		// Alles, was niemand kennt, faellt auf `unknown` und wird gemeldet.
		{
			"erfundener Grad", "catastrophic", SeverityUnknown, true,
			"kein Abbruch, kein Verwerfen, keine erfundene Einstufung",
		},
		{"Zahl", "7", SeverityUnknown, true, ""},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, unrecognised := normalizeSeverity(tc.raw)
			assert.Equal(t, tc.want, got, tc.warum)
			assert.Equal(t, tc.unrecognised, unrecognised,
				"das Meldeflag trennt 'der Scanner sagte unknown' von 'der Scanner sagte etwas Unbekanntes'")
		})
	}
}

// TestNormalizeSeverityIstTotal: Jede Eingabe muss auf einen Wert fallen, den der
// CHECK zulaesst. Genau das beendet die Stapel-Abbrueche — es darf keinen
// Eingabewert geben, fuer den die Funktion etwas Ungueltiges zurueckgibt.
func TestNormalizeSeverityIstTotal(t *testing.T) {
	inputs := []string{
		"", " ", "critical", "CRITICAL", "unknown", "none", "n/a", "7", "catastrophic",
		"süper-kritisch", "'; DROP TABLE vb_findings; --", "\x00\xff",
	}
	for _, in := range inputs {
		got, _ := normalizeSeverity(in)
		_, ok := canonicalSeverity[got]
		require.True(t, ok,
			"normalizeSeverity(%q) = %q liegt ausserhalb der erlaubten Menge — genau so kippt ein Stapel", in, got)
	}
}

// TestSeverityReportNote prueft die Meldung, die der Nutzer am Scan zu sehen
// bekommt. Still umdeuten ist keine Option — wenn nichts umgedeutet wurde, darf
// aber auch nichts dastehen.
func TestSeverityReportNote(t *testing.T) {
	t.Run("nichts umgedeutet, nichts zu melden", func(t *testing.T) {
		var r severityReport
		assert.Empty(t, r.note(), "eine leere Meldung darf keinen Hinweis am Scan erzeugen")
	})

	t.Run("meldet Anzahl und Rohwerte", func(t *testing.T) {
		var r severityReport
		r.add("catastrophic")
		r.add("catastrophic")
		r.add("weird")

		note := r.note()
		assert.Contains(t, note, "3 Fund(e)", "die Gesamtzahl gehoert in die Meldung")
		assert.Contains(t, note, `"catastrophic"×2`, "der Rohwert des Scanners gehoert in die Meldung")
		assert.Contains(t, note, `"weird"×1`)
		assert.Contains(t, note, "gespeichert", "der Nutzer muss erfahren, dass die Funde NICHT verworfen wurden")
	})

	t.Run("stabile Reihenfolge", func(t *testing.T) {
		// Ohne Sortierung waere die Meldung bei gleicher Eingabe mal so und mal so —
		// in Logs nicht vergleichbar und in Tests nicht pruefbar.
		var a, b severityReport
		for _, v := range []string{"zeta", "alpha", "mu"} {
			a.add(v)
		}
		for _, v := range []string{"mu", "zeta", "alpha"} {
			b.add(v)
		}
		assert.Equal(t, a.note(), b.note())
	})

	t.Run("kappt absurd lange Rohwerte", func(t *testing.T) {
		var r severityReport
		long := ""
		for i := 0; i < 200; i++ {
			long += "x"
		}
		r.add(long)
		assert.Less(t, len(r.note()), 200,
			"ein Rohwert aus einer Fremdquelle darf die Meldung nicht sprengen")
	})
}
