//go:build integration

package integration_test

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
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

	"github.com/matharnica/vakt/internal/auth"
	shareddb "github.com/matharnica/vakt/internal/shared/db"
)

// TestAPIKey_DeactivatedUserLosesAccess pinnt R1-SA21-D7.
//
// Die Schluessel-Abfrage pruefte nur den Schluessel selbst — revoked_at und
// expires_at — und nie den Menschen dahinter. Ein per SCIM deprovisionierter
// Nutzer (services/scim/service.go setzt users.is_active = FALSE) behielt
// seinen API-Schluessel damit UNBEFRISTET.
//
// Das wiegt schwerer als es klingt: ein Paseto-Token laeuft nach einer Stunde
// ab, ein API-Schluessel nicht. Und von den drei Wegen, auf denen jemand ein
// Konto verlaesst, widerrief nur EINER die Schluessel (vakthr
// enforceOffboardingRevocation) — SCIM-Deprovisionierung und
// usermgmt.RemoveUser taten es nicht.
//
// Der Test faehrt die echte AuthMiddleware gegen echtes Postgres. Er prueft
// beide Richtungen, denn ein Test, der nur die Sperre prueft, waere auch dann
// gruen, wenn die Abfrage gar keinen Schluessel mehr annimmt.
func TestAPIKey_DeactivatedUserLosesAccess(t *testing.T) {
	if testing.Short() {
		t.Skip("integration: -short mode")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
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

	var orgID, userID string
	require.NoError(t, pool.QueryRow(ctx, `
		INSERT INTO organizations (name, slug) VALUES ('Acme', 'acme') RETURNING id::text
	`).Scan(&orgID))
	require.NoError(t, pool.QueryRow(ctx, `
		INSERT INTO users (email, password_hash, display_name)
		VALUES ('leaver@acme.test', '$2a$10$abcdefghijklmnopqrstuvwxyz', 'Leaver')
		RETURNING id::text
	`).Scan(&userID))

	raw := "vakt_leaver_secret_value_123456"
	sum := sha256.Sum256([]byte(raw))
	_, err = pool.Exec(ctx, `
		INSERT INTO api_keys (org_id, created_by, name, key_hash, key_prefix, scopes)
		VALUES ($1::uuid, $2::uuid, 'leaver', $3, $4, $5)`,
		orgID, userID, hex.EncodeToString(sum[:]), raw[:12], []string{"vaktcomply"})
	require.NoError(t, err)

	key := mustKeyIntegration(t)
	call := func() *httptest.ResponseRecorder {
		e := echo.New()
		req := httptest.NewRequest(http.MethodGet, "/api/v1/vaktcomply/risks", nil)
		req.Header.Set("Authorization", "Bearer "+raw)
		rec := httptest.NewRecorder()
		c := e.NewContext(req, rec)
		h := auth.AuthMiddleware(key, pool)(func(c echo.Context) error {
			return c.JSON(http.StatusOK, map[string]string{"ok": "true"})
		})
		require.NoError(t, h(c))
		return rec
	}

	// Gegenprobe zuerst: solange der Nutzer aktiv ist, MUSS der Schluessel
	// funktionieren. Ohne diese Haelfte waere der Test auch dann gruen, wenn die
	// Abfrage ueberhaupt keinen Schluessel mehr annimmt.
	assert.Equal(t, http.StatusOK, call().Code,
		"ein Schluessel eines aktiven Nutzers muss weiter gelten")

	// Genau das, was die SCIM-Deprovisionierung tut.
	_, err = pool.Exec(ctx, `UPDATE users SET is_active = FALSE WHERE id = $1::uuid`, userID)
	require.NoError(t, err)

	rec := call()
	assert.Equal(t, http.StatusUnauthorized, rec.Code,
		"nach der Deaktivierung des Nutzers darf sein API-Schluessel nicht mehr gelten — "+
			"API-Schluessel sind langlebig, der Zugriff waere sonst unbefristet")

	// Die Ablehnung muss BENENNBAR sein, nicht nur wirksam. Ein nacktes
	// "invalid api key" waere hier irrefuehrend: der Schluessel ist weder
	// widerrufen noch abgelaufen, er sieht administrativ gesund aus. Ohne diese
	// Zusicherung faellt eine CI-Pipeline aus und niemand kann sehen, warum —
	// der schwerste betriebliche Einwand aus dem adversarialen Review.
	assert.Contains(t, rec.Body.String(), "AUTH_KEY_OWNER_INACTIVE",
		"die Ablehnung muss ihren Grund nennen, sonst ist der Ausfall nicht diagnostizierbar")
}
