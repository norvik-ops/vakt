// Copyright (c) 2026 NorvikOps. All rights reserved.
// SPDX-License-Identifier: Elastic-2.0
// See LICENSE file in the project root for terms.

package main

import (
	"context"
	"encoding/hex"
	"net/http"
	httppprof "net/http/pprof" // S98-4: pprof handlers, registered on a dedicated mux (not DefaultServeMux)
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/hibiken/asynq"
	"github.com/labstack/echo/v4"
	"github.com/redis/go-redis/v9"
	"github.com/rs/zerolog"

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
	"github.com/matharnica/vakt/internal/shared/clienterrors"
	"github.com/matharnica/vakt/internal/shared/comments"
	sharedcrypto "github.com/matharnica/vakt/internal/shared/crypto"
	"github.com/matharnica/vakt/internal/shared/dashboard"
	"github.com/matharnica/vakt/internal/shared/dataexport"
	shareddb "github.com/matharnica/vakt/internal/shared/db"
	"github.com/matharnica/vakt/internal/shared/demo"
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
	"github.com/matharnica/vakt/internal/shared/updatecheck"
	"github.com/matharnica/vakt/internal/shared/usermgmt"
)

// readinessDBPinger / readinessRedisPinger are the minimal surfaces the
// readiness handler needs, so it can be unit-tested with fakes (S87-4).
// *pgxpool.Pool and *redis.Client satisfy them in production.
type readinessDBPinger interface {
	Ping(ctx context.Context) error
}
type readinessRedisPinger interface {
	Ping(ctx context.Context) *redis.StatusCmd
}

// readinessHandler returns the /health/ready handler. On a DB or Redis failure
// it returns a generic component status (503) — never the raw err.Error(),
// which could leak internal hostnames/ports/driver details to an
// unauthenticated client (S87-4, F-08). The detail is logged for Ops.
func readinessHandler(db readinessDBPinger, rdb readinessRedisPinger, ver string, logger zerolog.Logger) echo.HandlerFunc {
	return func(c echo.Context) error {
		ctx := c.Request().Context()
		dbStart := time.Now()
		if err := db.Ping(ctx); err != nil {
			logger.Error().Err(err).Msg("health/ready: database ping failed")
			return c.JSON(http.StatusServiceUnavailable, map[string]string{
				"status": "unavailable", "component": "database", "error": "database unavailable",
			})
		}
		dbLatencyMs := time.Since(dbStart).Milliseconds()
		redisStart := time.Now()
		if err := rdb.Ping(ctx).Err(); err != nil {
			logger.Error().Err(err).Msg("health/ready: redis ping failed")
			return c.JSON(http.StatusServiceUnavailable, map[string]string{
				"status": "unavailable", "component": "redis", "error": "redis unavailable",
			})
		}
		redisLatencyMs := time.Since(redisStart).Milliseconds()
		return c.JSON(http.StatusOK, map[string]any{
			"status":           "ready",
			"db_latency_ms":    dbLatencyMs,
			"redis_latency_ms": redisLatencyMs,
			"version":          ver,
		})
	}
}

// registerRoutes connects to the database and registers all DB-dependent
// routes — public wizards, auth, module endpoints, admin, webhooks, metrics,
// and pprof. It returns early (without error) when the DB, Redis, or secret
// key prerequisites are not met; the caller's Echo instance is left with only
// the routes that were successfully registered.
func registerRoutes(lifecycleCtx context.Context, e *echo.Echo, internal *echo.Echo, cfg *config.Config, log zerolog.Logger, lic *license.License) {
	ctx := context.Background()
	pool, err := shareddb.Connect(ctx, cfg.DBUrl)
	if err != nil {
		log.Warn().Err(err).Msg("DB unavailable — all routes disabled")
		return
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
	trustcenter.Register(api, pool)
	log.Info().Msg("trust center routes registered")

	// Early Redis init — used by pre-auth rate limiters (nis2/setup) via IPRateLimitRedis
	// which fails open when rdb is nil, so public routes stay up even without Redis.
	var rdb *redis.Client
	var redisOpt *redis.Options
	if cfg.RedisUrl != "" {
		if parsedOpt, parseErr := redis.ParseURL(cfg.RedisUrl); parseErr == nil {
			redisOpt = parsedOpt
			rdb = redis.NewClient(redisOpt)
		}
	}
	// S98-5: let notify.Send push SSE wakeups via Redis Pub/Sub (no-op if rdb nil).
	notify.SetPublisher(rdb)

	// Sprint 19 / S19-1: NIS2-Self-Assessment-Wizard — public, no auth.
	// Rate-limited against abuse (5 req/min/IP). Redis-backed via IPRateLimitRedis
	// (fails open when Redis is unavailable, so the wizard stays reachable).
	nis2RateLimiter := sharedmw.IPRateLimitRedis(rdb, "nis2", 5, 5*time.Minute, true)
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
	setupRateLimiter := sharedmw.IPRateLimitRedis(rdb, "setup", 5, 5*time.Minute, true)
	setupHandler := setup.NewHandler(pool)
	setup.Register(api.Group("/setup", setupRateLimiter), setupHandler)
	log.Info().Msg("setup routes registered")

	if cfg.RedisUrl == "" || cfg.SecretKey == "" {
		log.Warn().Msg("VAKT_REDIS_URL or VAKT_SECRET_KEY not set — auth/module routes disabled")
		return
	}

	if redisOpt == nil {
		log.Warn().Msg("invalid Redis URL — auth/module routes disabled")
		return
	}

	// Decode the raw master key once and derive purpose-specific sub-keys via HKDF.
	// This ensures a compromise of one derived key cannot be extended to others.
	//
	// HISTORY (ADR-0058, S90-1): every service key below is derived — vault, TOTP,
	// alert, GitHub, cloud, webhook AND paseto. Before commit 2f06da9f (v0.25–0.29)
	// these services used the RAW master key directly; that commit switched them to
	// derived keys WITHOUT a re-encryption migration. This was a breaking change for
	// any instance that had persisted raw-key-encrypted secrets, but it is safe in
	// practice because the install base is demo-ephemeral + pentest-local only (no
	// long-lived instance carried pre-2f06da9f ciphertext across the upgrade). See
	// ADR-0058 for the verify result and the documented emergency lazy-upgrade path.
	rawMasterKey, err := hex.DecodeString(cfg.SecretKey)
	if err != nil {
		log.Warn().Err(err).Msg("invalid secret key (hex decode) — auth/module routes disabled")
		return
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
		return
	}

	// Auth routes — rate-limited (5 req/min per IP, S45-5), no token middleware (they issue tokens).

	// S46-3 / S90-8 (#9): re-register /health now that pool + rdb exist. This
	// intentionally overrides the nil-db/rdb fallback registered earlier (search
	// "fallback /health registration"); Echo uses the last handler for the route.
	// Overriding here gives us DB + Redis + AI component statuses.
	e.GET("/health", func(c echo.Context) error {
		return healthHandler(c, cfg, pool, rdb)
	})

	// Extend readiness check to include Redis now that rdb is available.
	e.GET("/health/ready", readinessHandler(pool, rdb, version, log))

	// Auth routes — Redis-backed IP rate limit (5 req/min) on the four
	// credential-submission endpoints, plus a per-IP Redis limiter on the full
	// auth group for burst protection on other endpoints (S45-5, S78-6d).
	authRateLimiter := sharedmw.IPRateLimitRedis(rdb, "auth", 5, time.Minute, false)
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
	if raw := os.Getenv("VAKT_RATELIMIT_IP_MAX"); raw != "" {
		if ipMax, err := strconv.Atoi(raw); err == nil && ipMax > 0 {
			authSvc = authSvc.WithIPLockoutMax(ipMax)
			log.Info().Int("ip_max", ipMax).Msg("auth: custom VAKT_RATELIMIT_IP_MAX configured")
		}
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
	// ValidateUUIDParams sits on the whole protected tree, not on individual module
	// groups: a malformed UUID path param otherwise reaches a ::uuid cast and
	// Postgres 22P02 turns into a 500. Mounting per-group left admin/alerting/
	// account/integrations/webhooks unguarded — one choke point cannot drift.
	// OrgIPAllowlist runs after AuthMiddleware (needs org_id) and enforces the
	// org's admin IP allowlist — mounted once here so every /admin route, across
	// all ~6 admin sub-groups and any added later, is covered without a variant-miss
	// (S131-C2/R-H17). The middleware itself no-ops for non-/admin paths.
	protected := api.Group("", auth.AuthMiddleware(pasetoKey, pool, rdb), sharedmw.ValidateUUIDParams(), sharedmw.OrgIPAllowlist(pool))

	// CSRF protection: double-submit-cookie pattern on state-changing methods.
	// API-key requests (Bearer sk_/vakt_) are exempt because they are not
	// browser-driven. Webhook deliveries from external systems are also exempt
	// (they authenticate via HMAC signature, not cookie). Auth routes that
	// establish a session sit outside `protected` and therefore aren't gated.
	// S131-R-L05: the former "/api/v1/webhooks/receive" exemption was dead — no
	// such route exists (outbound webhook management lives under /api/v1/webhooks
	// and DOES need CSRF; the inbound HMAC-verified Personio webhook is mounted on
	// a public group without this middleware). Removed to close the prefix trap.
	protected.Use(auth.CSRFMiddleware())

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

	// /auth/me is registered after CSRF and MFA middleware so it inherits the full
	// protected chain. It is also listed in mfaExemptPaths (auth/middleware.go) so
	// users can retrieve their own profile during the MFA setup flow.
	protected.GET("/auth/me", authHandler.Me)

	// Licence auto-renewal.
	//
	// ON BY DEFAULT for a Pro instance since 2026-07-12, and the word "default" is
	// doing a lot of work there, so be precise about what it means:
	//
	// The instance contacts api.norvikops.de ONLY in the last quarter of its key's
	// life (capped at a month) — a yearly customer roughly ONCE A YEAR, a monthly one
	// a handful of times per renewal. Outside that window it makes no outbound call to
	// us at all. It sends the licence's renewal token and nothing else: no org data,
	// no user count, no compliance content, no instance identifier.
	//
	// That is a licence renewal, not a heartbeat, and the distinction is not
	// wordplay — it is the difference between one call a year and 365. A daily
	// check-in would have been telemetry wearing a different name, and no self-hosting
	// buyer would have believed otherwise. Rightly.
	//
	// VAKT_LICENSE_AUTORENEW=false switches it off. Then Norvik mails the next key and
	// the instance never speaks to us — a supported path, not a trap. The Community
	// Edition has no key and never calls at all.
	licHandler := license.RegisterRoutes(api, lic, auth.AuthMiddleware(pasetoKey, pool, rdb), pool, rdb)
	if cfg.LicenseAutoRenew {
		licHandler.WithAutoRenewal()
		refresher := license.NewAutoRefresher(cfg.LicenseToken, cfg.LicenseRefreshURL, true, licHandler, pool, rdb)
		go refresher.Start(lifecycleCtx)
	} else {
		log.Info().Msg("license: auto-renewal disabled (VAKT_LICENSE_AUTORENEW=false) — this instance will never contact Norvik")
	}
	log.Info().Msg("license routes registered")

	// Update check service (opt-in, no phone-home)
	updateSvc := updatecheck.NewService(cfg.UpdateCheck, cfg.Version, rdb)
	updatecheck.Register(protected, updateSvc)
	updateSvc.StartBackgroundRefresh(lifecycleCtx)
	log.Info().Msg("update check routes registered")

	// Admin routes (also require Admin role)
	asynqClient := asynq.NewClient(asynq.RedisClientOpt{Addr: redisOpt.Addr, Password: redisOpt.Password})
	adminSvc := admin.NewService(pool, cfg.ModulesEnabled)
	adminSvc.WithNotifyService(notify.NewService(pool, cfg))
	adminSvc.WithMasterKey(rawMasterKey)
	adminHealth := admin.NewHealthHandler(pool, rdb, cfg)
	adminHandler := admin.NewHandler(adminSvc)
	// S90-4: wire Redis so permission changes invalidate the module-permission cache.
	adminHandler.Permissions.WithRedis(rdb)
	admin.Register(protected, adminHandler, adminHealth, pool, rdb)
	internal.GET("/api/v1/internal/backup-config", adminHandler.GetInternalBackupConfig)
	// Job queue stats — admin-only, same auth guard as other admin routes.
	jobsHandler := admin.NewJobsHandler(redisOpt.Addr, redisOpt.Password)
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
	// applied inside scimSvc.Register. authSvc is wired as SessionRevoker so
	// that SCIM-driven deactivations immediately invalidate active tokens (S78-1).
	scimSvc.Register(api.Group("/scim/v2"), pool, authSvc)
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
		vbSvc := vaktscan.NewService(pool, asynq.RedisClientOpt{Addr: redisOpt.Addr, Password: redisOpt.Password})
		vbSvc.WithRedis(rdb)
		vbSvc.WithWebhooks(webhookSvc)
		vaktscan.Register(protected.Group("/vaktscan", auth.RequireModuleAccess(pool, "vaktscan", rdb)), vaktscan.NewHandler(vbSvc))
		log.Info().Msg("vaktscan routes registered")
	}

	// S78-6d: Redis-backed rate limiters (multi-replica safe).
	auditorRateLimiter := sharedmw.IPRateLimitRedis(rdb, "auditor", 30, time.Minute, false)
	auditorAcceptRateLimiter := sharedmw.IPRateLimitRedis(rdb, "auditor_accept", 10, time.Minute, false)

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
			aiClient := ai.NewAIClient(cfg.AIBaseURL, cfg.AIAPIKey, cfg.AIModel)
			if cfg.AIReportTimeoutSeconds > 0 {
				aiClient.WithTimeout(time.Duration(cfg.AIReportTimeoutSeconds) * time.Second)
			}
			ckSvc.WithAIClient(aiClient)
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
		vaktcomply.Register(protected.Group("/vaktcomply", auth.RequireModuleAccess(pool, "vaktcomply", rdb)), ckHandler)
		// Sprint 22 / S22-6: authentifizierter NIS2-Wizard-Migrate-Endpoint
		// (POST /vaktcomply/nis2-assessment/migrate-from-anonymous).
		nis2wizard.RegisterAuthenticated(protected.Group("/vaktcomply", auth.RequireModuleAccess(pool, "vaktcomply", rdb)), nis2wizardHandler)
		// Auditor portal uses URL token — exempt from Bearer auth; rate-limited to 30 req/min per IP
		portalRateLimiter := sharedmw.IPRateLimitRedis(rdb, "portal", 30, time.Minute, false)
		vaktcomply.RegisterPublic(api.Group("/vaktcomply"), ckHandler, portalRateLimiter)
		// Policy acceptance — public token routes (no Bearer auth), rate-limited
		vaktcomply.RegisterPolicyAcceptPublic(api.Group(""), ckHandler, portalRateLimiter)
		// Audit package export
		audit.RegisterExport(protected.Group("/vaktcomply", auth.RequireModuleAccess(pool, "vaktcomply", rdb)), pool)
		// One-click audit report PDF
		audit.RegisterReport(protected.Group("/vaktcomply", auth.RequireModuleAccess(pool, "vaktcomply", rdb)), pool)
		// AI-generated reports via OpenAI-compatible provider.
		// Sprint 15 (S15-1/2/3/5): Rate-Limit + Daily-Quota + Response-Cache
		// + Streaming-SSE-Endpoint laufen über RegisterWithOptions, sofern
		// Redis verfügbar ist.
		aiFailOpen := os.Getenv("VAKT_AI_FAIL_OPEN_ON_OUTAGE") == "true"
		if aiFailOpen {
			log.Warn().Msg("ai: VAKT_AI_FAIL_OPEN_ON_OUTAGE=true — rate-limit + quota checks will fail open during Redis/Postgres outages (audit-relevant choice)")
		}
		ai.RegisterWithOptions(protected.Group("/vaktcomply", auth.RequireModuleAccess(pool, "vaktcomply", rdb)), pool, cfg.AIProvider, cfg.AIBaseURL, cfg.AIAPIKey, cfg.AIModel, ai.RegisterOptions{
			Redis:            rdb,
			RateLimitRPM:     cfg.AIRateLimitRPM,
			DailyTokenLimit:  cfg.AIDailyTokenLimit,
			CacheTTLSeconds:  cfg.AICacheTTLSeconds,
			CostPerMTokenIn:  cfg.AICostPerMTokenIn,
			CostPerMTokenOut: cfg.AICostPerMTokenOut,
			FailOpenOnOutage: aiFailOpen,
		})
		// Auditor portal — read-only vaktcomply access via session token (no Bearer auth).
		// license.DBMiddleware is added so feature gates (FeatureAuditPDF etc.) resolve
		// correctly for the auditor's org without a Paseto token in the request (S78-6c).
		vaktcomply.RegisterAuditor(api.Group("/auditor/vaktcomply", auditorRateLimiter, auditor.AuditorAuth(pool), license.DBMiddleware(pool, lic, rdb)), ckHandler)
		// Auto-evidence inbox — GitHub, SecReflex, SecPulse
		evidence_auto.RegisterRoutes(protected.Group("/vaktcomply", auth.RequireModuleAccess(pool, "vaktcomply", rdb)), pool)
		log.Info().Msg("vaktcomply routes registered")
	}

	if cfg.IsModuleEnabled("vaktvault") && cfg.SecretKey != "" {
		soSvc := vaktvault.NewService(pool, vaultKey, asynqClient)
		vaultHandler := vaktvault.NewHandler(soSvc)
		vaktvault.Register(protected.Group("/vaktvault", auth.RequireModuleAccess(pool, "vaktvault", rdb)), vaultHandler)
		// S127-3 (D6): public share-link consumer route. The external recipient has
		// no session; the route is validated by the URL token (hash-stored) alone.
		vaultShareRL := sharedmw.IPRateLimitRedis(rdb, "vaktvault_share", 30, time.Minute, false)
		vaktvault.RegisterPublic(api.Group("/vaktvault"), vaultHandler, vaultShareRL)
		log.Info().Msg("vaktvault routes registered (protected + public share)")
	}

	if cfg.IsModuleEnabled("vaktaware") {
		pgSvc := vaktaware.NewService(pool, vaktaware.SMTPConfig{
			Host: cfg.SMTPHost, Port: cfg.SMTPPort,
			User: cfg.SMTPUser, Pass: cfg.SMTPPass, From: cfg.SMTPFrom,
		}, asynq.RedisClientOpt{Addr: redisOpt.Addr, Password: redisOpt.Password})
		awareHandler := vaktaware.NewHandler(pgSvc)
		vaktaware.Register(protected.Group("/vaktaware", auth.RequireModuleAccess(pool, "vaktaware", rdb)), awareHandler)
		// S127-1 (D4/D5): public tracking + phish-report. Mounted on the PUBLIC api
		// group — the phishing target's mail client/browser has no session, so these
		// MUST NOT sit behind auth/CSRF/license. Redis-backed IP rate limit (10/min,
		// replica-safe, fail-open) replaces the old per-replica in-memory limiter.
		// Depends on VAKT_TRUSTED_PROXIES for a correct c.RealIP() behind Caddy.
		awareTrackRL := sharedmw.IPRateLimitRedis(rdb, "vaktaware_track", 10, time.Minute, false)
		vaktaware.RegisterPublic(api.Group("/vaktaware"), awareHandler, awareTrackRL)
		log.Info().Msg("vaktaware routes registered (protected + public tracking)")
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
		// S121-B5 (R6): mounted on `protected` for CSRF; channel writes Admin-gated inside Register.
		alerting.Register(protected, pool, alertKey, alerting.SMTPConfig{
			Host: cfg.SMTPHost, Port: cfg.SMTPPort,
			User: cfg.SMTPUser, Pass: cfg.SMTPPass, From: cfg.SMTPFrom,
		})
		log.Info().Msg("alerting routes registered")
	}

	if cfg.IsModuleEnabled("vaktprivacy") {
		poSvc := vaktprivacy.NewService(pool, asynq.RedisClientOpt{Addr: redisOpt.Addr, Password: redisOpt.Password})
		tiaSvc := vaktprivacy.NewTIAService(pool)
		poHandler := vaktprivacy.NewHandler(poSvc).WithDB(pool).WithTIA(tiaSvc)
		if alertSvc != nil {
			poHandler.WithAlerting(alertSvc.Fire)
		}
		vaktprivacy.Register(protected.Group("/vaktprivacy", auth.RequireModuleAccess(pool, "vaktprivacy", rdb)), poHandler)
		// DSR portal uses URL slug/token — exempt from Bearer auth; rate-limited to 30 req/min per IP
		dsrPortalRateLimiter := sharedmw.IPRateLimitRedis(rdb, "dsr_portal", 30, time.Minute, false)
		vaktprivacy.RegisterPublic(api.Group("/vaktprivacy"), poHandler, dsrPortalRateLimiter)
		log.Info().Msg("vaktprivacy routes registered")
	}

	// HR module — onboarding and offboarding workflows (S78-6a: guarded by IsModuleEnabled)
	var hrHandler *vakthr.Handler
	if cfg.IsModuleEnabled("vakthr") {
		hrSvc := vakthr.NewService(vakthr.NewRepository(pool)).
			WithEvidenceWriter(hrEvidence).
			WithAccessReviewTrigger(hrAccessReview).
			WithSessionRevoker(authSvc) // S131-C1: offboarding must kill the access token (pw_version), not just refresh sessions
		hrHandler = vakthr.NewHandler(hrSvc)
		vakthr.Register(protected.Group("/vakthr", auth.RequireModuleAccess(pool, "vakthr", rdb)), hrHandler)
		log.Info().Msg("vakthr routes registered")
	}

	// Account self-service: DSGVO Art. 17 (delete) and Art. 20 (export).
	accountHandler := account.NewHandler(account.NewService(pool))
	account.Register(protected, accountHandler)
	// Sprint 22 S22-11: Login-History-Endpoint.
	account.RegisterLoginHistory(protected, account.NewLoginHistoryHandler(pool))
	log.Info().Msg("account routes registered")

	// GitHub integration — branch protection, PR review, dependency alert compliance checks.
	// S122-B1 (MA-02): Admin-only. Creating an integration stores a GitHub PAT; a
	// SecurityAnalyst was able to POST one and wire a repo to the org. Gate at the group
	// mount, Admin-only (NOT Admin,SecurityAnalyst — the live PoC used a SecurityAnalyst).
	if cfg.SecretKey != "" {
		ghintegration.RegisterRoutes(protected.Group("/integrations/github", auth.RequireRole("Admin")), pool, ghKey)
		log.Info().Msg("github integration routes registered")
	}

	// Cloud integrations — AWS + Azure automated evidence collection.
	// S122-B1 (MA-01): Admin-only. These routes write cloud provider credentials
	// (13 providers × PUT /config). A SecurityAnalyst could store AWS keys. Admin-only.
	if cfg.SecretKey != "" {
		cloudSvc := cloudintegration.RegisterRoutes(protected.Group("/integrations/cloud", auth.RequireRole("Admin")), pool, cloudKey, cloudEvidence)
		log.Info().Msg("cloud integration routes registered")

		// Inject Personio secret provider into the HR handler so the webhook can verify HMAC sigs.
		// Only registered when vakthr is enabled (hrHandler is nil otherwise).
		if hrHandler != nil {
			hrHandler.WithPersonioSecrets(cloudSvc)
			api.POST("/vakthr/webhooks/personio/:org_id", hrHandler.HandlePersonioWebhook)
			log.Info().Msg("personio webhook route registered at /api/v1/vakthr/webhooks/personio/:org_id")
		}
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
	dataexport.RegisterRoutes(protected.Group("/export"), pool, cfg.ModulesEnabled)
	log.Info().Msg("data export routes registered")

	// Auditor portal — invite management (admin) + public accept route
	// Public auditor accept route rate-limited to 30 req/min per IP.
	// S122-B1 (MA-03): Admin-only. POST /auditor/invites issues an external magic-link
	// granting read access to the whole org; a SecurityAnalyst could mint one. Admin-only.
	auditor.RegisterRoutes(protected.Group("/auditor", auth.RequireRole("Admin")), pool)
	auditor.RegisterPublicRoutes(api.Group("/auditor", auditorAcceptRateLimiter), pool)
	log.Info().Msg("auditor routes registered")

	// User management & team invitations
	// Public invite accept route rate-limited to 10 req/min per IP (same as auth).
	inviteRateLimiter := sharedmw.IPRateLimitRedis(rdb, "invite", 10, time.Minute, false)
	umSvc := usermgmt.NewService(pool, usermgmt.SMTPConfig{
		Host: cfg.SMTPHost, Port: cfg.SMTPPort,
		User: cfg.SMTPUser, Pass: cfg.SMTPPass, From: cfg.SMTPFrom,
	}, cfg.FrontendURL).WithSessionRevoker(authSvc) // S78-1: revoke sessions on remove/demote
	usermgmt.RegisterRoutes(protected.Group("/admin"), api.Group("/invite", inviteRateLimiter), umSvc, pool)
	log.Info().Msg("user management routes registered")

	// Onboarding wizard status and dismiss
	onboarding.RegisterRoutes(protected.Group("/onboarding"), pool)
	log.Info().Msg("onboarding routes registered")

	// Trust Center admin — configure public trust page
	trustcenter.RegisterAdmin(protected, pool)
	log.Info().Msg("trust center admin routes registered")

	// Dashboard — shared cross-module score endpoint (aggregate cached in Redis for 60 s).
	// S121-B3 (R3): mounted on `protected` (not bare `api`) so state-changing routes
	// get CSRF/MFA/OrgRL; UpdateScoreConfig is additionally Admin-gated inside Register.
	dashboard.Register(protected.Group("/dashboard"), pool, rdb)
	log.Info().Msg("dashboard routes registered")

	// Global search — cross-module text search
	search.Register(api, pool, auth.AuthMiddleware(pasetoKey, pool, rdb))

	// Retention config API — data-pruning settings per org.
	// S121-B5 (R5): mounted on `protected` for CSRF; UpdateConfig is Admin-gated.
	retention.Register(protected, pool)
	log.Info().Msg("retention routes registered")

	// 2FA/TOTP — local account second factor
	if cfg.SecretKey != "" {
		// Redis-backed rate limiter (5 attempts / 5 min per IP) — shared across
		// replicas and survives restarts, unlike the Echo in-memory store.
		totpRateLimiter := sharedmw.TOTPRateLimit(rdb)
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

	// Billing lives in its own process now (cmd/billing).
	//
	// It was here until 2026-07-12, which meant VAKT_LICENSE_PRIVATE_KEY sat in the
	// environment of THIS process — the one with ~919 routes. Anyone who can sign
	// with that key can mint unlimited valid Pro licences, and they cannot be
	// revoked: Vakt has no phone-home, by design. A single authorisation slip in any
	// one of those routes would have made the product free, permanently, invisibly.
	//
	// So the key moved to a process with six routes. Caddy sends /api/v1/billing/*
	// to it; nothing else about the URL changed, because api.norvikops.de is
	// compiled into every shipped instance (license/autorefresh.go) and can never
	// move.

	// S46-1: Prometheus metrics — IP-allowlisted (loopback + Docker-internal only).
	// Optionally also token-gated via VAKT_METRICS_TOKEN.
	if cfg.MetricsEnabled {
		metricsToken := os.Getenv("VAKT_METRICS_TOKEN")
		metrics.RegisterWithOptions(e, pool, metrics.RegisterOptions{
			RedisAddr:     redisOpt.Addr,
			RedisPassword: redisOpt.Password, // S121-C3 (I1): auth for --requirepass Redis
			MetricsToken:  metricsToken,
		})
		log.Info().Msg("metrics endpoint registered")
	}

	// S98-4: pprof — only when VAKT_PPROF_ENABLED=true.
	// Handlers go on a DEDICATED mux (never DefaultServeMux, so nothing is
	// auto-exposed on the main API server) and the server is bound to
	// 127.0.0.1:6060 only, so it is unreachable from outside the host.
	if os.Getenv("VAKT_PPROF_ENABLED") == "true" {
		pprofMux := http.NewServeMux()
		pprofMux.HandleFunc("/debug/pprof/", httppprof.Index) // serves heap/goroutine/allocs/... by name
		pprofMux.HandleFunc("/debug/pprof/cmdline", httppprof.Cmdline)
		pprofMux.HandleFunc("/debug/pprof/profile", httppprof.Profile)
		pprofMux.HandleFunc("/debug/pprof/symbol", httppprof.Symbol)
		pprofMux.HandleFunc("/debug/pprof/trace", httppprof.Trace)
		pprofSrv := &http.Server{
			Addr:              "127.0.0.1:6060",
			Handler:           pprofMux,
			ReadHeaderTimeout: 5 * time.Second,
		}
		go func() {
			log.Warn().Str("addr", pprofSrv.Addr).Msg("pprof server started — localhost only")
			// nosemgrep: go.lang.security.audit.net.use-tls.use-tls -- loopback-only (127.0.0.1) diagnostic endpoint; TLS unnecessary and adds no security on the local interface
			if err := pprofSrv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
				log.Error().Err(err).Msg("pprof server error")
			}
		}()
	}

	// API documentation — Swagger UI + OpenAPI spec
	apidocs.Register(e)
	log.Info().Msg("api docs registered")

	// Client-side error reporting — unauthenticated, rate-limited, best-effort.
	// Receives structured errors from the React ErrorBoundary for ops visibility.
	// S78-6d: Redis-backed; 5 req/min per IP, fail-open on Redis outage.
	// S90-2: persistence + admin view moved behind clienterrors.Repository/Handler
	// so main.go no longer executes raw SQL.
	clientErrRL := sharedmw.IPRateLimitRedis(rdb, "client_err", 5, time.Minute, false)
	ce := clienterrors.NewHandler(clienterrors.NewRepository(pool))
	api.POST("/errors", ce.Record, clientErrRL)
	protected.GET("/admin/client-errors", ce.List, auth.RequireRole("Admin"))
	log.Info().Msg("client error endpoint registered")
}
