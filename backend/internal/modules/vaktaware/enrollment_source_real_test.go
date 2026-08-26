// Copyright (c) 2026 NorvikOps. All rights reserved.
// SPDX-License-Identifier: Elastic-2.0

package vaktaware

import (
	"context"
	"os"
	"regexp"
	"sort"
	"testing"

	"github.com/jackc/pgx/v5"
	"github.com/stretchr/testify/require"
)

// Die I4-Sperrklinke gegen R1-35-01: die Abbildung zwischen zwei Wertemengen
// wird GEGEN DAS SCHEMA geprüft, nicht gegen eine zweite Liste im Testcode.
//
// Der ursprüngliche Defekt bestand darin, dass zwei Wertemengen für dasselbe
// gehalten wurden. Ein Test, der die erlaubten Werte im Testcode aufzählt,
// wiederholt genau diese Annahme und wäre die ganze Zeit grün gewesen. Der
// Test liest deshalb beide Mengen aus pg_constraint — der einzigen Stelle, die
// bei einer Migration zwangsläufig mitwandert.
//
// Ausführen mit:
//
//	VAKT_DB_URL=... go test ./internal/modules/vaktaware/ -run TestEnrollmentSource

// checkLiterals liest die erlaubten Zeichenketten eines
// `CHECK (spalte IN ('a','b',...))`-Constraints aus dem Katalog.
func checkLiterals(t *testing.T, conn *pgx.Conn, table, column string) []string {
	t.Helper()
	ctx := context.Background()

	var def string
	err := conn.QueryRow(ctx, `
		SELECT pg_get_constraintdef(c.oid)
		FROM pg_constraint c
		JOIN pg_class t ON t.oid = c.conrelid
		JOIN pg_attribute a ON a.attrelid = t.oid AND a.attnum = ANY (c.conkey)
		WHERE c.contype = 'c' AND t.relname = $1 AND a.attname = $2`,
		table, column).Scan(&def)
	require.NoError(t, err, "kein CHECK-Constraint auf %s.%s gefunden — Nenner falsch", table, column)

	lits := regexp.MustCompile(`'([^']+)'`).FindAllStringSubmatch(def, -1)
	require.NotEmpty(t, lits, "CHECK-Definition ohne Zeichenketten-Literale: %s", def)

	out := make([]string, 0, len(lits))
	for _, m := range lits {
		out = append(out, m[1])
	}
	sort.Strings(out)
	return out
}

// TestEnrollmentSourceMappingMatchesSchema ist die eigentliche Sperrklinke.
//
// Drei Aussagen:
//
//	(A) Die Abbildung ist TOTAL über die Quell-Wertemenge: jeder trigger_type,
//	    den das Schema erlaubt, hat einen Zielwert. Ein neuer Auslösertyp ohne
//	    Eintrag macht den Test rot, statt erst beim INSERT auf 23514 zu laufen.
//	(B) Jeder erzeugte Zielwert liegt IN der Ziel-Wertemenge. Das ist die
//	    Aussage, die R1-35-01 verletzt hat.
//	(C) Die beiden Mengen sind nachweislich VERSCHIEDEN. Wären sie identisch,
//	    wäre (A)+(B) trivial erfüllt und der Test würde nichts mehr aussagen —
//	    diese Prüfung hält fest, warum es die Abbildung überhaupt gibt.
func TestEnrollmentSourceMappingMatchesSchema(t *testing.T) {
	dbURL := os.Getenv("VAKT_DB_URL")
	if dbURL == "" {
		t.Skip("VAKT_DB_URL not set — needs a migrated Postgres (CI sets it)")
	}
	ctx := context.Background()
	conn, err := pgx.Connect(ctx, dbURL)
	require.NoError(t, err)
	defer func() { _ = conn.Close(ctx) }()

	triggerTypes := checkLiterals(t, conn, "sr_enrollment_rules", "trigger_type")
	sources := checkLiterals(t, conn, "sr_campaign_enrollments", "source")

	require.NotEmpty(t, triggerTypes, "Nenner leer")
	require.NotEmpty(t, sources, "Nenner leer")
	t.Logf("geprüft: %d Auslösertypen %v gegen %d Herkunftswerte %v",
		len(triggerTypes), triggerTypes, len(sources), sources)

	inSources := make(map[string]bool, len(sources))
	for _, s := range sources {
		inSources[s] = true
	}

	// (A) + (B)
	for _, tt := range triggerTypes {
		got, err := enrollmentSourceFor(tt)
		require.NoError(t, err,
			"trigger_type %q ist im Schema erlaubt, hat aber keine Abbildung auf einen "+
				"sr_campaign_enrollments.source-Wert — jeder Auto-INSERT dafür würde mit 23514 abgewiesen", tt)
		require.True(t, inSources[got],
			"trigger_type %q wird auf %q abgebildet, was der CHECK-Constraint nicht erlaubt (%v)",
			tt, got, sources)
	}

	// (C) Nicht-Vakuität: die Mengen sind wirklich disjunkt genug, dass eine
	// Abbildung nötig ist. Kein einziger Auslösertyp darf direkt als Herkunft
	// durchgehen — genau das hat der Code vorher getan.
	for _, tt := range triggerTypes {
		require.False(t, inSources[tt],
			"trigger_type %q ist zugleich ein gültiger source-Wert — dann wäre die Abbildung "+
				"überflüssig und dieser Test sagt nichts mehr aus", tt)
	}
}

// TestEnrollmentSourceForRejectsUnknownTrigger hält die Fehlerrichtung fest:
// ein unbekannter Auslösertyp wird an der Naht abgewiesen und nicht als
// erfundener Wert an die Datenbank durchgereicht.
func TestEnrollmentSourceForRejectsUnknownTrigger(t *testing.T) {
	_, err := enrollmentSourceFor("employee_promoted")
	require.Error(t, err)
	require.Contains(t, err.Error(), "employee_promoted",
		"die Fehlermeldung muss den abgelehnten Wert nennen")
}
