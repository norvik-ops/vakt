// Copyright (c) 2026 NorvikOps. All rights reserved.
// SPDX-License-Identifier: Elastic-2.0

package ai

import (
	"context"
	"errors"
	"testing"

	"github.com/redis/go-redis/v9"
)

// TestCheckRateLimit_FailClosed verifies that when Redis is unreachable and
// the tracker is in its default (fail-closed) mode, the rate-limit check
// returns ErrUsageCheckUnavailable instead of silently allowing the call.
// Pairs with the audit finding from outputs/final_audit.md (Top-3 #2): the
// previous behaviour was `log.Warn ... — allowing`, inconsistent with the
// auth-lockout fail-closed posture from ADR-0044.
func TestCheckRateLimit_FailClosed(t *testing.T) {
	// Point the client at a port that nothing listens on so every Incr fails.
	rdb := redis.NewClient(&redis.Options{Addr: "127.0.0.1:1"})
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
	rdb := redis.NewClient(&redis.Options{Addr: "127.0.0.1:1"})
	tracker := NewUsageTracker(rdb, nil, UsageTrackerConfig{RateLimitRPM: 10}).
		WithFailOpenOnOutage(true)

	if err := tracker.CheckRateLimit(context.Background(), "00000000-0000-0000-0000-000000000001"); err != nil {
		t.Fatalf("expected nil error with fail-open, got %v", err)
	}
}
