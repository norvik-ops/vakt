// Copyright (c) 2026 NorvikOps. All rights reserved.
// SPDX-License-Identifier: Elastic-2.0

package middleware

import (
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/labstack/echo/v4"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// AdminIPAllowlist is mounted on the WHOLE protected tree (cmd/api/routes.go),
// which is only safe because it passes non-admin paths through. If that scope
// ever inverts, the coverage gate in cmd/api stays green — it only ever asks
// about /admin routes — while every ordinary API call starts returning 403 for
// any operator who set VAKT_ADMIN_ALLOWED_IPS. This test is the counterweight:
// it pins both directions of the scope, at the level of the middleware itself.
//
// Same shape of trap as the ValidateUUIDParams denylist that swallowed the "*"
// catch-all and turned every 404 into a 400 — a guard mounted at the top is only
// as good as its own definition of "not my business".
func TestAdminIPAllowlistScope(t *testing.T) {
	cases := []struct {
		name       string
		routePath  string
		wantBlock  bool
		wantReason string
	}{
		{"admin sub-path", "/api/v1/admin/users/:id/role", true, "admin surface must be restricted"},
		{"admin group root", "/api/v1/admin", true, "the group root is admin surface too"},
		{"admin catch-all", "/api/v1/admin/*", true, "an unknown path under /admin must not be a way around the guard"},
		{"ordinary module route", "/api/v1/vaktcomply/controls", false, "must pass through — the guard sits on the whole protected tree"},
		{"own profile", "/api/v1/auth/me", false, "must pass through"},
		// "administration" is not "admin": a substring match without the
		// separators would restrict unrelated routes and, worse, do it silently.
		{"path that merely contains the word", "/api/v1/vakthr/administration", false, "must pass through"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			// The guard reads the environment once, when it is built — so this
			// must be set before AdminIPAllowlist(), not after.
			t.Setenv("VAKT_ADMIN_ALLOWED_IPS", "10.99.99.0/24")

			e := echo.New()
			e.IPExtractor = echo.ExtractIPDirect()
			handlerRan := false
			e.Add(http.MethodGet, tc.routePath, func(c echo.Context) error {
				handlerRan = true
				return c.NoContent(http.StatusOK)
			}, AdminIPAllowlist())

			// httptest.NewRequest uses 192.0.2.1 (TEST-NET-1) — outside the CIDR above.
			reqPath := strings.ReplaceAll(tc.routePath, ":id", "00000000-0000-0000-0000-000000000000")
			reqPath = strings.ReplaceAll(reqPath, "*", "whatever")
			rec := httptest.NewRecorder()
			e.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, reqPath, nil))

			if tc.wantBlock {
				require.Equal(t, http.StatusForbidden, rec.Code, tc.wantReason)
				assert.Contains(t, rec.Body.String(), "IP_NOT_ALLOWED", tc.wantReason)
				assert.False(t, handlerRan, "handler must not run for a blocked IP")
			} else {
				require.Equal(t, http.StatusOK, rec.Code, tc.wantReason)
				assert.True(t, handlerRan, tc.wantReason)
			}
		})
	}
}

// An unset VAKT_ADMIN_ALLOWED_IPS means "no env restriction" — the documented
// default. Getting this wrong would lock every operator who never configured one
// out of their own admin surface on upgrade.
func TestAdminIPAllowlistUnsetPassesEverything(t *testing.T) {
	t.Setenv("VAKT_ADMIN_ALLOWED_IPS", "")

	e := echo.New()
	e.IPExtractor = echo.ExtractIPDirect()
	e.Add(http.MethodGet, "/api/v1/admin/users", func(c echo.Context) error {
		return c.NoContent(http.StatusOK)
	}, AdminIPAllowlist())

	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/api/v1/admin/users", nil))
	assert.Equal(t, http.StatusOK, rec.Code, "unset allowlist must not restrict anything")
}

// A listed IP must reach the handler — otherwise the guard would be "secure" by
// being permanently closed, which is the failure mode nobody reports as a bug
// until an admin is locked out.
func TestAdminIPAllowlistAllowsListedIP(t *testing.T) {
	t.Setenv("VAKT_ADMIN_ALLOWED_IPS", "192.0.2.0/24")

	e := echo.New()
	e.IPExtractor = echo.ExtractIPDirect()
	e.Add(http.MethodGet, "/api/v1/admin/users", func(c echo.Context) error {
		return c.NoContent(http.StatusOK)
	}, AdminIPAllowlist())

	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/api/v1/admin/users", nil))
	assert.Equal(t, http.StatusOK, rec.Code, "192.0.2.1 is inside 192.0.2.0/24")
}

// A bare address in the allowlist means ONE host. For IPv4 that is /32; for
// IPv6 it is /128. Appending /32 to an IPv6 address does not fail and does not
// warn — it silently widens the entry to 2^96 addresses, which is the whole
// point of the allowlist gone. The bug is invisible in every IPv4 test, so it
// needs its own case per family.
func TestAdminIPAllowlistBareAddressIsASingleHost(t *testing.T) {
	cases := []struct {
		name      string
		allowlist string
		clientIP  string
		wantAllow bool
		why       string
	}{
		{"IPv4 exact host allowed", "192.0.2.1", "192.0.2.1", true, "the listed host itself must pass"},
		{"IPv4 neighbour blocked", "192.0.2.1", "192.0.2.2", false, "a bare IPv4 is /32, not the subnet"},
		{"IPv6 exact host allowed", "2001:db8::1", "2001:db8::1", true, "the listed host itself must pass"},
		{"IPv6 neighbour blocked", "2001:db8::1", "2001:db8::2", false,
			"a bare IPv6 must be /128 — with /32 this neighbour would be allowed"},
		{"IPv6 far address in same /32 blocked", "2001:db8::1", "2001:db8:ffff:ffff::99", false,
			"with the /32 bug this address was inside the range: 2^96 hosts opened by one entry"},
		{"IPv6 ULA neighbour blocked", "fd00::5", "fd00::6", false, "same trap for unique-local addresses"},
		{"explicit IPv6 prefix still honoured", "2001:db8::/64", "2001:db8::dead:beef", true,
			"an entry that already carries a mask must be taken as written"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Setenv("VAKT_ADMIN_ALLOWED_IPS", tc.allowlist)

			e := echo.New()
			e.IPExtractor = echo.ExtractIPDirect()
			e.Add(http.MethodGet, "/api/v1/admin/users", func(c echo.Context) error {
				return c.NoContent(http.StatusOK)
			}, AdminIPAllowlist())

			req := httptest.NewRequest(http.MethodGet, "/api/v1/admin/users", nil)
			req.RemoteAddr = net.JoinHostPort(tc.clientIP, "54321")
			rec := httptest.NewRecorder()
			e.ServeHTTP(rec, req)

			if tc.wantAllow {
				assert.Equal(t, http.StatusOK, rec.Code, tc.why)
			} else {
				assert.Equal(t, http.StatusForbidden, rec.Code, tc.why)
			}
		})
	}
}

func TestIsAdminPath(t *testing.T) {
	for path, want := range map[string]bool{
		"/api/v1/admin":                 true,
		"/api/v1/admin/":                true,
		"/api/v1/admin/users":           true,
		"/api/v1/admin/staging/promote": true,
		"/api/v1/admin/*":               true,
		"/api/v1/vaktcomply/controls":   false,
		"/api/v1/trust-center/config":   false, // known scope gap, documented on the guard
		"/api/v1/vakthr/administration": false,
		"/health":                       false,
	} {
		assert.Equal(t, want, IsAdminPath(path), "IsAdminPath(%q)", path)
	}
}
