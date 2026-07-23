package usermgmt

import (
	"net/http"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/labstack/echo/v4"
)

// RegisterRoutes wires the user-management routes into Echo.
//
// adminGroup must already be behind PasetoMiddleware / AuthMiddleware.
// An additional admin-only check is applied here: the caller must have
// role='admin' in the users table.
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
	admin.GET("/invitations", h.ListInvitations)
	admin.POST("/invitations", h.CreateInvitation)
	admin.DELETE("/invitations/:id", h.RevokeInvitation)

	// Public routes — no auth required.
	publicGroup.GET("/info", h.GetInvitationInfo)
	publicGroup.POST("/accept", h.AcceptInvitation)
}

// requireAdmin is middleware that checks whether the authenticated user has
// role='admin' in the users table. The user_id must already be set in context
// by the upstream auth middleware.
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

			var role, email string
			err := db.QueryRow(c.Request().Context(),
				`SELECT role, email FROM users WHERE id = $1::uuid`, userID,
			).Scan(&role, &email)
			if err != nil {
				return c.JSON(http.StatusUnauthorized, map[string]string{
					"error": "user not found",
					"code":  "AUTH_INVALID_TOKEN",
				})
			}

			if role != "admin" {
				return c.JSON(http.StatusForbidden, map[string]string{
					"error": "admin role required",
					"code":  "AUTH_INSUFFICIENT_ROLE",
				})
			}

			// Make email available to handlers.
			c.Set("user_email", email)
			return next(c)
		}
	}
}
