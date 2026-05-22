// Copyright (c) 2026 NorvikOps. All rights reserved.
// SPDX-License-Identifier: Elastic-2.0

package middleware

import (
	"context"
	"net"
	"net/http"
	"strings"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/labstack/echo/v4"
	"github.com/rs/zerolog/log"
)

// OrgIPAllowlist returns middleware that enforces a per-org IP allowlist.
// The allowlist is loaded from organizations.admin_ip_allowlist (comma-separated CIDRs).
// When the column is NULL or empty, all IPs are allowed.
// This middleware must run AFTER auth middleware (requires org_id in context).
func OrgIPAllowlist(db *pgxpool.Pool) echo.MiddlewareFunc {
	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			orgID, _ := c.Get("org_id").(string)
			if orgID == "" {
				return next(c)
			}

			raw := loadOrgIPAllowlist(c.Request().Context(), db, orgID)
			if raw == "" {
				return next(c)
			}

			var nets []*net.IPNet
			for _, entry := range strings.Split(raw, ",") {
				entry = strings.TrimSpace(entry)
				if entry == "" {
					continue
				}
				if !strings.Contains(entry, "/") {
					entry += "/32"
				}
				_, ipNet, err := net.ParseCIDR(entry)
				if err != nil {
					log.Warn().Str("cidr", entry).Str("org_id", orgID).Msg("org_ip_allowlist: invalid CIDR, skipping")
					continue
				}
				nets = append(nets, ipNet)
			}
			if len(nets) == 0 {
				return next(c)
			}

			clientIP := net.ParseIP(c.RealIP())
			if clientIP == nil {
				log.Warn().Str("ip", c.RealIP()).Str("org_id", orgID).Msg("org_ip_allowlist: unparseable client IP")
				return c.JSON(http.StatusForbidden, map[string]string{"error": "forbidden", "code": "IP_NOT_ALLOWED"})
			}
			for _, n := range nets {
				if n.Contains(clientIP) {
					return next(c)
				}
			}
			log.Warn().Str("ip", c.RealIP()).Str("org_id", orgID).Msg("org_ip_allowlist: IP not in org allowlist")
			return c.JSON(http.StatusForbidden, map[string]string{"error": "forbidden", "code": "IP_NOT_ALLOWED"})
		}
	}
}

func loadOrgIPAllowlist(ctx context.Context, db *pgxpool.Pool, orgID string) string {
	if db == nil {
		return ""
	}
	var raw *string
	_ = db.QueryRow(ctx,
		`SELECT admin_ip_allowlist FROM organizations WHERE id = $1::uuid`, orgID,
	).Scan(&raw)
	if raw == nil {
		return ""
	}
	return *raw
}
