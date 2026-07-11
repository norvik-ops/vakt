// Copyright (c) 2026 NorvikOps. All rights reserved.
// SPDX-License-Identifier: Elastic-2.0

package metrics

import (
	"net"
	"net/http"
	"strings"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/labstack/echo/v4"
)

// metricsTokenAuth enforces Bearer token auth when VAKT_METRICS_TOKEN is set.
// Falls through to the IP allowlist if no token is configured.
func metricsTokenAuth(token string, next echo.HandlerFunc) echo.HandlerFunc {
	return func(c echo.Context) error {
		if token == "" {
			return next(c)
		}
		auth := c.Request().Header.Get("Authorization")
		if strings.HasPrefix(auth, "Bearer ") && strings.TrimPrefix(auth, "Bearer ") == token {
			return next(c)
		}
		return c.JSON(http.StatusUnauthorized, map[string]string{
			"error": "unauthorized",
			"code":  "METRICS_TOKEN_REQUIRED",
		})
	}
}

// cidr172bridge is the Docker bridge range 172.16.0.0/12, parsed once at init.
var cidr172bridge = func() *net.IPNet {
	_, n, _ := net.ParseCIDR("172.16.0.0/12")
	return n
}()

// cidr10private is the RFC 1918 10.0.0.0/8 range, parsed once at init.
var cidr10private = func() *net.IPNet {
	_, n, _ := net.ParseCIDR("10.0.0.0/8")
	return n
}()

// metricsIPAllowlist restricts /metrics access to localhost and Docker-internal
// network ranges. All other clients receive 403.
func metricsIPAllowlist(next echo.HandlerFunc) echo.HandlerFunc {
	return func(c echo.Context) error {
		ip := c.RealIP()
		if isAllowedMetricsIP(ip) {
			return next(c)
		}
		return c.JSON(http.StatusForbidden, map[string]string{
			"error": "forbidden",
			"code":  "METRICS_ACCESS_DENIED",
		})
	}
}

// isAllowedMetricsIP returns true for loopback (127.x / ::1), the Docker bridge
// range (172.16.0.0/12 — previously accepted any 172.x), and RFC1918 10.x.
func isAllowedMetricsIP(raw string) bool {
	// Strip IPv6-mapped IPv4 prefix if present.
	raw = strings.TrimPrefix(raw, "::ffff:")

	parsed := net.ParseIP(raw)
	if parsed == nil {
		return false
	}

	if parsed.IsLoopback() {
		return true
	}
	if cidr172bridge != nil && cidr172bridge.Contains(parsed) {
		return true
	}
	if cidr10private != nil && cidr10private.Contains(parsed) {
		return true
	}
	return false
}

// RegisterOptions carries optional configuration for the metrics endpoint.
type RegisterOptions struct {
	// RedisAddr is the host:port of the Redis server used for queue-depth metrics.
	// Leave empty to skip queue-depth metrics.
	RedisAddr string
	// RedisPassword is the Redis AUTH password. Required when Redis runs with
	// --requirepass (the shipped compose default), otherwise queue-depth and
	// Asynq metrics fail silently with NOAUTH (S121-C3 / I1).
	RedisPassword string
	// MetricsToken is the Bearer token required to access /metrics.
	// When empty, only the IP allowlist applies.
	MetricsToken string
}

// S121-F2 (T6): removed the deprecated Register(e, db) — it had no callers
// (cmd/api uses RegisterWithOptions). Use RegisterWithOptions directly.

// RegisterWithOptions mounts the /metrics endpoint with extended options.
func RegisterWithOptions(e *echo.Echo, db *pgxpool.Pool, opts RegisterOptions) {
	h := NewHandler(db)
	if opts.RedisAddr != "" {
		h.WithRedisAddr(opts.RedisAddr)
		h.WithRedisPassword(opts.RedisPassword)
	}
	tokenMiddleware := func(next echo.HandlerFunc) echo.HandlerFunc {
		return metricsTokenAuth(opts.MetricsToken, next)
	}
	e.GET("/metrics", h.ServeMetrics, metricsIPAllowlist, tokenMiddleware)
}
