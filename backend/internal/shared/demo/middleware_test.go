// Copyright (c) 2026 NorvikOps. All rights reserved.
// SPDX-License-Identifier: Elastic-2.0

package demo

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

// TestBlockedRoutes_noDeadPrefixes verifies that the demo block list does not
// contain prefixes that are known to be wrong (stale route references).
// Extend this list whenever a route is renamed.
func TestBlockedRoutes_noDeadPrefixes(t *testing.T) {
	deadPrefixes := []string{
		"/api/v1/auth/totp",       // real routes are under /auth/2fa/
		"/api/v1/alerting/webhooks", // real routes are under /alerting/channels
		"/api/v1/vaktvault/secrets", // real routes are nested under /vaktvault/projects/
	}

	for _, br := range BlockedRoutes {
		for _, dead := range deadPrefixes {
			assert.False(
				t,
				strings.HasPrefix(br.Prefix, dead),
				"BlockedRoute %s %s uses a dead prefix %q — update to the real route path",
				br.Method, br.Prefix, dead,
			)
		}
	}
}

// TestBlockedRoutes_requiredPrefixesPresent verifies that the block list
// contains the correct prefixes for security-critical routes.
func TestBlockedRoutes_requiredPrefixesPresent(t *testing.T) {
	type want struct {
		method string
		prefix string
	}
	required := []want{
		{"POST", "/api/v1/auth/2fa/"},
		{"POST", "/api/v1/alerting/channels"},
		{"DELETE", "/api/v1/alerting/channels"},
		{"POST", "/api/v1/vaktvault/projects"},
		{"DELETE", "/api/v1/vaktvault/projects"},
		{"PUT", "/api/v1/vaktvault/projects"},
	}

	for _, req := range required {
		found := false
		for _, br := range BlockedRoutes {
			if br.Method == req.method && br.Prefix == req.prefix {
				found = true
				break
			}
		}
		assert.True(t, found,
			"BlockedRoutes must contain {%s %s}", req.method, req.prefix)
	}
}

// TestBlockedRoutes_allPrefixesHaveAPIV1 ensures every entry has the correct
// base path — a missing /api/v1 prefix would never match real routes.
func TestBlockedRoutes_allPrefixesHaveAPIV1(t *testing.T) {
	for _, br := range BlockedRoutes {
		assert.True(t,
			strings.HasPrefix(br.Prefix, "/api/v1/"),
			"BlockedRoute {%s %s} must start with /api/v1/", br.Method, br.Prefix,
		)
	}
}
