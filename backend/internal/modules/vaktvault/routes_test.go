// Copyright (c) 2026 NorvikOps. All rights reserved.
// SPDX-License-Identifier: Elastic-2.0

package vaktvault

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/labstack/echo/v4"
	"github.com/labstack/echo/v4/middleware"
	"github.com/stretchr/testify/assert"
)

// TestGetProjectRouteIsRegistered is a regression test for ProjectDetailPage.tsx
// 404ing on every project: Service.GetProject and Repository.GetProject were
// fully implemented, but no route ever called Handler.GetProject — only List
// and Delete were wired for /projects/:id. Found via a live Playwright sweep
// of parameterized detail pages, not a static route diff.
func TestGetProjectRouteIsRegistered(t *testing.T) {
	e := echo.New()
	e.Use(middleware.Recover())
	g := e.Group("")
	Register(g, &Handler{}) // service-less — a 404 here means routing is still broken

	req := httptest.NewRequest(http.MethodGet, "/projects/00000000-0000-0000-0000-000000000000", nil)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	assert.NotEqual(t, http.StatusNotFound, rec.Code,
		"GET /projects/:id must resolve to a handler, not 404")
}
