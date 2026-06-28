// Copyright (c) 2026 NorvikOps. All rights reserved.
// SPDX-License-Identifier: Elastic-2.0
// See LICENSE file in the project root for terms.

package main

import (
	"context"
	"net/http"
	"os"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/labstack/echo/v4"
	"github.com/redis/go-redis/v9"
	"github.com/rs/zerolog"

	"github.com/matharnica/vakt/internal/admin"
	"github.com/matharnica/vakt/internal/config"
	"github.com/matharnica/vakt/internal/license"
)

// ── S46-3: /health response types ────────────────────────────────────────────

// componentStatus is the per-subsystem health entry.
type componentStatus struct {
	Status    string `json:"status"` // "ok" | "error" | "disabled"
	LatencyMs int64  `json:"latency_ms,omitempty"`
}

// healthComponents groups all subsystem statuses.
type healthComponents struct {
	DB    componentStatus `json:"db"`
	Redis componentStatus `json:"redis"`
	AI    componentStatus `json:"ai"`
}

// healthResponse is the canonical /health response.
// CRITICAL fields (demo, sso_enabled, version) must always be present.
type healthResponse struct {
	Status     string           `json:"status"` // "ok" | "degraded" | "down"
	Version    string           `json:"version"`
	Demo       bool             `json:"demo"`
	SSOEnabled bool             `json:"sso_enabled"`
	Components healthComponents `json:"components"`
}

// healthHandler builds the /health response. db and rdb may be nil when called
// before the DB/Redis connections are established (early startup).
func healthHandler(c echo.Context, cfg *config.Config, db *pgxpool.Pool, rdb *redis.Client) error {
	// sso_enabled: env-var Casdoor OR DB-stored OIDC config (S105-2)
	ssoEnabled := cfg.CasdoorURL != "" && cfg.CasdoorClientID != ""
	if !ssoEnabled && db != nil {
		ctx, cancel := context.WithTimeout(c.Request().Context(), 1*time.Second)
		defer cancel()
		if ok, err := admin.NewRepository(db).OIDCEnabledExists(ctx); err == nil {
			ssoEnabled = ok
		}
	}
	resp := healthResponse{
		Status:     "ok",
		Version:    cfg.Version,
		Demo:       cfg.DemoSeed,
		SSOEnabled: ssoEnabled,
	}

	// DB component check
	if db != nil {
		ctx, cancel := context.WithTimeout(c.Request().Context(), 2*time.Second)
		defer cancel()
		start := time.Now()
		err := db.Ping(ctx)
		resp.Components.DB = componentStatus{
			Status:    "ok",
			LatencyMs: time.Since(start).Milliseconds(),
		}
		if err != nil {
			resp.Components.DB.Status = "error"
			resp.Status = "down"
		}
	} else {
		resp.Components.DB = componentStatus{Status: "disabled"}
	}

	// Redis component check
	if rdb != nil {
		ctx, cancel := context.WithTimeout(c.Request().Context(), 1*time.Second)
		defer cancel()
		start := time.Now()
		err := rdb.Ping(ctx).Err()
		resp.Components.Redis = componentStatus{
			Status:    "ok",
			LatencyMs: time.Since(start).Milliseconds(),
		}
		if err != nil {
			resp.Components.Redis.Status = "error"
			if resp.Status != "down" {
				resp.Status = "degraded"
			}
		}
	} else {
		resp.Components.Redis = componentStatus{Status: "disabled"}
	}

	// AI component check
	if cfg.AIProvider == "" || cfg.AIProvider == "disabled" {
		resp.Components.AI = componentStatus{Status: "disabled"}
	} else {
		resp.Components.AI = componentStatus{Status: "ok"}
	}

	// Determine HTTP status code
	httpStatus := http.StatusOK
	if resp.Status == "degraded" || resp.Status == "down" {
		httpStatus = http.StatusServiceUnavailable
	}
	return c.JSON(httpStatus, resp)
}

func setupEcho(lifecycleCtx context.Context, cfg *config.Config) (*echo.Echo, *echo.Echo) {
	log := zerolog.New(os.Stdout).With().Timestamp().Logger()

	e := echo.New()
	e.HideBanner = true
	e.HidePort = true

	// Trust X-Forwarded-For from the reverse proxy (nginx in the compose stack).
	// VAKT_TRUSTED_PROXIES is a comma-separated CIDR list — each range becomes
	// an echo.TrustIPRange so XFF entries originating from outside the trusted
	// set are ignored. Echo's bare ExtractIPFromXFFHeader() with no options
	// trusts every hop in the chain, which lets an external client spoof a
	// trusted IP by sending its own XFF header. Audit finding "XFF without
	// TrustOption" — see docs/market-readiness-strategy.md.
	if trustedProxies := os.Getenv("VAKT_TRUSTED_PROXIES"); trustedProxies != "" {
		opts := buildXFFTrustOptions(trustedProxies, &log)
		e.IPExtractor = echo.ExtractIPFromXFFHeader(opts...)
		log.Info().Str("trusted_proxies", trustedProxies).Int("trust_options", len(opts)).Msg("IPExtractor configured for reverse proxy with explicit trust options")
	} else {
		e.IPExtractor = echo.ExtractIPDirect()
		log.Warn().Msg("VAKT_TRUSTED_PROXIES not set — running in direct IP mode. If this instance is behind a reverse proxy, IP-based rate limits and admin allowlists will see the proxy IP instead of the client IP. Set VAKT_TRUSTED_PROXIES to the proxy CIDR to fix.")
	}

	lic := license.Load(cfg.LicenseKey, cfg.DemoSeed)

	// Apply the global middleware chain (request-id, OTel, security headers,
	// logging, CORS, body limit, timeout, demo guard, license context).
	applyMiddleware(e, cfg, log, lic)

	// Liveness — always responds while the process is up.
	// Enthält flags die das Frontend braucht (siehe useDemoMode, Login.tsx):
	//   demo         — schaltet die Login-Page in den Ephemeral-Demo-Flow
	//   sso_enabled  — blendet den SSO-Button ein/aus
	//   version      — wird im Footer angezeigt
	//
	// S46-3: response extended with `components` for operational visibility.
	// CRITICAL: demo, sso_enabled, version must never be removed — they are
	// used by the frontend and the release smoke-test (api-contract-checklist.md).
	//
	// S90-8 (#9): this is the *fallback* /health registration. It runs with nil
	// db/rdb so that /health still answers when VAKT_DB_URL is unset and we take
	// the early return below (line ~322). When a DB IS available, the same route
	// is deliberately re-registered further down (search "re-register /health")
	// with live pool+rdb so the response gains DB/Redis/AI component statuses —
	// Echo keeps the last registration for a given method+path. The duplication
	// is intentional: removing either breaks one of the two startup paths.
	e.GET("/health", func(c echo.Context) error {
		return healthHandler(c, cfg, nil, nil)
	})

	// security.txt — public, no auth, RFC 9116.
	e.GET("/.well-known/security.txt", admin.HandleSecurityTXT)

	// Internal server — not exposed via Caddy/Nginx, Docker-network only.
	// Hosts routes that must never be reachable from the public internet.
	internal := echo.New()
	internal.HideBanner = true
	internal.HidePort = true

	if cfg.DBUrl == "" {
		log.Warn().Msg("VAKT_DB_URL not set — all routes disabled")
		return e, internal
	}

	// Register all DB-dependent routes (auth, modules, admin, etc.). Returns
	// early-and-silently inside registerRoutes if the DB/Redis/secret-key
	// prerequisites are not met; the partially configured Echo instance is
	// still returned so /health stays reachable.
	registerRoutes(lifecycleCtx, e, internal, cfg, log, lic)

	return e, internal
}
