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
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"

	"github.com/matharnica/vakt/internal/auth"
	"github.com/matharnica/vakt/internal/config"
	"github.com/matharnica/vakt/internal/modules/secprivacy"
	"github.com/matharnica/vakt/internal/modules/secpulse"
	"github.com/matharnica/vakt/internal/modules/secreflex"
	"github.com/matharnica/vakt/internal/modules/secvault"
	"github.com/matharnica/vakt/internal/modules/secvitals"
	"github.com/matharnica/vakt/internal/shared/alerting"
	"github.com/matharnica/vakt/internal/shared/bsi"
	"github.com/matharnica/vakt/internal/shared/crossevidence"
	"github.com/matharnica/vakt/internal/shared/demo"
	"github.com/matharnica/vakt/internal/shared/emaildigest"
	cloudintegration "github.com/matharnica/vakt/internal/shared/integrations/cloud"
	"github.com/matharnica/vakt/internal/shared/notifications"
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

func buildServer(pool *pgxpool.Pool) (*asynq.Server, *asynq.ServeMux) {
	cfg, _ := config.Load()
	redisAddr := "localhost:6379"
	if cfg != nil && cfg.RedisUrl != "" {
		redisAddr = cfg.RedisUrl
	}

	srv := asynq.NewServer(
		asynq.RedisClientOpt{Addr: redisAddr},
		asynq.Config{
			Concurrency: workerConcurrency(),
			Queues: map[string]int{
				"critical":    5,
				"default":     10,
				"low":         3,
				"maintenance": 2, // SBOM generation and EOL checks — lower priority than user-facing jobs
			},
		},
	)

	mux := asynq.NewServeMux()

	// ── SecPulse scan handlers ────────────────────────────────────────────────
	mux.HandleFunc(secpulse.TaskScanTrivy, handleScanJob(cfg, pool))
	mux.HandleFunc(secpulse.TaskScanNuclei, handleScanJob(cfg, pool))
	mux.HandleFunc(secpulse.TaskScanOpenVAS, handleScanJob(cfg, pool))

	// ── SecVitals: daily control-owner due-date reminder ─────────────────────
	mux.HandleFunc(taskControlOwnerReminder, handleControlOwnerReminder(cfg, pool))

	// ── GitHub CI evidence sync (daily) ──────────────────────────────────────
	mux.HandleFunc(taskGitHubCISync, handleGitHubCISync(cfg, pool))

	// ── SecPulse EPSS enrichment (daily) ─────────────────────────────────────
	mux.HandleFunc(secpulse.TaskEPSSEnrich, handleEPSSEnrich(cfg, pool))

	// ── SecPulse report generation ────────────────────────────────────────────
	mux.HandleFunc(secpulse.TaskGenerateReport, handleGenerateReport(cfg, pool))

	// ── SecReflex campaign send ───────────────────────────────────────────────
	mux.HandleFunc(secreflex.TaskSendCampaign, handleSendCampaign(cfg, pool))

	// ── SecReflex training reminder ───────────────────────────────────────────
	mux.HandleFunc(secreflex.TaskTrainingReminder, handleTrainingReminder(cfg, pool))

	// ── SecVault git scanning ─────────────────────────────────────────────────
	mux.HandleFunc(secvault.TaskGitScan, handleGitScan(cfg, pool))

	// ── SecPrivacy: AVV expiry check ──────────────────────────────────────────
	mux.HandleFunc(secprivacy.TaskAVVExpiryCheck, handleAVVExpiryCheck(cfg, pool))

	// ── SecPrivacy→SecVitals: breach → incident ───────────────────────────────
	mux.HandleFunc(secprivacy.TaskBreachIncidentCreate, handleBreachIncidentCreate(cfg, pool))

	// ── SecPulse→SecVitals: resolved finding → patch-control evidence ─────────
	mux.HandleFunc(secpulse.TaskAutoEvidence, handleAutoEvidence(cfg, pool))

	// ── SecPulse SBOM generation (syft) ───────────────────────────────────────
	mux.HandleFunc(secpulse.TaskSBOMGenerate, handleSBOMGenerate(cfg, pool))

	// ── SecPulse EOL check (endoflife.date) ───────────────────────────────────
	mux.HandleFunc(secpulse.TaskEOLCheck, handleEOLCheck(cfg, pool))

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
	mux.HandleFunc(secvitals.TaskEvidenceExpiryAlert, handleEvidenceExpiryAlert(pool))

	// SecVitals: periodic DORA/NIS2 incident deadline check
	mux.HandleFunc(secvitals.TaskIncidentDeadlineCheck, handleIncidentDeadlineCheck(pool))

	// SecVitals: daily supplier certificate expiry check
	mux.HandleFunc(secvitals.TaskCertExpiryCheck, handleCertExpiryCheck(pool))

	// SecVitals: daily overdue control test CAPA creation
	mux.HandleFunc(taskControlTestCheck, handleControlTestCheck(pool))

	// Error budget: weekly SLO compliance report
	mux.HandleFunc(taskErrorBudgetReport, handleErrorBudgetReport(pool))

	// SecVitals: CCM — run all due automated control checks
	mux.HandleFunc(secvitals.TaskCCMRunDue, handleCCMRunDue(pool))

	// SecVitals: daily compliance score snapshot for trend charts
	mux.HandleFunc(secvitals.TaskScoreSnapshot, handleScoreSnapshot(pool))

	// Notifications: daily compliance deadline email alerts
	mux.HandleFunc(notifications.TaskNotifyDeadlines, handleNotifyDeadlines(cfg, pool))

	// Auth: daily cleanup of expired and used password reset tokens
	mux.HandleFunc(auth.TaskCleanupPasswordResetTokens, handleCleanupPasswordResetTokens(pool))

	// Cloud integrations: daily evidence collection from AWS + Azure
	mux.HandleFunc(cloudintegration.TaskCloudSync, handleCloudSync(cfg, pool))

	// Scheduled reports: daily delivery run
	mux.HandleFunc(scheduledreports.TaskProcessScheduledReports, handleProcessScheduledReports(cfg, pool))

	// Queue health: every 5 minutes — warn when failed/archived jobs accumulate
	mux.HandleFunc(taskQueueHealthCheck, handleQueueHealthCheck(cfg))

	return srv, mux
}

// handleScanJob returns an asynq handler that runs the appropriate scanner
// (trivy, nuclei, or openvas) based on the task type.

func EnqueueScanTask(client *asynq.Client, taskType string, payload []byte) error {
	queue := "default"
	if taskType == secpulse.TaskScanOpenVAS {
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

	cfg, _ := config.Load()

	// Open a single shared DB pool for the entire worker process lifetime.
	// All handler closures receive this pool — no per-job reconnects.
	ctx := context.Background()
	pool, err := connectDB(ctx, cfg)
	if err != nil {
		logger.Fatal().Err(err).Msg("failed to connect to database")
	}
	defer pool.Close()

	// Log EPSS opt-in status so operators know whether external enrichment is active.
	if cfg == nil || !cfg.EPSSEnabled {
		log.Info().Msg("EPSS enrichment disabled — set VAKT_EPSS_ENABLED=true to enable (sends CVE IDs to api.first.org)")
	}

	srv, mux := buildServer(pool)
	scheduler := buildScheduler(cfg)

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, os.Interrupt, syscall.SIGTERM)

	// Minimal health server on :9090 — used by the Docker Compose healthcheck.
	// Returns 200 OK while the worker process is alive and its DB pool is healthy.
	go func() {
		healthMux := http.NewServeMux()
		healthMux.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
			if err := pool.Ping(r.Context()); err != nil {
				http.Error(w, `{"status":"unavailable"}`, http.StatusServiceUnavailable)
				return
			}
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{"status":"ok"}`))
		})
		healthSrv := &http.Server{
			Addr:         ":9090",
			Handler:      healthMux,
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
