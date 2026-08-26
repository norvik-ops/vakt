// Copyright (c) 2026 NorvikOps. All rights reserved.
// SPDX-License-Identifier: Elastic-2.0

package main

import (
	"context"
	"encoding/json"
	"net/http"
	"os"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestAIAgentRoutesRejectViewer is the regression gate for R1-SA13-01 (CRITICAL).
//
// The defect: the three AI-agent routes were registered without any
// auth.RequireRole, so a Viewer got 200 on POST /ai/agent/run and the agent
// actually started. A Viewer is the "may read, may change nothing" role; an
// agent run is the opposite — it plans actions and, once approved, executes
// tools (DefaultAgentTools includes the write tool add_control_note). Approve
// and reject are the same capability from the other side: whoever may release a
// run decides whether tools execute. AgentRunManager.Decide only ever answered
// "is this the same org/user", never "is this role allowed to".
//
// Why the REAL setupEcho() tree and not a rebuilt router: the guard has to hold
// through the whole production chain — Paseto auth, CSRF, ValidateUUIDParams,
// RequireModuleAccess on the /vaktcomply group, then the route middleware. A
// Nachbau decides for itself what to mount and can drift from routes.go without
// turning red (the limitation internal/rbaccov documents about itself in
// rbac_test.go). This test asks the actual tree.
//
// Three assertions per route, and the second and third are what make the first
// mean anything:
//
//  1. Viewer → 403 AND code == AUTH_INSUFFICIENT_ROLE. Asserting the code, not
//     just the status, pins the denial to the role gate. A bare 403 check would
//     also pass if CSRF or RequireModuleAccess had rejected the request first —
//     the test would be green for the wrong reason and would stay green if
//     someone removed the role gate again.
//  2. SecurityAnalyst → not 403. Baseline: the writer role must still get in.
//  3. Admin → not 403. Same. Both are expected to be stopped further down the
//     chain by license.Require(FeatureAgentWriteTools) with 402 on a non-Pro
//     licence, which is fine and deliberate: aiWrite is ordered BEFORE the
//     licence gate, so 402 proves the role layer passed them through.
func TestAIAgentRoutesRejectViewer(t *testing.T) {
	dbURL := os.Getenv("VAKT_DB_URL")
	secret := os.Getenv("VAKT_SECRET_KEY")
	if dbURL == "" || secret == "" || os.Getenv("VAKT_REDIS_URL") == "" {
		t.Skip("needs VAKT_DB_URL + VAKT_REDIS_URL + VAKT_SECRET_KEY (CI sets all three)")
	}

	ctx := context.Background()
	pool, err := pgxpool.New(ctx, dbURL)
	require.NoError(t, err, "connect")
	defer pool.Close()

	viewerTok := seedUserWithRole(ctx, t, pool, secret, "Viewer", "viewer")
	// users.role is constrained to admin|editor|viewer; a SecurityAnalyst's
	// denormalised cache value is 'editor'. The authorisation that matters here
	// comes from the org_members role / token claim, not this column.
	analystTok := seedUserWithRole(ctx, t, pool, secret, "SecurityAnalyst", "editor")
	adminTok := seedUserWithRole(ctx, t, pool, secret, "Admin", "admin")
	e, _ := setupEcho(ctx, testConfig())

	// Well-formed on purpose: :run_id is not in nonUUIDParamNames, so
	// ValidateUUIDParams (mounted on `protected`) would answer 400 for a
	// malformed value and the request would never reach the role gate.
	const runID = "00000000-0000-0000-0000-000000000000"
	const base = "/api/v1/vaktcomply/ai"

	routes := []struct {
		method string
		path   string
	}{
		{http.MethodPost, base + "/agent/run"},
		{http.MethodPost, base + "/agent/runs/" + runID + "/approve"},
		{http.MethodPost, base + "/agent/runs/" + runID + "/reject"},
	}

	for _, r := range routes {
		r := r
		t.Run(r.method+" "+r.path, func(t *testing.T) {
			vRec := doRBACReq(e, r.method, r.path, viewerTok)
			require.Equal(t, http.StatusForbidden, vRec.Code,
				"Viewer must get 403 on %s %s — a 200 here is R1-SA13-01: the agent "+
					"runs for a read-only role. Body: %s", r.method, r.path, vRec.Body.String())

			var body struct {
				Code          string   `json:"code"`
				RequiredRoles []string `json:"required_roles"`
			}
			require.NoError(t, json.Unmarshal(vRec.Body.Bytes(), &body), "decode 403 body")
			assert.Equal(t, "AUTH_INSUFFICIENT_ROLE", body.Code,
				"the 403 must come from the role gate, not from CSRF or "+
					"RequireModuleAccess — otherwise this test passes for the wrong reason. Body: %s",
				vRec.Body.String())
			assert.ElementsMatch(t, []string{"Admin", "SecurityAnalyst"}, body.RequiredRoles,
				"403 body should name the roles that would work")

			for role, tok := range map[string]string{"SecurityAnalyst": analystTok, "Admin": adminTok} {
				aRec := doRBACReq(e, r.method, r.path, tok)
				assert.NotEqual(t, http.StatusForbidden, aRec.Code,
					"%s must NOT get 403 on %s %s — if the writer roles are blocked too, the "+
						"Viewer 403 above proves nothing. Body: %s",
					role, r.method, r.path, aRec.Body.String())
			}
		})
	}
}
