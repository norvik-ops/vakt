// Copyright (c) 2026 NorvikOps. All rights reserved.
// SPDX-License-Identifier: Elastic-2.0

package vaktscan

import "strings"

// Die drei Dedup-Schlüssel von vb_findings — und warum ein Leerstring hier kein
// „kein Wert" ist.
//
// ── Der Fehler, den das hier verhindert (2026-07-14) ─────────────────────────
//
// Migration 120 legt drei PARTIELLE Unique-Indexe an:
//
//	idx_vb_findings_dedup_cve       (org_id, asset_id, cve_id)            WHERE cve_id      IS NOT NULL
//	idx_vb_findings_dedup_template  (org_id, asset_id, scanner, template) WHERE template_id IS NOT NULL
//	idx_vb_findings_dedup_rawid     (org_id, raw_id, scanner)             WHERE raw_id      IS NOT NULL
//
// „Partiell", schreibt die Migration selbst, „weil die jeweiligen Spalten NULL sein
// dürfen und mehrere NULL-Werte erlaubt sein müssen". Genau darauf ist alles
// gebaut: Ein Fund ohne Template soll nicht mit jedem anderen Fund ohne Template
// kollidieren.
//
// Der Go-Code hat aber nie NULL geschrieben. `Finding.TemplateID` und
// `Finding.RawID` sind `string`; fehlt der Wert, ist er `""` — und `''` ist in
// PostgreSQL **NOT NULL**. Die partiellen Indexe griffen also für JEDEN Fund, und
// zwar mit demselben Schlüssel:
//
//   - Zwei Trivy-Funde auf demselben Asset teilen sich (org, asset, 'trivy', '')
//     → Unique-Verletzung beim zweiten.
//   - Schlimmer noch der raw_id-Index: Er läuft über (org, raw_id, scanner), OHNE
//     das Asset. Mit raw_id = '' konnte eine Organisation genau EINEN Trivy-Fund
//     halten — über alle Assets hinweg.
//
// Und weil pgx einen Batch in eine implizite Transaktion legt, riss die eine
// kollidierende Zeile den gesamten Batch mit: `BatchUpsertFindings` loggte die
// Zeile, zählte die davor als Erfolg und gab einen positiven Zähler zurück —
// während in der Datenbank NICHTS ankam. Ein Scan mit zwei Funden meldete also
// „abgeschlossen, 1 Fund" und speicherte null.
//
// Aufgefallen ist es, als der Scan-Weg zum ersten Mal von einem Test durchlaufen
// wurde. Vorher konnte ihn kein Test aufrufen (Unterprozess + Datenbank), und der
// Demo-Seed füllt vb_findings direkt — in der Demo sah Vakt Scan also aus, als
// funktioniere es.

// dedupKey macht aus einem optionalen Textwert das, was das Schema erwartet: NULL,
// wenn er fehlt. Ein Leerstring ist ein Wert, kein fehlender Wert — und für einen
// partiellen Unique-Index ist dieser Unterschied alles.
func dedupKey(s string) *string {
	if s == "" {
		return nil
	}
	return &s
}

// cveKey ist dasselbe für `cve_id` — und zugleich die einzige Stelle, an der
// entschieden wird, welcher Text dort landet.
//
// Zwei Gründe, warum das ein eigener Choke-Point ist und nicht bei jedem
// Importer einzeln steht:
//
//  1. Der Unique-Index vergleicht Text exakt. „cve-2021-44228" und
//     „CVE-2021-44228" sind für PostgreSQL zwei Schlüssel, für einen Menschen
//     eine Schwachstelle. Ohne Normalisierung hängt die Deduplizierung an der
//     Schreiblaune des jeweiligen Werkzeugs.
//  2. Ein Leerstring ist NICHT „keine CVE": ein leerer Text ist in PostgreSQL
//     NOT NULL, der partielle
//     Index `WHERE cve_id IS NOT NULL` griffe also für jede Zeile und ließe alle
//     Funde eines Assets zu einer einzigen zusammenfallen — genau die Klasse,
//     die Migration 243 für raw_id/template_id aufgeräumt hat.
//
// Der Wert, der hier herauskommt, entscheidet zugleich, welcher Arbiter beim
// Upsert greift (Repository.UpsertImportedFinding). Deshalb liegen
// Normalisierung und Arbiter-Wahl bewusst nebeneinander und nicht an zwei Enden
// des Codes.
func cveKey(s *string) *string {
	if s == nil {
		return nil
	}
	v := strings.ToUpper(strings.TrimSpace(*s))
	if v == "" {
		return nil
	}
	return &v
}
