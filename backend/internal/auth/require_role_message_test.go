// Copyright (c) 2026 NorvikOps. All rights reserved.
// SPDX-License-Identifier: Elastic-2.0

package auth_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/labstack/echo/v4"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/matharnica/vakt/internal/auth"
)

// forbidBody drives RequireRole(allowed...) with a caller holding `held` and
// returns the decoded 403 body. It asserts the status, so a test that wired the
// roles the wrong way round fails here rather than asserting on an empty body.
func forbidBody(t *testing.T, held []string, allowed ...string) map[string]any {
	t.Helper()

	key, err := auth.GenerateSymmetricKey(testHexKey)
	require.NoError(t, err)
	tok, err := auth.IssueAccessToken(key, auth.Claims{UserID: "u1", OrgID: "o1", Roles: held})
	require.NoError(t, err)

	e := echo.New()
	req := httptest.NewRequest(http.MethodPatch, "/", nil)
	req.Header.Set("Authorization", "Bearer "+tok)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)

	handler := func(c echo.Context) error { return c.NoContent(http.StatusNoContent) }
	require.NoError(t, auth.PasetoMiddleware(key, nil)(auth.RequireRole(allowed...)(handler))(c))
	require.Equal(t, http.StatusForbidden, rec.Code)

	var body map[string]any
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &body), "403 body must be JSON: %s", rec.Body.String())
	return body
}

// TestRequireRole_ForbiddenNamesTheMissingRole pins the ESK-13 half of the
// contract: a 403 from RequireRole must say WHICH role was required.
//
// The defect it guards against was measured live: PATCH
// /vaktcomply/audit-program/:id/complete answered a fully authenticated Admin
// with {"code":"AUTH_INSUFFICIENT_ROLE","error":"forbidden"} — no role name
// anywhere. That is the exact route ADR-0055 reserves for InternalAuditor, so
// the one word the operator needed ("InternalAuditor") was the one word the
// product would not say, and the obvious self-help was an UPDATE against
// org_members.
//
// Non-vacuity: every assertion below fails if the role name is dropped from
// either field. Reverting the middleware to the old body
// (map[string]string{"error":"forbidden","code":...}) makes the required_roles
// lookup fail AND the error-string assertions fail — checked by running this
// test against the reverted middleware, not assumed.
func TestRequireRole_ForbiddenNamesTheMissingRole(t *testing.T) {
	t.Run("single role, the SoD case", func(t *testing.T) {
		body := forbidBody(t, []string{"Admin"}, "InternalAuditor")

		assert.Equal(t, "AUTH_INSUFFICIENT_ROLE", body["code"])
		assert.Equal(t, []any{"InternalAuditor"}, body["required_roles"],
			"the machine-readable field must carry the role")
		assert.Contains(t, body["error"], "InternalAuditor",
			"the prose message must name the role too — the frontend surfaces `error`, not `required_roles`")
	})

	t.Run("several roles, in the order the route declares them", func(t *testing.T) {
		body := forbidBody(t, []string{"Viewer"}, "Admin", "SecurityAnalyst")

		assert.Equal(t, []any{"Admin", "SecurityAnalyst"}, body["required_roles"],
			"order must be the route's, not a map iteration order")
		assert.Equal(t, "forbidden: requires role Admin or SecurityAnalyst", body["error"])
	})

	t.Run("no internal detail beyond the role names", func(t *testing.T) {
		body := forbidBody(t, []string{"Viewer"}, "InternalAuditor")

		// P10: the role name is part of the published contract (openapi enums,
		// ADR-0055); a stack trace, a SQL string or a file path is not. The body
		// must therefore consist of exactly these three keys and nothing else.
		keys := make([]string, 0, len(body))
		for k := range body {
			keys = append(keys, k)
		}
		assert.ElementsMatch(t, []string{"error", "code", "required_roles"}, keys,
			"403 body must not grow fields that leak internals")

		// And the caller's own identity/claims must not be echoed back.
		assert.NotContains(t, body["error"], "u1")
		assert.NotContains(t, body["error"], "o1")
		assert.NotContains(t, body["error"], "Viewer", "must not report what the caller HAS, only what the route WANTS")
	})

	t.Run("a caller who holds the role still passes", func(t *testing.T) {
		// The control that keeps the three subtests above from being green for
		// the wrong reason (a middleware that rejects everyone).
		key, err := auth.GenerateSymmetricKey(testHexKey)
		require.NoError(t, err)
		tok, err := auth.IssueAccessToken(key, auth.Claims{UserID: "u2", OrgID: "o1", Roles: []string{"InternalAuditor"}})
		require.NoError(t, err)

		e := echo.New()
		req := httptest.NewRequest(http.MethodPatch, "/", nil)
		req.Header.Set("Authorization", "Bearer "+tok)
		rec := httptest.NewRecorder()
		c := e.NewContext(req, rec)

		handler := func(c echo.Context) error { return c.NoContent(http.StatusNoContent) }
		require.NoError(t, auth.PasetoMiddleware(key, nil)(auth.RequireRole("InternalAuditor")(handler))(c))
		assert.Equal(t, http.StatusNoContent, rec.Code)
	})
}
