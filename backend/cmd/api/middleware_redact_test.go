// Copyright (c) 2026 NorvikOps. All rights reserved.
// SPDX-License-Identifier: Elastic-2.0
// See LICENSE file in the project root for terms.

package main

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/labstack/echo/v4"
	"github.com/rs/zerolog"
)

// TestRedactQuery guards a leak that actually happened.
//
// The billing approval token is stored in the database only as a SHA-256 hash,
// so that a leaked backup cannot be used to approve invoices. Then the access log
// printed the plaintext token from the query string on every click — and the logs
// are shipped to Loki on a different host. The hashing was pointless.
//
// If this test ever fails, someone has re-opened that hole.
func TestRedactQuery(t *testing.T) {
	tests := []struct {
		name string
		in   string
		want string
	}{
		{
			name: "billing approval token — the leak this exists for",
			in:   "/api/v1/billing/quote-request/6c46491b/approve?token=20b156f376b7e61c283481",
			want: "/api/v1/billing/quote-request/6c46491b/approve?token=***",
		},
		{
			name: "no query string is left alone",
			in:   "/api/v1/health",
			want: "/api/v1/health",
		},
		{
			name: "harmless params stay readable — logs must remain useful",
			in:   "/api/v1/findings?page=2&limit=25",
			want: "/api/v1/findings?page=2&limit=25",
		},
		{
			name: "sensitive param mixed with harmless ones",
			in:   "/api/v1/x?page=2&token=deadbeef&limit=25",
			want: "/api/v1/x?page=2&token=***&limit=25",
		},
		{
			name: "case-insensitive key match",
			in:   "/api/v1/x?TOKEN=deadbeef",
			want: "/api/v1/x?TOKEN=***",
		},
		{
			name: "OAuth code and state",
			in:   "/auth/callback?code=abc123&state=xyz789",
			want: "/auth/callback?code=***&state=***",
		},
		{
			name: "valueless param must not panic or be mangled",
			in:   "/api/v1/x?flag&token=abc",
			want: "/api/v1/x?flag&token=***",
		},
		{
			name: "empty token value is still redacted",
			in:   "/api/v1/x?token=",
			want: "/api/v1/x?token=***",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := redactQuery(tt.in); got != tt.want {
				t.Errorf("redactQuery(%q)\n  got:  %q\n  want: %q", tt.in, got, tt.want)
			}
		})
	}
}

// tokenRoutes is the measured set of routes that carry a raw, secret token in a
// PATH segment. Counted on 2026-08-07 against the route registrations in
// internal/**/routes.go (and portal.go / auditor/handler.go), non-test files:
// 16 routes. The list is here so a failure names the route that leaks, instead
// of reporting "one of many".
//
// This test does NOT drive the redaction — redactPathParams keys on the
// parameter NAME from the route template, so it covers routes added after this
// list was written. The list is a witness, not the mechanism.
var tokenRoutes = []string{
	// vaktcomply — auditor bundle, supplier portal, policy acceptance
	"/api/v1/vaktcomply/auditor/:token",
	"/api/v1/vaktcomply/auditor/:token/export",
	"/api/v1/vaktcomply/supplier/:token",
	"/api/v1/vaktcomply/supplier/:token/save",
	"/api/v1/vaktcomply/supplier/:token/submit",
	"/api/v1/vaktcomply/supplier/:token/upload",
	"/api/v1/vaktcomply/policy-accept/:token",
	// vaktvault — one-time secret share link
	"/api/v1/vaktvault/share/:token",
	// vaktaware — phishing click / open / form-submit tracking
	"/api/v1/vaktaware/t/:token",
	"/api/v1/vaktaware/t/:token/submit",
	"/api/v1/vaktaware/track/:token",
	// vaktprivacy — DSR self-service status
	"/api/v1/vaktprivacy/dsr-portal/status/:token",
	// auditor invite acceptance
	"/api/v1/auditor/accept/:token",
	// billing portal
	"/api/v1/billing/portal/:token",
	"/api/v1/billing/portal/:token/seat",
}

// marker is a value that must never survive into a log line.
const marker = "4f1c9ae2b70d5836marker"

// TestRedactPathTokenOnEveryTokenRoute is the regression guard for R1-24-01.
//
// redactQuery only ever touched the query string: with no "?" in the URI it
// returned the URI untouched, and it never looked at the path even when there
// was one. So every route below logged its raw token. Those tokens are stored
// as SHA-256 hashes so a leaked backup is worthless, and the access log shipped
// the plaintext to Loki on another host — the hashing was undone by the log.
//
// If this fails, someone re-opened that hole for the named route.
func TestRedactPathTokenOnEveryTokenRoute(t *testing.T) {
	for _, route := range tokenRoutes {
		t.Run(route, func(t *testing.T) {
			uri := strings.Replace(route, ":token", marker, 1)
			got := redactURI(route, uri)

			if strings.Contains(got, marker) {
				t.Errorf("token leaked into the access log for %s\n  uri: %q\n  logged: %q", route, uri, got)
			}
			if !strings.Contains(got, "***") {
				t.Errorf("expected the token segment to be masked for %s, got %q", route, got)
			}
		})
	}
}

// TestRedactPathKeepsIdentifiersReadable is the baseline: normal routes must
// come through unchanged. Logs that mask everything stop being read, and the
// deliberate non-secrets here would cost real debugging ability —
// /…/secrets/:key is the secret's NAME (DATABASE_URL), not its value, and
// /physical-templates/:code is an ISO 27001 A.7.x template code.
func TestRedactPathKeepsIdentifiersReadable(t *testing.T) {
	tests := []struct {
		name  string
		route string
		uri   string
	}{
		{"plain id", "/api/v1/vaktcomply/controls/:id", "/api/v1/vaktcomply/controls/8f2b1c44"},
		{"secret NAME is an identifier, not a secret", "/api/v1/vaktvault/projects/:project_id/envs/:env_id/secrets/:key", "/api/v1/vaktvault/projects/p1/envs/e1/secrets/DATABASE_URL"},
		{"template code", "/api/v1/vaktcomply/physical-templates/:code/apply", "/api/v1/vaktcomply/physical-templates/A.7.4/apply"},
		{"slug", "/api/v1/vaktprivacy/dsr-portal/:slug", "/api/v1/vaktprivacy/dsr-portal/acme-gmbh"},
		{"static route without params", "/api/v1/health", "/api/v1/health"},
		{"pagination stays readable", "/api/v1/findings", "/api/v1/findings?page=2&limit=25"},
		{"unmatched path keeps the 404 signal", "/api/v1/*", "/api/v1/does/not/exist"},
		{"no route template at all", "", "/api/v1/whatever/8f2b1c44"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := redactURI(tt.route, tt.uri); got != tt.uri {
				t.Errorf("redactURI(%q, %q)\n  got:  %q\n  want it unchanged", tt.route, tt.uri, got)
			}
		})
	}
}

// TestRedactPathAndQueryTogether covers the seam: a token in the path AND
// sensitive plus harmless query parameters on the same request.
func TestRedactPathAndQueryTogether(t *testing.T) {
	route := "/api/v1/vaktcomply/auditor/:token/export"
	uri := "/api/v1/vaktcomply/auditor/" + marker + "/export?format=pdf&token=" + marker
	want := "/api/v1/vaktcomply/auditor/***/export?format=pdf&token=***"

	if got := redactURI(route, uri); got != want {
		t.Errorf("redactURI\n  got:  %q\n  want: %q", got, want)
	}
}

// TestRequestLoggerRedactsPathToken drives the REAL middleware through a real
// Echo router, not a rebuilt copy of the chain.
//
// It is what proves the approach is even possible: the redaction reads c.Path()
// (the route template), which is only populated because Echo's ServeHTTP runs
// the router BEFORE the e.Use chain. A pure unit test on redactURI would pass
// even if that assumption were wrong.
func TestRequestLoggerRedactsPathToken(t *testing.T) {
	var buf bytes.Buffer
	log := zerolog.New(&buf)

	e := echo.New()
	e.Use(requestLogger(log))
	e.GET("/api/v1/vaktcomply/auditor/:token/export", func(c echo.Context) error {
		return c.String(http.StatusOK, "ok")
	})

	uri := "/api/v1/vaktcomply/auditor/" + marker + "/export"
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, uri, nil))

	logged := buf.String()
	if logged == "" {
		t.Fatal("request logger produced no output — the test is vacuous")
	}
	if strings.Contains(logged, marker) {
		t.Errorf("auditor token reached the access log verbatim:\n%s", logged)
	}
	if !strings.Contains(logged, "/api/v1/vaktcomply/auditor/***/export") {
		t.Errorf("expected the masked path in the log line, got:\n%s", logged)
	}
}
