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

// isReadMethod reports whether an HTTP method is a read.
//
// This is the whole definition of "reading" for the expired-license carve-out
// below, and it is deliberately the method and nothing else. Two consequences
// were weighed and accepted:
//
//   - A GET that produces an export (PDF, XLSX, ZIP, CSV) counts as a read and
//     stays open on an expired key. That is the point: 34 of the gated GET
//     routes are exports, and "get your evidence out after the subscription
//     lapsed" is the promise on License.Expired. Excluding them would gut the
//     carve-out exactly where a customer needs it. An export handler may append
//     an audit-log row (see LogBCMReportExport) — recording that someone read
//     their data is a side effect of the read, not a mutation of the record.
//   - A POST that only queries (POST /org/siem/test, POST
//     /org/saml-config/fetch-metadata) stays closed. Both are admin
//     configuration probes, not access to customer data, so blocking them costs
//     the customer nothing that License.Expired promises. Carving them out
//     would mean a per-route list of "writes that are really reads" — a second
//     source of truth next to the method, and this file exists because the last
//     second source of truth drifted.
func isReadMethod(method string) bool {
	return method == http.MethodGet || method == http.MethodHead
}

// Allows is THE decision for "may this request use this feature". Every gate —
// route middleware and handler-level check alike — goes through here.
//
// It used to be two decisions. license.Require knew the expired-license
// carve-out; features.Require, which its own doc called "a thin wrapper around
// license.Require", reimplemented the check and dropped it. 164 of 167 route
// gates used the second one, so the promise on License.Expired ("an expired
// Pro/Enterprise license retains read access […] This prevents data lock-out")
// held on three POST routes and nowhere else, and HasReadOnly had no reachable
// caller. Two functions answering one question is two answers; this is one.
//
// The rules, in order:
//
//	nil license                    → no
//	feature granted and not expired → yes  (Has covers demo and legacy-Pro keys)
//	expired, read method, tier covers it → yes
//	otherwise                      → no
//
// Note what does NOT change: a Community license has Expired == false, so it
// never reaches the carve-out and is gated exactly as before. Same for a valid
// Pro key, which is granted by Has on the second rule. The expired branch is the
// entire blast radius.
func Allows(lic *License, method, feature string) bool {
	if lic == nil {
		return false
	}
	if lic.Has(feature) {
		return true
	}
	return lic.Expired && isReadMethod(method) && lic.HasReadOnly(feature)
}

// deny renders the 402 for a request Allows rejected. An expired key gets a
// different body from an unlicensed one: the customer has paid before and needs
// to be told that renewing restores writes and that the data is still there.
func deny(c echo.Context, lic *License, feature string) error {
	if lic != nil && lic.Expired {
		return c.JSON(http.StatusPaymentRequired, map[string]string{
			"error":   "license_expired",
			"message": "Your Vakt Pro license has expired. Renew at https://vakt.norvikops.de to re-enable write access. Your data is still readable.",
			"feature": feature,
		})
	}
	return c.JSON(http.StatusPaymentRequired, map[string]string{
		"error":   "feature_not_available",
		"message": "This feature requires Vakt Pro. Visit https://vakt.norvikops.de for details.",
		"feature": feature,
	})
}

// Require returns an Echo middleware that rejects requests when the active
// license does not grant the given feature for this request's method. The
// license must have been placed on the Echo context under the key "license" by
// a prior middleware.
//
// For expired Pro/Enterprise licenses: GET and HEAD are allowed (read-only
// access preserved); write methods return 402 with a renewal prompt.
//
// features.Require is this function. Do not add a second one.
func Require(feature string) echo.MiddlewareFunc {
	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			lic, _ := c.Get("license").(*License)
			if !Allows(lic, c.Request().Method, feature) {
				return deny(c, lic, feature)
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
	Expired   bool       `json:"expired"`   // true → key past ExpiresAt; read-only mode
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
		Expired:   l.Expired,
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
		Expired:   c.Expired,
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

// DBMiddleware returns an Echo middleware that loads the per-org license key from
// the database (if one was activated via the API) and puts it on the context.
//
// If rdb is non-nil the result is cached in Redis for licenceCacheTTL (60 s) to avoid
// a DB round-trip on every authenticated request.
//
// If a DB key is found and is valid it overwrites the instance license on the context.
// The middleware is a no-op when org_id is not present on the context (e.g. public routes) —
// those routes are served by the instance license (see Instance).
// It does not check any revocation blocklist — see the comment at the DB-query step.
func DBMiddleware(db *pgxpool.Pool, inst *Instance, rdb ...*redis.Client) echo.MiddlewareFunc {
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
							c.Set("license", communityLicense())
						} else {
							c.Set("license", cacheToLicense(lc))
						}
						return next(c)
					}
				}
			}

			// --- DB queries (cache miss) ---
			//
			// There is deliberately NO revocation-blocklist lookup here. ADR-0052
			// settled that key expiry is the only revocation mechanism for
			// self-hosted instances: Norvik cannot write to a customer's database,
			// so a push-based blocklist was never workable. The old
			// ls_revoked_subscriptions probe outlived its Polar/LemonSqueezy
			// webhook, kept querying a table migration 235 had dropped (SQLSTATE
			// 42P01 on every cache miss), swallowed the error and fell through —
			// i.e. it could never have blocked anyone. Revocation now works by not
			// renewing the key; it lapses within its validity window.

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

			// Fall back to the instance license (env key, adopted DB key, or community).
			if inst != nil {
				c.Set("license", inst.Get())
			}
			return next(c)
		}
	}
}
