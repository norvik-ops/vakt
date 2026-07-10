// Copyright (c) 2026 NorvikOps. All rights reserved.
// SPDX-License-Identifier: Elastic-2.0

package vaktscan

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/labstack/echo/v4"
	"github.com/labstack/echo/v4/middleware"
	"github.com/stretchr/testify/assert"
)

// TestDeleteFindingRouteIsRegistered is a regression test for FindingsPage.tsx's
// delete button (single + bulk) calling DELETE /vaktscan/findings/:id against a
// handler that never existed — found via a live functional sweep, not just a
// route diff, since the frontend path looked plausible on its own.
func TestDeleteFindingRouteIsRegistered(t *testing.T) {
	e := echo.New()
	e.Use(middleware.Recover())
	g := e.Group("")
	Register(g, &Handler{}) // service-less — a 404 here means routing is still broken

	req := httptest.NewRequest(http.MethodDelete, "/findings/00000000-0000-0000-0000-000000000000", nil)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	assert.NotEqual(t, http.StatusNotFound, rec.Code,
		"DELETE /findings/:id must resolve to a handler, not 404")
}
