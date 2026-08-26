// Package scim implements the SCIM 2.0 (RFC 7643/7644) provisioning API.
package scim

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"mime"
	"net/http"
	"strings"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/labstack/echo/v4"
	"github.com/rs/zerolog/log"

	"github.com/matharnica/vakt/internal/shared/safego"
)

// MediaTypeSCIMJSON is the media type RFC 7644 §3.1 prescribes for SCIM
// requests and responses.
const MediaTypeSCIMJSON = "application/scim+json"

// SCIMMediaTypeMiddleware makes the SCIM group speak its own media type.
//
// R1-07-B07-2: Echo's DefaultBinder switches on the Content-Type and knows only
// application/json, so c.Bind failed on every write route that carried
// application/scim+json and the handler answered 400 invalidValue. Entra ID and
// Okta send exactly that type, which made the documented SCIM provisioning
// unusable with the two most common identity providers. Measured live on 6 of 6
// write routes.
//
// application/scim+json (with any parameters, e.g. charset) is rewritten to
// application/json for the binder. Only that one type — anything else is left
// alone, so a form-encoded body still fails to bind rather than being read as
// SCIM.
//
// The RESPONSE media type is deliberately NOT changed here, although §3.1 asks
// for scim+json in both directions: openapi.yaml documents these responses as
// application/json, and that file belongs to another track (it has to be
// regenerated with npm run api-types in the same commit). Sending a type the
// spec document does not name would trade a measured defect for a contract
// drift. The request side is what was broken — Entra ID and Okta read a JSON
// response either way. Reported as a follow-up.
func SCIMMediaTypeMiddleware(next echo.HandlerFunc) echo.HandlerFunc {
	return func(c echo.Context) error {
		req := c.Request()
		if mediaType, params, err := mime.ParseMediaType(req.Header.Get(echo.HeaderContentType)); err == nil &&
			mediaType == MediaTypeSCIMJSON {
			rewritten := echo.MIMEApplicationJSON
			if charset := params["charset"]; charset != "" {
				rewritten += "; charset=" + charset
			}
			req.Header.Set(echo.HeaderContentType, rewritten)
		}

		return next(c)
	}
}

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
			// context.WithoutCancel detaches from the request lifecycle so the
			// DB write survives after the response is sent, while still inheriting
			// the trace context for observability.
			safego.Run(context.WithoutCancel(c.Request().Context()), "scim.update_last_used", func(ctx context.Context) error {
				// orgid-lint: global — scoped by token_hash, a globally-unique auth credential
				_, err := db.Exec(ctx,
					`UPDATE scim_tokens SET last_used_at = NOW() WHERE token_hash = $1`,
					tokenHash,
				)
				return err
			})

			c.Set("scim_org_id", orgID)
			return next(c)
		}
	}
}

// scimError returns a minimal SCIM-compatible error response.
func scimError(c echo.Context, status int, scimType, detail string) error {
	return c.JSON(status, scimErrorResponse{
		Schemas:  []string{"urn:ietf:params:scim:api:messages:2.0:Error"},
		Status:   status,
		ScimType: scimType,
		Detail:   detail,
	})
}

type scimErrorResponse struct {
	Schemas  []string `json:"schemas"`
	Status   int      `json:"status"`
	ScimType string   `json:"scimType,omitempty"`
	Detail   string   `json:"detail,omitempty"`
}
