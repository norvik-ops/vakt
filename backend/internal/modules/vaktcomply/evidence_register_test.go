// Copyright (c) 2026 NorvikOps. All rights reserved.
// SPDX-License-Identifier: Elastic-2.0

package vaktcomply

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/labstack/echo/v4"
	"github.com/labstack/echo/v4/middleware"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Org-wide evidence register (page „Nachweise", Kernweg 9). The DB-backed
// behaviour (org isolation, attachment count, control/framework join) is pinned
// by internal/integration_test/evidence_register_real_test.go; these tests pin
// what needs no database: the service refuses a missing org, the handler
// answers 401 without one, and the route chain lets every role read while the
// write routes next to it stay closed for a Viewer.

func TestListEvidenceForOrg_RejectsMissingOrg(t *testing.T) {
	s := &Service{} // no repo: the guard must answer before any query
	items, err := s.ListEvidenceForOrg(context.Background(), "")
	require.Error(t, err)
	assert.True(t, errors.Is(err, errMissingOrg))
	assert.Nil(t, items)
}

func TestListOrgEvidence_NoOrgIs401(t *testing.T) {
	e := echo.New()
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	rec := httptest.NewRecorder()
	h := &Handler{}
	require.NoError(t, h.ListOrgEvidence(e.NewContext(req, rec)))
	assert.Equal(t, http.StatusUnauthorized, rec.Code)
}

// serveAs routes one request through the real vaktcomply route table with the
// given role. The handler has no service: a request the role check lets through
// panics at the first service call, which Recover turns into a 500. So "not 403"
// proves the role passed, 403 proves it was stopped before any handler code.
func serveAs(t *testing.T, role, method, path, body string) *httptest.ResponseRecorder {
	t.Helper()
	e := echo.New()
	e.Use(middleware.Recover())
	e.Use(func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			c.Set("roles", []string{role})
			c.Set("org_id", "00000000-0000-0000-0000-0000000000aa")
			c.Set("user_id", "00000000-0000-0000-0000-0000000000bb")
			return next(c)
		}
	})
	registerRoutes(e.Group(""), &Handler{})
	req := httptest.NewRequest(method, path, strings.NewReader(body))
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)
	return rec
}

func TestEvidenceRegister_EveryRoleMayRead(t *testing.T) {
	for _, role := range []string{"Admin", "SecurityAnalyst", "Viewer", "AuditorReadOnly"} {
		rec := serveAs(t, role, http.MethodGet, "/evidence", "")
		assert.NotEqual(t, http.StatusForbidden, rec.Code, "%s must be allowed to read the register", role)
		assert.NotEqual(t, http.StatusNotFound, rec.Code, "GET /evidence must be routed")
		assert.NotEqual(t, http.StatusMethodNotAllowed, rec.Code, "GET /evidence must be routed")
	}
}

// The dialog „Nachweis hinzufügen" with a file posts to the upload route; the
// Anhänge block posts to evidence-files. A Viewer must be stopped on both —
// the UI hides the buttons, the backend is the actual lock.
func TestEvidenceWrites_ViewerIs403(t *testing.T) {
	const ctl = "/controls/00000000-0000-0000-0000-0000000000cc"
	writes := []struct{ method, path string }{
		{http.MethodPost, ctl + "/evidence"},
		{http.MethodPost, ctl + "/evidence/upload"},
		{http.MethodPost, ctl + "/evidence-files"},
		{http.MethodDelete, "/evidence-files/00000000-0000-0000-0000-0000000000dd"},
		{http.MethodPost, "/evidence/00000000-0000-0000-0000-0000000000dd/review"},
	}
	for _, w := range writes {
		for _, role := range []string{"Viewer", "AuditorReadOnly"} {
			rec := serveAs(t, role, w.method, w.path, `{}`)
			assert.Equal(t, http.StatusForbidden, rec.Code, "%s %s as %s", w.method, w.path, role)
		}
		rec := serveAs(t, "SecurityAnalyst", w.method, w.path, `{}`)
		assert.NotEqual(t, http.StatusForbidden, rec.Code, "%s %s as SecurityAnalyst must pass the role check", w.method, w.path)
	}
}
