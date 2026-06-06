// Copyright (c) 2026 NorvikOps. All rights reserved.
// SPDX-License-Identifier: Elastic-2.0

package middleware

import (
	"net/http"
	"time"

	"github.com/labstack/echo/v4"
	"github.com/redis/go-redis/v9"
)

const (
	// demoRLLimit is the maximum number of demo-start requests allowed per IP
	// per demoRLWindow. 10 req/5 min is generous for legitimate browser use
	// (multiple tabs, refreshes) while still protecting the DB against flood.
	demoRLLimit = 10
	// demoRLWindow is the rolling window over which demoRLLimit is applied.
	demoRLWindow = 5 * time.Minute
)

// DemoStartRateLimiter returns an Echo middleware that limits POST /demo/start
// requests per IP using Redis INCR with TTL — correct for multi-replica
// deployments where an in-memory store would be bypassed by simply hitting a
// different replica.
//
// Behaviour on Redis unavailability: fail open (don't block legitimate users).
// The demo/start endpoint is public and non-critical; availability is preferred
// over strict enforcement during Redis outages.
func DemoStartRateLimiter(rdb *redis.Client) echo.MiddlewareFunc {
	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			if rdb == nil {
				// No Redis client configured — fail open.
				return next(c)
			}

			ip := c.RealIP()
			key := "rate:demo_start:" + ip
			ctx := c.Request().Context()

			count, err := incrWithTTL(ctx, rdb, key, demoRLWindow)
			if err != nil {
				// Redis unavailable — fail open.
				return next(c)
			}

			if count > demoRLLimit {
				return c.JSON(http.StatusTooManyRequests, map[string]string{
					"message": "rate limit exceeded",
				})
			}

			return next(c)
		}
	}
}
