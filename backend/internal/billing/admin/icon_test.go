// Copyright (c) 2026 NorvikOps. All rights reserved.
// SPDX-License-Identifier: Elastic-2.0
// See LICENSE file in the project root for terms.

package admin

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/labstack/echo/v4"
)

func TestAppleTouchIconIsServedAsPNG(t *testing.T) {
	e := echo.New()
	e.GET("/apple-touch-icon.png", AppleTouchIcon)

	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/apple-touch-icon.png", nil))

	if rec.Code != http.StatusOK {
		t.Fatalf("status %d, want 200", rec.Code)
	}
	if ct := rec.Header().Get("Content-Type"); ct != "image/png" {
		t.Fatalf("content-type %q, want image/png", ct)
	}
	if !bytes.HasPrefix(rec.Body.Bytes(), []byte("\x89PNG")) {
		t.Fatal("body is not a PNG")
	}
}
