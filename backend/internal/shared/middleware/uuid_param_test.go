// Copyright (c) 2026 NorvikOps. All rights reserved.
// SPDX-License-Identifier: Elastic-2.0

package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/labstack/echo/v4"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestValidateUUIDParams asserts the middleware rejects a malformed UUID in a
// UUID-typed path param (400) while leaving valid UUIDs and deliberately
// non-UUID params (:name, :control_ref, :type) untouched. Regression guard for
// the malformed-id -> Postgres 22P02 -> 500 class the live probe surfaced.
func TestValidateUUIDParams(t *testing.T) {
	e := echo.New()
	e.Use(ValidateUUIDParams())
	handlerHit := func(c echo.Context) error { return c.String(http.StatusOK, "ok") }

	e.GET("/controls/:id/measures", handlerHit)
	e.GET("/frameworks/:name/enable", handlerHit)
	e.GET("/soa/entries/:control_ref", handlerHit)
	e.GET("/bsi/reports/:type", handlerHit)
	e.GET("/employees/:eid", handlerHit)
	// 2026-07-16: these three 500'd until the guard learned their names.
	e.GET("/admin/users/:user_id/permissions", handlerHit)
	e.GET("/trust/policies/:policyId/publish", handlerHit)
	e.GET("/incident-reports/:reportId/pdf", handlerHit)
	// S3/D13-A: nested second params that a name-based allowlist would have kept
	// missing forever — the denylist validates them without needing to know they
	// exist. Deliberately non-UUID nested params (:stage, :key, :step_id) must stay
	// untouched.
	e.GET("/controls/:id/tasks/:taskId", handlerHit)
	e.GET("/bsi/target-objects/:id/dependencies/:depId", handlerHit)
	e.GET("/bsi/target-objects/:id/risks/:riskId", handlerHit)
	e.GET("/access-reviews/:id/items/:itemId", handlerHit)
	e.GET("/incidents/:id/nis2/submit/:stage", handlerHit)
	e.GET("/projects/:project_id/envs/:env_id/secrets/:key", handlerHit)
	e.GET("/hr/checklist-runs/:id/steps/:step_id", handlerHit)

	cases := []struct {
		name string
		path string
		want int
	}{
		{"malformed uuid in :id is rejected", "/controls/not-a-uuid/measures", http.StatusBadRequest},
		{"valid uuid in :id passes", "/controls/3f2504e0-4f89-11d3-9a0c-0305e82c3301/measures", http.StatusOK},
		{"malformed uuid in :eid is rejected", "/employees/nope", http.StatusBadRequest},
		{"malformed uuid in :user_id is rejected", "/admin/users/nope/permissions", http.StatusBadRequest},
		{"malformed uuid in :policyId is rejected", "/trust/policies/nope/publish", http.StatusBadRequest},
		{"malformed uuid in :reportId is rejected", "/incident-reports/nope/pdf", http.StatusBadRequest},
		{"valid uuid in :reportId passes", "/incident-reports/3f2504e0-4f89-11d3-9a0c-0305e82c3301/pdf", http.StatusOK},
		{"non-uuid :name param is untouched", "/frameworks/CRA/enable", http.StatusOK},
		{"non-uuid :control_ref param is untouched", "/soa/entries/A.5.1", http.StatusOK},
		{"non-uuid :type param is untouched", "/bsi/reports/A1", http.StatusOK},
		{"malformed nested :taskId is rejected", "/controls/3f2504e0-4f89-11d3-9a0c-0305e82c3301/tasks/not-a-uuid", http.StatusBadRequest},
		{"malformed nested :depId is rejected", "/bsi/target-objects/3f2504e0-4f89-11d3-9a0c-0305e82c3301/dependencies/not-a-uuid", http.StatusBadRequest},
		{"malformed nested :riskId is rejected", "/bsi/target-objects/3f2504e0-4f89-11d3-9a0c-0305e82c3301/risks/not-a-uuid", http.StatusBadRequest},
		{"malformed nested :itemId is rejected", "/access-reviews/3f2504e0-4f89-11d3-9a0c-0305e82c3301/items/not-a-uuid", http.StatusBadRequest},
		{"valid nested :taskId passes", "/controls/3f2504e0-4f89-11d3-9a0c-0305e82c3301/tasks/3f2504e0-4f89-11d3-9a0c-0305e82c3301", http.StatusOK},
		{"non-uuid :stage param is untouched", "/incidents/3f2504e0-4f89-11d3-9a0c-0305e82c3301/nis2/submit/initial", http.StatusOK},
		{"non-uuid :key param is untouched", "/projects/3f2504e0-4f89-11d3-9a0c-0305e82c3301/envs/3f2504e0-4f89-11d3-9a0c-0305e82c3301/secrets/my-secret-name", http.StatusOK},
		{"non-uuid :step_id param is untouched", "/hr/checklist-runs/3f2504e0-4f89-11d3-9a0c-0305e82c3301/steps/step-1", http.StatusOK},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			rec := httptest.NewRecorder()
			e.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, tc.path, nil))
			assert.Equal(t, tc.want, rec.Code, "path %s", tc.path)
		})
	}
}

// TestValidateUUIDParams_EmptyParamRejected: an empty UUID segment IS produced by
// Echo for a "//" in the path (Caddy does not normalise it), and previously fell
// through to a ::uuid cast → 22P02 → 500 (R-H02/S131-D5). Empty is not a valid UUID,
// so the middleware now returns 400 like any other malformed value.
func TestValidateUUIDParams_EmptyParamRejected(t *testing.T) {
	mw := ValidateUUIDParams()
	e := echo.New()
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)
	c.SetParamNames("id")
	c.SetParamValues("")
	err := mw(func(c echo.Context) error { return c.NoContent(http.StatusOK) })(c)
	require.NoError(t, err)
	assert.Equal(t, http.StatusBadRequest, rec.Code)
}

// TestUnknownPathStillReturns404 is the regression guard for the catch-all trap.
//
// Echo's Group.Use() auto-registers RouteNotFound("/*", …) WITH the group's full
// middleware chain, so this guard runs on `/api/v1/*` too. Its param is "*" and
// its value is the unmatched rest-path — which is never a UUID. Without "*" on
// the denylist, every 404 for an authenticated request to an unknown path turned
// into "400 invalid id: must be a UUID".
//
// That mislabel is expensive here specifically: the live 404 is how this project
// finds its recurring "handler exists but no route" class, and frontend hooks
// that map 404 → [] would instead see a 400.
func TestUnknownPathStillReturns404(t *testing.T) {
	e := echo.New()
	g := e.Group("/api/v1")
	g.Use(ValidateUUIDParams()) // triggers Echo's RouteNotFound auto-registration
	g.GET("/things/:id", func(c echo.Context) error { return c.String(http.StatusOK, "ok") })

	for _, path := range []string{
		"/api/v1/does-not-exist",
		"/api/v1/vaktcomply/nope/deeper",
		"/api/v1/policies/export/xlsx",
	} {
		req := httptest.NewRequest(http.MethodGet, path, nil)
		rec := httptest.NewRecorder()
		e.ServeHTTP(rec, req)
		assert.Equal(t, http.StatusNotFound, rec.Code,
			"unknown path %s must stay 404 — a 400 'must be a UUID' hides wiring bugs", path)
	}

	// The guard must still do its job on a real UUID-typed param.
	req := httptest.NewRequest(http.MethodGet, "/api/v1/things/not-a-uuid", nil)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)
	require.Equal(t, http.StatusBadRequest, rec.Code,
		"a malformed :id must still be rejected — the catch-all exemption must not disable the guard")
}
