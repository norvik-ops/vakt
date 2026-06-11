// Copyright (c) 2026 NorvikOps. All rights reserved.
// SPDX-License-Identifier: Elastic-2.0

package auth

// Unit tests for denyListFallback nil-guard paths.
//
// These tests verify that all denyListFallback methods handle nil receiver
// and nil db gracefully — no panics, sensible defaults. This is important
// because the fallback is always constructed (NewService wires it up) but
// might be called with a nil db in constrained test environments.

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

// TestDenyListFallback_NilReceiver_RevokeIsNoop verifies that calling
// revokeInFallback on a nil *denyListFallback does not panic.
func TestDenyListFallback_NilReceiver_RevokeIsNoop(t *testing.T) {
	var f *denyListFallback
	// Must not panic — nil guard at the top of the method.
	f.revokeInFallback(context.Background(), "abc123", time.Now().Add(time.Hour))
}

// TestDenyListFallback_NilReceiver_IsRevokedReturnsFalse verifies that
// isRevokedInFallback on a nil receiver returns false (safe default).
func TestDenyListFallback_NilReceiver_IsRevokedReturnsFalse(t *testing.T) {
	var f *denyListFallback
	revoked := f.isRevokedInFallback(context.Background(), "abc123")
	assert.False(t, revoked, "nil fallback must report token as not revoked (fail-open for deny-list)")
}

// TestDenyListFallback_NilDB_RevokeIsNoop verifies that the nil-db guard
// inside revokeInFallback suppresses the call without panicking.
func TestDenyListFallback_NilDB_RevokeIsNoop(t *testing.T) {
	f := &denyListFallback{db: nil}
	f.revokeInFallback(context.Background(), "abc123", time.Now().Add(time.Hour))
}

// TestDenyListFallback_NilDB_IsRevokedReturnsFalse verifies the nil-db guard
// in isRevokedInFallback returns false (token valid, fail-open).
func TestDenyListFallback_NilDB_IsRevokedReturnsFalse(t *testing.T) {
	f := &denyListFallback{db: nil}
	revoked := f.isRevokedInFallback(context.Background(), "abc123")
	assert.False(t, revoked)
}

// TestCleanupExpiredFallbackEntries_NilDB verifies that passing nil as the
// DB pool to cleanupExpiredFallbackEntries does not panic.
func TestCleanupExpiredFallbackEntries_NilDB(t *testing.T) {
	// The nil guard at the start of the function must prevent any DB call.
	cleanupExpiredFallbackEntries(context.Background(), nil)
}
