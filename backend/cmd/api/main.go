// Copyright (c) 2026 NorvikOps. All rights reserved.
// SPDX-License-Identifier: Elastic-2.0
// See LICENSE file in the project root for terms.

package main

import (
	"context"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"runtime"
	"strings"
	"syscall"
	"time"

	"github.com/rs/zerolog"

	"github.com/matharnica/vakt/internal/auth"
	"github.com/matharnica/vakt/internal/config"
	"github.com/matharnica/vakt/internal/shared/audit"
	shareddb "github.com/matharnica/vakt/internal/shared/db"
	"github.com/matharnica/vakt/internal/shared/demoseed"
	"github.com/matharnica/vakt/internal/shared/logging"
	"github.com/matharnica/vakt/internal/shared/telemetry"
)

// version is injected at build time via -ldflags "-X main.version=..."
var version = "dev"

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
	logging.ApplyLevelFromEnv()
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

	// S87-5 (F-07): wire the hard Secure-cookie override before any request is served.
	auth.SetForceSecureCookies(cfg.ForceSecureCookies)
	if cfg.ForceSecureCookies {
		log.Info().Msg("VAKT_FORCE_SECURE_COOKIES=true — all session/CSRF cookies will be marked Secure")
	}

	// S88-6: opt-in audit-log Syslog/SIEM forwarder (default off). A bad target
	// is a startup error so misconfiguration surfaces immediately.
	if fwd, fErr := audit.NewSyslogForwarder(audit.SyslogConfigFromEnv()); fErr != nil {
		log.Fatal().Err(fErr).Msg("audit syslog forwarder config invalid")
	} else if fwd != nil {
		audit.SetForwarder(fwd)
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
	e, internal := setupEcho(serverCtx, cfg)

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

	// S98-3: Slowloris hardening — cap slow header/body readers.
	// WriteTimeout=0 because SSE streams (notifications, AI) must not be cut.
	e.Server.ReadHeaderTimeout = 5 * time.Second
	e.Server.ReadTimeout = 15 * time.Second
	e.Server.IdleTimeout = 120 * time.Second
	e.Server.WriteTimeout = 0

	log.Info().Str("port", cfg.InternalPort).Msg("internal server starting")
	go func() {
		if err := internal.Start(":" + cfg.InternalPort); err != nil && err != http.ErrServerClosed {
			log.Fatal().Err(err).Msg("internal server error")
		}
	}()

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
	if err := internal.Shutdown(ctx); err != nil {
		log.Error().Err(err).Msg("internal server shutdown error")
	}
}
