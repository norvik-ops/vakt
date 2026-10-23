//go:build integration

// Copyright (c) 2026 NorvikOps. All rights reserved.
// SPDX-License-Identifier: Elastic-2.0

package integration_test

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/labstack/echo/v4"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/matharnica/vakt/internal/modules/vaktcomply/assets"
	sharedmw "github.com/matharnica/vakt/internal/shared/middleware"
)

// assetsHTTP mounts the asset inventory routes on a real Echo, backed by a real
// service and database, exactly like cmd/api: canonical under /vaktcomply and as
// the compatibility alias under /vaktscan. Only the identity is faked.
//
// History: this was the asset half of the vaktscan HTTP-surface test (S121). The
// live sweep found the 500s here (vb_assets.deleted_at, the shadowed
// classification-summary route), so the surface keeps its own test after the
// inventory moved to vaktcomply and the Scan module was archived.
func assetsHTTP(t *testing.T, pool *pgxpool.Pool, orgID, userID string) *echo.Echo {
	t.Helper()
	e := echo.New()
	identity := func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			c.Set("user_id", userID)
			c.Set("org_id", orgID)
			c.Set("roles", []string{"Admin"})
			return next(c)
		}
	}
	h := assets.NewHandler(assets.NewService(pool))
	// ValidateUUIDParams is the guard cmd/api puts on these groups (S121-F3).
	assets.Register(e.Group("/vaktcomply", identity, sharedmw.ValidateUUIDParams()), h)
	assets.Register(e.Group("/vaktscan", identity, sharedmw.ValidateUUIDParams()), h)
	return e
}

// call issues a request and returns status + body. Every response is checked for a
// 5xx: a 500 from any of these routes is the exact failure mode the S121 live
// sweep found.
func call(t *testing.T, e *echo.Echo, method, path string, body any) (int, []byte) {
	t.Helper()
	var rdr *bytes.Reader
	if body != nil {
		raw, err := json.Marshal(body)
		require.NoError(t, err)
		rdr = bytes.NewReader(raw)
	} else {
		rdr = bytes.NewReader(nil)
	}
	req := httptest.NewRequest(method, path, rdr)
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	if rec.Code >= 500 {
		t.Errorf("%s %s → %d (server error)\n%s", method, path, rec.Code, rec.Body.String())
	}
	return rec.Code, rec.Body.Bytes()
}

// TestAssets_HTTPSurface drives every asset route against a real database.
func TestAssets_HTTPSurface(t *testing.T) {
	if testing.Short() {
		t.Skip("integration: -short mode")
	}
	pool, orgID, cleanup := bootPostgresWithOrg(t)
	defer cleanup()
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()

	var userID string
	require.NoError(t, pool.QueryRow(ctx,
		`INSERT INTO users (email) VALUES ('assets-http@acme.test') RETURNING id::text`).Scan(&userID))

	e := assetsHTTP(t, pool, orgID, userID)

	code, body := call(t, e, http.MethodPost, "/vaktcomply/assets", map[string]any{
		"name": "prod-web-01", "type": "server", "criticality": "critical",
		"environment": "prod", "classification": "confidential", "tags": []string{"edge"},
	})
	require.Equal(t, http.StatusCreated, code, "create asset: %s", body)
	var asset struct {
		ID             string `json:"id"`
		Classification string `json:"classification"`
	}
	require.NoError(t, json.Unmarshal(body, &asset))
	require.NotEmpty(t, asset.ID)
	assert.Equal(t, "confidential", asset.Classification,
		"the classification must survive the round trip — it was written and never selected until S124-3")

	code, body = call(t, e, http.MethodGet, "/vaktcomply/assets", nil)
	assert.Equal(t, http.StatusOK, code, "list assets: %s", body)

	code, body = call(t, e, http.MethodGet, "/vaktcomply/assets/"+asset.ID, nil)
	assert.Equal(t, http.StatusOK, code, "get asset: %s", body)

	code, body = call(t, e, http.MethodPut, "/vaktcomply/assets/"+asset.ID, map[string]any{
		"name": "prod-web-01", "type": "server", "criticality": "high",
	})
	assert.Equal(t, http.StatusOK, code, "update asset: %s", body)

	// Static route registered BEFORE /assets/:id — if that ordering ever regresses,
	// this call is swallowed by the :id handler and answers 400/404 instead.
	code, body = call(t, e, http.MethodGet, "/vaktcomply/assets/classification-summary", nil)
	assert.Equal(t, http.StatusOK, code,
		"classification-summary must not be shadowed by /assets/:id: %s", body)

	code, _ = call(t, e, http.MethodGet, "/vaktcomply/assets/"+asset.ID+"/protection-need", nil)
	assert.Less(t, code, 500, "protection-need must not 500 (it did: vb_assets.deleted_at, S121)")

	// The compatibility alias serves the same rows.
	code, body = call(t, e, http.MethodGet, "/vaktscan/assets/"+asset.ID, nil)
	assert.Equal(t, http.StatusOK, code, "alias /vaktscan/assets/:id: %s", body)

	code, _ = call(t, e, http.MethodDelete, "/vaktcomply/assets/"+asset.ID, nil)
	assert.Equal(t, http.StatusNoContent, code, "delete asset")
}

// TestAssets_MalformedIDs: a syntactically INVALID id must be rejected as bad
// input (400) by the UUID guard, not reach a ::uuid cast and become a 500 (22P02).
func TestAssets_MalformedIDs(t *testing.T) {
	if testing.Short() {
		t.Skip("integration: -short mode")
	}
	pool, orgID, cleanup := bootPostgresWithOrg(t)
	defer cleanup()
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	defer cancel()

	var userID string
	require.NoError(t, pool.QueryRow(ctx,
		`INSERT INTO users (email) VALUES ('assets-bad@acme.test') RETURNING id::text`).Scan(&userID))

	e := assetsHTTP(t, pool, orgID, userID)
	for _, path := range []string{
		"/vaktcomply/assets/not-a-uuid",
		"/vaktcomply/assets/not-a-uuid/protection-need",
		"/vaktscan/assets/not-a-uuid",
	} {
		code, body := call(t, e, http.MethodGet, path, nil)
		assert.Equal(t, http.StatusBadRequest, code,
			fmt.Sprintf("GET %s: a malformed UUID must be rejected as bad input, not become a 500 (22P02): %s", path, body))
	}
}

// TestAssets_ImportCSV_RoundTrip (S126, moved from vaktscan with the inventory) runs the CSV asset import against
// a real database. Until now only its failure paths were covered — the unit test
// constructs a bare &Service{} and asserts on errors that occur before the first
// repository call, because a concrete *Repository leaves no other way in. So the
// half that writes rows has never been executed by a test.
func TestAssets_ImportCSV_RoundTrip(t *testing.T) {
	if testing.Short() {
		t.Skip("integration: -short mode")
	}
	pool, orgID, cleanup := bootPostgresWithOrg(t)
	defer cleanup()
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	defer cancel()

	assetSvc := assets.NewService(pool)

	csv := strings.Join([]string{
		"name,type,criticality,tags",
		"prod-web-01,server,critical,\"edge,public\"",
		"prod-db-01,database,high,internal",
		// No criticality: the importer defaults it rather than rejecting the row.
		"build-runner,container,,ci",
		// Invalid type: must be reported, and must not take the good rows with it.
		"mystery-box,teapot,low,",
	}, "\n")

	imported, failed, errs, err := assetSvc.ImportAssetsCSV(ctx, orgID, strings.NewReader(csv))
	require.NoError(t, err, "a bad row is a row-level failure, not an import-level one")
	assert.Equal(t, 3, imported)
	assert.Equal(t, 1, failed)
	require.Len(t, errs, 1)
	assert.Contains(t, errs[0], "mystery-box", "the rejected row must be named, or the user cannot fix it")

	list, total, err := assetSvc.ListAssets(ctx, orgID, 1, 50, "")
	require.NoError(t, err)
	assert.Equal(t, 3, total)

	byName := map[string]assets.Asset{}
	for _, a := range list {
		byName[a.Name] = a
	}
	require.Contains(t, byName, "prod-web-01")
	assert.Equal(t, "critical", byName["prod-web-01"].Criticality)
	assert.ElementsMatch(t, []string{"edge", "public"}, byName["prod-web-01"].Tags,
		"a quoted, comma-separated tag cell is one column with two tags")
	assert.Equal(t, "medium", byName["build-runner"].Criticality, "a blank criticality defaults to medium")
}
