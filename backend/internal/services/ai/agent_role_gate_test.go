// Copyright (c) 2026 NorvikOps. All rights reserved.
// SPDX-License-Identifier: Elastic-2.0

package ai

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/labstack/echo/v4"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestAgentHandlersEnforceWriterRoleThemselves proves the SECOND enforcement
// layer of R1-SA13-01 — the check inside the handlers — is load-bearing on its
// own, with no middleware mounted at all.
//
// It needs its own test because the end-to-end gate in cmd/api cannot see this
// layer: on a Community licence, license.Require(FeatureAgentWriteTools) answers
// 402 before the handler ever runs, so removing the route-level aiWrite there
// produces 402, not a Viewer breakthrough. On the Pro instance where the defect
// was actually observed live (the Viewer got 200, so the licence gate passed)
// this handler check is exactly what stops it.
//
// The handlers are invoked directly with a hand-built context, so nothing but
// requireWriterRole can produce the 403.
func TestAgentHandlersEnforceWriterRoleThemselves(t *testing.T) {
	h := &AgentHandler{runMgr: &AgentRunManager{}}

	handlers := map[string]echo.HandlerFunc{
		"AgentRun":   h.AgentRun,
		"ApproveRun": h.ApproveRun,
		"RejectRun":  h.RejectRun,
	}

	// Roles that must be refused. "Viewer" is the reported defect; the empty and
	// unknown cases pin the deny-by-default shape — an absent roles claim must
	// not be read as "allowed".
	denied := map[string][]string{
		"Viewer":          {"Viewer"},
		"AuditorReadOnly": {"AuditorReadOnly"},
		"no roles claim":  nil,
		"unknown role":    {"Wizard"},
		"lowercase admin": {"admin"}, // exact match, no case folding, no hierarchy
	}

	for hName, handler := range handlers {
		for roleName, roles := range denied {
			t.Run(hName+"/"+roleName, func(t *testing.T) {
				e := echo.New()
				req := httptest.NewRequest(http.MethodPost, "/", strings.NewReader(`{"goal":"x"}`))
				req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
				rec := httptest.NewRecorder()
				c := e.NewContext(req, rec)
				c.Set("org_id", "11111111-1111-1111-1111-111111111111")
				c.Set("user_id", "22222222-2222-2222-2222-222222222222")
				if roles != nil {
					c.Set("roles", roles)
				}

				require.NoError(t, handler(c))
				assert.Equal(t, http.StatusForbidden, rec.Code,
					"%s must refuse role %v inside the handler, independently of any "+
						"middleware. Body: %s", hName, roles, rec.Body.String())
				assert.Contains(t, rec.Body.String(), "AUTH_INSUFFICIENT_ROLE")
			})
		}
	}

	// Non-vacuity: a writer role must NOT be stopped by requireWriterRole. If it
	// were, the assertions above would be green for the trivial reason that the
	// handler rejects everyone.
	for _, role := range WriterRoles {
		t.Run("allowed/"+role, func(t *testing.T) {
			e := echo.New()
			req := httptest.NewRequest(http.MethodPost, "/", nil)
			rec := httptest.NewRecorder()
			c := e.NewContext(req, rec)
			c.Set("org_id", "11111111-1111-1111-1111-111111111111")
			c.Set("roles", []string{role})
			// decideRun is the shared body behind Approve/Reject and returns
			// without touching the network or the DB, so it is the safe probe.
			require.NoError(t, h.decideRun(c, true))
			assert.NotEqual(t, http.StatusForbidden, rec.Code,
				"%s must pass the role gate — it is a writer role. Body: %s", role, rec.Body.String())
		})
	}
}
