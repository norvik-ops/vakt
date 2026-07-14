//go:build integration

// Copyright (c) 2026 NorvikOps. All rights reserved.
// SPDX-License-Identifier: Elastic-2.0

package integration_test

import (
	"context"
	"testing"
	"time"

	"github.com/hibiken/asynq"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/matharnica/vakt/internal/modules/vaktscan"
)

// Der komplette Scan-Weg gegen echtes Postgres — Ziel-Prüfung, Ausführung,
// Parsen, Dedup-Upsert, Statuswechsel — ohne dass trivy installiert sein muss.
//
// Bis hierher war davon nichts testbar: RunTrivyScan startet einen Unterprozess
// und schreibt in die Datenbank, also konnte kein Test die Funktion überhaupt
// aufrufen. Genau deshalb ist der Statuswechsel nie geprüft worden — und ein Scan,
// der auf „running" stehen bleibt, sieht in der Oberfläche aus wie einer, der noch
// läuft, für immer.

func TestVaktscan_RunTrivyScan_KompletterWeg(t *testing.T) {
	if testing.Short() {
		t.Skip("integration: -short mode")
	}
	pool, orgID, cleanup := bootPostgresWithOrg(t)
	defer cleanup()
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	defer cancel()

	// Zwei Eigenheiten des Ziel-Guards, die dieser Test nebenbei festhält:
	//
	//  1. Der SSRF-Guard behandelt einen fehlgeschlagenen DNS-Lookup fail-safe als
	//     „privat". Ein Image-Name ist kein Hostname und löst nie auf — ein Operator,
	//     der seine eigenen Images scannen will, MUSS also dieses Flag setzen.
	//  2. Der Argument-Injection-Guard lehnt jedes Ziel mit `/` ab. Ein Image mit
	//     Registry- oder Namespace-Präfix (`ghcr.io/foo/bar:1.0`, `library/nginx`)
	//     fällt damit heraus; nur bare Namen wie `nginx:1.21` kommen durch. Das ist
	//     bestehendes, sicherheitsmotiviertes Verhalten und hier NICHT geändert —
	//     aber es ist eine Einschränkung, die niemand aufgeschrieben hatte.
	t.Setenv("VAKT_SCAN_ALLOW_PRIVATE", "true")

	svc := vaktscan.NewService(pool, asynq.RedisClientOpt{})
	repo := vaktscan.NewRepository(pool)

	asset, err := svc.CreateAsset(ctx, orgID, "", vaktscan.CreateAssetInput{
		Name: "nginx", Type: "container", Criticality: "high",
	})
	require.NoError(t, err)

	scan, err := svc.TriggerScan(ctx, orgID, asset.ID, vaktscan.CreateScanInput{
		Scanner:   "trivy",
		TargetURL: "nginx:1.21",
	})
	require.NoError(t, err)

	// Der Scanner wird durch eine aufgezeichnete Trivy-Ausgabe ersetzt. Alles
	// andere ist echt: dieselbe Funktion, dieselbe Datenbank, dieselben Queries.
	restore := vaktscan.SetScanRunnerForTest(func(_ context.Context, name string, _ ...string) ([]byte, error) {
		assert.Equal(t, "trivy", name)
		return []byte(`{"Results":[{"Target":"nginx:1.21","Vulnerabilities":[
			{"VulnerabilityID":"CVE-2021-3711","Title":"openssl overflow","Severity":"CRITICAL",
			 "CVSS":{"nvd":{"V3Score":9.8}}},
			{"VulnerabilityID":"CVE-2021-3712","Title":"openssl read overrun","Severity":"MEDIUM",
			 "CVSS":{"nvd":{"V3Score":7.4}}}
		]}]}`), nil
	})
	defer restore()

	require.NoError(t, vaktscan.RunTrivyScan(ctx, pool, vaktscan.ScanPayload{
		ScanID:    scan.ID,
		OrgID:     orgID,
		AssetID:   asset.ID,
		AssetName: "nginx",
		Scanner:   "trivy",
		TargetURL: "nginx:1.21",
	}))

	// Die Funde sind in der Datenbank — und zwar dedupliziert über den echten
	// Upsert-Weg, nicht bloß eingefügt.
	findings, err := repo.ListFindings(ctx, orgID, vaktscan.FindingFilter{})
	require.NoError(t, err)
	require.Len(t, findings, 2)

	byCVE := map[string]vaktscan.Finding{}
	for _, f := range findings {
		require.NotNil(t, f.CVEID)
		byCVE[*f.CVEID] = f
	}
	crit := byCVE["CVE-2021-3711"]
	assert.Equal(t, "critical", crit.Severity)
	require.NotNil(t, crit.CVSSScore)
	assert.InDelta(t, 9.8, *crit.CVSSScore, 0.001)
	require.NotNil(t, crit.RiskScore, "ohne Risikowert kann nichts priorisiert werden")
	assert.Equal(t, "trivy", crit.Scanner)

	// Der Scan MUSS auf `completed` stehen und seine Funde zählen. Bliebe er auf
	// `running`, sähe die Oberfläche einen Scan, der ewig läuft.
	updated, err := repo.GetScan(ctx, orgID, scan.ID)
	require.NoError(t, err)
	assert.Equal(t, "completed", updated.Status)
	assert.Equal(t, 2, updated.FindingCount)

	// Derselbe Scan noch einmal: Die Dedup-Regel darf die Funde nicht verdoppeln.
	require.NoError(t, vaktscan.RunTrivyScan(ctx, pool, vaktscan.ScanPayload{
		ScanID: scan.ID, OrgID: orgID, AssetID: asset.ID,
		AssetName: "nginx", Scanner: "trivy", TargetURL: "nginx:1.21",
	}))
	findings, err = repo.ListFindings(ctx, orgID, vaktscan.FindingFilter{})
	require.NoError(t, err)
	assert.Len(t, findings, 2, "ein zweiter Scan desselben Images findet dieselben Lücken — nicht doppelt so viele")
}

// TestVaktscan_RunTrivyScan_ScannerFehlerMarkiertDenScanAlsGescheitert hält fest,
// was passiert, wenn der Scanner NICHT läuft.
//
// Das ist der wichtigere Fall: Ein Scan, der scheitert und trotzdem als
// abgeschlossen dasteht, meldet null Funde — und null Funde ist von einem sauberen
// System nicht zu unterscheiden. Der Fehler muss sichtbar bleiben.
func TestVaktscan_RunTrivyScan_ScannerFehlerMarkiertDenScanAlsGescheitert(t *testing.T) {
	if testing.Short() {
		t.Skip("integration: -short mode")
	}
	pool, orgID, cleanup := bootPostgresWithOrg(t)
	defer cleanup()
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	defer cancel()

	t.Setenv("VAKT_SCAN_ALLOW_PRIVATE", "true")

	svc := vaktscan.NewService(pool, asynq.RedisClientOpt{})
	repo := vaktscan.NewRepository(pool)

	asset, err := svc.CreateAsset(ctx, orgID, "", vaktscan.CreateAssetInput{
		Name: "nginx", Type: "container", Criticality: "high",
	})
	require.NoError(t, err)
	scan, err := svc.TriggerScan(ctx, orgID, asset.ID, vaktscan.CreateScanInput{
		Scanner: "trivy", TargetURL: "nginx:1.21",
	})
	require.NoError(t, err)

	restore := vaktscan.SetScanRunnerForTest(func(context.Context, string, ...string) ([]byte, error) {
		return nil, assert.AnError // trivy fehlt, stürzt ab, oder das Image gibt es nicht
	})
	defer restore()

	err = vaktscan.RunTrivyScan(ctx, pool, vaktscan.ScanPayload{
		ScanID: scan.ID, OrgID: orgID, AssetID: asset.ID,
		AssetName: "nginx", Scanner: "trivy", TargetURL: "nginx:1.21",
	})
	require.Error(t, err, "ein gescheiterter Scanner darf kein Erfolg sein")

	updated, err := repo.GetScan(ctx, orgID, scan.ID)
	require.NoError(t, err)
	assert.Equal(t, "failed", updated.Status,
		"ein gescheiterter Scan muss als gescheitert dastehen — sonst meldet er null Funde, und null Funde sieht aus wie ein sauberes System")
	assert.NotEmpty(t, updated.ErrorMessage, "der Grund muss beim Scan stehen, nicht nur im Log")
}

// TestVaktscan_RunTrivyScan_AbgelehntesZielLaesstDenScanNichtHaengen hält den Fund
// fest, den der obige Test ausgelöst hat.
//
// Der Status wird ganz zu Beginn auf "running" gesetzt. Lehnt der SSRF-Guard das
// Ziel danach ab, kehrte die Funktion zurück, OHNE den Status wieder zu verlassen —
// der Scan blieb für immer auf "running" stehen. In der Oberfläche dreht sich damit
// ein Spinner ohne Ende, und der Grund der Ablehnung steht nur im Log, wo ihn
// niemand sucht. Ein hängender Scan ist die dritte Variante derselben Lüge: Er sagt
// nicht „abgelehnt", er sagt „gleich".
func TestVaktscan_RunTrivyScan_AbgelehntesZielLaesstDenScanNichtHaengen(t *testing.T) {
	if testing.Short() {
		t.Skip("integration: -short mode")
	}
	pool, orgID, cleanup := bootPostgresWithOrg(t)
	defer cleanup()
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	defer cancel()

	svc := vaktscan.NewService(pool, asynq.RedisClientOpt{})
	repo := vaktscan.NewRepository(pool)

	asset, err := svc.CreateAsset(ctx, orgID, "", vaktscan.CreateAssetInput{
		Name: "intern", Type: "server", Criticality: "high",
	})
	require.NoError(t, err)
	scan, err := svc.TriggerScan(ctx, orgID, asset.ID, vaktscan.CreateScanInput{
		Scanner: "trivy", TargetURL: "127.0.0.1",
	})
	require.NoError(t, err)

	// Kein VAKT_SCAN_ALLOW_PRIVATE: Der Guard muss ablehnen — und den Scan abschließen.
	runnerLief := false
	restore := vaktscan.SetScanRunnerForTest(func(context.Context, string, ...string) ([]byte, error) {
		runnerLief = true
		return []byte(`{"Results":[]}`), nil
	})
	defer restore()

	err = vaktscan.RunTrivyScan(ctx, pool, vaktscan.ScanPayload{
		ScanID: scan.ID, OrgID: orgID, AssetID: asset.ID,
		AssetName: "intern", Scanner: "trivy", TargetURL: "127.0.0.1",
	})
	require.Error(t, err, "ein Loopback-Ziel muss abgelehnt werden")
	assert.False(t, runnerLief, "der Scanner darf gar nicht erst starten")

	updated, err := repo.GetScan(ctx, orgID, scan.ID)
	require.NoError(t, err)
	assert.Equal(t, "failed", updated.Status,
		"ein abgelehnter Scan muss abgeschlossen sein — sonst dreht sich der Spinner für immer")
	assert.Contains(t, updated.ErrorMessage, "private or loopback",
		"der Grund gehört an den Scan, nicht nur ins Log")
}

// TestVaktscan_BatchUpsert_MehrereFundeUeberMehrereAssets nagelt den schwersten
// Fund dieser Runde fest.
//
// vb_findings hat drei PARTIELLE Unique-Indexe (Migration 120), und die Migration
// schreibt selbst dazu: „partial, weil die Spalten NULL sein dürfen und mehrere
// NULL-Werte erlaubt sein müssen". Der Go-Code schrieb aber nie NULL, sondern den
// Leerstring — und ” ist NOT NULL. Damit griffen die Indexe für JEDEN Fund:
//
//   - template-Index (org, asset, scanner, template_id): zwei Trivy-Funde auf
//     demselben Asset kollidieren.
//   - rawid-Index (org, raw_id, scanner) — OHNE Asset: eine Organisation konnte
//     genau EINEN Trivy-Fund halten, über alle Assets hinweg.
//
// Weil pgx einen Batch in eine implizite Transaktion legt, riss die kollidierende
// Zeile den ganzen Batch mit: Der Scan meldete „abgeschlossen, 1 Fund" und
// speicherte NULL Funde.
func TestVaktscan_BatchUpsert_MehrereFundeUeberMehrereAssets(t *testing.T) {
	if testing.Short() {
		t.Skip("integration: -short mode")
	}
	pool, orgID, cleanup := bootPostgresWithOrg(t)
	defer cleanup()
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	defer cancel()

	svc := vaktscan.NewService(pool, asynq.RedisClientOpt{})
	repo := vaktscan.NewRepository(pool)

	assetA, err := svc.CreateAsset(ctx, orgID, "", vaktscan.CreateAssetInput{
		Name: "web-01", Type: "server", Criticality: "high",
	})
	require.NoError(t, err)
	assetB, err := svc.CreateAsset(ctx, orgID, "", vaktscan.CreateAssetInput{
		Name: "web-02", Type: "server", Criticality: "high",
	})
	require.NoError(t, err)

	mk := func(assetID, cve string) vaktscan.Finding {
		c := cve
		return vaktscan.Finding{
			OrgID: orgID, AssetID: assetID, CVEID: &c,
			Title: cve, Severity: "high", Status: "open",
			Scanner: "trivy", Sources: []string{"trivy"},
			// Kein TemplateID, kein RawID — genau der Normalfall bei Trivy.
		}
	}

	// Zwei Funde auf EINEM Asset: früher Kollision auf dem template-Index.
	count, err := repo.BatchUpsertFindings(ctx, orgID, []vaktscan.Finding{
		mk(assetA.ID, "CVE-2026-0001"),
		mk(assetA.ID, "CVE-2026-0002"),
	})
	require.NoError(t, err)
	assert.Equal(t, 2, count)

	// Ein Fund auf einem ZWEITEN Asset: früher Kollision auf dem rawid-Index, der
	// das Asset gar nicht enthält — eine Org konnte genau einen Trivy-Fund halten.
	count, err = repo.BatchUpsertFindings(ctx, orgID, []vaktscan.Finding{
		mk(assetB.ID, "CVE-2026-0003"),
	})
	require.NoError(t, err)
	assert.Equal(t, 1, count)

	findings, err := repo.ListFindings(ctx, orgID, vaktscan.FindingFilter{})
	require.NoError(t, err)
	assert.Len(t, findings, 3,
		"drei Funde wurden gemeldet, drei müssen in der Datenbank stehen — der gemeldete Zähler war jahrelang eine Behauptung, kein Ergebnis")

	// Und die Dedup-Regel greift weiterhin: derselbe Fund noch einmal verdoppelt nicht.
	count, err = repo.BatchUpsertFindings(ctx, orgID, []vaktscan.Finding{
		mk(assetA.ID, "CVE-2026-0001"),
	})
	require.NoError(t, err)
	assert.Equal(t, 1, count)

	findings, err = repo.ListFindings(ctx, orgID, vaktscan.FindingFilter{})
	require.NoError(t, err)
	assert.Len(t, findings, 3, "derselbe Fund ein zweites Mal ist kein neuer Fund")
}
