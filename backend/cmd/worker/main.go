// Copyright (c) 2026 NorvikOps. All rights reserved.
// SPDX-License-Identifier: Elastic-2.0
// See LICENSE file in the project root for terms.

package main

import (
	"context"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"syscall"
	"time"

	"github.com/hibiken/asynq"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/redis/go-redis/v9"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"

	"github.com/matharnica/vakt/internal/admin"
	"github.com/matharnica/vakt/internal/auth"
	"github.com/matharnica/vakt/internal/config"
	"github.com/matharnica/vakt/internal/modules/vaktcomply"
	"github.com/matharnica/vakt/internal/services/siem"
	"github.com/matharnica/vakt/internal/shared/audit"
	"github.com/matharnica/vakt/internal/shared/bsi"
	"github.com/matharnica/vakt/internal/shared/demo"
	"github.com/matharnica/vakt/internal/shared/emaildigest"
	"github.com/matharnica/vakt/internal/shared/logging"
	"github.com/matharnica/vakt/internal/shared/metrics"
	"github.com/matharnica/vakt/internal/shared/nis2wizard"
	"github.com/matharnica/vakt/internal/shared/notifications"
	"github.com/matharnica/vakt/internal/shared/notify"
	cloudintegration "github.com/matharnica/vakt/internal/shared/platform/integrations/cloud"
	"github.com/matharnica/vakt/internal/shared/redisopt"
	"github.com/matharnica/vakt/internal/shared/retention"
	"github.com/matharnica/vakt/internal/shared/scheduledreports"
)

// workerConcurrency returns the Asynq concurrency from env (VAKT_WORKER_CONCURRENCY),
// defaulting to 8.
func workerConcurrency() int {
	if v := os.Getenv("VAKT_WORKER_CONCURRENCY"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 {
			return n
		}
	}
	return 8
}

// newMetricsRedis opens a Redis client used by the metrics middleware.
// Returns nil when the URL is empty or invalid — metrics recording is then
// silently skipped (we never want a metric write to break the worker).
func newMetricsRedis(redisURL string) *redis.Client {
	if redisURL == "" {
		return nil
	}
	opt, err := redis.ParseURL(redisURL)
	if err != nil {
		// Fall back to bare host:port form.
		opt = &redis.Options{Addr: redisURL}
	}
	return redis.NewClient(opt)
}

// R1-14b-01: die Ableitung von VAKT_REDIS_URL zu den Asynq-Optionen stand
// frueher hier als lokale Funktion asynqRedisOpt — und noch einmal, ohne DB, an
// vier Stellen in cmd/api/routes.go. Zwei Ableitungen sind zwei Wahrheiten; wer
// eine davon anfasst, hat die andere nicht in der Hand. Sie liegt jetzt in
// internal/shared/redisopt und wird von beiden Binaries benutzt.

func buildServer(pool *pgxpool.Pool) (*asynq.Server, *asynq.ServeMux) {
	cfg, _ := config.Load()

	redisOpt := redisopt.AsynqFromURL(cfg.RedisUrl)

	srv := asynq.NewServer(
		redisOpt,
		asynq.Config{
			Concurrency: workerConcurrency(),
			Queues: map[string]int{
				vaktcomply.Queue: 5, // evidence collection, deadline checks
				// Generic queues kept for backward compat with external enqueues.
				"critical": 10,
				"default":  5,
				"low":      3,
				// Queues of the archived modules (docs/stories/module-archivieren.md).
				// Still served at the lowest weight so tasks queued before an update
				// are drained: moduleGuardMiddleware drops them instead of leaving
				// them in Redis forever.
				"vaktscan":    1,
				"maintenance": 1,
				"vaktprivacy": 1,
				"vaktaware":   1,
				"vaktvault":   1,
				"vakthr":      1,
			},
		},
	)

	mux := asynq.NewServeMux()

	// R1-W19-10: classify terminal (non-retryable) errors so they are archived
	// once instead of retried up to 25× (deleted row, FK violation, malformed
	// UUID). Outermost so its SkipRetry rewrite reaches the processor. See
	// terminal_errors.go.
	mux.Use(terminalErrorMiddleware)

	// Tasks of archived modules still queued in Redis are dropped before any handler runs.
	mux.Use(archivedTaskMiddleware)

	// S58-1: emit per-task-type duration + result into Redis so /metrics on the
	// API side can publish Prometheus counters without needing a worker scrape
	// endpoint. Best-effort — Redis failures never affect task execution.
	if rdb := newMetricsRedis(cfg.RedisUrl); rdb != nil {
		mux.Use(metrics.AsynqInstrumentingMiddleware(rdb))
		// S98-5: worker-generated notifications also push SSE wakeups.
		notify.SetPublisher(rdb)
	}

	// ── SecVitals: daily control-owner due-date reminder ─────────────────────
	mux.HandleFunc(taskControlOwnerReminder, handleControlOwnerReminder(cfg, pool))

	// ── GitHub CI evidence sync (daily) ──────────────────────────────────────
	mux.HandleFunc(taskGitHubCISync, handleGitHubCISync(cfg, pool))

	// Demo cleanup — hourly pruning of ephemeral demo orgs older than 4 hours
	mux.HandleFunc(demo.TaskCleanupEphemeralOrgs, handleDemoCleanup(pool))

	// Data retention — daily pruning of expired records
	mux.HandleFunc(retention.TaskRetentionRun, handleRetentionRun(pool))
	mux.HandleFunc(audit.TaskPartitionMaint, handlePartitionMaint(pool))

	// Weekly e-mail digest — Monday morning status summary
	mux.HandleFunc(emaildigest.TaskWeeklyDigest, handleWeeklyDigest(cfg, pool))

	// BSI CERT-Bund feed — daily advisory sync + asset matching
	mux.HandleFunc(bsi.TaskBSIFeedSync, handleBSIFeedSync(pool))

	// SecVitals: daily evidence expiry alert
	mux.HandleFunc(vaktcomply.TaskEvidenceExpiryAlert, handleEvidenceExpiryAlert(pool))

	// SecVitals: periodic DORA/NIS2 incident deadline check
	mux.HandleFunc(vaktcomply.TaskIncidentDeadlineCheck, handleIncidentDeadlineCheck(cfg, pool))

	// SecVitals: DORA Ampel-Status update every 5 minutes (S37-4)
	mux.HandleFunc(vaktcomply.TaskDORADeadlineStatus, handleDORADeadlineStatus(cfg, pool))

	// SecVitals: daily NIS2 classified-incident deadline check (S39-2)
	mux.HandleFunc(vaktcomply.TaskNIS2ObligationCheck, handleNIS2ObligationCheck(cfg, pool))

	// SecVitals: daily supplier certificate expiry check
	mux.HandleFunc(vaktcomply.TaskCertExpiryCheck, handleCertExpiryCheck(cfg, pool))

	// SecVitals: daily overdue control test CAPA creation
	mux.HandleFunc(taskControlTestCheck, handleControlTestCheck(cfg, pool))

	// Error budget: weekly SLO compliance report
	mux.HandleFunc(taskErrorBudgetReport, handleErrorBudgetReport(pool))

	// SecVitals: CCM — run all due automated control checks
	mux.HandleFunc(vaktcomply.TaskCCMRunDue, handleCCMRunDue(cfg, pool))

	// SecVitals: daily compliance score snapshot for trend charts
	mux.HandleFunc(vaktcomply.TaskScoreSnapshot, handleScoreSnapshot(cfg, pool))

	// Notifications: daily compliance deadline email alerts
	mux.HandleFunc(notifications.TaskNotifyDeadlines, handleNotifyDeadlines(cfg, pool))

	// Notifications: external delivery (Slack/Teams/email/webhook). The enqueue
	// side (notify.Service.Notify) shipped for years with NO registered handler,
	// so every notification hit "handler not found", retried 25×, then archived
	// while its DB row stayed 'pending' forever (R1-19-W01). This is the consumer.
	mux.HandleFunc(notify.NotificationJobType, notify.DeliverHandler(pool, notify.DefaultSenders(cfg)))

	// Admin: daily revocation of expired SCIM tokens
	mux.HandleFunc(admin.TaskSCIMTokenExpiry, handleSCIMTokenExpiry(pool))

	// Auth: daily cleanup of expired and used password reset tokens
	mux.HandleFunc(auth.TaskCleanupPasswordResetTokens, handleCleanupPasswordResetTokens(pool))

	// Auth: daily cleanup of expired deny-list fallback entries (S31-4)
	mux.HandleFunc(auth.TaskCleanupDenyListFallback, handleCleanupDenyListFallback(pool))

	// Sprint 22 S22-12 + S22-13: Cleanup-Jobs für NIS2-Wizard-anonyme-Runs
	// (täglich) und Login-History > 90 Tage (wöchentlich).
	mux.HandleFunc(nis2wizard.TaskCleanupAnonymousRuns, handleCleanupNIS2AnonymousRuns(pool))
	mux.HandleFunc(auth.TaskCleanupLoginHistory, handleCleanupLoginHistory(pool))

	// Cloud integrations: daily evidence collection from AWS + Azure
	mux.HandleFunc(cloudintegration.TaskCloudSync, handleCloudSync(cfg, pool))

	// Scheduled reports: daily delivery run
	mux.HandleFunc(scheduledreports.TaskProcessScheduledReports, handleProcessScheduledReports(cfg, pool))

	// Queue health: every 5 minutes — warn when failed/archived jobs accumulate
	mux.HandleFunc(taskQueueHealthCheck, handleQueueHealthCheck(cfg))

	// SIEM forward: every 5 minutes — forward pending audit entries to configured SIEM
	mux.HandleFunc(siem.TaskSIEMForward, handleSIEMForward(pool))

	// S52-1: daily evidence freshness AI insight generation
	mux.HandleFunc(vaktcomply.TaskEvidenceFreshnessCheck, handleEvidenceFreshnessCheck(cfg, pool))

	// S52-4: Monday AI compliance weekly digest
	mux.HandleFunc(vaktcomply.TaskAIWeeklyDigest, handleAIWeeklyDigest(cfg, pool))

	// S61-3: daily effectiveness-check overdue alert for major_nc CAPAs
	mux.HandleFunc(taskEffectivenessCheckOverdueAlert, handleEffectivenessCheckOverdueAlert(pool))

	// S61-7: daily ISMS KPI snapshot
	mux.HandleFunc(vaktcomply.TaskISMSKPISnapshot, handleISMSKPISnapshot(cfg, pool))

	// S67-4: daily evidence staleness sweep
	mux.HandleFunc(vaktcomply.TaskEvidenceStalenessCheck, handleEvidenceStalenessCheck(cfg, pool))

	// S74-2: daily BSI check progress snapshot
	mux.HandleFunc(vaktcomply.TaskBSIKPISnapshot, handleBSIKPISnapshot(cfg, pool))

	// S86-4: daily BCM evidence sync for DER.4 controls
	mux.HandleFunc(vaktcomply.TaskBCMEvidenceSync, handleBCMEvidenceSync(cfg, pool))

	// S88-2: daily backup-freshness check (ISO A.8.13 / DER.4 evidence + staleness)
	mux.HandleFunc(vaktcomply.TaskBackupFreshnessCheck, handleBackupFreshnessCheck(cfg, pool))

	return srv, mux
}

func main() {
	logging.ApplyLevelFromEnv()
	logger := zerolog.New(os.Stdout).With().Timestamp().Logger()

	cfg, err := config.Load()
	if err != nil {
		logger.Fatal().Err(err).Msg("config load failed")
	}

	if err := cfg.Validate(); err != nil {
		logger.Fatal().Err(err).Msg("configuration error — check .env file")
	}
	// Scan, Vault, Aware, Privacy and HR are archived; naming them no longer
	// enables anything. One warning so an old configuration is noticed.
	if archived := cfg.ArchivedModulesRequested(); len(archived) > 0 {
		logger.Warn().Strs("modules", archived).
			Msg("VAKT_MODULES_ENABLED nennt archivierte Module — sie werden ignoriert (docs/UPGRADE.md)")
	}

	// S88-6: opt-in audit Syslog/SIEM forwarder — the worker also emits audit
	// entries (cross-module evidence, cleanup jobs), so it shares the forwarder.
	if fwd, fErr := audit.NewSyslogForwarder(audit.SyslogConfigFromEnv()); fErr != nil {
		logger.Fatal().Err(fErr).Msg("audit syslog forwarder config invalid")
	} else if fwd != nil {
		audit.SetForwarder(fwd)
	}

	// S46-2: Worker startup diagnostics — mirror of api/main.go summary log.
	// NEVER log SecretKey, passwords, or tokens.
	logger.Info().
		Str("version", cfg.Version).
		Bool("demo_mode", cfg.DemoSeed).
		Bool("epss_enabled", cfg.EPSSEnabled).
		Int("worker_concurrency", workerConcurrency()).
		Msg("vakt worker startup complete")

	// Open a single shared DB pool for the entire worker process lifetime.
	// All handler closures receive this pool — no per-job reconnects.
	ctx := context.Background()
	pool, err := connectDB(ctx, cfg)
	if err != nil {
		logger.Fatal().Err(err).Msg("failed to connect to database")
	}
	defer pool.Close()

	// Log EPSS opt-in status so operators know whether external enrichment is active.
	if !cfg.EPSSEnabled {
		log.Info().Msg("EPSS enrichment disabled — set VAKT_EPSS_ENABLED=true to enable (sends CVE IDs to api.first.org)")
	}

	srv, mux := buildServer(pool)
	scheduler := buildScheduler(cfg)

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, os.Interrupt, syscall.SIGTERM)

	// Health server — port is configurable via WORKER_HEALTH_PORT (default 9090).
	// See health.go for the endpoint set.
	// Audit P1-5 closure: the readiness probe now covers Asynq queue
	// reachability, not just the DB ping.
	healthAddr := ":9090"
	if p := os.Getenv("WORKER_HEALTH_PORT"); p != "" {
		healthAddr = ":" + p
	}
	go func() {
		inspector := asynq.NewInspector(redisopt.AsynqFromURL(cfg.RedisUrl))
		defer func() { _ = inspector.Close() }()
		healthSrv := &http.Server{
			Addr:         healthAddr,
			Handler:      buildHealthHandlers(pool, inspector),
			ReadTimeout:  5 * time.Second,
			WriteTimeout: 5 * time.Second,
		}
		if err := healthSrv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			logger.Error().Err(err).Msg("worker health server error")
		}
	}()

	go func() {
		if err := srv.Run(mux); err != nil {
			logger.Fatal().Err(err).Msg("worker error")
		}
	}()

	go func() {
		if err := scheduler.Run(); err != nil {
			logger.Fatal().Err(err).Msg("scheduler error")
		}
	}()

	<-quit
	logger.Info().Msg("shutting down worker")
	srv.Shutdown()
	scheduler.Shutdown()
}
