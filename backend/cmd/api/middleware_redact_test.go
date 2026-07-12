// Copyright (c) 2026 NorvikOps. All rights reserved.
// SPDX-License-Identifier: Elastic-2.0
// See LICENSE file in the project root for terms.

package main

import "testing"

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
