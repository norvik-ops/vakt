// Copyright (c) 2026 NorvikOps. All rights reserved.
// SPDX-License-Identifier: Elastic-2.0

package middleware

import (
	"context"
	"fmt"
	"math"
	"net/http"
	"strconv"
	"sync"
	"time"

	"github.com/labstack/echo/v4"
	"github.com/redis/go-redis/v9"
	"github.com/rs/zerolog/log"
	"golang.org/x/time/rate"
)

const (
	// orgRateLimitPerMinute is the default per-org request cap (300 req/min).
	orgRateLimitPerMinute = 300
	// orgRateLimitExpiresIn controls how long an idle org entry is kept in the store.
	orgRateLimitExpiresIn = 5 * time.Minute
)

// orgVisitor holds the per-org token-bucket limiter.
type orgVisitor struct {
	lim      *rate.Limiter
	lastSeen time.Time
}

// orgRateLimitStore is an in-memory store keyed by org_id.
type orgRateLimitStore struct {
	mu        sync.Mutex
	visitors  map[string]*orgVisitor
	rateLimit rate.Limit
	burst     int
	expiresIn time.Duration
	lastClean time.Time
}

func newOrgRateLimitStore(reqPerMinute int, expiresIn time.Duration) *orgRateLimitStore {
	r := rate.Limit(float64(reqPerMinute) / 60.0)
	burst := reqPerMinute
	if burst <= 0 {
		burst = int(math.Max(1, math.Ceil(float64(r))))
	}
	return &orgRateLimitStore{
		visitors:  make(map[string]*orgVisitor),
		rateLimit: r,
		burst:     burst,
		expiresIn: expiresIn,
		lastClean: time.Now(),
	}
}

// allow checks whether the given org_id is within its rate limit.
// Returns (allowed, remaining, resetUnix).
func (s *orgRateLimitStore) allow(orgID string) (bool, int, int64) {
	s.mu.Lock()
	defer s.mu.Unlock()

	now := time.Now()
	v, ok := s.visitors[orgID]
	if !ok {
		v = &orgVisitor{lim: rate.NewLimiter(s.rateLimit, s.burst)}
		s.visitors[orgID] = v
	}
	v.lastSeen = now

	// Periodic cleanup of stale entries.
	if now.Sub(s.lastClean) > s.expiresIn {
		for id, vis := range s.visitors {
			if now.Sub(vis.lastSeen) > s.expiresIn {
				delete(s.visitors, id)
			}
		}
		s.lastClean = now
	}

	allowed := v.lim.Allow()

	// Calculate remaining tokens after the Allow() call.
	tokens := v.lim.Tokens()
	remaining := int(math.Floor(tokens))
	if remaining < 0 {
		remaining = 0
	}

	// X-RateLimit-Reset: when the bucket will next have ≥1 token.
	var waitSeconds float64
	if tokens < 1 {
		waitSeconds = (1 - tokens) / float64(s.rateLimit)
	}
	reset := now.Add(time.Duration(waitSeconds * float64(time.Second))).Unix()

	return allowed, remaining, reset
}

// OrgRateLimit returns an in-memory per-org_id rate limit middleware.
//
// USE OrgRateLimitRedis FOR PRODUCTION — the in-memory version is only suitable
// for single-replica deployments. With multiple replicas each instance maintains
// its own counter, so the effective limit becomes (300 × replica_count).
//
// Behaviour: 300 req/min token-bucket per org_id, X-RateLimit-* headers on every
// response, HTTP 429 on rejection.
func OrgRateLimit() echo.MiddlewareFunc {
	store := newOrgRateLimitStore(orgRateLimitPerMinute, orgRateLimitExpiresIn)
	limitStr := strconv.Itoa(orgRateLimitPerMinute)

	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			orgID, _ := c.Get("org_id").(string)
			if orgID == "" {
				return next(c)
			}

			allowed, remaining, reset := store.allow(orgID)

			c.Response().Header().Set("X-RateLimit-Limit", limitStr)
			c.Response().Header().Set("X-RateLimit-Remaining", strconv.Itoa(remaining))
			c.Response().Header().Set("X-RateLimit-Reset", strconv.FormatInt(reset, 10))

			if !allowed {
				return c.JSON(http.StatusTooManyRequests, map[string]string{
					"error": "rate limit exceeded",
					"code":  "RATE_LIMIT_EXCEEDED",
				})
			}
			return next(c)
		}
	}
}

// OrgRateLimitRedis returns a multi-replica-safe per-org_id rate limiter backed
// by Redis. Uses a fixed-window counter (INCR + EXPIRE) keyed by org_id and the
// current UTC minute. Simpler than a sliding window but adequate at this scale
// and consistent across replicas.
//
// Falls back to allow-and-log on Redis errors so a transient cache outage does
// not lock customers out of the application (rate limiting is a quality-of-
// service measure, not an auth boundary).
func OrgRateLimitRedis(client *redis.Client) echo.MiddlewareFunc {
	const limit = orgRateLimitPerMinute
	limitStr := strconv.Itoa(limit)

	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			orgID, _ := c.Get("org_id").(string)
			if orgID == "" || client == nil {
				return next(c)
			}

			ctx, cancel := context.WithTimeout(c.Request().Context(), 100*time.Millisecond)
			defer cancel()

			// Fixed-window: bucket = current UTC minute.
			now := time.Now().UTC()
			bucket := now.Format("200601021504")
			key := fmt.Sprintf("vakt:ratelimit:org:%s:%s", orgID, bucket)

			pipe := client.Pipeline()
			incrCmd := pipe.Incr(ctx, key)
			pipe.Expire(ctx, key, 70*time.Second) // 60s window + small buffer
			if _, err := pipe.Exec(ctx); err != nil {
				log.Warn().Err(err).Str("org_id", orgID).Msg("rate limit: redis error, allowing")
				return next(c)
			}
			count := incrCmd.Val()

			// X-RateLimit-Reset: top of the next minute.
			resetUnix := now.Truncate(time.Minute).Add(time.Minute).Unix()
			remaining := limit - int(count)
			if remaining < 0 {
				remaining = 0
			}

			c.Response().Header().Set("X-RateLimit-Limit", limitStr)
			c.Response().Header().Set("X-RateLimit-Remaining", strconv.Itoa(remaining))
			c.Response().Header().Set("X-RateLimit-Reset", strconv.FormatInt(resetUnix, 10))

			if count > int64(limit) {
				retryAfter := resetUnix - now.Unix()
				if retryAfter < 1 {
					retryAfter = 1
				}
				c.Response().Header().Set("Retry-After", strconv.FormatInt(retryAfter, 10))
				return c.JSON(http.StatusTooManyRequests, map[string]string{
					"error": "rate limit exceeded",
					"code":  "RATE_LIMIT_EXCEEDED",
				})
			}
			return next(c)
		}
	}
}
