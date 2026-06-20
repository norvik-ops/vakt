// Copyright (c) 2026 NorvikOps. All rights reserved.
// SPDX-License-Identifier: Elastic-2.0
// See LICENSE file in the project root for terms.

package main

import (
	"context"
	"fmt"
	"strings"

	"github.com/hibiken/asynq"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/redis/go-redis/v9"
	"github.com/rs/zerolog/log"
	"golang.org/x/sync/errgroup"

	"github.com/matharnica/vakt/internal/admin"
	"github.com/matharnica/vakt/internal/auth"
	"github.com/matharnica/vakt/internal/config"
	"github.com/matharnica/vakt/internal/modules/vaktcomply"
	"github.com/matharnica/vakt/internal/services/ai"
	"github.com/matharnica/vakt/internal/services/alerting"
	"github.com/matharnica/vakt/internal/services/siem"
	"github.com/matharnica/vakt/internal/shared/audit"
	"github.com/matharnica/vakt/internal/shared/bsi"
	"github.com/matharnica/vakt/internal/shared/demo"
	"github.com/matharnica/vakt/internal/shared/emaildigest"
	"github.com/matharnica/vakt/internal/shared/errorbudget"
	"github.com/matharnica/vakt/internal/shared/nis2wizard"
	"github.com/matharnica/vakt/internal/shared/notifications"
	"github.com/matharnica/vakt/internal/shared/notify"
	"github.com/matharnica/vakt/internal/shared/retention"
	"github.com/matharnica/vakt/internal/shared/scheduledreports"
)

// handleSLAOverdueCheck fires alerting events for findings past their SLA deadline.
// It groups findings by org and fires one alert per org that has overdue findings.
// Uses errgroup with limit 5 to process orgs in parallel.
func handleSLAOverdueCheck(cfg *config.Config, pool *pgxpool.Pool) asynq.HandlerFunc {
	return func(ctx context.Context, _ *asynq.Task) error {
		if cfg == nil || cfg.SecretKey == "" {
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

		alertSvc := alerting.NewService(pool, workerKey(cfg, "vakt-alert-v1"), alerting.SMTPConfig{Host: cfg.SMTPHost, Port: cfg.SMTPPort, User: cfg.SMTPUser, Pass: cfg.SMTPPass, From: cfg.SMTPFrom})

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

// handlePartitionMaint pre-creates upcoming audit_log year partitions and drops
// partitions past VAKT_AUDIT_RETENTION_YEARS (S98-10).
func handlePartitionMaint(pool *pgxpool.Pool) asynq.HandlerFunc {
	return func(ctx context.Context, _ *asynq.Task) error {
		return audit.MaintainPartitions(ctx, pool, audit.RetentionYearsFromEnv())
	}
}

func handleCleanupPasswordResetTokens(pool *pgxpool.Pool) asynq.HandlerFunc {
	return func(ctx context.Context, _ *asynq.Task) error {
		return auth.CleanupPasswordResetTokens(ctx, pool)
	}
}

func handleCleanupDenyListFallback(pool *pgxpool.Pool) asynq.HandlerFunc {
	return func(ctx context.Context, _ *asynq.Task) error {
		return auth.CleanupDenyListFallback(ctx, pool)
	}
}

// Sprint 22 / S22-12: täglicher Cleanup für abgelaufene NIS2-Wizard-Runs.
func handleCleanupNIS2AnonymousRuns(pool *pgxpool.Pool) asynq.HandlerFunc {
	return func(ctx context.Context, _ *asynq.Task) error {
		return nis2wizard.CleanupAnonymousRuns(ctx, pool)
	}
}

// Sprint 22 / S22-13: wöchentlicher Cleanup für Login-History > 90 Tage.
func handleCleanupLoginHistory(pool *pgxpool.Pool) asynq.HandlerFunc {
	return func(ctx context.Context, _ *asynq.Task) error {
		return auth.CleanupLoginHistory(ctx, pool)
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

// handleBSIFeedSync syncs the BSI CERT-Bund RSS feed. The feed is external and
// occasionally unreachable (DNS hiccups, BSI maintenance windows, IPv6 routing
// drama on the BSI side). We treat such transient failures as soft errors:
// log a warning and return nil so Asynq does not enqueue a retry storm that
// flips the worker's queue_health to unhealthy. The job is scheduled daily,
// so the next run picks up automatically.
//
// Hard errors — DB unreachable, schema drift, panic — still propagate.
func handleBSIFeedSync(pool *pgxpool.Pool) asynq.HandlerFunc {
	return func(ctx context.Context, _ *asynq.Task) error {
		svc := bsi.NewBSIService(pool)
		err := svc.SyncFeed(ctx)
		if err == nil {
			return nil
		}
		// SyncFeed wraps FetchAdvisories errors with the "bsi: fetch advisories"
		// prefix when the upstream HTTP call fails or the response is non-200.
		// Those are exactly the cases we want to swallow.
		msg := err.Error()
		if strings.Contains(msg, "bsi: fetch advisories") {
			log.Warn().Err(err).Msg("bsi: feed sync skipped — external feed unreachable; will retry on next schedule")
			return nil
		}
		return err
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
		// Wire the vaktcomply service as the board report provider so that
		// scheduled reports of type "board_report" can generate a PDF attachment.
		svc.WithBoardReportProvider(vaktcomply.NewService(pool))
		if err := svc.ProcessDue(ctx); err != nil {
			log.Error().Err(err).Msg("scheduled_reports: process_due failed")
			return err
		}
		log.Info().Msg("scheduled_reports: process_due completed")
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
		redisOpt := asynq.RedisClientOpt{Addr: "localhost:6379"}
		if cfg != nil && cfg.RedisUrl != "" {
			if parsed, err := redis.ParseURL(cfg.RedisUrl); err == nil {
				redisOpt = asynq.RedisClientOpt{
					Addr:     parsed.Addr,
					Password: parsed.Password,
					DB:       parsed.DB,
				}
			}
		}

		inspector := asynq.NewInspector(redisOpt)
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

// handleSIEMForward handles siem:forward_pending jobs.
// It forwards up to 100 unforwarded audit entries per org to the configured SIEM backend.
func handleSIEMForward(pool *pgxpool.Pool) asynq.HandlerFunc {
	return func(ctx context.Context, _ *asynq.Task) error {
		svc := siem.NewService(pool)
		if err := svc.ForwardPending(ctx); err != nil {
			log.Error().Err(err).Msg("siem forward pending failed")
			return err
		}
		return nil
	}
}

// handleAIWeeklyDigest runs every Monday at 08:00 UTC for orgs that have opted in to
// the AI compliance digest (ai_weekly_digest_enabled = true). It gathers a summary of
// open controls, stale evidences, and upcoming deadlines, asks the LLM for a narrative,
// and sends the result via in-app notification and optionally via email. S52-4.
func handleAIWeeklyDigest(cfg *config.Config, pool *pgxpool.Pool) asynq.HandlerFunc {
	return func(ctx context.Context, _ *asynq.Task) error {
		if cfg == nil || cfg.AIProvider == "disabled" || cfg.AIProvider == "" {
			log.Info().Msg("ai_weekly_digest: AI provider disabled, skipping")
			return nil
		}

		// Fetch orgs with digest opted in.
		rows, err := pool.Query(ctx, `
			SELECT id::text, name
			FROM organizations
			WHERE ai_weekly_digest_enabled = true
		`)
		if err != nil {
			return fmt.Errorf("ai_weekly_digest: list orgs: %w", err)
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

		client := ai.NewAIClient(cfg.AIBaseURL, cfg.AIAPIKey, cfg.AIModel)
		repo := vaktcomply.NewRepository(pool)

		smtpCfg := emaildigest.SMTPConfig{}
		if cfg != nil {
			smtpCfg.Host = cfg.SMTPHost
			smtpCfg.Port = cfg.SMTPPort
			smtpCfg.User = cfg.SMTPUser
			smtpCfg.Pass = cfg.SMTPPass
			smtpCfg.From = cfg.SMTPFrom
		}

		g, gCtx := errgroup.WithContext(ctx)
		sem := make(chan struct{}, 3)
		for _, o := range orgs {
			o := o
			sem <- struct{}{}
			g.Go(func() error {
				defer func() { <-sem }()
				if err := generateAndSendAIDigest(gCtx, pool, repo, client, o.id, o.name, smtpCfg); err != nil {
					log.Error().Err(err).Str("org_id", o.id).Msg("ai_weekly_digest: failed for org")
				}
				return nil
			})
		}
		return g.Wait()
	}
}

// generateAndSendAIDigest collects compliance context, calls the LLM, and sends
// the result as an in-app notification (+ email if SMTP is configured).
func generateAndSendAIDigest(
	ctx context.Context,
	pool *pgxpool.Pool,
	repo *vaktcomply.Repository,
	client *ai.AIClient,
	orgID, orgName string,
	smtpCfg emaildigest.SMTPConfig,
) error {
	// Top-3 controls by missing+in_progress ordered by weight.
	type controlRow struct {
		title  string
		status string
	}
	var topControls []controlRow
	ctrlRows, err := pool.Query(ctx, `
		SELECT title, manual_status
		FROM ck_controls
		WHERE org_id = $1::uuid
		  AND manual_status IN ('missing','in_progress')
		ORDER BY weight DESC
		LIMIT 3
	`, orgID)
	if err == nil {
		defer ctrlRows.Close()
		for ctrlRows.Next() {
			var r controlRow
			if ctrlRows.Scan(&r.title, &r.status) == nil {
				topControls = append(topControls, r)
			}
		}
	}

	// Top-2 stale evidence insights.
	insights, err := repo.ListActiveAIInsights(ctx, orgID)
	if err != nil {
		insights = nil
	}
	var staleInsights []vaktcomply.AIInsight
	for _, ins := range insights {
		if ins.Type == "evidence_stale" {
			staleInsights = append(staleInsights, ins)
			if len(staleInsights) >= 2 {
				break
			}
		}
	}

	// Next compliance deadline from ck_tasks.
	var nextDeadline string
	_ = pool.QueryRow(ctx, `
		SELECT title
		FROM ck_tasks
		WHERE org_id = $1::uuid AND due_date IS NOT NULL AND status != 'done'
		ORDER BY due_date ASC
		LIMIT 1
	`, orgID).Scan(&nextDeadline)

	// Build prompt — user-controlled strings (org name, control titles, insight
	// titles, task titles) are wrapped to satisfy ADR-0032 (prompt injection guard).
	var sb strings.Builder
	sb.WriteString("Erstelle einen kurzen KI-Compliance-Wochenüberblick auf Deutsch für ")
	sb.WriteString(ai.WrapUserContent(orgName))
	sb.WriteString(".\n\n")
	if len(topControls) > 0 {
		sb.WriteString("Offene / in Bearbeitung befindliche Controls:\n")
		for _, c := range topControls {
			fmt.Fprintf(&sb, "- %s (%s)\n", ai.WrapUserContent(c.title), c.status)
		}
		sb.WriteString("\n")
	}
	if len(staleInsights) > 0 {
		sb.WriteString("Veraltete Evidences:\n")
		for _, ins := range staleInsights {
			fmt.Fprintf(&sb, "- %s\n", ai.WrapUserContent(ins.Title))
		}
		sb.WriteString("\n")
	}
	if nextDeadline != "" {
		fmt.Fprintf(&sb, "Nächste Frist: %s\n\n", ai.WrapUserContent(nextDeadline))
	}
	sb.WriteString("Fasse den Status in 3–4 Sätzen zusammen und gib 2 konkrete Empfehlungen für diese Woche. Antworte ausschließlich auf Deutsch.")

	narrative, err := client.GenerateWithSystem(ctx,
		"Du bist ein ISO-27001/NIS2/BSI-Compliance-Experte. Antworte auf Deutsch, konkret und handlungsorientiert.",
		sb.String(),
	)
	if err != nil {
		return fmt.Errorf("ai_weekly_digest: generate narrative: %w", err)
	}

	// Send in-app notification.
	notify.Send(ctx, pool, orgID, "Dein KI-Compliance-Digest", narrative, "info", "vaktcomply")

	// Send email if SMTP is configured.
	if smtpCfg.Host != "" {
		if err := emaildigest.SendAIDigestEmail(ctx, pool, smtpCfg, orgID, orgName, narrative); err != nil {
			log.Warn().Err(err).Str("org_id", orgID).Msg("ai_weekly_digest: email send failed")
		}
	}

	log.Info().Str("org_id", orgID).Msg("ai_weekly_digest: sent")
	return nil
}

// handleSCIMTokenExpiry revokes SCIM tokens that have passed their expires_at timestamp.
func handleSCIMTokenExpiry(pool *pgxpool.Pool) asynq.HandlerFunc {
	return func(ctx context.Context, _ *asynq.Task) error {
		repo := admin.NewRepository(pool)
		count, err := repo.RevokeExpiredSCIMTokens(ctx)
		if err != nil {
			return fmt.Errorf("scim token expiry: %w", err)
		}
		if count > 0 {
			log.Info().Int64("revoked", count).Msg("scim token expiry: revoked expired tokens")
		}
		return nil
	}
}
