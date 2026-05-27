// Copyright (c) 2026 NorvikOps. All rights reserved.
// SPDX-License-Identifier: Elastic-2.0

package main

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

// TestBuildXFFTrustOptions_RangesParsed verifies that every valid CIDR in
// the input produces an additional TrustOption beyond the two unconditional
// defaults (TrustLoopback + TrustLinkLocal).
func TestBuildXFFTrustOptions_RangesParsed(t *testing.T) {
	cases := []struct {
		name     string
		input    string
		wantMin  int // minimum number of options expected
		wantMax  int // maximum, since we don't pin the loopback impl details
		expectOK bool
	}{
		{
			name:     "single docker bridge range",
			input:    "172.0.0.0/8",
			wantMin:  3, // loopback + link-local-off + 1 CIDR
			wantMax:  3,
			expectOK: true,
		},
		{
			name:     "two ranges",
			input:    "172.16.0.0/12, 10.0.0.0/8",
			wantMin:  4,
			wantMax:  4,
			expectOK: true,
		},
		{
			name:     "empty input — only defaults",
			input:    "",
			wantMin:  2,
			wantMax:  2,
			expectOK: true,
		},
		{
			name:     "ipv6 range",
			input:    "fd00::/8",
			wantMin:  3,
			wantMax:  3,
			expectOK: true,
		},
		{
			name:     "invalid entries are skipped, valid ones survive",
			input:    "172.16.0.0/12, not-a-cidr, 10.0.0.0/8, 999.999.999.999/8",
			wantMin:  4,
			wantMax:  4,
			expectOK: true,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			opts := buildXFFTrustOptions(tc.input, nil)
			assert.GreaterOrEqual(t, len(opts), tc.wantMin, "expected at least %d options, got %d", tc.wantMin, len(opts))
			assert.LessOrEqual(t, len(opts), tc.wantMax, "expected at most %d options, got %d", tc.wantMax, len(opts))
		})
	}
}

// TestBuildXFFTrustOptions_TolerantOfWhitespace ensures realistic operator
// input (whitespace around commas, trailing newlines from env files) does
// not silently drop ranges.
func TestBuildXFFTrustOptions_TolerantOfWhitespace(t *testing.T) {
	opts := buildXFFTrustOptions("  172.0.0.0/8 ,\t10.0.0.0/8\n", nil)
	// 2 defaults + 2 CIDRs = 4
	assert.Equal(t, 4, len(opts))
}
