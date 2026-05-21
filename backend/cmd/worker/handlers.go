// Copyright (c) 2026 NorvikOps. All rights reserved.
// SPDX-License-Identifier: Elastic-2.0
// See LICENSE file in the project root for terms.

package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/smtp"
	"regexp"
	"time"

	"github.com/hibiken/asynq"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/rs/zerolog/log"
	"golang.org/x/sync/errgroup"

	"github.com/matharnica/vakt/internal/auth"
	"github.com/matharnica/vakt/internal/config"
	"github.com/matharnica/vakt/internal/modules/secprivacy"
	"github.com/matharnica/vakt/internal/modules/secpulse"
	"github.com/matharnica/vakt/internal/modules/secreflex"
	"github.com/matharnica/vakt/internal/modules/secvault"
	"github.com/matharnica/vakt/internal/modules/secvitals"
	"github.com/matharnica/vakt/internal/services/alerting"
	"github.com/matharnica/vakt/internal/shared/bsi"
	"github.com/matharnica/vakt/internal/shared/controltests"
	"github.com/matharnica/vakt/internal/services/crossevidence"
	"github.com/matharnica/vakt/internal/shared/demo"
	"github.com/matharnica/vakt/internal/shared/emaildigest"
	"github.com/matharnica/vakt/internal/shared/errorbudget"
	cloudintegration "github.com/matharnica/vakt/internal/shared/integrations/cloud"
	ghintegration "github.com/matharnica/vakt/internal/shared/integrations/github"
	"github.com/matharnica/vakt/internal/shared/notifications"
	"github.com/matharnica/vakt/internal/shared/notify"
	"github.com/matharnica/vakt/internal/shared/retention"
	"github.com/matharnica/vakt/internal/shared/scheduledreports"
)

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

		title := payload.Scope.Title

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
		if err := notifications.CheckCertificationDeadlines(ctx, pool, m); err != nil {
			log.Error().Err(err).Msg("certification deadline check failed")
		}
		return nil
	}
}

// handleProcessScheduledReports runs all active reports that are due for delivery.
func handleProcessScheduledReports(cfg *config.Config, pool *pgxpool.Pool) asynq.HandlerFunc {
	return func(ctx context.Context, _ *asynq.Task) error {
		smtpCfg := scheduledreports.SMTPConfig{}
		if cfg != nil {
			smtpCfg.Host = cfg.SMTPHost
			smtpCfg.Port = cfg.SMTPPort
			smtpCfg.User = cfg.SMTPUser
			smtpCfg.Pass = cfg.SMTPPass
			smtpCfg.From = cfg.SMTPFrom
		}
		svc := scheduledreports.NewService(pool, smtpCfg)
		// Wire the secvitals service as the board report provider so that
		// scheduled reports of type "board_report" can generate a PDF attachment.
		svc.WithBoardReportProvider(secvitals.NewService(pool))
		if err := svc.ProcessDue(ctx); err != nil {
			log.Error().Err(err).Msg("scheduled_reports: process_due failed")
			return err
		}
		log.Info().Msg("scheduled_reports: process_due completed")
		return nil
	}
}

// taskControlOwnerReminder is the Asynq task name for the daily control-owner reminder.
const taskControlOwnerReminder = "secvitals:control_owner_reminder"

// taskGitHubCISync is the Asynq task name for the daily GitHub CI evidence sync.
const taskGitHubCISync = "github:ci_evidence:sync"

// reEmail matches a basic e-mail address to decide whether to send a reminder.
var reEmail = regexp.MustCompile(`^[^@\s]+@[^@\s]+\.[^@\s]+$`)

// handleControlOwnerReminder queries all controls whose due_date (from ck_tasks) is in
// exactly 7 days, whose status is neither implemented nor not_applicable, and whose
// soa_responsible looks like a valid e-mail address, then sends a plain-HTML reminder.
func handleControlOwnerReminder(cfg *config.Config, pool *pgxpool.Pool) asynq.HandlerFunc {
	return func(ctx context.Context, _ *asynq.Task) error {
		if cfg == nil || cfg.SMTPHost == "" {
			log.Info().Msg("control_owner_reminder: SMTP not configured, skipping")
			return nil
		}

		// Query controls with a task due in exactly 7 days that are not yet done.
		type reminderRow struct {
			OrgID       string
			ControlID   string
			ControlDBID string
			Title       string
			Responsible string
			DueDate     time.Time
		}

		rows, err := pool.Query(ctx, `
			SELECT
			    c.org_id::text,
			    c.control_id,
			    c.id::text,
			    c.title,
			    COALESCE(c.soa_responsible, '') AS responsible,
			    t.due_date::timestamptz
			FROM ck_controls c
			JOIN ck_tasks t ON t.entity_id = c.id
			                AND t.entity_type = 'control'
			                AND t.org_id = c.org_id
			WHERE t.due_date = CURRENT_DATE + INTERVAL '7 days'
			  AND t.status NOT IN ('done', 'closed')
			  AND COALESCE(c.manual_status, '') NOT IN ('implemented', 'not_applicable')
			  AND c.not_applicable = false
			  AND COALESCE(c.soa_responsible, '') <> ''
		`)
		if err != nil {
			return fmt.Errorf("control_owner_reminder: query: %w", err)
		}
		defer rows.Close()

		var reminders []reminderRow
		for rows.Next() {
			var r reminderRow
			if err := rows.Scan(&r.OrgID, &r.ControlID, &r.ControlDBID, &r.Title, &r.Responsible, &r.DueDate); err != nil {
				log.Warn().Err(err).Msg("control_owner_reminder: scan row")
				continue
			}
			reminders = append(reminders, r)
		}
		if err := rows.Err(); err != nil {
			return fmt.Errorf("control_owner_reminder: rows error: %w", err)
		}

		if len(reminders) == 0 {
			log.Info().Msg("control_owner_reminder: no controls due in 7 days")
			return nil
		}

		smtpAddr := cfg.SMTPHost + ":" + cfg.SMTPPort
		if smtpAddr == ":" {
			smtpAddr = "localhost:25"
		}
		var smtpAuth smtp.Auth
		if cfg.SMTPUser != "" && cfg.SMTPPass != "" {
			smtpAuth = smtp.PlainAuth("", cfg.SMTPUser, cfg.SMTPPass, cfg.SMTPHost)
		}

		frontendURL := cfg.FrontendURL
		if frontendURL == "" {
			frontendURL = "https://app.vakt.io"
		}

		sent := 0
		for _, r := range reminders {
			if !reEmail.MatchString(r.Responsible) {
				log.Debug().
					Str("control_id", r.ControlDBID).
					Str("responsible", r.Responsible).
					Msg("control_owner_reminder: not a valid e-mail, skipping")
				continue
			}

			subject := fmt.Sprintf("Erinnerung: Control %s fällig in 7 Tagen", r.ControlID)
			link := fmt.Sprintf("%s/secvitals/controls/%s", frontendURL, r.ControlDBID)
			dueDateStr := r.DueDate.Format("02.01.2006")

			var buf bytes.Buffer
			buf.WriteString(`<!DOCTYPE html><html><body style="font-family:sans-serif;color:#1a202c;">`)
			buf.WriteString(`<h2 style="color:#2b6cb0;">Vakt — Control-Erinnerung</h2>`)
			buf.WriteString(`<p>Das folgende Control ist in <strong>7 Tagen</strong> fällig:</p>`)
			buf.WriteString(`<table border="0" cellpadding="6"><tbody>`)
			fmt.Fprintf(&buf, `<tr><td><strong>Control:</strong></td><td>%s — %s</td></tr>`, r.ControlID, r.Title)
			fmt.Fprintf(&buf, `<tr><td><strong>Fälligkeitsdatum:</strong></td><td>%s</td></tr>`, dueDateStr)
			fmt.Fprintf(&buf, `<tr><td><strong>Link:</strong></td><td><a href="%s">Control öffnen</a></td></tr>`, link)
			buf.WriteString(`</tbody></table>`)
			buf.WriteString(`<p style="color:#718096;font-size:0.85em;">Diese E-Mail wurde automatisch von Vakt versandt.</p>`)
			buf.WriteString(`</body></html>`)

			headers := fmt.Sprintf(
				"From: %s\r\nTo: %s\r\nSubject: %s\r\nMIME-Version: 1.0\r\nContent-Type: text/html; charset=UTF-8\r\n\r\n",
				cfg.SMTPFrom, r.Responsible, subject,
			)
			msg := []byte(headers + buf.String())

			if sendErr := smtp.SendMail(smtpAddr, smtpAuth, cfg.SMTPFrom, []string{r.Responsible}, msg); sendErr != nil {
				log.Warn().
					Err(sendErr).
					Str("control_id", r.ControlDBID).
					Str("to", r.Responsible).
					Msg("control_owner_reminder: send failed")
				continue
			}
			sent++
			log.Info().
				Str("control_id", r.ControlDBID).
				Str("to", r.Responsible).
				Msg("control_owner_reminder: sent")
		}

		log.Info().
			Int("sent", sent).
			Int("total", len(reminders)).
			Msg("control_owner_reminder: completed")
		return nil
	}
}

// handleGitHubCISync collects GitHub Actions CI run evidence for all organisations.
// For each org, it queries all GitHub integrations and fetches the 10 most recent
// completed runs, inserting a ck_evidence row for each successful run.
func handleGitHubCISync(cfg *config.Config, pool *pgxpool.Pool) asynq.HandlerFunc {
	return func(ctx context.Context, _ *asynq.Task) error {
		rows, err := pool.Query(ctx, `SELECT id::text FROM organizations WHERE is_deleted = false`)
		if err != nil {
			return fmt.Errorf("github_ci_sync: list orgs: %w", err)
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

		for _, orgID := range orgIDs {
			if err := ghintegration.CollectCIEvidence(ctx, pool, orgID); err != nil {
				log.Error().Err(err).Str("org_id", orgID).Msg("github_ci_sync: org failed")
			}
		}
		log.Info().Int("orgs", len(orgIDs)).Msg("github_ci_sync: completed")
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
		svc := cloudintegration.NewService(pool, masterKey, cloudintegration.NoopEvidenceWriter())
		if err := svc.SyncAllEnabled(ctx); err != nil {
			log.Error().Err(err).Msg("cloud_sync: failed")
			return err
		}
		log.Info().Msg("cloud_sync: completed")
		return nil
	}
}

// connectDB opens a pgx pool using the config's DB URL.

func handleControlTestCheck(pool *pgxpool.Pool) asynq.HandlerFunc {
	return func(ctx context.Context, _ *asynq.Task) error {
		if err := controltests.CheckOverdueControlTests(ctx, pool); err != nil {
			log.Error().Err(err).Msg("control_test_check: failed")
			return err
		}
		return nil
	}
}

// handleErrorBudgetReport runs the weekly SLO compliance report against audit_log.
func handleErrorBudgetReport(pool *pgxpool.Pool) asynq.HandlerFunc {
	return func(ctx context.Context, _ *asynq.Task) error {
		cfg := errorbudget.LoadConfig()
		if err := errorbudget.WeeklyReport(ctx, pool, cfg); err != nil {
			log.Error().Err(err).Msg("errorbudget: weekly report failed")
			return err
		}
		return nil
	}
}

// handleQueueHealthCheck inspects Asynq queues and logs warnings when failed or
// archived job counts exceed thresholds. No DB required — reads directly from Redis.
func handleQueueHealthCheck(cfg *config.Config) asynq.HandlerFunc {
	return func(ctx context.Context, _ *asynq.Task) error {
		redisAddr := "localhost:6379"
		if cfg != nil && cfg.RedisUrl != "" {
			redisAddr = cfg.RedisUrl
		}

		inspector := asynq.NewInspector(asynq.RedisClientOpt{Addr: redisAddr})
		defer inspector.Close()

		queues, err := inspector.Queues()
		if err != nil {
			log.Warn().Err(err).Msg("queue_health: could not list queues")
			return nil
		}

		for _, q := range queues {
			info, err := inspector.GetQueueInfo(q)
			if err != nil {
				continue
			}
			if info.Failed > 0 {
				log.Warn().
					Str("queue", q).
					Int("failed", info.Failed).
					Int("archived", info.Archived).
					Msg("queue_health: failed jobs detected — review /admin/health or asynq CLI")
			}
			if info.Archived > 10 {
				log.Warn().
					Str("queue", q).
					Int("archived", info.Archived).
					Msg("queue_health: high archived job count — consider running 'asynq queue purge'")
			}
		}
		return nil
	}
}

// handleEPSSEnrich enriches all open findings across all organisations with
// EPSS scores fetched from the FIRST.org API. Errors for individual orgs are
// logged but do not abort processing of remaining orgs.
// Enrichment is opt-in via VAKT_EPSS_ENABLED=true because it sends CVE IDs to
// the external api.first.org service, which contradicts the self-hosted
// data-privacy promise.
func handleEPSSEnrich(cfg *config.Config, pool *pgxpool.Pool) asynq.HandlerFunc {
	return func(ctx context.Context, _ *asynq.Task) error {
		if cfg == nil || !cfg.EPSSEnabled {
			log.Info().Msg("epss_enrich: skipped — set VAKT_EPSS_ENABLED=true to enable (sends CVE IDs to api.first.org)")
			return nil
		}

		rows, err := pool.Query(ctx, `SELECT id::text FROM organizations WHERE is_deleted = false`)
		if err != nil {
			return fmt.Errorf("epss_enrich: list orgs: %w", err)
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

		for _, orgID := range orgIDs {
			if err := secpulse.UpdateEPSSScores(ctx, pool, orgID); err != nil {
				log.Error().Err(err).Str("org_id", orgID).Msg("epss_enrich: org failed")
			}
		}
		log.Info().Int("orgs", len(orgIDs)).Msg("epss_enrich: completed")
		return nil
	}
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

