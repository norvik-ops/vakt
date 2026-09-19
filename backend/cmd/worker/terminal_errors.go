// Copyright (c) 2026 NorvikOps. All rights reserved.
// SPDX-License-Identifier: Elastic-2.0
// See LICENSE file in the project root for terms.

package main

import (
	"context"
	"errors"
	"fmt"

	"github.com/hibiken/asynq"
	"github.com/jackc/pgx/v5"

	"github.com/matharnica/vakt/internal/shared/apperr"
)

// R1-W19-10: central classification of terminal (non-retryable) worker errors.
//
// Before this, no handler except notify.DeliverHandler used asynq.SkipRetry, so
// a job that failed on a permanently-broken input was retried up to 25× before
// being archived. The three classes below can NEVER succeed on retry — the row
// is gone, the referenced parent is gone, or the caller-supplied id is malformed
// — yet each retry counted as a failure and fed the queue:health alarm with
// noise that no operator action can clear.
//
// asynq decides retry-vs-archive purely from the error the handler RETURNS
// (processor.go: `errors.Is(err, SkipRetry)`); the Config.IsFailure hook only
// flips the failure-stats flag and does NOT stop the retry loop. So the fix has
// to rewrite the returned error, which is what terminalErrorMiddleware does:
// wrap any terminal error in asynq.SkipRetry so the task is archived once
// instead of bouncing 25×.
//
// What this does NOT do: it does not classify transient failures (DB timeout,
// Redis hiccup, HTTP 5xx from an upstream) — those must keep retrying, so they
// fall through unchanged.

// isTerminalError reports whether err can never succeed on retry:
//   - pgx.ErrNoRows      — the target row was deleted between enqueue and run
//   - 23503 (FK violation) — the referenced parent no longer exists
//   - 22P02 (bad UUID/int) — the enqueued id is malformed
//
// It deliberately uses errors.Is(pgx.ErrNoRows) rather than apperr.IsNotFound:
// the latter also treats any message ending in "not found" as not-found, which
// is too broad a net for a retry decision.
func isTerminalError(err error) bool {
	if err == nil {
		return false
	}
	return errors.Is(err, pgx.ErrNoRows) ||
		apperr.IsReferenceConflict(err) || // 23503
		apperr.IsBadParam(err) // 22P02 (and 22003/22007/22008)
}

// terminalErrorMiddleware wraps every handler so a terminal error is converted
// to an asynq.SkipRetry-wrapping error. The original error is preserved in the
// chain (double %w) so downstream classification/logging still sees it.
func terminalErrorMiddleware(next asynq.Handler) asynq.Handler {
	return asynq.HandlerFunc(func(ctx context.Context, t *asynq.Task) error {
		err := next.ProcessTask(ctx, t)
		if isTerminalError(err) {
			return fmt.Errorf("%w: terminal, not retried: %w", asynq.SkipRetry, err)
		}
		return err
	})
}
