//go:build integration

// Copyright (c) 2026 NorvikOps. All rights reserved.
// SPDX-License-Identifier: Elastic-2.0

package auth

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/require"
	"github.com/testcontainers/testcontainers-go"
	tcwait "github.com/testcontainers/testcontainers-go/wait"
)

// The unit tests in totp_replay_failclosed_test.go cover the FAILURE directions
// (Redis unreachable → fail closed; documented opt-out honoured). Both assert
// "returns an error", which would stay green for an implementation that rejects
// EVERY code — i.e. one that locks every user out of MFA entirely.
//
// These cover the directions that need a real Redis: a fresh code must be
// ACCEPTED, and the same code inside its window must be REFUSED carrying the
// replay sentinel (not the outage one). Together they pin that the two error
// classes stay distinguishable — which is what lets the handler answer 503 for
// an outage and 422 for a genuine replay (ADR-0044).
//
// Lives in package auth deliberately: exporting test-only helpers from
// production code just to reach an external test package would be worse.

func realRedis(ctx context.Context, t *testing.T) (*redis.Client, func()) {
	t.Helper()
	ctr, err := testcontainers.GenericContainer(ctx, testcontainers.GenericContainerRequest{
		ContainerRequest: testcontainers.ContainerRequest{
			Image:        "redis:7-alpine",
			ExposedPorts: []string{"6379/tcp"},
			WaitingFor:   tcwait.ForListeningPort("6379/tcp").WithStartupTimeout(60 * time.Second),
		},
		Started: true,
	})
	if err != nil {
		if strings.Contains(err.Error(), "permission denied") ||
			strings.Contains(err.Error(), "Cannot connect to the Docker daemon") {
			t.Skipf("integration: Docker unavailable (%v)", err)
		}
		t.Fatalf("redis container: %v", err)
	}
	host, err := ctr.Host(ctx)
	require.NoError(t, err)
	port, err := ctr.MappedPort(ctx, "6379/tcp")
	require.NoError(t, err)

	rdb := redis.NewClient(&redis.Options{Addr: host + ":" + port.Port()})
	return rdb, func() {
		_ = rdb.Close()
		_ = ctr.Terminate(ctx)
	}
}

func TestTOTPReplay_AcceptsFirstUseRejectsSecond(t *testing.T) {
	if testing.Short() {
		t.Skip("integration: -short mode")
	}
	ctx := context.Background()
	rdb, cleanup := realRedis(ctx, t)
	defer cleanup()

	h := &TotpHandler{svc: &Service{redis: rdb}}

	// Without this first assertion, an implementation that refuses everything
	// would pass every fail-closed test and break all logins.
	require.NoError(t, h.checkAndMarkTOTPCode(ctx, "user-1", "123456"),
		"a code used for the first time must be accepted")

	err := h.checkAndMarkTOTPCode(ctx, "user-1", "123456")
	require.Error(t, err, "replaying a code inside its window must be refused")
	require.True(t, errors.Is(err, ErrTOTPCodeReplayed),
		"a real replay must carry the replay sentinel")
	require.False(t, errors.Is(err, ErrTOTPReplayCheckUnavailable),
		"a replay must never be reported as an outage — they map to different HTTP statuses")
}

// A spent code for one user must not lock another user out, and a different code
// for the same user must still work: a key-scoping regression would be a
// self-inflicted denial of service on MFA.
func TestTOTPReplay_ScopedPerUserAndCode(t *testing.T) {
	if testing.Short() {
		t.Skip("integration: -short mode")
	}
	ctx := context.Background()
	rdb, cleanup := realRedis(ctx, t)
	defer cleanup()

	h := &TotpHandler{svc: &Service{redis: rdb}}

	require.NoError(t, h.checkAndMarkTOTPCode(ctx, "user-1", "123456"))
	require.NoError(t, h.checkAndMarkTOTPCode(ctx, "user-2", "123456"),
		"another user's code must be unaffected")
	require.NoError(t, h.checkAndMarkTOTPCode(ctx, "user-1", "654321"),
		"a different code for the same user must be unaffected")
}
