//go:build integration

// Copyright (c) 2026 NorvikOps. All rights reserved.
// SPDX-License-Identifier: Elastic-2.0

package integration_test

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/labstack/echo/v4"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/matharnica/vakt/internal/modules/vaktcomply"
	sharedmw "github.com/matharnica/vakt/internal/shared/middleware"
)

// Regression guard for R1-14c-12: formal risk acceptance was unreachable.
//
// POST /vaktcomply/risks/:id/accept demanded treatment_status = 'accepted'. That
// value is allowed by nothing — not by the input validation
// (oneof=pending in_progress implemented verified) and not by the CHECK constraint
// on ck_risks.treatment_status. There was therefore no way to satisfy the
// precondition, not even with a hand-written UPDATE, and the fully wired dialog in
// RiskDetailPage.tsx answered 409 from the day it shipped. The documented
// acceptance of a residual risk is evidence an auditor asks for (ISO 27001 6.1.3
// and 8.3, BSI-Grundschutz), so this was a compliance record the product could
// promise but never produce.
//
// The fix reads the register status (open | mitigated | accepted | closed), which
// does allow 'accepted' and which the frontend already used to gate the dialog.
// The treatment_status enum is deliberately NOT widened — see the subtest
// "Behandlungsstatus_bleibt_unveraendert".
//
// A test that only asserts the status code would not catch a handler that answers
// 200 without writing anything, so every assertion below ends in the database.
func TestRiskAcceptance_ReachableAndRecorded(t *testing.T) {
	pool, orgID, cleanup := bootPostgresWithOrg(t)
	defer cleanup()
	ctx := context.Background()

	var userID string
	require.NoError(t, pool.QueryRow(ctx, `
		INSERT INTO users (email) VALUES ('risk-owner@example.org')
		RETURNING id::text`).Scan(&userID))

	e := vaktcomplyHTTP(t, pool, orgID, userID, "SecurityAnalyst")

	// --- a risk in its natural initial state -------------------------------
	code, body := ckCall(t, e, http.MethodPost, "/vaktcomply/risks", map[string]any{
		"title":      "Ungepatchter Grenz-Router",
		"likelihood": 3,
		"impact":     4,
		"treatment":  "accept",
	})
	require.Equal(t, http.StatusCreated, code, string(body))
	var created struct {
		ID              string `json:"id"`
		Status          string `json:"status"`
		TreatmentStatus string `json:"treatment_status"`
	}
	require.NoError(t, json.Unmarshal(body, &created))
	require.NotEmpty(t, created.ID)
	require.Equal(t, "open", created.Status)
	require.Equal(t, "pending", created.TreatmentStatus)
	riskPath := "/vaktcomply/risks/" + created.ID

	t.Run("Baseline_ohne_Akzeptanz_bleibt_ablehnbar", func(t *testing.T) {
		code, body := ckCall(t, e, http.MethodPost, riskPath+"/accept", map[string]any{
			"justification": "zu frueh",
		})
		assert.Equal(t, http.StatusConflict, code, string(body))
		assert.Contains(t, string(body), "CK_RISK_NOT_ACCEPTED_TREATMENT")

		// Und zwar wirklich abgelehnt: es darf nichts geschrieben worden sein.
		acceptedAt, _, _ := readAcceptance(t, pool, orgID, created.ID)
		assert.False(t, acceptedAt, "eine abgelehnte Akzeptanz darf keine Spur hinterlassen")
	})

	t.Run("Behandlungsstatus_bleibt_unveraendert", func(t *testing.T) {
		// Die vier bestehenden Werte verhalten sich unveraendert …
		for _, want := range []string{"pending", "in_progress", "implemented", "verified"} {
			code, body := ckCall(t, e, http.MethodPatch, riskPath+"/treatment", map[string]any{
				"treatment_status": want,
			})
			require.Equal(t, http.StatusOK, code, string(body))

			var got string
			require.NoError(t, pool.QueryRow(ctx,
				`SELECT treatment_status FROM ck_risks WHERE id = $1::uuid AND org_id = $2::uuid`,
				created.ID, orgID).Scan(&got))
			assert.Equal(t, want, got, "der Behandlungsstatus muss in der Datenbank stehen")
		}

		// … und 'accepted' ist dort weiterhin KEIN gueltiger Wert. Das ist der Kern
		// der gewaehlten Lesart: treatment_status ist eine Fortschrittsachse
		// (pending → in_progress → implemented → verified). Haette der Fix diesen
		// Wert zugelassen, muesste eine Akzeptanz das 'verified' ueberschreiben und
		// den Nachweis der umgesetzten Behandlung loeschen.
		code, body := ckCall(t, e, http.MethodPatch, riskPath+"/treatment", map[string]any{
			"treatment_status": "accepted",
		})
		assert.Equal(t, http.StatusUnprocessableEntity, code, string(body))
	})

	t.Run("Ganzer_Weg_bis_in_die_Datenbank", func(t *testing.T) {
		// Schritt 1: die Entscheidung im Risikoregister festhalten.
		code, body := ckCall(t, e, http.MethodPatch, riskPath, map[string]any{
			"title":      "Ungepatchter Grenz-Router",
			"likelihood": 3,
			"impact":     4,
			"status":     "accepted",
			"treatment":  "accept",
		})
		require.Equal(t, http.StatusOK, code, string(body))

		// Schritt 2: die formale Akzeptanz — der Aufruf, der vorher immer 409 gab.
		const justification = "Restrisiko vom Risikoeigner getragen, Ersatz erst Q4."
		code, body = ckCall(t, e, http.MethodPost, riskPath+"/accept", map[string]any{
			"justification": justification,
		})
		require.Equal(t, http.StatusOK, code, string(body))

		// Schritt 3: der Datenbankzustand. Ein Statuscode allein wuerde auch bei
		// einem Handler gruen sein, der nichts schreibt.
		var acceptedBy, storedJustification string
		var treatmentStatus string
		var acceptedAtSet bool
		require.NoError(t, pool.QueryRow(ctx, `
			SELECT risk_accepted_by::text,
			       risk_accepted_at IS NOT NULL,
			       risk_acceptance_justification,
			       treatment_status
			  FROM ck_risks WHERE id = $1::uuid AND org_id = $2::uuid`,
			created.ID, orgID,
		).Scan(&acceptedBy, &acceptedAtSet, &storedJustification, &treatmentStatus))

		assert.Equal(t, userID, acceptedBy, "wer akzeptiert hat, muss nachweisbar sein")
		assert.True(t, acceptedAtSet, "wann akzeptiert wurde, muss nachweisbar sein")
		assert.Equal(t, justification, storedJustification, "die Begruendung ist der Nachweis")
		assert.Equal(t, "verified", treatmentStatus,
			"die Akzeptanz darf den Fortschritt der Behandlung nicht ueberschreiben")
	})

	t.Run("Unbekanntes_Risiko_ist_404_kein_500", func(t *testing.T) {
		code, body := ckCall(t, e, http.MethodPost,
			"/vaktcomply/risks/00000000-0000-0000-0000-000000000000/accept",
			map[string]any{"justification": "egal"})
		assert.Equal(t, http.StatusNotFound, code, string(body))
	})

	t.Run("Viewer_darf_nicht_akzeptieren", func(t *testing.T) {
		// Die Freigabe haengt an derselben Rolle wie das Setzen des Registerstatus
		// (beide Routen tragen rw = Admin|SecurityAnalyst). Ein Viewer kommt an
		// keine der beiden Stellen — sonst waere die Vorbedingung selbst umgehbar.
		viewer := vaktcomplyHTTP(t, pool, orgID, userID, "Viewer")
		code, _ := ckCall(t, viewer, http.MethodPost, riskPath+"/accept",
			map[string]any{"justification": "nicht meine Rolle"})
		assert.Equal(t, http.StatusForbidden, code)

		code, _ = ckCall(t, viewer, http.MethodPatch, riskPath, map[string]any{
			"title": "x", "likelihood": 1, "impact": 1, "status": "accepted", "treatment": "accept",
		})
		assert.Equal(t, http.StatusForbidden, code)
	})
}

// vaktcomplyHTTP mounts the real vaktcomply routes on a real Echo against a real
// database. Only the identity is faked — everything else, including the RBAC guard
// on the write routes and the UUID param guard the production group carries
// (S121-F3), is the wiring that actually ships.
func vaktcomplyHTTP(t *testing.T, pool *pgxpool.Pool, orgID, userID, role string) *echo.Echo {
	t.Helper()
	e := echo.New()
	g := e.Group("/vaktcomply",
		func(next echo.HandlerFunc) echo.HandlerFunc {
			return func(c echo.Context) error {
				c.Set("user_id", userID)
				c.Set("org_id", orgID)
				c.Set("roles", []string{role})
				return next(c)
			}
		},
		sharedmw.ValidateUUIDParams(),
	)
	vaktcomply.Register(g, vaktcomply.NewHandler(vaktcomply.NewService(pool)))
	return e
}

func ckCall(t *testing.T, e *echo.Echo, method, path string, body any) (int, []byte) {
	t.Helper()
	raw, err := json.Marshal(body)
	require.NoError(t, err)
	req := httptest.NewRequest(method, path, bytes.NewReader(raw))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)
	return rec.Code, rec.Body.Bytes()
}

// readAcceptance reports whether an acceptance has been recorded, plus its payload.
func readAcceptance(t *testing.T, pool *pgxpool.Pool, orgID, riskID string) (bool, string, string) {
	t.Helper()
	var set bool
	var by, justification string
	require.NoError(t, pool.QueryRow(context.Background(), `
		SELECT risk_accepted_at IS NOT NULL,
		       COALESCE(risk_accepted_by::text, ''),
		       risk_acceptance_justification
		  FROM ck_risks WHERE id = $1::uuid AND org_id = $2::uuid`,
		riskID, orgID).Scan(&set, &by, &justification))
	return set, by, justification
}
