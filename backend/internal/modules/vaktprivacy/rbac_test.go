package vaktprivacy_test

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/labstack/echo/v4"
	echomw "github.com/labstack/echo/v4/middleware"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/matharnica/vakt/internal/auth"
	"github.com/matharnica/vakt/internal/modules/vaktprivacy"
)

// testHexKey mirrors the constant from auth package tests.
const testHexKey = "0102030405060708090a0b0c0d0e0f101112131415161718191a1b1c1d1e1f20"

// writeRoutes enumerates every vaktprivacy write endpoint that must be
// protected by RequireRole.  Each entry is (method, path) as registered
// under the /vaktprivacy prefix.
var writeRoutes = []struct {
	method string
	path   string
}{
	{http.MethodPost, "/vaktprivacy/vvt"},
	{http.MethodPut, "/vaktprivacy/vvt/1"},
	{http.MethodDelete, "/vaktprivacy/vvt/1"},
	{http.MethodPost, "/vaktprivacy/avvs"},
	{http.MethodPost, "/vaktprivacy/avvs/from-template"},
	{http.MethodPut, "/vaktprivacy/avvs/1"},
	{http.MethodDelete, "/vaktprivacy/avvs/1"},
	{http.MethodPatch, "/vaktprivacy/avvs/1/scc"},
	{http.MethodPost, "/vaktprivacy/breaches"},
	{http.MethodPut, "/vaktprivacy/breaches/1"},
	{http.MethodDelete, "/vaktprivacy/breaches/1"},
	{http.MethodPost, "/vaktprivacy/breaches/1/notify-authority"},
	{http.MethodPost, "/vaktprivacy/dsr"},
	{http.MethodPut, "/vaktprivacy/dsr/1"},
	{http.MethodDelete, "/vaktprivacy/dsr/1"},
	{http.MethodPost, "/vaktprivacy/dsr/1/resolve"},
	{http.MethodPatch, "/vaktprivacy/dsr/1/assign"},
	{http.MethodPut, "/vaktprivacy/processing-activities/1/retention"},
	{http.MethodPatch, "/vaktprivacy/dsr-portal-settings"},
}

// TestVaktprivacyRBAC verifies that AuditorReadOnly tokens receive 403 on all
// write endpoints and that Admin tokens are allowed through.
func TestVaktprivacyRBAC(t *testing.T) {
	key, err := auth.GenerateSymmetricKey(testHexKey)
	require.NoError(t, err)

	auditTok, err := auth.IssueAccessToken(key, auth.Claims{
		UserID: "auditor-1",
		OrgID:  "org-1",
		Roles:  []string{"AuditorReadOnly"},
	})
	require.NoError(t, err)

	adminTok, err := auth.IssueAccessToken(key, auth.Claims{
		UserID: "admin-1",
		OrgID:  "org-1",
		Roles:  []string{"Admin"},
	})
	require.NoError(t, err)

	e := echo.New()
	e.Use(echomw.Recover()) // catch handler panics from nil services in unit tests
	h := &vaktprivacy.Handler{}
	g := e.Group("/vaktprivacy", auth.PasetoMiddleware(key, nil))
	vaktprivacy.Register(g, h)

	for _, tc := range writeRoutes {
		tc := tc
		t.Run(tc.method+" "+tc.path, func(t *testing.T) {
			// AuditorReadOnly must be rejected.
			req := httptest.NewRequest(tc.method, tc.path, nil)
			req.Header.Set("Authorization", "Bearer "+auditTok)
			rec := httptest.NewRecorder()
			e.ServeHTTP(rec, req)
			assert.Equal(t, http.StatusForbidden, rec.Code,
				"AuditorReadOnly should get 403 on %s %s", tc.method, tc.path)

			// Admin must not be rejected by the RBAC layer (handler itself may
			// return 404/500 without a DB, which is fine — we only care it's not 403).
			req2 := httptest.NewRequest(tc.method, tc.path, nil)
			req2.Header.Set("Authorization", "Bearer "+adminTok)
			rec2 := httptest.NewRecorder()
			e.ServeHTTP(rec2, req2)
			assert.NotEqual(t, http.StatusForbidden, rec2.Code,
				"Admin should not get 403 on %s %s", tc.method, tc.path)
		})
	}
}
