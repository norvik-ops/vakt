// Copyright (c) 2026 NorvikOps. All rights reserved.
// SPDX-License-Identifier: Elastic-2.0

package vaktscan

import "testing"

// R1-RV-01: Der SARIF-Import hat `cve_id` nie gesetzt. Die ruleId traegt bei den
// verbreiteten SARIF-Erzeugern die Schwachstellenkennung, wurde aber nur in
// `raw_id` und in den Titel geschrieben.
//
// Diese Tabelle haelt die ERKENNUNGSREGEL fest, nicht nur ihr Ergebnis: erkannt
// wird ausschliesslich eine CVE-Kennung am ANFANG der ruleId, alles dahinter ist
// Beiwerk des jeweiligen Werkzeugs (Grype haengt den Paketnamen an). Wer die
// Regel lockert, faellt in die Gegenrichtung: eine Semgrep-Regel-Kennung als CVE
// zu schreiben, wuerde voellig unverwandte Funde zusammenlegen — falsche
// Deduplizierung ist schlimmer als keine, weil sie Befunde VERSCHWINDEN laesst.
func TestSarifCVEFromRuleID(t *testing.T) {
	cases := []struct {
		name   string
		ruleID string
		want   string // "" = keine CVE erkannt
	}{
		// ── erkannt ──────────────────────────────────────────────────────────
		{"Trivy: ruleId ist die nackte CVE", "CVE-2021-44228", "CVE-2021-44228"},
		{"Grype: CVE plus angehaengter Paketname", "CVE-2021-44228-log4j-core", "CVE-2021-44228"},
		{"OSV: CVE mit Pfad-Suffix", "CVE-2023-45283/golang.org/x/net", "CVE-2023-45283"},
		{"fuenfstellige laufende Nummer", "CVE-2024-123456", "CVE-2024-123456"},
		{"Kleinschreibung wird normalisiert", "cve-2021-44228", "CVE-2021-44228"},
		{"Rand-Leerzeichen werden entfernt", "  CVE-2021-44228  ", "CVE-2021-44228"},

		// ── bewusst NICHT erkannt ────────────────────────────────────────────
		{"Semgrep-Regel ist keine CVE", "go.lang.security.audit.xss.no-direct-write-to-responsewriter", ""},
		{"CodeQL-Regel ist keine CVE", "go/sql-injection", ""},
		{"GHSA bleibt aussen vor", "GHSA-1234-5678-9abc", ""},
		{"CVE nicht am Anfang zaehlt nicht", "pkg-CVE-2021-44228", ""},
		{"zu kurze laufende Nummer", "CVE-2021-123", ""},
		{"Jahr unvollstaendig", "CVE-21-44228", ""},
		{"leere ruleId", "", ""},
		{"nur das Praefix", "CVE-", ""},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := sarifCVEFromRuleID(tc.ruleID)
			if tc.want == "" {
				if got != nil {
					t.Fatalf("ruleId %q darf keine CVE ergeben, bekam %q", tc.ruleID, *got)
				}
				return
			}
			if got == nil {
				t.Fatalf("ruleId %q muss %q ergeben, bekam nil", tc.ruleID, tc.want)
			}
			if *got != tc.want {
				t.Fatalf("ruleId %q: erwartet %q, bekam %q", tc.ruleID, tc.want, *got)
			}
		})
	}
}

// cveKey ist der Choke-Point, an dem entschieden wird, was in die Spalte `cve_id`
// geht — und damit, welcher partielle Unique-Index fuer die Zeile greift. Ein
// Leerstring ist dort KEIN fehlender Wert: ein leerer Text ist in PostgreSQL
// NOT NULL, der
// partielle Index `WHERE cve_id IS NOT NULL` wuerde also fuer jede Zeile greifen
// und alle Funde eines Assets zu einem einzigen zusammenfallen lassen. Genau
// diese Klasse hat Migration 243 fuer `raw_id`/`template_id` aufgeraeumt
// (siehe dedup_keys.go).
func TestCVEKey(t *testing.T) {
	str := func(s string) *string { return &s }

	cases := []struct {
		name string
		in   *string
		want *string
	}{
		{"nil bleibt nil", nil, nil},
		{"Leerstring wird NULL, nicht ''", str(""), nil},
		{"nur Leerzeichen wird NULL", str("   "), nil},
		{"Kleinschreibung wird normalisiert", str("cve-2021-44228"), str("CVE-2021-44228")},
		{"Rand-Leerzeichen entfallen", str(" CVE-2021-44228 "), str("CVE-2021-44228")},
		{"unveraendert, wenn schon normal", str("CVE-2021-44228"), str("CVE-2021-44228")},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := cveKey(tc.in)
			switch {
			case tc.want == nil && got != nil:
				t.Fatalf("erwartet nil, bekam %q", *got)
			case tc.want != nil && got == nil:
				t.Fatalf("erwartet %q, bekam nil", *tc.want)
			case tc.want != nil && *got != *tc.want:
				t.Fatalf("erwartet %q, bekam %q", *tc.want, *got)
			}
		})
	}
}
