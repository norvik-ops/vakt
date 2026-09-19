// Copyright (c) 2026 NorvikOps. All rights reserved.
// SPDX-License-Identifier: Elastic-2.0

package main

import (
	"context"
	"errors"
	"fmt"
	"testing"

	"github.com/hibiken/asynq"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
)

func TestIsTerminalError(t *testing.T) {
	cases := []struct {
		name string
		err  error
		want bool
	}{
		{"nil", nil, false},
		{"deleted row (pgx.ErrNoRows)", pgx.ErrNoRows, true},
		{"wrapped pgx.ErrNoRows", fmt.Errorf("load incident: %w", pgx.ErrNoRows), true},
		{"FK violation 23503", &pgconn.PgError{Code: "23503"}, true},
		{"malformed uuid 22P02", &pgconn.PgError{Code: "22P02"}, true},
		{"transient serialization 40001", &pgconn.PgError{Code: "40001"}, false},
		{"deadlock 40P01", &pgconn.PgError{Code: "40P01"}, false},
		{"generic error", errors.New("connection reset by peer"), false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := isTerminalError(tc.err); got != tc.want {
				t.Fatalf("isTerminalError(%v) = %v, want %v", tc.err, got, tc.want)
			}
		})
	}
}

// TestTerminalErrorMiddleware verifies the middleware turns a terminal error into
// a SkipRetry-wrapping error (→ archive, no retry) while leaving a transient error
// and a nil result untouched (→ normal retry / success).
func TestTerminalErrorMiddleware(t *testing.T) {
	wrap := func(returned error) error {
		h := terminalErrorMiddleware(asynq.HandlerFunc(func(context.Context, *asynq.Task) error {
			return returned
		}))
		return h.ProcessTask(context.Background(), asynq.NewTask("t", nil))
	}

	// Terminal → SkipRetry, and the original error stays inspectable in the chain.
	terminal := fmt.Errorf("update: %w", &pgconn.PgError{Code: "23503"})
	got := wrap(terminal)
	if !errors.Is(got, asynq.SkipRetry) {
		t.Fatalf("terminal error not wrapped in SkipRetry: %v", got)
	}
	var pgErr *pgconn.PgError
	if !errors.As(got, &pgErr) || pgErr.Code != "23503" {
		t.Fatalf("original pg error lost from chain: %v", got)
	}

	// Transient → passed through unchanged, must still retry.
	transient := errors.New("dial tcp: i/o timeout")
	if got := wrap(transient); errors.Is(got, asynq.SkipRetry) {
		t.Fatalf("transient error must not be marked SkipRetry: %v", got)
	}

	// Success → nil stays nil.
	if got := wrap(nil); got != nil {
		t.Fatalf("nil result must stay nil, got %v", got)
	}
}
