// Copyright (c) 2026 NorvikOps. All rights reserved.
// SPDX-License-Identifier: Elastic-2.0

package middleware

import (
	"fmt"
	"net/http"
	"time"

	"github.com/labstack/echo/v4"
	"github.com/redis/go-redis/v9"
)

// IPRateLimitRedis returns a per-IP Redis-backed rate limiter as an Echo middleware.
//
// It replaces Echo's in-memory middleware.NewRateLimiterMemoryStore for cases
// where multi-replica deployments share rate-limit state via Redis.
//
// Parameters:
//   - rdb: Redis client. When nil, behaviour depends on failClosed.
//   - keyPrefix: namespaces the Redis key (e.g. "nis2", "setup", "auditor").
//   - limit: maximum requests per window from a single IP.
//   - window: rolling time window over which limit is enforced.
//   - failClosed: when true, a Redis error or nil client returns 503 instead
//     of passing the request through. Use for public, abuse-sensitive endpoints
//     where Vakt already requires Redis to be running (SEC-M08).
//
// Auth-path rate limiters (DemoStartRateLimiter, AuthRateLimit) use their own
// fail-open policy — they must not lock users out during Redis outages.
func IPRateLimitRedis(rdb *redis.Client, keyPrefix string, limit int64, window time.Duration, failClosed bool) echo.MiddlewareFunc {
	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			if rdb == nil {
				if failClosed {
					return c.JSON(http.StatusServiceUnavailable, map[string]string{
						"error": "rate limiter unavailable",
						"code":  "RATE_LIMITER_UNAVAILABLE",
					})
				}
				return next(c)
			}
			key := "rate:" + keyPrefix + ":" + c.RealIP()
			count, err := incrWithTTL(c.Request().Context(), rdb, key, window)
			if err != nil {
				if failClosed {
					return c.JSON(http.StatusServiceUnavailable, map[string]string{
						"error": "rate limiter unavailable",
						"code":  "RATE_LIMITER_UNAVAILABLE",
					})
				}
				// Redis unavailable — fail open (auth-path default).
				return next(c)
			}
			if count > limit {
				c.Response().Header().Set("Retry-After", fmt.Sprintf("%d", int(window.Seconds())))
				return c.JSON(http.StatusTooManyRequests, map[string]string{
					"error": "rate limit exceeded",
					"code":  "RATE_LIMIT_EXCEEDED",
				})
			}
			return next(c)
		}
	}
}
