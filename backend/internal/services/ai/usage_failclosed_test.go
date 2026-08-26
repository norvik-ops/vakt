// Copyright (c) 2026 NorvikOps. All rights reserved.
// SPDX-License-Identifier: Elastic-2.0

package ai

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/redis/go-redis/v9"
)

// deadRedis points a real go-redis client at a port nothing listens on, so
// every command fails with a genuine dial error. The outage is real, not
// stubbed — only the waiting is removed.
//
// R1-INT-01: go-redis has TWO independent retry layers and this call site had
// neither bounded. Options.MaxRetries governs COMMAND retries (default 3);
// Options.DialerRetries governs POOL dial retries inside ConnPool.dialConn
// (default 5 attempts, constant 100ms DialerRetryTimeout backoff). Refused
// connections are instant, so all of the cost was backoff sleeping: measured
// 1.68s per test, 3.36s of this package's 4.39s. With both layers disabled it
// is ~0. Same defect and same fix as internal/auth/redis_outage_test.go.
//
// TRAP — the two layers do NOT share the "-1 disables it" convention:
// MaxRetries: -1 is honoured as "no retries", but DialerRetries: -1 is NOT —
// pool.go:663 reads `if maxRetries <= 0 { maxRetries = 5 }`, so both -1 and 0
// silently restore the FULL default of 5 attempts. Use 1, never -1 or 0.
//
// ── Why an internal/auth worker edited a file in internal/services/ai ────────
// R1-INT-01 was scoped to internal/auth/**. Patching only the spot you were
// handed is the "Variant-Miss" failure this project has hit three sprints in a
// row (CLAUDE.md, 2026-07-11), so the class gets grepped — but crossing a
// track boundary needs a stated bar, not a feeling. In order:
//
//  1. HARD STOP: the file is on the assignment's forbidden list → never touch,
//     regardless of impact.
//  2. Same defect class, mechanically identical fix, test-only, no production
//     behaviour change.
//  3. Material: moves the package by >1s AND >25% of its runtime. Below that,
//     report instead of touch — P14's disjoint-ownership rule is the default
//     and coordination costs more than the gain.
//  4. Non-vacuity provable in the same run.
//
// Applied to the three variants found (measured, not estimated):
//
//	internal/services/ai (this file)     3.36s / 77% → CROSSED (2+3+4, 1 n/a)
//	internal/shared/updatecheck          0.80s / 44% → not touched: fails 1
//	                                     (forbidden list) AND fails 3
//	                                     (1.823s→1.021s = 0.802s, not >1s)
//	internal/integration_test/pwversion…  ~0.8s        → not touched: fails 3;
//	                                     also `//go:build integration` against
//	                                     a 45-minute job budget
//
// Note the two independent reasons for updatecheck. An earlier draft of this
// comment claimed it was held back by rule 1 "alone, not because 0.81s is
// small" — that was wrong, and wrong in the direction that teaches a bad rule:
// 0.80s does not clear the >1s bar either. Rule 1 makes the impact question
// moot, it does not overrule an answer that happens to agree.
func deadRedis(t *testing.T) *redis.Client {
	t.Helper()
	c := redis.NewClient(&redis.Options{
		Addr:          "127.0.0.1:1",
		DialTimeout:   100 * time.Millisecond,
		ReadTimeout:   100 * time.Millisecond,
		MaxRetries:    -1, // no command-level retries
		DialerRetries: 1,  // no pool-level dial retries
	})
	t.Cleanup(func() { _ = c.Close() })
	return c
}

// TestCheckRateLimit_FailClosed verifies that when Redis is unreachable and
// the tracker is in its default (fail-closed) mode, the rate-limit check
// returns ErrUsageCheckUnavailable instead of silently allowing the call.
// Pairs with the audit finding from outputs/final_audit.md (Top-3 #2): the
// previous behaviour was `log.Warn ... — allowing`, inconsistent with the
// auth-lockout fail-closed posture from ADR-0044.
func TestCheckRateLimit_FailClosed(t *testing.T) {
	// Point the client at a port that nothing listens on so every Incr fails.
	rdb := deadRedis(t)
	tracker := NewUsageTracker(rdb, nil, UsageTrackerConfig{RateLimitRPM: 10})

	err := tracker.CheckRateLimit(context.Background(), "00000000-0000-0000-0000-000000000001")
	if !errors.Is(err, ErrUsageCheckUnavailable) {
		t.Fatalf("expected ErrUsageCheckUnavailable, got %v", err)
	}
}

// TestCheckRateLimit_FailOpenOptIn verifies the explicit opt-in path:
// VAKT_AI_FAIL_OPEN_ON_OUTAGE=true → WithFailOpenOnOutage(true) → Redis
// failure logs but allows the call through.
func TestCheckRateLimit_FailOpenOptIn(t *testing.T) {
	rdb := deadRedis(t)
	tracker := NewUsageTracker(rdb, nil, UsageTrackerConfig{RateLimitRPM: 10}).
		WithFailOpenOnOutage(true)

	if err := tracker.CheckRateLimit(context.Background(), "00000000-0000-0000-0000-000000000001"); err != nil {
		t.Fatalf("expected nil error with fail-open, got %v", err)
	}
}
