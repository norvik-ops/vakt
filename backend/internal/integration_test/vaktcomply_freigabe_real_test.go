//go:build integration

// Copyright (c) 2026 NorvikOps. All rights reserved.
// SPDX-License-Identifier: Elastic-2.0

package integration_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/labstack/echo/v4"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/matharnica/vakt/internal/modules/vaktcomply"
	sharedmw "github.com/matharnica/vakt/internal/shared/middleware"
)

// R1-W7C-N1: Bei der ISMS-Scope-Freigabe und der Management-Review-Freigabe
// beantworteten Route und Service dieselbe Berechtigungsfrage verschieden — und
// der Service las `c.Get("role")`, einen Kontextschluessel, den im gesamten
// Backend niemand setzt (die AuthMiddleware setzt `roles` als []string). Der
// Vergleich `userRole != "admin"` war damit fuer JEDEN Aufrufer wahr: Beide
// Freigaben antworteten jedem mit 403, auch einem Admin. Der
// ISO-27001-Freigabeweg war still ausgefallen — der Nachweis "geprueft und
// freigegeben" konnte gar nicht erst entstehen.
//
// Warum dieser Test die Fehlerklasse ueberhaupt sieht und die drei geloeschten
// Unit-Tests sie nicht sahen: Jene riefen die Rollenpruefung direkt auf dem
// Service auf und reichten die Rolle als Argument mit. Ein Test, der seine
// Eingabe selbst mitbringt, kann eine kaputte Naht zwischen Middleware und
// Handler nicht sehen. Hier laeuft der Request durch die echte
// Routenregistrierung samt der dort montierten auth.RequireRole-Kette, die
// Identitaet kommt unter demselben Schluessel und Typ in den Kontext, den die
// AuthMiddleware setzt (`roles`, []string — middleware.go:129), und die Aussage
// am Ende ist der DATENBANKZUSTAND, nicht der Statuscode.
func complyHTTP(t *testing.T, pool *pgxpool.Pool, orgID, userID string, roles []string) *echo.Echo {
	t.Helper()
	e := echo.New()
	g := e.Group("/vaktcomply",
		func(next echo.HandlerFunc) echo.HandlerFunc {
			return func(c echo.Context) error {
				c.Set("org_id", orgID)
				c.Set("user_id", userID)
				c.Set("roles", roles)
				return next(c)
			}
		},
		// Derselbe Guard, den cmd/api/routes.go auf `protected` haengt. Er
		// gehoert mit in den Test: Er ist die bewusst gewaehlte
		// Durchsetzungsstelle fuer kaputte UUIDs (S121-F3), und ein Test ohne
		// ihn testet eine Konfiguration, die es nicht gibt.
		sharedmw.ValidateUUIDParams(),
	)
	vaktcomply.Register(g, vaktcomply.NewHandler(vaktcomply.NewService(pool)).WithDB(pool))
	return e
}

// seedUser legt einen Nutzer an und gibt seine ID zurueck. approved_by zeigt auf
// users, also muss der Freigeber existieren, damit der UPDATE ueberhaupt haelt.
func seedUser(t *testing.T, pool *pgxpool.Pool, email string) string {
	t.Helper()
	var id string
	require.NoError(t, pool.QueryRow(context.Background(),
		`INSERT INTO users (email) VALUES ($1) RETURNING id::text`, email).Scan(&id))
	return id
}

func doJSON(t *testing.T, e *echo.Echo, method, path, body string) *httptest.ResponseRecorder {
	t.Helper()
	var r *http.Request
	if body == "" {
		r = httptest.NewRequest(method, path, nil)
	} else {
		r = httptest.NewRequest(method, path, strings.NewReader(body))
		r.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	}
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, r)
	return rec
}

// TestISMSScopeApprovalReachesTheDatabase erteilt die Freigabe mit einer
// berechtigten Rolle durch die echte Kette und prueft danach in der Datenbank
// nach, WER wann WELCHEN Datensatz freigegeben hat. Die Gegenrichtung —
// SecurityAnalyst — wird abgelehnt, und zwar nachweislich ohne die Zeile
// anzufassen.
func TestISMSScopeApprovalReachesTheDatabase(t *testing.T) {
	pool, orgID, cleanup := bootPostgresWithOrg(t)
	defer cleanup()
	ctx := context.Background()

	adminID := seedUser(t, pool, "scope-admin@example.org")
	analystID := seedUser(t, pool, "scope-analyst@example.org")

	// Entwurf anlegen — ueber die echte Route, nicht per SQL, damit der
	// Ausgangszustand derselbe ist, den die Anwendung erzeugt.
	admin := complyHTTP(t, pool, orgID, adminID, []string{"Admin"})
	rec := doJSON(t, admin, http.MethodPost, "/vaktcomply/isms-scope",
		`{"scope_definition":"Geltungsbereich: Rechenzentrum Nord","change_note":"Erstfassung"}`)
	require.Equal(t, http.StatusCreated, rec.Code, rec.Body.String())

	var created struct {
		ID     string `json:"id"`
		Status string `json:"status"`
	}
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &created))
	require.NotEmpty(t, created.ID)
	require.Equal(t, "draft", created.Status)

	// Gegenrichtung zuerst: SecurityAnalyst darf nicht freigeben.
	analyst := complyHTTP(t, pool, orgID, analystID, []string{"SecurityAnalyst"})
	rec = doJSON(t, analyst, http.MethodPost, "/vaktcomply/isms-scope/approve",
		`{"id":"`+created.ID+`"}`)
	require.Equal(t, http.StatusForbidden, rec.Code, rec.Body.String())
	assert.Contains(t, rec.Body.String(), "AUTH_INSUFFICIENT_ROLE")

	var statusAfterDenial string
	require.NoError(t, pool.QueryRow(ctx,
		`SELECT status FROM ck_isms_scope WHERE id = $1::uuid AND org_id = $2::uuid`,
		created.ID, orgID).Scan(&statusAfterDenial))
	assert.Equal(t, "draft", statusAfterDenial,
		"eine abgelehnte Freigabe darf den Datensatz nicht veraendert haben")

	// Und jetzt der Weg, der bis zu diesem Fix fuer NIEMANDEN funktioniert hat.
	rec = doJSON(t, admin, http.MethodPost, "/vaktcomply/isms-scope/approve",
		`{"id":"`+created.ID+`"}`)
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())

	// Die eigentliche Aussage: der Datenbankzustand. Ein Test, der nur den
	// Statuscode ansieht, wuerde eine Freigabe akzeptieren, die nichts schreibt.
	var (
		status     string
		approvedBy string
		approvedAt *string
	)
	require.NoError(t, pool.QueryRow(ctx, `
		SELECT status, approved_by::text, approved_at::text
		FROM ck_isms_scope WHERE id = $1::uuid AND org_id = $2::uuid`, created.ID, orgID).
		Scan(&status, &approvedBy, &approvedAt))

	assert.Equal(t, "approved", status, "die Freigabe muss den Status setzen")
	assert.Equal(t, adminID, approvedBy, "es muss der freigebende Nutzer eingetragen sein, nicht irgendeiner")
	require.NotNil(t, approvedAt, "ohne Zeitstempel ist der Nachweis wertlos")
	assert.NotEmpty(t, *approvedAt)

	// Genau dieser eine Datensatz, kein zweiter.
	var approvedCount int
	require.NoError(t, pool.QueryRow(ctx,
		`SELECT count(*) FROM ck_isms_scope WHERE org_id = $1::uuid AND status = 'approved'`, orgID).
		Scan(&approvedCount))
	assert.Equal(t, 1, approvedCount)
}

// TestManagementReviewApprovalReachesTheDatabase ist die zweite Haelfte
// desselben Defekts. Die berechtigte Rolle ist hier InternalAuditor, nicht
// Admin — so steht es in ADR-0055 (Rollenmatrix: Management-Review freigeben,
// Admin ❌, SecurityAnalyst ❌, InternalAuditor ✅). Der geloeschte Service-Check
// verlangte "admin" und widersprach dem ADR; aufgefallen war das nie, weil er
// ohnehin jeden ablehnte.
func TestManagementReviewApprovalReachesTheDatabase(t *testing.T) {
	pool, orgID, cleanup := bootPostgresWithOrg(t)
	defer cleanup()
	ctx := context.Background()

	adminID := seedUser(t, pool, "mr-admin@example.org")
	auditorID := seedUser(t, pool, "mr-auditor@example.org")

	admin := complyHTTP(t, pool, orgID, adminID, []string{"Admin"})
	rec := doJSON(t, admin, http.MethodPost, "/vaktcomply/management-reviews",
		`{"review_date":"2026-07-11","review_type":"annual"}`)
	require.Equal(t, http.StatusCreated, rec.Code, rec.Body.String())

	var created struct {
		ID     string `json:"id"`
		Status string `json:"status"`
	}
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &created))
	require.NotEmpty(t, created.ID)
	require.Equal(t, "draft", created.Status)

	// Gegenrichtung: Der Admin, der das Review angelegt hat, darf es nicht
	// selbst freigeben. Das ist die Trennung aus ADR-0055 und zugleich die
	// Vier-Augen-Wirkung an dieser Stelle.
	rec = doJSON(t, admin, http.MethodPost, "/vaktcomply/management-reviews/"+created.ID+"/approve", "")
	require.Equal(t, http.StatusForbidden, rec.Code, rec.Body.String())
	assert.Contains(t, rec.Body.String(), "AUTH_INSUFFICIENT_ROLE")

	var statusAfterDenial string
	require.NoError(t, pool.QueryRow(ctx,
		`SELECT status FROM ck_management_reviews WHERE id = $1::uuid AND org_id = $2::uuid`,
		created.ID, orgID).Scan(&statusAfterDenial))
	assert.Equal(t, "draft", statusAfterDenial)

	auditor := complyHTTP(t, pool, orgID, auditorID, []string{"InternalAuditor"})
	rec = doJSON(t, auditor, http.MethodPost, "/vaktcomply/management-reviews/"+created.ID+"/approve", "")
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())

	var (
		status     string
		approvedBy string
		approvedAt *string
	)
	require.NoError(t, pool.QueryRow(ctx, `
		SELECT status, approved_by::text, approved_at::text
		FROM ck_management_reviews WHERE id = $1::uuid AND org_id = $2::uuid`, created.ID, orgID).
		Scan(&status, &approvedBy, &approvedAt))

	assert.Equal(t, "approved", status)
	assert.Equal(t, auditorID, approvedBy,
		"der Freigeber muss der Auditor sein — und damit ein anderer als der Ersteller")
	assert.NotEqual(t, created.ID, "")
	require.NotNil(t, approvedAt)
	assert.NotEmpty(t, *approvedAt)

	// Der Ersteller bleibt unveraendert daneben stehen: Erst beide Spalten
	// zusammen belegen, dass Erstellung und Freigabe von verschiedenen Personen
	// kamen.
	var createdBy string
	require.NoError(t, pool.QueryRow(ctx,
		`SELECT created_by::text FROM ck_management_reviews WHERE id = $1::uuid AND org_id = $2::uuid`,
		created.ID, orgID).Scan(&createdBy))
	assert.Equal(t, adminID, createdBy)
	assert.NotEqual(t, createdBy, approvedBy)
}
