// Copyright (c) 2026 NorvikOps. All rights reserved.
// SPDX-License-Identifier: Elastic-2.0

package features

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/labstack/echo/v4"

	"github.com/matharnica/vakt/internal/license"
)

func runGate(t *testing.T, lic *license.License, feature Feature) *httptest.ResponseRecorder {
	t.Helper()
	return runGateMethod(t, lic, feature, http.MethodGet)
}

func runGateMethod(t *testing.T, lic *license.License, feature Feature, method string) *httptest.ResponseRecorder {
	t.Helper()
	e := echo.New()
	req := httptest.NewRequest(method, "/", nil)
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

// TestRequireExpiredLicenseAllowsReadsBlocksWrites replaces
// TestRequireDeniesExpiredLicense, which asserted that a GET on an expired Pro
// key returns 402.
//
// That test was not merely wrong, it was one half of a contradiction that shipped
// green: license/middleware_test.go:TestRequireMiddleware_ExpiredProReadOnly sent
// the same GET with the same expired Pro license through the other gate and
// asserted 200. Both ran, both passed, so the codebase had no settled answer to
// "what does an expired license do on a read". The doc on license.License.Expired
// is a promise to a paying customer — "retains read access […] prevents data
// lock-out" — and the 402 body already told them "Your data is still readable",
// so the promise decides and this expectation flips. The case itself is kept:
// an expired key is still checked here, and now on every method.
func TestRequireExpiredLicenseAllowsReadsBlocksWrites(t *testing.T) {
	lic := &license.License{Tier: "pro", Features: []string{FeatureAPI}, Expired: true}

	for _, method := range []string{http.MethodGet, http.MethodHead} {
		rec := runGateMethod(t, lic, FeatureAPI, method)
		if rec.Code != http.StatusOK {
			t.Errorf("expired Pro, %s: got %d, want 200 — read access must survive expiry "+
				"(license.License.Expired doc; R1-17-01)", method, rec.Code)
		}
	}

	for _, method := range []string{http.MethodPost, http.MethodPut, http.MethodPatch, http.MethodDelete} {
		rec := runGateMethod(t, lic, FeatureAPI, method)
		if rec.Code != http.StatusPaymentRequired {
			t.Errorf("expired Pro, %s: got %d, want 402 — writes must stay blocked", method, rec.Code)
		}
		if body := rec.Body.String(); !strings.Contains(body, "license_expired") {
			t.Errorf("expired Pro, %s: body %q must carry license_expired, not the "+
				"generic feature_not_available — the customer has paid before and needs "+
				"the renewal prompt", method, body)
		}
	}
}

// TestCommunityLicenceIsUnaffectedByTheExpiryCarveOut pins the edition that has
// no expiry at all. A Community license carries Expired == false, so it can never
// reach the read-only branch — the carve-out must not become a way to read Pro
// data for free. This is the baseline the fix must not move.
func TestCommunityLicenceIsUnaffectedByTheExpiryCarveOut(t *testing.T) {
	lic := &license.License{Tier: "community", Features: []string{}}
	for _, method := range []string{http.MethodGet, http.MethodHead, http.MethodPost, http.MethodDelete} {
		rec := runGateMethod(t, lic, FeatureAPI, method)
		if rec.Code != http.StatusPaymentRequired {
			t.Errorf("community, %s: got %d, want 402 — the expiry carve-out must not "+
				"leak Pro features into the free edition", method, rec.Code)
		}
	}
}

// TestValidProLicenceIsUnaffectedByTheExpiryCarveOut pins the other baseline:
// a live Pro key passes on every method, reads and writes alike.
func TestValidProLicenceIsUnaffectedByTheExpiryCarveOut(t *testing.T) {
	lic := &license.License{Tier: "pro", Features: []string{FeatureAPI}}
	for _, method := range []string{http.MethodGet, http.MethodHead, http.MethodPost, http.MethodDelete} {
		rec := runGateMethod(t, lic, FeatureAPI, method)
		if rec.Code != http.StatusOK {
			t.Errorf("valid pro, %s: got %d, want 200", method, rec.Code)
		}
	}
}

// TestRequireIsLicenseRequire is the anti-drift guard. The defect was not that
// features.Require was wrong in an interesting way — it was that it existed at
// all as a second implementation, drifted from the one its own doc named. A
// behavioural equivalence check across the full matrix stays true no matter how
// either side is refactored, and goes red the moment someone reintroduces a
// private copy of the decision here.
func TestRequireIsLicenseRequire(t *testing.T) {
	methods := []string{http.MethodGet, http.MethodHead, http.MethodPost, http.MethodPut, http.MethodPatch, http.MethodDelete}
	licences := map[string]*license.License{
		"nil":               nil,
		"community":         {Tier: "community", Features: []string{}},
		"pro-with":          {Tier: "pro", Features: []string{FeatureAPI}},
		"pro-without":       {Tier: "pro", Features: []string{FeatureSSO}},
		"pro-expired":       {Tier: "pro", Features: []string{FeatureAPI}, Expired: true},
		"pro-expired-bare":  {Tier: "pro", Features: []string{}, Expired: true},
		"demo":              {Tier: "community", Demo: true},
		"expired-community": {Tier: "community", Features: []string{FeatureAPI}, Expired: true},
	}

	for name, lic := range licences {
		for _, method := range methods {
			mine := runGateMethod(t, lic, FeatureAPI, method)

			e := echo.New()
			rec := httptest.NewRecorder()
			c := e.NewContext(httptest.NewRequest(method, "/", nil), rec)
			if lic != nil {
				c.Set("license", lic)
			}
			handler := license.Require(FeatureAPI)(func(c echo.Context) error {
				return c.NoContent(http.StatusOK)
			})
			if err := handler(c); err != nil {
				t.Fatalf("license.Require handler error: %v", err)
			}

			if mine.Code != rec.Code {
				t.Errorf("%s/%s: features.Require says %d, license.Require says %d — "+
					"the two gates disagree again. features.Require must delegate, not "+
					"reimplement (R1-17-01)", name, method, mine.Code, rec.Code)
			}
		}
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
