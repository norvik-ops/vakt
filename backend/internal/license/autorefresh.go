// Copyright (c) 2026 NorvikOps. All rights reserved.
// SPDX-License-Identifier: Elastic-2.0

package license

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/redis/go-redis/v9"
	"github.com/rs/zerolog/log"
)

const (
	defaultRefreshURL  = "https://api.norvikops.de"
	refreshInterval    = 24 * time.Hour
	refreshHTTPTimeout = 15 * time.Second
)

// AutoRefresher polls the Norvik license endpoint every 24h and silently
// activates the latest key when it differs from the current one.
// It requires VAKT_LICENSE_TOKEN to be set; without it, it is a no-op.
type AutoRefresher struct {
	token   string
	baseURL string
	handler *Handler
	db      *pgxpool.Pool
	rdb     *redis.Client
	// orgID is resolved lazily on the first refresh from the license_keys table.
	// Scopes the DB UPDATE to the single org that activated a key (self-hosted).
	orgID string
}

// NewAutoRefresher creates an AutoRefresher.
// baseURL defaults to https://api.norvikops.de when empty.
func NewAutoRefresher(token, baseURL string, h *Handler, db *pgxpool.Pool, rdb *redis.Client) *AutoRefresher {
	if baseURL == "" {
		baseURL = defaultRefreshURL
	}
	return &AutoRefresher{
		token:   token,
		baseURL: baseURL,
		handler: h,
		db:      db,
		rdb:     rdb,
	}
}

// Start runs the refresh loop until ctx is cancelled.
// Refreshes immediately on start, then every 24h.
func (r *AutoRefresher) Start(ctx context.Context) {
	if r.token == "" {
		return
	}
	log.Info().Str("base_url", r.baseURL).Msg("license: auto-refresh enabled")
	r.tryRefresh(ctx)

	ticker := time.NewTicker(refreshInterval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			r.tryRefresh(ctx)
		}
	}
}

func (r *AutoRefresher) tryRefresh(ctx context.Context) {
	key, err := r.fetchKey(ctx)
	if err != nil {
		log.Warn().Err(err).Msg("license: auto-refresh fetch failed — keeping current license")
		return
	}

	lic, err := parse(key)
	if err != nil {
		log.Warn().Err(err).Msg("license: auto-refresh key invalid — keeping current license")
		return
	}

	// Update in-memory license immediately.
	r.handler.mu.Lock()
	current := r.handler.lic
	if current != nil && current.ExpiresAt != nil && lic.ExpiresAt != nil &&
		!lic.ExpiresAt.After(*current.ExpiresAt) {
		r.handler.mu.Unlock()
		log.Debug().Msg("license: auto-refresh key not newer — no update")
		return
	}
	r.handler.lic = lic
	r.handler.mu.Unlock()

	// Persist to DB so DBMiddleware serves the refreshed key per-org.
	// Resolve the org_id once (lazy) — scopes the UPDATE to the org that
	// activated a license key, preventing unintended cross-org writes.
	if r.db != nil {
		if r.orgID == "" {
			_ = r.db.QueryRow(ctx,
				`SELECT org_id FROM license_keys ORDER BY activated_at DESC LIMIT 1`,
			).Scan(&r.orgID)
		}
		if r.orgID != "" {
			res, dbErr := r.db.Exec(ctx,
				`UPDATE license_keys SET key_value = $1, activated_at = NOW() WHERE org_id = $2::uuid`,
				key, r.orgID,
			)
			if dbErr != nil {
				log.Warn().Err(dbErr).Msg("license: auto-refresh DB persist failed")
			} else if res.RowsAffected() == 0 {
				log.Warn().Str("org_id", r.orgID).Msg("license: auto-refresh DB update matched no rows")
			}
		}
	}

	// Invalidate Redis so the next request re-reads from DB (pattern scan).
	if r.rdb != nil {
		iter := r.rdb.Scan(ctx, 0, "license:*", 0).Iterator()
		for iter.Next(ctx) {
			_ = r.rdb.Del(ctx, iter.Val()).Err()
		}
	}

	log.Info().
		Str("tier", lic.Tier).
		Str("org", lic.OrgName).
		Msgf("license: auto-refresh applied new key (expires %s)",
			func() string {
				if lic.ExpiresAt != nil {
					return lic.ExpiresAt.Format("2006-01-02")
				}
				return "never"
			}())
}

type refreshResponse struct {
	Key string `json:"key"`
}

func (r *AutoRefresher) fetchKey(ctx context.Context) (string, error) {
	url := fmt.Sprintf("%s/api/v1/billing/license", r.baseURL)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return "", err
	}
	req.Header.Set("Authorization", "Bearer "+r.token)
	req.Header.Set("User-Agent", "vakt-license-refresh/1.0")

	client := &http.Client{Timeout: refreshHTTPTimeout}
	resp, err := client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("refresh endpoint returned %d", resp.StatusCode)
	}

	body, err := io.ReadAll(io.LimitReader(resp.Body, 4096))
	if err != nil {
		return "", err
	}

	var rr refreshResponse
	if err := json.Unmarshal(body, &rr); err != nil {
		return "", err
	}
	if rr.Key == "" {
		return "", fmt.Errorf("refresh response missing key")
	}
	return rr.Key, nil
}
