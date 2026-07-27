// Copyright (c) 2026 NorvikOps. All rights reserved.
// SPDX-License-Identifier: Elastic-2.0

package auth

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/require"
)

// TestCheckAndMarkTOTPCode_FailClosedOnRedisError is the S12/D24-3 (≡ V21-1)
// regression: without a reachable Redis, the single-use guarantee for a TOTP
// code cannot be enforced, so the code must be rejected rather than accepted.
//
// The error must ALSO be distinguishable from a genuine replay. Both used to
// collapse into "invalid code", which tells a user with a working authenticator
// to chase clock drift while the operator's logs stay silent (ADR-0044).
func TestCheckAndMarkTOTPCode_FailClosedOnRedisError(t *testing.T) {
	h := &TotpHandler{svc: &Service{redis: dialFailingRedis(t)}}

	err := h.checkAndMarkTOTPCode(context.Background(), "user-1", "123456")
	require.Error(t, err, "a TOTP code must be rejected when replay-protection storage is unreachable")
	require.True(t, errors.Is(err, ErrTOTPReplayCheckUnavailable),
		"an outage must carry its own sentinel so the handler can answer 503, not 422 'already used'")
	require.False(t, errors.Is(err, ErrTOTPCodeReplayed),
		"an outage must never be reported as a replay")
}

// TestCheckAndMarkTOTPCode_RejectsMissingRedisClient covers the nil-redis
// case (e.g. TotpHandler constructed without a Service/redis wired up) —
// this must also fail closed, not silently accept the code.
func TestCheckAndMarkTOTPCode_RejectsMissingRedisClient(t *testing.T) {
	h := &TotpHandler{svc: &Service{redis: nil}}

	err := h.checkAndMarkTOTPCode(context.Background(), "user-1", "123456")
	require.Error(t, err)
	require.True(t, errors.Is(err, ErrTOTPReplayCheckUnavailable))
}

// TestCheckAndMarkTOTPCode_RespectsFailOpenOptOut pins that the documented
// operator switch actually governs this path. VAKT_AUTH_FAIL_OPEN_ON_REDIS_OUTAGE
// is documented as controlling auth behaviour during a Redis outage; if it did
// not apply here, the documented opt-out would silently exclude the one code
// path users hit on every MFA login.
func TestCheckAndMarkTOTPCode_RespectsFailOpenOptOut(t *testing.T) {
	h := &TotpHandler{svc: &Service{redis: dialFailingRedis(t), failOpenOnRedisOutage: true}}

	require.NoError(t, h.checkAndMarkTOTPCode(context.Background(), "user-1", "123456"),
		"with fail-open explicitly enabled, a Redis outage must not block the login")
}
