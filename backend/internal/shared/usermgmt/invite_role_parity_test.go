// Copyright (c) 2026 NorvikOps. All rights reserved.
// SPDX-License-Identifier: Elastic-2.0

package usermgmt

import (
	"reflect"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// R1-14cA-11 — neighbouring routes spoke incompatible role vocabularies.
//
// The invitation took admin|editor|viewer, the role change took the platform
// names. A client that knew one set failed on the other, and the error told it
// nothing about why. The two lists now match, and this pins them together — a
// comment saying "keep in sync" is what let them drift in the first place.

func oneofValues(t *testing.T, structType any, field string) []string {
	t.Helper()
	f, ok := reflect.TypeOf(structType).FieldByName(field)
	require.True(t, ok, "%T has no field %s — the parity check is now vacuous", structType, field)
	for _, part := range strings.Split(f.Tag.Get("validate"), ",") {
		if strings.HasPrefix(part, "oneof=") {
			return strings.Fields(strings.TrimPrefix(part, "oneof="))
		}
	}
	t.Fatalf("%T.%s lost its oneof rule — the route now takes any string", structType, field)
	return nil
}

func TestInviteAndRoleChange_acceptTheSameVocabulary(t *testing.T) {
	invite := oneofValues(t, InviteInput{}, "Role")
	update := oneofValues(t, UpdateRoleInput{}, "Role")

	assert.ElementsMatch(t, update, invite,
		"an invitation and a role change disagree about which role names exist — "+
			"whoever knows one route's vocabulary fails on the other (R1-14cA-11)")
}

// Every accepted value must have a mapping. A value the validator lets through
// but assignableRoles does not know falls back to Viewer at accept time, which
// looks like a successful invitation that quietly granted less than it said.
func TestEveryAcceptedRole_hasAnAssignment(t *testing.T) {
	for _, structAndField := range []struct {
		s     any
		field string
	}{
		{InviteInput{}, "Role"},
		{UpdateRoleInput{}, "Role"},
	} {
		for _, role := range oneofValues(t, structAndField.s, structAndField.field) {
			assign, ok := assignableRoles[role]
			assert.True(t, ok, "%T accepts role %q but assignableRoles does not map it", structAndField.s, role)
			if ok {
				assert.NotEmpty(t, assign.platform, "role %q maps to an empty platform role", role)
				assert.Contains(t, []string{"admin", "editor", "viewer"}, assign.simple,
					"role %q maps users.role to %q, which violates users_role_check", role, assign.simple)
			}
		}
	}
}

// The invitation-accept path writes users.role. Before R1-14cA-11 it wrote the
// invitation's raw string, which the widened vocabulary would push straight into
// CHECK (role IN ('admin','editor','viewer')) — a 500 after the invitee had
// already clicked the link and chosen a password.
func TestAssignmentFor_neverYieldsAValueTheColumnRejects(t *testing.T) {
	cases := []string{"Admin", "SecurityAnalyst", "Viewer", "AuditorReadOnly", "InternalAuditor",
		"admin", "editor", "viewer", "something-a-much-older-invitation-carried", ""}
	for _, role := range cases {
		got := assignmentFor(role)
		assert.Contains(t, []string{"admin", "editor", "viewer"}, got.simple,
			"assignmentFor(%q).simple = %q — users_role_check rejects that", role, got.simple)
		assert.NotEmpty(t, got.platform, "assignmentFor(%q) has no platform role", role)
	}
}
