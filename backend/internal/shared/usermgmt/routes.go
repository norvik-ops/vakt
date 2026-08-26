package usermgmt

import (
	"net/http"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/labstack/echo/v4"

	"github.com/matharnica/vakt/internal/auth"
)

// RegisterRoutes wires the user-management routes into Echo.
//
// adminGroup must already be behind PasetoMiddleware / AuthMiddleware.
// An additional admin-only check is applied here: the caller must hold the Admin
// role in org_members for the organisation their token is scoped to.
//
// publicGroup must NOT be behind any authentication middleware.
//
// mfaSensitive enforces a TOTP step-up on write routes (role change, delete,
// reset-mfa) when the org opted into require_mfa_sensitive_calls (S131-R-H24).
// It skips safe methods, so the admin user-list GET is unaffected.
func RegisterRoutes(adminGroup *echo.Group, publicGroup *echo.Group, svc *Service, db *pgxpool.Pool, mfaSensitive echo.MiddlewareFunc) {
	h := newHandler(svc)

	// Admin routes — require the caller to be an admin.
	admin := adminGroup.Group("", requireAdmin(db), mfaSensitive)
	admin.GET("/users", h.ListUsers)
	admin.PATCH("/users/:id/role", h.UpdateUserRole)
	admin.DELETE("/users/:id", h.RemoveUser)
	// S131-R-H23: MFA break-glass — admin clears a locked-out member's TOTP + recovery codes.
	admin.POST("/users/:id/reset-mfa", h.ResetUserMFA)
	admin.GET("/invitations", h.ListInvitations)
	admin.POST("/invitations", h.CreateInvitation)
	admin.DELETE("/invitations/:id", h.RevokeInvitation)

	// Public routes — no auth required.
	publicGroup.GET("/info", h.GetInvitationInfo)
	publicGroup.POST("/accept", h.AcceptInvitation)
}

// requireAdmin is middleware that checks whether the authenticated caller holds
// the Admin role in org_members for the org their token is scoped to. user_id and
// org_id must already be set in context by the upstream auth middleware.
//
// It reads org_members.role_id -> roles.name, NOT the users.role cache, and that
// is the whole point (REV-ESK13 §2.1/§2.2): ensureNotLastAdmin counts org_members,
// and a guard that protects one column while the gate authorises over another is
// only correct as long as the two agree. They do not agree on existing instances
// — every role change before ESK-13 wrote the cache alone — and each direction of
// that drift used to produce a defect of its own:
//
//   - org_members=Admin, users.role='viewer': the member carries ["Admin"] in
//     their token and passes auth.RequireRole("Admin") on every other module, but
//     was rejected here. With the last-admin guard counting org_members, demoting
//     the org's other admin was permitted and left NOBODY able to assign roles —
//     measured, 0 remaining admins. Repairing that needed an UPDATE against the
//     database, which is the self-help ESK-13 exists to abolish.
//   - org_members=Viewer, users.role='admin': the remainder of every promotion
//     made before ESK-13. It passed here while carrying Viewer claims everywhere
//     else — and since the role change now writes org_members, it would have let
//     that member mint themselves the Admin claims the operator had withheld.
//     That is the D24-1 escalation shape (migration 249) through a second door.
//
// ADR-0074/ADR-0077 already name org_members the source of truth and users.role a
// cache; migration 249 demoted the cache on that basis. This makes the last reader
// of the cache follow the same rule — after it, users.role authorises nothing at
// all (`grep -rn "FROM users" backend --include=*.go | grep role`: no other site).
// Migration 253 cleans up the cache values the drift left behind.
//
// We also fetch and store the caller's email so that CreateInvitation can use
// it as the "invited_by" value without an extra DB round-trip in the handler.
func requireAdmin(db *pgxpool.Pool) echo.MiddlewareFunc {
	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			userID, _ := c.Get("user_id").(string)
			if userID == "" {
				return c.JSON(http.StatusUnauthorized, map[string]string{
					"error": "unauthorized",
					"code":  "AUTH_MISSING_TOKEN",
				})
			}
			orgID, _ := c.Get("org_id").(string)
			if orgID == "" {
				// Fail closed: an unscoped token cannot be an admin OF anything.
				// Not a 401 — the caller is authenticated, just not in an org.
				return c.JSON(http.StatusForbidden, auth.InsufficientRoleResponse{
					Error:         "forbidden: requires role Admin",
					Code:          "AUTH_INSUFFICIENT_ROLE",
					RequiredRoles: []string{"Admin"},
				})
			}

			// LEFT JOIN, so a user who exists but is not a member of this org gets
			// the 403 below rather than the "user not found" 401 — the membership
			// answer belongs to the role check, not to authentication.
			var role, email string
			err := db.QueryRow(c.Request().Context(), `
				SELECT COALESCE(r.name, ''), u.email
				FROM users u
				LEFT JOIN org_members om ON om.user_id = u.id AND om.org_id = $2::uuid
				LEFT JOIN roles r ON r.id = om.role_id
				WHERE u.id = $1::uuid`, userID, orgID,
			).Scan(&role, &email)
			if err != nil {
				return c.JSON(http.StatusUnauthorized, map[string]string{
					"error": "user not found",
					"code":  "AUTH_INVALID_TOKEN",
				})
			}

			if role != "Admin" {
				// Same type as auth.RequireRole's 403, so the two guards cannot
				// drift into two shapes: name the role that is missing, in prose
				// and machine-readable. "forbidden" alone leaves the operator
				// guessing (ESK-13).
				return c.JSON(http.StatusForbidden, auth.InsufficientRoleResponse{
					Error:         "forbidden: requires role Admin",
					Code:          "AUTH_INSUFFICIENT_ROLE",
					RequiredRoles: []string{"Admin"},
				})
			}

			// Make email available to handlers.
			c.Set("user_email", email)
			return next(c)
		}
	}
}
