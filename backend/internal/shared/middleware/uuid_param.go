// Copyright (c) 2026 NorvikOps. All rights reserved.
// SPDX-License-Identifier: Elastic-2.0

package middleware

import (
	"net/http"

	"github.com/google/uuid"
	"github.com/labstack/echo/v4"
)

// nonUUIDParamNames is a curated DENYLIST of path-param names that are
// deliberately NOT resource UUIDs, across the six business modules and shared
// packages. Everything else is validated by default (see ValidateUUIDParams) —
// S3/D13-A inverted this from an allowlist (uuidParamNames) after finding that
// an allowlist silently exempts every new nested param (:itemId, :depId,
// :riskId, :taskId, :controlId, ...) until someone remembers to add it, which
// is the exact same subset trap as v0.42.25/26/27's route-wiring gaps. A
// denylist fails CLOSED instead: an unrecognised new param gets validated,
// and a legitimately non-UUID param has to be added here explicitly, with a
// reason, or the guard breaks it.
var nonUUIDParamNames = map[string]bool{
	// Echo's catch-all. Group.Use() auto-registers RouteNotFound("", …) and
	// RouteNotFound("/*", …) for every group that has middleware — WITH the full
	// group chain, so `/api/v1/*` runs through this guard. Its param is "*" and
	// its value is the unmatched rest-path.
	//
	// Without this entry every 404 under /api/v1 becomes "400 invalid id: must be
	// a UUID" for an authenticated request to any unknown path. That is not a
	// cosmetic mislabel: the live 404 is the diagnostic signal this project uses
	// to find its recurring "handler without route" class (v0.42.25/26/27), and
	// frontend hooks that turn 404 into [] would start seeing 400 instead. G4
	// cannot catch it — the coverage test skips paths without ":" and only
	// asserts != 500.
	"*":             true,
	"name":          true, // framework/catalog/group names
	"control_ref":   true, // "A.5.1"-style ISO/BSI control reference
	"type":          true, // report/entity type enum
	"token":         true, // hex-encoded opaque tokens (invite/session/share/portal) — not UUIDs
	"slug":          true,
	"stage":         true, // NIS2 report stage enum (early_warning|full_report|final_report)
	"key":           true, // vaktvault secret key name (user-chosen string)
	"code":          true, // physical-control template code ("PHY-01"-style)
	"email":         true, // admin/password-reset lookups by email address
	"severity":      true, // vaktscan SLA policy severity enum
	"v":             true, // policy version number (int, validated by the handler itself)
	"step_id":       true, // vakthr checklist step id — TEXT column, JSON item key, not UUID
	"bausteinId":    true, // BSI Baustein reference ("APP.1.1"-style)
	"anforderungId": true, // BSI Anforderung reference ("APP.1.1.A1"-style)
}

// ValidateUUIDParams rejects a request whose UUID-typed path param is
// syntactically not a UUID, returning 400 before the handler runs. Without it,
// a malformed id reaches a query that casts it to ::uuid and Postgres raises
// SQLSTATE 22P02; handlers that do not special-case that error map it to 500.
// A crafted "/vaktcomply/controls/not-a-uuid/measures" (or a nested
// "/controls/:id/tasks/not-a-uuid") is the only way to hit it — the SPA always
// sends real UUIDs from list responses — so this is defence-in-depth against
// malformed input, turning a spurious 500 into a correct 400.
//
// Every path param is validated EXCEPT those in nonUUIDParamNames (see its
// doc comment for why a denylist, not an allowlist).
func ValidateUUIDParams() echo.MiddlewareFunc {
	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			names := c.ParamNames()
			values := c.ParamValues()
			for i, name := range names {
				if nonUUIDParamNames[name] {
					continue
				}
				v := values[i]
				// An empty UUID param (a "//" in the path Caddy did not normalise,
				// e.g. /vaktcomply/controls//measures) previously fell through here
				// and reached a ::uuid cast → 22P02 → 500 (R-H02/S131-D5). Empty is
				// not a valid UUID, so it is a 400 like any other malformed value.
				if _, err := uuid.Parse(v); err != nil {
					return c.JSON(http.StatusBadRequest, map[string]string{
						"error": "invalid id: must be a UUID",
						"code":  "INVALID_UUID_PARAM",
					})
				}
			}
			return next(c)
		}
	}
}
