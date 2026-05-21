package audit

import (
	"net/http"
	"strconv"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/labstack/echo/v4"
)

// RegisterRoutes registers the GET /audit-log endpoint on the supplied Echo group.
// The group must already be protected by auth middleware so that org_id is available.
//
// Supported query params:
//
//	limit      int     — number of entries to return (default 100, max 500)
//	offset     int     — number of entries to skip (default 0)
//	from       string  — RFC3339 timestamp; filter created_at >= from
//	to         string  — RFC3339 timestamp; filter created_at <= to
//	user_email string  — case-insensitive substring match on user_email
//	action     string  — exact match on action field
func RegisterRoutes(g *echo.Group, db *pgxpool.Pool) {
	g.GET("", func(c echo.Context) error {
		orgID, _ := c.Get("org_id").(string)
		if orgID == "" {
			return c.JSON(http.StatusUnauthorized, map[string]string{
				"error": "unauthorized",
				"code":  "AUTH_MISSING_ORG",
			})
		}

		var filters ListFilters

		// limit
		filters.Limit = 100
		if raw := c.QueryParam("limit"); raw != "" {
			if n, err := strconv.Atoi(raw); err == nil && n > 0 {
				filters.Limit = n
			}
		}

		// offset
		if raw := c.QueryParam("offset"); raw != "" {
			if n, err := strconv.Atoi(raw); err == nil && n >= 0 {
				filters.Offset = n
			}
		}

		// from (RFC3339)
		if raw := c.QueryParam("from"); raw != "" {
			if t, err := time.Parse(time.RFC3339, raw); err == nil {
				filters.From = &t
			}
		}

		// to (RFC3339)
		if raw := c.QueryParam("to"); raw != "" {
			if t, err := time.Parse(time.RFC3339, raw); err == nil {
				filters.To = &t
			}
		}

		// user_email (substring, case-insensitive)
		if raw := c.QueryParam("user_email"); raw != "" {
			filters.UserEmail = raw
		}

		// action (exact match)
		if raw := c.QueryParam("action"); raw != "" {
			filters.Action = raw
		}

		result, err := List(c.Request().Context(), db, orgID, filters)
		if err != nil {
			return c.JSON(http.StatusInternalServerError, map[string]string{
				"error": "failed to fetch audit log",
				"code":  "AUDIT_LOG_FETCH_FAILED",
			})
		}

		return c.JSON(http.StatusOK, result)
	})
}
