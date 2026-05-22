// Copyright (c) 2026 NorvikOps. All rights reserved.
// SPDX-License-Identifier: Elastic-2.0
// See LICENSE file in the project root for terms.

package main

import (
	"github.com/hibiken/asynq"
	"github.com/rs/zerolog/log"

	"github.com/matharnica/vakt/internal/auth"
	"github.com/matharnica/vakt/internal/config"
	"github.com/matharnica/vakt/internal/modules/secprivacy"
	"github.com/matharnica/vakt/internal/modules/secpulse"
	"github.com/matharnica/vakt/internal/modules/secvitals"
	"github.com/matharnica/vakt/internal/services/alerting"
	"github.com/matharnica/vakt/internal/shared/bsi"
	"github.com/matharnica/vakt/internal/shared/demo"
	"github.com/matharnica/vakt/internal/shared/emaildigest"
	cloudintegration "github.com/matharnica/vakt/internal/shared/integrations/cloud"
	"github.com/matharnica/vakt/internal/shared/nis2wizard"
	"github.com/matharnica/vakt/internal/shared/notifications"
	"github.com/matharnica/vakt/internal/shared/retention"
	"github.com/matharnica/vakt/internal/shared/scheduledreports"
)

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

	// Hourly: send weekly digest to orgs whose configured weekday+hour matches now.
	// Each org independently sets its preferred day (0=Sun…6=Sat) and hour (UTC).
	if _, err := scheduler.Register("0 * * * *",
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

	// Daily at 01:00 UTC: enrich all findings with EPSS scores from FIRST.org.
	if _, err := scheduler.Register("0 1 * * *",
		asynq.NewTask(secpulse.TaskEPSSEnrich, nil),
	); err != nil {
		log.Error().Err(err).Msg("failed to register EPSS enrich cron")
	}

	// Daily at 09:00 UTC: send control-owner due-date reminders (7-day advance notice).
	if _, err := scheduler.Register("0 9 * * *",
		asynq.NewTask(taskControlOwnerReminder, nil),
	); err != nil {
		log.Error().Err(err).Msg("failed to register control owner reminder cron")
	}

	// Daily at 05:00 UTC: collect GitHub Actions CI run evidence for all orgs.
	if _, err := scheduler.Register("0 5 * * *",
		asynq.NewTask(taskGitHubCISync, nil),
	); err != nil {
		log.Error().Err(err).Msg("failed to register GitHub CI evidence sync cron")
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

	// Sprint 22 S22-12: täglich 03:15 UTC — abgelaufene NIS2-Wizard-Runs aufräumen.
	if _, err := scheduler.Register("15 3 * * *",
		nis2wizard.NewCleanupAnonymousRunsTask(),
	); err != nil {
		log.Error().Err(err).Msg("failed to register nis2 anonymous runs cleanup cron")
	}

	// Sprint 22 S22-13: wöchentlich Sonntag 04:00 UTC — login_history > 90d aufräumen.
	if _, err := scheduler.Register("0 4 * * 0",
		auth.NewCleanupLoginHistoryTask(),
	); err != nil {
		log.Error().Err(err).Msg("failed to register login history cleanup cron")
	}

	// Daily at 04:00 UTC: collect cloud evidence from all enabled AWS + Azure integrations.
	if _, err := scheduler.Register("0 4 * * *",
		cloudintegration.NewCloudSyncTask(),
	); err != nil {
		log.Error().Err(err).Msg("failed to register cloud sync cron")
	}

	// Daily at 08:00 UTC: process all due scheduled reports.
	if _, err := scheduler.Register("0 8 * * *",
		asynq.NewTask(scheduledreports.TaskProcessScheduledReports, nil),
	); err != nil {
		log.Error().Err(err).Msg("failed to register scheduled reports cron")
	}

	// Every 5 minutes: check for failed/archived job accumulation.
	if _, err := scheduler.Register("*/5 * * * *",
		asynq.NewTask(taskQueueHealthCheck, nil),
	); err != nil {
		log.Error().Err(err).Msg("failed to register queue health check cron")
	}

	// Daily at 06:30 UTC: create CAPAs for controls whose test interval has elapsed.
	if _, err := scheduler.Register("30 6 * * *",
		asynq.NewTask(taskControlTestCheck, nil),
	); err != nil {
		log.Error().Err(err).Msg("failed to register control test check cron")
	}

	// Every Monday at 09:00 UTC: compute and log the weekly SLO error budget report.
	if _, err := scheduler.Register("0 9 * * 1",
		asynq.NewTask(taskErrorBudgetReport, nil),
	); err != nil {
		log.Error().Err(err).Msg("failed to register error budget report cron")
	}

	return scheduler
}

const taskQueueHealthCheck = "queue:health:check"

// taskControlTestCheck is the Asynq task name for the daily overdue control test CAPA check.
const taskControlTestCheck = "secvitals:control_test_check"

// taskErrorBudgetReport is the Asynq task name for the weekly SLO error budget report.
const taskErrorBudgetReport = "errorbudget:weekly_report"

// handleControlTestCheck creates CAPAs for controls whose test_interval_days has elapsed.
