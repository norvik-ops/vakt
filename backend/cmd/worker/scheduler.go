// Copyright (c) 2026 NorvikOps. All rights reserved.
// SPDX-License-Identifier: Elastic-2.0
// See LICENSE file in the project root for terms.

package main

import (
	"github.com/hibiken/asynq"
	"github.com/rs/zerolog/log"

	"github.com/matharnica/vakt/internal/admin"
	"github.com/matharnica/vakt/internal/auth"
	"github.com/matharnica/vakt/internal/config"
	"github.com/matharnica/vakt/internal/modules/vaktcomply"
	"github.com/matharnica/vakt/internal/modules/vakthr"
	"github.com/matharnica/vakt/internal/modules/vaktprivacy"
	"github.com/matharnica/vakt/internal/modules/vaktscan"
	"github.com/matharnica/vakt/internal/modules/vaktvault"
	"github.com/matharnica/vakt/internal/services/alerting"
	"github.com/matharnica/vakt/internal/services/siem"
	"github.com/matharnica/vakt/internal/shared/bsi"
	"github.com/matharnica/vakt/internal/shared/demo"
	"github.com/matharnica/vakt/internal/shared/emaildigest"
	"github.com/matharnica/vakt/internal/shared/nis2wizard"
	"github.com/matharnica/vakt/internal/shared/notifications"
	cloudintegration "github.com/matharnica/vakt/internal/shared/platform/integrations/cloud"
	"github.com/matharnica/vakt/internal/shared/retention"
	"github.com/matharnica/vakt/internal/shared/scheduledreports"
)

func buildScheduler(cfg *config.Config) *asynq.Scheduler {
	var redisURL string
	if cfg != nil {
		redisURL = cfg.RedisUrl
	}
	scheduler := asynq.NewScheduler(
		asynqRedisOpt(redisURL),
		&asynq.SchedulerOpts{},
	)

	// Daily at 08:00 UTC: check AVV expiry and send alerts.
	if _, err := scheduler.Register("0 8 * * *",
		asynq.NewTask(vaktprivacy.TaskAVVExpiryCheck, nil),
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
		asynq.NewTask(vaktscan.TaskEPSSEnrich, nil),
	); err != nil {
		log.Error().Err(err).Msg("failed to register EPSS enrich cron")
	}

	// Daily at 02:30 UTC: pre-compute risk trend snapshots per org.
	// The dashboard reads from vb_risk_trend_snapshots instead of running
	// generate_series × vb_findings at request time.
	if _, err := scheduler.Register("30 2 * * *",
		asynq.NewTask(vaktscan.TaskRiskTrendSnapshot, nil),
	); err != nil {
		log.Error().Err(err).Msg("failed to register risk trend snapshot cron")
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
		vaktcomply.NewEvidenceExpiryAlertTask(),
	); err != nil {
		log.Error().Err(err).Msg("failed to register evidence expiry alert cron")
	}

	// Every 4 hours: check for overdue DORA/NIS2 incident deadlines.
	if _, err := scheduler.Register("0 */4 * * *",
		vaktcomply.NewIncidentDeadlineCheckTask(),
	); err != nil {
		log.Error().Err(err).Msg("failed to register incident deadline check cron")
	}

	// Daily at 08:30 UTC: check NIS2-classified incidents (obligation = "probably") for deadline alerts (S39-2).
	if _, err := scheduler.Register("30 8 * * *",
		vaktcomply.NewNIS2ObligationCheckTask(),
	); err != nil {
		log.Error().Err(err).Msg("failed to register nis2 obligation check cron")
	}

	// Every 5 minutes: update DORA IKT-incident Ampel-Status (S37-4).
	if _, err := scheduler.Register("*/5 * * * *",
		vaktcomply.NewDORADeadlineStatusTask(),
	); err != nil {
		log.Error().Err(err).Msg("failed to register DORA deadline status cron")
	}

	// Daily at 07:00 UTC: check supplier certificate expiry.
	if _, err := scheduler.Register("0 7 * * *",
		vaktcomply.NewCertExpiryCheckTask(),
	); err != nil {
		log.Error().Err(err).Msg("failed to register cert expiry check cron")
	}

	// Daily at 10:00 UTC: run all due CCM checks.
	if _, err := scheduler.Register("0 10 * * *",
		vaktcomply.NewCCMRunDueTask(),
	); err != nil {
		log.Error().Err(err).Msg("failed to register CCM run-due cron")
	}

	// Daily at 23:00 UTC: capture compliance score snapshot for trend charts.
	if _, err := scheduler.Register("0 23 * * *",
		vaktcomply.NewScoreSnapshotTask(),
	); err != nil {
		log.Error().Err(err).Msg("failed to register score snapshot cron")
	}

	// Daily at 08:00 UTC: send compliance deadline email alerts.
	if _, err := scheduler.Register("0 8 * * *",
		notifications.NewNotifyDeadlinesTask(),
	); err != nil {
		log.Error().Err(err).Msg("failed to register deadline notification cron")
	}

	// Daily at 03:10 UTC: delete expired and old used password-reset tokens.
	// Shifted from 03:00 to avoid pile-up with hourly demo-cleanup + digest jobs.
	if _, err := scheduler.Register("10 3 * * *",
		auth.NewCleanupPasswordResetTokensTask(),
	); err != nil {
		log.Error().Err(err).Msg("failed to register password reset token cleanup cron")
	}

	// Daily at 03:45 UTC: revoke SCIM tokens that have passed their expires_at.
	if _, err := scheduler.Register("45 3 * * *",
		admin.NewSCIMTokenExpiryTask(),
	); err != nil {
		log.Error().Err(err).Msg("failed to register SCIM token expiry cron")
	}

	// Daily at 03:20 UTC: delete expired rows from token_deny_list_fallback (S31-4).
	// Shifted from 03:05 to spread DB load across the 03:xx window.
	if _, err := scheduler.Register("20 3 * * *",
		auth.NewCleanupDenyListFallbackTask(),
	); err != nil {
		log.Error().Err(err).Msg("failed to register deny-list fallback cleanup cron")
	}

	// Sprint 22 S22-12: täglich 03:30 UTC — abgelaufene NIS2-Wizard-Runs aufräumen.
	// Shifted from 03:15 to spread DB load across the 03:xx window.
	if _, err := scheduler.Register("30 3 * * *",
		nis2wizard.NewCleanupAnonymousRunsTask(),
	); err != nil {
		log.Error().Err(err).Msg("failed to register nis2 anonymous runs cleanup cron")
	}

	// Sprint 22 S22-13: wöchentlich Sonntag 04:30 UTC — login_history > 90d aufräumen.
	// Shifted from 04:00 to avoid collision with Watchtower (0 4 * * *) and cloud sync.
	if _, err := scheduler.Register("30 4 * * 0",
		auth.NewCleanupLoginHistoryTask(),
	); err != nil {
		log.Error().Err(err).Msg("failed to register login history cleanup cron")
	}

	// Daily at 04:20 UTC: collect cloud evidence from all enabled AWS + Azure integrations.
	// Shifted from 04:00 to avoid collision with Watchtower restart window.
	if _, err := scheduler.Register("20 4 * * *",
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

	// Every 5 minutes: forward pending audit entries to configured SIEM backends.
	if _, err := scheduler.Register("*/5 * * * *",
		siem.NewSIEMForwardTask(),
	); err != nil {
		log.Error().Err(err).Msg("failed to register siem forward cron")
	}

	// S52-1: daily at 08:30 UTC — evidence freshness AI insight generation.
	if _, err := scheduler.Register("30 8 * * *",
		vaktcomply.NewEvidenceFreshnessCheckTask(),
	); err != nil {
		log.Error().Err(err).Msg("failed to register evidence freshness check cron")
	}

	// S52-4: every Monday at 08:00 UTC — AI compliance weekly digest (opt-in orgs only).
	if _, err := scheduler.Register("0 8 * * 1",
		vaktcomply.NewAIWeeklyDigestTask(),
	); err != nil {
		log.Error().Err(err).Msg("failed to register AI weekly digest cron")
	}

	// Daily at 03:00 UTC: rescan all tracked TLS certificates and update expiry status.
	if _, err := scheduler.Register("0 3 * * *",
		asynq.NewTask(vaktscan.TaskCertScan, nil),
	); err != nil {
		log.Error().Err(err).Msg("failed to register cert scan cron")
	}

	// S69-3: Daily at 04:00 UTC — update sla_status on all open findings.
	if _, err := scheduler.Register("0 4 * * *",
		asynq.NewTask(vaktscan.TaskSLACheck, nil),
	); err != nil {
		log.Error().Err(err).Msg("failed to register sla check cron")
	}

	// S61-3: Daily at 09:30 UTC — alert on major_nc CAPAs with overdue effectiveness checks.
	if _, err := scheduler.Register("30 9 * * *",
		asynq.NewTask(taskEffectivenessCheckOverdueAlert, nil),
	); err != nil {
		log.Error().Err(err).Msg("failed to register effectiveness check overdue alert cron")
	}

	// S61-7: daily at 06:00 UTC — compute and persist ISMS KPI snapshots for all orgs.
	if _, err := scheduler.Register("0 6 * * *",
		vaktcomply.NewISMSKPISnapshotTask(),
	); err != nil {
		log.Error().Err(err).Msg("failed to register ISMS KPI snapshot cron")
	}

	// S67-4: daily at 03:30 UTC — sweep evidence staleness for all controls.
	if _, err := scheduler.Register("30 3 * * *",
		asynq.NewTask(vaktcomply.TaskEvidenceStalenessCheck, nil),
	); err != nil {
		log.Error().Err(err).Msg("failed to register evidence staleness check cron")
	}

	// S74-2: daily at 06:15 UTC — update BSI check progress in KPI snapshots.
	if _, err := scheduler.Register("15 6 * * *",
		vaktcomply.NewBSIKPISnapshotTask(),
	); err != nil {
		log.Error().Err(err).Msg("failed to register BSI KPI snapshot cron")
	}

	// S68-2: daily at 08:15 UTC — mark overdue DSRs and send 3-day deadline warnings.
	if _, err := scheduler.Register("15 8 * * *",
		asynq.NewTask(vaktprivacy.TaskDSRDeadlineCheck, nil),
	); err != nil {
		log.Error().Err(err).Msg("failed to register dsr deadline check cron")
	}

	// S68-5: daily at 08:20 UTC — send deletion reminder notifications for reminders due within 14 days.
	if _, err := scheduler.Register("20 8 * * *",
		asynq.NewTask(vaktprivacy.TaskDeletionReminderCheck, nil),
	); err != nil {
		log.Error().Err(err).Msg("failed to register deletion reminder check cron")
	}

	// S70-4: daily at 07:30 UTC — check contractor expiry and mark expiring_soon/offboarding.
	if _, err := scheduler.Register("30 7 * * *",
		asynq.NewTask(vakthr.TaskContractorExpiryCheck, nil),
	); err != nil {
		log.Error().Err(err).Msg("failed to register contractor expiry check cron")
	}

	// S70-5: 1st of Jan/Apr/Jul/Oct at 06:00 UTC — create quarterly vault access reviews.
	if _, err := scheduler.Register("0 6 1 1,4,7,10 *",
		asynq.NewTask(vaktvault.TaskQuarterlyAccessReview, nil),
	); err != nil {
		log.Error().Err(err).Msg("failed to register quarterly access review cron")
	}

	return scheduler
}

const taskQueueHealthCheck = "queue:health:check"

// taskControlTestCheck is the Asynq task name for the daily overdue control test CAPA check.
const taskControlTestCheck = "vaktcomply:control_test_check"

// taskErrorBudgetReport is the Asynq task name for the weekly SLO error budget report.
const taskErrorBudgetReport = "errorbudget:weekly_report"

// handleControlTestCheck creates CAPAs for controls whose test_interval_days has elapsed.
