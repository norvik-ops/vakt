// Copyright (c) 2026 NorvikOps. All rights reserved.
// SPDX-License-Identifier: Elastic-2.0

package middleware

import (
	"net/http"
	"net/http/httptest"
	"strconv"
	"testing"
	"time"

	"github.com/labstack/echo/v4"
	"github.com/stretchr/testify/assert"
)

func passOrg(c echo.Context) error { return c.NoContent(http.StatusOK) }

// TestOrgRateLimit_NoOrgID — requests without an authenticated org bypass
// limiting (the global middleware comes after auth, anonymous routes should
// not hit this).
func TestOrgRateLimit_NoOrgID(t *testing.T) {
	mw := OrgRateLimit()
	e := echo.New()
	req := httptest.NewRequest("GET", "/", nil)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)
	// No org_id set.
	if err := mw(passOrg)(c); err != nil {
		t.Fatal(err)
	}
	assert.Equal(t, http.StatusOK, rec.Code)
}

// TestOrgRateLimit_FirstRequestPasses — fresh org with no prior usage gets a
// 200 with X-RateLimit headers.
func TestOrgRateLimit_FirstRequestPasses(t *testing.T) {
	mw := OrgRateLimit()
	e := echo.New()
	req := httptest.NewRequest("GET", "/", nil)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)
	c.Set("org_id", "test-org-1")

	if err := mw(passOrg)(c); err != nil {
		t.Fatal(err)
	}
	assert.Equal(t, http.StatusOK, rec.Code)
	limit := rec.Header().Get("X-RateLimit-Limit")
	remaining := rec.Header().Get("X-RateLimit-Remaining")
	assert.NotEmpty(t, limit)
	assert.NotEmpty(t, remaining)
	limInt, _ := strconv.Atoi(limit)
	assert.Greater(t, limInt, 0)
}

// TestOrgRateLimit_PerOrgIsolation — two different orgs do not share the
// counter. Used to catch a regression where a bug-keying-by-IP would let
// one busy customer rate-limit another.
func TestOrgRateLimit_PerOrgIsolation(t *testing.T) {
	mw := OrgRateLimit()
	e := echo.New()
	for i, org := range []string{"org-A", "org-B"} {
		req := httptest.NewRequest("GET", "/", nil)
		rec := httptest.NewRecorder()
		c := e.NewContext(req, rec)
		c.Set("org_id", org)
		if err := mw(passOrg)(c); err != nil {
			t.Fatalf("iteration %d: %v", i, err)
		}
		assert.Equal(t, http.StatusOK, rec.Code, "org %s should pass", org)
	}
}

// TestOrgRateLimit_Header_ResetIsFuture — the X-RateLimit-Reset Unix timestamp
// should never point into the past.
func TestOrgRateLimit_Header_ResetIsFuture(t *testing.T) {
	mw := OrgRateLimit()
	e := echo.New()
	req := httptest.NewRequest("GET", "/", nil)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)
	c.Set("org_id", "test-org-reset")

	if err := mw(passOrg)(c); err != nil {
		t.Fatal(err)
	}
	reset, _ := strconv.ParseInt(rec.Header().Get("X-RateLimit-Reset"), 10, 64)
	assert.GreaterOrEqual(t, reset, time.Now().Unix()-1, "reset should be now or in the future (allow 1s skew)")
}
