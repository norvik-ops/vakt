//go:build integration

package integration_test

import (
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

	"github.com/matharnica/vakt/internal/modules/vaktcomply"
	"github.com/matharnica/vakt/internal/modules/vaktcomply/policy"
	shareddb "github.com/matharnica/vakt/internal/shared/db"
)

// TestFrameworkSoAExport_ServesCanonicalSource pinnt R1-36a-D02 / ADR-0091.
//
// Es gab zwei Statement-of-Applicability-Quellen, die sich widersprechen konnten:
//
//	GET /vaktcomply/frameworks/:id/soa.pdf -> ck_controls (ExportSoAPDF, alt)
//	GET /vaktcomply/soa/export?format=pdf  -> ck_soa_entries (ExportDedicatedSoA)
//
// Entscheidung des Betreibers: ck_soa_entries ist kanonisch. Der Framework-Export
// darf keine zweite, abweichende Wahrheit mehr erzeugen. Für ISO 27001 liefert er
// deshalb dasselbe Dokument wie der dedizierte Export, sobald eine kanonische SoA
// initialisiert ist.
//
// Der Test fährt den ECHTEN dedizierten Schreibweg (InitDedicatedSoA +
// UpdateDedicatedSoAEntry mit einer eindeutigen Evidence-Referenz), zieht danach
// BEIDE Exporte wirklich als PDF-Bytes durch ihre Handler und packt den sichtbaren
// Text aus. Erwartung: beide enthalten die eindeutige Markierung UND ihr sichtbarer
// Text ist identisch — beide lesen ck_soa_entries.
//
// Nicht-Vakuität: Lässt man ExportSoAPDF wieder aus ck_controls lesen (die
// isISO27001Framework-Weiche entfernen), fehlt der Framework-PDF die Markierung
// aus ck_soa_entries und die Textgleichheit fällt.
func TestFrameworkSoAExport_ServesCanonicalSource(t *testing.T) {
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
		INSERT INTO organizations (name, slug) VALUES ('CanonOrg', 'canonorg')
		RETURNING id::text`).Scan(&orgID))
	fwID := seedFramework(ctx, t, pool, orgID, "ISO 27001")

	svc := vaktcomply.NewService(pool)
	h := vaktcomply.NewHandler(svc)
	e := echo.New()

	// Kanonische SoA anlegen und eine Zeile mit einer eindeutigen Evidence-Referenz
	// versehen. Diese Markierung existiert NUR in ck_soa_entries, nie in ck_controls.
	const marker = "EVIDENCE-CANON-7f3a"
	require.NoError(t, svc.InitDedicatedSoA(ctx, orgID))
	require.NoError(t, svc.UpdateDedicatedSoAEntry(ctx, orgID, "A.5.7", policy.UpdateSoAEntryInput{
		Applicable:           true,
		Justification:        "Bedrohungslage wird ausgewertet",
		ImplementationStatus: "implemented",
		EvidenceReference:    marker,
	}))

	pull := func(t *testing.T, handler echo.HandlerFunc, withFramework bool) []string {
		t.Helper()
		req := httptest.NewRequest(http.MethodGet, "/", nil)
		rec := httptest.NewRecorder()
		c := e.NewContext(req, rec)
		c.Set("org_id", orgID)
		if withFramework {
			c.SetParamNames("id")
			c.SetParamValues(fwID)
		}
		require.NoError(t, handler(c))
		require.Equal(t, http.StatusOK, rec.Code)
		require.Equal(t, "application/pdf", rec.Header().Get("Content-Type"))
		return pdfVisibleText(t, rec.Body.Bytes())
	}

	frameworkText := pull(t, h.ExportSoAPDF, true)        // /frameworks/:id/soa.pdf
	dedicatedText := pull(t, h.ExportDedicatedSoA, false) // /soa/export?format=pdf

	assert.Contains(t, strings.Join(dedicatedText, "\x1f"), marker,
		"der dedizierte Export muss die Markierung aus ck_soa_entries tragen")
	assert.Contains(t, strings.Join(frameworkText, "\x1f"), marker,
		"der Framework-Export muss dieselbe Markierung tragen — er liest jetzt ck_soa_entries")
	assert.Equal(t, dedicatedText, frameworkText,
		"beide SoA-Exporte müssen denselben sichtbaren Inhalt liefern (eine Quelle)")
}
