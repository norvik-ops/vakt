// Copyright (c) 2026 NorvikOps. All rights reserved.
// SPDX-License-Identifier: Elastic-2.0

package apperr

import (
	"errors"
	"fmt"
	"net/http"
	"testing"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
)

func pgErr(code string) error { return &pgconn.PgError{Code: code} }

func TestIsNotFound(t *testing.T) {
	cases := []struct {
		name string
		err  error
		want bool
	}{
		{"nil", nil, false},
		{"sentinel", ErrNotFound, true},
		{"wrapped sentinel", fmt.Errorf("widget %d not found: %w", 7, ErrNotFound), true},
		{"pgx no rows", pgx.ErrNoRows, true},
		{"raw suffix (no sentinel)", errors.New("isms scope not found"), true},
		{"module-local sentinel message", errors.New("not found"), true},
		// The critical negative: a real schema error must stay a 500, not become 404.
		{"schema column missing", errors.New(`ERROR: column "x" does not exist (SQLSTATE 42703)`), false},
		{"generic failure", errors.New("connection reset"), false},
		{"contains but not suffix", errors.New("not found: retrying upstream"), false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := IsNotFound(tc.err); got != tc.want {
				t.Errorf("IsNotFound(%v) = %v, want %v", tc.err, got, tc.want)
			}
		})
	}
}

func TestClassifiers(t *testing.T) {
	cases := []struct {
		name          string
		err           error
		badParam      bool
		conflict      bool
		unprocessable bool
	}{
		{"bad uuid", pgErr("22P02"), true, false, false},
		{"numeric overflow", pgErr("22003"), true, false, false},
		{"bad datetime", pgErr("22007"), true, false, false},
		{"unique violation", pgErr("23505"), false, true, false},
		{"check violation", pgErr("23514"), false, false, true},
		{"not null", pgErr("23502"), false, false, true},
		// 23503 is NOT unprocessable: Postgres raises it both for a bad reference
		// on INSERT and for a DELETE blocked by existing children. The latter has
		// no request input, so 422 "input violates a data rule" would be untrue.
		// It classifies as a reference conflict (409) instead.
		{"foreign key", pgErr("23503"), false, false, false},
		{"wrapped unique", fmt.Errorf("create: %w", pgErr("23505")), false, true, false},
		{"non-pg error", errors.New("boom"), false, false, false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if IsBadParam(tc.err) != tc.badParam {
				t.Errorf("IsBadParam = %v, want %v", IsBadParam(tc.err), tc.badParam)
			}
			if IsConflict(tc.err) != tc.conflict {
				t.Errorf("IsConflict = %v, want %v", IsConflict(tc.err), tc.conflict)
			}
			if IsUnprocessable(tc.err) != tc.unprocessable {
				t.Errorf("IsUnprocessable = %v, want %v", IsUnprocessable(tc.err), tc.unprocessable)
			}
		})
	}
}

func TestStatus(t *testing.T) {
	cases := []struct {
		name string
		err  error
		want int
	}{
		{"nil", nil, 0},
		{"not found", ErrNotFound, http.StatusNotFound},
		{"bad param", pgErr("22P02"), http.StatusBadRequest},
		{"conflict", pgErr("23505"), http.StatusConflict},
		{"unprocessable check", pgErr("23514"), http.StatusUnprocessableEntity},
		{"unprocessable not-null", pgErr("23502"), http.StatusUnprocessableEntity},
		{"reference conflict (fk)", pgErr("23503"), http.StatusConflict},
		{"unknown -> 0 (caller falls back to 500)", errors.New("boom"), 0},
		// not-found wins over a co-occurring schema-looking suffix
		{"wrapped not found", fmt.Errorf("scope not found"), http.StatusNotFound},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := Status(tc.err); got != tc.want {
				t.Errorf("Status(%v) = %d, want %d", tc.err, got, tc.want)
			}
		})
	}
}
