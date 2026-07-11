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
	"os"
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
	// S78-7: Auditor session management routes. Hit without auth → 401, which
	// the spec documents. Validates that both paths are present in the spec and
	// that the 401 response schema matches what Echo's auth middleware emits.
	{name: "auditor_sessions_list", method: http.MethodGet,
		realPath: "/api/v1/auditor/sessions", specPath: "/api/v1/auditor/sessions"},
	{name: "auditor_sessions_revoke", method: http.MethodDelete,
		realPath: "/api/v1/auditor/sessions/00000000-0000-0000-0000-000000000001",
		specPath: "/api/v1/auditor/sessions/{id}"},
	// /demo/start is intentionally NOT in this list yet: openapi.yaml does
	// not document the endpoint, which is itself a finding (ADR-0017 §1
	// says every frontend-consumed endpoint must be in the spec). Adding
	// the case here would only produce a confusing "operation not found"
	// failure instead of the real issue. Track it as a follow-up.

	// S80-6: One core list endpoint per module — hit without auth → 401.
	// Each spec entry now documents '401: Unauthorized' so the validator
	// can match the response. Expanding these to authenticated cases requires
	// a seeded test DB (track in ADR-0017 follow-up).
	{name: "vaktscan_findings", method: http.MethodGet,
		realPath: "/api/v1/vaktscan/findings", specPath: "/api/v1/vaktscan/findings"},
	{name: "vaktcomply_frameworks", method: http.MethodGet,
		realPath: "/api/v1/vaktcomply/frameworks", specPath: "/api/v1/vaktcomply/frameworks"},
	{name: "vaktvault_projects", method: http.MethodGet,
		realPath: "/api/v1/vaktvault/projects", specPath: "/api/v1/vaktvault/projects"},
	{name: "vaktprivacy_vvt", method: http.MethodGet,
		realPath: "/api/v1/vaktprivacy/vvt", specPath: "/api/v1/vaktprivacy/vvt"},
	{name: "vakthr_contractors", method: http.MethodGet,
		realPath: "/api/v1/vakthr/contractors", specPath: "/api/v1/vakthr/contractors"},
	// GET /system/update — no auth required for read; returns 200 with UpdateInfo schema.
	{name: "system_update_get", method: http.MethodGet,
		realPath: "/api/v1/system/update", specPath: "/api/v1/system/update"},
	// S105-1: direct user creation — no auth → 401
	{name: "admin_create_user", method: http.MethodPost,
		realPath: "/api/v1/admin/users", specPath: "/api/v1/admin/users",
		body: `{"email":"x@x.com","password":"tencharpass","role":"Viewer"}`},
	// S105-2: OIDC config — no auth → 401
	{name: "admin_oidc_config_get", method: http.MethodGet,
		realPath: "/api/v1/admin/org/oidc-config", specPath: "/api/v1/admin/org/oidc-config"},
	// S105-3: SAML config — no auth → 401 (Pro-gated, but auth runs first)
	{name: "admin_saml_config_get", method: http.MethodGet,
		realPath: "/api/v1/admin/org/saml-config", specPath: "/api/v1/admin/org/saml-config"},
	// Migration 229: SMTP config — no auth → 401
	{name: "admin_smtp_get", method: http.MethodGet,
		realPath: "/api/v1/admin/org/smtp", specPath: "/api/v1/admin/org/smtp"},
	// Migration 230: Backup config — no auth → 401
	{name: "admin_backup_config_get", method: http.MethodGet,
		realPath: "/api/v1/admin/org/backup-config", specPath: "/api/v1/admin/org/backup-config"},
	// Migration 231: LDAP config — no auth → 401
	{name: "admin_ldap_config_get", method: http.MethodGet,
		realPath: "/api/v1/admin/org/ldap", specPath: "/api/v1/admin/org/ldap"},
	// Migration 232: backup dest — no auth → 401
	{name: "admin_backup_dest_get", method: http.MethodGet,
		realPath: "/api/v1/admin/org/backup-dest", specPath: "/api/v1/admin/org/backup-dest"},
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

	e, _ := setupEcho(context.Background(), testConfig())

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

// TestOpenAPIReverseContract verifies that every path+method documented in the
// embedded OpenAPI spec has a corresponding Echo route. This catches the inverse
// of the drift that TestOpenAPIContract covers: a spec entry with no handler
// (dead documentation or forgotten route registration).
//
// Requires VAKT_DB_URL to be set — routes under module groups are only
// registered when a live DB connection is available. In CI this is always set.
// Skip locally when running without a database.
//
// Allowlist entries are spec paths that are intentionally served at a different
// real path — document the reason so future readers know it's intentional.
func TestOpenAPIReverseContract(t *testing.T) {
	if os.Getenv("VAKT_DB_URL") == "" {
		t.Skip("VAKT_DB_URL not set — reverse contract test requires a running database (runs in CI)")
	}

	specBytes, err := apidocs.SpecBytes()
	if err != nil {
		t.Fatalf("read embedded spec: %v", err)
	}

	loader := openapi3.NewLoader()
	doc, err := loader.LoadFromData(specBytes)
	if err != nil {
		t.Fatalf("parse spec: %v", err)
	}

	// Build the set of all registered Echo routes: "METHOD /path".
	e, _ := setupEcho(context.Background(), testConfig())
	echoRoutes := make(map[string]bool)
	for _, r := range e.Routes() {
		echoRoutes[r.Method+" "+r.Path] = true
	}

	// Paths in the spec that are served at a different real URL in Echo.
	// Key format: "METHOD /api/v1<specPath>".
	reverseAllowlist := map[string]string{
		// /health is mounted at the root, not under /api/v1, because it must
		// be reachable before auth middleware resolves.  Covered by TestOpenAPIContract.
		"GET /api/v1/health":       "mounted at /health (root level)",
		"GET /api/v1/health/ready": "mounted at /health/ready (root level)",

		// Demo routes — only registered when cfg.DemoSeed=true (VAKT_DEMO=true).
		// Not set in CI test env by design; routes exist in production demo instances.
		"POST /api/v1/demo/start": "demo-only route, requires VAKT_DEMO=true",
		"POST /api/v1/demo/login": "demo-only route, requires VAKT_DEMO=true",

		// Supplier portal — spec-ahead, implementation pending (TODO: S80+).
		"GET /api/v1/supplier/{token}":         "supplier portal not yet implemented",
		"POST /api/v1/supplier/{token}/save":   "supplier portal not yet implemented",
		"POST /api/v1/supplier/{token}/submit": "supplier portal not yet implemented",
		"POST /api/v1/supplier/{token}/upload": "supplier portal not yet implemented",

		// AI routes — registered conditionally via ai.RegisterWithOptions only when
		// cfg.AIProvider != "disabled". CI does not set VAKT_AI_PROVIDER.
		"GET /api/v1/vaktcomply/ai/status":                       "AI routes require VAKT_AI_PROVIDER set",
		"GET /api/v1/vaktcomply/ai/models":                       "AI routes require VAKT_AI_PROVIDER set",
		"GET /api/v1/vaktcomply/ai/usage":                        "AI routes require VAKT_AI_PROVIDER set",
		"GET /api/v1/vaktcomply/ai/insights":                     "AI routes require VAKT_AI_PROVIDER set",
		"DELETE /api/v1/vaktcomply/ai/insights/{id}":             "AI routes require VAKT_AI_PROVIDER set",
		"POST /api/v1/vaktcomply/ai/report":                      "AI routes require VAKT_AI_PROVIDER set",
		"POST /api/v1/vaktcomply/ai/advice":                      "AI routes require VAKT_AI_PROVIDER set",
		"POST /api/v1/vaktcomply/ai/chat/stream":                 "AI routes require VAKT_AI_PROVIDER set",
		"POST /api/v1/vaktcomply/ai/incident-guide":              "AI routes require VAKT_AI_PROVIDER set",
		"POST /api/v1/vaktcomply/ai/draft-policy":                "AI routes require VAKT_AI_PROVIDER set",
		"POST /api/v1/vaktcomply/ai/controls/{id}/explain":       "AI routes require VAKT_AI_PROVIDER set",
		"POST /api/v1/vaktcomply/ai/risks/{id}/narrative":        "AI routes require VAKT_AI_PROVIDER set",
		"POST /api/v1/vaktcomply/ai/agent/run":                   "AI routes require VAKT_AI_PROVIDER set",
		"POST /api/v1/vaktcomply/ai/agent/runs/{run_id}/approve": "AI routes require VAKT_AI_PROVIDER set",
		"POST /api/v1/vaktcomply/ai/agent/runs/{run_id}/reject":  "AI routes require VAKT_AI_PROVIDER set",

		// S121-F7: controls sub-resource param drift resolved — spec now uses {id}
		// to match routes.go (:id). No allowlist entries needed.

		// Protection needs — code registers /protection-needs/assessments/{id} path.
		// Spec uses shorter /protection-needs/{id}. TODO: align paths in one direction.
		"GET /api/v1/vaktcomply/protection-needs":                "path mismatch: code uses /assessments/ prefix, TODO align",
		"POST /api/v1/vaktcomply/protection-needs":               "path mismatch: code uses /assessments/ prefix, TODO align",
		"GET /api/v1/vaktcomply/protection-needs/{id}":           "path mismatch: code uses /assessments/ prefix, TODO align",
		"PUT /api/v1/vaktcomply/protection-needs/{id}":           "path mismatch: code uses /assessments/ prefix, TODO align",
		"DELETE /api/v1/vaktcomply/protection-needs/{id}":        "path mismatch: code uses /assessments/ prefix, TODO align",
		"POST /api/v1/vaktcomply/protection-needs/{id}/finalize": "path mismatch: code uses /assessments/ prefix, TODO align",

		// CCM checks — spec has PUT /{id} + GET /{id} but code only has PATCH /{id}/toggle.
		"PUT /api/v1/vaktcomply/ccm/checks/{id}": "spec has PUT, code has PATCH /toggle; TODO align",
		"GET /api/v1/vaktcomply/ccm/checks/{id}": "spec-ahead, no single-check GET handler yet; TODO",

		// Policies — spec has DELETE /{id} but code only has PATCH /{id}.
		"DELETE /api/v1/vaktcomply/policies/{id}": "spec-ahead, no DELETE policy handler yet; TODO",

		// Controls measures — spec has PUT with {controlId}, code has PATCH with :id.
		"PUT /api/v1/vaktcomply/controls/{controlId}/measures/{mid}": "spec has PUT, code has PATCH/:id (method + param mismatch); TODO align",

		// S121-F7: the spec-only POST /bcp/plans/{id}/link-evidence op was removed
		// (the backend links BCP evidence via /evidence; the dead FE hook went in
		// F2/C3), so no allowlist entry is needed.
		// S121-D2 (D4): board-report route is now registered — no allowlist entry
		// needed; the reverse-contract gate actively verifies it.

		// DSR single GET — only PUT /{id} registered, no GET /{id} handler yet.
		"GET /api/v1/vaktprivacy/dsr/{id}": "GET single DSR not yet registered, TODO",
	}

	serverURL := "/api/v1" // must match openapi.yaml servers[0].url

	failures := 0
	for specPath, pathItem := range doc.Paths.Map() {
		fullPath := serverURL + specPath
		echoPath := openAPIPathToEcho(fullPath)

		for method := range pathItem.Operations() {
			httpMethod := strings.ToUpper(method)
			key := httpMethod + " " + echoPath
			specKey := httpMethod + " " + fullPath

			if reason, ok := reverseAllowlist[specKey]; ok {
				t.Logf("SKIP %s — %s", specKey, reason)
				continue
			}

			if !echoRoutes[key] {
				t.Errorf("spec operation %s %s has no Echo route (tried %s)",
					httpMethod, specPath, echoPath)
				failures++
			}
		}
	}
	if failures == 0 {
		t.Logf("reverse contract: all %d spec operations have Echo routes", len(doc.Paths.Map()))
	}
}

// openAPIPathToEcho converts an OpenAPI path (e.g. /api/v1/foo/{id}/bar) to
// the Echo route format (e.g. /api/v1/foo/:id/bar).
func openAPIPathToEcho(path string) string {
	var result strings.Builder
	for i := 0; i < len(path); i++ {
		if path[i] == '{' {
			j := strings.Index(path[i:], "}")
			if j < 0 {
				result.WriteByte(path[i])
				continue
			}
			result.WriteString(":")
			result.WriteString(path[i+1 : i+j])
			i += j
		} else {
			result.WriteByte(path[i])
		}
	}
	return result.String()
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
