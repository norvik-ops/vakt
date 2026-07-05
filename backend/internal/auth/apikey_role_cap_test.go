// Copyright (c) 2026 NorvikOps. All rights reserved.
// SPDX-License-Identifier: Elastic-2.0

package auth

import "testing"

// S120-4: an API key must never grant more than its creator's current role.
func TestCapRoleAtCreator(t *testing.T) {
	tests := []struct {
		name        string
		scopeRole   string
		creatorRole string
		want        string
	}{
		{"personal key of admin keeps analyst default", "SecurityAnalyst", "Admin", "SecurityAnalyst"},
		{"personal key of analyst stays analyst", "SecurityAnalyst", "SecurityAnalyst", "SecurityAnalyst"},
		{"personal key of viewer capped to viewer", "SecurityAnalyst", "Viewer", "Viewer"},
		{"personal key of auditor capped to viewer", "SecurityAnalyst", "AuditorReadOnly", "Viewer"},
		{"personal key of internal auditor capped to viewer", "SecurityAnalyst", "InternalAuditor", "Viewer"},
		{"admin scope capped at analyst creator", "Admin", "SecurityAnalyst", "SecurityAnalyst"},
		{"admin scope with admin creator", "Admin", "Admin", "Admin"},
		{"read-only key stays viewer regardless", "Viewer", "Admin", "Viewer"},
		{"creator left org caps to viewer", "SecurityAnalyst", "", "Viewer"},
		{"unknown creator role caps to viewer", "Admin", "SomethingElse", "Viewer"},
	}
	for _, tc := range tests {
		if got := capRoleAtCreator(tc.scopeRole, tc.creatorRole); got != tc.want {
			t.Errorf("%s: capRoleAtCreator(%q, %q) = %q, want %q", tc.name, tc.scopeRole, tc.creatorRole, got, tc.want)
		}
	}
}
