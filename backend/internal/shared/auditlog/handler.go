package auditlog

import (
	"net/http"
	"strconv"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/labstack/echo/v4"
)

// RegisterRoutes registers the GET /audit-log endpoint on the supplied Echo group.
// The group must already be protected by auth middleware so that org_id is available.
func RegisterRoutes(g *echo.Group, db *pgxpool.Pool) {
	g.GET("", func(c echo.Context) error {
		orgID, _ := c.Get("org_id").(string)
		if orgID == "" {
			return c.JSON(http.StatusUnauthorized, map[string]string{
				"error": "unauthorized",
				"code":  "AUTH_MISSING_ORG",
			})
		}

		limit := 50
		if raw := c.QueryParam("limit"); raw != "" {
			if n, err := strconv.Atoi(raw); err == nil && n > 0 {
				limit = n
			}
		}

		entries, err := List(c.Request().Context(), db, orgID, limit)
		if err != nil {
			return c.JSON(http.StatusInternalServerError, map[string]string{
				"error": "failed to fetch audit log",
				"code":  "AUDIT_LOG_FETCH_FAILED",
			})
		}

		if entries == nil {
			entries = []LogEntry{}
		}

		return c.JSON(http.StatusOK, entries)
	})
}
