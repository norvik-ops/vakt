//go:build integration

package integration_test

import (
	"bytes"
	"compress/zlib"
	"context"
	"encoding/csv"
	"io"
	"net/http"
	"net/http/httptest"
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/labstack/echo/v4"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/testcontainers/testcontainers-go/modules/postgres"

	"github.com/matharnica/vakt/internal/modules/vaktcomply"
	shareddb "github.com/matharnica/vakt/internal/shared/db"
)

// TestSoAApplicability_AllExportsAgree pinnt R1-20-02.
//
// Die Anwendbarkeit einer Kontrolle stand in ZWEI Spalten von ck_controls:
// not_applicable (Migration 016) und soa_applicable (Migration 113). Sie
// bedeuten dasselbe, invers zueinander, und wurden getrennt geschrieben:
// PATCH /vaktcomply/soa/:control_id setzte nur soa_applicable, der
// Control-Dialog und der Freigabe-Workflow nur not_applicable.
//
// Zwei Exporte lasen je eine der beiden Spalten — und beide unter demselben
// Alias `applicable`:
//
//	/vaktcomply/frameworks/:id/soa.pdf -> (NOT c.not_applicable) AS applicable
//	/vaktcomply/soa.csv                -> c.soa_applicable       AS applicable
//
// Nach einem PATCH stand in derselben Zeile soa_applicable = false UND
// not_applicable = false. Die CSV sagte "Nein", die PDF im selben Moment "Ja".
// Fuer ISO 27001 Klausel 6.1.3 ist die Anwendbarkeitserklaerung DER Nachweis;
// zwei Dokumente aus einem System, die sich ueber dieselbe Kontrolle
// widersprechen, sind fuer einen Auditor schlimmer als ein fehlendes Dokument.
//
// Der Test faehrt den ECHTEN Schreibweg (Service hinter PATCH
// /vaktcomply/soa/:control_id) und zieht danach beide Exporte WIRKLICH — die
// CSV als Bytes durch den Handler, die PDF als Bytes durch den Handler und
// wieder ausgepackt. Nur die Spalten zu lesen wuerde den Defekt nicht zeigen:
// beide Spalten sahen fuer sich stimmig aus, der Widerspruch entstand erst
// zwischen den Lesern.
//
// Nicht Teil dieses Tests: /vaktcomply/soa/export.{pdf,xlsx,docx}. Der liest
// ck_soa_entries — eine eigene, versionierte und freigebbare Fassung
// (Migration 181: version, is_approved, approved_by/at, manually_set), die aus
// einem statischen ISO-27001-Annex-A-Katalog befuellt wird
// (policy/soa_controls_seed.go) und ck_controls nur ueber
// SyncSoAImplementationStatus beruehrt — und dort ausschliesslich den
// Umsetzungsstand, nie die Anwendbarkeit. Das ist ein bewusst getrenntes
// Dokument, kein dritter Zufall, und bleibt deshalb aussen vor.
//
// Nicht-Vakuitaet: dreht man Migration 264 zurueck (soa_applicable wieder als
// frei beschreibbare Spalte) und laesst UpdateSoAApplicability wieder
// soa_applicable schreiben, faellt die Zusicherung fuer CTRL-B in der PDF.
func TestSoAApplicability_AllExportsAgree(t *testing.T) {
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
		INSERT INTO organizations (name, slug) VALUES ('SoaOrg', 'soaorg')
		RETURNING id::text`).Scan(&orgID))
	fwID := seedFramework(ctx, t, pool, orgID, "ISO 27001")
	ctrlA := seedControl(ctx, t, pool, orgID, fwID, "CTRLA", "Access", "implemented")
	ctrlB := seedControl(ctx, t, pool, orgID, fwID, "CTRLB", "Access", "implemented")

	svc := vaktcomply.NewService(pool)
	h := vaktcomply.NewHandler(svc)
	e := echo.New()

	// Der echte Schreibweg hinter PATCH /vaktcomply/soa/:control_id.
	// CTRL-A bleibt anwendbar, CTRL-B wird ausgeschlossen.
	require.NoError(t, svc.UpdateSoAApplicability(ctx, orgID, ctrlA, true, "wird angewendet", ""))
	require.NoError(t, svc.UpdateSoAApplicability(ctx, orgID, ctrlB, false, "", "kein Rechenzentrum im Betrieb"))

	t.Run("beide Spalten koennen sich nicht mehr widersprechen", func(t *testing.T) {
		var mismatches int
		require.NoError(t, pool.QueryRow(ctx, `
			SELECT COUNT(*) FROM ck_controls
			WHERE org_id = $1::uuid AND (NOT not_applicable) <> soa_applicable`,
			orgID).Scan(&mismatches))
		assert.Zero(t, mismatches,
			"(NOT not_applicable) und soa_applicable muessen fuer jede Zeile dasselbe sagen")

		// Der Ausschluss ist wirklich angekommen — sonst waere die Zusicherung
		// oben auch dann gruen, wenn der PATCH gar nichts geschrieben haette.
		var na bool
		require.NoError(t, pool.QueryRow(ctx,
			`SELECT not_applicable FROM ck_controls WHERE org_id = $1::uuid AND id = $2::uuid`,
			orgID, ctrlB).Scan(&na))
		require.True(t, na, "der PATCH muss not_applicable gesetzt haben")
	})

	t.Run("soa_applicable ist strukturell unbeschreibbar", func(t *testing.T) {
		// ADR-0082: die Ableitung ist erzwungen, nicht eine Frage der Disziplin.
		// Ein kuenftiger Schreibpfad scheitert hart statt still zu divergieren.
		//
		// Der Versuch laeuft in einer Transaktion, die immer zurueckgerollt
		// wird: waere die Spalte doch beschreibbar, duerfte der Schreibvorgang
		// die Exportpruefungen weiter unten nicht verfaelschen. Genau das ist
		// bei der Nicht-Vakuitaets-Probe passiert — der geglueckte Schreibzugriff
		// hat CTRL-B wieder auf "anwendbar" gesetzt und der CSV-Zusicherung
		// einen falschen Grund fuer ihr Fehlschlagen gegeben.
		tx, err := pool.Begin(ctx)
		require.NoError(t, err)
		defer func() { _ = tx.Rollback(ctx) }()

		_, err = tx.Exec(ctx,
			`UPDATE ck_controls SET soa_applicable = true WHERE org_id = $1::uuid AND id = $2::uuid`,
			orgID, ctrlB)
		require.Error(t, err, "ein Schreibversuch auf die abgeleitete Spalte muss scheitern")
		var pgErr *pgconn.PgError
		require.ErrorAs(t, err, &pgErr)
		assert.Equal(t, "428C9", pgErr.Code,
			"erwartet ERRCODE_GENERATED_ALWAYS: %s", pgErr.Message)
	})

	// Beide Exporte werden wirklich gezogen. Erwartung je Kontrolle.
	want := map[string]string{"CTRLA": "Ja", "CTRLB": "Nein"}

	t.Run("SoA-CSV", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/", nil)
		rec := httptest.NewRecorder()
		c := e.NewContext(req, rec)
		c.Set("org_id", orgID)
		require.NoError(t, h.GetSoACSV(c))
		require.Equal(t, http.StatusOK, rec.Code)

		records, err := csv.NewReader(bytes.NewReader(rec.Body.Bytes())).ReadAll()
		require.NoError(t, err)
		require.Len(t, records, 3, "Kopfzeile + zwei Controls")

		got := map[string]string{}
		for _, row := range records[1:] {
			// Spalten: Framework, Domain, Kontrolle, Anwendbar, ...
			for id := range want {
				if strings.Contains(row[2], id) {
					got[id] = row[3]
				}
			}
		}
		assert.Equal(t, want, got, "die CSV muss beide Kontrollen so ausweisen, wie sie gesetzt wurden")
	})

	t.Run("SoA-PDF", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/", nil)
		rec := httptest.NewRecorder()
		c := e.NewContext(req, rec)
		c.Set("org_id", orgID)
		c.SetParamNames("id")
		c.SetParamValues(fwID)
		require.NoError(t, h.ExportSoAPDF(c))
		require.Equal(t, http.StatusOK, rec.Code)
		require.Equal(t, "application/pdf", rec.Header().Get("Content-Type"))

		text := pdfVisibleText(t, rec.Body.Bytes())
		for id, expected := range want {
			assert.Equal(t, expected, applicabilityAfter(t, text, id),
				"die PDF muss fuer %s dasselbe ausweisen wie die CSV", id)
		}
	})
}

// pdfVisibleText gibt die Textstuecke eines fpdf-Dokuments in Ausgabereihenfolge
// zurueck. fpdf komprimiert die Inhaltsstroeme mit zlib; die Textoperanden
// stehen darin als "(…) Tj".
func pdfVisibleText(t *testing.T, pdf []byte) []string {
	t.Helper()
	var out []string
	streamRe := regexp.MustCompile(`(?s)stream\r?\n(.*?)endstream`)
	textRe := regexp.MustCompile(`\(((?:\\.|[^\\()])*)\)\s*Tj`)
	for _, m := range streamRe.FindAllSubmatch(pdf, -1) {
		zr, err := zlib.NewReader(bytes.NewReader(m[1]))
		if err != nil {
			continue // nicht jeder Strom ist ein komprimierter Inhaltsstrom
		}
		raw, err := io.ReadAll(zr)
		_ = zr.Close()
		if err != nil {
			continue
		}
		for _, tm := range textRe.FindAllSubmatch(raw, -1) {
			if s := strings.TrimSpace(decodePDFString(tm[1])); s != "" {
				out = append(out, s)
			}
		}
	}
	require.NotEmpty(t, out, "aus der PDF liess sich kein Text lesen — der Test wuerde sonst leer gruen")
	return out
}

// decodePDFString loest die Escapes eines PDF-Literalstrings auf und wirft
// danach die Null-Bytes weg: fpdf schreibt den Text als UTF-16BE, in dem jedes
// ASCII-Zeichen ein fuehrendes 0x00 traegt.
func decodePDFString(b []byte) string {
	var buf bytes.Buffer
	for i := 0; i < len(b); i++ {
		if b[i] != '\\' || i+1 >= len(b) {
			buf.WriteByte(b[i])
			continue
		}
		i++
		switch c := b[i]; c {
		case 'n':
			buf.WriteByte('\n')
		case 'r':
			buf.WriteByte('\r')
		case 't':
			buf.WriteByte('\t')
		case 'b':
			buf.WriteByte('\b')
		case 'f':
			buf.WriteByte('\f')
		default:
			if c >= '0' && c <= '7' { // oktale Escape-Sequenz, bis zu drei Ziffern
				v := 0
				n := 0
				for n < 3 && i < len(b) && b[i] >= '0' && b[i] <= '7' {
					v = v*8 + int(b[i]-'0')
					i++
					n++
				}
				i--
				buf.WriteByte(byte(v))
				continue
			}
			buf.WriteByte(c)
		}
	}
	return strings.ReplaceAll(buf.String(), "\x00", "")
}

// applicabilityAfter sucht das Label der Kontrolle und gibt das naechste
// Ja/Nein danach zurueck — die Zelle "Anwendbar" der Tabellenzeile.
func applicabilityAfter(t *testing.T, text []string, controlID string) string {
	t.Helper()
	for i, s := range text {
		if !strings.Contains(s, controlID) {
			continue
		}
		for _, s2 := range text[i+1:] {
			if s2 == "Ja" || s2 == "Nein" {
				return s2
			}
		}
	}
	t.Fatalf("in der PDF wurde zu %s keine Anwendbarkeit gefunden; gelesener Text: %v", controlID, text)
	return ""
}
