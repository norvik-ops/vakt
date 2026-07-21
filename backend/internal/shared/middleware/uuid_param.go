// Copyright (c) 2026 NorvikOps. All rights reserved.
// SPDX-License-Identifier: Elastic-2.0

package middleware

import (
	"net/http"

	"github.com/google/uuid"
	"github.com/labstack/echo/v4"
)

// uuidParamNames lists the path-param names that ALWAYS carry a resource UUID
// across the six business modules. Non-UUID identifiers deliberately use other
// names (:name for framework names, :control_ref for "A.5.1"-style refs, :type
// for report types, :token for share tokens, :slug, :report, :v for version
// ints) and are intentionally NOT validated here.
var uuidParamNames = map[string]bool{
	"id":  true,
	"cid": true, // control id
	"fid": true, // framework id
	"eid": true, // employee/entity id
	// S125-7 (N5/D13): additional resource-UUID params that were unguarded, so a
	// malformed value hit ::uuid and produced 500 instead of 400. All verified as
	// resource IDs (never a slug/ref/name). Non-UUID params keep distinct names
	// (see the doc comment above) and are deliberately excluded.
	"project_id":  true, // vaktvault project id
	"env_id":      true, // vaktvault environment id
	"result_id":   true, // vaktscan scan-result id
	"schedule_id": true, // vaktscan schedule id
	"rid":         true, // vakthr role id
	"control_id":  true, // vaktcomply control id (distinct from :control_ref)
	"tid":         true, // task id
	"mid":         true, // measure id
	"qid":         true, // quiz question id
	"aid":         true, // quiz answer id
	// 2026-07-16: found by re-probing with the now-complete OpenAPI (133 GET-{id}
	// routes instead of 80). These three sit outside the module groups the guard
	// was originally mounted on, or use a name the set did not know — all 500'd.
	"user_id":  true, // admin user id
	"policyId": true, // trust-center policy id
	"reportId": true, // vaktcomply incident-report id
}

// ValidateUUIDParams rejects a request whose UUID-typed path param (see
// uuidParamNames) is syntactically not a UUID, returning 400 before the handler
// runs. Without it, a malformed id reaches a query that casts it to ::uuid and
// Postgres raises SQLSTATE 22P02; handlers that do not special-case that error
// map it to 500. A crafted "/vaktcomply/controls/not-a-uuid/measures" is the
// only way to hit it — the SPA always sends real UUIDs from list responses — so
// this is defence-in-depth against malformed input, turning a spurious 500 into
// a correct 400. Params carrying a valid UUID, or params not in the set, pass
// through untouched.
func ValidateUUIDParams() echo.MiddlewareFunc {
	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			names := c.ParamNames()
			values := c.ParamValues()
			for i, name := range names {
				if !uuidParamNames[name] {
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
