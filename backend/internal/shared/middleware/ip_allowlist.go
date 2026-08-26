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

// IsAdminPath reports whether a registered route path belongs to the admin
// surface. It is the single definition of that scope, shared by the env-based
// AdminIPAllowlist and the per-org OrgIPAllowlist — two guards that enforce the
// same restriction from two sources and must therefore cover the same routes.
// They used to carry two independent copies of this condition, which is exactly
// how their coverage drifted apart (Codeaudit v5b, R1-10-V01: env 48/59 vs. org
// 59/59).
//
// The argument is echo's *registered* path (c.Path(), e.g.
// "/api/v1/admin/users/:id/role"), not the raw request URL, so a request cannot
// dodge the guard with encoding tricks or a traversal segment.
func IsAdminPath(p string) bool {
	return strings.Contains(p, "/admin/") || strings.HasSuffix(p, "/admin")
}

// AdminIPAllowlist is IPAllowlist scoped to the admin surface, meant to be
// mounted ONCE on the top-level protected group instead of on each /admin
// sub-group.
//
// Why it exists: IPAllowlist() itself is unconditional, so it can only be
// mounted where every route below it is an admin route. That forced one mount
// per /admin sub-group — and there were four of them, of which two carried the
// guard. Rollenwechsel, MFA-Break-Glass and the admin password-reset-token
// generator sat in the two that did not. A guard belongs at the topmost point
// under which its invariant holds, not on each sub-group; the path predicate is
// what makes that single mount possible (same construction as OrgIPAllowlist).
//
// Non-admin routes are passed through untouched, so mounting this on the whole
// protected tree does not restrict the rest of the API.
//
// KNOWN SCOPE GAP, identical to and deliberately consistent with OrgIPAllowlist
// — and it is not a rounding error. Measured against the real setupEcho() tree
// on 2026-07-29 by probing every GET route outside /admin with a real token:
// 80 of them deny a Viewer on role grounds, and 45 still deny a SecurityAnalyst,
// i.e. are genuinely Admin-only. They sit on /integrations (40), /auditor (2),
// /vaktcomply, /vaktvault and /settings (1 each). Write routes were not counted,
// so the real number of admin-privileged routes outside this guard is higher.
//
// The /vaktcomply entry is GET /vaktcomply/approvals, and it is easy to "correct"
// away by mistake: vaktcomply/routes.go registers it with NO route middleware, so
// grepping that file for RequireRole("Admin") suggests no admin-only GET can exist
// under the prefix. The check lives in the handler body instead
// (ListPendingApprovals -> isOrgAdmin -> 403 "admin role required"); verified by
// request, where Viewer and SecurityAnalyst get 403 and Admin gets 200. Counting
// route middleware alone undercounts this gap — probe it, do not grep it.
//
// Neither allowlist covers any of them. "Both allowlists cover the same routes"
// is a statement about the two mechanisms relative to each other, NOT a claim
// that the admin-privileged surface is IP-restricted — it is not.
//
// Closing the gap needs role-based rather than path-based scoping, which is a
// product decision (an operator who allowlists an office network would suddenly
// lock analysts out of module pages): deferred, not overlooked.
//
// If you re-measure, expect the rate limiters to eat your denominator, because
// they are mounted BEFORE the route-level role check and turn a 403 into a 429:
//   - the per-org limit (300/min) needs a FRESH ORG every few dozen probes;
//     without that the count swings between 49 and 80 and looks like noise;
//   - the auth/TOTP limiters are keyed by IP, not org, so re-seeding the org does
//     not help — httptest sends 192.0.2.1 for every request unless you set
//     RemoteAddr, and a run that only re-seeds the org still loses ~20 probes.
//
// Count the 429s and require ZERO of them before believing the result. Every
// number above was produced that way and reproduced three times identically; a
// measurement with a non-zero 429 count is a lower bound, not a value.
func AdminIPAllowlist() echo.MiddlewareFunc {
	// IPAllowlist() reads the environment once, at construction time. Building it
	// here (not per request) keeps that property and the single startup log line.
	guard := IPAllowlist()
	return func(next echo.HandlerFunc) echo.HandlerFunc {
		guarded := guard(next)
		return func(c echo.Context) error {
			if !IsAdminPath(c.Path()) {
				return next(c)
			}
			return guarded(c)
		}
	}
}

// IPAllowlist returns middleware that restricts access to the given CIDR ranges.
// If VAKT_ADMIN_ALLOWED_IPS is empty, all IPs are allowed (default open).
// The env var accepts comma-separated CIDRs or IPs, e.g. "10.0.0.0/8,192.168.1.0/24".
//
// This variant is unconditional: it restricts EVERY route below its mount point.
// For the admin surface use AdminIPAllowlist, which carries the path scope and is
// mounted once at the top.
func IPAllowlist() echo.MiddlewareFunc {
	raw := strings.TrimSpace(os.Getenv("VAKT_ADMIN_ALLOWED_IPS"))
	if raw == "" {
		return func(next echo.HandlerFunc) echo.HandlerFunc {
			return next
		}
	}

	// The NORMALISATION is shared with the org allowlist (NormalizeCIDR); the
	// LOOP around it deliberately is not, even though ParseAllowlistCIDRs does
	// almost the same thing. The difference is the log line below: an operator
	// who typos a CIDR in the environment gets a warning at startup and can act
	// on it, whereas the shared parser drops bad entries silently — which is the
	// right behaviour there (the org list is validated at save time by
	// admin/handler_org.go, so a bad entry cannot get in) and the wrong one here
	// (nothing validates an env var). Keep them apart on purpose; if you ever
	// merge them, the warning has to survive.
	var nets []*net.IPNet
	for _, entry := range strings.Split(raw, ",") {
		// NormalizeCIDR turns a bare address into a single host: /32 for IPv4,
		// /128 for IPv6. This used to append /32 unconditionally, which for an
		// IPv6 address is not an error and not a warning — it just silently
		// widens one intended host to 2^96 addresses. Both allowlists now parse
		// through the same helper; a second copy of this rule is how the two of
		// them drifted apart before.
		entry = NormalizeCIDR(entry)
		if entry == "" {
			continue
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
