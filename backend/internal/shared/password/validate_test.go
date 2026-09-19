// Copyright (c) 2026 NorvikOps. All rights reserved.
// SPDX-License-Identifier: Elastic-2.0

package password

import (
	"errors"
	"testing"
)

// TestValidateStrength pins the canonical policy that auth, setup, admin and
// usermgmt now all delegate to (R1-W7C-N2). It is non-tautological: each case
// names the single rule it violates, so a policy that silently drops one of the
// complexity requirements turns a "weak" case green and fails the test.
func TestValidateStrength(t *testing.T) {
	cases := []struct {
		name    string
		pw      string
		wantErr bool
	}{
		{"too short", "Ab1!xyz", true},                            // 7 chars
		{"exactly 10 but no upper", "abc1!defgh", true},           // missing uppercase
		{"no digit", "Abcdef!ghij", true},                         // missing digit
		{"no special", "Abcdef1ghij", true},                       // missing special
		{"empty", "", true},                                       // all missing
		{"strong", "Str0ng!Passw", false},                         // 12 chars, all classes
		{"strong with backslash special", "Longpass1\\XY", false}, // backslash is a valid special
		{"strong with backtick special", "Longpass1`XY", false},   // backtick is a valid special
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			err := ValidateStrength(tc.pw)
			if tc.wantErr {
				if err == nil {
					t.Fatalf("ValidateStrength(%q) = nil, want ErrWeakPassword", tc.pw)
				}
				if !errors.Is(err, ErrWeakPassword) {
					t.Fatalf("ValidateStrength(%q) = %v, want ErrWeakPassword", tc.pw, err)
				}
				return
			}
			if err != nil {
				t.Fatalf("ValidateStrength(%q) = %v, want nil", tc.pw, err)
			}
		})
	}
}
