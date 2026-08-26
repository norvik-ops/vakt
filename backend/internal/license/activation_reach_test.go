// Copyright (c) 2026 NorvikOps. All rights reserved.
// SPDX-License-Identifier: Elastic-2.0

package license

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/labstack/echo/v4"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Dieser Test faehrt den A/B-Vergleich aus R1-17-L04 nach: DERSELBE Schluessel,
// einmal als Umgebungsvariable gesetzt, einmal ueber POST /license/activate
// aktiviert. Beide Wege muessen zum selben Ergebnis fuehren.
//
// Warum der Nachbau der Gruppen-Montage und nicht der echte Baum aus
// cmd/api/routes.go: das license-Paket muss publicKeyPEM ersetzen koennen
// (setupTestKeys), um ueberhaupt einen gueltigen Schluessel zu haben — der echte
// Signierschluessel liegt in der Signierumgebung ausserhalb dieses Repos und ist
// hier nicht erreichbar. Der
// Nachbau bildet exakt die drei Stellen ab, auf die es ankommt:
//
//	1. die globale Middleware aus cmd/api/middleware.go, die auf JEDER Route
//	   c.Set("license", …) mit der Instanz-Lizenz macht,
//	2. die `protected`-Gruppe mit DBMiddleware (cmd/api/routes.go:365),
//	3. eine Route auf der baren `api`-Gruppe mit einem Feature-Gate — die Klasse,
//	   in der /auth/oidc/*, /auth/saml/* und /scim/v2/* liegen.
//
// Require statt features.Require, weil internal/shared/platform/features das
// license-Paket importiert (Zyklus). Beide lesen dieselbe Context-Lizenz und
// entscheiden ueber lic.Has(feature) — fuer diesen Repro sind sie deckungsgleich.

// mountLikeProduction baut die Routen-Montage aus cmd/api nach und gibt den
// Echo-Router zurueck. staticLic ist das, was license.Load(cfg.LicenseKey, …)
// beim Start liefert.
func mountLikeProduction(t *testing.T, staticLic *License) (*echo.Echo, *Handler) {
	t.Helper()
	e := echo.New()
	inst := NewInstance(staticLic)

	// (1) cmd/api/middleware.go: globale Lizenz auf jeder Route.
	e.Use(inst.Middleware())

	// Steht fuer auth.AuthMiddleware: setzt org_id und roles.
	fakeAuth := func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			c.Set("org_id", "11111111-1111-1111-1111-111111111111")
			c.Set("user_id", "22222222-2222-2222-2222-222222222222")
			c.Set("roles", []string{"Admin"})
			return next(c)
		}
	}

	api := e.Group("/api/v1")
	h := RegisterRoutes(api, inst, fakeAuth, nil)

	// (3) Feature-gegatete Route auf der BAREN api-Gruppe — die Klasse aus L04.
	api.GET("/sso-gated", func(c echo.Context) error {
		return c.NoContent(http.StatusOK)
	}, fakeAuth, Require(FeatureSSO))

	return e, h
}

func activate(t *testing.T, e *echo.Echo, key string) *httptest.ResponseRecorder {
	t.Helper()
	body, err := json.Marshal(activateRequest{Key: key})
	require.NoError(t, err)
	req := httptest.NewRequest(http.MethodPost, "/api/v1/license/activate", strings.NewReader(string(body)))
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	req.AddCookie(&http.Cookie{Name: csrfCookieName, Value: "tok"})
	req.Header.Set(csrfHeaderName, "tok")
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)
	return rec
}

func getLicense(t *testing.T, e *echo.Echo) (int, licenseResponse) {
	t.Helper()
	req := httptest.NewRequest(http.MethodGet, "/api/v1/license", nil)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)
	var resp licenseResponse
	if rec.Code == http.StatusOK {
		require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &resp))
	}
	return rec.Code, resp
}

func getGated(t *testing.T, e *echo.Echo) int {
	t.Helper()
	req := httptest.NewRequest(http.MethodGet, "/api/v1/sso-gated", nil)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)
	return rec.Code
}

// TestSameKey_EnvVarAndActivation_AgreeOnFeatures ist der Regressionstest zu
// R1-17-L04 (Feature-Gates ausserhalb von `protected`) und R1-17-L05/R1-07-B02
// (GET /license meldet nach der Aktivierung weiter "community").
func TestSameKey_EnvVarAndActivation_AgreeOnFeatures(t *testing.T) {
	priv, restore := setupTestKeys(t)
	defer restore()

	exp := time.Now().Add(365 * 24 * time.Hour).Unix()
	key := makeTestKey(t, priv, payload{
		Tier:     "pro",
		Features: []string{FeatureSSO, FeatureAuditPDF},
		Org:      "Acme GmbH",
		IssuedAt: time.Now().Unix(),
		Exp:      &exp,
	})

	// --- A: derselbe Schluessel als Umgebungsvariable (VAKT_LICENSE_KEY) ---
	t.Run("als Umgebungsvariable", func(t *testing.T) {
		e, _ := mountLikeProduction(t, Load(key, false))

		code, resp := getLicense(t, e)
		require.Equal(t, http.StatusOK, code)
		assert.Equal(t, "pro", resp.Tier, "GET /license meldet den env-Schluessel als pro")
		assert.True(t, resp.IsPro)

		assert.Equal(t, http.StatusOK, getGated(t, e),
			"die feature-gegatete Route auf der baren api-Gruppe ist mit env-Schluessel offen")
	})

	// --- B: derselbe Schluessel ueber POST /license/activate ---
	t.Run("ueber POST license activate", func(t *testing.T) {
		e, _ := mountLikeProduction(t, Load("", false)) // kein env-Schluessel: community

		rec := activate(t, e, key)
		require.Equal(t, http.StatusOK, rec.Code, "Aktivierung muss gelingen: %s", rec.Body.String())
		var actResp licenseResponse
		require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &actResp))
		require.Equal(t, "pro", actResp.Tier, "die Aktivierung selbst antwortet mit pro")

		// R1-17-L05 / R1-07-B02: dieselbe Instanz, direkt danach gefragt.
		code, resp := getLicense(t, e)
		require.Equal(t, http.StatusOK, code)
		assert.Equal(t, "pro", resp.Tier,
			"GET /license meldet nach erfolgreicher Aktivierung weiter community — vier Frontend-Konsumenten verstecken darauf die Pro-Oberflaeche")
		assert.True(t, resp.IsPro, "is_pro muss nach der Aktivierung true sein")

		// R1-17-L04: die Route ausserhalb von `protected`.
		assert.Equal(t, http.StatusOK, getGated(t, e),
			"eine per UI aktivierte Pro-Lizenz erreicht die Routen auf der baren api-Gruppe nicht (402)")
	})
}
