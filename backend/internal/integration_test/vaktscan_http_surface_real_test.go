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
	"testing"
	"time"

	"github.com/hibiken/asynq"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/labstack/echo/v4"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/matharnica/vakt/internal/modules/vaktscan"
	sharedmw "github.com/matharnica/vakt/internal/shared/middleware"
)

// vaktscanHTTP mounts the real vaktscan routes on a real Echo, backed by a real
// service and a real database — the only piece faked is the identity, which the
// auth middleware would otherwise put into the context.
//
// The point is to exercise the HANDLERS. Before this test they had 0% coverage:
// 36 functions in handler.go, none of them ever executed by anything but a human
// with a browser. That is not a coincidence — every vaktscan defect in this
// project's history (the raw-SQL 500s, the CSV export calling a `.csv` suffix that
// was really a query parameter, DeleteFinding wired in the UI with no handler at
// all) lived exactly here, and every one was found by a live sweep because no test
// could reach the layer.
func vaktscanHTTP(t *testing.T, pool *pgxpool.Pool, orgID, userID string) *echo.Echo {
	t.Helper()
	e := echo.New()
	g := e.Group("/vaktscan",
		func(next echo.HandlerFunc) echo.HandlerFunc {
			return func(c echo.Context) error {
				c.Set("user_id", userID)
				c.Set("org_id", orgID)
				c.Set("roles", []string{"Admin"})
				return next(c)
			}
		},
		// The same guard cmd/api/routes.go puts on this group. It is mounted here on
		// purpose: it is where the project decided a malformed UUID gets rejected
		// (S121-F3), so a test that leaves it off is not testing vaktscan — it is
		// testing a configuration that does not exist.
		sharedmw.ValidateUUIDParams(),
	)
	vaktscan.Register(g, vaktscan.NewHandler(vaktscan.NewService(pool, asynq.RedisClientOpt{})))
	return e
}

// call issues a request and returns status + body. Every response is checked for a
// 5xx by the caller: a 500 from any of these routes is the exact failure mode the
// S121 live sweep found 46 of, and the whole reason this file exists.
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

// TestVaktscan_HTTPSurface_Community drives the whole Community route surface of
// vaktscan against a real database and asserts that each one answers — and answers
// with the shape it claims.
func TestVaktscan_HTTPSurface_Community(t *testing.T) {
	if testing.Short() {
		t.Skip("integration: -short mode")
	}
	pool, orgID, cleanup := bootPostgresWithOrg(t)
	defer cleanup()
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()

	var userID string
	require.NoError(t, pool.QueryRow(ctx,
		`INSERT INTO users (email) VALUES ('scan-http@acme.test') RETURNING id::text`).Scan(&userID))

	e := vaktscanHTTP(t, pool, orgID, userID)

	// ── Assets ────────────────────────────────────────────────────────────────
	code, body := call(t, e, http.MethodPost, "/vaktscan/assets", map[string]any{
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

	code, body = call(t, e, http.MethodGet, "/vaktscan/assets", nil)
	assert.Equal(t, http.StatusOK, code, "list assets: %s", body)

	code, body = call(t, e, http.MethodGet, "/vaktscan/assets/"+asset.ID, nil)
	assert.Equal(t, http.StatusOK, code, "get asset: %s", body)

	code, body = call(t, e, http.MethodPut, "/vaktscan/assets/"+asset.ID, map[string]any{
		"name": "prod-web-01", "type": "server", "criticality": "high",
	})
	assert.Equal(t, http.StatusOK, code, "update asset: %s", body)

	// Static route registered BEFORE /assets/:id — if that ordering ever regresses,
	// this call is swallowed by the :id handler and answers 400/404 instead.
	code, body = call(t, e, http.MethodGet, "/vaktscan/assets/classification-summary", nil)
	assert.Equal(t, http.StatusOK, code,
		"classification-summary must not be shadowed by /assets/:id: %s", body)

	code, _ = call(t, e, http.MethodGet, "/vaktscan/assets/"+asset.ID+"/protection-need", nil)
	assert.Less(t, code, 500, "protection-need must not 500 (it did: vb_assets.deleted_at, S121)")

	// ── Findings ──────────────────────────────────────────────────────────────
	repo := vaktscan.NewRepository(pool)
	cve := "CVE-2026-9999"
	finding, err := repo.UpsertFinding(ctx, orgID, vaktscan.Finding{
		AssetID: asset.ID, CVEID: &cve, Title: "OpenSSL overflow",
		Severity: "critical", Status: "open", Scanner: "trivy", Sources: []string{"trivy"},
	})
	require.NoError(t, err)

	code, body = call(t, e, http.MethodGet, "/vaktscan/findings", nil)
	assert.Equal(t, http.StatusOK, code, "list findings: %s", body)

	code, body = call(t, e, http.MethodGet, "/vaktscan/findings/"+finding.ID, nil)
	assert.Equal(t, http.StatusOK, code, "get finding: %s", body)

	code, body = call(t, e, http.MethodPatch, "/vaktscan/findings/"+finding.ID, map[string]any{
		"status": "accepted_risk", "justification": "compensating control in place",
	})
	assert.Equal(t, http.StatusOK, code, "update finding: %s", body)

	code, body = call(t, e, http.MethodPost, "/vaktscan/findings/bulk", map[string]any{
		"ids": []string{finding.ID}, "status": "open",
	})
	assert.Less(t, code, 500, "bulk update: %s", body)

	// ── Suppressions ──────────────────────────────────────────────────────────
	code, body = call(t, e, http.MethodPost, "/vaktscan/suppressions", map[string]any{
		"cve_id": cve, "reason": "false positive on this image", "scope": "global",
	})
	assert.Less(t, code, 500, "create suppression: %s", body)

	code, body = call(t, e, http.MethodGet, "/vaktscan/suppressions", nil)
	assert.Equal(t, http.StatusOK, code, "list suppressions: %s", body)

	// ── SLA ───────────────────────────────────────────────────────────────────
	for _, path := range []string{"/vaktscan/sla-dashboard", "/vaktscan/sla-config",
		"/vaktscan/sla-policies", "/vaktscan/sla/summary"} {
		code, body = call(t, e, http.MethodGet, path, nil)
		assert.Equal(t, http.StatusOK, code, "%s: %s", path, body)
	}

	code, body = call(t, e, http.MethodPut, "/vaktscan/sla-policies/critical", map[string]any{
		"remediation_days": 7, "advance_warning_days": 2,
	})
	assert.Less(t, code, 500, "upsert sla policy: %s", body)

	code, body = call(t, e, http.MethodPost, "/vaktscan/sla-policies/reset", nil)
	assert.Less(t, code, 500, "reset sla policies: %s", body)

	// ── Certificates ──────────────────────────────────────────────────────────
	// The static routes must come before /:id, same trap as the assets summary.
	code, body = call(t, e, http.MethodGet, "/vaktscan/certificates", nil)
	assert.Equal(t, http.StatusOK, code, "list certificates: %s", body)

	code, body = call(t, e, http.MethodGet, "/vaktscan/certificates/expiring", nil)
	assert.Equal(t, http.StatusOK, code,
		"certificates/expiring must not 500 — it did, for every caller: the query bound an "+
			"int into ($2 || ' days')::interval, which pgx cannot type (S121): %s", body)

	// ── Scanner status ────────────────────────────────────────────────────────
	code, body = call(t, e, http.MethodGet, "/vaktscan/scanner-status", nil)
	assert.Equal(t, http.StatusOK, code, "scanner-status: %s", body)

	// ── Delete paths (they exist only because S121 built them) ────────────────
	code, _ = call(t, e, http.MethodDelete, "/vaktscan/findings/"+finding.ID, nil)
	assert.Less(t, code, 500, "delete finding")

	code, _ = call(t, e, http.MethodDelete, "/vaktscan/assets/"+asset.ID, nil)
	assert.Less(t, code, 500, "delete asset")
}

// TestVaktscan_HTTPSurface_MalformedIDs is the other half of the live sweep: a
// syntactically INVALID id, not merely a non-existent one.
//
// A well-formed dummy UUID takes the not-found path and answers 404, which looks
// fine and proves nothing. A malformed segment reaches a query that casts it to
// ::uuid, Postgres raises SQLSTATE 22P02, and any handler that only maps
// ErrNoRows falls through to a 500. That class was 34 routes wide when it was
// found, and it is invisible through the UI (the frontend only ever sends real
// UUIDs from a list response).
func TestVaktscan_HTTPSurface_MalformedIDs(t *testing.T) {
	if testing.Short() {
		t.Skip("integration: -short mode")
	}
	pool, orgID, cleanup := bootPostgresWithOrg(t)
	defer cleanup()
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	defer cancel()

	var userID string
	require.NoError(t, pool.QueryRow(ctx,
		`INSERT INTO users (email) VALUES ('scan-bad@acme.test') RETURNING id::text`).Scan(&userID))

	e := vaktscanHTTP(t, pool, orgID, userID)

	// ValidateUUIDParams is the enforcement point (S121-F3): a malformed id is
	// client input, not a not-found, and mapping it inside every handler would have
	// meant 34 handlers each remembering to. The guard sits on the group instead.
	//
	// So this pins the guard, not the handlers: each of these must come back 400.
	// Without it they answer 500 — verified while writing this test, which is
	// precisely why the guard exists and why a test has to hold it in place.
	for _, path := range []string{
		"/vaktscan/assets/not-a-uuid",
		"/vaktscan/assets/not-a-uuid/protection-need",
		"/vaktscan/findings/not-a-uuid",
		"/vaktscan/certificates/not-a-uuid",
		"/vaktscan/scans/not-a-uuid",
	} {
		code, body := call(t, e, http.MethodGet, path, nil)
		assert.Equal(t, http.StatusBadRequest, code,
			fmt.Sprintf("GET %s: a malformed UUID must be rejected as bad input, not become a 500 (22P02): %s", path, body))
	}
}
