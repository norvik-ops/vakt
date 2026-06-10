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
	"github.com/matharnica/vakt/internal/modules/vaktaware"
	"github.com/matharnica/vakt/internal/modules/vaktcomply"
	"github.com/matharnica/vakt/internal/modules/vakthr"
	"github.com/matharnica/vakt/internal/modules/vaktprivacy"
	"github.com/matharnica/vakt/internal/modules/vaktscan"
	"github.com/matharnica/vakt/internal/modules/vaktvault"
	"github.com/matharnica/vakt/internal/services/alerting"
	"github.com/matharnica/vakt/internal/services/crossevidence"
	"github.com/matharnica/vakt/internal/services/siem"
	"github.com/matharnica/vakt/internal/shared/bsi"
	"github.com/matharnica/vakt/internal/shared/demo"
	"github.com/matharnica/vakt/internal/shared/emaildigest"
	"github.com/matharnica/vakt/internal/shared/metrics"
	"github.com/matharnica/vakt/internal/shared/nis2wizard"
	"github.com/matharnica/vakt/internal/shared/notifications"
	cloudintegration "github.com/matharnica/vakt/internal/shared/platform/integrations/cloud"
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

// asynqRedisOpt converts VAKT_REDIS_URL to an asynq.RedisClientOpt.
// Accepts both full URLs (redis://host:port) and bare addr (host:port).
func asynqRedisOpt(redisURL string) asynq.RedisClientOpt {
	if redisURL == "" {
		return asynq.RedisClientOpt{Addr: "localhost:6379"}
	}
	if parsed, err := redis.ParseURL(redisURL); err == nil {
		return asynq.RedisClientOpt{
			Addr:     parsed.Addr,
			Password: parsed.Password,
			DB:       parsed.DB,
		}
	}
	// Bare host:port (no scheme) — pass through directly.
	return asynq.RedisClientOpt{Addr: redisURL}
}

func buildServer(pool *pgxpool.Pool) (*asynq.Server, *asynq.ServeMux) {
	cfg, _ := config.Load()

	redisOpt := asynqRedisOpt(cfg.RedisUrl)

	srv := asynq.NewServer(
		redisOpt,
		asynq.Config{
			Concurrency: workerConcurrency(),
			Queues: map[string]int{
				// Module-dedicated queues (S31-3): each module has its own namespace so
				// a long-running scan batch cannot starve breach-notification or evidence jobs.
				vaktscan.QueueScans: 8, // scanner jobs — highest module concurrency
				vaktcomply.Queue:    5, // evidence collection, deadline checks
				vaktprivacy.Queue:   5, // breach notifications, AVV checks
				vaktaware.Queue:     3, // campaign send, training reminders
				// Generic queues kept for backward compat with external enqueues.
				"critical":                10,
				"default":                 5,
				"low":                     3,
				vaktscan.QueueMaintenance: 2, // SBOM generation and EOL checks
			},
		},
	)

	mux := asynq.NewServeMux()

	// S58-1: emit per-task-type duration + result into Redis so /metrics on the
	// API side can publish Prometheus counters without needing a worker scrape
	// endpoint. Best-effort — Redis failures never affect task execution.
	if rdb := newMetricsRedis(cfg.RedisUrl); rdb != nil {
		mux.Use(metrics.AsynqInstrumentingMiddleware(rdb))
	}

	// ── SecPulse scan handlers ────────────────────────────────────────────────
	mux.HandleFunc(vaktscan.TaskScanTrivy, handleScanJob(cfg, pool))
	mux.HandleFunc(vaktscan.TaskScanNuclei, handleScanJob(cfg, pool))
	mux.HandleFunc(vaktscan.TaskScanOpenVAS, handleScanJob(cfg, pool))

	// ── SecVitals: daily control-owner due-date reminder ─────────────────────
	mux.HandleFunc(taskControlOwnerReminder, handleControlOwnerReminder(cfg, pool))

	// ── GitHub CI evidence sync (daily) ──────────────────────────────────────
	mux.HandleFunc(taskGitHubCISync, handleGitHubCISync(cfg, pool))

	// ── SecPulse EPSS enrichment (daily) ─────────────────────────────────────
	mux.HandleFunc(vaktscan.TaskEPSSEnrich, handleEPSSEnrich(cfg, pool))
	mux.HandleFunc(vaktscan.TaskRiskTrendSnapshot, handleRiskTrendSnapshot(pool))

	// ── SecPulse report generation ────────────────────────────────────────────
	mux.HandleFunc(vaktscan.TaskGenerateReport, handleGenerateReport(cfg, pool))

	// ── SecReflex campaign send ───────────────────────────────────────────────
	mux.HandleFunc(vaktaware.TaskSendCampaign, handleSendCampaign(cfg, pool))

	// ── SecReflex training reminder ───────────────────────────────────────────
	mux.HandleFunc(vaktaware.TaskTrainingReminder, handleTrainingReminder(cfg, pool))

	// ── SecReflex auto-enrollment (S65-2) ─────────────────────────────────────
	mux.HandleFunc(vaktaware.TaskAutoEnrollment, handleAutoEnrollment(pool))

	// ── SecReflex BSI ORP.3 evidence sync (S65-4) ────────────────────────────
	mux.HandleFunc(vaktaware.TaskORP3EvidenceSync, handleORP3EvidenceSync(pool))

	// ── SecVault git scanning ─────────────────────────────────────────────────
	mux.HandleFunc(vaktvault.TaskGitScan, handleGitScan(cfg, pool))

	// ── SecPrivacy: AVV expiry check ──────────────────────────────────────────
	mux.HandleFunc(vaktprivacy.TaskAVVExpiryCheck, handleAVVExpiryCheck(cfg, pool))

	// ── SecPrivacy→SecVitals: breach → incident ───────────────────────────────
	mux.HandleFunc(vaktprivacy.TaskBreachIncidentCreate, handleBreachIncidentCreate(cfg, pool))

	// ── S68-2: daily DSR overdue marking + 3-day warning notifications ────────
	mux.HandleFunc(vaktprivacy.TaskDSRDeadlineCheck, handleDSRDeadlineCheck(pool))

	// ── S68-5: daily deletion reminder notifications (14-day window) ──────────
	mux.HandleFunc(vaktprivacy.TaskDeletionReminderCheck, handleDeletionReminderCheck(pool))

	// ── SecPulse→SecVitals: resolved finding → patch-control evidence ─────────
	mux.HandleFunc(vaktscan.TaskAutoEvidence, handleAutoEvidence(cfg, pool))

	// ── SecPulse SBOM generation (syft) ───────────────────────────────────────
	mux.HandleFunc(vaktscan.TaskSBOMGenerate, handleSBOMGenerate(cfg, pool))

	// ── SecPulse EOL check (endoflife.date) ───────────────────────────────────
	mux.HandleFunc(vaktscan.TaskEOLCheck, handleEOLCheck(cfg, pool))

	// ── SecPulse TLS certificate rescan (daily) ────────────────────────────
	mux.HandleFunc(vaktscan.TaskCertScan, handleCertScan(pool))
	// ── S69-3: SLA check (daily) ──────────────────────────────────────────
	mux.HandleFunc(vaktscan.TaskSLACheck, handleSLACheck(pool))

	// ── Alerting: scheduled overdue checks ────────────────────────────────────
	mux.HandleFunc(alerting.TaskSLAOverdueCheck, handleSLAOverdueCheck(cfg, pool))
	mux.HandleFunc(alerting.TaskDSROverdueCheck, handleDSROverdueCheck(cfg, pool))

	// Demo cleanup — hourly pruning of ephemeral demo orgs older than 4 hours
	mux.HandleFunc(demo.TaskCleanupEphemeralOrgs, handleDemoCleanup(pool))

	// Data retention — daily pruning of expired records
	mux.HandleFunc(retention.TaskRetentionRun, handleRetentionRun(pool))

	// Weekly e-mail digest — Monday morning status summary
	mux.HandleFunc(emaildigest.TaskWeeklyDigest, handleWeeklyDigest(cfg, pool))

	// BSI CERT-Bund feed — daily advisory sync + asset matching
	mux.HandleFunc(bsi.TaskBSIFeedSync, handleBSIFeedSync(pool))

	// Cross-module evidence — training/DSR/rotation events → SecVitals controls
	mux.HandleFunc(crossevidence.TaskRecordEvidence, handleRecordEvidence(pool))

	// SecVitals: daily evidence expiry alert
	mux.HandleFunc(vaktcomply.TaskEvidenceExpiryAlert, handleEvidenceExpiryAlert(pool))

	// SecVitals: periodic DORA/NIS2 incident deadline check
	mux.HandleFunc(vaktcomply.TaskIncidentDeadlineCheck, handleIncidentDeadlineCheck(pool))

	// SecVitals: DORA Ampel-Status update every 5 minutes (S37-4)
	mux.HandleFunc(vaktcomply.TaskDORADeadlineStatus, handleDORADeadlineStatus(pool))

	// SecVitals: daily NIS2 classified-incident deadline check (S39-2)
	mux.HandleFunc(vaktcomply.TaskNIS2ObligationCheck, handleNIS2ObligationCheck(pool))

	// SecVitals: daily supplier certificate expiry check
	mux.HandleFunc(vaktcomply.TaskCertExpiryCheck, handleCertExpiryCheck(pool))

	// SecVitals: daily overdue control test CAPA creation
	mux.HandleFunc(taskControlTestCheck, handleControlTestCheck(pool))

	// Error budget: weekly SLO compliance report
	mux.HandleFunc(taskErrorBudgetReport, handleErrorBudgetReport(pool))

	// SecVitals: CCM — run all due automated control checks
	mux.HandleFunc(vaktcomply.TaskCCMRunDue, handleCCMRunDue(pool))

	// SecVitals: daily compliance score snapshot for trend charts
	mux.HandleFunc(vaktcomply.TaskScoreSnapshot, handleScoreSnapshot(pool))

	// Notifications: daily compliance deadline email alerts
	mux.HandleFunc(notifications.TaskNotifyDeadlines, handleNotifyDeadlines(cfg, pool))

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

	// S52-5: per-finding AI evidence suggestion (enqueued when finding resolved)
	mux.HandleFunc(vaktcomply.TaskAIEvidenceSuggestion, handleAIEvidenceSuggestion(cfg, pool))

	// S52-4: Monday AI compliance weekly digest
	mux.HandleFunc(vaktcomply.TaskAIWeeklyDigest, handleAIWeeklyDigest(cfg, pool))

	// S61-3: daily effectiveness-check overdue alert for major_nc CAPAs
	mux.HandleFunc(taskEffectivenessCheckOverdueAlert, handleEffectivenessCheckOverdueAlert(pool))

	// S61-7: daily ISMS KPI snapshot
	mux.HandleFunc(vaktcomply.TaskISMSKPISnapshot, handleISMSKPISnapshot(pool))

	// S67-4: daily evidence staleness sweep
	mux.HandleFunc(vaktcomply.TaskEvidenceStalenessCheck, handleEvidenceStalenessCheck(pool))

	// S70-4: daily contractor expiry check
	mux.HandleFunc(vakthr.TaskContractorExpiryCheck, handleContractorExpiryCheck(pool))
	// S70-5: quarterly vault access review creation
	mux.HandleFunc(vaktvault.TaskQuarterlyAccessReview, handleQuarterlyAccessReview(pool))

	return srv, mux
}

// handleScanJob returns an asynq handler that runs the appropriate scanner
// (trivy, nuclei, or openvas) based on the task type.

func EnqueueScanTask(client *asynq.Client, taskType string, payload []byte) error {
	queue := "default"
	if taskType == vaktscan.TaskScanOpenVAS {
		queue = "low"
	}
	task := asynq.NewTask(taskType, payload,
		asynq.MaxRetry(3),
		asynq.Timeout(10*time.Minute),
	)
	_, err := client.Enqueue(task, asynq.Queue(queue))
	return err
}

func main() {
	logger := zerolog.New(os.Stdout).With().Timestamp().Logger()

	cfg, err := config.Load()
	if err != nil {
		logger.Fatal().Err(err).Msg("config load failed")
	}

	if err := cfg.Validate(); err != nil {
		logger.Fatal().Err(err).Msg("configuration error — check .env file")
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

	// Health server on :9090 — see health.go for the endpoint set.
	// Audit P1-5 closure: the readiness probe now covers Asynq queue
	// reachability, not just the DB ping.
	go func() {
		inspector := asynq.NewInspector(asynqRedisOpt(cfg.RedisUrl))
		defer func() { _ = inspector.Close() }()
		healthSrv := &http.Server{
			Addr:         ":9090",
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
