// Copyright (c) 2026 NorvikOps. All rights reserved.
// SPDX-License-Identifier: Elastic-2.0

package middleware

import (
	"context"
	"fmt"
	"net/http"
	"time"

	"github.com/labstack/echo/v4"
	"github.com/redis/go-redis/v9"
)

const (
	// authRLLimit is the maximum number of auth attempts allowed per IP per window.
	// S45-5: reduced to 5 req/min per IP for all credential-submission endpoints.
	authRLLimit = 5
	// authRLWindow is the rolling window over which the limit is applied.
	authRLWindow = time.Minute
)

// AuthRateLimit returns an Echo middleware that enforces an IP-based token-bucket
// rate limit of 5 requests per minute using Redis as the backing store.
//
// On every call the middleware:
//  1. Increments a Redis counter keyed by "auth_rl:<ip>".
//  2. Sets the key TTL to authRLWindow on the first increment.
//  3. Returns 429 with a JSON error body when the counter exceeds authRLLimit.
//
// The Redis-backed approach ensures that rate-limit state survives process
// restarts and is shared across multiple API replicas.
func AuthRateLimit(rdb *redis.Client) echo.MiddlewareFunc {
	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			ip := c.RealIP()
			key := fmt.Sprintf("auth_rl:%s", ip)
			ctx := c.Request().Context()

			count, err := incrWithTTL(ctx, rdb, key, authRLWindow)
			if err != nil {
				// Fail open: if Redis is unavailable we do not block legitimate users.
				return next(c)
			}

			if count > authRLLimit {
				return c.JSON(http.StatusTooManyRequests, map[string]string{
					"error": "Too many attempts",
					"code":  "AUTH_RATE_LIMIT",
				})
			}

			return next(c)
		}
	}
}

const (
	// totpRLLimit is the maximum number of TOTP verification attempts allowed
	// per IP per totpRLWindow.  5 attempts per 5 minutes matches the TOTP code
	// rotation period and prevents brute-force sweeps without blocking genuine
	// retries caused by clock skew.
	totpRLLimit = 5
	// totpRLWindow is the rolling window over which totpRLLimit is applied.
	totpRLWindow = 5 * time.Minute
)

// TOTPRateLimit returns an Echo middleware that enforces an IP-based rate limit
// of 5 TOTP verification attempts per 5 minutes using Redis as the backing store.
//
// Redis-backed storage ensures the limit is shared across replicas and survives
// process restarts — unlike the Echo in-memory rate limiter which resets on
// every restart and is not effective behind a load balancer.
//
// Fails open when Redis is unavailable so that a Redis outage does not lock
// users out of 2FA-protected endpoints.
func TOTPRateLimit(rdb *redis.Client) echo.MiddlewareFunc {
	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			ip := c.RealIP()
			key := fmt.Sprintf("totp_rl:%s", ip)
			ctx := c.Request().Context()

			count, err := incrWithTTL(ctx, rdb, key, totpRLWindow)
			if err != nil {
				// Fail open: if Redis is unavailable we do not block legitimate users.
				return next(c)
			}

			if count > totpRLLimit {
				return c.JSON(http.StatusTooManyRequests, map[string]string{
					"error": "Too many TOTP attempts",
					"code":  "TOTP_RATE_LIMIT",
				})
			}

			return next(c)
		}
	}
}

// incrWithTTL atomically increments key and, on the first increment, sets its
// expiry to ttl.  Returns the new counter value.
func incrWithTTL(ctx context.Context, rdb *redis.Client, key string, ttl time.Duration) (int64, error) {
	pipe := rdb.Pipeline()
	incr := pipe.Incr(ctx, key)
	pipe.Expire(ctx, key, ttl)
	if _, err := pipe.Exec(ctx); err != nil {
		return 0, err
	}
	return incr.Val(), nil
}
