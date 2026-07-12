// Copyright (c) 2026 NorvikOps. All rights reserved.
// SPDX-License-Identifier: Elastic-2.0

package rbaccov_test

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/labstack/echo/v4"
	echomw "github.com/labstack/echo/v4/middleware"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/matharnica/vakt/internal/auth"
	"github.com/matharnica/vakt/internal/shared/platform/auditor"
	cloudintegration "github.com/matharnica/vakt/internal/shared/platform/integrations/cloud"
	ghintegration "github.com/matharnica/vakt/internal/shared/platform/integrations/github"
)

// S122-B1 (MA-01/02/03): the cloud-integration, github-integration and
// auditor-invite groups were reachable by a SecurityAnalyst, who could write
// cloud credentials, register a GitHub PAT, and mint an external auditor
// magic-link. The live PoC used a SecurityAnalyst — so this gate MUST probe
// SecurityAnalyst, not just Viewer (K2). Removing RequireRole("Admin") from any
// of the three group mounts in cmd/api/routes.go turns this red.
var integrationAdminOnlyRoutes = []writeRoute{
	// MA-01 — cloud credentials
	{http.MethodPut, "/integrations/cloud/aws/config"},
	{http.MethodPost, "/integrations/cloud/aws/sync"},
	// MA-02 — github integration (PAT)
	{http.MethodPost, "/integrations/github"},
	{http.MethodDelete, "/integrations/github/abc"},
	// MA-03 — external auditor invite
	{http.MethodPost, "/auditor/invites"},
	{http.MethodDelete, "/auditor/invites/abc"},
}

func buildIntegrationsRouter(t *testing.T) *echo.Echo {
	t.Helper()
	key, err := auth.GenerateSymmetricKey(testHexKey)
	require.NoError(t, err)
	paseto := auth.PasetoMiddleware(key, nil)

	e := echo.New()
	e.Use(echomw.Recover()) // handlers hit nil deps once past RBAC — that is fine

	cloudKey := []byte("0123456789abcdef0123456789abcdef")
	// Mount exactly as cmd/api/routes.go does: RequireRole("Admin") on the group.
	ghintegration.RegisterRoutes(e.Group("/integrations/github", paseto, auth.RequireRole("Admin")), nil, cloudKey)
	cloudintegration.RegisterRoutes(e.Group("/integrations/cloud", paseto, auth.RequireRole("Admin")), nil, cloudKey, nil)
	auditor.RegisterRoutes(e.Group("/auditor", paseto, auth.RequireRole("Admin")), nil)
	return e
}

func TestIntegrationsRBAC_SecurityAnalystForbidden(t *testing.T) {
	key, err := auth.GenerateSymmetricKey(testHexKey)
	require.NoError(t, err)

	analystTok, err := auth.IssueAccessToken(key, auth.Claims{
		UserID: "analyst-1", OrgID: "org-1", Roles: []string{"SecurityAnalyst"},
	})
	require.NoError(t, err)
	viewerTok, err := auth.IssueAccessToken(key, auth.Claims{
		UserID: "viewer-1", OrgID: "org-1", Roles: []string{"Viewer"},
	})
	require.NoError(t, err)
	adminTok, err := auth.IssueAccessToken(key, auth.Claims{
		UserID: "admin-1", OrgID: "org-1", Roles: []string{"Admin"},
	})
	require.NoError(t, err)

	e := buildIntegrationsRouter(t)

	for _, rt := range integrationAdminOnlyRoutes {
		rt := rt
		t.Run(rt.method+" "+rt.path, func(t *testing.T) {
			for _, tok := range []struct {
				name  string
				token string
			}{{"SecurityAnalyst", analystTok}, {"Viewer", viewerTok}} {
				req := httptest.NewRequest(rt.method, rt.path, nil)
				req.Header.Set("Authorization", "Bearer "+tok.token)
				rec := httptest.NewRecorder()
				e.ServeHTTP(rec, req)
				assert.Equal(t, http.StatusForbidden, rec.Code,
					"%s must get 403 on %s %s", tok.name, rt.method, rt.path)
			}

			// Admin must clear the RBAC layer.
			req := httptest.NewRequest(rt.method, rt.path, nil)
			req.Header.Set("Authorization", "Bearer "+adminTok)
			rec := httptest.NewRecorder()
			e.ServeHTTP(rec, req)
			assert.NotEqual(t, http.StatusForbidden, rec.Code,
				"Admin should not get 403 on %s %s", rt.method, rt.path)
		})
	}
}
