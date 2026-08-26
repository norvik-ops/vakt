package scim

import (
	"net"
	"net/http"
	"os"
	"strings"

	"github.com/labstack/echo/v4"
	"github.com/rs/zerolog/log"

	sharedmw "github.com/matharnica/vakt/internal/shared/middleware"
)

// IPAllowlist restricts the SCIM endpoints to the CIDRs in VAKT_SCIM_ALLOWED_IPS.
// Empty or unset means no restriction, which is the default.
//
// R1-F3W2A-04: the full SCIM user and group lifecycle — PATCH and DELETE
// /Users are deprovisioning and role assignment — sits outside BOTH IP
// allowlists. PATCH /admin/users/:id/role has the same effect and lies behind
// two of them; a leaked SCIM token works today from any address on earth.
//
// Why its OWN environment variable and not the admin list:
//
//   - The caller is an IdP push, not a browser. The office network and the
//     identity provider's data centre are near-disjoint address sets, so
//     reusing VAKT_ADMIN_ALLOWED_IPS would force an operator to widen the admin
//     surface to let SCIM in — the opposite of what either list is for.
//   - Pulling /scim/v2 under IsAdminPath would be wrong for a second reason:
//     that predicate is a path prefix today and therefore completely checkable
//     against the route tree. Turning it into an enumeration of privileged
//     areas rebuilds the drifting list that produced the original defect.
//
// Opt-in on purpose: a self-hosted instance whose IdP has no fixed egress
// addresses must not lock itself out of provisioning on upgrade.
func IPAllowlist() echo.MiddlewareFunc {
	raw := strings.TrimSpace(os.Getenv("VAKT_SCIM_ALLOWED_IPS"))
	if raw == "" {
		return func(next echo.HandlerFunc) echo.HandlerFunc { return next }
	}

	// NormalizeCIDR is shared with the two existing allowlists: a bare address
	// becomes /32 for IPv4 and /128 for IPv6. Appending /32 unconditionally is
	// how one intended IPv6 host silently became 2^96 of them.
	var nets []*net.IPNet
	for _, entry := range strings.Split(raw, ",") {
		entry = sharedmw.NormalizeCIDR(entry)
		if entry == "" {
			continue
		}
		_, ipNet, err := net.ParseCIDR(entry)
		if err != nil {
			log.Warn().Str("entry", entry).Msg("scim_ip_allowlist: invalid CIDR, skipping")
			continue
		}
		nets = append(nets, ipNet)
	}

	// Nothing parsed but something was configured: the operator meant to
	// restrict and every entry was a typo. Denying is the only reading of that
	// which is not "the guard silently did nothing".
	if len(nets) == 0 {
		log.Error().Msg("scim_ip_allowlist: VAKT_SCIM_ALLOWED_IPS is set but no entry parsed — SCIM is closed")
	} else {
		log.Info().Int("cidrs", len(nets)).Msg("scim_ip_allowlist: SCIM endpoint restriction active")
	}

	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			clientIP := net.ParseIP(c.RealIP())
			for _, n := range nets {
				if clientIP != nil && n.Contains(clientIP) {
					return next(c)
				}
			}
			log.Warn().Str("ip", c.RealIP()).Msg("scim_ip_allowlist: blocked request to SCIM endpoint")
			return c.JSON(http.StatusForbidden, map[string]string{
				"error": "forbidden",
				"code":  "IP_NOT_ALLOWED",
			})
		}
	}
}
