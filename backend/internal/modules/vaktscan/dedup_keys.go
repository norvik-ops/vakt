// Copyright (c) 2026 NorvikOps. All rights reserved.
// SPDX-License-Identifier: Elastic-2.0

package vaktscan

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
