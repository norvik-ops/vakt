// Copyright (c) 2026 NorvikOps. All rights reserved.
// SPDX-License-Identifier: Elastic-2.0

// Package apperr is the single source of truth for turning a repository/service
// error into the right HTTP status class — the 4xx-vs-5xx boundary (S4, D13-B).
//
// It carries NO echo/HTTP dependency so repositories and services can import it
// for the ErrNotFound sentinel without pulling the web layer in. The HTTP-facing
// responder that consumes Status/Classify lives in shared/httputil.
//
// Why this exists: a live write-sweep repeatedly found ~40+ handlers answering
// 500 for what is really a client mistake (a not-found row, a duplicate, a
// bad-shaped input). The historical failure mode was each module reinventing its
// own isNotFound() that knew only a subset of sentinels, so a repo that returned
// a raw fmt.Errorf("X not found") (no sentinel) slipped through as a 500. This
// package centralises that logic so every module classifies identically.
package apperr

import (
	"errors"
	"strings"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
)

// ErrNotFound is the canonical "resource does not exist" sentinel. Repositories
// SHOULD wrap it so that IsNotFound matches by errors.Is:
//
//	fmt.Errorf("widget %s: %w", id, apperr.ErrNotFound)   // "widget 7: not found"
//
// Note the idiom: name the resource, then wrap. Writing "widget not found: %w"
// yields the stuttering "widget not found: not found" in logs. Even a raw string
// ending in "not found" is caught by the HasSuffix fallback below, but wrapping
// the sentinel is the documented contract.
var ErrNotFound = errors.New("not found")

// IsNotFound reports whether err means "the requested resource does not exist".
//
// It matches, in order: the canonical ErrNotFound sentinel, pgx.ErrNoRows (a
// bare SELECT that returned no row), and — as a safety net for repositories that
// return an un-wrapped fmt.Errorf("… not found") — any message ENDING in "not
// found".
//
// The suffix check is deliberately HasSuffix, never Contains: a real schema bug
// surfaces as `ERROR: column "x" does not exist`, which ends in "does not exist"
// and must stay a 500 so the next sweep catches it — Contains("not found") would
// swallow it into a 404 and hide the bug.
func IsNotFound(err error) bool {
	if err == nil {
		return false
	}
	if errors.Is(err, ErrNotFound) || errors.Is(err, pgx.ErrNoRows) {
		return true
	}
	return strings.HasSuffix(err.Error(), "not found")
}

// IsBadParam reports whether err was caused by malformed caller input that
// reached Postgres: a bad UUID/int in a path or body (SQLSTATE 22P02,
// invalid_text_representation), a numeric overflow (22003), or an invalid
// date/time value (22007/22008). These are client errors → 400, never 500.
//
// Note: a malformed UUID in a *path* param is normally caught up front by the
// ValidateUUIDParams middleware; IsBadParam is the in-handler fallback for
// body-supplied values and non-UUID path params.
func IsBadParam(err error) bool {
	code := sqlState(err)
	switch code {
	case "22P02", // invalid_text_representation (bad UUID / int / enum)
		"22003", // numeric_value_out_of_range
		"22007", // invalid_datetime_format
		"22008": // datetime_field_overflow
		return true
	}
	return false
}

// IsConflict reports whether err is a unique-constraint violation (SQLSTATE
// 23505) — the row already exists → 409.
func IsConflict(err error) bool { return sqlState(err) == "23505" }

// IsUnprocessable reports whether err is a constraint violation that means the
// submitted values are individually valid in shape but violate a data rule: a
// CHECK constraint (23514) or a NOT NULL violation (23502). These map to 422
// Unprocessable Entity.
func IsUnprocessable(err error) bool {
	switch sqlState(err) {
	case "23514", // check_violation
		"23502": // not_null_violation
		return true
	}
	return false
}

// IsReferenceConflict reports a foreign-key violation (23503) → 409.
//
// 23503 deliberately does NOT map to 422. Postgres raises the same SQLSTATE for
// both directions, and one of them has no input at all:
//
//   - INSERT/UPDATE referencing a parent that does not exist
//   - DELETE of a parent that still has children ("still in use")
//
// Answering the DELETE case with 422 "input violates a data rule" is misleading
// — there is no input to violate anything. 409 with a reference-specific message
// is true for both readings, which is why it gets its own class rather than
// riding along with the CHECK/NOT NULL bucket.
func IsReferenceConflict(err error) bool { return sqlState(err) == "23503" }

// Status maps err to the HTTP status a handler should return. It returns 0 when
// err is not one of the recognised client-error classes, so the caller keeps its
// own fallback (normally 500). Order matters: not-found and bad-param are checked
// before the generic constraint classes.
func Status(err error) int {
	switch {
	case err == nil:
		return 0
	case IsNotFound(err):
		return 404 // http.StatusNotFound
	case IsBadParam(err):
		return 400 // http.StatusBadRequest
	case IsConflict(err), IsReferenceConflict(err):
		return 409 // http.StatusConflict
	case IsUnprocessable(err):
		return 422 // http.StatusUnprocessableEntity
	default:
		return 0
	}
}

// sqlState extracts the 5-char SQLSTATE from a wrapped pgconn.PgError, or "" if
// err is not a Postgres error.
func sqlState(err error) string {
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) {
		return pgErr.Code
	}
	return ""
}
