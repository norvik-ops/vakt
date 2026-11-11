// Copyright (c) 2026 NorvikOps. All rights reserved.
// SPDX-License-Identifier: Elastic-2.0

package license

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/labstack/echo/v4"
)

// DBMiddleware liest einen aktivierten Schlüssel je Org aus license_keys. Ist er
// ungültig, steht im Log nur der feste Grund, nie der rohe parse()-Fehler.
// Braucht eine migrierte Postgres (VAKT_DB_URL), wie die Real-DB-Tests in auth.
func TestDBMiddleware_InvalidKeyLogsReasonOnly(t *testing.T) {
	dbURL := os.Getenv("VAKT_DB_URL")
	if dbURL == "" {
		t.Skip("VAKT_DB_URL not set — der Test braucht eine migrierte Postgres")
	}
	ctx := context.Background()
	pool, err := pgxpool.New(ctx, dbURL)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(pool.Close)

	priv, restore := setupTestKeys(t)
	defer restore()
	bad := signRaw(t, priv, []byte("not-json-renewal-secret-do-not-log"))

	var orgID string
	slug := fmt.Sprintf("licenselog-%d", os.Getpid())
	if err := pool.QueryRow(ctx,
		`INSERT INTO organizations (name, slug) VALUES ($1, $1) RETURNING id::text`, slug,
	).Scan(&orgID); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		// license_keys hängt per ON DELETE CASCADE an der Org (Migration 210).
		if _, err := pool.Exec(context.Background(), `DELETE FROM organizations WHERE id = $1::uuid`, orgID); err != nil {
			t.Errorf("Test-Org nicht gelöscht: %v", err)
		}
	})
	if _, err := pool.Exec(ctx,
		`INSERT INTO license_keys (org_id, key_value) VALUES ($1::uuid, $2)`, orgID, bad,
	); err != nil {
		t.Fatal(err)
	}

	buf := captureLog(t)
	e := echo.New()
	c := e.NewContext(httptest.NewRequest(http.MethodGet, "/", nil), httptest.NewRecorder())
	c.Set("org_id", orgID)
	mw := DBMiddleware(pool, NewInstance(communityLicense()))
	if err := mw(func(echo.Context) error { return nil })(c); err != nil {
		t.Fatal(err)
	}
	assertReasonOnly(t, buf.String(), "license key could not be decoded", bad)
}
