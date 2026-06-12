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
	return makeRequestMethod(e, lic, feature, http.MethodGet)
}

func makeRequestMethod(e *echo.Echo, lic *License, feature, method string) *httptest.ResponseRecorder {
	req := httptest.NewRequest(method, "/", nil)
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

	// Explicitly-listed features must pass.
	for _, f := range grantedFeatures {
		rec := makeRequest(e, lic, f)
		assert.Equal(t, http.StatusOK, rec.Code, "pro license should pass feature %q", f)
	}

	// Legacy Pro features are implicitly granted even when not listed (S79-1).
	for _, f := range legacyProFeatures {
		rec := makeRequest(e, lic, f)
		assert.Equal(t, http.StatusOK, rec.Code, "legacy pro feature %q should be implicitly granted", f)
	}

	// Features neither explicitly listed NOR in legacyProFeatures must be rejected.
	isGranted := func(f string) bool {
		for _, g := range grantedFeatures {
			if g == f {
				return true
			}
		}
		for _, g := range legacyProFeatures {
			if g == f {
				return true
			}
		}
		return false
	}
	for _, f := range allFeatures {
		if !isGranted(f) {
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

// TestRequireMiddleware_ExpiredProReadOnly verifies S79-3:
// expired Pro licenses allow GET/HEAD but block POST/PUT/DELETE.
func TestRequireMiddleware_ExpiredProReadOnly(t *testing.T) {
	e := echo.New()

	lic := &License{
		Tier:     "pro",
		Features: []string{FeatureAuditPDF, FeatureBSIGrundschutz},
		Expired:  true,
	}

	readMethods := []string{http.MethodGet, http.MethodHead}
	for _, method := range readMethods {
		rec := makeRequestMethod(e, lic, FeatureAuditPDF, method)
		assert.Equal(t, http.StatusOK, rec.Code, "expired Pro license should allow %s on listed feature", method)
	}

	writeMethods := []string{http.MethodPost, http.MethodPut, http.MethodPatch, http.MethodDelete}
	for _, method := range writeMethods {
		rec := makeRequestMethod(e, lic, FeatureAuditPDF, method)
		assert.Equal(t, http.StatusPaymentRequired, rec.Code, "expired Pro license must block %s", method)
		body := rec.Body.String()
		assert.Contains(t, body, "license_expired", "%s must include license_expired error code", method)
	}
}

// TestRequireMiddleware_ExpiredProLegacyFeatureReadOnly verifies that legacy Pro
// features (implicitly granted) are also readable on an expired key (S79-1 + S79-3).
func TestRequireMiddleware_ExpiredProLegacyFeatureReadOnly(t *testing.T) {
	e := echo.New()

	// Key has NO explicit features, but tier=pro + Expired=true.
	lic := &License{Tier: "pro", Features: []string{}, Expired: true}

	for _, f := range legacyProFeatures {
		rec := makeRequestMethod(e, lic, f, http.MethodGet)
		assert.Equal(t, http.StatusOK, rec.Code, "expired Pro key should allow GET on legacy feature %q", f)

		rec = makeRequestMethod(e, lic, f, http.MethodPost)
		assert.Equal(t, http.StatusPaymentRequired, rec.Code, "expired Pro key must block POST on feature %q", f)
	}
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
