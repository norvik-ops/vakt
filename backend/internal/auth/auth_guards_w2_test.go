// Copyright (c) 2026 NorvikOps. All rights reserved.
// SPDX-License-Identifier: Elastic-2.0

package auth_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strings"
	"testing"

	"github.com/labstack/echo/v4"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/matharnica/vakt/internal/auth"
	"github.com/matharnica/vakt/internal/config"
)

// R1-07-B07-3 — /auth/oidc/initiate took a provider the callback cannot use.
//
// The user was redirected to the identity provider, signed in there, and only
// then hit 422 on the callback: the failure landed at the latest possible
// moment, after authenticating at a foreign system, where nothing the user can
// do fixes it. The check belongs before the redirect.
func TestOIDCInitiate_refusesAProviderTheCallbackCannotComplete(t *testing.T) {
	h := auth.NewHandler(nil, &config.Config{
		CasdoorURL:      "https://casdoor.example.test",
		CasdoorClientID: "vakt",
		FrontendURL:     "http://localhost:5173",
	})
	e := echo.New()

	rec := httptest.NewRecorder()
	c := e.NewContext(httptest.NewRequest(http.MethodGet, "/api/v1/auth/oidc/initiate?provider=okta", nil), rec)
	require.NoError(t, h.OIDCInitiate(c))

	assert.Equal(t, http.StatusUnprocessableEntity, rec.Code,
		"an unsupported provider was accepted and the user sent off to sign in for nothing: %s", rec.Body.String())
	assert.Contains(t, rec.Body.String(), "AUTH_OIDC_PROVIDER_UNSUPPORTED")
	assert.NotContains(t, rec.Body.String(), "redirect_url",
		"the redirect must not be handed out for a provider the callback rejects")
}

// A supported provider still gets its redirect.
func TestOIDCInitiate_supportedProviderStillRedirects(t *testing.T) {
	h := auth.NewHandler(auth.NewService(nil, nil, mustKey(t)), &config.Config{
		CasdoorURL:      "https://casdoor.example.test",
		CasdoorClientID: "vakt",
		FrontendURL:     "http://localhost:5173",
	})
	e := echo.New()

	rec := httptest.NewRecorder()
	c := e.NewContext(httptest.NewRequest(http.MethodGet, "/api/v1/auth/oidc/initiate?provider=keycloak", nil), rec)
	require.NoError(t, h.OIDCInitiate(c))

	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	var body map[string]string
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &body))
	assert.Contains(t, body["redirect_url"], "casdoor.example.test")
}

// The initiate gate and the callback's validator must name the same providers.
// A struct tag cannot reference a variable, so this is what keeps the two lists
// from drifting apart — and drift here recreates exactly R1-07-B07-3.
func TestOIDCProviders_matchTheCallbackValidatorTag(t *testing.T) {
	field, ok := reflect.TypeOf(auth.OIDCCallbackInput{}).FieldByName("Provider")
	require.True(t, ok, "OIDCCallbackInput.Provider is gone — the parity check is now vacuous")

	tag := field.Tag.Get("validate")
	var oneof string
	for _, part := range strings.Split(tag, ",") {
		if strings.HasPrefix(part, "oneof=") {
			oneof = strings.TrimPrefix(part, "oneof=")
		}
	}
	require.NotEmpty(t, oneof, "the Provider field lost its oneof rule — the callback now takes anything")

	assert.ElementsMatch(t, strings.Fields(oneof), auth.OIDCProviders,
		"initiate and callback disagree about which providers exist — a user can be sent to sign in "+
			"at a provider the callback then refuses (R1-07-B07-3)")
}

// R1-F3W2A-02 — the admin password-reset route mounted itself WITHOUT the
// step-up when the middleware could not be built, and only logged a warning.
//
// It mints a credential for another member of the org; ESK-5 named it the
// weakest-protected admin action. A guard whose absence is merely logged is not
// a guard.
func TestRegisterAdminRoutes_refusesWhenStepUpCannotBeBuilt(t *testing.T) {
	// No secret key -> buildMFASensitive cannot derive the TOTP key -> nil.
	h := auth.NewHandler(auth.NewService(nil, nil, mustKey(t)), &config.Config{})

	e := echo.New()
	g := e.Group("/api/v1", func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			c.Set("roles", []string{"Admin"})
			return next(c)
		}
	})
	auth.RegisterAdminRoutes(g, h)

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/v1/admin/users/victim@example.com/password-reset-token", nil)
	e.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusServiceUnavailable, rec.Code,
		"the password-reset-token route answered without its step-up guard: %s", rec.Body.String())
	assert.Contains(t, rec.Body.String(), "MFA_STEPUP_UNAVAILABLE")
}
