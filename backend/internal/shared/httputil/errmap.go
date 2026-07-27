// Copyright (c) 2026 NorvikOps. All rights reserved.
// SPDX-License-Identifier: Elastic-2.0

package httputil

import (
	"net/http"

	"github.com/labstack/echo/v4"

	"github.com/matharnica/vakt/internal/shared/apperr"
)

// RespondError is the shared 4xx-vs-5xx responder for handler DB-error branches
// (S4, D13-B). Give it the raw error plus the fallback message/code the handler
// would otherwise have used for its 500, and it picks the right status:
//
//   - resource not found         → 404 NOT_FOUND
//   - malformed input (22P02…)   → 400 BAD_REQUEST
//   - unique violation (23505)   → 409 CONFLICT
//   - foreign-key violation      → 409 REFERENCE_CONFLICT ("still referenced")
//   - check / not-null (2350x)   → 422 UNPROCESSABLE
//   - anything else              → the caller's fallback status (default 500)
//
// The raw error is never sent to the client — only the curated message/code.
// The fallback message/code are used only for the 500 case; the 4xx cases carry
// a stable, generic message and code so the boundary is uniform across modules.
func RespondError(c echo.Context, err error, fallbackMsg, fallbackCode string) error {
	switch apperr.Status(err) {
	case http.StatusNotFound:
		return errorJSON(c, http.StatusNotFound, "resource not found", "NOT_FOUND")
	case http.StatusBadRequest:
		return errorJSON(c, http.StatusBadRequest, "invalid input", "BAD_REQUEST")
	case http.StatusConflict:
		// Both 23505 and 23503 are 409, but they mean opposite things and one of
		// them (a DELETE blocked by children) has no request body at all — so
		// they must not share a message.
		if apperr.IsReferenceConflict(err) {
			return errorJSON(c, http.StatusConflict,
				"resource is still referenced by other data", "REFERENCE_CONFLICT")
		}
		return errorJSON(c, http.StatusConflict, "resource already exists", "CONFLICT")
	case http.StatusUnprocessableEntity:
		return errorJSON(c, http.StatusUnprocessableEntity, "input violates a data rule", "UNPROCESSABLE")
	default:
		return errorJSON(c, http.StatusInternalServerError, fallbackMsg, fallbackCode)
	}
}

// errorJSON writes the platform-standard error envelope: {"error": …, "code": …}.
func errorJSON(c echo.Context, status int, msg, code string) error {
	return c.JSON(status, map[string]string{"error": msg, "code": code})
}
