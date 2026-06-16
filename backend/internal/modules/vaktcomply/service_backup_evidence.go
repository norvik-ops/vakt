// Copyright (c) 2026 NorvikOps. All rights reserved.
// SPDX-License-Identifier: Elastic-2.0

// S88-2: Backup-/Restore-Nachweis-Workflow (ISO 27001:2022 A.8.13, BSI DER.4).
// Documentation-first: a NACHWEIS registry with staleness detection and an
// evidence bridge into the compliance controls — Vakt never runs backups itself.

package vaktcomply

import (
	"context"
	"fmt"
	"time"

	"github.com/rs/zerolog/log"
)

// TaskBackupFreshnessCheck is the daily Asynq task that flags overdue backups /
// restore tests and syncs A.8.13 + DER.4 evidence.
const TaskBackupFreshnessCheck = "comply:backup_freshness_check"

// frequencyInterval maps a backup frequency to its expected interval.
func frequencyInterval(freq string) time.Duration {
	switch freq {
	case "hourly":
		return time.Hour
	case "weekly":
		return 7 * 24 * time.Hour
	case "monthly":
		return 31 * 24 * time.Hour
	default: // daily
		return 24 * time.Hour
	}
}

// backupStaleness returns on_track|at_risk|overdue for the last successful backup
// relative to its configured frequency. A missing/failed last success is overdue.
func backupStaleness(lastSuccess *time.Time, freq, lastStatus string, now time.Time) string {
	if lastSuccess == nil || lastStatus == "failed" {
		return "overdue"
	}
	interval := frequencyInterval(freq)
	age := now.Sub(*lastSuccess)
	switch {
	case age > 2*interval:
		return "overdue"
	case age > interval:
		return "at_risk"
	default:
		return "on_track"
	}
}

// restoreStaleness returns on_track|at_risk|overdue for the most recent restore
// test relative to restore_max_age_days. A missing test is overdue.
func restoreStaleness(lastTest *time.Time, maxAgeDays int, now time.Time) string {
	if lastTest == nil {
		return "overdue"
	}
	if maxAgeDays <= 0 {
		maxAgeDays = 365
	}
	maxAge := time.Duration(maxAgeDays) * 24 * time.Hour
	age := now.Sub(*lastTest)
	switch {
	case age > maxAge:
		return "overdue"
	case age > time.Duration(float64(maxAge)*0.8):
		return "at_risk"
	default:
		return "on_track"
	}
}

// ── Backup Jobs CRUD (raw SQL, org-scoped) ─────────────────────────────────

func (s *Service) ListBackupJobs(ctx context.Context, orgID string) ([]BackupJob, error) {
	rows, err := s.db.Query(ctx, `
		SELECT j.id::text, j.org_id::text, j.name, j.source, j.destination, j.frequency,
		       j.encrypted, j.last_success_at, j.last_status, j.restore_max_age_days,
		       j.notes, j.created_at, j.updated_at,
		       (SELECT MAX(t.tested_at) FROM ck_backup_restore_tests t
		          WHERE t.job_id = j.id AND t.result <> 'failed') AS last_restore_test_at
		FROM ck_backup_jobs j
		WHERE j.org_id = $1
		ORDER BY j.name ASC`, orgID)
	if err != nil {
		return nil, fmt.Errorf("list backup jobs: %w", err)
	}
	defer rows.Close()

	now := time.Now().UTC()
	out := []BackupJob{}
	for rows.Next() {
		var j BackupJob
		var lastRestore *time.Time
		if err := rows.Scan(&j.ID, &j.OrgID, &j.Name, &j.Source, &j.Destination, &j.Frequency,
			&j.Encrypted, &j.LastSuccessAt, &j.LastStatus, &j.RestoreMaxAgeDays,
			&j.Notes, &j.CreatedAt, &j.UpdatedAt, &lastRestore); err != nil {
			return nil, fmt.Errorf("scan backup job: %w", err)
		}
		j.LastRestoreTestAt = lastRestore
		j.BackupStatus = backupStaleness(j.LastSuccessAt, j.Frequency, j.LastStatus, now)
		j.RestoreStatus = restoreStaleness(lastRestore, j.RestoreMaxAgeDays, now)
		out = append(out, j)
	}
	return out, rows.Err()
}

func (s *Service) CreateBackupJob(ctx context.Context, orgID string, in BackupJobInput) (BackupJob, error) {
	if in.RestoreMaxAgeDays == 0 {
		in.RestoreMaxAgeDays = 365
	}
	if in.LastStatus == "" {
		in.LastStatus = "unknown"
	}
	var lastSuccess *time.Time
	if in.LastSuccessAt != "" {
		t, err := time.Parse(time.RFC3339, in.LastSuccessAt)
		if err != nil {
			return BackupJob{}, fmt.Errorf("parse last_success_at: %w", err)
		}
		lastSuccess = &t
	}
	var id string
	err := s.db.QueryRow(ctx, `
		INSERT INTO ck_backup_jobs
		  (org_id, name, source, destination, frequency, encrypted,
		   last_success_at, last_status, restore_max_age_days, notes)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10)
		RETURNING id::text`,
		orgID, in.Name, in.Source, in.Destination, in.Frequency, in.Encrypted,
		lastSuccess, in.LastStatus, in.RestoreMaxAgeDays, in.Notes).Scan(&id)
	if err != nil {
		return BackupJob{}, fmt.Errorf("create backup job: %w", err)
	}
	return s.GetBackupJob(ctx, orgID, id)
}

func (s *Service) GetBackupJob(ctx context.Context, orgID, id string) (BackupJob, error) {
	jobs, err := s.ListBackupJobs(ctx, orgID)
	if err != nil {
		return BackupJob{}, err
	}
	for _, j := range jobs {
		if j.ID == id {
			return j, nil
		}
	}
	return BackupJob{}, fmt.Errorf("backup job not found")
}

func (s *Service) UpdateBackupJob(ctx context.Context, orgID, id string, in BackupJobInput) (BackupJob, error) {
	if in.RestoreMaxAgeDays == 0 {
		in.RestoreMaxAgeDays = 365
	}
	if in.LastStatus == "" {
		in.LastStatus = "unknown"
	}
	var lastSuccess *time.Time
	if in.LastSuccessAt != "" {
		t, err := time.Parse(time.RFC3339, in.LastSuccessAt)
		if err != nil {
			return BackupJob{}, fmt.Errorf("parse last_success_at: %w", err)
		}
		lastSuccess = &t
	}
	tag, err := s.db.Exec(ctx, `
		UPDATE ck_backup_jobs
		SET name=$3, source=$4, destination=$5, frequency=$6, encrypted=$7,
		    last_success_at=$8, last_status=$9, restore_max_age_days=$10, notes=$11,
		    updated_at=NOW()
		WHERE id=$1 AND org_id=$2`,
		id, orgID, in.Name, in.Source, in.Destination, in.Frequency, in.Encrypted,
		lastSuccess, in.LastStatus, in.RestoreMaxAgeDays, in.Notes)
	if err != nil {
		return BackupJob{}, fmt.Errorf("update backup job: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return BackupJob{}, fmt.Errorf("backup job not found")
	}
	return s.GetBackupJob(ctx, orgID, id)
}

func (s *Service) DeleteBackupJob(ctx context.Context, orgID, id string) error {
	tag, err := s.db.Exec(ctx, `DELETE FROM ck_backup_jobs WHERE id=$1 AND org_id=$2`, id, orgID)
	if err != nil {
		return fmt.Errorf("delete backup job: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return fmt.Errorf("backup job not found")
	}
	return nil
}

// ── Restore Tests ──────────────────────────────────────────────────────────

func (s *Service) ListRestoreTests(ctx context.Context, orgID, jobID string) ([]BackupRestoreTest, error) {
	rows, err := s.db.Query(ctx, `
		SELECT id::text, org_id::text, job_id::text, tested_at, result,
		       rto_target_hours, rto_actual_hours, tester, notes, created_at
		FROM ck_backup_restore_tests
		WHERE org_id=$1 AND job_id=$2
		ORDER BY tested_at DESC`, orgID, jobID)
	if err != nil {
		return nil, fmt.Errorf("list restore tests: %w", err)
	}
	defer rows.Close()
	out := []BackupRestoreTest{}
	for rows.Next() {
		var t BackupRestoreTest
		var tested time.Time
		if err := rows.Scan(&t.ID, &t.OrgID, &t.JobID, &tested, &t.Result,
			&t.RTOTargetHours, &t.RTOActualHours, &t.Tester, &t.Notes, &t.CreatedAt); err != nil {
			return nil, fmt.Errorf("scan restore test: %w", err)
		}
		t.TestedAt = tested.Format("2006-01-02")
		out = append(out, t)
	}
	return out, rows.Err()
}

func (s *Service) CreateRestoreTest(ctx context.Context, orgID, jobID string, in RestoreTestInput) (BackupRestoreTest, error) {
	// Verify the job belongs to the org (defence against cross-org job_id).
	var exists bool
	if err := s.db.QueryRow(ctx,
		`SELECT EXISTS(SELECT 1 FROM ck_backup_jobs WHERE id=$1 AND org_id=$2)`,
		jobID, orgID).Scan(&exists); err != nil {
		return BackupRestoreTest{}, fmt.Errorf("verify job: %w", err)
	}
	if !exists {
		return BackupRestoreTest{}, fmt.Errorf("backup job not found")
	}
	tested, err := time.Parse("2006-01-02", in.TestedAt)
	if err != nil {
		return BackupRestoreTest{}, fmt.Errorf("parse tested_at: %w", err)
	}
	var id string
	err = s.db.QueryRow(ctx, `
		INSERT INTO ck_backup_restore_tests
		  (org_id, job_id, tested_at, result, rto_target_hours, rto_actual_hours, tester, notes)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8)
		RETURNING id::text`,
		orgID, jobID, tested, in.Result, in.RTOTargetHours, in.RTOActualHours, in.Tester, in.Notes).Scan(&id)
	if err != nil {
		return BackupRestoreTest{}, fmt.Errorf("create restore test: %w", err)
	}
	// Sync evidence opportunistically (best-effort).
	if err := s.SyncBackupEvidence(ctx, orgID); err != nil {
		log.Warn().Err(err).Str("org_id", orgID).Msg("backup evidence sync after restore test")
	}
	tests, err := s.ListRestoreTests(ctx, orgID, jobID)
	if err != nil {
		return BackupRestoreTest{}, err
	}
	for _, t := range tests {
		if t.ID == id {
			return t, nil
		}
	}
	return BackupRestoreTest{}, fmt.Errorf("restore test not found after insert")
}

// ── Summary + Evidence sync ────────────────────────────────────────────────

func (s *Service) GetBackupSummary(ctx context.Context, orgID string) (BackupSummary, error) {
	jobs, err := s.ListBackupJobs(ctx, orgID)
	if err != nil {
		return BackupSummary{}, err
	}
	var sum BackupSummary
	sum.TotalJobs = len(jobs)
	for _, j := range jobs {
		if j.BackupStatus == "overdue" {
			sum.OverdueBackups++
		}
		if j.RestoreStatus == "overdue" {
			sum.OverdueRestores++
		}
		if j.LastRestoreTestAt != nil {
			sum.TestedJobs++
		}
	}
	return sum, nil
}

// SyncBackupEvidence attaches A.8.13 (and DER.4.A1 once BSI is enabled) evidence
// when the org has at least one backup job with a passed restore test on record.
func (s *Service) SyncBackupEvidence(ctx context.Context, orgID string) error {
	jobs, err := s.ListBackupJobs(ctx, orgID)
	if err != nil {
		return fmt.Errorf("backup evidence: list jobs: %w", err)
	}
	tested := 0
	for _, j := range jobs {
		if j.LastRestoreTestAt != nil && j.RestoreStatus != "overdue" {
			tested++
		}
	}
	if tested == 0 {
		return nil
	}
	title := "Backup-/Restore-Nachweis dokumentiert"
	desc := fmt.Sprintf("%d Backup-Job(s) mit aktuellem Restore-Test dokumentiert (ISO 27001 A.8.13)", tested)
	// ISO 27001:2022 A.8.13 — Information backup.
	s.ensureBackupEvidence(ctx, orgID, "A.8.13", title, desc)
	// BSI DER.4.A1 — once the BSI catalog is enabled in the org (no-op otherwise).
	s.ensureBackupEvidence(ctx, orgID, "BSI-DER.4.A1", title, desc)
	return nil
}

// ensureBackupEvidence idempotently attaches collector evidence to a control.
func (s *Service) ensureBackupEvidence(ctx context.Context, orgID, controlCode, title, desc string) {
	controlID, err := s.repo.FindControlByCode(ctx, orgID, controlCode)
	if err != nil || controlID == "" {
		return // control / framework not present — skip silently
	}
	payload := []byte(fmt.Sprintf(`{"source":"backup_evidence","control_code":%q,"description":%q}`, controlCode, desc))
	if _, err := s.repo.AddCollectorEvidence(ctx, orgID, controlID, "", "automated", title, payload); err != nil {
		log.Warn().Err(err).Str("org_id", orgID).Str("control", controlCode).Msg("backup evidence sync")
	}
}

// CheckBackupFreshness is the body of the daily Asynq job: it syncs evidence and
// returns the count of overdue items for logging. Reminders are surfaced via the
// dashboard summary; this keeps the write path simple and idempotent.
func (s *Service) CheckBackupFreshness(ctx context.Context, orgID string) (overdue int, err error) {
	sum, err := s.GetBackupSummary(ctx, orgID)
	if err != nil {
		return 0, err
	}
	if err := s.SyncBackupEvidence(ctx, orgID); err != nil {
		return 0, err
	}
	return sum.OverdueBackups + sum.OverdueRestores, nil
}
