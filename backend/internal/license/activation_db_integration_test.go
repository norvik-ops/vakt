//go:build integration

// Copyright (c) 2026 NorvikOps. All rights reserved.
// SPDX-License-Identifier: Elastic-2.0

package license

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/labstack/echo/v4"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/modules/postgres"
	"github.com/testcontainers/testcontainers-go/wait"
)

// Die beiden Faelle, die der reine In-Memory-Test nicht erreicht, weil sie an
// der Datenbank haengen:
//
//	1. NEUSTART. Der aktivierte Schluessel liegt in license_keys, nicht in
//	   VAKT_LICENSE_KEY. Ohne AdoptActivatedKey startet die Instanz wieder als
//	   Community — und meldet dem Kunden dauerhaft "community", obwohl er
//	   aktiviert hat (R1-07-B02, "DAUERHAFT").
//	2. ORG-GENAUIGKEIT von GET /license. Die Route haengt auf der baren
//	   api-Gruppe; ohne DBMiddleware in ihrer Kette antwortet sie mit der
//	   Instanz-Lizenz statt mit der des fragenden Mandanten (R1-17-L05).
//
// Die zweite Zusicherung braucht ZWEI Orgs mit verschiedenen Schluesseln:
// haette nur eine einen, waere die Instanz-Lizenz zufaellig dieselbe Antwort
// und der Test wuerde gruen bleiben, auch wenn die Route wieder org-blind waere.

const pgImageForLicense = "postgres:16.14-alpine@sha256:57c72fd2a128e416c7fcc499958864df5301e940bca0a56f58fddf30ffc07777"

func startPostgres(ctx context.Context, t *testing.T) *pgxpool.Pool {
	t.Helper()
	c, err := postgres.Run(ctx,
		pgImageForLicense,
		postgres.WithDatabase("vakt_test"),
		postgres.WithUsername("vakt"),
		postgres.WithPassword("vakt"),
		postgres.WithSQLDriver("pgx"),
		testcontainers.WithWaitStrategy(
			wait.ForLog("database system is ready to accept connections").
				WithOccurrence(2).WithStartupTimeout(2*time.Minute)),
	)
	require.NoError(t, err, "postgres container")
	t.Cleanup(func() { _ = c.Terminate(context.Background()) })

	dsn, err := c.ConnectionString(ctx, "sslmode=disable")
	require.NoError(t, err)
	pool, err := pgxpool.New(ctx, dsn)
	require.NoError(t, err)
	t.Cleanup(pool.Close)

	// Nur die Tabelle, die die Lizenz-Kette anfasst (Migration 081).
	_, err = pool.Exec(ctx, `
		CREATE TABLE license_keys (
			id           BIGSERIAL   PRIMARY KEY,
			org_id       UUID        NOT NULL UNIQUE,
			key_value    TEXT        NOT NULL,
			activated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
			activated_by UUID        NULL
		)`)
	require.NoError(t, err)
	return pool
}

// mountWithDB baut dieselbe Montage wie mountLikeProduction, aber mit echter
// Datenbank und einer festlegbaren Org je Request.
func mountWithDB(t *testing.T, inst *Instance, pool *pgxpool.Pool, orgID *string) *echo.Echo {
	t.Helper()
	e := echo.New()
	e.Use(inst.Middleware())

	authAs := func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			c.Set("org_id", *orgID)
			c.Set("user_id", "22222222-2222-2222-2222-222222222222")
			c.Set("roles", []string{"Admin"})
			return next(c)
		}
	}

	api := e.Group("/api/v1")
	RegisterRoutes(api, inst, authAs, pool)
	api.GET("/sso-gated", func(c echo.Context) error {
		return c.NoContent(http.StatusOK)
	}, authAs, Require(FeatureSSO))
	return e
}

func TestActivatedKeySurvivesRestartAndStaysOrgSpecific(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()

	pool := startPostgres(ctx, t)

	priv, restore := setupTestKeys(t)
	defer restore()

	exp := time.Now().Add(365 * 24 * time.Hour).Unix()
	mkKey := func(org string, feats []string) string {
		return makeTestKey(t, priv, payload{
			Tier: "pro", Features: feats, Org: org,
			IssuedAt: time.Now().Unix(), Exp: &exp,
		})
	}
	orgA := "11111111-1111-1111-1111-111111111111"
	orgB := "33333333-3333-3333-3333-333333333333"
	// Beide Schluessel tragen SSO, damit die org-lose Route unabhaengig davon
	// prueft, welcher der beiden gerade die Instanz-Lizenz ist. Unterschieden
	// werden sie ueber OrgName und je ein eigenes Feature.
	keyA := mkKey("Org A", []string{FeatureSSO, FeatureMultiFramework})
	keyB := mkKey("Org B", []string{FeatureSSO, FeatureAuditPDF})

	// --- Lauf 1: beide Orgs aktivieren ueber die API ---
	current := orgA
	inst := NewInstance(Load("", false)) // kein env-Schluessel
	e := mountWithDB(t, inst, pool, &current)

	post := func(key string) *httptest.ResponseRecorder {
		body, _ := json.Marshal(activateRequest{Key: key})
		req := httptest.NewRequest(http.MethodPost, "/api/v1/license/activate", strings.NewReader(string(body)))
		req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
		req.AddCookie(&http.Cookie{Name: csrfCookieName, Value: "tok"})
		req.Header.Set(csrfHeaderName, "tok")
		rec := httptest.NewRecorder()
		e.ServeHTTP(rec, req)
		return rec
	}
	require.Equal(t, http.StatusOK, post(keyA).Code)
	current = orgB
	require.Equal(t, http.StatusOK, post(keyB).Code)

	// --- Lauf 2: NEUSTART. Neuer Holder, neuer Router, nichts im Speicher. ---
	inst2 := NewInstance(Load("", false))
	AdoptActivatedKey(ctx, pool, inst2)
	e2 := mountWithDB(t, inst2, pool, &current)

	assert.True(t, inst2.Get().IsPro(),
		"nach dem Neustart faellt die Instanz auf community zurueck — der aktivierte Schluessel steht in license_keys und wird nicht gelesen")

	getLic := func(router *echo.Echo) licenseResponse {
		req := httptest.NewRequest(http.MethodGet, "/api/v1/license", nil)
		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, req)
		require.Equal(t, http.StatusOK, rec.Code)
		var resp licenseResponse
		require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &resp))
		return resp
	}

	// Org B fragt: bekommt ihren eigenen Schluessel (audit_pdf).
	current = orgB
	respB := getLic(e2)
	assert.Equal(t, "pro", respB.Tier)
	assert.Equal(t, "Org B", respB.OrgName)
	assert.Contains(t, respB.Features, FeatureAuditPDF)
	assert.NotContains(t, respB.Features, FeatureMultiFramework)

	// Org A fragt DIESELBE Route: muss ihren eigenen Schluessel bekommen, nicht
	// den zuletzt aktivierten der Instanz.
	current = orgA
	respA := getLic(e2)
	assert.Equal(t, "Org A", respA.OrgName,
		"GET /license antwortet mit der Instanz-Lizenz statt mit der des fragenden Mandanten — die Route haengt ohne DBMiddleware auf der baren api-Gruppe")
	assert.Contains(t, respA.Features, FeatureMultiFramework)
	assert.NotContains(t, respA.Features, FeatureAuditPDF,
		"Org A bekommt die Features von Org B — die Route loest die Lizenz nicht je Mandant auf")

	// Und die Route ausserhalb von `protected` traegt nach dem Neustart wieder
	// die aktivierte Lizenz (R1-17-L04). Sie hat kein org_id zur Verfuegung und
	// wird daher aus der Instanz-Lizenz bedient — das ist der Punkt, den
	// Instance dokumentiert.
	req := httptest.NewRequest(http.MethodGet, "/api/v1/sso-gated", nil)
	rec := httptest.NewRecorder()
	e2.ServeHTTP(rec, req)
	assert.Equal(t, http.StatusOK, rec.Code,
		"die feature-gegatete Route auf der baren api-Gruppe antwortet nach dem Neustart mit 402")
}
