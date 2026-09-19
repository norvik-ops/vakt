// Copyright (c) 2026 NorvikOps. All rights reserved.
// SPDX-License-Identifier: Elastic-2.0

package ai

import (
	"context"
	"strings"

	"github.com/rs/zerolog/log"

	"github.com/matharnica/vakt/internal/db"
)

// deriveToolScopes maps a user's module-level RBAC (user_module_permissions,
// can_read/can_write per module) to the concrete AI-tool scopes they may use.
// Each tool scope has the shape "<module>.<resource>.<action>" (e.g.
// "vaktcomply.controls.write"); the module prefix equals the module names in
// user_module_permissions exactly (vaktscan/vaktcomply/…). A scope is granted
// only if the user holds the matching bit for that module — read scopes need
// can_read, write scopes need can_write.
//
// Fail-closed by construction: an unrecognised scope shape, or a module the user
// has no bit for, is simply not added. This derivation is deliberately per-tool
// and read/write-exact rather than granting a "<module>.*" wildcard: hasScope()
// treats "vaktcomply.*" as matching BOTH read and write tools, which would hand
// a write tool to a read-only user. R1-W8A-N1(b).
func deriveToolScopes(modPerms []db.UserModulePermissions, toolScopes []string) []string {
	canRead := make(map[string]bool, len(modPerms))
	canWrite := make(map[string]bool, len(modPerms))
	for _, mp := range modPerms {
		if mp.CanRead {
			canRead[mp.Module] = true
		}
		if mp.CanWrite {
			canWrite[mp.Module] = true
		}
	}
	var out []string
	for _, scope := range toolScopes {
		parts := strings.Split(scope, ".")
		if len(parts) < 3 {
			continue
		}
		module := parts[0]
		action := parts[len(parts)-1]
		switch {
		case action == "read" && canRead[module]:
			out = append(out, scope)
		case action == "write" && canWrite[module]:
			out = append(out, scope)
		}
	}
	return out
}

// derivePermissions loads the caller's module RBAC and derives the concrete
// AI-tool scopes they may use. Fail-closed: on any DB error it returns nil, so
// the agent falls back to unscoped tools only. R1-W8A-N1(b): nothing set the
// "permissions" context key before, so a legitimately privileged user could
// never actually use a scoped tool.
func (h *AgentHandler) derivePermissions(ctx context.Context, orgID, userID string) []string {
	q := db.New(h.db)
	modPerms, err := q.GetUserModulePermissions(ctx, db.GetUserModulePermissionsParams{OrgID: orgID, UserID: userID})
	if err != nil {
		log.Warn().Err(err).Str("org_id", orgID).Str("user_id", userID).
			Msg("ai agent: module permission lookup failed — denying all scoped tools")
		return nil
	}
	var scopes []string
	for _, t := range DefaultAgentTools(h.db) {
		if s := t.RequireScope(); s != "" {
			scopes = append(scopes, s)
		}
	}
	return deriveToolScopes(modPerms, scopes)
}
