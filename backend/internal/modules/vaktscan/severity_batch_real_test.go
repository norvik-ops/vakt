// Copyright (c) 2026 NorvikOps. All rights reserved.
// SPDX-License-Identifier: Elastic-2.0

package vaktscan

import (
	"context"
	"fmt"
	"os"
	"sort"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// R1-W6C-N1: Trivy und Nuclei liefern regulaer den Schweregrad `unknown`, der
// CHECK auf vb_findings.severity kannte ihn nicht. Weil pgx einen Batch in EINER
// impliziten Transaktion faehrt, riss eine einzige solche Zeile den GANZEN Stapel
// mit — der Scan galt als fehlgeschlagen und KEIN Fund wurde gespeichert.
//
// Gemessen statt erinnert (beide Mengen im Commit dokumentiert):
//
//	trivy image --help  → Allowed values: UNKNOWN, LOW, MEDIUM, HIGH, CRITICAL
//	                      (default [UNKNOWN,LOW,MEDIUM,HIGH,CRITICAL])
//	nuclei -h           → -severity: info, low, medium, high, critical, unknown
//	CHECK (Migration 007) → critical, high, medium, low, info
//
// ── Warum dieser Test aus der DATENBANK zaehlt ───────────────────────────────
//
// Der von BatchUpsertFindings gemeldete Zaehler allein ist als Nachweis zu
// schwach. Genau dieser Zaehler war in diesem Projekt schon einmal positiv,
// waehrend in der Datenbank nichts ankam (Migration 243, "Zeile loggen und
// weiterzaehlen"). Ein Test, der nur den Rueckgabewert prueft, haette das
// durchgewunken. Also: `SELECT count(*)` gegen echtes Postgres, und der gemeldete
// Zaehler wird zusaetzlich GEGEN diese Zahl geprueft.

// severityTestDB oeffnet die Wegwerf-Postgres oder ueberspringt den Test.
func severityTestDB(ctx context.Context, t *testing.T) *pgxpool.Pool {
	t.Helper()
	dbURL := os.Getenv("VAKT_DB_URL")
	if dbURL == "" {
		t.Skip("VAKT_DB_URL not set — dieser Test braucht eine migrierte Postgres (CI setzt sie)")
	}
	pool, err := pgxpool.New(ctx, dbURL)
	require.NoError(t, err)
	// Cleanup statt defer: Cleanups laufen LIFO und NACH den defers, die
	// Fixture-Aufraeumung braucht den Pool also noch (siehe
	// upsert_rawid_arbiter_real_test.go).
	t.Cleanup(pool.Close)
	return pool
}

// countFindings zaehlt die tatsaechlich gespeicherten Funde einer Organisation.
func countFindings(ctx context.Context, t *testing.T, pool *pgxpool.Pool, orgID string) int {
	t.Helper()
	var n int
	require.NoError(t, pool.QueryRow(ctx,
		`SELECT count(*) FROM vb_findings WHERE org_id = $1`, orgID).Scan(&n))
	return n
}

// storedSeverities liest die gespeicherten Schweregrade, sortiert.
func storedSeverities(ctx context.Context, t *testing.T, pool *pgxpool.Pool, orgID string) []string {
	t.Helper()
	rows, err := pool.Query(ctx,
		`SELECT severity FROM vb_findings WHERE org_id = $1`, orgID)
	require.NoError(t, err)
	defer rows.Close()

	var got []string
	for rows.Next() {
		var s string
		require.NoError(t, rows.Scan(&s))
		got = append(got, s)
	}
	require.NoError(t, rows.Err())
	sort.Strings(got)
	return got
}

// batchOf baut n Funde mit den gegebenen Schweregraden. Jeder bekommt eine eigene
// CVE-Kennung, damit die Dedup-Logik sie nicht zusammenfasst — sonst pruefte der
// Test die Zeilenzahl einer Zusammenfuehrung statt die eines Stapels.
func batchOf(assetID string, severities ...string) []Finding {
	out := make([]Finding, 0, len(severities))
	for i, sev := range severities {
		cveCopy := fmt.Sprintf("CVE-2026-%04d", 1000+i) // eindeutig je Zeile
		out = append(out, Finding{
			AssetID:  assetID,
			Title:    "Fund " + sev,
			CVEID:    &cveCopy,
			Severity: sev,
			Status:   "open",
			Scanner:  "trivy",
			Sources:  []string{"trivy"},
		})
	}
	return out
}

// ABNAHME 1 — gruen auf der Baseline.
//
// Ein Stapel mit ausschliesslich bekannten Schweregraden verhaelt sich
// unveraendert: alle Zeilen landen, die Grade stehen unveraendert in der
// Datenbank, und es gibt nichts zu melden.
func TestBatchUpsertFindings_BekannteSchweregradeUnveraendert(t *testing.T) {
	ctx := context.Background()
	pool := severityTestDB(ctx, t)
	orgID, assetID := seedUpsertFixture(ctx, t, pool)
	repo := NewRepository(pool)

	findings := batchOf(assetID, "critical", "high", "medium", "low", "info")

	count, err := repo.BatchUpsertFindings(ctx, orgID, findings)
	require.NoError(t, err, "ein Stapel aus bekannten Schweregraden muss durchlaufen")

	inDB := countFindings(ctx, t, pool, orgID)
	assert.Equal(t, 5, inDB, "alle fuenf Funde muessen in der Datenbank stehen")
	assert.Equal(t, inDB, count, "der gemeldete Zaehler muss der Datenbank entsprechen")

	assert.Equal(t,
		[]string{"critical", "high", "info", "low", "medium"},
		storedSeverities(ctx, t, pool, orgID),
		"bekannte Schweregrade duerfen nicht umgedeutet werden")
}

// ABNAHME 2 — ROT bei echter Regression.
//
// Der Stapel enthaelt `unknown`, den echten Wert aus Trivy und Nuclei. Vor dem
// Fix schlaegt BatchUpsertFindings hier mit SQLSTATE 23514 fehl UND die Datenbank
// bleibt leer — beides wird geprueft, denn der eigentliche Schaden ist nicht die
// eine abgelehnte Zeile, sondern die vier mitgerissenen.
func TestBatchUpsertFindings_UnknownKipptDenStapelNicht(t *testing.T) {
	ctx := context.Background()
	pool := severityTestDB(ctx, t)
	orgID, assetID := seedUpsertFixture(ctx, t, pool)
	repo := NewRepository(pool)

	// Die kritische Zeile steht in der MITTE: So belegt die Zeilenzahl, dass
	// weder die Funde davor noch die dahinter verloren gehen.
	findings := batchOf(assetID, "critical", "high", "unknown", "medium", "low")

	count, err := repo.BatchUpsertFindings(ctx, orgID, findings)
	require.NoError(t, err,
		"ein regulaerer Trivy-/Nuclei-Schweregrad darf den Stapel nicht abbrechen (SQLSTATE 23514)")

	inDB := countFindings(ctx, t, pool, orgID)
	assert.Equal(t, 5, inDB,
		"alle fuenf Funde muessen gespeichert sein — eine abgelehnte Zeile rollt sonst den ganzen Stapel zurueck")
	assert.Equal(t, inDB, count,
		"der gemeldete Zaehler muss der Datenbank entsprechen — ein positiver Zaehler ueber leerer Tabelle ist genau der Defekt aus Migration 243")

	assert.Contains(t, storedSeverities(ctx, t, pool, orgID), "unknown",
		"`unknown` muss als eigener Zustand gespeichert sein, nicht als `info` — sonst weist der Auditbericht eine Bewertung aus, die nie stattgefunden hat")
}

// ABNAHME 2b — der naechste unbekannte Wert kippt ebenfalls nichts.
//
// Selbst mit korrigiertem Wertebereich bleibt die Frage offen, was passiert, wenn
// Trivy morgen einen sechsten Grad einfuehrt. Der Fund wird als `unknown`
// gespeichert — nicht verworfen, nicht umgedeutet in etwas Bewertetes — und der
// Nutzer erfaehrt es ueber den Hinweis am Scan.
func TestBatchUpsertFindings_UnbekannterWertWirdUnknownStattVerworfen(t *testing.T) {
	ctx := context.Background()
	pool := severityTestDB(ctx, t)
	orgID, assetID := seedUpsertFixture(ctx, t, pool)
	repo := NewRepository(pool)

	findings := batchOf(assetID, "high", "catastrophic", "low")

	count, err := repo.BatchUpsertFindings(ctx, orgID, findings)
	require.NoError(t, err, "ein Wert, den niemand kennt, darf den Stapel nicht abbrechen")

	inDB := countFindings(ctx, t, pool, orgID)
	assert.Equal(t, 3, inDB, "der Fund wird umgedeutet, nicht verworfen — still verwerfen ist keine Option")
	assert.Equal(t, inDB, count)

	assert.Equal(t, []string{"high", "low", "unknown"},
		storedSeverities(ctx, t, pool, orgID),
		"ein unbekannter Rohwert wird `unknown` — nicht `medium`, das waere eine erfundene Einstufung")
}

// TestCanonicalSeverityMatchesConstraint faehrt die Menge im Go-Code gegen die
// Menge im Schema.
//
// Ohne diesen Test waere `canonicalSeverity` eine Behauptung ueber die Datenbank,
// die niemand nachprueft — und genau so ist der Defekt entstanden: Der Code kannte
// eine Menge, das Schema eine andere, und gemerkt hat es erst die Produktion.
// Wer hier etwas hinzufuegt, braucht eine Migration, sonst wird dieser Test rot.
func TestCanonicalSeverityMatchesConstraint(t *testing.T) {
	ctx := context.Background()
	pool := severityTestDB(ctx, t)

	var def string
	// orgid-lint: global — Katalogabfrage gegen pg_constraint. Der Systemkatalog
	// kennt keine Mandanten; ein org_id-Filter waere hier gar nicht formulierbar.
	require.NoError(t, pool.QueryRow(ctx, `
		SELECT pg_get_constraintdef(oid)
		FROM pg_constraint
		WHERE conrelid = 'vb_findings'::regclass
		  AND conname  = 'vb_findings_severity_check'`).Scan(&def),
		"der CHECK auf vb_findings.severity muss existieren")

	for sev := range canonicalSeverity {
		assert.Contains(t, def, "'"+sev+"'",
			"canonicalSeverity kennt %q, der CHECK in der Datenbank nicht — Migration fehlt", sev)
	}

	// Gegenrichtung: Der CHECK darf nichts erlauben, was der Code nicht kennt —
	// sonst faellt ein gueltiger Wert stillschweigend auf `unknown`.
	for _, sev := range []string{"critical", "high", "medium", "low", "info", "unknown"} {
		_, known := canonicalSeverity[sev]
		assert.True(t, known, "der CHECK erlaubt %q, canonicalSeverity kennt es nicht", sev)
	}
}
