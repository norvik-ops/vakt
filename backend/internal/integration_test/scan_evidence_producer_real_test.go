//go:build integration

// Copyright (c) 2026 NorvikOps. All rights reserved.
// SPDX-License-Identifier: Elastic-2.0

package integration_test

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/hibiken/asynq"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/require"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/modules/postgres"
	"github.com/testcontainers/testcontainers-go/wait"

	"github.com/matharnica/vakt/internal/modules/vaktcomply"
	"github.com/matharnica/vakt/internal/modules/vaktscan"
	"github.com/matharnica/vakt/internal/services/crossevidence"
	shareddb "github.com/matharnica/vakt/internal/shared/db"
	"github.com/matharnica/vakt/internal/shared/platform/events"
	"github.com/matharnica/vakt/internal/shared/redisopt"
)

// TestScanEvidenceProducer ist die Abnahme zu R1-14b-02.
//
// CLAUDE.md verspricht: "Scanner results flow as compliance evidence into Vakt
// Comply." Die Bruecke war gebaut, hatte aber keinen Erzeuger: das einzige
// finding-created-Ereignis entstand in Service.UpsertFinding, einer Methode mit
// null Produktiv-Aufrufern. Live belegt: ein CSV-Import mit severity=critical
// legte das Finding an (imported: 1), ck_scan_evidence_map blieb bei 0.
//
// Der Test faehrt die GANZE Kette gegen echtes Postgres und echtes Redis:
//
//	echter Importpfad (Service.ImportCSV, NICHT Service.UpsertFinding)
//	  -> Repository (der Punkt, an dem kein Pfad vorbeikommt)
//	  -> Asynq-Task in der Warteschlange "vaktcomply"
//	  -> Consumer (dieselbe Weiche wie cmd/worker/handlers_secvitals.go)
//	  -> vaktcomply.RecordScanFindingEvidence
//	  -> ZEILE in ck_scan_evidence_map, aus der Datenbank gelesen
//
// Ein eingestelltes Ereignis ist ausdruecklich KEIN Nachweis — geprueft wird die
// Zeile beim Empfaenger.
func TestScanEvidenceProducer(t *testing.T) {
	if testing.Short() {
		t.Skip("integration: -short mode")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 4*time.Minute)
	defer cancel()

	pgC, err := postgres.Run(ctx,
		imagePostgres,
		postgres.WithDatabase("vakt_test"),
		postgres.WithUsername("vakt"),
		postgres.WithPassword("vakt"),
		postgres.WithSQLDriver("pgx"),
		postgres.BasicWaitStrategies(),
	)
	if err != nil {
		if strings.Contains(err.Error(), "permission denied") ||
			strings.Contains(err.Error(), "Cannot connect to the Docker daemon") {
			t.Skipf("integration: Docker unavailable (%v)", err)
		}
		t.Fatalf("postgres container: %v", err)
	}
	defer func() { _ = pgC.Terminate(ctx) }()

	redisC, err := testcontainers.GenericContainer(ctx, testcontainers.GenericContainerRequest{
		ContainerRequest: testcontainers.ContainerRequest{
			Image:        imageRedis,
			ExposedPorts: []string{"6379/tcp"},
			WaitingFor:   wait.ForListeningPort("6379/tcp").WithStartupTimeout(60 * time.Second),
		},
		Started: true,
	})
	require.NoError(t, err)
	defer func() { _ = redisC.Terminate(ctx) }()
	rHost, err := redisC.Host(ctx)
	require.NoError(t, err)
	rPort, err := redisC.MappedPort(ctx, "6379/tcp")
	require.NoError(t, err)

	// Bewusst OHNE Datenbanknummer im Pfad: mit Suffix landen Erzeuger und
	// Verbraucher in verschiedenen Redis-Datenbanken, und der Test misst dann
	// jenen Defekt (R1-14b-01) statt dieser Kette.
	redisURL := fmt.Sprintf("redis://%s:%s", rHost, rPort.Port())

	dsn, err := pgC.ConnectionString(ctx, "sslmode=disable")
	require.NoError(t, err)
	require.NoError(t, shareddb.RunMigrations(dsn, migrationsDir(t)))

	pool, err := pgxpool.New(ctx, dsn)
	require.NoError(t, err)
	defer pool.Close()

	// vaktscan liest die Adresse aus derselben Variable wie API und Worker.
	t.Setenv("VAKT_REDIS_URL", redisURL)

	// --- Fixture: Org, Control (Stichwort "vulnerabilit"), Asset ----------
	var orgID, fwID string
	require.NoError(t, pool.QueryRow(ctx, `
		INSERT INTO organizations (name, slug) VALUES ('Acme', 'acme-scan-evidence') RETURNING id::text`).Scan(&orgID))
	require.NoError(t, pool.QueryRow(ctx, `
		INSERT INTO ck_frameworks (org_id, name) VALUES ($1, 'ISO 27001') RETURNING id::text`, orgID).Scan(&fwID))
	_, err = pool.Exec(ctx, `
		INSERT INTO ck_controls (framework_id, org_id, control_id, title, description, domain)
		VALUES ($1::uuid, $2::uuid, 'A.8.8', 'Management of technical vulnerabilities', '', 'Technological')`,
		fwID, orgID)
	require.NoError(t, err)

	var assetID string
	require.NoError(t, pool.QueryRow(ctx, `
		INSERT INTO vb_assets (org_id, name, type) VALUES ($1::uuid, 'prod-api', 'server') RETURNING id::text`,
		orgID).Scan(&assetID))

	// --- Consumer: dieselbe Weiche wie cmd/worker/handlers_secvitals.go ---
	// cmd/worker ist package main und nicht importierbar; die Weiche ist hier
	// nachgebaut, der Empfaenger dahinter ist der echte Dienst.
	complySvc := vaktcomply.NewService(pool)
	mux := asynq.NewServeMux()
	mux.HandleFunc(crossevidence.TaskRecordEvidence, func(c context.Context, task *asynq.Task) error {
		var payload crossevidence.EvidencePayload
		if err := json.Unmarshal(task.Payload(), &payload); err != nil {
			return err
		}
		if payload.ResourceType != events.ResourceTypeFindingCreated {
			return nil
		}
		_, err := complySvc.RecordScanFindingEvidence(c, payload.OrgID, payload.ResourceID, payload.Title)
		return err
	})
	srv := asynq.NewServer(redisopt.AsynqFromURL(redisURL), asynq.Config{
		Concurrency: 2,
		Queues:      map[string]int{crossevidence.Queue: 1},
	})
	require.NoError(t, srv.Start(mux))
	defer srv.Shutdown()

	// --- Ausloeser: ECHTER Importpfad ------------------------------------
	// Service.ImportCSV, nicht Service.UpsertFinding. Genau dieser Aufruf hat
	// den Defekt live belegt.
	scanSvc := vaktscan.NewService(pool, asynq.RedisClientOpt{})
	csv := []byte("title,severity,cve_id,description,cvss_score\n" +
		"Log4Shell RCE,critical,CVE-2021-44228,Remote code execution,10.0\n")

	imported, err := scanSvc.ImportCSV(ctx, orgID, assetID, csv)
	require.NoError(t, err)
	require.Equal(t, 1, imported)

	var findingID string
	require.NoError(t, pool.QueryRow(ctx,
		`SELECT id::text FROM vb_findings WHERE org_id = $1::uuid`, orgID).Scan(&findingID))

	// --- Nachweis: die ZEILE, nicht die Warteschlange ---------------------
	var mapCount int
	require.Eventually(t, func() bool {
		if qErr := pool.QueryRow(context.Background(),
			`SELECT COUNT(*) FROM ck_scan_evidence_map WHERE org_id = $1::uuid AND finding_id = $2`,
			orgID, findingID).Scan(&mapCount); qErr != nil {
			return false
		}
		return mapCount > 0
	}, 60*time.Second, 250*time.Millisecond,
		"ck_scan_evidence_map muss nach einem echten Import eine Zeile tragen — ohne den Fix bleibt sie leer")

	require.Equal(t, 1, mapCount, "genau eine Zuordnung auf das Vulnerability-Control")

	var evCount int
	require.NoError(t, pool.QueryRow(ctx,
		`SELECT COUNT(*) FROM ck_evidence WHERE org_id = $1::uuid AND source = 'automated'`, orgID).Scan(&evCount))
	require.Equal(t, 1, evCount, "die Evidenz selbst muss ebenfalls geschrieben sein, nicht nur die Zuordnung")

	// --- Gegenprobe: erneuter Import derselben Datei ----------------------
	// Der Fund ist dann nicht mehr neu; es darf kein zweites Ereignis und keine
	// zweite Zeile entstehen. Das ist die Entscheidung "nur bei Neuanlage".
	imported, err = scanSvc.ImportCSV(ctx, orgID, assetID, csv)
	require.NoError(t, err)
	require.Equal(t, 1, imported)

	time.Sleep(3 * time.Second)
	require.NoError(t, pool.QueryRow(ctx,
		`SELECT COUNT(*) FROM ck_scan_evidence_map WHERE org_id = $1::uuid`, orgID).Scan(&mapCount))
	require.Equal(t, 1, mapCount, "ein wiedergesehener Fund darf keine zweite Zuordnung erzeugen")
}
