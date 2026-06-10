// Copyright (c) 2026 NorvikOps. All rights reserved.
// SPDX-License-Identifier: Elastic-2.0
// See LICENSE file in the project root for terms.

package main

import (
	"context"
	"encoding/hex"
	"errors"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"runtime"
	"strings"
	"syscall"
	"time"

	"github.com/hibiken/asynq"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/labstack/echo/v4"
	"github.com/labstack/echo/v4/middleware"
	"github.com/redis/go-redis/v9"
	"github.com/rs/zerolog"
	"go.opentelemetry.io/contrib/instrumentation/github.com/labstack/echo/otelecho"
	"golang.org/x/time/rate"

	"github.com/matharnica/vakt/internal/admin"
	"github.com/matharnica/vakt/internal/auth"
	"github.com/matharnica/vakt/internal/config"
	"github.com/matharnica/vakt/internal/license"
	"github.com/matharnica/vakt/internal/modules/vaktaware"
	"github.com/matharnica/vakt/internal/modules/vaktcomply"
	"github.com/matharnica/vakt/internal/modules/vakthr"
	"github.com/matharnica/vakt/internal/modules/vaktprivacy"
	"github.com/matharnica/vakt/internal/modules/vaktscan"
	"github.com/matharnica/vakt/internal/modules/vaktvault"
	"github.com/matharnica/vakt/internal/services/ai"
	"github.com/matharnica/vakt/internal/services/alerting"
	"github.com/matharnica/vakt/internal/services/evidence_auto"
	scimSvc "github.com/matharnica/vakt/internal/services/scim"
	"github.com/matharnica/vakt/internal/shared/account"
	"github.com/matharnica/vakt/internal/shared/apidocs"
	"github.com/matharnica/vakt/internal/shared/apikeys"
	"github.com/matharnica/vakt/internal/shared/audit"
	"github.com/matharnica/vakt/internal/shared/comments"
	sharedcrypto "github.com/matharnica/vakt/internal/shared/crypto"
	"github.com/matharnica/vakt/internal/shared/dashboard"
	"github.com/matharnica/vakt/internal/shared/dataexport"
	shareddb "github.com/matharnica/vakt/internal/shared/db"
	"github.com/matharnica/vakt/internal/shared/demo"
	"github.com/matharnica/vakt/internal/shared/demoseed"
	"github.com/matharnica/vakt/internal/shared/feedback"
	"github.com/matharnica/vakt/internal/shared/metrics"
	sharedmw "github.com/matharnica/vakt/internal/shared/middleware"
	"github.com/matharnica/vakt/internal/shared/nis2wizard"
	"github.com/matharnica/vakt/internal/shared/notifications"
	"github.com/matharnica/vakt/internal/shared/notify"
	"github.com/matharnica/vakt/internal/shared/onboarding"
	"github.com/matharnica/vakt/internal/shared/platform/auditor"
	cloudintegration "github.com/matharnica/vakt/internal/shared/platform/integrations/cloud"
	ghintegration "github.com/matharnica/vakt/internal/shared/platform/integrations/github"
	"github.com/matharnica/vakt/internal/shared/platform/ldap"
	"github.com/matharnica/vakt/internal/shared/platform/trustcenter"
	sharedwebhooks "github.com/matharnica/vakt/internal/shared/platform/webhooks"
	"github.com/matharnica/vakt/internal/shared/retention"
	"github.com/matharnica/vakt/internal/shared/scheduledreports"
	"github.com/matharnica/vakt/internal/shared/search"
	"github.com/matharnica/vakt/internal/shared/setup"
	"github.com/matharnica/vakt/internal/shared/telemetry"
	"github.com/matharnica/vakt/internal/shared/updatecheck"
	"github.com/matharnica/vakt/internal/shared/usermgmt"
	lswebhook "github.com/matharnica/vakt/internal/webhooks/lemonsqueezy"
	polarwebhook "github.com/matharnica/vakt/internal/webhooks/polar"
)

// version is injected at build time via -ldflags "-X main.version=..."
var version = "dev"

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
	resp := healthResponse{
		Status:     "ok",
		Version:    cfg.Version,
		Demo:       cfg.DemoSeed,
		SSOEnabled: cfg.CasdoorURL != "" && cfg.CasdoorClientID != "",
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

// ─────────────────────────────────────────────────────────────────────────────

func setupEcho(lifecycleCtx context.Context, cfg *config.Config) *echo.Echo {
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

	// X-Request-ID — applied first so every subsequent log entry can reference it.
	e.Use(sharedmw.RequestID())

	// OpenTelemetry HTTP instrumentation — wraps every request in a span when
	// telemetry.Init() configured an exporter. No-op when OTEL_EXPORTER_OTLP_ENDPOINT
	// is unset (still safe to register; the global tracer provider is the noop one).
	e.Use(otelecho.Middleware("vakt-api",
		otelecho.WithSkipper(func(c echo.Context) bool {
			// Don't span on /metrics (Prometheus polls every 30s — would dominate
			// the trace volume) or on /health (likewise scraped by Zabbix).
			p := c.Request().URL.Path
			return p == "/metrics" || p == "/health"
		}),
	))

	// Trace ID — unique per request, emitted as X-Trace-ID response header and
	// enriched into the zerolog context for structured log correlation.
	e.Use(auth.TraceMiddleware())

	// style-src-elem 'self': only external stylesheets (<link>, <style> blocks) from same origin.
	// style-src-attr 'unsafe-inline': inline style= attributes allowed — required by Radix UI
	// which sets CSS custom properties (--radix-*) via element.style.setProperty() at runtime.
	// Splitting elem/attr is meaningfully safer than a blanket 'unsafe-inline' on style-src:
	// inline attributes cannot inject <style> blocks or @import rules, severely limiting CSS
	// exfiltration attack surface. Nonce-based CSP would be cleaner but requires Vite integration.
	e.Use(middleware.SecureWithConfig(middleware.SecureConfig{
		XSSProtection:         "0",
		ContentTypeNosniff:    "nosniff",
		XFrameOptions:         "DENY",
		HSTSMaxAge:            31536000,
		ContentSecurityPolicy: "default-src 'self'; script-src 'self'; style-src-elem 'self'; style-src-attr 'unsafe-inline'; img-src 'self' data: blob:; font-src 'self' data:; connect-src 'self'; frame-ancestors 'none'; object-src 'none'; base-uri 'self'",
	}))
	e.Use(func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			c.Response().Header().Set("Referrer-Policy", "strict-origin-when-cross-origin")
			c.Response().Header().Set("Permissions-Policy", "camera=(), microphone=(), geolocation=(), payment=(), usb=()")
			c.Response().Header().Set("Cross-Origin-Opener-Policy", "same-origin")
			// Cross-Origin-Resource-Policy: prevent other origins from loading
			// our JSON/asset responses via <img>/<script>/fetch — completes the
			// COOP/CORP/COEP triple that gives modern Spectre-class isolation.
			// API responses are same-origin only by design; no third-party
			// consumer exists.
			c.Response().Header().Set("Cross-Origin-Resource-Policy", "same-origin")
			return next(c)
		}
	})
	e.Use(middleware.RequestLoggerWithConfig(middleware.RequestLoggerConfig{
		LogMethod:  true,
		LogURI:     true,
		LogStatus:  true,
		LogLatency: true,
		LogValuesFunc: func(c echo.Context, v middleware.RequestLoggerValues) error {
			log.Info().
				Str("method", v.Method).
				Str("uri", v.URI).
				Int("status", v.Status).
				Dur("latency", v.Latency).
				Msg("request")
			return nil
		},
	}))
	e.Use(middleware.Recover())
	e.Use(middleware.GzipWithConfig(middleware.GzipConfig{
		Level: 5,
		Skipper: func(c echo.Context) bool {
			p := c.Request().URL.Path
			return p == "/metrics" || p == "/health"
		},
	}))
	e.Use(middleware.CORSWithConfig(middleware.CORSConfig{
		AllowOrigins:     cfg.CORSOrigins,
		AllowMethods:     []string{http.MethodGet, http.MethodPost, http.MethodPut, http.MethodPatch, http.MethodDelete, http.MethodOptions},
		AllowHeaders:     []string{echo.HeaderOrigin, echo.HeaderContentType, echo.HeaderAccept, echo.HeaderAuthorization, "X-Request-ID"},
		ExposeHeaders:    []string{"X-RateLimit-Limit", "X-RateLimit-Remaining", "X-RateLimit-Reset", "X-Request-ID"},
		AllowCredentials: true,
		MaxAge:           86400,
	}))
	if len(cfg.CORSOrigins) == 1 && cfg.CORSOrigins[0] == "*" {
		log.Warn().Msg("CORS is configured to allow all origins (*) with credentials — set VAKT_CORS_ORIGINS for production")
	}
	e.Use(middleware.BodyLimit("10MB"))
	e.Use(middleware.ContextTimeoutWithConfig(middleware.ContextTimeoutConfig{
		Timeout: 30 * time.Second,
		ErrorHandler: func(err error, c echo.Context) error {
			if err != nil && errors.Is(err, context.DeadlineExceeded) {
				return c.JSON(http.StatusServiceUnavailable, map[string]string{
					"error": "request timeout",
					"code":  "REQUEST_TIMEOUT",
				})
			}
			return err
		},
	}))
	e.Use(demo.Guard(cfg.DemoSeed))

	lic := license.Load(cfg.LicenseKey, cfg.DemoSeed)
	e.Use(func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			c.Set("license", lic)
			return next(c)
		}
	})

	// Liveness — always responds while the process is up.
	// Enthält flags die das Frontend braucht (siehe useDemoMode, Login.tsx):
	//   demo         — schaltet die Login-Page in den Ephemeral-Demo-Flow
	//   sso_enabled  — blendet den SSO-Button ein/aus
	//   version      — wird im Footer angezeigt
	//
	// S46-3: response extended with `components` for operational visibility.
	// CRITICAL: demo, sso_enabled, version must never be removed — they are
	// used by the frontend and the release smoke-test (api-contract-checklist.md).
	e.GET("/health", func(c echo.Context) error {
		return healthHandler(c, cfg, nil, nil)
	})

	// security.txt — public, no auth, RFC 9116.
	e.GET("/.well-known/security.txt", admin.HandleSecurityTXT)

	if cfg.DBUrl == "" {
		log.Warn().Msg("VAKT_DB_URL not set — all routes disabled")
		return e
	}

	ctx := context.Background()
	pool, err := shareddb.Connect(ctx, cfg.DBUrl)
	if err != nil {
		log.Warn().Err(err).Msg("DB unavailable — all routes disabled")
		return e
	}

	api := e.Group("/api/v1")

	// Readiness — checks DB connectivity (registered after pool is available).
	e.GET("/health/ready", func(c echo.Context) error {
		if err := pool.Ping(c.Request().Context()); err != nil {
			log.Error().Err(err).Msg("health/ready: database ping failed")
			return c.JSON(http.StatusServiceUnavailable, map[string]string{
				"status": "unavailable", "component": "database", "error": "database unavailable",
			})
		}
		return c.JSON(http.StatusOK, map[string]string{"status": "ready"})
	})

	// Trust Center — public, no auth
	trustcenter.Register(e, pool)
	log.Info().Msg("trust center routes registered")

	// Sprint 19 / S19-1: NIS2-Self-Assessment-Wizard — public, no auth.
	// Rate-limited gegen Abuse (5 Calls/min/IP). CE-Top-of-Funnel-Asset.
	nis2RateLimiter := middleware.RateLimiter(middleware.NewRateLimiterMemoryStoreWithConfig(
		middleware.RateLimiterMemoryStoreConfig{Rate: rate.Limit(5.0 / 60.0), Burst: 10, ExpiresIn: 5 * time.Minute},
	))
	nis2wizardHandler := nis2wizard.NewHandler(nis2wizard.NewService(pool), cfg.SecretKey)
	nis2wizard.Register(api.Group("/public/nis2-assessment", nis2RateLimiter), nis2wizardHandler)
	log.Info().Msg("nis2 wizard public routes registered")

	// S28-1: NIS2 Embedded-Mode — override the global X-Frame-Options: DENY and
	// CSP frame-ancestors 'none' for paths that must be embeddable in partner iframes.
	// Applies to both the API endpoints and the frontend SPA route (/nis2-check).
	e.Use(func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			p := c.Request().URL.Path
			isNIS2Public := strings.HasPrefix(p, "/nis2-check") ||
				strings.HasPrefix(p, "/api/v1/public/nis2-assessment")
			if isNIS2Public {
				// Remove the restrictive X-Frame-Options set by the global Secure middleware.
				c.Response().Header().Del("X-Frame-Options")
				// Override the CSP to allow framing from any origin (see ADR-0028).
				c.Response().Header().Set("Content-Security-Policy",
					"default-src 'self'; script-src 'self'; style-src-elem 'self'; style-src-attr 'unsafe-inline'; img-src 'self' data: blob:; font-src 'self' data:; connect-src 'self'; frame-ancestors *; object-src 'none'; base-uri 'self'")
				// Minimize hostname leakage when navigating from the embedded iframe.
				c.Response().Header().Set("Referrer-Policy", "strict-origin-when-cross-origin")
			}
			return next(c)
		}
	})

	// Setup wizard — rate-limited, no auth (only works before first org exists).
	setupRateLimiter := middleware.RateLimiter(middleware.NewRateLimiterMemoryStoreWithConfig(
		middleware.RateLimiterMemoryStoreConfig{Rate: rate.Limit(5.0 / 60.0), Burst: 5, ExpiresIn: 5 * time.Minute},
	))
	setupHandler := setup.NewHandler(pool)
	setup.Register(api.Group("/setup", setupRateLimiter), setupHandler)
	log.Info().Msg("setup routes registered")

	if cfg.RedisUrl == "" || cfg.SecretKey == "" {
		log.Warn().Msg("VAKT_REDIS_URL or VAKT_SECRET_KEY not set — auth/module routes disabled")
		return e
	}

	redisOpt, err := redis.ParseURL(cfg.RedisUrl)
	if err != nil {
		log.Warn().Err(err).Msg("invalid Redis URL — auth/module routes disabled")
		return e
	}

	// Decode the raw master key once and derive purpose-specific sub-keys via HKDF.
	// This ensures a compromise of one derived key cannot be extended to others.
	// NOTE: Rotating to derived keys for vault/TOTP/alert/GitHub/cloud requires a
	// re-encryption migration (planned for v0.30.0). Only PASETO is switched here
	// because PASETO tokens are stateless — a key change merely invalidates sessions.
	rawMasterKey, err := hex.DecodeString(cfg.SecretKey)
	if err != nil {
		log.Warn().Err(err).Msg("invalid secret key (hex decode) — auth/module routes disabled")
		return e
	}

	// Derive per-service keys via HKDF-SHA256.  Each service gets a unique 32-byte
	// sub-key so a compromise of one cannot be extended to others.
	deriveKey := func(purpose string) []byte {
		k, kErr := sharedcrypto.DeriveServiceKey(rawMasterKey, purpose)
		if kErr != nil {
			log.Fatal().Err(kErr).Str("purpose", purpose).Msg("HKDF key derivation failed")
		}
		return k
	}
	vaultKey := deriveKey("vakt-vault-v1")
	totpKey := deriveKey("vakt-totp-v1")
	alertKey := deriveKey("vakt-alert-v1")
	ghKey := deriveKey("vakt-github-v1")
	cloudKey := deriveKey("vakt-cloud-v1")
	webhookKey := deriveKey("vakt-webhook-v1")

	pasetoKeyBytes := deriveKey("vakt-paseto-v1")
	pasetoKey, err := auth.GenerateSymmetricKeyFromBytes(pasetoKeyBytes)
	if err != nil {
		log.Warn().Err(err).Msg("invalid derived PASETO key — auth/module routes disabled")
		return e
	}

	// Auth routes — rate-limited (5 req/min per IP, S45-5), no token middleware (they issue tokens).
	rdb := redis.NewClient(redisOpt)

	// S46-3: Now that we have pool + rdb, re-register /health with full component checks.
	// The initial registration (before DB/Redis were available) returns a minimal response.
	// Overriding here gives us DB + Redis + AI component statuses.
	e.GET("/health", func(c echo.Context) error {
		return healthHandler(c, cfg, pool, rdb)
	})

	// Extend readiness check to include Redis now that rdb is available.
	e.GET("/health/ready", func(c echo.Context) error {
		ctx := c.Request().Context()
		dbStart := time.Now()
		if err := pool.Ping(ctx); err != nil {
			log.Error().Err(err).Msg("health/ready: database ping failed")
			return c.JSON(http.StatusServiceUnavailable, map[string]string{
				"status": "unavailable", "component": "database", "error": err.Error(),
			})
		}
		dbLatencyMs := time.Since(dbStart).Milliseconds()
		redisStart := time.Now()
		if err := rdb.Ping(ctx).Err(); err != nil {
			log.Error().Err(err).Msg("health/ready: redis ping failed")
			return c.JSON(http.StatusServiceUnavailable, map[string]string{
				"status": "unavailable", "component": "redis", "error": err.Error(),
			})
		}
		redisLatencyMs := time.Since(redisStart).Milliseconds()
		return c.JSON(http.StatusOK, map[string]any{
			"status":           "ready",
			"db_latency_ms":    dbLatencyMs,
			"redis_latency_ms": redisLatencyMs,
			"version":          version,
		})
	})

	// Auth routes — Redis-backed IP rate limit (5 req/min) on the four
	// credential-submission endpoints, plus a broader in-memory limiter on the
	// full auth group for burst protection on other endpoints (S45-5).
	authRateLimiter := middleware.RateLimiter(middleware.NewRateLimiterMemoryStoreWithConfig(
		middleware.RateLimiterMemoryStoreConfig{Rate: rate.Limit(5.0 / 60.0), Burst: 5, ExpiresIn: 5 * time.Minute},
	))
	redisAuthRL := sharedmw.AuthRateLimit(rdb)
	authSvc := auth.NewService(pool, rdb, pasetoKey)
	// ADR-0044: default fail-closed on Redis outage. Operators can opt back
	// into the legacy fail-open behaviour by setting
	// VAKT_AUTH_FAIL_OPEN_ON_REDIS_OUTAGE=true. This accepts a short
	// brute-force window during a Redis outage in exchange for availability.
	if os.Getenv("VAKT_AUTH_FAIL_OPEN_ON_REDIS_OUTAGE") == "true" {
		authSvc = authSvc.WithFailOpenOnRedisOutage(true)
		log.Warn().Msg("auth: VAKT_AUTH_FAIL_OPEN_ON_REDIS_OUTAGE=true — lockout checks will fail open during Redis outages (audit-relevant choice)")
	}
	authHandler := auth.NewHandler(authSvc, cfg)
	authGroup := api.Group("/auth", authRateLimiter)
	auth.Register(authGroup, authHandler)
	// Apply Redis-backed rate limit specifically to the 4 credential routes.
	api.POST("/auth/login", authHandler.Login, redisAuthRL)
	api.POST("/auth/register", authHandler.Register, redisAuthRL)
	api.POST("/auth/password-reset/request", authHandler.RequestPasswordReset, redisAuthRL)
	api.POST("/auth/password-reset/confirm", authHandler.ResetPassword, redisAuthRL)
	log.Info().Msg("auth routes registered")

	// All subsequent routes require a valid Paseto token
	protected := api.Group("", auth.AuthMiddleware(pasetoKey, pool, rdb))
	protected.GET("/auth/me", authHandler.Me)

	// CSRF protection: double-submit-cookie pattern on state-changing methods.
	// API-key requests (Bearer sk_/vakt_) are exempt because they are not
	// browser-driven. Webhook deliveries from external systems are also exempt
	// (they authenticate via HMAC signature, not cookie). Auth routes that
	// establish a session sit outside `protected` and therefore aren't gated.
	protected.Use(auth.CSRFMiddleware(
		"/api/v1/webhooks/receive",
	))

	// Org-wide MFA enforcement: if the org has require_mfa=true and the user has
	// not completed TOTP setup, return 403 MFA_REQUIRED on all protected routes
	// except the 2FA setup/confirm flow and logout.
	protected.Use(auth.MFAEnforceMiddleware(pool))

	// Per-request license resolution: load DB key / check revocation blocklist after auth sets org_id.
	// rdb is passed for optional Redis caching (60 s TTL) to avoid 2 DB queries per request.
	protected.Use(license.DBMiddleware(pool, lic, rdb))

	// Global per-org rate limiting: 300 req/min, keyed by org_id from Paseto claims.
	// Must be applied after auth middleware has populated org_id in the context.
	// Redis-backed variant is multi-replica safe; in-memory fallback is only used
	// when Redis is not configured (rare — auth itself requires Redis).
	if rdb != nil {
		protected.Use(sharedmw.OrgRateLimitRedis(rdb))
	} else {
		log.Warn().Msg("Redis not configured — using in-memory per-org rate limiter. This is NOT multi-replica safe: the effective limit scales with replica count. Configure VAKT_REDIS_URL for production deployments.")
		protected.Use(sharedmw.OrgRateLimit())
	}

	// License auto-refresh: when VAKT_LICENSE_TOKEN is set the instance polls
	// api.norvikops.de every 24h and silently activates the latest key.
	licHandler := license.RegisterRoutes(api, lic, auth.AuthMiddleware(pasetoKey, pool, rdb), pool, rdb)
	if cfg.LicenseToken != "" {
		licHandler.WithAutoRenewal()
		refresher := license.NewAutoRefresher(cfg.LicenseToken, cfg.LicenseRefreshURL, licHandler, pool, rdb)
		go refresher.Start(lifecycleCtx)
		log.Info().Msg("license: auto-renewal active")
	}
	log.Info().Msg("license routes registered")

	// Update check service (opt-in, no phone-home)
	updateSvc := updatecheck.NewService(cfg.UpdateCheck, cfg.Version, rdb)
	updatecheck.Register(protected, updateSvc)
	updateSvc.StartBackgroundRefresh(lifecycleCtx)
	log.Info().Msg("update check routes registered")

	// Admin routes (also require Admin role)
	asynqClient := asynq.NewClient(asynq.RedisClientOpt{Addr: redisOpt.Addr})
	adminSvc := admin.NewService(pool, cfg.ModulesEnabled)
	adminSvc.WithNotifyService(notify.NewService(pool, cfg))
	adminHealth := admin.NewHealthHandler(pool, rdb, cfg)
	adminHandler := admin.NewHandler(adminSvc)
	admin.Register(protected, adminHandler, adminHealth, pool, rdb)
	// Job queue stats — admin-only, same auth guard as other admin routes.
	jobsHandler := admin.NewJobsHandler(redisOpt.Addr)
	protected.GET("/admin/jobs", jobsHandler.GetQueueStats, auth.RequireRole("Admin"), sharedmw.IPAllowlist())
	// Admin-scoped auth management routes (password reset token generation without SMTP).
	auth.RegisterAdminRoutes(protected, authHandler)
	log.Info().Msg("admin routes registered")

	if cfg.Staging {
		admin.RegisterStaging(protected, admin.NewStagingHandler(cfg.PromoteURL, cfg.PromoteSecret))
		log.Info().Msg("staging routes registered")
	}

	// SCIM 2.0 provisioning — uses its own Bearer token auth (not Paseto).
	// Mounted on the plain api group; SCIMAuthMiddleware + feature gate are
	// applied inside scimSvc.Register.
	scimSvc.Register(api.Group("/scim/v2"), pool)
	log.Info().Msg("scim routes registered")

	// Outgoing webhooks — created before modules so event triggers can be wired in.
	// The webhookSvc is also registered as routes below (after module routes).
	webhookSvc := sharedwebhooks.NewWebhookService(pool, webhookKey)
	// One-time, idempotent migration of legacy plaintext secrets to the
	// enc:v1: format.  See ADR-0043 — sprint 58 closure on the
	// "webhooks.secret stored as plaintext" audit subnote.
	if migrated, err := webhookSvc.MigrateLegacyPlaintextSecrets(lifecycleCtx); err != nil {
		log.Warn().Err(err).Msg("webhooks: legacy plaintext migration encountered errors")
	} else if migrated > 0 {
		log.Info().Int("migrated", migrated).Msg("webhooks: legacy plaintext secrets upgraded to enc:v1:")
	}

	// Module routes — all behind auth middleware, sharing the same DB pool
	if cfg.IsModuleEnabled("vaktscan") {
		vbSvc := vaktscan.NewService(pool, asynq.RedisClientOpt{Addr: redisOpt.Addr})
		vbSvc.WithRedis(rdb)
		vbSvc.WithWebhooks(webhookSvc)
		vaktscan.Register(protected.Group("/vaktscan", auth.RequireModuleAccess(pool, "vaktscan")), vaktscan.NewHandler(vbSvc))
		log.Info().Msg("vaktscan routes registered")
	}

	auditorRateLimiter := middleware.RateLimiter(middleware.NewRateLimiterMemoryStoreWithConfig(
		middleware.RateLimiterMemoryStoreConfig{Rate: rate.Limit(30.0 / 60.0), Burst: 30, ExpiresIn: 5 * time.Minute},
	))
	auditorAcceptRateLimiter := middleware.RateLimiter(middleware.NewRateLimiterMemoryStoreWithConfig(
		middleware.RateLimiterMemoryStoreConfig{Rate: rate.Limit(10.0 / 60.0), Burst: 10, ExpiresIn: 5 * time.Minute},
	))

	// cloudEvidence bridges vaktcomply → cloud integration without a direct import.
	// It is set inside the vaktcomply block and falls back to a no-op when vaktcomply is disabled.
	var cloudEvidence = cloudintegration.NoopEvidenceWriter()

	// hrEvidence bridges hr → vaktcomply without a direct import.
	// Set inside the vaktcomply block; falls back to a no-op when vaktcomply is disabled.
	hrEvidence := vakthr.EvidenceWriter(vakthr.NoopEvidenceWriter())

	// hrAccessReview triggers an access-review campaign in vaktcomply when an offboarding run completes.
	// Set inside the vaktcomply block; falls back to a no-op when vaktcomply is disabled.
	hrAccessReview := vakthr.AccessReviewTrigger(&vakthr.NoopAccessReviewTrigger{})

	if cfg.IsModuleEnabled("vaktcomply") {
		ckSvc := vaktcomply.NewService(pool)
		ckSvc.WithRedis(rdb)
		ckSvc.WithNotifyService(notify.NewService(pool, cfg))
		ckSvc.WithWebhooks(webhookSvc)
		if cfg.AIProvider != "disabled" && cfg.AIProvider != "" && cfg.AIBaseURL != "" {
			ckSvc.WithAIClient(ai.NewAIClient(cfg.AIBaseURL, cfg.AIAPIKey, cfg.AIModel))
		}
		cloudEvidence = vaktcomply.NewCloudEvidenceWriter(ckSvc.Repo())
		hrEvidence = vaktcomply.NewHREvidenceWriter(pool)
		hrAccessReview = vaktcomply.NewHRAccessReviewTrigger(pool)
		ckSvc.ReseedBuiltinControls(ctx)
		ckSvc.SeedBuiltinMeasures(ctx)
		if err := ckSvc.SeedFrameworkMappings(ctx); err != nil {
			log.Warn().Err(err).Msg("seed framework mappings failed (non-critical)")
		}
		// S69-1: Seed prerequisite chains (global, org-agnostic).
		if err := ckSvc.SeedPrerequisiteChains(ctx); err != nil {
			log.Warn().Err(err).Msg("seed prerequisite chains failed (non-critical)")
		}
		if err := vaktcomply.SeedPolicyTemplates(ctx, pool); err != nil {
			log.Warn().Err(err).Msg("seed policy templates failed (non-critical)")
		}
		ckHandler := vaktcomply.NewHandler(ckSvc).WithDB(pool)
		ckHandler.WithPolicyAcceptanceConfig(vaktcomply.PolicyAcceptanceHandlerConfig{
			SMTPHost:    cfg.SMTPHost,
			SMTPPort:    cfg.SMTPPort,
			SMTPUser:    cfg.SMTPUser,
			SMTPPass:    cfg.SMTPPass,
			SMTPFrom:    cfg.SMTPFrom,
			FrontendURL: cfg.FrontendURL,
		})
		// Evidence file uploads — ensure upload directory exists at startup.
		if err := os.MkdirAll(filepath.Join(cfg.UploadDir, "evidence"), 0o755); err != nil {
			log.Warn().Err(err).Msg("could not create evidence upload dir")
		}
		efSvc := vaktcomply.NewEvidenceFileService(ckSvc.Repo(), cfg.UploadDir)
		ckHandler.WithEvidenceFileService(efSvc)
		vaktcomply.Register(protected.Group("/vaktcomply", auth.RequireModuleAccess(pool, "vaktcomply")), ckHandler)
		// Sprint 22 / S22-6: authentifizierter NIS2-Wizard-Migrate-Endpoint
		// (POST /vaktcomply/nis2-assessment/migrate-from-anonymous).
		nis2wizard.RegisterAuthenticated(protected.Group("/vaktcomply", auth.RequireModuleAccess(pool, "vaktcomply")), nis2wizardHandler)
		// Auditor portal uses URL token — exempt from Bearer auth; rate-limited to 30 req/min per IP
		portalRateLimiter := middleware.RateLimiter(middleware.NewRateLimiterMemoryStoreWithConfig(
			middleware.RateLimiterMemoryStoreConfig{Rate: rate.Limit(30.0 / 60.0), Burst: 30, ExpiresIn: 5 * time.Minute},
		))
		vaktcomply.RegisterPublic(api.Group("/vaktcomply", portalRateLimiter), ckHandler)
		// Policy acceptance — public token routes (no Bearer auth), rate-limited
		vaktcomply.RegisterPolicyAcceptPublic(api.Group("", portalRateLimiter), ckHandler)
		// Audit package export
		audit.RegisterExport(protected.Group("/vaktcomply", auth.RequireModuleAccess(pool, "vaktcomply")), pool)
		// One-click audit report PDF
		audit.RegisterReport(protected.Group("/vaktcomply", auth.RequireModuleAccess(pool, "vaktcomply")), pool)
		// AI-generated reports via OpenAI-compatible provider.
		// Sprint 15 (S15-1/2/3/5): Rate-Limit + Daily-Quota + Response-Cache
		// + Streaming-SSE-Endpoint laufen über RegisterWithOptions, sofern
		// Redis verfügbar ist.
		aiFailOpen := os.Getenv("VAKT_AI_FAIL_OPEN_ON_OUTAGE") == "true"
		if aiFailOpen {
			log.Warn().Msg("ai: VAKT_AI_FAIL_OPEN_ON_OUTAGE=true — rate-limit + quota checks will fail open during Redis/Postgres outages (audit-relevant choice)")
		}
		ai.RegisterWithOptions(protected.Group("/vaktcomply", auth.RequireModuleAccess(pool, "vaktcomply")), pool, cfg.AIProvider, cfg.AIBaseURL, cfg.AIAPIKey, cfg.AIModel, ai.RegisterOptions{
			Redis:            rdb,
			RateLimitRPM:     cfg.AIRateLimitRPM,
			DailyTokenLimit:  cfg.AIDailyTokenLimit,
			CacheTTLSeconds:  cfg.AICacheTTLSeconds,
			CostPerMTokenIn:  cfg.AICostPerMTokenIn,
			CostPerMTokenOut: cfg.AICostPerMTokenOut,
			FailOpenOnOutage: aiFailOpen,
		})
		// Auditor portal — read-only vaktcomply access via session token (no Bearer auth)
		vaktcomply.RegisterAuditor(api.Group("/auditor/vaktcomply", auditorRateLimiter, auditor.AuditorAuth(pool)), ckHandler)
		// Auto-evidence inbox — GitHub, SecReflex, SecPulse
		evidence_auto.RegisterRoutes(protected.Group("/vaktcomply", auth.RequireModuleAccess(pool, "vaktcomply")), pool)
		log.Info().Msg("vaktcomply routes registered")
	}

	if cfg.IsModuleEnabled("vaktvault") && cfg.SecretKey != "" {
		soSvc := vaktvault.NewService(pool, vaultKey, asynqClient)
		vaktvault.Register(protected.Group("/vaktvault", auth.RequireModuleAccess(pool, "vaktvault")), vaktvault.NewHandler(soSvc))
		log.Info().Msg("vaktvault routes registered")
	}

	if cfg.IsModuleEnabled("vaktaware") {
		pgSvc := vaktaware.NewService(pool, vaktaware.SMTPConfig{
			Host: cfg.SMTPHost, Port: cfg.SMTPPort,
			User: cfg.SMTPUser, Pass: cfg.SMTPPass, From: cfg.SMTPFrom,
		}, asynq.RedisClientOpt{Addr: redisOpt.Addr})
		vaktaware.Register(protected.Group("/vaktaware", auth.RequireModuleAccess(pool, "vaktaware")), vaktaware.NewHandler(pgSvc))
		log.Info().Msg("vaktaware routes registered")
	}

	// External alerting & webhooks (cross-module) — created before modules that fire events.
	var alertSvc *alerting.Service
	if cfg.SecretKey != "" {
		alertSvc = alerting.NewService(pool, alertKey, alerting.SMTPConfig{
			Host: cfg.SMTPHost,
			Port: cfg.SMTPPort,
			User: cfg.SMTPUser,
			Pass: cfg.SMTPPass,
			From: cfg.SMTPFrom,
		})
		alerting.Register(api, pool, alertKey, alerting.SMTPConfig{
			Host: cfg.SMTPHost, Port: cfg.SMTPPort,
			User: cfg.SMTPUser, Pass: cfg.SMTPPass, From: cfg.SMTPFrom,
		}, auth.AuthMiddleware(pasetoKey, pool, rdb))
		log.Info().Msg("alerting routes registered")
	}

	if cfg.IsModuleEnabled("vaktprivacy") {
		poSvc := vaktprivacy.NewService(pool, asynq.RedisClientOpt{Addr: redisOpt.Addr})
		tiaSvc := vaktprivacy.NewTIAService(pool)
		poHandler := vaktprivacy.NewHandler(poSvc).WithDB(pool).WithTIA(tiaSvc)
		if alertSvc != nil {
			poHandler.WithAlerting(alertSvc.Fire)
		}
		vaktprivacy.Register(protected.Group("/vaktprivacy", auth.RequireModuleAccess(pool, "vaktprivacy")), poHandler)
		// DSR portal uses URL slug/token — exempt from Bearer auth; rate-limited to 30 req/min per IP
		dsrPortalRateLimiter := middleware.RateLimiter(middleware.NewRateLimiterMemoryStoreWithConfig(
			middleware.RateLimiterMemoryStoreConfig{Rate: rate.Limit(30.0 / 60.0), Burst: 30, ExpiresIn: 5 * time.Minute},
		))
		vaktprivacy.RegisterPublic(api.Group("/vaktprivacy", dsrPortalRateLimiter), poHandler)
		log.Info().Msg("vaktprivacy routes registered")
	}

	// HR module — onboarding and offboarding workflows
	hrSvc := vakthr.NewService(vakthr.NewRepository(pool)).
		WithEvidenceWriter(hrEvidence).
		WithAccessReviewTrigger(hrAccessReview)
	hrHandler := vakthr.NewHandler(hrSvc)
	vakthr.Register(protected.Group("/vakthr", auth.RequireModuleAccess(pool, "vakthr")), hrHandler)
	log.Info().Msg("vakthr routes registered")

	// Account self-service: DSGVO Art. 17 (delete) and Art. 20 (export).
	accountHandler := account.NewHandler(account.NewService(pool))
	account.Register(protected, accountHandler)
	// Sprint 22 S22-11: Login-History-Endpoint.
	account.RegisterLoginHistory(protected, account.NewLoginHistoryHandler(pool))
	log.Info().Msg("account routes registered")

	// GitHub integration — branch protection, PR review, dependency alert compliance checks
	if cfg.SecretKey != "" {
		ghintegration.RegisterRoutes(protected.Group("/integrations/github"), pool, ghKey)
		log.Info().Msg("github integration routes registered")
	}

	// Cloud integrations — AWS + Azure automated evidence collection
	if cfg.SecretKey != "" {
		cloudSvc := cloudintegration.RegisterRoutes(protected.Group("/integrations/cloud"), pool, cloudKey, cloudEvidence)
		log.Info().Msg("cloud integration routes registered")

		// Inject Personio secret provider into the HR handler so the webhook can verify HMAC sigs.
		// The webhook is registered on the public api group (no Bearer auth — HMAC only).
		hrHandler.WithPersonioSecrets(cloudSvc)
		api.POST("/vakthr/webhooks/personio/:org_id", hrHandler.HandlePersonioWebhook)
		log.Info().Msg("personio webhook route registered at /api/v1/vakthr/webhooks/personio/:org_id")
	}

	// Outgoing webhooks — org-scoped event delivery (cross-module).
	// webhookSvc was created before the module section; register routes here.
	webhookHandler := sharedwebhooks.NewHandler(webhookSvc)
	sharedwebhooks.Register(protected.Group("/webhooks"), webhookHandler)
	log.Info().Msg("webhook routes registered")

	// Scheduled reports — automated compliance/findings/risk report delivery via email
	srSvc := scheduledreports.NewService(pool, scheduledreports.SMTPConfig{
		Host: cfg.SMTPHost,
		Port: cfg.SMTPPort,
		User: cfg.SMTPUser,
		Pass: cfg.SMTPPass,
		From: cfg.SMTPFrom,
	})
	scheduledreports.Register(protected.Group("/reports"), scheduledreports.NewHandler(srSvc))
	log.Info().Msg("scheduled reports routes registered")

	// API key management — personal keys for programmatic access (Pro feature)
	apikeys.Register(protected, pool)
	log.Info().Msg("api key routes registered")

	// Shared comments — threaded discussion on findings and controls
	comments.Register(protected, pool)
	log.Info().Msg("comments routes registered")

	// Notification preferences — per-user email and in-app opt-in/out settings
	notifPrefsSvc := notifications.NewPreferencesService(pool)
	notifPrefsHandler := notifications.NewPreferencesHandler(notifPrefsSvc)
	notifications.RegisterPreferences(protected.Group("/notifications"), notifPrefsHandler)
	log.Info().Msg("notification preferences routes registered")

	// Audit log — compliance event history
	audit.RegisterRoutes(protected.Group("/audit-log"), pool)
	log.Info().Msg("audit log routes registered")

	// Full data export — DSGVO Art. 20 portability + migration safety
	dataexport.RegisterRoutes(protected.Group("/export"), pool)
	log.Info().Msg("data export routes registered")

	// Auditor portal — invite management (admin) + public accept route
	// Public auditor accept route rate-limited to 30 req/min per IP.
	auditor.RegisterRoutes(protected.Group("/auditor"), pool)
	auditor.RegisterPublicRoutes(api.Group("/auditor", auditorAcceptRateLimiter), pool)
	log.Info().Msg("auditor routes registered")

	// User management & team invitations
	// Public invite accept route rate-limited to 10 req/min per IP (same as auth).
	inviteRateLimiter := middleware.RateLimiter(middleware.NewRateLimiterMemoryStoreWithConfig(
		middleware.RateLimiterMemoryStoreConfig{Rate: rate.Limit(10.0 / 60.0), Burst: 10, ExpiresIn: 5 * time.Minute},
	))
	umSvc := usermgmt.NewService(pool, usermgmt.SMTPConfig{
		Host: cfg.SMTPHost, Port: cfg.SMTPPort,
		User: cfg.SMTPUser, Pass: cfg.SMTPPass, From: cfg.SMTPFrom,
	}, cfg.FrontendURL)
	usermgmt.RegisterRoutes(protected.Group("/admin"), api.Group("/invite", inviteRateLimiter), umSvc, pool)
	log.Info().Msg("user management routes registered")

	// Onboarding wizard status and dismiss
	onboarding.RegisterRoutes(protected.Group("/onboarding"), pool)
	log.Info().Msg("onboarding routes registered")

	// Trust Center admin — configure public trust page
	trustcenter.RegisterAdmin(protected, pool)
	log.Info().Msg("trust center admin routes registered")

	// Dashboard — shared cross-module score endpoint (aggregate cached in Redis for 60 s)
	dashboard.Register(api.Group("/dashboard"), pool, rdb, auth.AuthMiddleware(pasetoKey, pool, rdb))
	log.Info().Msg("dashboard routes registered")

	// Global search — cross-module text search
	search.Register(api, pool, auth.AuthMiddleware(pasetoKey, pool, rdb))

	// Retention config API — data-pruning settings per org
	retention.Register(api, pool, auth.AuthMiddleware(pasetoKey, pool, rdb))
	log.Info().Msg("retention routes registered")

	// 2FA/TOTP — local account second factor
	if cfg.SecretKey != "" {
		totpRateLimiter := middleware.RateLimiter(middleware.NewRateLimiterMemoryStoreWithConfig(
			middleware.RateLimiterMemoryStoreConfig{Rate: rate.Limit(5.0 / 60.0), Burst: 5, ExpiresIn: 5 * time.Minute},
		))
		auth.RegisterTOTP(api.Group("/auth"), pool, totpKey, auth.AuthMiddleware(pasetoKey, pool, rdb), authSvc, totpRateLimiter)
		log.Info().Msg("2FA/TOTP routes registered")
	}

	// Session management — list and revoke active sessions
	auth.RegisterSessions(protected.Group("/auth/sessions"), pool, rdb)
	log.Info().Msg("session routes registered")

	// LDAP/AD sync — available when VAKT_LDAP_URL is configured
	ldapCfg := ldap.Config{
		URL:         cfg.LDAPUrl,
		BindDN:      cfg.LDAPBindDN,
		BindPass:    cfg.LDAPBindPass,
		BaseDN:      cfg.LDAPBaseDN,
		UserFilter:  cfg.LDAPUserFilter,
		GroupFilter: cfg.LDAPGroupFilter,
		TLS:         cfg.LDAPTLS,
	}
	ldap.Register(protected.Group(""), ldapCfg, auth.AuthMiddleware(pasetoKey, pool, rdb))
	log.Info().Msg("ldap routes registered")

	// Demo routes — only active in demo mode
	if cfg.DemoSeed {
		feedback.Register(api, pool, auth.AuthMiddleware(pasetoKey, pool, rdb))
		log.Info().Msg("demo feedback routes registered")

		// Rate-limit POST /demo/start to 10 req per 5 min per IP to prevent DB flood.
		// Uses Redis so the limit is shared across all replicas — an in-memory store
		// would let a client bypass the limit by hitting different pods (SCALE-007).
		// Fails open when Redis is unavailable to keep the demo accessible.
		demoStartRateLimiter := sharedmw.DemoStartRateLimiter(rdb)
		demoStartHandler := demo.NewStartHandler(pool, cfg.SecretKey, authSvc)
		demo.RegisterStart(api.Group("/demo", demoStartRateLimiter), demoStartHandler)
		log.Info().Msg("demo start route registered")
	}

	// LemonSqueezy webhook — kept for backward compat, unauthenticated, signature-verified
	if cfg.LSWebhookSecret != "" && cfg.LicensePrivateKey != "" {
		lsHandler := lswebhook.NewHandler(cfg.LSWebhookSecret, cfg.LicensePrivateKey, lswebhook.SMTPConfig{
			Host: cfg.SMTPHost, Port: cfg.SMTPPort,
			User: cfg.SMTPUser, Pass: cfg.SMTPPass, From: cfg.SMTPFrom,
		}).WithDB(pool).WithRedis(rdb)
		lswebhook.Register(api, lsHandler)
		log.Info().Msg("lemonsqueezy webhook registered")
	}

	// Polar.sh webhook — unauthenticated, signature-verified (POST /api/v1/billing/webhook)
	if cfg.PolarWebhookSecret != "" && cfg.LicensePrivateKey != "" {
		polarHandler := polarwebhook.NewHandler(cfg.PolarWebhookSecret, cfg.LicensePrivateKey, polarwebhook.SMTPConfig{
			Host: cfg.SMTPHost, Port: cfg.SMTPPort,
			User: cfg.SMTPUser, Pass: cfg.SMTPPass, From: cfg.SMTPFrom,
		}).WithDB(pool).WithRedis(rdb)
		polarwebhook.Register(api, polarHandler)
		log.Info().Msg("polar webhook registered at /api/v1/billing/webhook")
	}

	// S46-1: Prometheus metrics — IP-allowlisted (loopback + Docker-internal only).
	// Optionally also token-gated via VAKT_METRICS_TOKEN.
	if cfg.MetricsEnabled {
		metricsToken := os.Getenv("VAKT_METRICS_TOKEN")
		metrics.RegisterWithOptions(e, pool, metrics.RegisterOptions{
			RedisAddr:    redisOpt.Addr,
			MetricsToken: metricsToken,
		})
		log.Info().Msg("metrics endpoint registered")
	}

	// API documentation — Swagger UI + OpenAPI spec
	apidocs.Register(e)
	log.Info().Msg("api docs registered")

	// Client-side error reporting — unauthenticated, rate-limited, best-effort.
	// Receives structured errors from the React ErrorBoundary for ops visibility.
	clientErrRL := middleware.RateLimiterWithConfig(middleware.RateLimiterConfig{
		Store: middleware.NewRateLimiterMemoryStoreWithConfig(
			middleware.RateLimiterMemoryStoreConfig{Rate: 5, Burst: 10, ExpiresIn: time.Minute},
		),
		IdentifierExtractor: func(c echo.Context) (string, error) { return c.RealIP(), nil },
		DenyHandler: func(c echo.Context, _ string, _ error) error {
			return c.NoContent(http.StatusTooManyRequests)
		},
	})
	api.POST("/errors", func(c echo.Context) error {
		var payload struct {
			Message        string `json:"message"`
			Stack          string `json:"stack"`
			ComponentStack string `json:"component_stack"`
			URL            string `json:"url"`
			TraceID        string `json:"trace_id"`
		}
		if err := c.Bind(&payload); err != nil {
			return c.NoContent(http.StatusBadRequest)
		}
		msg := sanitizeLogField(payload.Message, 500)
		url := sanitizeLogField(payload.URL, 512)
		trace := sanitizeLogField(payload.TraceID, 64)
		stack := sanitizeLogField(payload.Stack, 4000)
		compStack := sanitizeLogField(payload.ComponentStack, 4000)
		ua := sanitizeLogField(c.Request().Header.Get("User-Agent"), 300)

		log.Error().
			Str("source", "client").
			Str("url", url).
			Str("trace_id", trace).
			Str("message", msg).
			Str("stack", stack).
			Msg("client-side error boundary triggered")

		// Persist for admin visibility. org_id/user_id are nullable — if the
		// error occurred before login (or auth state was already lost), the
		// entry is still recorded but unscoped.
		var orgID, userID *string
		if v, ok := c.Get("org_id").(string); ok && v != "" {
			orgID = &v
		}
		if v, ok := c.Get("user_id").(string); ok && v != "" {
			userID = &v
		}
		if _, err := pool.Exec(c.Request().Context(), `
			INSERT INTO client_errors
				(org_id, user_id, message, stack, component_stack, url, user_agent, trace_id)
			VALUES
				($1::uuid, $2::uuid, $3, $4, $5, $6, $7, $8)
		`, orgID, userID, msg, stack, compStack, url, ua, trace); err != nil {
			log.Warn().Err(err).Msg("client error: persist failed (logged only)")
		}
		return c.NoContent(http.StatusNoContent)
	}, clientErrRL)
	log.Info().Msg("client error endpoint registered")

	// Admin view of recent client errors (last 200, scoped to org for non-admins).
	protected.GET("/admin/client-errors", func(c echo.Context) error {
		orgID, _ := c.Get("org_id").(string)
		rows, err := pool.Query(c.Request().Context(), `
			SELECT id::text, COALESCE(org_id::text,''), COALESCE(user_id::text,''),
			       message, COALESCE(stack,''), COALESCE(component_stack,''),
			       COALESCE(url,''), COALESCE(user_agent,''), COALESCE(trace_id,''),
			       occurred_at
			FROM client_errors
			WHERE org_id = $1::uuid OR org_id IS NULL
			ORDER BY occurred_at DESC
			LIMIT 200
		`, orgID)
		if err != nil {
			return c.JSON(http.StatusInternalServerError, map[string]string{"error": "query failed"})
		}
		defer rows.Close()
		type errorEntry struct {
			ID             string    `json:"id"`
			OrgID          string    `json:"org_id"`
			UserID         string    `json:"user_id"`
			Message        string    `json:"message"`
			Stack          string    `json:"stack"`
			ComponentStack string    `json:"component_stack"`
			URL            string    `json:"url"`
			UserAgent      string    `json:"user_agent"`
			TraceID        string    `json:"trace_id"`
			OccurredAt     time.Time `json:"occurred_at"`
		}
		out := make([]errorEntry, 0, 50)
		for rows.Next() {
			var e errorEntry
			if err := rows.Scan(&e.ID, &e.OrgID, &e.UserID, &e.Message, &e.Stack,
				&e.ComponentStack, &e.URL, &e.UserAgent, &e.TraceID, &e.OccurredAt); err != nil {
				continue
			}
			out = append(out, e)
		}
		return c.JSON(http.StatusOK, out)
	}, auth.RequireRole("Admin"))

	return e
}

// sanitizeLogField strips ANSI escape codes and non-printable control characters
// from untrusted strings before writing them to structured logs, preventing log injection.
func sanitizeLogField(s string, maxLen int) string {
	if len(s) > maxLen {
		s = s[:maxLen]
	}
	out := make([]byte, 0, len(s))
	i := 0
	for i < len(s) {
		b := s[i]
		if b == 0x1b && i+1 < len(s) && s[i+1] == '[' {
			// Skip ANSI CSI sequence: ESC [ ... <final byte 0x40–0x7E>
			i += 2
			for i < len(s) && (s[i] < 0x40 || s[i] > 0x7e) {
				i++
			}
			i++ // consume final byte
			continue
		}
		if b >= 0x20 || b == '\n' || b == '\r' || b == '\t' {
			out = append(out, b)
		}
		i++
	}
	return string(out)
}

// enabledModuleList returns the list of active modules by parsing the
// VAKT_MODULES_ENABLED config value. Used for startup-diagnostic logging.
func enabledModuleList(cfg *config.Config) []string {
	var out []string
	for _, mod := range strings.Split(cfg.ModulesEnabled, ",") {
		if m := strings.TrimSpace(mod); m != "" {
			out = append(out, m)
		}
	}
	return out
}

func migrationsDir() string {
	_, filename, _, ok := runtime.Caller(0)
	if !ok {
		return "db/migrations"
	}
	return filepath.Join(filepath.Dir(filename), "..", "..", "db", "migrations")
}

func main() {
	log := zerolog.New(os.Stdout).With().Timestamp().Logger()

	// OpenTelemetry — opt-in. With no OTEL_EXPORTER_OTLP_ENDPOINT set, the
	// returned shutdown is a no-op and the operator gets a clear "disabled"
	// log line. See ADR-0011.
	otelShutdown := telemetry.Init(telemetry.FromEnv())
	defer func() {
		_ = otelShutdown(context.Background())
	}()

	cfg, err := config.Load()
	if err != nil {
		log.Fatal().Err(err).Msg("config load failed")
	}

	if version != "dev" {
		cfg.Version = version
	}

	if err := cfg.Validate(); err != nil {
		log.Fatal().Err(err).Msg("configuration error — check .env file")
	}

	if cfg.AutoMigrate && cfg.DBUrl != "" {
		log.Info().Msg("running database migrations")
		if err := shareddb.RunMigrations(cfg.DBUrl, migrationsDir()); err != nil {
			log.Fatal().Err(err).Msg("migration failed")
		}
		log.Info().Msg("migrations complete")
	}

	if cfg.DemoSeed && cfg.DBUrl != "" {
		seedCtx, seedCancel := context.WithTimeout(context.Background(), 30*time.Second)
		seedPool, seedErr := shareddb.Connect(seedCtx, cfg.DBUrl)
		if seedErr == nil {
			if err := demoseed.Run(seedCtx, seedPool, cfg.SecretKey); err != nil {
				log.Warn().Err(err).Msg("demoseed failed — continuing without demo data")
			}
			seedPool.Close()
		}
		seedCancel()
	}

	serverCtx, serverCancel := context.WithCancel(context.Background())
	e := setupEcho(serverCtx, cfg)

	// S46-2: Startup diagnostics — one structured log entry summarising the
	// effective configuration. NEVER log SecretKey, passwords, or tokens.
	log.Info().
		Str("version", cfg.Version).
		Str("ai_provider", cfg.AIProvider).
		Bool("demo_mode", cfg.DemoSeed).
		Bool("smtp_configured", cfg.SMTPHost != "" && cfg.SMTPHost != "localhost").
		Bool("metrics_enabled", cfg.MetricsEnabled).
		Bool("sso_configured", cfg.CasdoorURL != "" && cfg.CasdoorClientID != "").
		Strs("modules", enabledModuleList(cfg)).
		Msg("vakt startup complete")

	if cfg.DemoSeed {
		log.Warn().Msg("demo mode active — ephemeral sessions are open to the public, do NOT use in production")
	}

	if strings.HasPrefix(cfg.FrontendURL, "https://") {
		log.Info().Msg("HTTPS frontend detected — ensure reverse proxy sets X-Forwarded-Proto: https so session cookies get the Secure flag")
	}

	go func() {
		if err := e.Start(":" + cfg.APIPort); err != nil && err != http.ErrServerClosed {
			log.Fatal().Err(err).Msg("server error")
		}
	}()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, os.Interrupt, syscall.SIGTERM)
	<-quit

	serverCancel() // stop background goroutines (e.g. update-check refresh)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := e.Shutdown(ctx); err != nil {
		log.Error().Err(err).Msg("shutdown error")
	}
}
