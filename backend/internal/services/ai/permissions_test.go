// Copyright (c) 2026 NorvikOps. All rights reserved.
// SPDX-License-Identifier: Elastic-2.0

package ai

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/matharnica/vakt/internal/db"
)

// TestDeriveToolScopes covers R1-W8A-N1(b): the module RBAC (can_read/can_write)
// must map to the concrete read/write tool scopes exactly — and must NOT hand a
// write scope to a read-only user (the reason a "<module>.*" wildcard is wrong).
func TestDeriveToolScopes(t *testing.T) {
	all := []string{
		"vaktscan.findings.read",
		"vaktcomply.evidence.read",
		"vaktcomply.controls.read",
		"vaktcomply.controls.write",
	}

	// Read-only on vaktcomply: gets both vaktcomply.read scopes, NOT the write
	// scope, NOT any vaktscan scope.
	got := deriveToolScopes([]db.UserModulePermissions{
		{Module: "vaktcomply", CanRead: true, CanWrite: false},
	}, all)
	assert.ElementsMatch(t, []string{"vaktcomply.evidence.read", "vaktcomply.controls.read"}, got,
		"read bit grants read scopes only, scoped to the module")

	// Write-only on vaktcomply: gets the write scope but NOT the read scopes —
	// this is the load-bearing case a wildcard would get wrong.
	got = deriveToolScopes([]db.UserModulePermissions{
		{Module: "vaktcomply", CanRead: false, CanWrite: true},
	}, all)
	assert.ElementsMatch(t, []string{"vaktcomply.controls.write"}, got,
		"write bit must not grant read scopes")

	// Full access across two modules.
	got = deriveToolScopes([]db.UserModulePermissions{
		{Module: "vaktcomply", CanRead: true, CanWrite: true},
		{Module: "vaktscan", CanRead: true},
	}, all)
	assert.ElementsMatch(t, all, got)

	// No permissions → no scoped tools (fail-closed).
	assert.Empty(t, deriveToolScopes(nil, all), "no RBAC rows means no scoped tools")

	// A module the user has no bit for is never granted.
	got = deriveToolScopes([]db.UserModulePermissions{
		{Module: "vaktvault", CanRead: true, CanWrite: true},
	}, all)
	assert.Empty(t, got, "bits on an unrelated module grant nothing")
}
