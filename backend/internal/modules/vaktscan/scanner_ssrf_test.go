// Copyright (c) 2026 NorvikOps. All rights reserved.
// SPDX-License-Identifier: Elastic-2.0

package vaktscan

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

// TestIsPrivateOrLoopback_IPLiterals covers bare IP addresses — these were
// already handled before SEC-H07.
func TestIsPrivateOrLoopback_IPLiterals(t *testing.T) {
	cases := []struct {
		target string
		want   bool
	}{
		{"127.0.0.1", true},
		{"127.0.0.1:8080", true},
		{"::1", true},
		{"[::1]:443", true},
		{"localhost", true},
		{"LOCALHOST", true},
		{"10.0.0.1", true},
		{"172.16.0.1", true},
		{"192.168.1.1", true},
		{"169.254.1.1", true}, // link-local
		{"8.8.8.8", false},
		{"1.1.1.1", false},
		{"203.0.113.5", false}, // TEST-NET (but not private range)
	}
	for _, tc := range cases {
		got := isPrivateOrLoopback(tc.target)
		assert.Equal(t, tc.want, got, "target=%q", tc.target)
	}
}

// TestIsPrivateOrLoopback_Hostname_PublicDomain verifies that a real public
// hostname is NOT blocked (regression: old code returned false for all hostnames).
// This test requires network access; skip if DNS is unavailable.
func TestIsPrivateOrLoopback_Hostname_PublicDomain(t *testing.T) {
	// dns.google reliably resolves to 8.8.8.8 / 8.8.4.4 — public, not private.
	got := isPrivateOrLoopback("dns.google")
	assert.False(t, got, "dns.google should not be classified as private")
}

// TestIsPrivateOrLoopback_UnresolvableFails verifies that an unresolvable
// hostname is treated as private (fail-safe / SEC-H07).
func TestIsPrivateOrLoopback_UnresolvableFails(t *testing.T) {
	got := isPrivateOrLoopback("this-hostname-does-not-exist.invalid")
	assert.True(t, got, "unresolvable hostnames must be blocked (fail-safe)")
}
