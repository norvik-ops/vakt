// Copyright (c) 2026 NorvikOps. All rights reserved.
// SPDX-License-Identifier: Elastic-2.0

package middleware

import (
	"net"
	"net/http"
	"os"
	"strings"

	"github.com/labstack/echo/v4"
	"github.com/rs/zerolog/log"
)

// IPAllowlist returns middleware that restricts access to the given CIDR ranges.
// If VAKT_ADMIN_ALLOWED_IPS is empty, all IPs are allowed (default open).
// The env var accepts comma-separated CIDRs or IPs, e.g. "10.0.0.0/8,192.168.1.0/24".
func IPAllowlist() echo.MiddlewareFunc {
	raw := strings.TrimSpace(os.Getenv("VAKT_ADMIN_ALLOWED_IPS"))
	if raw == "" {
		return func(next echo.HandlerFunc) echo.HandlerFunc {
			return next
		}
	}

	var nets []*net.IPNet
	for _, entry := range strings.Split(raw, ",") {
		entry = strings.TrimSpace(entry)
		if entry == "" {
			continue
		}
		// Support plain IPs by appending /32
		if !strings.Contains(entry, "/") {
			entry += "/32"
		}
		_, ipNet, err := net.ParseCIDR(entry)
		if err != nil {
			log.Warn().Str("entry", entry).Msg("ip_allowlist: invalid CIDR, skipping")
			continue
		}
		nets = append(nets, ipNet)
	}

	log.Info().Int("cidrs", len(nets)).Msg("ip_allowlist: admin endpoint restriction active")

	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			clientIP := net.ParseIP(c.RealIP())
			if clientIP == nil {
				return c.JSON(http.StatusForbidden, map[string]string{"error": "forbidden", "code": "IP_NOT_ALLOWED"})
			}
			for _, n := range nets {
				if n.Contains(clientIP) {
					return next(c)
				}
			}
			log.Warn().Str("ip", c.RealIP()).Msg("ip_allowlist: blocked request to admin endpoint")
			return c.JSON(http.StatusForbidden, map[string]string{"error": "forbidden", "code": "IP_NOT_ALLOWED"})
		}
	}
}
