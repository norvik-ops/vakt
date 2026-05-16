package auth_test

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/labstack/echo/v4"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/sechealth-app/sechealth/internal/auth"
)

// executeWithPasetoMiddleware issues a request through PasetoMiddleware and optional
// additional middleware, returning the recorded response.
func executeWithPasetoMiddleware(
	t *testing.T,
	authHeader string,
	key auth.SymmetricKey,
	extra ...echo.MiddlewareFunc,
) *httptest.ResponseRecorder {
	t.Helper()
	e := echo.New()
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	if authHeader != "" {
		req.Header.Set("Authorization", authHeader)
	}
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)

	handler := func(c echo.Context) error {
		return c.JSON(http.StatusOK, map[string]string{"ok": "true"})
	}

	// Build chain: PasetoMiddleware first, then extra middlewares, then handler.
	middlewares := append([]echo.MiddlewareFunc{auth.PasetoMiddleware(key)}, extra...)
	h := handler
	for i := len(middlewares) - 1; i >= 0; i-- {
		h = middlewares[i](h)
	}

	_ = h(c)
	return rec
}

func TestAuthMiddleware_NoHeader(t *testing.T) {
	key, err := auth.GenerateSymmetricKey(testHexKey)
	require.NoError(t, err)

	rec := executeWithPasetoMiddleware(t, "", key)
	assert.Equal(t, http.StatusUnauthorized, rec.Code)
	assert.Contains(t, rec.Body.String(), "AUTH_MISSING_TOKEN")
}

func TestAuthMiddleware_ValidToken(t *testing.T) {
	key, err := auth.GenerateSymmetricKey(testHexKey)
	require.NoError(t, err)

	claims := auth.Claims{
		UserID: "user-mw-test",
		OrgID:  "org-mw-test",
		Roles:  []string{"Admin"},
	}
	tok, err := auth.IssueAccessToken(key, claims)
	require.NoError(t, err)

	e := echo.New()
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("Authorization", "Bearer "+tok)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)

	var gotUserID, gotOrgID string
	var gotRoles []string

	handler := func(c echo.Context) error {
		gotUserID, _ = c.Get("user_id").(string)
		gotOrgID, _ = c.Get("org_id").(string)
		gotRoles, _ = c.Get("roles").([]string)
		return c.JSON(http.StatusOK, map[string]string{"ok": "true"})
	}
	h := auth.PasetoMiddleware(key)(handler)
	err = h(c)
	require.NoError(t, err)

	assert.Equal(t, http.StatusOK, rec.Code)
	assert.Equal(t, claims.UserID, gotUserID)
	assert.Equal(t, claims.OrgID, gotOrgID)
	assert.Equal(t, claims.Roles, gotRoles)
}

func TestAuthMiddleware_ExpiredToken(t *testing.T) {
	key, err := auth.GenerateSymmetricKey(testHexKey)
	require.NoError(t, err)

	tok, err := auth.IssueAccessTokenWithTTL(key, auth.Claims{
		UserID: "user-exp",
		OrgID:  "org-1",
		Roles:  []string{"Viewer"},
	}, -1*time.Second)
	require.NoError(t, err)

	rec := executeWithPasetoMiddleware(t, "Bearer "+tok, key)
	assert.Equal(t, http.StatusUnauthorized, rec.Code)
	assert.Contains(t, rec.Body.String(), "AUTH_INVALID_TOKEN")
}

func TestRequireRole_AllowedRole(t *testing.T) {
	key, err := auth.GenerateSymmetricKey(testHexKey)
	require.NoError(t, err)

	tok, err := auth.IssueAccessToken(key, auth.Claims{
		UserID: "u1",
		OrgID:  "o1",
		Roles:  []string{"SecurityAnalyst"},
	})
	require.NoError(t, err)

	e := echo.New()
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("Authorization", "Bearer "+tok)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)

	handler := func(c echo.Context) error {
		return c.JSON(http.StatusOK, map[string]string{"ok": "true"})
	}
	chain := auth.PasetoMiddleware(key)(auth.RequireRole("Admin", "SecurityAnalyst")(handler))
	err = chain(c)
	require.NoError(t, err)
	assert.Equal(t, http.StatusOK, rec.Code)
}

func TestRequireRole_ForbiddenRole(t *testing.T) {
	key, err := auth.GenerateSymmetricKey(testHexKey)
	require.NoError(t, err)

	tok, err := auth.IssueAccessToken(key, auth.Claims{
		UserID: "u2",
		OrgID:  "o1",
		Roles:  []string{"Viewer"},
	})
	require.NoError(t, err)

	e := echo.New()
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("Authorization", "Bearer "+tok)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)

	handler := func(c echo.Context) error {
		return c.JSON(http.StatusOK, map[string]string{"ok": "true"})
	}
	chain := auth.PasetoMiddleware(key)(auth.RequireRole("Admin", "SecurityAnalyst")(handler))
	err = chain(c)
	require.NoError(t, err)
	assert.Equal(t, http.StatusForbidden, rec.Code)
	assert.Contains(t, rec.Body.String(), "AUTH_INSUFFICIENT_ROLE")
}
