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
	refreshHTTPTimeout = 15 * time.Second

	// checkInterval is how often the instance LOOKS AT ITS OWN CLOCK. It is not how
	// often it calls anyone — the call only happens inside the renewal window.
	checkInterval = 24 * time.Hour

	// maxRenewWindow caps how early a renewal may start. Beyond a month there is
	// nothing to gain: the next invoice has not even been raised yet.
	maxRenewWindow = 30 * 24 * time.Hour
)

// renewWindow is derived from the KEY'S OWN LIFETIME, never fixed.
//
// A fixed 30-day window would be a disaster on the monthly plan: that key only lives
// 35 days, so it would sit INSIDE the window almost permanently and the instance
// would call every single day — the exact heartbeat we are trying not to build, hiding
// behind a name that says otherwise.
//
// A quarter of the key's life, capped at a month:
//
//	yearly  (395-day key)  ->  30 days  (capped)  ->  typically ONE call per year
//	monthly ( 35-day key)  ->   ~9 days            ->  a handful of calls per month
//
// Typically it is ONE call per renewal either way: the first successful fetch returns
// a key with a later expiry, which lifts the instance straight back out of the window
// and it goes quiet again. More calls only happen while an invoice is still unpaid —
// i.e. exactly when there is a real reason to keep asking.
func renewWindow(lic *License) time.Duration {
	if lic == nil || lic.ExpiresAt == nil {
		return 0
	}
	lifetime := lic.ExpiresAt.Sub(lic.IssuedAt)
	w := lifetime / 4
	if w > maxRenewWindow {
		w = maxRenewWindow
	}
	return w
}

// AutoRefresher fetches the next licence key when the current one is about to run
// out, and activates it silently.
//
// It is ON BY DEFAULT for a Pro instance, and that is a deliberate change from the
// old opt-in behaviour — but look at what it actually does before reading it as one:
//
//   - It only ever calls inside the last quarter of the key's life (capped at a
//     month). A yearly customer's instance contacts us roughly ONCE A YEAR; a monthly
//     one, a handful of times per renewal. The rest of the time it is silent — for
//     months, on the yearly plan. See renewWindow.
//   - It sends the licence's renewal token and nothing else. No org data, no user
//     count, no compliance content, no instance identifier.
//   - The token rides inside the signed key itself, so the customer configures
//     nothing. Before, renewal only worked for the people who happened to read the
//     right paragraph of the purchase mail; everyone else pasted a key by hand forever.
//   - VAKT_LICENSE_AUTORENEW=false turns it off completely. Then we mail the key
//     instead and the instance never speaks to us — a supported path, not a trap.
//   - The Community Edition has no key, so it never calls. Ever.
//
// If the server says no (unpaid, cancelled, revoked), nothing happens: the current
// key simply runs out. There is no kill switch and this is not one.
type AutoRefresher struct {
	token   string // fallback: VAKT_LICENSE_TOKEN. Normally read from the key itself.
	enabled bool
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
func NewAutoRefresher(token, baseURL string, enabled bool, h *Handler, db *pgxpool.Pool, rdb *redis.Client) *AutoRefresher {
	if baseURL == "" {
		baseURL = defaultRefreshURL
	}
	return &AutoRefresher{
		token:   token,
		enabled: enabled,
		baseURL: baseURL,
		handler: h,
		db:      db,
		rdb:     rdb,
	}
}

// currentToken is the renewal token for the key this instance is running.
//
// It comes from inside the signed key. VAKT_LICENSE_TOKEN remains as a fallback for
// keys issued before the token was embedded — and for anyone who prefers to set it
// explicitly.
func (r *AutoRefresher) currentToken() string {
	r.handler.mu.RLock()
	lic := r.handler.lic
	r.handler.mu.RUnlock()
	if lic != nil && lic.RenewalToken != "" {
		return lic.RenewalToken
	}
	return r.token
}

// due reports whether the current key is close enough to expiry to be worth asking
// about. This is the gate that turns a heartbeat into a renewal: outside the window,
// the instance does not call at all.
func (r *AutoRefresher) due() bool {
	r.handler.mu.RLock()
	lic := r.handler.lic
	r.handler.mu.RUnlock()
	if lic == nil || lic.ExpiresAt == nil {
		// A perpetual key never needs renewing. Nothing to ask for.
		return false
	}
	return time.Until(*lic.ExpiresAt) < renewWindow(lic)
}

// Start runs the refresh loop until ctx is cancelled.
// Refreshes immediately on start, then every 24h.
func (r *AutoRefresher) Start(ctx context.Context) {
	if !r.enabled {
		log.Info().Msg("license: auto-renewal disabled (VAKT_LICENSE_AUTORENEW=false) — this instance will never contact Norvik; keys arrive by mail")
		return
	}
	log.Info().Str("base_url", r.baseURL).
		Msg("license: auto-renewal armed — contacts Norvik only in the last quarter of the key's life, never otherwise")

	r.check(ctx)
	ticker := time.NewTicker(checkInterval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			r.check(ctx)
		}
	}
}

// check looks at the clock. It calls out ONLY if the key is about to expire — that
// distinction is the whole design, so it lives in one obvious place.
func (r *AutoRefresher) check(ctx context.Context) {
	if !r.due() {
		return
	}
	if r.currentToken() == "" {
		log.Warn().Msg("license: key is expiring but carries no renewal token — a new key must be entered by hand")
		return
	}
	r.tryRefresh(ctx)
}

func (r *AutoRefresher) tryRefresh(ctx context.Context) {
	key, err := r.fetchKey(ctx)
	if err != nil {
		// Keep the current key. It is still valid — this is a renewal, not a
		// permission check, and an outage on our side must never take a paying
		// customer down.
		log.Warn().Err(err).Msg("license: renewal fetch failed — keeping the current licence")
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
	req.Header.Set("Authorization", "Bearer "+r.currentToken())
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
