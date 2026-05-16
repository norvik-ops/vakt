package auditor

import (
	"net/http"
	"strings"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/labstack/echo/v4"
	"github.com/rs/zerolog/log"
)

// AuditorAuth validates an auditor session token from the Authorization header
// and populates the Echo context with "org_id" and "is_auditor".
//
// This middleware is intentionally separate from the regular user auth chain
// so that auditor sessions cannot be confused with user sessions.
func AuditorAuth(db *pgxpool.Pool) echo.MiddlewareFunc {
	svc := NewService(db)
	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			header := c.Request().Header.Get("Authorization")
			if header == "" {
				return c.JSON(http.StatusUnauthorized, map[string]string{
					"error": "unauthorized",
					"code":  "AUDITOR_MISSING_TOKEN",
				})
			}

			token := strings.TrimPrefix(header, "Bearer ")
			token = strings.TrimSpace(token)
			if token == "" {
				return c.JSON(http.StatusUnauthorized, map[string]string{
					"error": "unauthorized",
					"code":  "AUDITOR_MISSING_TOKEN",
				})
			}

			claims, err := svc.ValidateSession(c.Request().Context(), token)
			if err != nil {
				log.Debug().Err(err).Msg("auditor session validation failed")
				return c.JSON(http.StatusUnauthorized, map[string]string{
					"error": "invalid or expired auditor session",
					"code":  "AUDITOR_INVALID_TOKEN",
				})
			}

			c.Set("org_id", claims.OrgID)
			c.Set("is_auditor", true)
			c.Set("auditor_email", claims.AuditorEmail)
			return next(c)
		}
	}
}
