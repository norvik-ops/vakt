// Copyright (c) 2026 NorvikOps. All rights reserved.
// SPDX-License-Identifier: Elastic-2.0

package reporting

import (
	"context"
	"strings"
	"testing"
)

// TestValidateCCMURL covers the SSRF guard: only public http(s) URLs are
// allowed; loopback, private, link-local and the cloud metadata service must
// be rejected. This is security-relevant logic (the CCM runner makes outbound
// requests to operator-supplied URLs), so the invariant is unit-tested.
func TestValidateCCMURL(t *testing.T) {
	tests := []struct {
		name    string
		url     string
		wantErr bool
	}{
		{"non-http scheme", "ftp://example.com/file", true},
		{"file scheme", "file:///etc/passwd", true},
		{"garbage", "://not a url", true},
		{"loopback literal", "http://127.0.0.1/health", true},
		{"loopback name", "http://localhost/health", true},
		{"private 10.x", "http://10.0.0.5/", true},
		{"private 192.168", "http://192.168.1.1/", true},
		{"link-local", "http://169.254.0.1/", true},
		{"cloud metadata", "http://169.254.169.254/latest/meta-data/", true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateCCMURL(tt.url)
			if tt.wantErr && err == nil {
				t.Fatalf("validateCCMURL(%q) = nil, want error", tt.url)
			}
			if !tt.wantErr && err != nil {
				t.Fatalf("validateCCMURL(%q) = %v, want nil", tt.url, err)
			}
		})
	}
}

// TestRunHTTPEndpointCheck_MissingURL verifies the runner fails closed when the
// required `url` config key is absent rather than panicking or passing.
func TestRunHTTPEndpointCheck_MissingURL(t *testing.T) {
	status, output, err := runHTTPEndpointCheck(context.Background(), CCMCheck{
		CheckType: "http_endpoint",
		Config:    map[string]string{},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if status != "fail" {
		t.Fatalf("status = %q, want fail", status)
	}
	if !strings.Contains(output, "url") {
		t.Fatalf("output = %q, want mention of missing url", output)
	}
}

// TestRunHTTPEndpointCheck_BlockedURL verifies the runner rejects an internal
// target via the SSRF guard and never issues the request.
func TestRunHTTPEndpointCheck_BlockedURL(t *testing.T) {
	status, output, err := runHTTPEndpointCheck(context.Background(), CCMCheck{
		CheckType: "http_endpoint",
		Config:    map[string]string{"url": "http://169.254.169.254/"},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if status != "fail" {
		t.Fatalf("status = %q, want fail", status)
	}
	if !strings.Contains(output, "validation failed") {
		t.Fatalf("output = %q, want validation failure", output)
	}
}

// TestRunCheck_UnknownType ensures an unrecognised check type yields "unknown"
// rather than an error, keeping batch runs resilient.
func TestRunCheck_UnknownType(t *testing.T) {
	status, output, err := RunCheck(context.Background(), nil, CCMCheck{CheckType: "nope"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if status != "unknown" {
		t.Fatalf("status = %q, want unknown", status)
	}
	if !strings.Contains(output, "unknown check type") {
		t.Fatalf("output = %q, want unknown-check-type message", output)
	}
}

// TestRunCheck_CustomScriptUnsupported documents that custom_script is a no-op
// stub in this build (returns unknown, no error).
func TestRunCheck_CustomScriptUnsupported(t *testing.T) {
	status, _, err := RunCheck(context.Background(), nil, CCMCheck{CheckType: "custom_script"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if status != "unknown" {
		t.Fatalf("status = %q, want unknown", status)
	}
}

// TestUnmarshalCCMConfig verifies the config mapper never returns nil and
// tolerates empty/garbage JSON (defensive: config originates from the DB).
func TestUnmarshalCCMConfig(t *testing.T) {
	if got := unmarshalCCMConfig(nil); got == nil {
		t.Fatal("unmarshalCCMConfig(nil) = nil map, want empty non-nil map")
	}
	if got := unmarshalCCMConfig([]byte(`{"url":"https://x"}`)); got["url"] != "https://x" {
		t.Fatalf("unmarshalCCMConfig parsed wrong value: %v", got)
	}
	if got := unmarshalCCMConfig([]byte("not json")); got == nil {
		t.Fatal("unmarshalCCMConfig(garbage) = nil map, want empty non-nil map")
	}
}
