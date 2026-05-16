package auth

import (
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/labstack/echo/v4"
)

// RegisterTOTP mounts the 2FA/TOTP endpoints onto the given Echo group.
// All endpoints require a valid auth token (authMiddleware).
//
// Routes registered (relative to g):
//
//	GET  /2fa/status   — check if 2FA is enabled for the current user
//	POST /2fa/setup    — begin TOTP setup, returns secret + QR URI
//	POST /2fa/confirm  — confirm setup with first code, returns backup codes
//	POST /2fa/disable  — disable 2FA (requires valid code)
//	POST /2fa/verify   — verify a code or backup code (second-factor step)
func RegisterTOTP(g *echo.Group, db *pgxpool.Pool, masterKey []byte, authMiddleware echo.MiddlewareFunc) {
	h := NewTotpHandler(db, masterKey)

	twoFA := g.Group("/2fa", authMiddleware)
	twoFA.GET("/status", h.Status)
	twoFA.POST("/setup", h.Setup)
	twoFA.POST("/confirm", h.Confirm)
	twoFA.POST("/disable", h.Disable)
	twoFA.POST("/verify", h.Verify)
}
