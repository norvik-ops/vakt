// Copyright (c) 2026 NorvikOps. All rights reserved.
// SPDX-License-Identifier: Elastic-2.0
// See LICENSE file in the project root for terms.

package main

import (
	"time"

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
	"github.com/matharnica/vakt/internal/shared/audit"
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

	// --- 01:xx window ---

	// Daily at 01:03 UTC: enrich all findings with EPSS scores from FIRST.org.
	if _, err := scheduler.Register("3 1 * * *",
		asynq.NewTask(vaktscan.TaskEPSSEnrich, nil),
		asynq.Unique(23*time.Hour), asynq.Queue(vaktscan.QueueScans),
	); err != nil {
		log.Error().Err(err).Msg("failed to register EPSS enrich cron")
	}

	// --- 02:xx window ---

	// Daily at 02:07 UTC: prune expired data per org retention policy.
	if _, err := scheduler.Register("7 2 * * *",
		retention.NewRetentionRunTask(),
		asynq.Unique(23*time.Hour),
	); err != nil {
		log.Error().Err(err).Msg("failed to register retention cron")
	}

	// Daily at 02:30 UTC: pre-compute risk trend snapshots per org.
	// The dashboard reads from vb_risk_trend_snapshots instead of running
	// generate_series × vb_findings at request time.
	if _, err := scheduler.Register("30 2 * * *",
		asynq.NewTask(vaktscan.TaskRiskTrendSnapshot, nil),
		asynq.Unique(23*time.Hour), asynq.Queue(vaktscan.QueueScans),
	); err != nil {
		log.Error().Err(err).Msg("failed to register risk trend snapshot cron")
	}

	// --- 03:xx window ---

	// Daily at 03:04 UTC: rescan all tracked TLS certificates and update expiry status.
	if _, err := scheduler.Register("4 3 * * *",
		asynq.NewTask(vaktscan.TaskCertScan, nil),
		asynq.Unique(23*time.Hour), asynq.Queue(vaktscan.QueueScans),
	); err != nil {
		log.Error().Err(err).Msg("failed to register cert scan cron")
	}

	// Daily at 03:10 UTC: delete expired and old used password-reset tokens.
	if _, err := scheduler.Register("10 3 * * *",
		auth.NewCleanupPasswordResetTokensTask(),
		asynq.Unique(23*time.Hour),
	); err != nil {
		log.Error().Err(err).Msg("failed to register password reset token cleanup cron")
	}

	// Daily at 03:20 UTC: delete expired rows from token_deny_list_fallback (S31-4).
	if _, err := scheduler.Register("20 3 * * *",
		auth.NewCleanupDenyListFallbackTask(),
		asynq.Unique(23*time.Hour),
	); err != nil {
		log.Error().Err(err).Msg("failed to register deny-list fallback cleanup cron")
	}

	// Monthly (1st at 03:30 UTC): pre-create upcoming audit_log year partitions
	// and drop partitions past VAKT_AUDIT_RETENTION_YEARS (S98-10).
	if _, err := scheduler.Register("30 3 1 * *",
		audit.NewPartitionMaintTask(),
		asynq.Unique(27*24*time.Hour),
	); err != nil {
		log.Error().Err(err).Msg("failed to register audit partition maintenance cron")
	}

	// Daily at 03:32 UTC: clean up expired NIS2-wizard anonymous runs (S22-12).
	// Shifted +2min from 03:30 to avoid monthly collision with audit partition maint.
	if _, err := scheduler.Register("32 3 * * *",
		nis2wizard.NewCleanupAnonymousRunsTask(),
		asynq.Unique(23*time.Hour),
	); err != nil {
		log.Error().Err(err).Msg("failed to register nis2 anonymous runs cleanup cron")
	}

	// Daily at 03:30 UTC: sweep evidence staleness for all controls (S67-4).
	if _, err := scheduler.Register("30 3 * * *",
		asynq.NewTask(vaktcomply.TaskEvidenceStalenessCheck, nil),
		asynq.Unique(23*time.Hour), asynq.Queue(vaktcomply.Queue),
	); err != nil {
		log.Error().Err(err).Msg("failed to register evidence staleness check cron")
	}

	// Daily at 03:45 UTC: revoke SCIM tokens that have passed their expires_at.
	if _, err := scheduler.Register("45 3 * * *",
		admin.NewSCIMTokenExpiryTask(),
		asynq.Unique(23*time.Hour),
	); err != nil {
		log.Error().Err(err).Msg("failed to register SCIM token expiry cron")
	}

	// --- 04:xx window ---

	// S69-3: Daily at 04:07 UTC — update sla_status on all open findings.
	if _, err := scheduler.Register("7 4 * * *",
		asynq.NewTask(vaktscan.TaskSLACheck, nil),
		asynq.Unique(23*time.Hour), asynq.Queue(vaktscan.QueueScans),
	); err != nil {
		log.Error().Err(err).Msg("failed to register sla check cron")
	}

	// Daily at 04:20 UTC: collect cloud evidence from all enabled AWS + Azure integrations.
	// Shifted from 04:00 to avoid collision with Watchtower restart window.
	if _, err := scheduler.Register("20 4 * * *",
		cloudintegration.NewCloudSyncTask(),
		asynq.Unique(23*time.Hour),
	); err != nil {
		log.Error().Err(err).Msg("failed to register cloud sync cron")
	}

	// Weekly Sunday at 04:30 UTC: clean up login_history rows older than 90d (S22-13).
	// Shifted from 04:00 to avoid collision with Watchtower and cloud sync.
	if _, err := scheduler.Register("30 4 * * 0",
		auth.NewCleanupLoginHistoryTask(),
		asynq.Unique(6*24*time.Hour),
	); err != nil {
		log.Error().Err(err).Msg("failed to register login history cleanup cron")
	}

	// --- 05:xx window ---

	// Daily at 05:08 UTC: collect GitHub Actions CI run evidence for all orgs.
	if _, err := scheduler.Register("8 5 * * *",
		asynq.NewTask(taskGitHubCISync, nil),
		asynq.Unique(23*time.Hour), asynq.Queue(vaktcomply.Queue),
	); err != nil {
		log.Error().Err(err).Msg("failed to register GitHub CI evidence sync cron")
	}

	// --- 06:xx window ---

	// Daily at 06:02 UTC: sync BSI CERT-Bund advisories and match to assets.
	// Opt-out via VAKT_BSI_FEED_ENABLED=false for air-gapped environments.
	if cfg != nil && cfg.BSIFeedEnabled {
		if _, err := scheduler.Register("2 6 * * *",
			bsi.NewBSIFeedSyncTask(),
			asynq.Unique(23*time.Hour),
		); err != nil {
			log.Error().Err(err).Msg("failed to register BSI feed sync cron")
		}
	} else {
		log.Info().Msg("BSI CERT-Bund feed sync disabled (VAKT_BSI_FEED_ENABLED=false)")
	}

	// S61-7: daily at 06:05 UTC — compute and persist ISMS KPI snapshots for all orgs.
	// Shifted +5min from 06:00 to avoid thundering-herd with BSI feed sync.
	if _, err := scheduler.Register("5 6 * * *",
		vaktcomply.NewISMSKPISnapshotTask(),
		asynq.Unique(23*time.Hour),
	); err != nil {
		log.Error().Err(err).Msg("failed to register ISMS KPI snapshot cron")
	}

	// S74-2: daily at 06:15 UTC — update BSI check progress in KPI snapshots.
	if _, err := scheduler.Register("15 6 * * *",
		vaktcomply.NewBSIKPISnapshotTask(),
		asynq.Unique(23*time.Hour),
	); err != nil {
		log.Error().Err(err).Msg("failed to register BSI KPI snapshot cron")
	}

	// Daily at 06:30 UTC: create CAPAs for controls whose test interval has elapsed.
	if _, err := scheduler.Register("30 6 * * *",
		asynq.NewTask(taskControlTestCheck, nil),
		asynq.Unique(23*time.Hour), asynq.Queue(vaktcomply.Queue),
	); err != nil {
		log.Error().Err(err).Msg("failed to register control test check cron")
	}

	// --- 07:xx window ---

	// Daily at 07:03 UTC: sync DER.4 evidence from BIA/WAP/contact data (S86-4).
	if _, err := scheduler.Register("3 7 * * *",
		asynq.NewTask(vaktcomply.TaskBCMEvidenceSync, nil),
		asynq.Unique(23*time.Hour), asynq.Queue(vaktcomply.Queue),
	); err != nil {
		log.Error().Err(err).Msg("failed to register BCM evidence sync cron")
	}

	// Daily at 07:05 UTC: check supplier certificate expiry.
	// Shifted +5min from 07:00 to avoid thundering-herd with BCM evidence sync.
	if _, err := scheduler.Register("5 7 * * *",
		vaktcomply.NewCertExpiryCheckTask(),
		asynq.Unique(23*time.Hour),
	); err != nil {
		log.Error().Err(err).Msg("failed to register cert expiry check cron")
	}

	// Daily at 07:30 UTC: flag overdue backups/restore tests, sync A.8.13 evidence (S88-2).
	if _, err := scheduler.Register("30 7 * * *",
		asynq.NewTask(vaktcomply.TaskBackupFreshnessCheck, nil),
		asynq.Unique(23*time.Hour), asynq.Queue(vaktcomply.Queue),
	); err != nil {
		log.Error().Err(err).Msg("failed to register backup freshness check cron")
	}

	// S70-4: daily at 07:35 UTC — check contractor expiry and mark expiring_soon/offboarding.
	// Shifted +5min from 07:30 to avoid thundering-herd with backup freshness check.
	if _, err := scheduler.Register("35 7 * * *",
		asynq.NewTask(vakthr.TaskContractorExpiryCheck, nil),
		asynq.Unique(23*time.Hour), asynq.Queue(vakthr.Queue),
	); err != nil {
		log.Error().Err(err).Msg("failed to register contractor expiry check cron")
	}

	// --- 08:xx window (staggered to avoid thundering herd) ---

	// Daily at 08:02 UTC: check AVV expiry and send alerts.
	if _, err := scheduler.Register("2 8 * * *",
		asynq.NewTask(vaktprivacy.TaskAVVExpiryCheck, nil),
		asynq.Unique(23*time.Hour), asynq.Queue(vaktprivacy.Queue),
	); err != nil {
		log.Error().Err(err).Msg("failed to register AVV expiry cron")
	}

	// Daily at 08:05 UTC: check for overdue SLA findings.
	// Shifted +5min from 08:00 to stagger the 08:xx cluster.
	if _, err := scheduler.Register("5 8 * * *",
		asynq.NewTask(alerting.TaskSLAOverdueCheck, nil),
		asynq.Unique(23*time.Hour),
	); err != nil {
		log.Error().Err(err).Msg("failed to register SLA overdue check cron")
	}

	// Daily at 08:10 UTC: check for overdue DSR requests.
	// Shifted +10min from 08:00 to stagger the 08:xx cluster.
	if _, err := scheduler.Register("10 8 * * *",
		asynq.NewTask(alerting.TaskDSROverdueCheck, nil),
		asynq.Unique(23*time.Hour),
	); err != nil {
		log.Error().Err(err).Msg("failed to register DSR overdue check cron")
	}

	// Daily at 08:15 UTC: send compliance deadline email alerts.
	if _, err := scheduler.Register("15 8 * * *",
		notifications.NewNotifyDeadlinesTask(),
		asynq.Unique(23*time.Hour),
	); err != nil {
		log.Error().Err(err).Msg("failed to register deadline notification cron")
	}

	// S68-2: daily at 08:17 UTC — mark overdue DSRs and send 3-day deadline warnings.
	if _, err := scheduler.Register("17 8 * * *",
		asynq.NewTask(vaktprivacy.TaskDSRDeadlineCheck, nil),
		asynq.Unique(23*time.Hour), asynq.Queue(vaktprivacy.Queue),
	); err != nil {
		log.Error().Err(err).Msg("failed to register dsr deadline check cron")
	}

	// S68-5: daily at 08:20 UTC — send deletion reminder notifications for reminders due within 14 days.
	if _, err := scheduler.Register("20 8 * * *",
		asynq.NewTask(vaktprivacy.TaskDeletionReminderCheck, nil),
		asynq.Unique(23*time.Hour), asynq.Queue(vaktprivacy.Queue),
	); err != nil {
		log.Error().Err(err).Msg("failed to register deletion reminder check cron")
	}

	// Daily at 08:25 UTC: process all due scheduled reports.
	// Shifted +25min from 08:00 to stagger the 08:xx cluster.
	if _, err := scheduler.Register("25 8 * * *",
		asynq.NewTask(scheduledreports.TaskProcessScheduledReports, nil),
		asynq.Unique(23*time.Hour),
	); err != nil {
		log.Error().Err(err).Msg("failed to register scheduled reports cron")
	}

	// Daily at 08:30 UTC: check NIS2-classified incidents for deadline alerts (S39-2).
	if _, err := scheduler.Register("30 8 * * *",
		vaktcomply.NewNIS2ObligationCheckTask(),
		asynq.Unique(23*time.Hour),
	); err != nil {
		log.Error().Err(err).Msg("failed to register nis2 obligation check cron")
	}

	// S52-1: daily at 08:35 UTC — evidence freshness AI insight generation.
	// Shifted +5min from 08:30 to avoid thundering-herd with NIS2 obligation check.
	if _, err := scheduler.Register("35 8 * * *",
		vaktcomply.NewEvidenceFreshnessCheckTask(),
		asynq.Unique(23*time.Hour),
	); err != nil {
		log.Error().Err(err).Msg("failed to register evidence freshness check cron")
	}

	// S52-4: every Monday at 08:01 UTC — AI compliance weekly digest (opt-in orgs only).
	if _, err := scheduler.Register("1 8 * * 1",
		vaktcomply.NewAIWeeklyDigestTask(),
		asynq.Unique(6*24*time.Hour),
	); err != nil {
		log.Error().Err(err).Msg("failed to register AI weekly digest cron")
	}

	// --- 09:xx window ---

	// Daily at 09:03 UTC: send control-owner due-date reminders (7-day advance notice).
	if _, err := scheduler.Register("3 9 * * *",
		asynq.NewTask(taskControlOwnerReminder, nil),
		asynq.Unique(23*time.Hour), asynq.Queue(vaktcomply.Queue),
	); err != nil {
		log.Error().Err(err).Msg("failed to register control owner reminder cron")
	}

	// Daily at 09:10 UTC: alert on evidence expiring within 30 days.
	// Shifted +10min from 09:00 to avoid thundering-herd with control owner reminder.
	if _, err := scheduler.Register("10 9 * * *",
		vaktcomply.NewEvidenceExpiryAlertTask(),
		asynq.Unique(23*time.Hour),
	); err != nil {
		log.Error().Err(err).Msg("failed to register evidence expiry alert cron")
	}

	// Every Monday at 09:05 UTC: compute and log the weekly SLO error budget report.
	if _, err := scheduler.Register("5 9 * * 1",
		asynq.NewTask(taskErrorBudgetReport, nil),
		asynq.Unique(6*24*time.Hour),
	); err != nil {
		log.Error().Err(err).Msg("failed to register error budget report cron")
	}

	// S61-3: Daily at 09:30 UTC — alert on major_nc CAPAs with overdue effectiveness checks.
	if _, err := scheduler.Register("30 9 * * *",
		asynq.NewTask(taskEffectivenessCheckOverdueAlert, nil),
		asynq.Unique(23*time.Hour), asynq.Queue(vaktcomply.Queue),
	); err != nil {
		log.Error().Err(err).Msg("failed to register effectiveness check overdue alert cron")
	}

	// --- 10:xx window ---

	// Daily at 10:04 UTC: run all due CCM checks.
	if _, err := scheduler.Register("4 10 * * *",
		vaktcomply.NewCCMRunDueTask(),
		asynq.Unique(23*time.Hour),
	); err != nil {
		log.Error().Err(err).Msg("failed to register CCM run-due cron")
	}

	// --- 23:xx window ---

	// Daily at 23:05 UTC: capture compliance score snapshot for trend charts.
	if _, err := scheduler.Register("5 23 * * *",
		vaktcomply.NewScoreSnapshotTask(),
		asynq.Unique(23*time.Hour),
	); err != nil {
		log.Error().Err(err).Msg("failed to register score snapshot cron")
	}

	// --- Quarterly ---

	// S70-5: 1st of Jan/Apr/Jul/Oct at 06:02 UTC — create quarterly vault access reviews.
	if _, err := scheduler.Register("12 6 1 1,4,7,10 *",
		asynq.NewTask(vaktvault.TaskQuarterlyAccessReview, nil),
		asynq.Unique(89*24*time.Hour), asynq.Queue(vaktvault.Queue),
	); err != nil {
		log.Error().Err(err).Msg("failed to register quarterly access review cron")
	}

	// --- Sub-minute / high-frequency tasks ---

	// Every 5 minutes: update DORA IKT-incident Ampel-Status (S37-4).
	if _, err := scheduler.Register("*/5 * * * *",
		vaktcomply.NewDORADeadlineStatusTask(),
		asynq.Unique(6*time.Minute),
	); err != nil {
		log.Error().Err(err).Msg("failed to register DORA deadline status cron")
	}

	// Every 4 hours (at :03): check for overdue DORA/NIS2 incident deadlines.
	if _, err := scheduler.Register("3 */4 * * *",
		vaktcomply.NewIncidentDeadlineCheckTask(),
		asynq.Unique(230*time.Minute),
	); err != nil {
		log.Error().Err(err).Msg("failed to register incident deadline check cron")
	}

	// Every 5 minutes: forward pending audit entries to configured SIEM backends.
	if _, err := scheduler.Register("*/5 * * * *",
		siem.NewSIEMForwardTask(),
		asynq.Unique(6*time.Minute),
	); err != nil {
		log.Error().Err(err).Msg("failed to register siem forward cron")
	}

	// Every 5 minutes: check for failed/archived job accumulation.
	if _, err := scheduler.Register("*/5 * * * *",
		asynq.NewTask(taskQueueHealthCheck, nil),
		asynq.Unique(6*time.Minute),
	); err != nil {
		log.Error().Err(err).Msg("failed to register queue health check cron")
	}

	// Hourly: send weekly digest to orgs whose configured weekday+hour matches now.
	if _, err := scheduler.Register("2 * * * *",
		emaildigest.NewWeeklyDigestTask(),
		asynq.Unique(65*time.Minute),
	); err != nil {
		log.Error().Err(err).Msg("failed to register digest cron")
	}

	// Hourly: delete ephemeral demo orgs older than 4 hours (demo instances only).
	if cfg != nil && cfg.DemoSeed {
		if _, err := scheduler.Register("2 * * * *",
			demo.NewCleanupTask(),
			asynq.Unique(65*time.Minute),
		); err != nil {
			log.Error().Err(err).Msg("failed to register demo cleanup cron")
		}
	}

	return scheduler
}

const taskQueueHealthCheck = "queue:health:check"

// taskControlTestCheck is the Asynq task name for the daily overdue control test CAPA check.
const taskControlTestCheck = "vaktcomply:control_test_check"

// taskErrorBudgetReport is the Asynq task name for the weekly SLO error budget report.
const taskErrorBudgetReport = "errorbudget:weekly_report"

// handleControlTestCheck creates CAPAs for controls whose test_interval_days has elapsed.
