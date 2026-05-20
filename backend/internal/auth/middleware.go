// Copyright (c) 2026 NorvikOps. All rights reserved.
// SPDX-License-Identifier: Elastic-2.0

package auth

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"net/http"
	"strings"
	"time"

	"aidanwoods.dev/go-paseto"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/labstack/echo/v4"
	"github.com/redis/go-redis/v9"
	"github.com/rs/zerolog/log"
)

// SymmetricKey is an alias for the Paseto v4 symmetric key type so that test
// code and callers outside this package can reference it without importing the
// paseto library directly.
type SymmetricKey = paseto.V4SymmetricKey

// PasetoMiddleware validates a Paseto v4 bearer token and populates echo.Context
// with "user_id", "org_id", and "roles".  It does not handle API keys; use
// AuthMiddleware for the full (DB-backed) authentication chain.
//
// rdb is an optional Redis client used to check the token deny-list populated by
// the logout endpoint. Pass nil (or omit) to skip the deny-list check — this
// should only be done in tests. Production wiring MUST pass a Redis client so
// that logged-out tokens are rejected, matching the behaviour of AuthMiddleware.
func PasetoMiddleware(key paseto.V4SymmetricKey, rdb ...*redis.Client) echo.MiddlewareFunc {
	var redisClient *redis.Client
	if len(rdb) > 0 {
		redisClient = rdb[0]
	}
	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			header := c.Request().Header.Get("Authorization")
			// Fall back to httpOnly cookie if no Authorization header (browser sessions).
			if header == "" {
				if cookie, err := c.Cookie("access_token"); err == nil && cookie.Value != "" {
					header = "Bearer " + cookie.Value
				}
			}
			if header == "" {
				return c.JSON(http.StatusUnauthorized, map[string]string{
					"error": "unauthorized",
					"code":  "AUTH_MISSING_TOKEN",
				})
			}

			tokenStr, ok := bearerToken(header)
			if !ok {
				return c.JSON(http.StatusUnauthorized, map[string]string{
					"error": "invalid authorization header format",
					"code":  "AUTH_MISSING_TOKEN",
				})
			}

			// Check token deny-list (logout revocation).
			if redisClient != nil {
				denyKey := tokenDenyKey(tokenStr)
				ctx, cancel := context.WithTimeout(c.Request().Context(), 2*time.Second)
				exists, err := redisClient.Exists(ctx, denyKey).Result()
				cancel()
				if err != nil {
					log.Warn().Err(err).Msg("token deny-list check skipped — Redis unavailable")
				} else if exists > 0 {
					return c.JSON(http.StatusUnauthorized, map[string]string{
						"error": "token has been revoked",
						"code":  "AUTH_TOKEN_REVOKED",
					})
				}
			}

			claims, err := ParseAccessToken(key, tokenStr)
			if err != nil {
				log.Debug().Err(err).Msg("paseto token validation failed")
				return c.JSON(http.StatusUnauthorized, map[string]string{
					"error": "invalid or expired token",
					"code":  "AUTH_INVALID_TOKEN",
				})
			}

			// Verify pw_version — reject tokens issued before the last password change.
			if redisClient != nil {
				if err := checkPwVersion(c.Request().Context(), redisClient, claims); err != nil {
					return c.JSON(http.StatusUnauthorized, map[string]string{
						"error": "session invalidated — please log in again",
						"code":  "AUTH_SESSION_INVALIDATED",
					})
				}
			}

			c.Set("user_id", claims.UserID)
			c.Set("org_id", claims.OrgID)
			c.Set("roles", claims.Roles)
			c.Set("token_raw", tokenStr)
			return next(c)
		}
	}
}

// AuthMiddleware validates a Paseto bearer token or an API key (prefix "sk_")
// and populates echo.Context with "user_id", "org_id", and "roles".
//
// API key path performs a DB lookup against the api_keys table.
// rdb is used to check the token deny-list populated by the logout endpoint.
func AuthMiddleware(key paseto.V4SymmetricKey, db *pgxpool.Pool, rdb ...*redis.Client) echo.MiddlewareFunc {
	var redisClient *redis.Client
	if len(rdb) > 0 {
		redisClient = rdb[0]
	}
	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			header := c.Request().Header.Get("Authorization")
			// Fall back to httpOnly cookie if no Authorization header (browser sessions).
			if header == "" {
				if cookie, err := c.Cookie("access_token"); err == nil && cookie.Value != "" {
					header = "Bearer " + cookie.Value
				}
			}
			if header == "" {
				return c.JSON(http.StatusUnauthorized, map[string]string{
					"error": "unauthorized",
					"code":  "AUTH_MISSING_TOKEN",
				})
			}

			tokenStr, ok := bearerToken(header)
			if !ok {
				return c.JSON(http.StatusUnauthorized, map[string]string{
					"error": "invalid authorization header format",
					"code":  "AUTH_MISSING_TOKEN",
				})
			}

			// API key path — accept both legacy "sk_" and current "vakt_" prefixes.
			if strings.HasPrefix(tokenStr, "sk_") || strings.HasPrefix(tokenStr, "vakt_") {
				return handleAPIKey(c, next, db, tokenStr)
			}

			// Check token deny-list (logout revocation).
			if redisClient != nil {
				denyKey := tokenDenyKey(tokenStr)
				ctx, cancel := context.WithTimeout(c.Request().Context(), 2*time.Second)
				exists, err := redisClient.Exists(ctx, denyKey).Result()
				cancel()
				if err != nil {
					log.Warn().Err(err).Msg("token deny-list check skipped — Redis unavailable")
				} else if exists > 0 {
					return c.JSON(http.StatusUnauthorized, map[string]string{
						"error": "token has been revoked",
						"code":  "AUTH_TOKEN_REVOKED",
					})
				}
			}

			// Paseto path.
			claims, err := ParseAccessToken(key, tokenStr)
			if err != nil {
				log.Debug().Err(err).Msg("paseto token validation failed")
				return c.JSON(http.StatusUnauthorized, map[string]string{
					"error": "invalid or expired token",
					"code":  "AUTH_INVALID_TOKEN",
				})
			}

			// Verify pw_version — reject tokens issued before the last password change.
			if redisClient != nil {
				if err := checkPwVersion(c.Request().Context(), redisClient, claims); err != nil {
					return c.JSON(http.StatusUnauthorized, map[string]string{
						"error": "session invalidated — please log in again",
						"code":  "AUTH_SESSION_INVALIDATED",
					})
				}
			}

			c.Set("user_id", claims.UserID)
			c.Set("org_id", claims.OrgID)
			c.Set("roles", claims.Roles)
			c.Set("token_raw", tokenStr)
			return next(c)
		}
	}
}

// mfaExemptPaths are paths that must remain accessible even when org-wide MFA
// is required but the user has not yet set up TOTP.  They cover the 2FA setup
// flow, logout, and the health-check endpoint.
var mfaExemptPaths = []string{
	"/api/v1/auth/2fa/setup",
	"/api/v1/auth/2fa/confirm",
	"/api/v1/auth/logout",
	"/api/v1/health",
	"/health",
}

// MFAEnforceMiddleware must be applied after AuthMiddleware (user_id and org_id
// must already be set in the context).  It queries the DB to check whether the
// organisation has require_mfa=true and, if so, verifies that the current user
// has a confirmed TOTP secret (totp_secrets.enabled = true).  If not, it
// returns 403 with code "MFA_REQUIRED".
//
// Routes listed in mfaExemptPaths are always allowed through so that users can
// complete the TOTP setup flow without being locked out.
func MFAEnforceMiddleware(db *pgxpool.Pool) echo.MiddlewareFunc {
	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			// Allow exempted paths regardless of MFA policy.
			reqPath := c.Request().URL.Path
			for _, exempt := range mfaExemptPaths {
				if reqPath == exempt {
					return next(c)
				}
			}

			orgID, _ := c.Get("org_id").(string)
			userID, _ := c.Get("user_id").(string)
			if orgID == "" || userID == "" {
				return next(c)
			}

			ctx := c.Request().Context()

			// Check org-level MFA requirement.
			var requireMFA bool
			err := db.QueryRow(ctx,
				`SELECT require_mfa FROM organizations WHERE id = $1::uuid`, orgID,
			).Scan(&requireMFA)
			if err != nil {
				log.Error().Err(err).Str("org_id", orgID).Msg("mfa enforce: org lookup failed")
				return c.JSON(http.StatusServiceUnavailable, map[string]string{
					"error": "service temporarily unavailable",
					"code":  "SERVICE_UNAVAILABLE",
				})
			}

			if !requireMFA {
				return next(c)
			}

			// Org requires MFA — check if user has enabled TOTP.
			var totpEnabled bool
			err = db.QueryRow(ctx,
				`SELECT enabled FROM totp_secrets WHERE user_id = $1::uuid`, userID,
			).Scan(&totpEnabled)
			if err != nil || !totpEnabled {
				return c.JSON(http.StatusForbidden, map[string]string{
					"error": "MFA erforderlich",
					"code":  "MFA_REQUIRED",
				})
			}

			return next(c)
		}
	}
}

// scopePathPrefixes maps an API key scope to the URL path prefixes it is
// authorised to access. A scope of "admin" grants full access.
var scopePathPrefixes = map[string][]string{
	"secvault":   {"/api/v1/secvault/"},
	"secpulse":   {"/api/v1/secpulse/"},
	"secvitals":  {"/api/v1/secvitals/"},
	"secreflex":  {"/api/v1/secreflex/"},
	"secprivacy": {"/api/v1/secprivacy/"},
}

// handleAPIKey looks up the raw API key in the database by its SHA-256 hash,
// enforces scope-based path restrictions, then populates echo.Context with
// identity data if access is permitted.
//
// Keys with the "vakt_" prefix and empty scopes are treated as full-access
// personal keys (equivalent to the user's own session).
func handleAPIKey(c echo.Context, next echo.HandlerFunc, db *pgxpool.Pool, rawKey string) error {
	sum := sha256.Sum256([]byte(rawKey))
	keyHash := hex.EncodeToString(sum[:])

	const query = `
		SELECT ak.id, ak.org_id, ak.created_by, ak.scopes
		FROM api_keys ak
		WHERE ak.key_hash = $1
		  AND ak.revoked_at IS NULL
		  AND (ak.expires_at IS NULL OR ak.expires_at > NOW())`

	var keyID, orgID, createdBy string
	var scopes []string
	err := db.QueryRow(c.Request().Context(), query, keyHash).Scan(&keyID, &orgID, &createdBy, &scopes)
	if err != nil {
		log.Debug().Err(err).Msg("api key lookup failed")
		return c.JSON(http.StatusUnauthorized, map[string]string{
			"error": "invalid api key",
			"code":  "AUTH_INVALID_TOKEN",
		})
	}

	// Personal "vakt_" keys with empty scopes have full user-level access.
	// Legacy "sk_" keys without scopes are rejected (no default grant).
	isPersonalKey := strings.HasPrefix(rawKey, "vakt_")
	if len(scopes) == 0 && !isPersonalKey {
		log.Debug().Str("org_id", orgID).Msg("api key has empty scopes, denying access")
		return c.JSON(http.StatusForbidden, map[string]string{
			"error": "forbidden: api key has no scopes",
			"code":  "AUTH_INSUFFICIENT_SCOPE",
		})
	}

	// Update last_used_at asynchronously — do not block the request.
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
		defer cancel()
		_, _ = db.Exec(ctx,
			`UPDATE api_keys SET last_used_at = NOW() WHERE id = $1::uuid`,
			keyID,
		)
	}()

	// Personal keys with empty scopes have full user-level access — no path check.
	if isPersonalKey && len(scopes) == 0 {
		c.Set("user_id", createdBy)
		c.Set("org_id", orgID)
		c.Set("roles", []string{"SecurityAnalyst"})
		return next(c)
	}

	// Check whether this key's scopes permit the requested path.
	requestPath := c.Request().URL.Path
	allowed := false
	isAdmin := false
	for _, scope := range scopes {
		if scope == "admin" {
			isAdmin = true
			allowed = true
			break
		}
		if prefixes, ok := scopePathPrefixes[scope]; ok {
			for _, prefix := range prefixes {
				if strings.HasPrefix(requestPath, prefix) {
					allowed = true
					break
				}
			}
		}
		if allowed {
			break
		}
	}

	if !allowed {
		log.Debug().
			Str("org_id", orgID).
			Strs("scopes", scopes).
			Str("path", requestPath).
			Msg("api key scope does not permit this path")
		return c.JSON(http.StatusForbidden, map[string]string{
			"error": "forbidden: api key scope does not permit this path",
			"code":  "AUTH_INSUFFICIENT_SCOPE",
		})
	}

	var roles []string
	if isAdmin {
		roles = []string{"Admin"}
	} else {
		roles = []string{"SecurityAnalyst"}
	}

	c.Set("user_id", createdBy)
	c.Set("org_id", orgID)
	c.Set("roles", roles)
	return next(c)
}

// RequireRole returns middleware that enforces that at least one of the caller's
// roles appears in the allowedRoles list.
//
// Role hierarchy (highest to lowest): Admin > SecurityAnalyst > Viewer > AuditorReadOnly
func RequireRole(allowedRoles ...string) echo.MiddlewareFunc {
	allowed := make(map[string]struct{}, len(allowedRoles))
	for _, r := range allowedRoles {
		allowed[r] = struct{}{}
	}

	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			roles, _ := c.Get("roles").([]string)
			for _, r := range roles {
				if _, ok := allowed[r]; ok {
					return next(c)
				}
			}
			return c.JSON(http.StatusForbidden, map[string]string{
				"error": "forbidden",
				"code":  "AUTH_INSUFFICIENT_ROLE",
			})
		}
	}
}

// checkPwVersion compares the pw_version embedded in the token claims against
// the current value stored in Redis.  Returns a non-nil error if the token is
// stale (i.e. the user changed their password after this token was issued).
// Redis unavailability is treated as a pass-through to avoid locking users out
// during transient Redis downtime.
func checkPwVersion(ctx context.Context, rdb *redis.Client, claims *Claims) error {
	key := pwVersionKey(claims.UserID)
	ctx, cancel := context.WithTimeout(ctx, 2*time.Second)
	defer cancel()

	current, err := rdb.Get(ctx, key).Int64()
	if err != nil {
		if err == redis.Nil {
			// Key doesn't exist — treat current version as 0.
			// Tokens with pw_version == 0 are valid.
			if claims.PwVersion != 0 {
				return fmt.Errorf("pw_version mismatch")
			}
			return nil
		}
		// Redis unavailable — log and allow through.
		log.Warn().Err(err).Str("user_id", claims.UserID).Msg("pw_version check skipped — Redis unavailable")
		return nil
	}

	if claims.PwVersion != current {
		return fmt.Errorf("pw_version mismatch: token=%d current=%d", claims.PwVersion, current)
	}
	return nil
}

// bearerToken extracts the token string from an "Authorization: Bearer <token>" header.
func bearerToken(header string) (string, bool) {
	const prefix = "Bearer "
	if !strings.HasPrefix(header, prefix) {
		return "", false
	}
	tok := strings.TrimSpace(header[len(prefix):])
	if tok == "" {
		return "", false
	}
	return tok, true
}
