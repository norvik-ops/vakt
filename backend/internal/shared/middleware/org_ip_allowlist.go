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
// OrgIPAllowlist enforces the org-configured admin IP allowlist (the DB column is
// admin_ip_allowlist). It is deliberately scoped to routes under the /admin prefix
// (S131-C2/R-H17): the setting lives under /admin/org/ip-allowlist and restricts the
// admin surface, matching the env-based IPAllowlist already on those routes. Mounting
// it once on the `protected` group (rather than on each of the fragmented /admin
// sub-groups) covers every current and future /admin-PREFIX route without a
// variant-miss, while the path guard keeps the blast radius off the rest of the API.
//
// KNOWN SCOPE GAP (consistent with the existing env IPAllowlist, which has the same
// gap): a handful of Admin-ROLE-gated routes live on non-/admin prefixes —
// /trust-center/* (admin config/certs/policy publish) and /integrations/* — and are
// therefore NOT IP-restricted here. Closing that fully would need role-based rather
// than path-based scoping; deferred deliberately, not overlooked.
//
// Empty allowlist = pass, so an org that never configures one is unaffected.
func OrgIPAllowlist(db *pgxpool.Pool) echo.MiddlewareFunc {
	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			p := c.Path()
			if !strings.Contains(p, "/admin/") && !strings.HasSuffix(p, "/admin") {
				return next(c)
			}
			orgID, _ := c.Get("org_id").(string)
			if orgID == "" {
				return next(c)
			}

			raw := loadOrgIPAllowlist(c.Request().Context(), db, orgID)
			if raw == "" {
				return next(c)
			}

			nets := ParseAllowlistCIDRs(raw)
			if len(nets) == 0 {
				// Fail-OPEN on an empty/unparseable-at-runtime allowlist. This is a
				// conscious choice (P3): the save-time validation in the admin handler
				// already rejects invalid CIDRs, so a stored allowlist that yields zero
				// nets means it was legitimately empty. Failing closed here would lock
				// every admin out of an org on a transient parse/DB blip — worse than
				// briefly not enforcing an opt-in restriction.
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

// NormalizeCIDR turns a bare IP into a single-host CIDR: /32 for IPv4, /128 for IPv6.
// A bare IPv6 previously got /32, which is a /32 of 128 bits — ~7.9e28 addresses, so a
// single-host entry silently allowed a huge range (R-H17 review). An entry that already
// carries a mask is returned unchanged.
func NormalizeCIDR(entry string) string {
	entry = strings.TrimSpace(entry)
	if entry == "" || strings.Contains(entry, "/") {
		return entry
	}
	if strings.Contains(entry, ":") {
		return entry + "/128" // IPv6 single host
	}
	return entry + "/32" // IPv4 single host
}

// ParseAllowlistCIDRs parses a comma-separated allowlist into IPNets, skipping empty and
// unparseable entries. Shared by the enforcing middleware and the save-time validator so
// both apply identical normalisation (incl. the IPv6 /128 fix).
func ParseAllowlistCIDRs(raw string) []*net.IPNet {
	var nets []*net.IPNet
	for _, entry := range strings.Split(raw, ",") {
		entry = NormalizeCIDR(entry)
		if entry == "" {
			continue
		}
		if _, ipNet, err := net.ParseCIDR(entry); err == nil {
			nets = append(nets, ipNet)
		}
	}
	return nets
}

func loadOrgIPAllowlist(ctx context.Context, db *pgxpool.Pool, orgID string) string {
	if db == nil {
		return ""
	}
	var raw *string
	if err := db.QueryRow(ctx,
		`SELECT admin_ip_allowlist FROM organizations WHERE id = $1::uuid`, orgID,
	).Scan(&raw); err != nil {
		log.Warn().Err(err).Str("org_id", orgID).Msg("org_ip_allowlist: could not load allowlist — skipping check")
	}
	if raw == nil {
		return ""
	}
	return *raw
}
