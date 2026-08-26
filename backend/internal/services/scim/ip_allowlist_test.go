// Copyright (c) 2026 NorvikOps. All rights reserved.
// SPDX-License-Identifier: Elastic-2.0

package scim

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/labstack/echo/v4"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/matharnica/vakt/internal/license"
)

// R1-F3W2A-04 — the SCIM lifecycle sat outside every network guard.
//
// PATCH and DELETE /scim/v2/Users are deprovisioning and role assignment. The
// same effect via PATCH /admin/users/:id/role lies behind two IP allowlists;
// SCIM lay behind none, so a leaked SCIM token worked from any address.
//
// The tests drive the REAL Register wiring, because what was missing was not a
// behaviour but a mount.

func TestSCIMIPAllowlist_blocksAnAddressOutsideTheList(t *testing.T) {
	pool := scimTestDB(t)
	orgID := seedSCIMOrg(t, pool)
	token := seedSCIMToken(t, pool, orgID)

	t.Setenv("VAKT_SCIM_ALLOWED_IPS", "10.9.8.0/24")
	e := scimEchoWithAllowlist(t, pool)

	rec := doSCIMFromIP(t, e, http.MethodDelete,
		"/scim/v2/Users/00000000-0000-0000-0000-000000000001", token, "203.0.113.5:1234")

	assert.Equal(t, http.StatusForbidden, rec.Code,
		"a SCIM deprovisioning arrived from outside the allowlist and was served: %s", rec.Body.String())
	assert.Contains(t, rec.Body.String(), "IP_NOT_ALLOWED")
}

func TestSCIMIPAllowlist_letsTheConfiguredNetworkThrough(t *testing.T) {
	pool := scimTestDB(t)
	orgID := seedSCIMOrg(t, pool)
	token := seedSCIMToken(t, pool, orgID)

	t.Setenv("VAKT_SCIM_ALLOWED_IPS", "203.0.113.0/24")
	e := scimEchoWithAllowlist(t, pool)

	rec := doSCIMFromIP(t, e, http.MethodGet, "/scim/v2/Users", token, "203.0.113.5:1234")

	require.Equal(t, http.StatusOK, rec.Code,
		"the IdP's own network was blocked: %s", rec.Body.String())
}

// Unset means unrestricted — an instance whose IdP has no fixed egress
// addresses must not lose provisioning on upgrade.
func TestSCIMIPAllowlist_unsetIsOpen(t *testing.T) {
	pool := scimTestDB(t)
	orgID := seedSCIMOrg(t, pool)
	token := seedSCIMToken(t, pool, orgID)

	t.Setenv("VAKT_SCIM_ALLOWED_IPS", "")
	e := scimEchoWithAllowlist(t, pool)

	rec := doSCIMFromIP(t, e, http.MethodGet, "/scim/v2/Users", token, "198.51.100.7:5555")
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
}

// A list that parses to nothing is a typo, not a permission. Treating it as
// "allow all" would be a guard that silently did nothing.
func TestSCIMIPAllowlist_unparsableListClosesTheDoor(t *testing.T) {
	pool := scimTestDB(t)
	orgID := seedSCIMOrg(t, pool)
	token := seedSCIMToken(t, pool, orgID)

	t.Setenv("VAKT_SCIM_ALLOWED_IPS", "not-an-ip,also/nonsense")
	e := scimEchoWithAllowlist(t, pool)

	rec := doSCIMFromIP(t, e, http.MethodGet, "/scim/v2/Users", token, "203.0.113.5:1234")
	assert.Equal(t, http.StatusForbidden, rec.Code,
		"every entry was a typo and SCIM stayed wide open: %s", rec.Body.String())
}

// ─── helpers ─────────────────────────────────────────────────────────────────

func scimEchoWithAllowlist(t *testing.T, pool *pgxpool.Pool) *echo.Echo {
	t.Helper()
	e := echo.New()
	g := e.Group("/scim/v2", func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			c.Set("license", &license.License{
				Tier:     "pro",
				Features: []string{license.FeatureSCIMProvisioning},
			})
			return next(c)
		}
	})
	Register(g, pool)
	return e
}

func doSCIMFromIP(t *testing.T, e *echo.Echo, method, path, token, remoteAddr string) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(method, path, nil)
	req.RemoteAddr = remoteAddr
	req.Header.Set("Authorization", "Bearer "+token)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)
	return rec
}
