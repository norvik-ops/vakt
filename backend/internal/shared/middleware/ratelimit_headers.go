// Copyright (c) 2026 NorvikOps. All rights reserved.
// SPDX-License-Identifier: Elastic-2.0

// Package middleware provides shared Echo middleware utilities used across all
// Vakt modules.
package middleware

import (
	"math"
	"net/http"
	"strconv"
	"sync"
	"time"

	"github.com/labstack/echo/v4"
	"golang.org/x/time/rate"
)

// headerStore is a rate-limiter store that wraps golang.org/x/time/rate.Limiter
// and exposes the remaining token count so that WrapWithHeaders can write
// X-RateLimit-* response headers on every request.
type headerStore struct {
	mu        sync.Mutex
	visitors  map[string]*headerVisitor
	rateLimit rate.Limit
	burst     int
	expiresIn time.Duration
	lastClean time.Time
}

type headerVisitor struct {
	lim      *rate.Limiter
	lastSeen time.Time
}

// newHeaderStore constructs a headerStore for a given rate (req/s) and burst.
func newHeaderStore(r rate.Limit, burst int, expiresIn time.Duration) *headerStore {
	if burst <= 0 {
		burst = int(math.Max(1, math.Ceil(float64(r))))
	}
	if expiresIn <= 0 {
		expiresIn = 3 * time.Minute
	}
	return &headerStore{
		visitors:  make(map[string]*headerVisitor),
		rateLimit: r,
		burst:     burst,
		expiresIn: expiresIn,
		lastClean: time.Now(),
	}
}

// Allow implements middleware.RateLimiterStore.
func (s *headerStore) Allow(identifier string) (bool, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	now := time.Now()
	v, ok := s.visitors[identifier]
	if !ok {
		v = &headerVisitor{lim: rate.NewLimiter(s.rateLimit, s.burst)}
		s.visitors[identifier] = v
	}
	v.lastSeen = now

	if now.Sub(s.lastClean) > s.expiresIn {
		for id, vis := range s.visitors {
			if now.Sub(vis.lastSeen) > s.expiresIn {
				delete(s.visitors, id)
			}
		}
		s.lastClean = now
	}

	return v.lim.Allow(), nil
}

// tokens returns the current available token count for an identifier (rounded
// down). Returns burst if the identifier has never been seen.
func (s *headerStore) tokens(identifier string) int {
	s.mu.Lock()
	defer s.mu.Unlock()
	v, ok := s.visitors[identifier]
	if !ok {
		return s.burst
	}
	t := v.lim.Tokens()
	if t < 0 {
		return 0
	}
	return int(math.Floor(t))
}

// resetAt returns the Unix timestamp at which the limiter will next reach burst
// capacity.  For the purposes of the X-RateLimit-Reset header this is
// approximated as "now + time-to-refill-one-token" when tokens > 0, or
// "now + time-to-fill-all-tokens" when the bucket is empty.
func (s *headerStore) resetAt(identifier string) int64 {
	s.mu.Lock()
	defer s.mu.Unlock()
	v, ok := s.visitors[identifier]
	if !ok {
		return time.Now().Unix()
	}
	tokens := v.lim.Tokens()
	var waitSeconds float64
	if tokens < 1 {
		// time until we have 1 token = (1-tokens) / rate
		waitSeconds = (1 - tokens) / float64(s.rateLimit)
	}
	return time.Now().Add(time.Duration(waitSeconds * float64(time.Second))).Unix()
}

// WrapWithHeaders wraps an existing Echo rate-limiter middleware and adds
// X-RateLimit-Limit, X-RateLimit-Remaining, and X-RateLimit-Reset headers to
// every response from the wrapped handler.
//
// limit is the human-readable request ceiling that will be reported in the
// X-RateLimit-Limit header (e.g. 10 for "10 req/min").
//
// The underlying store is created from rate and burst parameters that must
// match those passed to the inner limiter so that the token counts reported in
// headers are consistent with what the limiter actually enforces.  Use
// NewRateLimitedGroup for the common case where you want both limiting and
// header injection with a single consistent configuration.
func WrapWithHeaders(limiter echo.MiddlewareFunc, store *headerStore, limit int) echo.MiddlewareFunc {
	return func(next echo.HandlerFunc) echo.HandlerFunc {
		// Compose: headers-writer wraps the existing limiter which wraps next.
		withHeaders := func(c echo.Context) error {
			identifier := c.RealIP()

			remaining := store.tokens(identifier)
			reset := store.resetAt(identifier)

			c.Response().Header().Set("X-RateLimit-Limit", strconv.Itoa(limit))
			c.Response().Header().Set("X-RateLimit-Remaining", strconv.Itoa(remaining))
			c.Response().Header().Set("X-RateLimit-Reset", strconv.FormatInt(reset, 10))

			return next(c)
		}
		return limiter(withHeaders)
	}
}

// RateLimitedGroup is a convenience bundle that carries both the Echo
// middleware (suitable for passing to echo Group/route methods) and the
// underlying store (needed by WrapWithHeaders).
type RateLimitedGroup struct {
	Middleware echo.MiddlewareFunc
	store      *headerStore
	limit      int
}

// NewRateLimitedGroup creates a rate-limiter middleware together with a
// matching headerStore, and returns them wrapped so that every response
// automatically gets X-RateLimit-* headers.
//
//   - reqPerMinute — the integer limit reported in X-RateLimit-Limit and used
//     as the burst value.
//   - expiresIn    — how long an idle visitor entry is kept (e.g. 5*time.Minute).
func NewRateLimitedGroup(reqPerMinute int, expiresIn time.Duration) *RateLimitedGroup {
	r := rate.Limit(float64(reqPerMinute) / 60.0)
	store := newHeaderStore(r, reqPerMinute, expiresIn)

	echoLimiter := echoRateLimiterFromStore(store)
	wrapped := WrapWithHeaders(echoLimiter, store, reqPerMinute)

	return &RateLimitedGroup{
		Middleware: wrapped,
		store:      store,
		limit:      reqPerMinute,
	}
}

// echoRateLimiterFromStore builds an Echo rate-limiter middleware that uses the
// given headerStore as its backing store.  The deny handler emits the standard
// Vakt JSON error shape and still sets X-RateLimit-* headers.
func echoRateLimiterFromStore(store *headerStore) echo.MiddlewareFunc {
	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			identifier := c.RealIP()
			allowed, err := store.Allow(identifier)
			if err != nil {
				return c.JSON(http.StatusInternalServerError, map[string]string{
					"error": "rate limiter error",
					"code":  "RATE_LIMITER_ERROR",
				})
			}
			if !allowed {
				remaining := store.tokens(identifier)
				reset := store.resetAt(identifier)
				c.Response().Header().Set("X-RateLimit-Limit", strconv.Itoa(store.burst))
				c.Response().Header().Set("X-RateLimit-Remaining", strconv.Itoa(remaining))
				c.Response().Header().Set("X-RateLimit-Reset", strconv.FormatInt(reset, 10))
				return c.JSON(http.StatusTooManyRequests, map[string]string{
					"error": "rate limit exceeded",
					"code":  "RATE_LIMIT_EXCEEDED",
				})
			}
			return next(c)
		}
	}
}
