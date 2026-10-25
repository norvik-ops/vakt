// Copyright (c) 2026 NorvikOps. All rights reserved.
// SPDX-License-Identifier: Elastic-2.0

package usermgmt

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

// R1-KW-2: the invitation list must report the platform role name — the same
// vocabulary as platform_role on GET /admin/users. Rows written before
// migration 255 still hold the legacy lowercase names; they are mapped on read
// so persisted data stays readable without a data migration.
func TestInvitationRoleForResponse_reportsPlatformRoleName(t *testing.T) {
	cases := map[string]string{
		// canonical values stored since migration 255 pass through unchanged
		"Admin":           "Admin",
		"SecurityAnalyst": "SecurityAnalyst",
		"Viewer":          "Viewer",
		"AuditorReadOnly": "AuditorReadOnly",
		"InternalAuditor": "InternalAuditor",
		// legacy rows
		"admin":  "Admin",
		"editor": "SecurityAnalyst",
		"viewer": "Viewer",
	}
	for stored, want := range cases {
		assert.Equal(t, want, invitationRoleForResponse(stored), "stored %q", stored)
	}
}

// Every value the invitation route accepts must come back out in the platform
// vocabulary, so the frontend type (PlatformRole) and the OpenAPI enum cover it.
func TestInvitationRoleForResponse_coversEveryInvitableRole(t *testing.T) {
	platform := map[string]bool{
		"Admin": true, "SecurityAnalyst": true, "Viewer": true,
		"AuditorReadOnly": true, "InternalAuditor": true,
	}
	for _, role := range oneofValues(t, InviteInput{}, "Role") {
		got := invitationRoleForResponse(role)
		assert.True(t, platform[got], "invitable role %q reported as %q — not a platform role name", role, got)
	}
}
