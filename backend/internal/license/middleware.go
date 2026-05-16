// Copyright (c) 2026 NorvikOps. All rights reserved.
// SPDX-License-Identifier: Elastic-2.0

package license

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/labstack/echo/v4"
	"github.com/redis/go-redis/v9"
	"github.com/rs/zerolog/log"
)

const licenceCacheTTL = 60 * time.Second

// Require returns an Echo middleware that rejects requests when the active
// license does not include the given feature. The license must have been
// placed on the Echo context under the key "license" by a prior middleware.
func Require(feature string) echo.MiddlewareFunc {
	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			lic, _ := c.Get("license").(*License)
			if lic == nil || !lic.Has(feature) {
				return c.JSON(http.StatusPaymentRequired, map[string]string{
					"error":   "feature_not_available",
					"message": "This feature requires Vakt Pro. Visit https://norvikops.de/vakt for details.",
					"feature": feature,
				})
			}
			return next(c)
		}
	}
}

// licenseCache is a JSON-serialisable snapshot used for Redis caching.
type licenseCache struct {
	Tier      string     `json:"tier"`
	Features  []string   `json:"features"`
	OrgName   string     `json:"org_name"`
	IssuedAt  time.Time  `json:"issued_at"`
	ExpiresAt *time.Time `json:"expires_at,omitempty"`
	Demo      bool       `json:"demo"`
	Community bool       `json:"community"` // true → downgraded; skip DB key lookup
	Revoked   bool       `json:"revoked"`   // true → org is in ls_revoked_subscriptions
}

func licenseToCache(l *License, community bool) licenseCache {
	return licenseCache{
		Tier:      l.Tier,
		Features:  l.Features,
		OrgName:   l.OrgName,
		IssuedAt:  l.IssuedAt,
		ExpiresAt: l.ExpiresAt,
		Demo:      l.Demo,
		Community: community,
		Revoked:   l.Revoked,
	}
}

func cacheToLicense(c licenseCache) *License {
	return &License{
		Tier:      c.Tier,
		Features:  c.Features,
		OrgName:   c.OrgName,
		IssuedAt:  c.IssuedAt,
		ExpiresAt: c.ExpiresAt,
		Demo:      c.Demo,
		Revoked:   c.Revoked,
	}
}

func licenseCacheKey(orgID string) string {
	return fmt.Sprintf("license:%s", orgID)
}

// InvalidateLicenseCache removes the cached license for the given org from Redis.
// Call this after activating or revoking a license key so the next request
// re-reads from the database rather than serving a stale cached result.
func InvalidateLicenseCache(ctx context.Context, rdb *redis.Client, orgID string) {
	if rdb == nil || orgID == "" {
		return
	}
	if err := rdb.Del(ctx, licenseCacheKey(orgID)).Err(); err != nil {
		log.Warn().Err(err).Str("org_id", orgID).Msg("license: failed to invalidate Redis cache")
	}
}

// DBMiddleware returns an Echo middleware that:
//  1. Checks whether the org's subscription has been revoked (cancelled/refunded).
//  2. Loads the per-org license key from the database (if one was activated via the API).
//
// If rdb is non-nil the result is cached in Redis for licenceCacheTTL (60 s) to avoid
// 2 DB round-trips on every authenticated request.
//
// If a DB key is found and is valid it overwrites the static license on the context.
// If the org is in the revocation blocklist the license is downgraded to Community.
// The middleware is a no-op when org_id is not present on the context (e.g. public routes).
func DBMiddleware(db *pgxpool.Pool, staticLic *License, rdb ...*redis.Client) echo.MiddlewareFunc {
	var redisClient *redis.Client
	if len(rdb) > 0 {
		redisClient = rdb[0]
	}

	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			orgID, _ := c.Get("org_id").(string)
			if orgID == "" || db == nil {
				// No org context or no DB — use the static license already set.
				return next(c)
			}

			ctx := c.Request().Context()

			// --- Redis cache lookup ---
			if redisClient != nil {
				cached, err := redisClient.Get(ctx, licenseCacheKey(orgID)).Result()
				if err == nil && cached != "" {
					var lc licenseCache
					if jsonErr := json.Unmarshal([]byte(cached), &lc); jsonErr == nil {
						if lc.Community {
							comm := communityLicense()
							comm.Revoked = lc.Revoked
							c.Set("license", comm)
						} else {
							c.Set("license", cacheToLicense(lc))
						}
						return next(c)
					}
				}
			}

			// --- DB queries (cache miss) ---

			// Check revocation blocklist first.
			var revokedCount int
			_ = db.QueryRow(ctx,
				`SELECT COUNT(*) FROM ls_revoked_subscriptions WHERE org_id = $1::uuid`,
				orgID,
			).Scan(&revokedCount)
			if revokedCount > 0 {
				log.Debug().Str("org_id", orgID).Msg("license: org is in revocation blocklist — downgrading to community")
				comm := communityLicense()
				comm.Revoked = true
				c.Set("license", comm)
				if redisClient != nil {
					lc := licenseToCache(comm, true)
					if b, err := json.Marshal(lc); err == nil {
						_ = redisClient.Set(ctx, licenseCacheKey(orgID), b, licenceCacheTTL).Err()
					}
				}
				return next(c)
			}

			// Check for a DB-persisted license key (activated via /api/v1/license/activate).
			var keyValue string
			err := db.QueryRow(ctx,
				`SELECT key_value FROM license_keys WHERE org_id = $1::uuid`,
				orgID,
			).Scan(&keyValue)
			if err == nil && keyValue != "" {
				lic, parseErr := parse(keyValue)
				if parseErr == nil {
					c.Set("license", lic)
					if redisClient != nil {
						lc := licenseToCache(lic, false)
						if b, marshalErr := json.Marshal(lc); marshalErr == nil {
							_ = redisClient.Set(ctx, licenseCacheKey(orgID), b, licenceCacheTTL).Err()
						}
					}
					return next(c)
				}
				log.Warn().Err(parseErr).Str("org_id", orgID).
					Msg("license: DB key is invalid — falling back to static license")
			}

			// Fall back to the static license loaded at startup (env var or community).
			if staticLic != nil {
				c.Set("license", staticLic)
			}
			return next(c)
		}
	}
}
