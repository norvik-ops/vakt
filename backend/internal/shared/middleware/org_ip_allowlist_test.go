// Copyright (c) 2026 NorvikOps. All rights reserved.
// SPDX-License-Identifier: Elastic-2.0

package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/labstack/echo/v4"
	"github.com/stretchr/testify/require"
)

// TestOrgIPAllowlist_SkipsNonAdminPaths is the scope guard for R-H17/S131-C2. The
// middleware is mounted once on the whole `protected` group so it covers every /admin
// route without a variant-miss — but it MUST NOT touch non-/admin routes, or a
// misconfigured org allowlist would lock the org out of the entire product. The path
// guard runs before any DB access, so a nil db here proves non-admin paths never even
// reach the allowlist lookup.
func TestOrgIPAllowlist_SkipsNonAdminPaths(t *testing.T) {
	mw := OrgIPAllowlist(nil) // db must never be dereferenced for non-admin paths
	e := echo.New()

	nonAdmin := []string{
		"/api/v1/vaktcomply/controls",
		"/api/v1/vaktscan/assets",
		"/api/v1/settings/team/members",
		"/api/v1/administrators", // contains "admin" but not the /admin/ segment
	}
	for _, path := range nonAdmin {
		called := false
		h := mw(func(c echo.Context) error { called = true; return c.NoContent(http.StatusOK) })
		req := httptest.NewRequest(http.MethodGet, path, nil)
		rec := httptest.NewRecorder()
		c := e.NewContext(req, rec)
		c.SetPath(path)
		c.Set("org_id", "org-with-a-restrictive-allowlist")
		require.NoError(t, h(c), "path %s", path)
		require.True(t, called, "non-admin path %s must pass through untouched", path)
		require.Equal(t, http.StatusOK, rec.Code, "path %s", path)
	}
}

// TestNormalizeCIDR guards the R-H17 review fix: a bare IPv6 must widen to /128, not
// /32 (a /32 of a 128-bit address ≈ 7.9e28 hosts — a single-host entry would silently
// allow a huge range).
func TestNormalizeCIDR(t *testing.T) {
	cases := map[string]string{
		"192.168.1.10":   "192.168.1.10/32",
		"10.0.0.0/8":     "10.0.0.0/8", // already masked → unchanged
		"2001:db8::1":    "2001:db8::1/128",
		"2001:db8::/48":  "2001:db8::/48", // already masked → unchanged
		"  172.16.0.1  ": "172.16.0.1/32", // trimmed
		"":               "",
	}
	for in, want := range cases {
		require.Equal(t, want, NormalizeCIDR(in), "NormalizeCIDR(%q)", in)
	}
}

// TestParseAllowlistCIDRs verifies the shared parser skips empties/garbage and applies
// the IPv6 fix, so the enforcing middleware and the save-time validator agree.
func TestParseAllowlistCIDRs(t *testing.T) {
	nets := ParseAllowlistCIDRs(" 192.168.1.0/24 , , not-a-cidr , 2001:db8::1 ")
	require.Len(t, nets, 2, "one IPv4 /24 + one IPv6 /128; empty and garbage skipped")
	// The IPv6 host is a /128, not a /32-widened range.
	require.Equal(t, "2001:db8::1/128", nets[1].String())
}
