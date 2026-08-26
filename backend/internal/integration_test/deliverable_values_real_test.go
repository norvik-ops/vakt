//go:build integration

package integration_test

import (
	"bytes"
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/labstack/echo/v4"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/testcontainers/testcontainers-go/modules/postgres"
	"github.com/xuri/excelize/v2"

	"github.com/matharnica/vakt/internal/modules/vaktcomply"
	shareddb "github.com/matharnica/vakt/internal/shared/db"
)

// TestDeliverableValues_FrameworkReadinessAndAssetLink pinnt zwei Befunde, die
// beide dieselbe Form haben: die Daten stehen in der Datenbank, der Leseweg
// wirft sie weg.
//
// R1-18-D3 — GET /vaktcomply/frameworks (und das Auditor-Portal darauf) lieferte
// readiness_score nie mit. Das Feld traegt `omitempty`, blieb auf 0 und fiel
// damit ganz aus der Antwort; der externe Auditor sah "Bereitschaft %" ohne
// Zahl — die Kennzahl, wegen der er das Portal oeffnet.
//
// N75-2 — PATCH .../asset-link setzt vb_asset_id (live 200, Spalte gesetzt),
// das anschliessende GET lieferte den Soft-Link nicht: protectionNeedFromRow
// selektierte die Spalte und verwarf sie.
//
// Wird einer der beiden Fixes zurueckgedreht, faellt der zugehoerige Untertest.
func TestDeliverableValues_FrameworkReadinessAndAssetLink(t *testing.T) {
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
		INSERT INTO organizations (name, slug) VALUES ('DelivOrg', 'delivorg')
		RETURNING id::text`).Scan(&orgID))

	svc := vaktcomply.NewService(pool)

	t.Run("R1-18-D3 readiness_score in der Framework-Liste", func(t *testing.T) {
		fwA := seedFramework(ctx, t, pool, orgID, "ISO 27001")
		fwB := seedFramework(ctx, t, pool, orgID, "NIS2")

		// ISO: 2 von 4 anwendbaren umgesetzt, 1 in Arbeit, 1 nicht anwendbar
		// -> (2 + 0,5) / 3 * 100 = 83,33
		seedControl(ctx, t, pool, orgID, fwA, "A-1", "Access", "implemented")
		seedControl(ctx, t, pool, orgID, fwA, "A-2", "Access", "implemented")
		seedControl(ctx, t, pool, orgID, fwA, "A-3", "Access", "in_progress")
		naID := seedControl(ctx, t, pool, orgID, fwA, "A-4", "Access", "")
		_, err := pool.Exec(ctx,
			`UPDATE ck_controls SET not_applicable = true WHERE org_id = $1::uuid AND id = $2::uuid`,
			orgID, naID)
		require.NoError(t, err)

		// NIS2: nichts umgesetzt -> 0. Der Untertest darf sich darauf nicht
		// stuetzen (0 ist auch der kaputte Wert), er dient nur der Abgrenzung.
		seedControl(ctx, t, pool, orgID, fwB, "B-1", "Ops", "")

		frameworks, err := svc.Policy.ListFrameworks(ctx, orgID)
		require.NoError(t, err)
		byName := map[string]float64{}
		for _, f := range frameworks {
			byName[f.Name] = f.ReadinessScore
		}
		require.Contains(t, byName, "ISO 27001")
		assert.InDelta(t, 83.33, byName["ISO 27001"], 0.01,
			"readiness_score fehlte in der Framework-Antwort — das Auditor-Portal "+
				"zeigte 'Bereitschaft %%' ohne Zahl")
		assert.InDelta(t, 0.0, byName["NIS2"], 0.01,
			"ein Framework ohne Umsetzung muss 0 bleiben — sonst rechnet der Fix nur irgendwas")
	})

	t.Run("R1-20-A11 Controls-XLSX zeigt Framework-Name und Owner", func(t *testing.T) {
		fwID := seedFramework(ctx, t, pool, orgID, "BSI IT-Grundschutz")
		ctrlID := seedControl(ctx, t, pool, orgID, fwID, "X-1", "Access", "implemented")
		_, err := pool.Exec(ctx,
			`UPDATE ck_controls SET owner = 'Frau Sørensen', last_reviewed_by = 'Pruefer Nord'
			  WHERE org_id = $1::uuid AND id = $2::uuid`, orgID, ctrlID)
		require.NoError(t, err)

		h := vaktcomply.NewHandler(vaktcomply.NewService(pool))
		e := echo.New()
		req := httptest.NewRequest(http.MethodGet, "/?framework_id="+fwID, nil)
		rec := httptest.NewRecorder()
		c := e.NewContext(req, rec)
		c.Set("org_id", orgID)
		require.NoError(t, h.ExportControlsXLSX(c))
		require.Equal(t, http.StatusOK, rec.Code)

		f, err := excelize.OpenReader(bytes.NewReader(rec.Body.Bytes()))
		require.NoError(t, err)
		defer func() { _ = f.Close() }()
		rows, err := f.GetRows("Controls")
		require.NoError(t, err)
		require.Len(t, rows, 2, "Kopfzeile + ein Control")
		require.Equal(t, []string{"Title", "Framework", "Status", "Owner", "Due Date"}, rows[0])

		assert.Equal(t, "BSI IT-Grundschutz", rows[1][1],
			"die Spalte Framework enthielt eine UUID statt des Namens")
		assert.NotEqual(t, fwID, rows[1][1], "die UUID darf nicht mehr durchschlagen")
		assert.Equal(t, "Frau Sørensen", rows[1][3],
			"die Spalte Owner las last_reviewed_by — wer zuletzt geprueft hat, "+
				"nicht wer verantwortlich ist")
	})

	t.Run("N75-2 Soft-Link kommt aus dem GET zurueck", func(t *testing.T) {
		var assetID string
		require.NoError(t, pool.QueryRow(ctx, `
			INSERT INTO vb_assets (org_id, name, type) VALUES ($1::uuid, 'db-1', 'database')
			RETURNING id::text`, orgID).Scan(&assetID))

		var pnaID string
		require.NoError(t, pool.QueryRow(ctx, `
			INSERT INTO ck_protection_need_assessments (org_id, name, object_type, object_name)
			VALUES ($1::uuid, 'Kundendatenbank', 'system', 'db-1')
			RETURNING id::text`, orgID).Scan(&pnaID))

		// Vor der Verknuepfung darf nichts drinstehen — sonst wuerde die
		// Zusicherung unten auch bei einem Fix greifen, der stumpf immer
		// irgendeine ID zurueckgibt.
		before, err := svc.Risk.GetProtectionNeedAssessment(ctx, orgID, pnaID)
		require.NoError(t, err)
		assert.Nil(t, before.VBAssetID, "ohne Verknuepfung muss das Feld leer sein")

		require.NoError(t, svc.Risk.LinkAssetToPNA(ctx, orgID, pnaID, &assetID))

		after, err := svc.Risk.GetProtectionNeedAssessment(ctx, orgID, pnaID)
		require.NoError(t, err)
		require.NotNil(t, after.VBAssetID,
			"der gesetzte Soft-Link kam aus dem GET nicht zurueck")
		assert.Equal(t, assetID, *after.VBAssetID)

		// Auch die Liste, nicht nur der Einzelabruf — beide gehen durch
		// protectionNeedFromRow.
		list, err := svc.Risk.ListProtectionNeedAssessments(ctx, orgID)
		require.NoError(t, err)
		var found bool
		for _, p := range list {
			if p.ID == pnaID {
				require.NotNil(t, p.VBAssetID)
				assert.Equal(t, assetID, *p.VBAssetID)
				found = true
			}
		}
		assert.True(t, found, "Schutzbedarfsfeststellung nicht in der Liste")

		// Entkoppeln muss das Feld wieder leeren.
		require.NoError(t, svc.Risk.LinkAssetToPNA(ctx, orgID, pnaID, nil))
		unlinked, err := svc.Risk.GetProtectionNeedAssessment(ctx, orgID, pnaID)
		require.NoError(t, err)
		assert.Nil(t, unlinked.VBAssetID)
	})
}
