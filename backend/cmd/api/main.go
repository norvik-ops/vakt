// Copyright (c) 2026 NorvikOps. All rights reserved.
// SPDX-License-Identifier: Elastic-2.0
// See LICENSE file in the project root for terms.

package main

import (
	"context"
	"encoding/hex"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"runtime"
	"syscall"
	"time"

	"github.com/hibiken/asynq"
	"github.com/labstack/echo/v4"
	"github.com/labstack/echo/v4/middleware"
	"github.com/redis/go-redis/v9"
	"github.com/rs/zerolog"
	"golang.org/x/time/rate"

	"github.com/sechealth-app/sechealth/internal/admin"
	"github.com/sechealth-app/sechealth/internal/auth"
	"github.com/sechealth-app/sechealth/internal/config"
	"github.com/sechealth-app/sechealth/internal/license"
	"github.com/sechealth-app/sechealth/internal/shared/demo"
	sharedmw "github.com/sechealth-app/sechealth/internal/shared/middleware"
	"github.com/sechealth-app/sechealth/internal/shared/updatecheck"
	"github.com/sechealth-app/sechealth/internal/modules/hr"
	"github.com/sechealth-app/sechealth/internal/modules/secvitals"
	"github.com/sechealth-app/sechealth/internal/modules/secreflex"
	"github.com/sechealth-app/sechealth/internal/modules/secprivacy"
	"github.com/sechealth-app/sechealth/internal/modules/secvault"
	"github.com/sechealth-app/sechealth/internal/modules/secpulse"
	"github.com/sechealth-app/sechealth/internal/shared/ai"
	"github.com/sechealth-app/sechealth/internal/shared/alerting"
	"github.com/sechealth-app/sechealth/internal/shared/evidence_auto"
	"github.com/sechealth-app/sechealth/internal/shared/apidocs"
	"github.com/sechealth-app/sechealth/internal/shared/auditexport"
	"github.com/sechealth-app/sechealth/internal/shared/auditlog"
	"github.com/sechealth-app/sechealth/internal/shared/auditreport"
	"github.com/sechealth-app/sechealth/internal/shared/dashboard"
	shareddb "github.com/sechealth-app/sechealth/internal/shared/db"
	"github.com/sechealth-app/sechealth/internal/shared/auditor"
	"github.com/sechealth-app/sechealth/internal/shared/demoseed"
	ghintegration "github.com/sechealth-app/sechealth/internal/shared/integrations/github"
	jiraintegration "github.com/sechealth-app/sechealth/internal/shared/integrations/jira"
	cloudintegration "github.com/sechealth-app/sechealth/internal/shared/integrations/cloud"
	"github.com/sechealth-app/sechealth/internal/shared/metrics"
	"github.com/sechealth-app/sechealth/internal/shared/notify"
	"github.com/sechealth-app/sechealth/internal/shared/retention"
	"github.com/sechealth-app/sechealth/internal/shared/search"
	"github.com/sechealth-app/sechealth/internal/shared/setup"
	"github.com/sechealth-app/sechealth/internal/shared/feedback"
	"github.com/sechealth-app/sechealth/internal/shared/ldap"
	"github.com/sechealth-app/sechealth/internal/shared/dataexport"
	"github.com/sechealth-app/sechealth/internal/shared/onboarding"
	"github.com/sechealth-app/sechealth/internal/shared/trustcenter"
	lswebhook "github.com/sechealth-app/sechealth/internal/webhooks/lemonsqueezy"
	"github.com/sechealth-app/sechealth/internal/shared/usermgmt"
	"github.com/sechealth-app/sechealth/internal/shared/apikeys"
	"github.com/sechealth-app/sechealth/internal/shared/comments"
)

// version is injected at build time via -ldflags "-X main.version=..."
var version = "dev"

func setupEcho(cfg *config.Config) *echo.Echo {
	log := zerolog.New(os.Stdout).With().Timestamp().Logger()

	e := echo.New()
	e.HideBanner = true
	e.HidePort = true

	e.Use(middleware.SecureWithConfig(middleware.SecureConfig{
		XSSProtection:         "0",
		ContentTypeNosniff:    "nosniff",
		XFrameOptions:         "DENY",
		HSTSMaxAge:            31536000,
		ContentSecurityPolicy: "default-src 'self'; script-src 'self' 'unsafe-inline'; style-src 'self' 'unsafe-inline'; img-src 'self' data: blob:; font-src 'self' data:; connect-src 'self'; frame-ancestors 'none'; object-src 'none'; base-uri 'self'",
	}))
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
	e.Use(middleware.BodyLimit("10MB"))
	e.Use(middleware.TimeoutWithConfig(middleware.TimeoutConfig{
		Timeout:      30 * time.Second,
		ErrorMessage: `{"error":"request timeout","code":"REQUEST_TIMEOUT"}`,
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
	e.GET("/health", func(c echo.Context) error {
		return c.JSON(http.StatusOK, map[string]interface{}{
			"status":      "ok",
			"version":     cfg.Version,
			"demo":        cfg.DemoSeed,
			"sso_enabled": cfg.CasdoorURL != "",
		})
	})

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

	pasetoKey, err := auth.GenerateSymmetricKey(cfg.SecretKey)
	if err != nil {
		log.Warn().Err(err).Msg("invalid secret key — auth/module routes disabled")
		return e
	}

	// Auth routes — rate-limited (10 req/min per IP), no token middleware (they issue tokens).
	rdb := redis.NewClient(redisOpt)

	// Extend readiness check to include Redis now that rdb is available.
	e.GET("/health/ready", func(c echo.Context) error {
		ctx := c.Request().Context()
		if err := pool.Ping(ctx); err != nil {
			log.Error().Err(err).Msg("health/ready: database ping failed")
			return c.JSON(http.StatusServiceUnavailable, map[string]string{
				"status": "unavailable", "component": "database", "error": "database unavailable",
			})
		}
		if err := rdb.Ping(ctx).Err(); err != nil {
			log.Error().Err(err).Msg("health/ready: redis ping failed")
			return c.JSON(http.StatusServiceUnavailable, map[string]string{
				"status": "unavailable", "component": "redis", "error": "cache unavailable",
			})
		}
		return c.JSON(http.StatusOK, map[string]string{"status": "ready"})
	})

	authRateLimiter := middleware.RateLimiter(middleware.NewRateLimiterMemoryStoreWithConfig(
		middleware.RateLimiterMemoryStoreConfig{Rate: rate.Limit(10.0 / 60.0), Burst: 10, ExpiresIn: 5 * time.Minute},
	))
	authSvc := auth.NewService(pool, rdb, pasetoKey)
	auth.Register(api.Group("/auth", authRateLimiter), auth.NewHandler(authSvc, cfg))
	log.Info().Msg("auth routes registered")

	// All subsequent routes require a valid Paseto token
	protected := api.Group("", auth.AuthMiddleware(pasetoKey, pool, rdb))

	// Org-wide MFA enforcement: if the org has require_mfa=true and the user has
	// not completed TOTP setup, return 403 MFA_REQUIRED on all protected routes
	// except the 2FA setup/confirm flow and logout.
	protected.Use(auth.MFAEnforceMiddleware(pool))

	// Per-request license resolution: load DB key / check revocation blocklist after auth sets org_id.
	// rdb is passed for optional Redis caching (60 s TTL) to avoid 2 DB queries per request.
	protected.Use(license.DBMiddleware(pool, lic, rdb))

	// Global per-org rate limiting: 300 req/min, keyed by org_id from Paseto claims.
	// Must be applied after auth middleware has populated org_id in the context.
	protected.Use(sharedmw.OrgRateLimit())

	// License info — returns current tier and available features; activate endpoint persists key in DB
	license.RegisterRoutes(api, lic, auth.AuthMiddleware(pasetoKey, pool, rdb), pool, rdb)
	log.Info().Msg("license routes registered")

	// Update check service (opt-in, no phone-home)
	updateSvc := updatecheck.NewService(cfg.UpdateCheck, cfg.Version, rdb)
	updatecheck.Register(protected, updateSvc)
	updateSvc.StartBackgroundRefresh(context.Background())
	log.Info().Msg("update check routes registered")

	// Admin routes (also require Admin role)
	asynqClient := asynq.NewClient(asynq.RedisClientOpt{Addr: redisOpt.Addr})
	adminSvc := admin.NewService(pool, cfg.ModulesEnabled, asynqClient)
	adminSvc.WithNotifyService(notify.NewService(pool, cfg))
	adminHealth := admin.NewHealthHandler(pool, rdb, cfg)
	adminHandler := admin.NewHandler(adminSvc).WithPasetoKey(pasetoKey)
	admin.Register(protected, adminHandler, adminHealth, pool, rdb)
	log.Info().Msg("admin routes registered")

	if cfg.Staging {
		admin.RegisterStaging(protected, admin.NewStagingHandler(cfg.PromoteURL, cfg.PromoteSecret))
		log.Info().Msg("staging routes registered")
	}

	// Module routes — all behind auth middleware, sharing the same DB pool
	if cfg.IsModuleEnabled("secpulse") {
		vbSvc := secpulse.NewService(pool, asynq.RedisClientOpt{Addr: redisOpt.Addr})
		vbSvc.WithRedis(rdb)
		secpulse.Register(protected.Group("/secpulse"), secpulse.NewHandler(vbSvc))
		log.Info().Msg("secpulse routes registered")
	}

	auditorRateLimiter := middleware.RateLimiter(middleware.NewRateLimiterMemoryStoreWithConfig(
		middleware.RateLimiterMemoryStoreConfig{Rate: rate.Limit(30.0 / 60.0), Burst: 30, ExpiresIn: 5 * time.Minute},
	))
	auditorAcceptRateLimiter := middleware.RateLimiter(middleware.NewRateLimiterMemoryStoreWithConfig(
		middleware.RateLimiterMemoryStoreConfig{Rate: rate.Limit(10.0 / 60.0), Burst: 10, ExpiresIn: 5 * time.Minute},
	))

	if cfg.IsModuleEnabled("secvitals") {
		ckSvc := secvitals.NewService(pool)
		ckSvc.WithRedis(rdb)
		ckSvc.WithNotifyService(notify.NewService(pool, cfg))
		if cfg.AIProvider != "disabled" && cfg.AIProvider != "" && cfg.AIBaseURL != "" {
			ckSvc.WithAIClient(ai.NewAIClient(cfg.AIBaseURL, cfg.AIAPIKey, cfg.AIModel))
		}
		ckSvc.ReseedBuiltinControls(ctx)
		ckSvc.SeedBuiltinMeasures(ctx)
		if err := ckSvc.SeedFrameworkMappings(ctx); err != nil {
			log.Warn().Err(err).Msg("seed framework mappings failed (non-critical)")
		}
		ckHandler := secvitals.NewHandler(ckSvc).WithDB(pool)
		ckHandler.WithPolicyAcceptanceConfig(secvitals.PolicyAcceptanceHandlerConfig{
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
		efSvc := secvitals.NewEvidenceFileService(ckSvc.Repo(), cfg.UploadDir)
		ckHandler.WithEvidenceFileService(efSvc)
		secvitals.Register(protected.Group("/secvitals"), ckHandler)
		// Auditor portal uses URL token — exempt from Bearer auth; rate-limited to 30 req/min per IP
		portalRateLimiter := middleware.RateLimiter(middleware.NewRateLimiterMemoryStoreWithConfig(
			middleware.RateLimiterMemoryStoreConfig{Rate: rate.Limit(30.0 / 60.0), Burst: 30, ExpiresIn: 5 * time.Minute},
		))
		secvitals.RegisterPublic(api.Group("/secvitals", portalRateLimiter), ckHandler)
		// Policy acceptance — public token routes (no Bearer auth), rate-limited
		secvitals.RegisterPolicyAcceptPublic(api.Group("", portalRateLimiter), ckHandler)
		// Audit package export
		auditexport.Register(protected.Group("/secvitals"), pool)
		// One-click audit report PDF
		auditreport.RegisterRoutes(protected.Group("/secvitals"), pool)
		// AI-generated reports via OpenAI-compatible provider
		ai.Register(protected.Group("/secvitals"), pool, cfg.AIProvider, cfg.AIBaseURL, cfg.AIAPIKey, cfg.AIModel)
		// Auditor portal — read-only secvitals access via session token (no Bearer auth)
		secvitals.RegisterAuditor(api.Group("/auditor/secvitals", auditorRateLimiter, auditor.AuditorAuth(pool)), ckHandler)
		// Auto-evidence inbox — GitHub, SecReflex, SecPulse
		evidence_auto.RegisterRoutes(protected.Group("/secvitals"), pool)
		log.Info().Msg("secvitals routes registered")
	}

	if cfg.IsModuleEnabled("secvault") && cfg.SecretKey != "" {
		masterKeyBytes, err := hex.DecodeString(cfg.SecretKey)
		if err != nil {
			log.Warn().Err(err).Msg("invalid secret key (hex decode) — secvault routes disabled")
		} else {
			soSvc := secvault.NewService(pool, masterKeyBytes, asynqClient)
			secvault.Register(protected.Group("/secvault"), secvault.NewHandler(soSvc))
			log.Info().Msg("secvault routes registered")
		}
	}

	if cfg.IsModuleEnabled("secreflex") {
		pgSvc := secreflex.NewService(pool, secreflex.SMTPConfig{
			Host: cfg.SMTPHost, Port: cfg.SMTPPort,
			User: cfg.SMTPUser, Pass: cfg.SMTPPass, From: cfg.SMTPFrom,
		}, asynq.RedisClientOpt{Addr: redisOpt.Addr})
		secreflex.Register(protected.Group("/secreflex"), secreflex.NewHandler(pgSvc))
		log.Info().Msg("secreflex routes registered")
	}

	// External alerting & webhooks (cross-module) — created before modules that fire events.
	var alertSvc *alerting.Service
	if cfg.SecretKey != "" {
		alertMasterKey, err := hex.DecodeString(cfg.SecretKey)
		if err != nil {
			log.Warn().Err(err).Msg("invalid secret key (hex decode) — alerting routes disabled")
		} else {
			alertSvc = alerting.NewService(pool, alertMasterKey, alerting.SMTPConfig{
				Host: cfg.SMTPHost,
				Port: cfg.SMTPPort,
				User: cfg.SMTPUser,
				Pass: cfg.SMTPPass,
				From: cfg.SMTPFrom,
			})
			alerting.Register(api, pool, alertMasterKey, alerting.SMTPConfig{
				Host: cfg.SMTPHost, Port: cfg.SMTPPort,
				User: cfg.SMTPUser, Pass: cfg.SMTPPass, From: cfg.SMTPFrom,
			}, auth.AuthMiddleware(pasetoKey, pool))
			log.Info().Msg("alerting routes registered")
		}
	}

	if cfg.IsModuleEnabled("secprivacy") {
		poSvc := secprivacy.NewService(pool, asynq.RedisClientOpt{Addr: redisOpt.Addr})
		poHandler := secprivacy.NewHandler(poSvc).WithDB(pool)
		if alertSvc != nil {
			poHandler.WithAlerting(alertSvc.Fire)
		}
		secprivacy.Register(protected.Group("/secprivacy"), poHandler)
		// DSR portal uses URL slug/token — exempt from Bearer auth; rate-limited to 30 req/min per IP
		dsrPortalRateLimiter := middleware.RateLimiter(middleware.NewRateLimiterMemoryStoreWithConfig(
			middleware.RateLimiterMemoryStoreConfig{Rate: rate.Limit(30.0 / 60.0), Burst: 30, ExpiresIn: 5 * time.Minute},
		))
		secprivacy.RegisterPublic(api.Group("/secprivacy", dsrPortalRateLimiter), poHandler)
		log.Info().Msg("secprivacy routes registered")
	}

	// HR module — onboarding and offboarding workflows
	hrHandler := hr.NewHandler(hr.NewService(hr.NewRepository(pool)))
	hr.Register(protected.Group("/hr"), hrHandler)
	log.Info().Msg("hr routes registered")

	// GitHub integration — branch protection, PR review, dependency alert compliance checks
	if cfg.SecretKey != "" {
		ghMasterKey, err := hex.DecodeString(cfg.SecretKey)
		if err != nil {
			log.Warn().Err(err).Msg("invalid secret key (hex decode) — github integration routes disabled")
		} else {
			ghintegration.RegisterRoutes(protected.Group("/integrations/github"), pool, ghMasterKey)
			log.Info().Msg("github integration routes registered")
		}
	}

	// Jira integration — push findings as Jira issues, store config encrypted
	if cfg.SecretKey != "" {
		jiraMasterKey, err := hex.DecodeString(cfg.SecretKey)
		if err != nil {
			log.Warn().Err(err).Msg("invalid secret key (hex decode) — jira integration routes disabled")
		} else {
			jiraintegration.RegisterRoutes(protected.Group("/integrations/jira"), pool, jiraMasterKey)
			log.Info().Msg("jira integration routes registered")
		}
	}

	// Cloud integrations — AWS + Azure automated evidence collection
	if cfg.SecretKey != "" {
		cloudMasterKey, err := hex.DecodeString(cfg.SecretKey)
		if err != nil {
			log.Warn().Err(err).Msg("invalid secret key (hex decode) — cloud integration routes disabled")
		} else {
			cloudintegration.RegisterRoutes(protected.Group("/integrations/cloud"), pool, cloudMasterKey)
			log.Info().Msg("cloud integration routes registered")
		}
	}

	// API key management — personal keys for programmatic access (Pro feature)
	apikeys.Register(protected, pool)
	log.Info().Msg("api key routes registered")

	// Shared comments — threaded discussion on findings and controls
	comments.Register(protected, pool)
	log.Info().Msg("comments routes registered")

	// Audit log — compliance event history
	auditlog.RegisterRoutes(protected.Group("/audit-log"), pool)
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
	dashboard.Register(api.Group("/dashboard"), pool, rdb, auth.AuthMiddleware(pasetoKey, pool))
	log.Info().Msg("dashboard routes registered")

	// Global search — cross-module text search
	search.Register(api, pool, auth.AuthMiddleware(pasetoKey, pool))

	// Retention config API — data-pruning settings per org
	retention.Register(api, pool, auth.AuthMiddleware(pasetoKey, pool))
	log.Info().Msg("retention routes registered")

	// 2FA/TOTP — local account second factor
	if cfg.SecretKey != "" {
		if totpKey, err := hex.DecodeString(cfg.SecretKey); err == nil {
			auth.RegisterTOTP(api.Group("/auth"), pool, totpKey, auth.AuthMiddleware(pasetoKey, pool), authSvc)
			log.Info().Msg("2FA/TOTP routes registered")
		}
	}

	// Session management — list and revoke active sessions
	auth.RegisterSessions(protected.Group("/auth/sessions"), pool)
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
	ldap.Register(protected.Group(""), ldapCfg, auth.AuthMiddleware(pasetoKey, pool))
	log.Info().Msg("ldap routes registered")

	// Demo routes — only active in demo mode
	if cfg.DemoSeed {
		feedback.Register(api, pool, auth.AuthMiddleware(pasetoKey, pool))
		log.Info().Msg("demo feedback routes registered")

		// Rate-limit POST /demo/start to 5 req/min per IP to prevent DB flood.
		demoStartRateLimiter := middleware.RateLimiter(middleware.NewRateLimiterMemoryStoreWithConfig(
			middleware.RateLimiterMemoryStoreConfig{Rate: rate.Limit(5.0 / 60.0), Burst: 5, ExpiresIn: 5 * time.Minute},
		))
		demoStartHandler := demo.NewStartHandler(pool, cfg.SecretKey)
		demo.RegisterStart(api.Group("/demo", demoStartRateLimiter), demoStartHandler)
		log.Info().Msg("demo start route registered")
	}

	// LemonSqueezy webhook — unauthenticated, signature-verified
	if cfg.LSWebhookSecret != "" && cfg.LicensePrivateKey != "" {
		lsHandler := lswebhook.NewHandler(cfg.LSWebhookSecret, cfg.LicensePrivateKey, lswebhook.SMTPConfig{
			Host: cfg.SMTPHost, Port: cfg.SMTPPort,
			User: cfg.SMTPUser, Pass: cfg.SMTPPass, From: cfg.SMTPFrom,
		}).WithDB(pool).WithRedis(rdb)
		lswebhook.Register(api, lsHandler)
		log.Info().Msg("lemonsqueezy webhook registered")
	}

	// Prometheus metrics — no auth, scraped directly
	metrics.Register(e, pool)
	log.Info().Msg("metrics endpoint registered")

	// API documentation — Swagger UI + OpenAPI spec
	apidocs.Register(e)
	log.Info().Msg("api docs registered")

	return e
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

	cfg, err := config.Load()
	if err != nil {
		log.Fatal().Err(err).Msg("config load failed")
	}

	if version != "dev" {
		cfg.Version = version
	}

	if cfg.SecretKey == "" {
		log.Fatal().Msg("VAKT_SECRET_KEY is required. Generate one with: openssl rand -hex 32")
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

	e := setupEcho(cfg)

	go func() {
		if err := e.Start(":" + cfg.APIPort); err != nil && err != http.ErrServerClosed {
			log.Fatal().Err(err).Msg("server error")
		}
	}()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, os.Interrupt, syscall.SIGTERM)
	<-quit

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := e.Shutdown(ctx); err != nil {
		log.Error().Err(err).Msg("shutdown error")
	}
}
