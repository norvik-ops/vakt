//go:build integration

package integration_test

import (
	"archive/zip"
	"bytes"
	"context"
	"encoding/csv"
	"io"
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

	"github.com/matharnica/vakt/internal/modules/vaktcomply"
	"github.com/matharnica/vakt/internal/shared/audit"
	shareddb "github.com/matharnica/vakt/internal/shared/db"
)

// payload ist die Nutzlast aus dem Befund: eine DDE-Zeile, die Excel und
// LibreOffice Calc beim Oeffnen der Datei ausfuehren.
const payload = `=cmd|/c calc!A1`

// TestCSVFormulaInjection_SoAAndAuditPackage pinnt R1-24-D03 (deckt
// R1-22-I09/SA-22).
//
// GET /vaktcomply/soa.csv schrieb nutzergesetzte Begruendungsfelder ohne jeden
// Formel-Schutz. csvEscape in handler_bsi.go quotet Komma, Anfuehrungszeichen
// und Zeilenumbruch — die Zeichen, die das CSV-FORMAT braucht, nicht die, die
// eine Tabellenkalkulation zur Auswertung bringt. encoding/csv macht dasselbe
// und auch nicht mehr. Kein einziger der CSV-Schreiber im Baum hatte einen
// Formel-Schutz.
//
// Der Test faehrt den gemessenen Weg nach: PATCH setzt die Begruendung,
// der Export liest sie zurueck. Zusaetzlich das Audit-Paket, weil das die
// Datei ist, die der Kunde dem Auditor aushaendigt.
//
// Ohne csvsafe faellt jede Zusicherung unten.
func TestCSVFormulaInjection_SoAAndAuditPackage(t *testing.T) {
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
		INSERT INTO organizations (name, slug) VALUES ('CsvOrg', 'csvorg')
		RETURNING id::text`).Scan(&orgID))
	fwID := seedFramework(ctx, t, pool, orgID, "ISO 27001")
	ctrlID := seedControl(ctx, t, pool, orgID, fwID, "C-1", "Access", "implemented")

	svc := vaktcomply.NewService(pool)
	h := vaktcomply.NewHandler(svc)
	e := echo.New()

	t.Run("SoA-Export", func(t *testing.T) {
		// Der Schreibweg, den der Befund benutzt hat: PATCH /vaktcomply/soa/:id.
		require.NoError(t, svc.UpdateSoAApplicability(ctx, orgID, ctrlID, true, payload, ""))

		// Beweis, dass die Nutzlast wirklich in der Datenbank steht — sonst
		// koennte die Zusicherung unten auch bei einem leeren Export gruen sein.
		var stored string
		require.NoError(t, pool.QueryRow(ctx,
			`SELECT COALESCE(soa_justification_yes,'') FROM ck_controls WHERE org_id = $1::uuid AND id = $2::uuid`,
			orgID,
			ctrlID).Scan(&stored))
		require.Equal(t, payload, stored, "die Nutzlast muss gespeichert sein")

		req := httptest.NewRequest(http.MethodGet, "/", nil)
		rec := httptest.NewRecorder()
		c := e.NewContext(req, rec)
		c.Set("org_id", orgID)
		require.NoError(t, h.GetSoACSV(c))
		require.Equal(t, http.StatusOK, rec.Code)

		records, err := csv.NewReader(bytes.NewReader(rec.Body.Bytes())).ReadAll()
		require.NoError(t, err)
		require.Len(t, records, 2, "Kopfzeile + ein Control")

		cell := records[1][5]
		assert.Equal(t, "'"+payload, cell,
			"die Begruendung stand woertlich mit fuehrendem = in der Exportzeile")
		assert.False(t, strings.HasPrefix(cell, "="),
			"eine Zelle darf nicht mit = beginnen — der Auditor oeffnet die Datei in Excel")
		// Der Inhalt bleibt lesbar: entschaerft heisst nicht geloescht.
		assert.Contains(t, cell, payload)
	})

	t.Run("Audit-Paket", func(t *testing.T) {
		// Dieselbe Nutzlast ueber einen anderen Schreibweg in eine andere
		// Datei desselben Pakets.
		_, err := pool.Exec(ctx,
			`UPDATE ck_controls SET title = $1 WHERE org_id = $2::uuid AND id = $3::uuid`, payload, orgID, ctrlID)
		require.NoError(t, err)

		pkg, err := audit.GeneratePackage(ctx, pool, orgID)
		require.NoError(t, err)
		zr, err := zip.NewReader(bytes.NewReader(pkg.Zip), int64(len(pkg.Zip)))
		require.NoError(t, err)

		var body []byte
		for _, f := range zr.File {
			if f.Name == "controls.csv" {
				rc, err := f.Open()
				require.NoError(t, err)
				body, err = io.ReadAll(rc)
				require.NoError(t, err)
				_ = rc.Close()
			}
		}
		require.NotEmpty(t, body, "controls.csv fehlt im Paket")

		records, err := csv.NewReader(bytes.NewReader(body)).ReadAll()
		require.NoError(t, err)
		require.Len(t, records, 2)
		assert.Equal(t, "'"+payload, records[1][4],
			"der Control-Titel ging unmaskiert in das Artefakt, das der Kunde dem Auditor vorlegt")
	})
}
