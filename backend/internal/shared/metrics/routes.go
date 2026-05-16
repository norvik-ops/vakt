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

// Register mounts the /metrics endpoint on the root Echo instance.
// Access is restricted to loopback and Docker-internal network ranges so that
// Prometheus can scrape the endpoint while external traffic is denied.
func Register(e *echo.Echo, db *pgxpool.Pool) {
	h := NewHandler(db)
	e.GET("/metrics", h.ServeMetrics, metricsIPAllowlist)
}
