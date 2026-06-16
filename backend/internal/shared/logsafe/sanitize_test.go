// Copyright (c) 2026 NorvikOps. All rights reserved.
// SPDX-License-Identifier: Elastic-2.0

package logsafe

import (
	"strings"
	"testing"
)

func TestSanitizeField(t *testing.T) {
	cases := []struct {
		name, in, want string
		maxLen         int
	}{
		{"plain", "hello", "hello", 100},
		{"strips ANSI CSI", "a\x1b[31mred\x1b[0mb", "aredb", 100},
		{"strips NUL + control", "x\x00\x07y", "xy", 100},
		{"keeps tab/newline", "a\tb\nc", "a\tb\nc", 100},
		{"truncates", "abcdef", "abc", 3},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := SanitizeField(tc.in, tc.maxLen)
			if got != tc.want {
				t.Errorf("SanitizeField(%q,%d)=%q want %q", tc.in, tc.maxLen, got, tc.want)
			}
		})
	}
}

func TestSanitizeField_TruncateBeforeScan(t *testing.T) {
	// Truncation happens first; ensure no panic on a cut mid-escape.
	in := strings.Repeat("\x1b[", 50)
	_ = SanitizeField(in, 3)
}
