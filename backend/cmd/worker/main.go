// Copyright (c) 2026 NorvikOps. All rights reserved.
// SPDX-License-Identifier: Elastic-2.0
// See LICENSE file in the project root for terms.

package main

import (
	"context"
	"encoding/json"
	"fmt"
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
	"golang.org/x/sync/errgroup"

	"github.com/sechealth-app/sechealth/internal/admin"
	"github.com/sechealth-app/sechealth/internal/auth"
	"github.com/sechealth-app/sechealth/internal/shared/demo"
	"github.com/sechealth-app/sechealth/internal/config"
	cloudintegration "github.com/sechealth-app/sechealth/internal/shared/integrations/cloud"
	"github.com/sechealth-app/sechealth/internal/modules/secprivacy"
	"github.com/sechealth-app/sechealth/internal/modules/secreflex"
	"github.com/sechealth-app/sechealth/internal/modules/secpulse"
	"github.com/sechealth-app/sechealth/internal/modules/secvault"
	"github.com/sechealth-app/sechealth/internal/modules/secvitals"
	"github.com/sechealth-app/sechealth/internal/shared/alerting"
	"github.com/sechealth-app/sechealth/internal/shared/bsi"
	"github.com/sechealth-app/sechealth/internal/shared/crossevidence"
	"github.com/sechealth-app/sechealth/internal/shared/db"
	"github.com/sechealth-app/sechealth/internal/shared/emaildigest"
	"github.com/sechealth-app/sechealth/internal/shared/notifications"
	"github.com/sechealth-app/sechealth/internal/shared/notify"
	"github.com/sechealth-app/sechealth/internal/shared/retention"
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

	// ── SecPulse report generation ────────────────────────────────────────────
	mux.HandleFunc(secpulse.TaskGenerateReport, handleGenerateReport(cfg, pool))

	// ── SecReflex campaign send ───────────────────────────────────────────────
	mux.HandleFunc(secreflex.TaskSendCampaign, handleSendCampaign(cfg, pool))

	// ── SecReflex training reminder ───────────────────────────────────────────
	mux.HandleFunc(secreflex.TaskTrainingReminder, handleTrainingReminder(cfg, pool))

	// ── SecVault git scanning ─────────────────────────────────────────────────
	mux.HandleFunc(secvault.TaskGitScan, handleGitScan(cfg, pool))

	// ── MSP org deletion ──────────────────────────────────────────────────────
	mux.HandleFunc(admin.TaskDeleteOrg, handleDeleteOrg(cfg, pool))

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

	return srv, mux
}

// handleScanJob returns an asynq handler that runs the appropriate scanner
// (trivy, nuclei, or openvas) based on the task type.
// Scan jobs use asynq.MaxRetry(3) and asynq.Timeout(10 * time.Minute).
func handleScanJob(cfg *config.Config, pool *pgxpool.Pool) asynq.HandlerFunc {
	return func(ctx context.Context, t *asynq.Task) error {
		var payload secpulse.ScanPayload
		if err := json.Unmarshal(t.Payload(), &payload); err != nil {
			return fmt.Errorf("parse scan payload: %w", err)
		}

		// Build alertSvc once — nil when alerting is not configured.
		var alertSvc *alerting.Service
		if cfg != nil && cfg.SecretKey != "" {
			if masterKey, keyErr := hexDecodeKey(cfg.SecretKey); keyErr == nil {
				alertSvc = alerting.NewService(pool, masterKey, alerting.SMTPConfig{Host: cfg.SMTPHost, Port: cfg.SMTPPort, User: cfg.SMTPUser, Pass: cfg.SMTPPass, From: cfg.SMTPFrom})
			}
		}

		var scanErr error
		switch t.Type() {
		case secpulse.TaskScanTrivy:
			scanErr = secpulse.RunTrivyScan(ctx, pool, payload)
			if scanErr != nil {
				log.Error().Err(scanErr).Str("scan_id", payload.ScanID).Msg("trivy scan failed")
			}
		case secpulse.TaskScanNuclei:
			scanErr = secpulse.RunNucleiScan(ctx, pool, payload)
			if scanErr != nil {
				log.Error().Err(scanErr).Str("scan_id", payload.ScanID).Msg("nuclei scan failed")
			}
		case secpulse.TaskScanOpenVAS:
			scanErr = secpulse.RunOpenVASScan(ctx, pool, payload)
			if scanErr != nil {
				log.Error().Err(scanErr).Str("scan_id", payload.ScanID).Msg("openvas scan failed")
			}
		default:
			return fmt.Errorf("unknown scan task type: %s", t.Type())
		}

		if scanErr != nil {
			// Fire scan.failed alert — non-fatal, best-effort.
			if alertSvc != nil {
				alertSvc.Fire(ctx, payload.OrgID, alerting.EventScanFailed, map[string]any{
					"scan_id": payload.ScanID,
					"scanner": payload.Scanner,
					"asset":   payload.AssetName,
					"error":   scanErr.Error(),
				})
			}
			return scanErr
		}

		// After a successful scan, fire finding.new_critical for any critical
		// findings that belong to this scan and were seen for the first time today.
		if alertSvc != nil {
			var criticalCount int
			row := pool.QueryRow(ctx, `
				SELECT COUNT(*)
				FROM vb_findings
				WHERE org_id = $1::uuid
				  AND scan_id = $2::uuid
				  AND severity = 'critical'
				  AND status = 'open'
				  AND created_at >= NOW() - INTERVAL '1 hour'
			`, payload.OrgID, payload.ScanID)
			if scanErr2 := row.Scan(&criticalCount); scanErr2 == nil && criticalCount > 0 {
				alertSvc.Fire(ctx, payload.OrgID, alerting.EventFindingNewCritical, map[string]any{
					"scan_id":        payload.ScanID,
					"scanner":        payload.Scanner,
					"asset":          payload.AssetName,
					"critical_count": criticalCount,
				})
			}
		}

		return nil
	}
}

// EnqueueScanTask enqueues a scan job with retry and timeout options appropriate
// for long-running scanner processes (Trivy, Nuclei, OpenVAS).
// OpenVAS scans can take up to 10 minutes and are placed on the "low" queue
// to avoid blocking faster Trivy/Nuclei jobs on the "default" queue.
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

// handleGenerateReport handles secpulse:generate_report jobs.
func handleGenerateReport(cfg *config.Config, pool *pgxpool.Pool) asynq.HandlerFunc {
	return func(ctx context.Context, t *asynq.Task) error {
		var payload secpulse.GenerateReportPayload
		if err := json.Unmarshal(t.Payload(), &payload); err != nil {
			return fmt.Errorf("parse report payload: %w", err)
		}

		repo := secpulse.NewRepository(pool)

		if err := repo.UpdateReport(ctx, payload.ReportID, "", "processing", nil); err != nil {
			return fmt.Errorf("mark report processing: %w", err)
		}

		title := ""
		if t, ok := payload.Scope["title"].(string); ok {
			title = t
		}

		log.Info().Str("report_id", payload.ReportID).Str("org_id", payload.OrgID).Msg("generating PDF report")

		pdfBytes, err := secpulse.GenerateReportPDF(ctx, pool, payload.OrgID, title)
		if err != nil {
			_ = repo.UpdateReport(ctx, payload.ReportID, "", "failed", nil)
			return fmt.Errorf("generate PDF: %w", err)
		}

		expiresAt := time.Now().Add(30 * 24 * time.Hour)
		if err := repo.StoreReportContent(ctx, payload.ReportID, pdfBytes, expiresAt); err != nil {
			return fmt.Errorf("store report content: %w", err)
		}

		log.Info().Str("report_id", payload.ReportID).Int("bytes", len(pdfBytes)).Msg("PDF report generated")
		return nil
	}
}

// handleSendCampaign handles secreflex:send_campaign jobs.
func handleSendCampaign(cfg *config.Config, pool *pgxpool.Pool) asynq.HandlerFunc {
	return func(ctx context.Context, t *asynq.Task) error {
		var payload struct {
			CampaignID string `json:"campaign_id"`
			OrgID      string `json:"org_id"`
		}
		if err := json.Unmarshal(t.Payload(), &payload); err != nil {
			return fmt.Errorf("parse send_campaign payload: %w", err)
		}

		smtpCfg := secreflex.SMTPConfig{}
		if cfg != nil {
			smtpCfg.Host = cfg.SMTPHost
			smtpCfg.Port = cfg.SMTPPort
			smtpCfg.User = cfg.SMTPUser
			smtpCfg.Pass = cfg.SMTPPass
			smtpCfg.From = cfg.SMTPFrom
			smtpCfg.AppURL = cfg.FrontendURL
		}

		svc := secreflex.NewService(pool, smtpCfg)
		return svc.SendCampaignEmails(ctx, payload.OrgID, payload.CampaignID)
	}
}

// handleTrainingReminder handles secreflex:training_reminder jobs.
// It queries members who have not completed any training in the last 14 days
// and sends them a reminder email via the configured SMTP server.
func handleTrainingReminder(cfg *config.Config, pool *pgxpool.Pool) asynq.HandlerFunc {
	return func(ctx context.Context, t *asynq.Task) error {
		var payload struct {
			OrgID string `json:"org_id"`
		}
		if err := json.Unmarshal(t.Payload(), &payload); err != nil {
			return fmt.Errorf("parse training_reminder payload: %w", err)
		}

		// Query targets that have an overdue assignment and have not completed
		// any training in the last 14 days.
		type reminderTarget struct {
			Email     string
			FirstName string
		}

		rows, err := pool.Query(ctx, `
			SELECT DISTINCT t.email, t.first_name
			FROM pg_targets t
			JOIN pg_assignments a ON a.target_id = t.id AND a.org_id = $1
			WHERE t.org_id = $1
			  AND t.is_bounced = false
			  AND NOT EXISTS (
			    SELECT 1 FROM pg_completions c
			    WHERE c.assignment_id = a.id
			      AND c.completed_at >= NOW() - INTERVAL '14 days'
			  )
		`, payload.OrgID)
		if err != nil {
			return fmt.Errorf("training_reminder: query targets: %w", err)
		}
		defer rows.Close()

		var targets []reminderTarget
		for rows.Next() {
			var rt reminderTarget
			if err := rows.Scan(&rt.Email, &rt.FirstName); err != nil {
				continue
			}
			targets = append(targets, rt)
		}
		if err := rows.Err(); err != nil {
			return fmt.Errorf("training_reminder: scan rows: %w", err)
		}

		if len(targets) == 0 {
			log.Info().Str("org_id", payload.OrgID).Msg("training_reminder: no pending reminders")
			return nil
		}

		if cfg == nil || cfg.SMTPHost == "" {
			log.Warn().Str("org_id", payload.OrgID).
				Int("targets", len(targets)).
				Msg("training_reminder: SMTP not configured, skipping send")
			return nil
		}

		smtpCfg := secreflex.SMTPConfig{
			Host:   cfg.SMTPHost,
			Port:   cfg.SMTPPort,
			User:   cfg.SMTPUser,
			Pass:   cfg.SMTPPass,
			From:   cfg.SMTPFrom,
			AppURL: cfg.FrontendURL,
		}
		svc := secreflex.NewService(pool, smtpCfg)

		sent := 0
		for _, target := range targets {
			if err := svc.SendTrainingReminderEmail(ctx, payload.OrgID, target.Email, target.FirstName); err != nil {
				log.Warn().Err(err).
					Str("org_id", payload.OrgID).
					Str("email", target.Email).
					Msg("training_reminder: send failed")
				continue
			}
			sent++
		}

		log.Info().
			Str("org_id", payload.OrgID).
			Int("sent", sent).
			Int("total", len(targets)).
			Msg("training_reminder: reminders dispatched")

		return nil
	}
}

// handleGitScan handles secvault:git_scan jobs.
// Credentials stored in the payload are AES-256-GCM-encrypted; they are
// decrypted here using the master key from config before the scan runs.
func handleGitScan(cfg *config.Config, pool *pgxpool.Pool) asynq.HandlerFunc {
	return func(ctx context.Context, t *asynq.Task) error {
		var payload struct {
			ScanID               string `json:"scan_id"`
			OrgID                string `json:"org_id"`
			RepoURL              string `json:"repo_url"`
			Branch               string `json:"branch"`
			EncryptedCredentials string `json:"encrypted_credentials,omitempty"`
		}
		if err := json.Unmarshal(t.Payload(), &payload); err != nil {
			return fmt.Errorf("parse git_scan payload: %w", err)
		}

		// Decrypt credentials if present.
		var creds *secvault.GitScanCredentials
		if payload.EncryptedCredentials != "" {
			if cfg == nil || cfg.SecretKey == "" {
				return fmt.Errorf("git_scan: master key not configured, cannot decrypt credentials")
			}
			masterKey, keyErr := hexDecodeKey(cfg.SecretKey)
			if keyErr != nil {
				return fmt.Errorf("git_scan: invalid master key: %w", keyErr)
			}
			plainJSON, decErr := secvault.DecryptPayloadField(payload.EncryptedCredentials, masterKey)
			if decErr != nil {
				return fmt.Errorf("git_scan: decrypt credentials: %w", decErr)
			}
			var c secvault.GitScanCredentials
			if jsonErr := json.Unmarshal([]byte(plainJSON), &c); jsonErr != nil {
				return fmt.Errorf("git_scan: unmarshal credentials: %w", jsonErr)
			}
			creds = &c
		}

		repo := secvault.NewRepository(pool)

		if err := repo.UpdateGitScanStatus(ctx, payload.ScanID, payload.OrgID, "running", 0, 0, 0, "", nil); err != nil {
			return fmt.Errorf("mark scan running: %w", err)
		}

		results, scanErr := secvault.RunGitScan(ctx, secvault.TriggerGitScanInput{
			RepoURL:     payload.RepoURL,
			Branch:      payload.Branch,
			Credentials: creds,
		})

		scannedAt := time.Now().UTC()
		if scanErr != nil {
			errMsg := scanErr.Error()
			return repo.UpdateGitScanStatus(ctx, payload.ScanID, payload.OrgID, "failed", 0, 0, 0, errMsg, &scannedAt)
		}

		if len(results) > 0 {
			if err := repo.SaveScanResults(ctx, payload.OrgID, payload.ScanID, results); err != nil {
				return fmt.Errorf("save scan results: %w", err)
			}
		}

		openCount := len(results)
		if err := repo.UpdateGitScanStatus(ctx, payload.ScanID, payload.OrgID, "completed", openCount, openCount, 0, "", &scannedAt); err != nil {
			return fmt.Errorf("mark scan completed: %w", err)
		}

		log.Info().
			Str("scan_id", payload.ScanID).
			Str("repo_url", payload.RepoURL).
			Int("findings", openCount).
			Msg("git scan completed")

		return nil
	}
}

// handleDeleteOrg handles admin:org:delete jobs. It verifies that the
// scheduled_deletion_at has passed before hard-deleting the org.
func handleDeleteOrg(cfg *config.Config, pool *pgxpool.Pool) asynq.HandlerFunc {
	return func(ctx context.Context, t *asynq.Task) error {
		var payload struct {
			OrgID       string `json:"org_id"`
			ScheduledAt string `json:"scheduled_at"`
		}
		if err := json.Unmarshal(t.Payload(), &payload); err != nil {
			return fmt.Errorf("parse delete_org payload: %w", err)
		}

		repo := admin.NewRepository(pool)

		org, err := repo.GetOrg(ctx, payload.OrgID)
		if err != nil {
			// Org already deleted or does not exist — not an error.
			log.Warn().Str("org_id", payload.OrgID).Msg("delete_org: org not found, skipping")
			return nil
		}

		if org.ScheduledDeletionAt == nil || org.ScheduledDeletionAt.After(time.Now().UTC()) {
			log.Info().
				Str("org_id", payload.OrgID).
				Msg("delete_org: grace period not yet elapsed, skipping")
			return nil
		}

		// Hard-delete the org and all its data via CASCADE constraints.
		if _, err := pool.Exec(ctx,
			`DELETE FROM organizations WHERE id = $1::uuid`, payload.OrgID,
		); err != nil {
			return fmt.Errorf("hard delete org %s: %w", payload.OrgID, err)
		}

		log.Info().Str("org_id", payload.OrgID).Msg("org hard-deleted")
		return nil
	}
}

// handleAVVExpiryCheck handles secprivacy:avv_expiry_check jobs.
// Marks overdue AVVs as expired and fires avv.expired alerts per org.
// Uses errgroup with limit 5 to process orgs in parallel.
func handleAVVExpiryCheck(cfg *config.Config, pool *pgxpool.Pool) asynq.HandlerFunc {
	return func(ctx context.Context, _ *asynq.Task) error {
		svc := secprivacy.NewService(pool, asynq.RedisClientOpt{})
		if err := svc.CheckAVVExpiry(ctx); err != nil {
			log.Error().Err(err).Msg("avv expiry check failed")
			return err
		}

		// Fire avv.expired alert once per org that has newly-expired AVVs,
		// rate-limited to once per 24 hours via notification_alert_state.
		if cfg == nil || cfg.SecretKey == "" {
			return nil
		}
		masterKey, keyErr := hexDecodeKey(cfg.SecretKey)
		if keyErr != nil {
			log.Error().Err(keyErr).Msg("avv_expiry_check: invalid master key")
			return nil
		}

		rows, queryErr := pool.Query(ctx, `
			SELECT DISTINCT a.org_id::text
			FROM po_avvs a
			WHERE a.status = 'expired'
			  AND NOT EXISTS (
			    SELECT 1 FROM notification_alert_state s
			    WHERE s.org_id = a.org_id
			      AND s.event_type = $1
			      AND s.last_fired_at > NOW() - INTERVAL '24 hours'
			  )
		`, alerting.EventAVVExpired)
		if queryErr != nil {
			log.Error().Err(queryErr).Msg("avv_expiry_check: query failed")
			return nil
		}
		defer rows.Close()

		var orgIDs []string
		for rows.Next() {
			var orgID string
			if err := rows.Scan(&orgID); err != nil {
				continue
			}
			orgIDs = append(orgIDs, orgID)
		}

		alertSvc := alerting.NewService(pool, masterKey, alerting.SMTPConfig{Host: cfg.SMTPHost, Port: cfg.SMTPPort, User: cfg.SMTPUser, Pass: cfg.SMTPPass, From: cfg.SMTPFrom})

		g, gCtx := errgroup.WithContext(ctx)
		sem := make(chan struct{}, 5)
		for _, orgID := range orgIDs {
			orgID := orgID
			sem <- struct{}{}
			g.Go(func() error {
				defer func() { <-sem }()
				alertSvc.Fire(gCtx, orgID, alerting.EventAVVExpired, map[string]any{
					"message": "One or more AVVs have expired and must be renewed.",
				})
				_, _ = pool.Exec(gCtx, `
					INSERT INTO notification_alert_state (org_id, event_type, last_fired_at)
					VALUES ($1::uuid, $2, NOW())
					ON CONFLICT (org_id, event_type) DO UPDATE SET last_fired_at = NOW()
				`, orgID, alerting.EventAVVExpired)
				return nil
			})
		}
		_ = g.Wait()
		return nil
	}
}

// handleBreachIncidentCreate creates a linked SecVitals incident when a breach is reported.
// This is the integration point between SecPrivacy (breach) and SecVitals (incident register).
func handleBreachIncidentCreate(cfg *config.Config, pool *pgxpool.Pool) asynq.HandlerFunc {
	return func(ctx context.Context, t *asynq.Task) error {
		var payload secprivacy.BreachIncidentPayload
		if err := json.Unmarshal(t.Payload(), &payload); err != nil {
			return fmt.Errorf("parse breach_incident payload: %w", err)
		}

		repo := secvitals.NewRepository(pool)
		breachID := payload.BreachID
		input := secvitals.CreateIncidentInput{
			Title:           "[Datenschutzverletzung] " + payload.Title,
			Description:     payload.Description,
			Severity:        "critical",
			DiscoveredAt:    payload.DiscoveredAt,
			AffectedSystems: []string{},
			BreachID:        &breachID,
		}

		incident, err := repo.CreateIncident(ctx, payload.OrgID, input, nil)
		if err != nil {
			log.Error().Err(err).Str("breach_id", payload.BreachID).Msg("failed to create incident from breach")
			return fmt.Errorf("create incident from breach: %w", err)
		}

		log.Info().
			Str("breach_id", payload.BreachID).
			Str("incident_id", incident.ID).
			Str("org_id", payload.OrgID).
			Msg("secprivacy→secvitals: incident created from breach")
		return nil
	}
}

// handleAutoEvidence creates SecVitals evidence entries for patch-management
// controls when a SecPulse finding is resolved.
func handleAutoEvidence(cfg *config.Config, pool *pgxpool.Pool) asynq.HandlerFunc {
	return func(ctx context.Context, t *asynq.Task) error {
		var payload secpulse.AutoEvidencePayload
		if err := json.Unmarshal(t.Payload(), &payload); err != nil {
			return fmt.Errorf("parse auto_evidence payload: %w", err)
		}

		repo := secvitals.NewRepository(pool)
		controls, err := repo.FindPatchControls(ctx, payload.OrgID)
		if err != nil || len(controls) == 0 {
			// No patch controls found — not an error, nothing to do.
			return nil
		}

		title := fmt.Sprintf("Auto-collected: Patch verified — %s", payload.Title)
		if payload.CVE != "" {
			title = fmt.Sprintf("Auto-collected: %s patched", payload.CVE)
		}

		collectorData, _ := json.Marshal(map[string]string{
			"finding_id": payload.FindingID,
			"cve":        payload.CVE,
			"source":     "secpulse",
		})

		for _, ctrl := range controls {
			if _, evidErr := repo.AddCollectorEvidence(ctx, payload.OrgID, ctrl.ID, "", "automated", title, collectorData); evidErr != nil {
				log.Warn().
					Err(evidErr).
					Str("control_id", ctrl.ID).
					Str("finding_id", payload.FindingID).
					Msg("auto_evidence: failed to add evidence for control")
			}
		}

		log.Info().
			Str("finding_id", payload.FindingID).
			Str("org_id", payload.OrgID).
			Int("controls_updated", len(controls)).
			Msg("secpulse→secvitals: auto-evidence created for resolved finding")

		return nil
	}
}

// handleSLAOverdueCheck fires alerting events for findings past their SLA deadline.
// It groups findings by org and fires one alert per org that has overdue findings.
// Uses errgroup with limit 5 to process orgs in parallel.
func handleSLAOverdueCheck(cfg *config.Config, pool *pgxpool.Pool) asynq.HandlerFunc {
	return func(ctx context.Context, _ *asynq.Task) error {
		if cfg == nil || cfg.SecretKey == "" {
			return nil
		}

		masterKey, err := hexDecodeKey(cfg.SecretKey)
		if err != nil {
			log.Error().Err(err).Msg("sla_overdue_check: invalid master key")
			return nil
		}

		// Only fire for orgs not alerted in the last 24h — prevents repeated
		// alerts for perpetually-overdue findings on every cron tick.
		rows, err := pool.Query(ctx, `
			SELECT DISTINCT f.org_id::text
			FROM vb_findings f
			WHERE f.status NOT IN ('resolved','false_positive')
			  AND f.sla_due_at IS NOT NULL
			  AND f.sla_due_at < NOW()
			  AND NOT EXISTS (
			    SELECT 1 FROM notification_alert_state s
			    WHERE s.org_id = f.org_id
			      AND s.event_type = $1
			      AND s.last_fired_at > NOW() - INTERVAL '24 hours'
			  )
		`, alerting.EventFindingSLAOverdue)
		if err != nil {
			log.Error().Err(err).Msg("sla_overdue_check: query failed")
			return nil
		}
		defer rows.Close()

		var orgIDs []string
		for rows.Next() {
			var orgID string
			if err := rows.Scan(&orgID); err != nil {
				continue
			}
			orgIDs = append(orgIDs, orgID)
		}

		alertSvc := alerting.NewService(pool, masterKey, alerting.SMTPConfig{Host: cfg.SMTPHost, Port: cfg.SMTPPort, User: cfg.SMTPUser, Pass: cfg.SMTPPass, From: cfg.SMTPFrom})

		g, gCtx := errgroup.WithContext(ctx)
		sem := make(chan struct{}, 5)
		for _, orgID := range orgIDs {
			orgID := orgID
			sem <- struct{}{}
			g.Go(func() error {
				defer func() { <-sem }()
				alertSvc.Fire(gCtx, orgID, alerting.EventFindingSLAOverdue, map[string]any{
					"message": "One or more findings have exceeded their SLA deadline.",
				})
				_, _ = pool.Exec(gCtx, `
					INSERT INTO notification_alert_state (org_id, event_type, last_fired_at)
					VALUES ($1::uuid, $2, NOW())
					ON CONFLICT (org_id, event_type) DO UPDATE SET last_fired_at = NOW()
				`, orgID, alerting.EventFindingSLAOverdue)
				return nil
			})
		}
		_ = g.Wait()
		return nil
	}
}

// handleDSROverdueCheck fires alerting events for DSR requests past their due date.
// Uses errgroup with limit 5 to process orgs in parallel.
func handleDSROverdueCheck(cfg *config.Config, pool *pgxpool.Pool) asynq.HandlerFunc {
	return func(ctx context.Context, _ *asynq.Task) error {
		if cfg == nil || cfg.SecretKey == "" {
			return nil
		}

		masterKey, err := hexDecodeKey(cfg.SecretKey)
		if err != nil {
			log.Error().Err(err).Msg("dsr_overdue_check: invalid master key")
			return nil
		}

		rows, err := pool.Query(ctx, `
			SELECT DISTINCT d.org_id::text
			FROM po_dsr d
			WHERE d.status NOT IN ('completed','rejected')
			  AND d.due_date < NOW()
			  AND NOT EXISTS (
			    SELECT 1 FROM notification_alert_state s
			    WHERE s.org_id = d.org_id
			      AND s.event_type = $1
			      AND s.last_fired_at > NOW() - INTERVAL '24 hours'
			  )
		`, alerting.EventDSROverdue)
		if err != nil {
			log.Error().Err(err).Msg("dsr_overdue_check: query failed")
			return nil
		}
		defer rows.Close()

		var orgIDs []string
		for rows.Next() {
			var orgID string
			if err := rows.Scan(&orgID); err != nil {
				continue
			}
			orgIDs = append(orgIDs, orgID)
		}

		alertSvc := alerting.NewService(pool, masterKey, alerting.SMTPConfig{Host: cfg.SMTPHost, Port: cfg.SMTPPort, User: cfg.SMTPUser, Pass: cfg.SMTPPass, From: cfg.SMTPFrom})

		g, gCtx := errgroup.WithContext(ctx)
		sem := make(chan struct{}, 5)
		for _, orgID := range orgIDs {
			orgID := orgID
			sem <- struct{}{}
			g.Go(func() error {
				defer func() { <-sem }()
				alertSvc.Fire(gCtx, orgID, alerting.EventDSROverdue, map[string]any{
					"message": "One or more DSR requests have exceeded their due date.",
				})
				_, _ = pool.Exec(gCtx, `
					INSERT INTO notification_alert_state (org_id, event_type, last_fired_at)
					VALUES ($1::uuid, $2, NOW())
					ON CONFLICT (org_id, event_type) DO UPDATE SET last_fired_at = NOW()
				`, orgID, alerting.EventDSROverdue)
				return nil
			})
		}
		_ = g.Wait()
		return nil
	}
}

// fromHexChar converts a hex character to its numeric value (0-15), or 255 on error.
func fromHexChar(c byte) byte {
	switch {
	case c >= '0' && c <= '9':
		return c - '0'
	case c >= 'a' && c <= 'f':
		return c - 'a' + 10
	case c >= 'A' && c <= 'F':
		return c - 'A' + 10
	default:
		return 255
	}
}

// hexDecodeKey decodes a hex-encoded 32-byte master key.
func hexDecodeKey(s string) ([]byte, error) {
	b := make([]byte, len(s)/2)
	for i := 0; i < len(s)-1; i += 2 {
		hi := fromHexChar(s[i])
		lo := fromHexChar(s[i+1])
		if hi == 255 || lo == 255 {
			return nil, fmt.Errorf("invalid hex at position %d", i)
		}
		b[i/2] = hi<<4 | lo
	}
	return b, nil
}

func handleDemoCleanup(pool *pgxpool.Pool) asynq.HandlerFunc {
	return func(ctx context.Context, _ *asynq.Task) error {
		return demo.HandleCleanup(ctx, pool)
	}
}

func handleRetentionRun(pool *pgxpool.Pool) asynq.HandlerFunc {
	return func(ctx context.Context, _ *asynq.Task) error {
		return retention.RunRetentionAllOrgs(ctx, pool)
	}
}

func handleCleanupPasswordResetTokens(pool *pgxpool.Pool) asynq.HandlerFunc {
	return func(ctx context.Context, _ *asynq.Task) error {
		return auth.CleanupPasswordResetTokens(ctx, pool)
	}
}

func handleWeeklyDigest(cfg *config.Config, pool *pgxpool.Pool) asynq.HandlerFunc {
	return func(ctx context.Context, _ *asynq.Task) error {
		smtpCfg := emaildigest.SMTPConfig{}
		if cfg != nil {
			smtpCfg.Host = cfg.SMTPHost
			smtpCfg.Port = cfg.SMTPPort
			smtpCfg.User = cfg.SMTPUser
			smtpCfg.Pass = cfg.SMTPPass
			smtpCfg.From = cfg.SMTPFrom
		}
		return emaildigest.SendDigestForAllOrgs(ctx, pool, smtpCfg)
	}
}

func handleBSIFeedSync(pool *pgxpool.Pool) asynq.HandlerFunc {
	return func(ctx context.Context, _ *asynq.Task) error {
		svc := bsi.NewBSIService(pool)
		return svc.SyncFeed(ctx)
	}
}

// handleRecordEvidence records cross-module compliance evidence in SecVitals.
// Triggered by secreflex (training), secprivacy (DSR), and secvault (rotation) events.
func handleRecordEvidence(pool *pgxpool.Pool) asynq.HandlerFunc {
	// keywords per source module → relevant SecVitals control domains
	sourceKeywords := map[string][]string{
		"secreflex":  {"training", "awareness", "schulung", "bewusstsein"},
		"secprivacy": {"datenschutz", "privacy", "dsar", "betroffene"},
		"secvault":   {"access", "password", "secret", "rotation", "credential"},
	}
	return func(ctx context.Context, t *asynq.Task) error {
		var payload crossevidence.EvidencePayload
		if err := json.Unmarshal(t.Payload(), &payload); err != nil {
			return fmt.Errorf("parse evidence payload: %w", err)
		}

		keywords := sourceKeywords[payload.Source]
		if len(keywords) == 0 {
			return nil
		}

		repo := secvitals.NewRepository(pool)
		controls, err := repo.FindControlsByKeywords(ctx, payload.OrgID, keywords)
		if err != nil || len(controls) == 0 {
			log.Info().
				Str("org_id", payload.OrgID).
				Str("source", payload.Source).
				Msg("crossevidence: no matching controls found")
			return nil
		}

		collectorData, _ := json.Marshal(map[string]string{
			"source":        payload.Source,
			"resource_type": payload.ResourceType,
			"resource_id":   payload.ResourceID,
		})

		for _, ctrl := range controls {
			if _, evidErr := repo.AddCollectorEvidence(
				ctx, payload.OrgID, ctrl.ID, "", "automated",
				payload.Title, collectorData,
			); evidErr != nil {
				log.Warn().
					Err(evidErr).
					Str("control_id", ctrl.ID).
					Str("source", payload.Source).
					Msg("crossevidence: add evidence failed")
			}
		}

		log.Info().
			Str("org_id", payload.OrgID).
			Str("source", payload.Source).
			Str("resource_type", payload.ResourceType).
			Int("controls_updated", len(controls)).
			Msg("crossevidence: evidence recorded")
		return nil
	}
}

// handleEvidenceExpiryAlert sends per-evidence in-app notifications for evidence
// expiring within 30 days that has not yet been notified (expiry_notified_at IS NULL).
// Runs daily at 09:00 UTC. Uses errgroup with limit 5 to process orgs in parallel.
func handleEvidenceExpiryAlert(pool *pgxpool.Pool) asynq.HandlerFunc {
	return func(ctx context.Context, _ *asynq.Task) error {
		rows, err := pool.Query(ctx, `SELECT id::text FROM organizations WHERE is_deleted = false`)
		if err != nil {
			return fmt.Errorf("evidence_expiry_alert: list orgs: %w", err)
		}
		defer rows.Close()

		var orgIDs []string
		for rows.Next() {
			var id string
			if err := rows.Scan(&id); err != nil {
				continue
			}
			orgIDs = append(orgIDs, id)
		}
		if err := rows.Err(); err != nil {
			return err
		}

		repo := secvitals.NewRepository(pool)
		threshold := time.Now().UTC().AddDate(0, 0, 30)

		g, gCtx := errgroup.WithContext(ctx)
		sem := make(chan struct{}, 5)
		for _, orgID := range orgIDs {
			orgID := orgID
			sem <- struct{}{}
			g.Go(func() error {
				defer func() { <-sem }()
				items, err := repo.GetUnnotifiedExpiringEvidence(gCtx, orgID, threshold)
				if err != nil || len(items) == 0 {
					return nil
				}
				// Send one in-app notification per evidence item for actionable granularity.
				notifiedIDs := make([]string, 0, len(items))
				for _, item := range items {
					dateStr := item.ExpiresAt.Format("02.01.2006")
					msg := fmt.Sprintf(
						"Evidence für Control '%s' läuft am %s ab und muss erneuert werden.",
						item.ControlTitle, dateStr,
					)
					notify.Send(gCtx, pool, orgID, "Nachweis läuft ab", msg, "warning", "secvitals")
					notifiedIDs = append(notifiedIDs, item.ID)
				}
				// Mark all notified items so we do not re-notify on subsequent runs.
				if markErr := repo.MarkEvidenceExpiryNotified(gCtx, notifiedIDs); markErr != nil {
					log.Error().Err(markErr).Str("org_id", orgID).Msg("evidence_expiry_alert: mark notified")
				}
				log.Info().Str("org_id", orgID).Int("count", len(notifiedIDs)).Msg("evidence_expiry_alert: sent")
				return nil
			})
		}
		return g.Wait()
	}
}

// handleIncidentDeadlineCheck iterates all organisations and fires in-app notifications
// for any DORA/NIS2 incident deadline that is overdue and has not yet been reported.
// Uses errgroup with limit 5 for parallel org processing.
func handleIncidentDeadlineCheck(pool *pgxpool.Pool) asynq.HandlerFunc {
	return func(ctx context.Context, _ *asynq.Task) error {
		rows, err := pool.Query(ctx, `SELECT id::text FROM organizations WHERE is_deleted = false`)
		if err != nil {
			return fmt.Errorf("incident_deadline_check: list orgs: %w", err)
		}
		defer rows.Close()

		var orgIDs []string
		for rows.Next() {
			var orgID string
			if err := rows.Scan(&orgID); err != nil {
				continue
			}
			orgIDs = append(orgIDs, orgID)
		}
		if err := rows.Err(); err != nil {
			return err
		}

		svc := secvitals.NewService(pool)

		g, gCtx := errgroup.WithContext(ctx)
		sem := make(chan struct{}, 5)
		for _, orgID := range orgIDs {
			orgID := orgID
			sem <- struct{}{}
			g.Go(func() error {
				defer func() { <-sem }()
				if err := svc.CheckOverdueDeadlines(gCtx, orgID); err != nil {
					log.Error().Err(err).Str("org_id", orgID).Msg("incident_deadline_check: failed")
				}
				return nil
			})
		}
		return g.Wait()
	}
}

// handleCertExpiryCheck sends in-app notifications for supplier certificates expiring within 30 days.
// Runs daily at 07:00 UTC. Uses errgroup with limit 5 for parallel org processing.
func handleCertExpiryCheck(pool *pgxpool.Pool) asynq.HandlerFunc {
	return func(ctx context.Context, _ *asynq.Task) error {
		rows, err := pool.Query(ctx, `SELECT id::text, name FROM organizations WHERE is_deleted = false`)
		if err != nil {
			return fmt.Errorf("cert_expiry_check: list orgs: %w", err)
		}
		defer rows.Close()

		type orgRow struct {
			id   string
			name string
		}
		var orgs []orgRow
		for rows.Next() {
			var o orgRow
			if err := rows.Scan(&o.id, &o.name); err != nil {
				continue
			}
			orgs = append(orgs, o)
		}
		if err := rows.Err(); err != nil {
			return err
		}

		repo := secvitals.NewRepository(pool)
		threshold := time.Now().UTC().AddDate(0, 0, 30)

		g, gCtx := errgroup.WithContext(ctx)
		sem := make(chan struct{}, 5)
		for _, o := range orgs {
			o := o
			sem <- struct{}{}
			g.Go(func() error {
				defer func() { <-sem }()
				items, err := repo.FindExpiringCerts(gCtx, o.id, threshold)
				if err != nil || len(items) == 0 {
					return nil
				}
				msg := fmt.Sprintf("%d Lieferanten-Zertifikate laufen in den nächsten 30 Tagen ab.", len(items))
				notify.Send(gCtx, pool, o.id, "Lieferanten-Zertifikate laufen ab", msg, "warning", "secvitals")
				log.Info().Str("org_id", o.id).Int("count", len(items)).Msg("cert_expiry_check: sent")
				return nil
			})
		}
		return g.Wait()
	}
}

// handleCCMRunDue runs all enabled CCM checks that are past their interval.
func handleCCMRunDue(pool *pgxpool.Pool) asynq.HandlerFunc {
	return func(ctx context.Context, _ *asynq.Task) error {
		svc := secvitals.NewService(pool)
		if err := svc.RunDueCCMChecks(ctx); err != nil {
			log.Error().Err(err).Msg("ccm_run_due: failed")
			return err
		}
		return nil
	}
}

// handleScoreSnapshot records daily compliance score snapshots for all organisations.
// The snapshots power the trend chart on the dashboard.
func handleScoreSnapshot(pool *pgxpool.Pool) asynq.HandlerFunc {
	return func(ctx context.Context, _ *asynq.Task) error {
		svc := secvitals.NewService(pool)
		if err := svc.RecordScoreSnapshotForAllOrgs(ctx); err != nil {
			log.Error().Err(err).Msg("score_snapshot: failed")
			return err
		}
		log.Info().Msg("score_snapshot: completed")
		return nil
	}
}

// handleNotifyDeadlines runs all compliance deadline email checks in one job.
func handleNotifyDeadlines(cfg *config.Config, pool *pgxpool.Pool) asynq.HandlerFunc {
	return func(ctx context.Context, _ *asynq.Task) error {
		m := notifications.NewMailer(cfg)
		if err := notifications.CheckBreachDeadlines(ctx, pool, m); err != nil {
			log.Error().Err(err).Msg("breach deadline check failed")
		}
		if err := notifications.CheckDSRDeadlines(ctx, pool, m); err != nil {
			log.Error().Err(err).Msg("dsr deadline check failed")
		}
		if err := notifications.CheckAVVExpiry(ctx, pool, m); err != nil {
			log.Error().Err(err).Msg("avv expiry check failed")
		}
		if err := notifications.CheckCCMFailures(ctx, pool, m); err != nil {
			log.Error().Err(err).Msg("ccm failure check failed")
		}
		return nil
	}
}

// handleCloudSync runs evidence collection for all enabled AWS + Azure cloud integrations.
func handleCloudSync(cfg *config.Config, pool *pgxpool.Pool) asynq.HandlerFunc {
	return func(ctx context.Context, _ *asynq.Task) error {
		if cfg == nil || cfg.SecretKey == "" {
			log.Warn().Msg("cloud_sync: master key not configured, skipping")
			return nil
		}
		masterKey, err := hexDecodeKey(cfg.SecretKey)
		if err != nil {
			log.Error().Err(err).Msg("cloud_sync: invalid master key")
			return err
		}
		svc := cloudintegration.NewService(pool, masterKey)
		if err := svc.SyncAllEnabled(ctx); err != nil {
			log.Error().Err(err).Msg("cloud_sync: failed")
			return err
		}
		log.Info().Msg("cloud_sync: completed")
		return nil
	}
}

// connectDB opens a pgx pool using the config's DB URL.
func connectDB(ctx context.Context, cfg *config.Config) (*pgxpool.Pool, error) {
	dbURL := ""
	if cfg != nil {
		dbURL = cfg.DBUrl
	}
	if dbURL == "" {
		dbURL = os.Getenv("VAKT_DB_URL")
	}
	pool, err := db.Connect(ctx, dbURL)
	if err != nil {
		return nil, fmt.Errorf("connect to database: %w", err)
	}
	return pool, nil
}

func buildScheduler(cfg *config.Config) *asynq.Scheduler {
	redisAddr := "localhost:6379"
	if cfg != nil && cfg.RedisUrl != "" {
		redisAddr = cfg.RedisUrl
	}

	scheduler := asynq.NewScheduler(
		asynq.RedisClientOpt{Addr: redisAddr},
		&asynq.SchedulerOpts{},
	)

	// Daily at 08:00 UTC: check AVV expiry and send alerts.
	if _, err := scheduler.Register("0 8 * * *",
		asynq.NewTask(secprivacy.TaskAVVExpiryCheck, nil),
	); err != nil {
		log.Error().Err(err).Msg("failed to register AVV expiry cron")
	}

	// Daily at 08:00 UTC: check for overdue SLA findings.
	if _, err := scheduler.Register("0 8 * * *",
		asynq.NewTask(alerting.TaskSLAOverdueCheck, nil),
	); err != nil {
		log.Error().Err(err).Msg("failed to register SLA overdue check cron")
	}

	// Daily at 08:00 UTC: check for overdue DSR requests.
	if _, err := scheduler.Register("0 8 * * *",
		asynq.NewTask(alerting.TaskDSROverdueCheck, nil),
	); err != nil {
		log.Error().Err(err).Msg("failed to register DSR overdue check cron")
	}

	// Hourly: delete ephemeral demo orgs older than 4 hours.
	if _, err := scheduler.Register("0 * * * *",
		demo.NewCleanupTask(),
	); err != nil {
		log.Error().Err(err).Msg("failed to register demo cleanup cron")
	}

	// Daily at 02:00 UTC: prune expired data per org retention policy.
	if _, err := scheduler.Register("0 2 * * *",
		retention.NewRetentionRunTask(),
	); err != nil {
		log.Error().Err(err).Msg("failed to register retention cron")
	}

	// Every Monday at 08:00 UTC: send weekly security digest to org admins.
	if _, err := scheduler.Register("0 8 * * 1",
		emaildigest.NewWeeklyDigestTask(),
	); err != nil {
		log.Error().Err(err).Msg("failed to register digest cron")
	}

	// Daily at 06:00 UTC: sync BSI CERT-Bund advisories and match to assets.
	if _, err := scheduler.Register("0 6 * * *",
		bsi.NewBSIFeedSyncTask(),
	); err != nil {
		log.Error().Err(err).Msg("failed to register BSI feed sync cron")
	}

	// Daily at 09:00 UTC: alert on evidence expiring within 30 days.
	if _, err := scheduler.Register("0 9 * * *",
		secvitals.NewEvidenceExpiryAlertTask(),
	); err != nil {
		log.Error().Err(err).Msg("failed to register evidence expiry alert cron")
	}

	// Every 4 hours: check for overdue DORA/NIS2 incident deadlines.
	if _, err := scheduler.Register("0 */4 * * *",
		secvitals.NewIncidentDeadlineCheckTask(),
	); err != nil {
		log.Error().Err(err).Msg("failed to register incident deadline check cron")
	}

	// Daily at 07:00 UTC: check supplier certificate expiry.
	if _, err := scheduler.Register("0 7 * * *",
		secvitals.NewCertExpiryCheckTask(),
	); err != nil {
		log.Error().Err(err).Msg("failed to register cert expiry check cron")
	}

	// Daily at 10:00 UTC: run all due CCM checks.
	if _, err := scheduler.Register("0 10 * * *",
		secvitals.NewCCMRunDueTask(),
	); err != nil {
		log.Error().Err(err).Msg("failed to register CCM run-due cron")
	}

	// Daily at 23:00 UTC: capture compliance score snapshot for trend charts.
	if _, err := scheduler.Register("0 23 * * *",
		secvitals.NewScoreSnapshotTask(),
	); err != nil {
		log.Error().Err(err).Msg("failed to register score snapshot cron")
	}

	// Daily at 08:00 UTC: send compliance deadline email alerts.
	if _, err := scheduler.Register("0 8 * * *",
		notifications.NewNotifyDeadlinesTask(),
	); err != nil {
		log.Error().Err(err).Msg("failed to register deadline notification cron")
	}

	// Daily at 03:00 UTC: delete expired and old used password-reset tokens.
	if _, err := scheduler.Register("0 3 * * *",
		auth.NewCleanupPasswordResetTokensTask(),
	); err != nil {
		log.Error().Err(err).Msg("failed to register password reset token cleanup cron")
	}

	// Daily at 04:00 UTC: collect cloud evidence from all enabled AWS + Azure integrations.
	if _, err := scheduler.Register("0 4 * * *",
		cloudintegration.NewCloudSyncTask(),
	); err != nil {
		log.Error().Err(err).Msg("failed to register cloud sync cron")
	}

	return scheduler
}

// handleSBOMGenerate handles secpulse:sbom:generate jobs.
// It calls RunSyftScan to generate a CycloneDX SBOM and persist it in the DB.
func handleSBOMGenerate(cfg *config.Config, pool *pgxpool.Pool) asynq.HandlerFunc {
	return func(ctx context.Context, t *asynq.Task) error {
		var payload secpulse.SBOMGeneratePayload
		if err := json.Unmarshal(t.Payload(), &payload); err != nil {
			return fmt.Errorf("parse sbom generate payload: %w", err)
		}

		if err := secpulse.RunSyftScan(ctx, pool, payload.OrgID, payload.AssetID, payload.Target); err != nil {
			log.Error().Err(err).
				Str("org_id", payload.OrgID).
				Str("asset_id", payload.AssetID).
				Msg("syft SBOM scan failed")
			return err
		}
		return nil
	}
}

// handleEOLCheck handles secpulse:eol:check jobs.
// It calls EOLChecker.CheckComponents to resolve EOL status for all SBOM components.
func handleEOLCheck(cfg *config.Config, pool *pgxpool.Pool) asynq.HandlerFunc {
	return func(ctx context.Context, t *asynq.Task) error {
		var payload secpulse.EOLCheckPayload
		if err := json.Unmarshal(t.Payload(), &payload); err != nil {
			return fmt.Errorf("parse eol check payload: %w", err)
		}

		checker := secpulse.NewEOLChecker(pool)
		if err := checker.CheckComponents(ctx, payload.OrgID, payload.SBOMID); err != nil {
			log.Error().Err(err).
				Str("org_id", payload.OrgID).
				Str("sbom_id", payload.SBOMID).
				Msg("EOL check failed")
			return err
		}
		return nil
	}
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
