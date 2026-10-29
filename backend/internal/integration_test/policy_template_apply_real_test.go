//go:build integration

// Copyright (c) 2026 NorvikOps. All rights reserved.
// SPDX-License-Identifier: Elastic-2.0

package integration_test

import (
	"context"
	"encoding/json"
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// R1-KW-8 (Paket 6): „Aus Vorlage“ auf der Richtlinienseite endete in
// 400 INVALID_UUID_PARAM. Die Route hiess POST /policy-templates/:id/apply, der
// Wert ist aber ein Slug („acceptable-use-policy“), keine UUID. ValidateUUIDParams
// prueft jeden Pfad-Param ausser denen auf seiner Denylist — und "id" steht dort
// zu Recht nicht. Der Handler wurde also nie erreicht.
//
// Der Test laeuft durch complyHTTP, also durch die echte Routenregistrierung
// hinter demselben Guard wie cmd/api/routes.go. Die Aussage am Ende ist die
// Zeile in ck_policies, nicht nur der Statuscode.
func TestPolicyFromBuiltinTemplatePassesTheUUIDGuard(t *testing.T) {
	pool, orgID, cleanup := bootPostgresWithOrg(t)
	defer cleanup()

	adminID := seedUser(t, pool, "vorlage-admin@example.org")
	admin := complyHTTP(t, pool, orgID, adminID, []string{"Admin"})

	rec := doJSON(t, admin, http.MethodPost, "/vaktcomply/policy-templates/acceptable-use-policy/apply", "")
	require.Equal(t, http.StatusCreated, rec.Code, rec.Body.String())

	var created struct {
		ID    string `json:"id"`
		Title string `json:"title"`
	}
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &created))
	require.NotEmpty(t, created.ID)
	assert.Equal(t, "Richtlinie zur akzeptablen Nutzung", created.Title)

	var title string
	require.NoError(t, pool.QueryRow(context.Background(),
		`SELECT title FROM ck_policies WHERE id = $1::uuid AND org_id = $2::uuid`,
		created.ID, orgID).Scan(&title))
	assert.Equal(t, "Richtlinie zur akzeptablen Nutzung", title)

	// Ein unbekannter Slug ist ein 404 des Handlers, kein 400 des Guards.
	rec = doJSON(t, admin, http.MethodPost, "/vaktcomply/policy-templates/gibt-es-nicht/apply", "")
	assert.Equal(t, http.StatusNotFound, rec.Code, rec.Body.String())
}
