// Copyright (c) 2026 NorvikOps. All rights reserved.
// SPDX-License-Identifier: Elastic-2.0

package vaktscan

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"testing"

	"github.com/google/uuid"
	"github.com/hibiken/asynq"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/require"
)

// R1-RV-01 / R1-F3-N1 — warum dieser Test gegen echtes PostgreSQL laeuft.
//
// Der Defekt bestand aus zwei Haelften, die nur zusammen einen Fix ergeben:
//
//  1. Der SARIF-Import setzte `cve_id` nie. Damit griff der scanner-agnostische
//     Dedup-Index `idx_vb_findings_dedup_cve (org_id, asset_id, cve_id)
//     WHERE cve_id IS NOT NULL` fuer SARIF-Importe nie, und die
//     EPSS-Anreicherung uebersprang die Funde ausdruecklich (scanner.go:
//     "Findings without a CVE are skipped"). Vakt Scan wirbt mit genau diesen
//     beiden Faehigkeiten.
//
//  2. `cve_id` allein zu setzen ist SCHLIMMER als der Ausgangszustand: Der
//     Import schrieb ueber den **raw_id**-Arbiter. Sobald `cve_id` gefuellt ist,
//     bewacht der CVE-Index dieselbe Zeile — und der Arbiter kennt ihn nicht.
//     Zwei Scanner mit derselben CVE auf demselben Asset kollidieren dann mit
//     SQLSTATE 23505, statt zusammengefuehrt zu werden.
//
// Keine dieser Haelften ist statisch sichtbar: `go build`, `go vet` und der
// PREPARE-Gate G5 sehen eine gueltige Query. Die Arbiter-Wahl entscheidet sich
// erst im Planner, die Unique-Verletzung erst beim Schreiben einer zweiten
// Zeile. Deshalb steht dieser Test gegen eine migrierte Datenbank und zaehlt die
// Zeilen **aus der Datenbank**, nicht aus dem zurueckgegebenen Zaehler: In genau
// diesem Projekt hat ein Import-Zaehler schon einmal Erfolg gemeldet, waehrend
// nichts ankam (Migration 243, pgx-Batch-Rollback).
func TestImportSARIF_CVEDedupAcrossScanners(t *testing.T) {
	dbURL := os.Getenv("VAKT_DB_URL")
	if dbURL == "" {
		t.Skip("VAKT_DB_URL not set — dieser Test braucht eine migrierte Postgres (CI setzt sie)")
	}
	ctx := context.Background()

	pool, err := pgxpool.New(ctx, dbURL)
	require.NoError(t, err)
	// Cleanup statt defer: Cleanups laufen NACH den defers, ein `defer
	// pool.Close()` liefe der Fixture-Aufraeumung zuvor. Siehe die ausfuehrliche
	// Begruendung in upsert_rawid_arbiter_real_test.go.
	t.Cleanup(pool.Close)

	orgID, assetA := seedUpsertFixture(ctx, t, pool)
	assetB := seedExtraAsset(ctx, t, pool, orgID, "w2e-asset-b")
	svc := NewService(pool, asynq.RedisClientOpt{})

	countRows := func(where string, args ...any) int {
		t.Helper()
		var n int
		require.NoError(t, pool.QueryRow(ctx,
			"SELECT count(*) FROM vb_findings WHERE "+where, args...).Scan(&n))
		return n
	}

	// ── Abnahme 1: Baseline — ohne CVE-artige ruleId bleibt alles wie bisher ──
	//
	// Semgrep/CodeQL liefern Regel-Kennungen, keine Schwachstellenkennungen.
	// Solche Funde muessen weiterhin ueber den raw_id-Arbiter laufen und
	// `cve_id = NULL` behalten — sonst legt der CVE-Index voellig unverwandte
	// Regeltreffer zusammen.
	semgrepRule := "go.lang.security.audit.xss.no-direct-write-to-responsewriter"
	n, err := svc.ImportSARIF(ctx, orgID, assetA, sarifPayload("Semgrep", semgrepRule))
	require.NoError(t, err, "ein SARIF ohne CVE-artige ruleId muss unveraendert durchlaufen")
	require.Equal(t, 1, n)

	require.Equal(t, 1, countRows("org_id = $1 AND raw_id = $2", orgID, semgrepRule),
		"Regeltreffer wird ueber raw_id abgelegt")
	require.Equal(t, 0, countRows("org_id = $1 AND cve_id IS NOT NULL", orgID),
		"eine Semgrep-Regel-Kennung darf NICHT als CVE geschrieben werden")

	// Zweiter Lauf desselben Reports: raw_id-Arbiter greift, eine Zeile bleibt.
	_, err = svc.ImportSARIF(ctx, orgID, assetA, sarifPayload("Semgrep", semgrepRule))
	require.NoError(t, err)
	require.Equal(t, 1, countRows("org_id = $1 AND raw_id = $2", orgID, semgrepRule),
		"zweimal derselbe Regeltreffer darf keine zweite Zeile anlegen")

	// ── Abnahme 2: die eigentliche Regression ────────────────────────────────
	//
	// Zwei Scanner, dieselbe CVE, dasselbe Asset. Grype haengt der ruleId den
	// Paketnamen an, Trivy nicht — beide muessen auf denselben Schluessel
	// fuehren. Ohne die Arbiter-Umstellung endet der zweite Import hier mit
	// 23505 auf idx_vb_findings_dedup_cve; ohne das Setzen von `cve_id` mit
	// zwei getrennten Zeilen.
	const cve = "CVE-2021-44228"

	_, err = svc.ImportSARIF(ctx, orgID, assetA, sarifPayload("trivy", cve))
	require.NoError(t, err, "erster CVE-Import (trivy)")

	_, err = svc.ImportSARIF(ctx, orgID, assetA, sarifPayload("grype", cve+"-log4j-core"))
	require.NoError(t, err,
		"zweiter Scanner mit derselben CVE muss zusammengefuehrt werden, nicht mit 23505 abbrechen")

	require.Equal(t, 1, countRows("org_id = $1 AND asset_id = $2 AND cve_id = $3", orgID, assetA, cve),
		"zwei Scanner, eine CVE, ein Asset = GENAU EINE Zeile (aus der DB gezaehlt, nicht aus dem Rueckgabewert)")

	// Der zusammengefuehrte Datensatz muss die Zusammenfuehrung auch belegen.
	var occurrences int
	var rawID *string
	var sources []string
	require.NoError(t, pool.QueryRow(ctx,
		`SELECT occurrence_count, raw_id, sources FROM vb_findings
		  WHERE org_id = $1 AND asset_id = $2 AND cve_id = $3`,
		orgID, assetA, cve).Scan(&occurrences, &rawID, &sources))
	require.Equal(t, 2, occurrences, "occurrence_count muss die zweite Sichtung mitzaehlen")
	require.Subset(t, sources, []string{"trivy", "grype"},
		"sources muss beide Werkzeuge tragen — sonst ist die Zusammenfuehrung verlustbehaftet")
	require.Nil(t, rawID,
		"eine CVE-geschluesselte Zeile darf kein raw_id tragen, sonst streiten zwei partielle Unique-Indexe um dieselbe Zeile")

	// ── Abnahme 2b: dieselbe CVE auf einem ANDEREN Asset ─────────────────────
	//
	// Der raw_id-Index laeuft ueber (org_id, raw_id, scanner) — OHNE das Asset.
	// Wuerde die CVE-geschluesselte Zeile weiterhin ein raw_id tragen, schlaege
	// genau hier 23505 zu: Der CVE-Arbiter sieht wegen des anderen Assets keinen
	// Konflikt, der raw_id-Index sehr wohl. Zwei Assets muessen zwei Zeilen
	// ergeben — ein Fund gehoert zu einem Asset.
	_, err = svc.ImportSARIF(ctx, orgID, assetB, sarifPayload("trivy", cve))
	require.NoError(t, err, "dieselbe CVE auf einem zweiten Asset darf nicht mit 23505 abbrechen")
	require.Equal(t, 2, countRows("org_id = $1 AND cve_id = $2", orgID, cve),
		"dieselbe CVE auf zwei Assets = zwei Zeilen")

	// ── Und der Zweck des Ganzen: die EPSS-Anreicherung findet den Fund ──────
	//
	// UpdateEPSSScores sammelt `SELECT DISTINCT cve_id ... WHERE cve_id IS NOT
	// NULL AND cve_id <> ''`. Vor dem Fix lieferte diese Abfrage fuer jeden
	// SARIF-Import die leere Menge — es gab keine Risikopriorisierung.
	require.Equal(t, 2, countRows(
		"org_id = $1 AND cve_id IS NOT NULL AND cve_id <> '' AND status NOT IN ('resolved','false_positive')", orgID),
		"die CVE-Funde muessen fuer die EPSS-Anreicherung sichtbar sein")
}

// TestImportCycloneDX_CVEDedupAcrossScanners belegt, dass der CycloneDX-Zweig
// dieselbe Umstellung braucht — und zwar nicht vorsorglich, sondern weil er
// heute schon bricht: Er setzt `cve_id` bereits (service_import.go), schrieb aber
// ebenfalls ueber den raw_id-Arbiter. Ein CycloneDX-Import einer CVE, die auf
// demselben Asset schon von einem anderen Werkzeug gemeldet wurde, endete
// deshalb mit 23505 — und weil ImportCycloneDX beim ersten Fehler zurueckkehrt,
// blieb der Rest des Berichts ungespeichert.
func TestImportCycloneDX_CVEDedupAcrossScanners(t *testing.T) {
	dbURL := os.Getenv("VAKT_DB_URL")
	if dbURL == "" {
		t.Skip("VAKT_DB_URL not set — dieser Test braucht eine migrierte Postgres (CI setzt sie)")
	}
	ctx := context.Background()

	pool, err := pgxpool.New(ctx, dbURL)
	require.NoError(t, err)
	t.Cleanup(pool.Close)

	orgID, assetID := seedUpsertFixture(ctx, t, pool)
	svc := NewService(pool, asynq.RedisClientOpt{})

	const cve = "CVE-2022-42889"

	_, err = svc.ImportSARIF(ctx, orgID, assetID, sarifPayload("trivy", cve))
	require.NoError(t, err, "SARIF-Import legt die CVE an")

	bom, err := json.Marshal(map[string]any{
		"vulnerabilities": []map[string]any{{
			"id":     cve,
			"detail": "Text4Shell",
			"ratings": []map[string]any{
				{"score": 9.8, "severity": "critical", "method": "CVSSv31"},
			},
		}},
	})
	require.NoError(t, err)

	_, err = svc.ImportCycloneDX(ctx, orgID, assetID, bom)
	require.NoError(t, err,
		"CycloneDX mit einer bereits vorhandenen CVE muss zusammenfuehren, nicht mit 23505 abbrechen")

	var n int
	require.NoError(t, pool.QueryRow(ctx,
		`SELECT count(*) FROM vb_findings WHERE org_id = $1 AND asset_id = $2 AND cve_id = $3`,
		orgID, assetID, cve).Scan(&n))
	require.Equal(t, 1, n, "SARIF + CycloneDX mit derselben CVE auf einem Asset = eine Zeile")
}

// sarifPayload baut einen minimalen, aber schema-echten SARIF-2.1.0-Bericht mit
// je einem Ergebnis pro ruleId.
func sarifPayload(tool string, ruleIDs ...string) []byte {
	results := make([]map[string]any, 0, len(ruleIDs))
	for _, id := range ruleIDs {
		results = append(results, map[string]any{
			"ruleId":  id,
			"level":   "error",
			"message": map[string]any{"text": "Befund aus " + tool},
			"locations": []map[string]any{{
				"physicalLocation": map[string]any{
					"artifactLocation": map[string]any{"uri": "pom.xml"},
					"region":           map[string]any{"startLine": 42},
				},
			}},
		})
	}

	doc := map[string]any{
		"version": "2.1.0",
		"runs": []map[string]any{{
			"tool":    map[string]any{"driver": map[string]any{"name": tool}},
			"results": results,
		}},
	}
	out, err := json.Marshal(doc)
	if err != nil {
		panic(err) // reines Testfixture aus Literalen — kann nicht fehlschlagen
	}
	return out
}

// seedExtraAsset haengt ein zweites Asset an eine bereits angelegte
// Fixture-Organisation. Aufraeumung laeuft ueber ON DELETE CASCADE der
// Organisation, die seedUpsertFixture registriert hat.
func seedExtraAsset(ctx context.Context, t *testing.T, pool *pgxpool.Pool, orgID, name string) string {
	t.Helper()
	assetID := uuid.NewString()
	_, err := pool.Exec(ctx,
		`INSERT INTO vb_assets (id, org_id, name, type) VALUES ($1, $2, $3, 'server')`,
		assetID, orgID, fmt.Sprintf("%s-%s", name, assetID[:8]))
	require.NoError(t, err, "zusaetzliches Fixture-Asset anlegen")
	return assetID
}
