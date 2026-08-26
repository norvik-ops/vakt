// Copyright (c) 2026 NorvikOps. All rights reserved.
// SPDX-License-Identifier: Elastic-2.0

package vaktscan

import (
	"context"
	"os"
	"sync"
	"testing"
	"time"

	"github.com/hibiken/asynq"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/require"

	"github.com/matharnica/vakt/internal/shared/platform/webhooks"
)

// R1-14b-02: Der einzige Erzeuger des finding-created-Ereignisses war
// Service.UpsertFinding — eine Methode mit null Produktiv-Aufrufern. Die echten
// Pfade (Scanner ueber BatchUpsertFindings, Importe ueber UpsertImportedFinding)
// gingen an ihr vorbei, also entstand nie ein Ereignis und nie eine Zeile in
// ck_scan_evidence_map.
//
// Dieser Test loest die Ereignisse ueber ECHTE Pfade aus — ImportCSV (Service,
// stellvertretend fuer SARIF/CycloneDX/CSV/Wazuh) und BatchUpsertFindings
// (stellvertretend fuer Trivy/Nuclei/OpenVAS) — und nie ueber Service.UpsertFinding.
// Die Kette bis zur Datenbankzeile prueft der Schwestertest
// internal/integration_test/scan_evidence_producer_real_test.go.

// captureSink haelt fest, was das Repository als NEU meldet.
type captureSink struct {
	mu    sync.Mutex
	calls [][]Finding
}

func (c *captureSink) FindingsCreated(_ context.Context, _ string, created []Finding) {
	c.mu.Lock()
	defer c.mu.Unlock()
	cp := make([]Finding, len(created))
	copy(cp, created)
	c.calls = append(c.calls, cp)
}

func (c *captureSink) flat() []Finding {
	c.mu.Lock()
	defer c.mu.Unlock()
	var out []Finding
	for _, batch := range c.calls {
		out = append(out, batch...)
	}
	return out
}

// waitFor wartet, bis genau n Ereignisse angekommen sind. Der Versand laeuft
// bewusst nebenlaeufig (safego.Run), damit kein Importpfad langsamer wird —
// ein synchrones Auslesen waere hier eine Race-Lotterie.
func (c *captureSink) waitFor(t *testing.T, n int, msg string) []Finding {
	t.Helper()
	require.Eventually(t, func() bool { return len(c.flat()) >= n },
		5*time.Second, 10*time.Millisecond, msg)
	// Kurz nachfassen: haetten wir zu VIELE gemeldet, faellt das sonst durch.
	time.Sleep(150 * time.Millisecond)
	got := c.flat()
	require.Len(t, got, n, msg)
	return got
}

// waitStable prueft, dass NICHT mehr als n Ereignisse ankommen.
func (c *captureSink) waitStable(t *testing.T, n int, msg string) {
	t.Helper()
	time.Sleep(300 * time.Millisecond)
	require.Len(t, c.flat(), n, msg)
}

// fakeWebhookTrigger ersetzt den echten Webhook-Versand. Ein echter
// httptest-Server taugt hier nicht: der SSRF-Schutz des Webhook-Dienstes lehnt
// Loopback-Ziele ab, der Versand kaeme also nie an.
type fakeWebhookTrigger struct {
	mu     sync.Mutex
	events []string
}

func (f *fakeWebhookTrigger) TriggerEvent(_ context.Context, _, eventType string, _ any) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.events = append(f.events, eventType)
}

func (f *fakeWebhookTrigger) count() int {
	f.mu.Lock()
	defer f.mu.Unlock()
	return len(f.events)
}

func TestFindingCreatedEmittedOnRealPaths(t *testing.T) {
	dbURL := os.Getenv("VAKT_DB_URL")
	if dbURL == "" {
		t.Skip("VAKT_DB_URL not set — dieser Test braucht eine migrierte Postgres (CI setzt sie)")
	}
	ctx := context.Background()

	pool, err := pgxpool.New(ctx, dbURL)
	require.NoError(t, err)
	t.Cleanup(pool.Close)

	orgID, assetID := seedUpsertFixture(ctx, t, pool)

	sink := &captureSink{}
	svc := NewService(pool, asynq.RedisClientOpt{})
	svc.repo.sinks = []findingCreatedSink{sink}

	// ---------------------------------------------------------------------
	// Pfad 1: ImportCSV — der Importweg, ueber den der Defekt live belegt wurde.
	// ---------------------------------------------------------------------
	csv := []byte("title,severity,cve_id,description,cvss_score\n" +
		"Log4Shell,critical,CVE-2021-44228,RCE,10.0\n")

	n, err := svc.ImportCSV(ctx, orgID, assetID, csv)
	require.NoError(t, err)
	require.Equal(t, 1, n)

	got := sink.waitFor(t, 1, "ein neuer Fund aus ImportCSV muss GENAU ein finding-created-Ereignis erzeugen — ohne den Fix sind es null")
	require.Equal(t, "critical", got[0].Severity)
	require.NotEmpty(t, got[0].ID, "das Ereignis braucht die finding_id, sonst kann die Bruecke keine Zeile schreiben")

	// ---------------------------------------------------------------------
	// Entscheidung zu Aktualisierungen: derselbe Fund erneut importiert ist
	// KEIN neues Ereignis. Er zaehlt nur hoch (occurrence_count 1 -> 2).
	// ---------------------------------------------------------------------
	n, err = svc.ImportCSV(ctx, orgID, assetID, csv)
	require.NoError(t, err)
	require.Equal(t, 1, n)
	sink.waitStable(t, 1, "ein wiedergesehener Fund darf KEIN zweites Ereignis erzeugen")

	// ---------------------------------------------------------------------
	// Pfad 2: BatchUpsertFindings — der Scanner-Pfad (Trivy/Nuclei/OpenVAS).
	// ---------------------------------------------------------------------
	batchSink := &captureSink{}
	repo := NewRepository(pool)
	repo.sinks = []findingCreatedSink{batchSink}

	batch := []Finding{
		{AssetID: assetID, Title: "Nuclei A", Severity: "high", Status: "open", Scanner: "nuclei", TemplateID: "tpl-a"},
		{AssetID: assetID, Title: "Nuclei B", Severity: "medium", Status: "open", Scanner: "nuclei", TemplateID: "tpl-b"},
	}
	count, err := repo.BatchUpsertFindings(ctx, orgID, batch)
	require.NoError(t, err)
	require.Equal(t, 2, count, "der Zaehler muss weiterhin aus der Datenbank kommen (GB-2)")
	batchSink.waitFor(t, 2, "beide neuen Funde muessen gemeldet werden — auch der mittlere Schweregrad; die critical/high-Schwelle zieht erst der Compliance-Sink")

	// Massenimport-Entscheidung, Gegenprobe: derselbe Batch erneut erzeugt
	// NULL Ereignisse, weil kein Fund neu ist.
	count, err = repo.BatchUpsertFindings(ctx, orgID, batch)
	require.NoError(t, err)
	require.Equal(t, 2, count)
	batchSink.waitStable(t, 2, "ein Re-Scan unveraenderter Funde darf keine Ereignisse nachlegen")

	// ---------------------------------------------------------------------
	// Webhook-Pfad: haengt am selben Ereignis und war ebenso tot.
	// ---------------------------------------------------------------------
	whSvc := NewService(pool, asynq.RedisClientOpt{})
	require.Len(t, whSvc.repo.sinks, 1, "frisch gebaut haengt nur die Compliance-Bruecke")
	whSvc.WithWebhooks(webhooks.NewWebhookService(pool, nil))
	require.Len(t, whSvc.repo.sinks, 2, "WithWebhooks MUSS den finding.created-Sink registrieren")

	fake := &fakeWebhookTrigger{}
	whSvc.webhookSvc = fake
	whSvc.repo.sinks = []findingCreatedSink{webhookSink{svc: whSvc}}

	csv2 := []byte("title,severity,cve_id,description,cvss_score\n" +
		"Spring4Shell,high,CVE-2022-22965,RCE,9.8\n")
	_, err = whSvc.ImportCSV(ctx, orgID, assetID, csv2)
	require.NoError(t, err)

	// triggerWebhook laeuft ueber safego.Run; auf die Goroutine warten.
	require.Eventually(t, func() bool { return fake.count() == 1 },
		5*time.Second, 20*time.Millisecond,
		"ein neuer Fund aus einem echten Importpfad muss den finding.created-Webhook ausloesen")
}
