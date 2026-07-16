// Copyright (c) 2026 NorvikOps. All rights reserved.
// SPDX-License-Identifier: Elastic-2.0

package license

import (
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/labstack/echo/v4"
	"github.com/redis/go-redis/v9"
	"github.com/rs/zerolog/log"
)

type licenseResponse struct {
	Tier               string     `json:"tier"`
	IsPro              bool       `json:"is_pro"`
	Features           []string   `json:"features"`
	OrgName            string     `json:"org_name"`
	ExpiresAt          *time.Time `json:"expires_at"`
	Demo               bool       `json:"demo"`
	AutoRenewalEnabled bool       `json:"auto_renewal_enabled"`
}

// Handler serves the /api/v1/license endpoint.
// mu guards h.lic against concurrent reads and writes (e.g. Activate updating
// h.lic while Get is reading it in another goroutine).
type Handler struct {
	mu                 sync.RWMutex
	lic                *License
	db                 *pgxpool.Pool
	rdb                *redis.Client
	autoRenewalEnabled bool
}

// NewHandler creates a Handler bound to the given License.
func NewHandler(lic *License) *Handler {
	return &Handler{lic: lic}
}

// WithDB attaches a database pool so that Activate can persist keys.
func (h *Handler) WithDB(db *pgxpool.Pool) *Handler {
	h.db = db
	return h
}

// WithRedis attaches a Redis client so that Activate can invalidate the license cache.
func (h *Handler) WithRedis(rdb *redis.Client) *Handler {
	h.rdb = rdb
	return h
}

// WithAutoRenewal marks the handler as having auto-renewal enabled so the
// license response includes auto_renewal_enabled: true.
func (h *Handler) WithAutoRenewal() *Handler {
	h.autoRenewalEnabled = true
	return h
}

// Get returns the current license state.
// It prefers the per-request license set by DBMiddleware (which carries the
// org-specific DB key) and falls back to the in-memory static license.
func (h *Handler) Get(c echo.Context) error {
	// DBMiddleware places the org-specific license on the Echo context; use that
	// when available so that per-org overrides are reflected.
	lic, _ := c.Get("license").(*License)
	if lic == nil {
		h.mu.RLock()
		lic = h.lic
		h.mu.RUnlock()
	}
	if lic == nil {
		lic = communityLicense()
	}
	features := lic.Features
	if features == nil {
		features = []string{}
	}
	return c.JSON(http.StatusOK, licenseResponse{
		Tier:               lic.Tier,
		IsPro:              lic.IsPro(),
		Features:           features,
		OrgName:            lic.OrgName,
		ExpiresAt:          lic.ExpiresAt,
		Demo:               lic.Demo,
		AutoRenewalEnabled: h.autoRenewalEnabled,
	})
}

type activateRequest struct {
	Key string `json:"key"`
}

// Activate handles POST /api/v1/license/activate.
// It validates the provided key, persists it in the database, and returns the
// resulting license info. The in-memory license is updated immediately so that
// subsequent requests within the same process see the new tier.
func (h *Handler) Activate(c echo.Context) error {
	var req activateRequest
	if err := c.Bind(&req); err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{
			"error": "invalid request body",
			"code":  "LICENSE_BAD_REQUEST",
		})
	}
	if req.Key == "" {
		return c.JSON(http.StatusBadRequest, map[string]string{
			"error": "key is required",
			"code":  "LICENSE_KEY_MISSING",
		})
	}

	lic, err := parse(req.Key)
	if err != nil {
		log.Warn().Err(err).Msg("license: activation rejected — invalid key")
		errMsg := err.Error()
		switch {
		case strings.Contains(errMsg, "invalid license key format"):
			errMsg = "Ungültiges Lizenzschlüssel-Format"
		case strings.Contains(errMsg, "signature verification failed"):
			errMsg = "Lizenzschlüssel konnte nicht verifiziert werden"
		case strings.Contains(errMsg, "license has expired"):
			errMsg = "Lizenzschlüssel ist abgelaufen"
		case strings.Contains(errMsg, "org_id mismatch"):
			errMsg = "Lizenzschlüssel gehört nicht zu dieser Organisation"
		default:
			errMsg = "Lizenzschlüssel ungültig"
		}
		return c.JSON(http.StatusUnprocessableEntity, map[string]string{"error": errMsg, "code": "LICENSE_INVALID_KEY"})
	}

	// Persist key in DB so it survives restarts (loaded before env-var fallback).
	if h.db != nil {
		orgID, _ := c.Get("org_id").(string)

		// Fix: if org_id is absent from the context, the INSERT would silently use an
		// empty string and the key would never match any real org in DBMiddleware.
		// Reject early with a clear error instead of persisting a dead record.
		if orgID == "" {
			return c.JSON(http.StatusUnauthorized, map[string]string{
				"error": "organization context required",
				"code":  "LICENSE_NO_ORG",
			})
		}

		userID, _ := c.Get("user_id").(string)

		_, dbErr := h.db.Exec(c.Request().Context(),
			`INSERT INTO license_keys (org_id, key_value, activated_at, activated_by)
			 VALUES ($1::uuid, $2, NOW(), $3::uuid)
			 ON CONFLICT (org_id) DO UPDATE SET key_value = $2, activated_at = NOW(), activated_by = $3::uuid`,
			orgID, req.Key, userID,
		)
		if dbErr != nil {
			log.Error().Err(dbErr).Msg("license: failed to persist key in DB")
			return c.JSON(http.StatusInternalServerError, map[string]string{
				"error": "could not persist license key",
				"code":  "LICENSE_PERSIST_FAILED",
			})
		}
	}

	// Update the in-memory license so subsequent requests (within this process) reflect the new tier.
	// Lock for write to prevent a data race with concurrent Get calls.
	h.mu.Lock()
	h.lic = lic
	h.mu.Unlock()

	// Invalidate the Redis cache so DBMiddleware re-reads from the database on
	// the next request rather than serving a stale Community license for up to 60 s.
	orgID, _ := c.Get("org_id").(string)
	InvalidateLicenseCache(c.Request().Context(), h.rdb, orgID)

	features := lic.Features
	if features == nil {
		features = []string{}
	}

	log.Info().
		Str("tier", lic.Tier).
		Str("org", lic.OrgName).
		Msg("license: Pro key activated successfully")

	return c.JSON(http.StatusOK, licenseResponse{
		Tier:               lic.Tier,
		IsPro:              lic.IsPro(),
		Features:           features,
		OrgName:            lic.OrgName,
		ExpiresAt:          lic.ExpiresAt,
		Demo:               lic.Demo,
		AutoRenewalEnabled: h.autoRenewalEnabled,
	})
}
