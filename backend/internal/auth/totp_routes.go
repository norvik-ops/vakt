package auth

import (
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/labstack/echo/v4"
)

// RegisterTOTP mounts the 2FA/TOTP endpoints onto the given Echo group.
// All endpoints require a valid auth token (authMiddleware).
// An optional rateLimiter middleware can be passed to rate-limit TOTP verification attempts.
//
// Routes registered (relative to g):
//
//	GET  /2fa/status                      — check if 2FA is enabled for the current user
//	POST /2fa/setup                       — begin TOTP setup, returns secret + QR URI
//	POST /2fa/confirm                     — confirm setup with first code, returns backup + recovery codes
//	POST /2fa/disable                     — disable 2FA (requires valid code)
//	POST /2fa/verify                      — verify a code or backup code (second-factor step)
//	POST /2fa/recovery                    — use a recovery code to obtain a new token pair
//	POST /2fa/recovery-codes/regenerate   — invalidate existing recovery codes and issue 8 new ones
func RegisterTOTP(g *echo.Group, db *pgxpool.Pool, masterKey []byte, authMiddleware echo.MiddlewareFunc, svc *Service, rateLimiter ...echo.MiddlewareFunc) {
	h := NewTotpHandler(db, masterKey, svc)

	middlewares := []echo.MiddlewareFunc{authMiddleware}
	if len(rateLimiter) > 0 && rateLimiter[0] != nil {
		middlewares = append(middlewares, rateLimiter[0])
	}

	// S124-1: /2fa/login-verify is the PUBLIC second stage of the two-stage login
	// — the caller holds only an mfa_pending token, no session. It is NOT behind
	// authMiddleware, but IS rate-limited (it is a TOTP brute-force surface).
	var publicMW []echo.MiddlewareFunc
	if len(rateLimiter) > 0 && rateLimiter[0] != nil {
		publicMW = append(publicMW, rateLimiter[0])
	}
	g.POST("/2fa/login-verify", h.LoginVerify, publicMW...)

	twoFA := g.Group("/2fa", middlewares...)
	twoFA.GET("/status", h.Status)
	twoFA.POST("/setup", h.Setup)
	twoFA.POST("/confirm", h.Confirm)
	twoFA.POST("/disable", h.Disable)
	twoFA.POST("/verify", h.Verify)
	twoFA.POST("/recovery", h.RecoveryLogin)
	twoFA.POST("/recovery-codes/regenerate", h.RegenerateRecoveryCodes)
}
