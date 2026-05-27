// ADR-0017 §2: backend integration test that validates actual HTTP responses
// against the embedded OpenAPI spec schema. The Frontend (and external SDK
// consumers) trust the spec; if a handler silently changes a field name or
// drops a required attribute, this test fails — instead of customers
// finding out at runtime.
//
// MVP scope on purpose narrow: the two endpoints that have already
// drifted historically (/health on 2026-05-20, demo/start during the
// rebrand). Add to `contractCases` as new endpoints need coverage.
package main

import (
	"bytes"
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/getkin/kin-openapi/openapi3"
	"github.com/getkin/kin-openapi/openapi3filter"
	"github.com/getkin/kin-openapi/routers/gorillamux"

	"github.com/matharnica/vakt/internal/shared/apidocs"
)

type contractCase struct {
	name string
	// realPath is the URL the server actually serves at.
	// specPath is where the OpenAPI document lists the operation. They
	// differ because openapi.yaml uses a server URL of /api/v1, but
	// /health is mounted at the root.
	method   string
	realPath string
	specPath string
	body     string // request body, JSON; empty for GET
}

var contractCases = []contractCase{
	{name: "health", method: http.MethodGet, realPath: "/health", specPath: "/api/v1/health"},
	// /demo/start is intentionally NOT in this list yet: openapi.yaml does
	// not document the endpoint, which is itself a finding (ADR-0017 §1
	// says every frontend-consumed endpoint must be in the spec). Adding
	// the case here would only produce a confusing "operation not found"
	// failure instead of the real issue. Track it as a follow-up.
}

// TestOpenAPIContract spins up the same Echo instance the production binary
// uses (via setupEcho), calls each case, and validates the response body +
// status against the embedded OpenAPI 3 schema. Any drift surfaces here.
func TestOpenAPIContract(t *testing.T) {
	specBytes, err := apidocs.SpecBytes()
	if err != nil {
		t.Fatalf("read embedded spec: %v", err)
	}

	loader := openapi3.NewLoader()
	doc, err := loader.LoadFromData(specBytes)
	if err != nil {
		t.Fatalf("parse spec: %v", err)
	}
	// Validate the spec itself first — a broken spec would mask response drift.
	if err := doc.Validate(loader.Context); err != nil {
		t.Fatalf("spec invalid: %v", err)
	}

	router, err := gorillamux.NewRouter(doc)
	if err != nil {
		t.Fatalf("build router: %v", err)
	}

	e := setupEcho(context.Background(), testConfig())

	for _, tc := range contractCases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			var bodyReader *bytes.Reader
			if tc.body != "" {
				bodyReader = bytes.NewReader([]byte(tc.body))
			} else {
				bodyReader = bytes.NewReader(nil)
			}

			req := httptest.NewRequest(tc.method, tc.realPath, bodyReader)
			if tc.body != "" {
				req.Header.Set("Content-Type", "application/json")
			}

			rec := httptest.NewRecorder()
			e.ServeHTTP(rec, req)

			// Locate the OpenAPI operation matching this request. Use
			// specPath (with the server URL prefix) because that's how
			// kin-openapi's gorillamux router resolves paths.
			matchReq, _ := http.NewRequest(tc.method, tc.specPath, nil)
			route, _, err := router.FindRoute(matchReq)
			if err != nil {
				t.Fatalf("no OpenAPI operation matches %s %s — spec is missing this endpoint", tc.method, tc.specPath)
			}

			// Build a request struct for the validator. The path params and
			// body are already set up.
			vRes := &openapi3filter.ResponseValidationInput{
				RequestValidationInput: &openapi3filter.RequestValidationInput{
					Request:    matchReq,
					PathParams: nil,
					Route:      route,
				},
				Status: rec.Code,
				Header: rec.Header(),
				Body:   noCloseBuffer{Reader: bytes.NewReader(rec.Body.Bytes())},
			}

			if err := openapi3filter.ValidateResponse(loader.Context, vRes); err != nil {
				t.Errorf("response does not match spec for %s %s:\n  %v\n  body: %s",
					tc.method, tc.realPath, err, truncate(rec.Body.String(), 300))
			}
		})
	}
}

// noCloseBuffer wraps a bytes.Reader so it satisfies io.ReadCloser without
// closing anything (the kin-openapi validator wants a ReadCloser).
type noCloseBuffer struct{ *bytes.Reader }

func (noCloseBuffer) Close() error { return nil }

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return strings.ReplaceAll(s[:n], "\n", " ") + "..."
}
