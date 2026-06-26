package vakthr_test

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/labstack/echo/v4"
	echomw "github.com/labstack/echo/v4/middleware"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/matharnica/vakt/internal/auth"
	"github.com/matharnica/vakt/internal/modules/vakthr"
)

const testHexKey = "0102030405060708090a0b0c0d0e0f101112131415161718191a1b1c1d1e1f20"

// TestRBACVaktHR verifies that AuditorReadOnly tokens receive 403 on all
// write endpoints (employee management, onboarding/offboarding, checklists).
func TestRBACVaktHR(t *testing.T) {
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
	e.Use(echomw.Recover())
	h := &vakthr.Handler{}
	g := e.Group("/vakthr", auth.PasetoMiddleware(key, nil))
	vakthr.Register(g, h)

	writeRoutes := []struct {
		method string
		path   string
	}{
		// Employee CRUD (admin only)
		{http.MethodPost, "/vakthr/employees"},
		{http.MethodPut, "/vakthr/employees/1"},
		{http.MethodDelete, "/vakthr/employees/1"},
		// Lifecycle transitions (admin only)
		{http.MethodPost, "/vakthr/employees/1/onboard"},
		{http.MethodPost, "/vakthr/employees/1/offboard"},
		// Checklist templates (admin only)
		{http.MethodPost, "/vakthr/checklists"},
		{http.MethodDelete, "/vakthr/checklists/1"},
		// Checklist runs (admin only to start; analyst can update progress)
		{http.MethodPost, "/vakthr/checklist-runs"},
	}

	for _, tc := range writeRoutes {
		tc := tc
		t.Run(tc.method+" "+tc.path, func(t *testing.T) {
			req := httptest.NewRequest(tc.method, tc.path, nil)
			req.Header.Set("Authorization", "Bearer "+auditTok)
			rec := httptest.NewRecorder()
			e.ServeHTTP(rec, req)
			assert.Equal(t, http.StatusForbidden, rec.Code,
				"AuditorReadOnly should get 403 on %s %s", tc.method, tc.path)

			req2 := httptest.NewRequest(tc.method, tc.path, nil)
			req2.Header.Set("Authorization", "Bearer "+adminTok)
			rec2 := httptest.NewRecorder()
			e.ServeHTTP(rec2, req2)
			assert.NotEqual(t, http.StatusForbidden, rec2.Code,
				"Admin should not get 403 on %s %s", tc.method, tc.path)
		})
	}
}
