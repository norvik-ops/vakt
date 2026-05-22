// Package scim implements the SCIM 2.0 (RFC 7643/7644) provisioning API.
package scim

import (
	"crypto/sha256"
	"encoding/hex"
	"net/http"
	"strings"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/labstack/echo/v4"
	"github.com/rs/zerolog/log"
)

// SCIMAuthMiddleware validates the Bearer token from the Authorization header
// against the scim_tokens table.  On success it sets "scim_org_id" in the
// Echo context so downstream handlers can retrieve the org.
func SCIMAuthMiddleware(db *pgxpool.Pool) echo.MiddlewareFunc {
	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			raw := c.Request().Header.Get("Authorization")
			if !strings.HasPrefix(raw, "Bearer ") {
				return scimError(c, http.StatusUnauthorized, "unauthorized", "Bearer token required")
			}
			token := strings.TrimPrefix(raw, "Bearer ")
			if token == "" {
				return scimError(c, http.StatusUnauthorized, "unauthorized", "Bearer token required")
			}

			// Hash the incoming token to compare against the stored digest.
			sum := sha256.Sum256([]byte(token))
			tokenHash := hex.EncodeToString(sum[:])

			var orgID string
			err := db.QueryRow(c.Request().Context(),
				`SELECT org_id::text FROM scim_tokens
				  WHERE token_hash = $1 AND revoked_at IS NULL`,
				tokenHash,
			).Scan(&orgID)
			if err != nil {
				log.Warn().Str("remote_ip", c.RealIP()).Msg("scim: invalid or revoked token")
				return scimError(c, http.StatusUnauthorized, "unauthorized", "Invalid or revoked SCIM token")
			}

			// Update last_used_at asynchronously — failures are non-fatal.
			go func() {
				if _, err := db.Exec(c.Request().Context(),
					`UPDATE scim_tokens SET last_used_at = NOW() WHERE token_hash = $1`,
					tokenHash,
				); err != nil {
					log.Warn().Err(err).Msg("scim: failed to update last_used_at")
				}
			}()

			c.Set("scim_org_id", orgID)
			return next(c)
		}
	}
}

// scimError returns a minimal SCIM-compatible error response.
func scimError(c echo.Context, status int, scimType, detail string) error {
	return c.JSON(status, scimErrorResponse{
		Schemas: []string{"urn:ietf:params:scim:api:messages:2.0:Error"},
		Status:  status,
		ScimType: scimType,
		Detail:  detail,
	})
}

type scimErrorResponse struct {
	Schemas  []string `json:"schemas"`
	Status   int      `json:"status"`
	ScimType string   `json:"scimType,omitempty"`
	Detail   string   `json:"detail,omitempty"`
}
