// Copyright (c) 2026 NorvikOps. All rights reserved.
// SPDX-License-Identifier: Elastic-2.0

// S88-2: Backup-/Restore-Nachweis-Workflow (ISO A.8.13 / BSI DER.4).

package vaktcomply

import "time"

// BackupJob is a documentation-only record of a backup job. Vakt does not run
// backups — it tracks the evidence an auditor asks for ("when did the last
// successful backup / restore test happen?").
type BackupJob struct {
	ID                string     `json:"id"`
	OrgID             string     `json:"org_id"`
	Name              string     `json:"name"`
	Source            string     `json:"source"`
	Destination       string     `json:"destination"`
	Frequency         string     `json:"frequency"` // hourly|daily|weekly|monthly
	Encrypted         bool       `json:"encrypted"`
	LastSuccessAt     *time.Time `json:"last_success_at"`
	LastStatus        string     `json:"last_status"` // unknown|success|failed
	RestoreMaxAgeDays int        `json:"restore_max_age_days"`
	Notes             string     `json:"notes"`
	CreatedAt         time.Time  `json:"created_at"`
	UpdatedAt         time.Time  `json:"updated_at"`

	// Derived, not stored:
	LastRestoreTestAt *time.Time `json:"last_restore_test_at,omitempty"`
	// Staleness status: on_track | at_risk | overdue
	BackupStatus  string `json:"backup_status"`
	RestoreStatus string `json:"restore_status"`
}

// BackupRestoreTest documents a restore-test run for a backup job.
type BackupRestoreTest struct {
	ID             string    `json:"id"`
	OrgID          string    `json:"org_id"`
	JobID          string    `json:"job_id"`
	TestedAt       string    `json:"tested_at"` // YYYY-MM-DD
	Result         string    `json:"result"`    // success|partial|failed
	RTOTargetHours int       `json:"rto_target_hours"`
	RTOActualHours int       `json:"rto_actual_hours"`
	Tester         string    `json:"tester"`
	Notes          string    `json:"notes"`
	CreatedAt      time.Time `json:"created_at"`
}

// CreateBackupJobInput / UpdateBackupJobInput hold validated create/update data.
type BackupJobInput struct {
	Name              string `json:"name" validate:"required,max=200"`
	Source            string `json:"source" validate:"max=500"`
	Destination       string `json:"destination" validate:"max=500"`
	Frequency         string `json:"frequency" validate:"required,oneof=hourly daily weekly monthly"`
	Encrypted         bool   `json:"encrypted"`
	RestoreMaxAgeDays int    `json:"restore_max_age_days" validate:"min=1,max=3650"`
	Notes             string `json:"notes" validate:"max=2000"`
	LastStatus        string `json:"last_status" validate:"omitempty,oneof=unknown success failed"`
	LastSuccessAt     string `json:"last_success_at"` // RFC3339, optional
}

// RestoreTestInput holds validated restore-test data.
type RestoreTestInput struct {
	TestedAt       string `json:"tested_at" validate:"required"` // YYYY-MM-DD
	Result         string `json:"result" validate:"required,oneof=success partial failed"`
	RTOTargetHours int    `json:"rto_target_hours" validate:"min=0,max=8760"`
	RTOActualHours int    `json:"rto_actual_hours" validate:"min=0,max=8760"`
	Tester         string `json:"tester" validate:"max=200"`
	Notes          string `json:"notes" validate:"max=2000"`
}

// BackupSummary aggregates backup-evidence health for the dashboard.
type BackupSummary struct {
	TotalJobs       int `json:"total_jobs"`
	OverdueBackups  int `json:"overdue_backups"`
	OverdueRestores int `json:"overdue_restores"`
	TestedJobs      int `json:"tested_jobs"`
}
