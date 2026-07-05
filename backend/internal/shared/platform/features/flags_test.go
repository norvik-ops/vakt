// Copyright (c) 2026 NorvikOps. All rights reserved.
// SPDX-License-Identifier: Elastic-2.0

package features

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/labstack/echo/v4"

	"github.com/matharnica/vakt/internal/license"
)

func runGate(t *testing.T, lic *license.License, feature Feature) *httptest.ResponseRecorder {
	t.Helper()
	e := echo.New()
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)
	if lic != nil {
		c.Set("license", lic)
	}
	handler := Require(feature)(func(c echo.Context) error {
		return c.NoContent(http.StatusOK)
	})
	if err := handler(c); err != nil {
		t.Fatalf("handler error: %v", err)
	}
	return rec
}

func TestRequireDeniesWithoutLicense(t *testing.T) {
	rec := runGate(t, nil, FeatureAPI)
	if rec.Code != http.StatusPaymentRequired {
		t.Errorf("no license: got %d, want 402", rec.Code)
	}
}

func TestRequireDeniesMissingFeature(t *testing.T) {
	lic := &license.License{Tier: "pro", Features: []string{FeatureSSO}}
	rec := runGate(t, lic, FeatureAPI)
	if rec.Code != http.StatusPaymentRequired {
		t.Errorf("missing feature: got %d, want 402", rec.Code)
	}
}

func TestRequireAllowsIncludedFeature(t *testing.T) {
	lic := &license.License{Tier: "pro", Features: []string{FeatureAPI}}
	rec := runGate(t, lic, FeatureAPI)
	if rec.Code != http.StatusOK {
		t.Errorf("included feature: got %d, want 200", rec.Code)
	}
}

func TestRequireDeniesExpiredLicense(t *testing.T) {
	lic := &license.License{Tier: "pro", Features: []string{FeatureAPI}, Expired: true}
	rec := runGate(t, lic, FeatureAPI)
	if rec.Code != http.StatusPaymentRequired {
		t.Errorf("expired license: got %d, want 402", rec.Code)
	}
}

func TestRequireAllowsDemoLicense(t *testing.T) {
	lic := &license.License{Tier: "community", Demo: true}
	rec := runGate(t, lic, FeatureAPI)
	if rec.Code != http.StatusOK {
		t.Errorf("demo license: got %d, want 200", rec.Code)
	}
}

func TestIsEnabled(t *testing.T) {
	e := echo.New()
	c := e.NewContext(httptest.NewRequest(http.MethodGet, "/", nil), httptest.NewRecorder())
	if IsEnabled(c, FeatureAPI) {
		t.Error("IsEnabled without license must be false")
	}
	c.Set("license", &license.License{Features: []string{FeatureAPI}})
	if !IsEnabled(c, FeatureAPI) {
		t.Error("IsEnabled with feature must be true")
	}
}
