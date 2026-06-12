// Copyright (c) 2026 NorvikOps. All rights reserved.
// SPDX-License-Identifier: Elastic-2.0

package httputil

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// net.LookupHost returns literal IPs as-is, so we can drive the full
// SSRF-guard code path with literal-IP URLs without any DNS dependency.

func TestValidateOutboundURL_ssrfBlocked(t *testing.T) {
	cases := []struct {
		name string
		url  string
	}{
		{"loopback", "http://127.0.0.1/"},
		{"IMDS AWS/Azure/GCP", "http://169.254.169.254/latest/meta-data"},
		{"RFC1918 class-A", "http://10.0.0.1:9090/metrics"},
		{"RFC1918 class-B", "http://172.16.0.1/"},
		{"RFC1918 class-C", "http://192.168.1.100:9090"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			err := ValidateOutboundURL(tc.url, false)
			require.Error(t, err, "expected SSRF block for %s", tc.url)
			assert.Contains(t, err.Error(), "private or link-local")
		})
	}
}

func TestValidateOutboundURL_allowPrivateBypass(t *testing.T) {
	// With allowPrivate=true the call succeeds even for RFC1918 targets.
	err := ValidateOutboundURL("http://192.168.1.100:9090", true)
	assert.NoError(t, err, "allow_private_target should permit RFC1918 URLs")
}

func TestValidateOutboundURL_imdsAllowPrivate(t *testing.T) {
	// Even the IMDS address is reachable when the operator explicitly opts in.
	err := ValidateOutboundURL("http://169.254.169.254/", true)
	assert.NoError(t, err)
}

func TestValidateOutboundURL_invalidScheme(t *testing.T) {
	err := ValidateOutboundURL("ftp://example.com/data", false)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "http or https")
}

func TestValidateOutboundURL_noScheme(t *testing.T) {
	err := ValidateOutboundURL("example.com", false)
	require.Error(t, err)
}

func TestValidateOutboundURL_emptyHostname(t *testing.T) {
	err := ValidateOutboundURL("http:///path", false)
	require.Error(t, err)
}

func TestValidateOutboundURL_privateRangesContainIMDS(t *testing.T) {
	// Verify the IMDS CIDR is registered — guards against accidental removal.
	found := false
	for _, n := range privateRanges {
		if n.String() == "169.254.0.0/16" {
			found = true
			break
		}
	}
	assert.True(t, found, "IMDS CIDR 169.254.0.0/16 must be in privateRanges")
}
