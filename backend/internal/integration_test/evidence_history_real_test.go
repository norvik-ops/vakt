//go:build integration

package integration_test

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/testcontainers/testcontainers-go/modules/postgres"

	shareddb "github.com/matharnica/vakt/internal/shared/db"
)

// TestEvidenceHistory_WrittenOnEveryChange pins R1-14b-A2.
//
// CLAUDE.md verspricht woertlich "stores versioned evidence". Gebaut waren die
// Tabelle ck_evidence_history (Migration 109), die Spalte ck_evidence.version,
// die Query ListCKEvidenceHistory und die Route GET /evidence/:id/history —
// aber kein einziger Schreiber. version blieb dauerhaft 1, die Tabelle blieb
// leer, der Endpunkt lieferte immer [].
//
// Migration 256 haengt die Versionierung an ck_evidence selbst, damit sie fuer
// jeden Produzenten gilt: manueller Upload, die rund 30 Collector-Stellen, den
// CI-Webhook, den Review-Pfad und die Zuordnung verwaister Auto-Evidenz.
// Der Test fuehrt genau diese Mutationen als reines SQL aus, also auf der
// Ebene, auf der der Trigger sitzt.
//
// Wird Migration 256 zurueckgedreht, faellt schon die erste Zusicherung
// (Genesis-Zeile nach dem INSERT).
func TestEvidenceHistory_WrittenOnEveryChange(t *testing.T) {
	if testing.Short() {
		t.Skip("integration: -short mode")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Minute)
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

	dsn, err := pgC.ConnectionString(ctx, "sslmode=disable")
	require.NoError(t, err)
	require.NoError(t, shareddb.RunMigrations(dsn, migrationsDir(t)))

	pool, err := pgxpool.New(ctx, dsn)
	require.NoError(t, err)
	defer pool.Close()

	var orgID string
	require.NoError(t, pool.QueryRow(ctx, `
		INSERT INTO organizations (name, slug) VALUES ('HistOrg', 'historg')
		RETURNING id::text`).Scan(&orgID))
	fwID := seedFramework(ctx, t, pool, orgID, "ISO 27001")
	ctrlID := seedControl(ctx, t, pool, orgID, fwID, "H-1", "Access", "")

	var uploaderID, reviewerID string
	require.NoError(t, pool.QueryRow(ctx, `
		INSERT INTO users (email, password_hash, display_name)
		VALUES ('uploader@histofix.test', 'x', 'Uploader')
		RETURNING id::text`).Scan(&uploaderID))
	require.NoError(t, pool.QueryRow(ctx, `
		INSERT INTO users (email, password_hash, display_name)
		VALUES ('reviewer@histofix.test', 'x', 'Reviewer')
		RETURNING id::text`).Scan(&reviewerID))

	type histRow struct {
		title      string
		status     string
		changeNote string
		changedBy  *string
	}
	history := func(evidenceID string) []histRow {
		rows, err := pool.Query(ctx, `
			SELECT COALESCE(title,''), COALESCE(status,''), COALESCE(change_note,''),
			       changed_by::text
			  FROM ck_evidence_history
			 WHERE evidence_id = $1::uuid AND org_id = $2::uuid
			 ORDER BY changed_at ASC, change_note ASC`, evidenceID, orgID)
		require.NoError(t, err)
		defer rows.Close()
		var out []histRow
		for rows.Next() {
			var h histRow
			require.NoError(t, rows.Scan(&h.title, &h.status, &h.changeNote, &h.changedBy))
			out = append(out, h)
		}
		require.NoError(t, rows.Err())
		return out
	}
	version := func(evidenceID string) int {
		var v int
		require.NoError(t, pool.QueryRow(ctx,
			`SELECT version FROM ck_evidence WHERE org_id = $1::uuid AND id = $2::uuid`,
			orgID, evidenceID).Scan(&v))
		return v
	}

	// ── Anlegen ──────────────────────────────────────────────────────────────
	var evID string
	require.NoError(t, pool.QueryRow(ctx, `
		INSERT INTO ck_evidence (control_id, org_id, title, description, uploaded_by)
		VALUES ($1::uuid, $2::uuid, 'Backup-Protokoll', 'Q1', $3::uuid)
		RETURNING id::text`, ctrlID, orgID, uploaderID).Scan(&evID))

	h := history(evID)
	require.Len(t, h, 1, "das Anlegen muss die Ursprungsfassung festhalten")
	assert.Equal(t, "Backup-Protokoll", h[0].title)
	assert.Equal(t, "created", h[0].changeNote)
	require.NotNil(t, h[0].changedBy, "changed_by muss beim Anlegen der Hochladende sein")
	assert.Equal(t, uploaderID, *h[0].changedBy)
	assert.Equal(t, 1, version(evID))

	// ── Inhaltliche Aenderung ────────────────────────────────────────────────
	_, err = pool.Exec(ctx, `
		UPDATE ck_evidence SET title = 'Backup-Protokoll Q2'
		 WHERE org_id = $1::uuid AND id = $2::uuid`, orgID, evID)
	require.NoError(t, err)

	h = history(evID)
	require.Len(t, h, 2, "eine inhaltliche Aenderung muss eine Fassung erzeugen")
	assert.Equal(t, "Backup-Protokoll Q2", h[1].title)
	assert.Equal(t, 2, version(evID), "version muss mitzaehlen")

	// ── Freigabe: changed_by muss der Pruefende sein ─────────────────────────
	_, err = pool.Exec(ctx, `
		UPDATE ck_evidence
		   SET status = 'approved', reviewed_by = $1::uuid, reviewed_at = NOW()
		 WHERE org_id = $2::uuid AND id = $3::uuid`, reviewerID, orgID, evID)
	require.NoError(t, err)

	h = history(evID)
	require.Len(t, h, 3, "die Freigabe muss eine Fassung erzeugen")
	assert.Equal(t, "approved", h[2].status)
	require.NotNil(t, h[2].changedBy, "wer freigegeben hat, ist die wichtigste Angabe der Historie")
	assert.Equal(t, reviewerID, *h[2].changedBy)
	assert.Equal(t, 3, version(evID))

	// ── Reine Verwaltungsschreibung darf KEINE Fassung erzeugen ──────────────
	// MarkCKEvidenceExpiryNotified setzt nur expiry_notified_at + updated_at.
	// Ohne diese Abgrenzung wuerde der Ablauf-Cron die Historie mit
	// inhaltsleeren Zeilen fluten und version taeglich hochzaehlen.
	_, err = pool.Exec(ctx, `
		UPDATE ck_evidence SET expiry_notified_at = NOW(), updated_at = NOW()
		 WHERE org_id = $1::uuid AND id = $2::uuid`, orgID, evID)
	require.NoError(t, err)

	assert.Len(t, history(evID), 3, "Ablauf-Benachrichtigung ist keine inhaltliche Aenderung")
	assert.Equal(t, 3, version(evID))

	// ── Zuordnung verwaister Auto-Evidenz ────────────────────────────────────
	// Der CI-Webhook legt Evidenz ohne control_id an; evidence_auto haengt sie
	// spaeter an ein Control. Auch das ist eine inhaltliche Aenderung.
	var autoID string
	require.NoError(t, pool.QueryRow(ctx, `
		INSERT INTO ck_evidence (org_id, title, source, status, auto_source_type, auto_source_ref)
		VALUES ($1::uuid, 'Trivy-Lauf', 'ci_webhook', 'pending', 'ci_webhook', 'run-1')
		RETURNING id::text`, orgID).Scan(&autoID))
	require.Len(t, history(autoID), 1)

	_, err = pool.Exec(ctx, `
		UPDATE ck_evidence SET control_id = $1::uuid, updated_at = NOW()
		 WHERE org_id = $2::uuid AND id = $3::uuid`, ctrlID, orgID, autoID)
	require.NoError(t, err)
	assert.Len(t, history(autoID), 2, "die Zuordnung an ein Control muss festgehalten werden")
	assert.Equal(t, 2, version(autoID))
}
