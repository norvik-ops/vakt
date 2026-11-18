//go:build integration

// Copyright (c) 2026 NorvikOps. All rights reserved.
// SPDX-License-Identifier: Elastic-2.0

package integration_test

import (
	"context"
	"encoding/json"
	"net/http"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/labstack/echo/v4"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/matharnica/vakt/internal/modules/vaktcomply"
	sharedmw "github.com/matharnica/vakt/internal/shared/middleware"
)

// Fälschungssichere Richtlinien-Freigabe (Migration 272).
//
// Vorher war das einzige „Wer" einer Freigabe last_updated_by — Freitext, den
// der Client mitschickt —, und PATCH /policies/:id schrieb keinen Audit-Eintrag.
// Jetzt setzt der Server approved_by/approved_at aus der angemeldeten Identität
// und schreibt einen Audit-Eintrag 'approve'. Jede Aussage endet in der
// Datenbank, nicht nur im Statuscode.
func TestPolicyApproval_IdentitaetKommtVomServer(t *testing.T) {
	pool, orgID, cleanup := bootPostgresWithOrg(t)
	defer cleanup()
	ctx := context.Background()

	var alice, mallory, otherOrg string
	require.NoError(t, pool.QueryRow(ctx, `
		INSERT INTO users (email, display_name) VALUES ('alice@example.org', 'Alice Freigabe')
		RETURNING id::text`).Scan(&alice))
	require.NoError(t, pool.QueryRow(ctx, `
		INSERT INTO users (email, display_name) VALUES ('mallory@example.org', 'Mallory')
		RETURNING id::text`).Scan(&mallory))
	require.NoError(t, pool.QueryRow(ctx, `
		INSERT INTO organizations (name, slug) VALUES ('Fremd', 'fremd')
		RETURNING id::text`).Scan(&otherOrg))

	e := policyHTTP(t, pool, orgID, alice, "SecurityAnalyst")

	code, body := ckCall(t, e, http.MethodPost, "/vaktcomply/policies", map[string]any{
		"title": "Passwortrichtlinie",
	})
	require.Equal(t, http.StatusCreated, code, string(body))
	var created struct {
		ID string `json:"id"`
	}
	require.NoError(t, json.Unmarshal(body, &created))
	path := "/vaktcomply/policies/" + created.ID

	t.Run("Viewer_darf_nicht_freigeben", func(t *testing.T) {
		viewer := policyHTTP(t, pool, orgID, mallory, "Viewer")
		code, _ := ckCall(t, viewer, http.MethodPatch, path, map[string]any{
			"title": "Passwortrichtlinie", "status": "active",
		})
		assert.Equal(t, http.StatusForbidden, code)
		assert.Equal(t, "draft", policyStatus(t, pool, orgID, created.ID), "der Viewer darf nichts geschrieben haben")
	})

	t.Run("Fremde_Org_sieht_nichts", func(t *testing.T) {
		foreign := policyHTTP(t, pool, otherOrg, mallory, "Admin")
		code, _ := ckCall(t, foreign, http.MethodGet, path, nil)
		assert.Equal(t, http.StatusNotFound, code)
		code, _ = ckCall(t, foreign, http.MethodPatch, path, map[string]any{
			"title": "übernommen", "status": "active",
		})
		assert.Equal(t, http.StatusNotFound, code)
		assert.Equal(t, "draft", policyStatus(t, pool, orgID, created.ID))
	})

	t.Run("Freigabe_traegt_angemeldeten_Nutzer_nicht_den_Freitext", func(t *testing.T) {
		code, body := ckCall(t, e, http.MethodPatch, path, map[string]any{
			"title":            "Passwortrichtlinie",
			"status":           "active",
			"updated_by":       "Fremder",
			"approved_by":      mallory,                // vom Client gefälscht
			"approved_at":      "2020-01-01T00:00:00Z", // vom Client gefälscht
			"approved_by_name": "Geschäftsführung",
		})
		require.Equal(t, http.StatusOK, code, string(body))

		var resp struct {
			ApprovedBy     *string `json:"approved_by"`
			ApprovedByName *string `json:"approved_by_name"`
			ApprovedAt     *string `json:"approved_at"`
			LastUpdatedBy  string  `json:"last_updated_by"`
		}
		require.NoError(t, json.Unmarshal(body, &resp))
		require.NotNil(t, resp.ApprovedBy)
		assert.Equal(t, alice, *resp.ApprovedBy)
		require.NotNil(t, resp.ApprovedByName)
		assert.Equal(t, "Alice Freigabe", *resp.ApprovedByName)
		require.NotNil(t, resp.ApprovedAt)
		assert.NotContains(t, *resp.ApprovedAt, "2020-01-01")
		assert.Equal(t, "Fremder", resp.LastUpdatedBy, "der Freitext bleibt „Bearbeitet von“")

		var by string
		var recent bool
		require.NoError(t, pool.QueryRow(ctx, `
			SELECT COALESCE(approved_by::text, ''), approved_at > NOW() - INTERVAL '5 minutes'
			  FROM ck_policies WHERE id = $1::uuid AND org_id = $2::uuid`,
			created.ID, orgID).Scan(&by, &recent))
		assert.Equal(t, alice, by, "in der Datenbank steht die angemeldete Identität")
		assert.True(t, recent, "approved_at ist der Serverzeitpunkt")

		var auditUser string
		require.NoError(t, pool.QueryRow(ctx, `
			SELECT COALESCE(user_id::text, '') FROM audit_log
			 WHERE org_id = $1::uuid AND action = 'approve'
			   AND resource_type = 'vakt-comply/policy' AND resource_id = $2`,
			orgID, created.ID).Scan(&auditUser))
		assert.Equal(t, alice, auditUser, "der Audit-Trail nennt die angemeldete Identität")

		code, body = ckCall(t, e, http.MethodGet, path, nil)
		require.Equal(t, http.StatusOK, code)
		assert.Contains(t, string(body), `"approved_by_name":"Alice Freigabe"`)
	})

	t.Run("Zurueck_auf_Entwurf_leert_Freigabe", func(t *testing.T) {
		code, body := ckCall(t, e, http.MethodPatch, path, map[string]any{
			"title": "Passwortrichtlinie", "status": "draft",
		})
		require.Equal(t, http.StatusOK, code, string(body))
		var set bool
		require.NoError(t, pool.QueryRow(ctx, `
			SELECT approved_by IS NOT NULL OR approved_at IS NOT NULL
			  FROM ck_policies WHERE id = $1::uuid AND org_id = $2::uuid`,
			created.ID, orgID).Scan(&set))
		assert.False(t, set)
	})
}

// policyHTTP mounts the real vaktcomply routes like vaktcomplyHTTP, plus the DB
// pool for audit logging — production wires it the same way (cmd/api/routes.go).
func policyHTTP(t *testing.T, pool *pgxpool.Pool, orgID, userID, role string) *echo.Echo {
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
	vaktcomply.Register(g, vaktcomply.NewHandler(vaktcomply.NewService(pool)).WithDB(pool))
	return e
}

func policyStatus(t *testing.T, pool *pgxpool.Pool, orgID, id string) string {
	t.Helper()
	var s string
	require.NoError(t, pool.QueryRow(context.Background(),
		`SELECT status FROM ck_policies WHERE id = $1::uuid AND org_id = $2::uuid`, id, orgID).Scan(&s))
	return s
}
