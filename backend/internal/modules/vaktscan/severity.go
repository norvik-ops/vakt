// Copyright (c) 2026 NorvikOps. All rights reserved.
// SPDX-License-Identifier: Elastic-2.0

package vaktscan

import (
	"fmt"
	"sort"
	"strings"
)

// Die Übersetzung eines Scanner-Rohwerts auf die Schweregrad-Skala, die das
// Schema kennt — an genau einem Ort.
//
// ── Warum es diese Datei gibt ─────────────────────────────────────────────────
//
// Trivy und Nuclei liefern regulär den Schweregrad `unknown`, der CHECK auf
// `vb_findings.severity` ließ ihn nicht zu. Gemessen, nicht erinnert:
//
//	trivy image --help  → Allowed values: UNKNOWN, LOW, MEDIUM, HIGH, CRITICAL
//	                      (default [UNKNOWN,LOW,MEDIUM,HIGH,CRITICAL])
//	nuclei -h           → -severity: info, low, medium, high, critical, unknown
//
// `UNKNOWN` steht bei Trivy in der **Vorgabemenge**, und RunTrivyScan ruft trivy
// ohne `--severity` auf — der Wert ist also kein Ausreißer, sondern normale
// Ausgabe. Weil pgx einen Batch in einer impliziten Transaktion fährt, riss eine
// einzige solche Zeile den **ganzen** Stapel mit: Der Scan wurde als
// fehlgeschlagen markiert und nichts wurde gespeichert.
//
// ── Warum `unknown` ein eigener Zustand ist und nicht `info` ──────────────────
//
// Der billige Weg wäre gewesen, `unknown` beim Einlesen auf `info` abzubilden.
// Das wäre eine Behauptung, die nicht stimmt: `info` heißt „bewertet, und zwar
// als unkritisch". Trivys `UNKNOWN` heißt „für diese Schwachstelle gibt es noch
// keine Bewertung" — solche Einträge bekommen später regelmäßig HIGH oder
// CRITICAL. Ein stiller Ersatz durch `info` gäbe ihnen den niedrigsten
// Risiko-Multiplikator (0,25 in ComputeRiskScore) und das info-SLA-Fenster,
// während im Auditbericht „informativ" steht — ein unbewerteter Fund, der als
// bewertet ausgewiesen wird. Genau die Klasse plausibel falscher Werte, an der
// dieses Projekt seine schlimmsten Defekte hatte.
//
// `none` ist der umgekehrte Fall und wird deshalb anders behandelt: In CVSS
// bedeutet das Rating „None" Score 0.0, also eine **durchgeführte** Bewertung mit
// dem Ergebnis „kein Effekt". Das ist `info` und nicht `unknown`.
//
// ── Warum das die Stapel-Abbrüche beendet ────────────────────────────────────
//
// normalizeSeverity ist total: Jeder Eingabewert bekommt ein Ergebnis aus der
// erlaubten Menge. Ein unbekannter Rohwert fällt auf `unknown` — der Fund wird
// also **nicht verworfen**, sondern sichtbar als unbewertet geführt, und keine
// Zeile kann den Stapel mehr kippen. Wenn Trivy morgen einen sechsten Grad
// einführt, ist die Folge ein Hinweis am Scan, kein fehlgeschlagener Import.

// Die Schweregrade, die `vb_findings.severity` per CHECK zulässt
// (Migration 007, erweitert um `unknown` in Migration 265).
const (
	SeverityCritical = "critical"
	SeverityHigh     = "high"
	SeverityMedium   = "medium"
	SeverityLow      = "low"
	SeverityInfo     = "info"
	// SeverityUnknown heißt „nicht bewertet", nicht „harmlos".
	SeverityUnknown = "unknown"
)

// canonicalSeverity spiegelt den CHECK-Constraint. Wer hier etwas hinzufügt,
// braucht eine Migration — der Test TestCanonicalSeverityMatchesConstraint
// fährt beide Mengen gegen echtes Postgres gegeneinander.
var canonicalSeverity = map[string]struct{}{
	SeverityCritical: {},
	SeverityHigh:     {},
	SeverityMedium:   {},
	SeverityLow:      {},
	SeverityInfo:     {},
	SeverityUnknown:  {},
}

// normalizeSeverity bildet einen Scanner-Rohwert auf die erlaubte Menge ab.
//
// Der zweite Rückgabewert sagt, ob der Rohwert **unbekannt** war. Er trennt zwei
// Fälle, die beide auf `unknown` laufen, aber verschieden viel bedeuten:
//
//   - false — erwartet: der Scanner hat `unknown` gesagt oder gar nichts.
//   - true  — unerwartet: der Scanner hat etwas gesagt, das wir nicht kennen.
//     Das ist ein Datenqualitäts-Signal und gehört dem Nutzer gemeldet.
func normalizeSeverity(raw string) (severity string, unrecognised bool) {
	s := strings.ToLower(strings.TrimSpace(raw))

	if _, ok := canonicalSeverity[s]; ok {
		return s, false
	}

	switch s {
	case "":
		// Gar keine Angabe ist keine Bewertung. Vorher stand hier `info` — das
		// war dieselbe Behauptung wie oben, nur für den leeren Fall.
		return SeverityUnknown, false
	case "none":
		// CVSS-Rating „None" = Score 0.0: bewertet, Ergebnis „kein Effekt".
		return SeverityInfo, false
	case "informational", "information":
		return SeverityInfo, false
	}

	return SeverityUnknown, true
}

// severityReport zählt die Rohwerte, die normalizeSeverity nicht kannte.
//
// Still verwerfen ist keine Option, still umdeuten auch nicht: Der Fund landet in
// der Datenbank (als `unknown`), und diese Meldung sagt dem Nutzer am Scan, dass
// und warum etwas umgedeutet wurde.
type severityReport struct {
	counts map[string]int
	total  int
}

// add vermerkt einen nicht erkannten Rohwert.
func (r *severityReport) add(raw string) {
	if r.counts == nil {
		r.counts = make(map[string]int)
	}
	// Der Rohwert kommt aus einer Fremdquelle und wird dem Nutzer angezeigt —
	// deshalb gekappt, damit eine absurd lange Zeile die Meldung nicht sprengt.
	key := strings.ToLower(strings.TrimSpace(raw))
	if len(key) > 32 {
		key = key[:32] + "…"
	}
	r.counts[key]++
	r.total++
}

// note formuliert den Hinweis für den Nutzer, oder "" wenn es nichts zu sagen gibt.
//
// Die Rohwerte werden sortiert ausgegeben, damit die Meldung bei gleicher Eingabe
// gleich aussieht — sonst wäre sie in Tests und Logs nicht vergleichbar.
func (r *severityReport) note() string {
	if r.total == 0 {
		return ""
	}

	keys := make([]string, 0, len(r.counts))
	for k := range r.counts {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	parts := make([]string, 0, len(keys))
	for _, k := range keys {
		parts = append(parts, fmt.Sprintf("%q×%d", k, r.counts[k]))
	}

	return fmt.Sprintf(
		"%d Fund(e) mit unbekanntem Schweregrad (%s) wurden als \"unknown\" eingestuft — sie sind gespeichert, aber unbewertet.",
		r.total, strings.Join(parts, ", "),
	)
}
