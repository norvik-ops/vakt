// Copyright (c) 2026 NorvikOps. All rights reserved.
// SPDX-License-Identifier: Elastic-2.0

package auth

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
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

			c.Set("user_id", claims.UserID)
			c.Set("org_id", claims.OrgID)
			c.Set("roles", claims.Roles)
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

			// API key path.
			if strings.HasPrefix(tokenStr, "sk_") {
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

			c.Set("user_id", claims.UserID)
			c.Set("org_id", claims.OrgID)
			c.Set("roles", claims.Roles)
			return next(c)
		}
	}
}

// scopePathPrefixes maps an API key scope to the URL path prefixes it is
// authorised to access. A scope of "admin" grants full access.
var scopePathPrefixes = map[string][]string{
	"secvault":  {"/api/v1/secvault/"},
	"secpulse":  {"/api/v1/secpulse/"},
	"secvitals": {"/api/v1/secvitals/"},
	"secreflex": {"/api/v1/secreflex/"},
	"secprivacy": {"/api/v1/secprivacy/"},
}

// handleAPIKey looks up the raw API key in the database by its SHA-256 hash,
// enforces scope-based path restrictions, then populates echo.Context with
// identity data if access is permitted.
func handleAPIKey(c echo.Context, next echo.HandlerFunc, db *pgxpool.Pool, rawKey string) error {
	sum := sha256.Sum256([]byte(rawKey))
	keyHash := hex.EncodeToString(sum[:])

	const query = `
		SELECT ak.org_id, ak.created_by, ak.scopes
		FROM api_keys ak
		WHERE ak.key_hash = $1
		  AND ak.revoked_at IS NULL
		  AND (ak.expires_at IS NULL OR ak.expires_at > NOW())`

	var orgID, createdBy string
	var scopes []string
	err := db.QueryRow(c.Request().Context(), query, keyHash).Scan(&orgID, &createdBy, &scopes)
	if err != nil {
		log.Debug().Err(err).Msg("api key lookup failed")
		return c.JSON(http.StatusUnauthorized, map[string]string{
			"error": "invalid api key",
			"code":  "AUTH_INVALID_TOKEN",
		})
	}

	// Reject keys with no scopes — no default grant.
	if len(scopes) == 0 {
		log.Debug().Str("org_id", orgID).Msg("api key has empty scopes, denying access")
		return c.JSON(http.StatusForbidden, map[string]string{
			"error": "forbidden: api key has no scopes",
			"code":  "AUTH_INSUFFICIENT_SCOPE",
		})
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
