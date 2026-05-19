// Copyright (c) 2026 NorvikOps. All rights reserved.
// SPDX-License-Identifier: Elastic-2.0

package license

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/labstack/echo/v4"
	"github.com/stretchr/testify/assert"
)

// makeRequest builds an Echo context with the given license set under "license",
// runs it through the Require(feature) middleware, and returns the response recorder.
func makeRequest(e *echo.Echo, lic *License, feature string) *httptest.ResponseRecorder {
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)
	if lic != nil {
		c.Set("license", lic)
	}

	handler := Require(feature)(func(c echo.Context) error {
		return c.JSON(http.StatusOK, map[string]string{"ok": "true"})
	})
	_ = handler(c)
	return rec
}

func TestRequireMiddleware_LicenseHasFeature(t *testing.T) {
	e := echo.New()

	// Build a license that explicitly contains FeatureSSO.
	lic := &License{
		Tier:     "pro",
		Features: []string{FeatureSSO, FeatureAuditPDF},
	}

	rec := makeRequest(e, lic, FeatureSSO)
	assert.Equal(t, http.StatusOK, rec.Code, "license with feature must pass through")
}

func TestRequireMiddleware_LicenseMissingFeature(t *testing.T) {
	e := echo.New()

	// Community license has no features.
	lic := communityLicense()

	rec := makeRequest(e, lic, FeatureSSO)
	assert.Equal(t, http.StatusPaymentRequired, rec.Code, "license without feature must return 402")
}

func TestRequireMiddleware_NilLicense(t *testing.T) {
	e := echo.New()

	// nil license on context — the middleware must not panic and must return 402.
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)
	// Intentionally do NOT set "license" on the context.

	handler := Require(FeatureSecReflex)(func(c echo.Context) error {
		return c.JSON(http.StatusOK, map[string]string{"ok": "true"})
	})
	_ = handler(c)

	assert.Equal(t, http.StatusPaymentRequired, rec.Code, "nil license must return 402")
}

func TestRequireMiddleware_DemoLicenseGrantsAll(t *testing.T) {
	e := echo.New()

	// Demo license grants every feature.
	lic := demoLicense()

	for _, feature := range allFeatures {
		rec := makeRequest(e, lic, feature)
		assert.Equal(t, http.StatusOK, rec.Code, "demo license must grant feature %q", feature)
	}
}

func TestRequireMiddleware_ProLicenseGrantsNamedFeatures(t *testing.T) {
	e := echo.New()

	grantedFeatures := []string{FeatureTISAX, FeatureDORA, FeatureAuditPDF}
	lic := &License{
		Tier:     "pro",
		Features: grantedFeatures,
	}

	// Granted features must pass.
	for _, f := range grantedFeatures {
		rec := makeRequest(e, lic, f)
		assert.Equal(t, http.StatusOK, rec.Code, "pro license should pass feature %q", f)
	}

	// Features NOT in the list must be rejected.
	for _, f := range allFeatures {
		granted := false
		for _, g := range grantedFeatures {
			if g == f {
				granted = true
				break
			}
		}
		if !granted {
			rec := makeRequest(e, lic, f)
			assert.Equal(t, http.StatusPaymentRequired, rec.Code, "feature %q not in license should be rejected", f)
		}
	}
}

func TestRequireMiddleware_ResponseBodyContainsFeatureName(t *testing.T) {
	e := echo.New()
	lic := communityLicense()

	rec := makeRequest(e, lic, FeatureSecPulse)
	body := rec.Body.String()
	assert.Contains(t, body, FeatureSecPulse, "response body must name the missing feature")
	assert.Contains(t, body, "feature_not_available", "response body must contain error code")
}

func TestRequireMiddleware_WrongTypeOnContext(t *testing.T) {
	e := echo.New()

	// Store a non-*License value under "license" — the type assertion returns nil.
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)
	c.Set("license", "i-am-not-a-license")

	handler := Require(FeatureSSO)(func(c echo.Context) error {
		return c.JSON(http.StatusOK, map[string]string{"ok": "true"})
	})
	_ = handler(c)

	assert.Equal(t, http.StatusPaymentRequired, rec.Code, "non-*License value must result in 402")
}

// TestLicenseCacheKey verifies key format for Redis cache entries.
func TestLicenseCacheKey(t *testing.T) {
	key := licenseCacheKey("org-123")
	assert.Equal(t, "license:org-123", key)

	key2 := licenseCacheKey("other-org")
	assert.NotEqual(t, key, key2)
}
