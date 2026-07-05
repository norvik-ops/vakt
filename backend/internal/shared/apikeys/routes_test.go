// Copyright (c) 2026 NorvikOps. All rights reserved.
// SPDX-License-Identifier: Elastic-2.0

package apikeys

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/labstack/echo/v4"
	"github.com/labstack/echo/v4/middleware"

	"github.com/matharnica/vakt/internal/license"
)

// newTestServer mounts Register behind stub middleware that injects the
// given role plus a Pro license with FeatureAPI. No DB — the RBAC gate must
// reject read-only roles before any handler/service code runs.
func newTestServer(role string) *echo.Echo {
	e := echo.New()
	e.Use(middleware.Recover()) // nil-DB handler panics become 500s
	g := e.Group("/api/v1", func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			c.Set("roles", []string{role})
			c.Set("org_id", "00000000-0000-0000-0000-000000000001")
			c.Set("user_id", "00000000-0000-0000-0000-000000000002")
			c.Set("license", &license.License{Tier: "pro", Features: []string{license.FeatureAPI}})
			return next(c)
		}
	})
	Register(g, nil)
	return e
}

func TestAPIKeyRoutesRBAC(t *testing.T) {
	writeCalls := []struct {
		method, path string
	}{
		{http.MethodPost, "/api/v1/api-keys"},
		{http.MethodDelete, "/api/v1/api-keys/00000000-0000-0000-0000-000000000009"},
		{http.MethodPost, "/api/v1/api-keys/00000000-0000-0000-0000-000000000009/rotate"},
	}

	for _, role := range []string{"Viewer", "AuditorReadOnly", "InternalAuditor"} {
		e := newTestServer(role)
		for _, call := range writeCalls {
			req := httptest.NewRequest(call.method, call.path, strings.NewReader(`{"name":"ci"}`))
			req.Header.Set("Content-Type", "application/json")
			rec := httptest.NewRecorder()
			e.ServeHTTP(rec, req)
			if rec.Code != http.StatusForbidden {
				t.Errorf("%s %s as %s: got %d, want 403", call.method, call.path, role, rec.Code)
			}
			if !strings.Contains(rec.Body.String(), "AUTH_INSUFFICIENT_ROLE") {
				t.Errorf("%s %s as %s: expected AUTH_INSUFFICIENT_ROLE, got %s", call.method, call.path, role, rec.Body.String())
			}
		}
	}
}

func TestAPIKeyCreateAdminPassesGate(t *testing.T) {
	// Admin + SecurityAnalyst pass the role gate. An empty body then fails
	// validation (422) — proof the request reached the handler without a DB.
	for _, role := range []string{"Admin", "SecurityAnalyst"} {
		e := newTestServer(role)
		req := httptest.NewRequest(http.MethodPost, "/api/v1/api-keys", strings.NewReader(`{}`))
		req.Header.Set("Content-Type", "application/json")
		rec := httptest.NewRecorder()
		e.ServeHTTP(rec, req)
		if rec.Code != http.StatusUnprocessableEntity {
			t.Errorf("POST /api-keys as %s: got %d, want 422 (past role gate, failed validation)", role, rec.Code)
		}
	}
}

func TestAPIKeyListNotRoleGated(t *testing.T) {
	// GET stays readable for auditors — but without a DB the handler
	// errors with 500, which still proves the role gate let it through.
	e := newTestServer("AuditorReadOnly")
	req := httptest.NewRequest(http.MethodGet, "/api/v1/api-keys", nil)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)
	if rec.Code == http.StatusForbidden {
		t.Errorf("GET /api-keys as AuditorReadOnly: got 403, list must not be role-gated")
	}
}
