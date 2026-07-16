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
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			rec := httptest.NewRecorder()
			e.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, tc.path, nil))
			assert.Equal(t, tc.want, rec.Code, "path %s", tc.path)
		})
	}
}

// TestValidateUUIDParams_EmptyParamPasses ensures an empty value (never produced
// by Echo for a matched segment, but defensive) does not 400.
func TestValidateUUIDParams_EmptyParamPasses(t *testing.T) {
	mw := ValidateUUIDParams()
	e := echo.New()
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)
	c.SetParamNames("id")
	c.SetParamValues("")
	err := mw(func(c echo.Context) error { return c.NoContent(http.StatusOK) })(c)
	require.NoError(t, err)
	assert.Equal(t, http.StatusOK, rec.Code)
}
