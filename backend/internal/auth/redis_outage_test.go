// Copyright (c) 2026 NorvikOps. All rights reserved.
// SPDX-License-Identifier: Elastic-2.0

package auth

import (
	"testing"
	"time"

	"github.com/redis/go-redis/v9"
)

// dialFailingRedis is the single dead-Redis client for this package. It points a
// real go-redis client at a port that is guaranteed to be unreachable, so every
// call returns a genuine dial error — exactly the situation the fail-open /
// fail-closed paths are designed for. The error is real, not stubbed: nothing
// about the outage semantics is faked here.
//
// R1-INT-01 — why DialerRetries matters, and why MaxRetries did NOT fix it:
// go-redis has TWO independent retry layers, and the options that look like
// they cover retries only touch the upper one.
//
//	Options.MaxRetries      → COMMAND retries, above the pool. -1 disables them.
//	Options.DialerRetries   → POOL dial retries, inside ConnPool.dialConn.
//	                          Defaults to 5 attempts with a constant
//	                          DialerRetryTimeout (100ms) backoff between them.
//
// Every client here already set MaxRetries: -1 and DialTimeout: 100ms, so this
// looked bounded. It was not: connection-refused is instant, so the dial itself
// cost nothing, but the pool still slept 4 × 100ms between its 5 attempts.
// Measured: 402ms per command, uniformly, across all 11 outage tests (~4.4s of
// package runtime) — and it is where the CI log's
// "connection pool: failed to dial after 5 attempts" came from.
//
// DialerRetries: 1 removes the retry loop entirely (one attempt, no backoff).
// Measured after: 0.15–0.5ms per command. The client still fails, still fails
// with a real net.OpError, and every assertion in the outage tests is unchanged
// — only the sleeping is gone.
//
// TRAP — the two layers do NOT share the "-1 disables it" convention, even
// though they sit two lines apart below:
//
//	MaxRetries:    -1  → disabled. Honoured as "no retries".
//	DialerRetries: -1  → NOT disabled. pool.go:663 reads
//	                     `if maxRetries <= 0 { maxRetries = 5 }`, so -1 and 0
//	                     both fall back to the FULL default of 5 attempts.
//
// Copying the `-1` idiom from the line above therefore leaves pool dial retries
// fully active while looking like it switched them off — with no error and no
// compile warning, just the 402ms back. Use 1 (one attempt), never -1 or 0.
func dialFailingRedis(t *testing.T) *redis.Client {
	t.Helper()
	c := redis.NewClient(&redis.Options{
		// Port 1 is reserved (tcpmux) and never bound in a test sandbox, so
		// the OS refuses the connection immediately.
		Addr:        "127.0.0.1:1",
		DialTimeout: 100 * time.Millisecond,
		ReadTimeout: 100 * time.Millisecond,
		// No command-level retries (upper layer).
		MaxRetries: -1,
		// No pool-level dial retries (lower layer) — see the comment above.
		DialerRetries: 1,
	})
	t.Cleanup(func() { _ = c.Close() })
	return c
}
