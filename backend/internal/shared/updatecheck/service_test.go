// Copyright (c) 2026 NorvikOps. All rights reserved.
// SPDX-License-Identifier: Elastic-2.0

package updatecheck

import (
	"context"
	"testing"
	"time"

	"github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/assert"
)

// dialFailingRedis returns a client that always errors (unreachable port).
func dialFailingRedis(t *testing.T) *redis.Client {
	t.Helper()
	return redis.NewClient(&redis.Options{
		Addr:        "127.0.0.1:1",
		DialTimeout: 50 * time.Millisecond,
		ReadTimeout: 50 * time.Millisecond,
		MaxRetries:  -1,
	})
}

// TestIsEnabled_FallsBackToEnvWhenRedisUnreachable verifies that isEnabled
// returns the env-var default when Redis cannot be reached.
func TestIsEnabled_FallsBackToEnvWhenRedisUnreachable(t *testing.T) {
	svc := &Service{enabled: true, rdb: dialFailingRedis(t)}
	assert.True(t, svc.isEnabled(context.Background()))

	svc2 := &Service{enabled: false, rdb: dialFailingRedis(t)}
	assert.False(t, svc2.isEnabled(context.Background()))
}

// TestIsNewer covers the domain invariant: update_available must only be true
// when the latest version is strictly greater than the current one.
func TestIsNewer(t *testing.T) {
	cases := []struct {
		candidate, current string
		want               bool
	}{
		{"1.2.4", "1.2.3", true},
		{"1.3.0", "1.2.9", true},
		{"2.0.0", "1.99.99", true},
		{"1.2.3", "1.2.3", false}, // same → no update
		{"1.2.2", "1.2.3", false}, // older
		{"", "1.2.3", false},      // empty candidate
		{"1.2.3", "", false},      // empty current
	}
	for _, c := range cases {
		got := isNewer(c.candidate, c.current)
		assert.Equalf(t, c.want, got, "isNewer(%q, %q)", c.candidate, c.current)
	}
}

func TestNormalizeVersion(t *testing.T) {
	assert.Equal(t, "1.2.3", normalizeVersion("v1.2.3"))
	assert.Equal(t, "1.2.3", normalizeVersion("1.2.3"))
	assert.Equal(t, "1.2.3", normalizeVersion("  v1.2.3  "))
}
