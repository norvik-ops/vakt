package secvitals

import (
	"time"

	"github.com/hibiken/asynq"
)

// Job type constants for ComplyKit Asynq tasks.
const (
	// TaskEvidenceExpiryAlert is the task type for daily evidence expiry alerts.
	TaskEvidenceExpiryAlert = "secvitals:evidence_expiry_alert"

	// TaskIncidentDeadlineCheck is the task type for checking overdue DORA/NIS2 incident deadlines.
	TaskIncidentDeadlineCheck = "secvitals:incident_deadline_check"

	// TaskCertExpiryCheck is the task type for daily supplier certificate expiry checks.
	TaskCertExpiryCheck = "secvitals:cert_expiry_check"

	// TaskCCMRunDue is the task type for running all due CCM checks.
	TaskCCMRunDue = "secvitals:ccm:run_due"

	// TaskScoreSnapshot is the task type for daily compliance score snapshots.
	TaskScoreSnapshot = "secvitals:score_snapshot"

	// TaskDORADeadlineStatus is the task type for computing DORA Ampel-Status every 5 minutes (S37-4).
	TaskDORADeadlineStatus = "secvitals:dora_deadline_status"

	// TaskNIS2ObligationCheck is the task type for checking NIS2-classified incident deadlines (S39-2).
	// It specifically targets incidents where classification_result.obligation = "probably".
	TaskNIS2ObligationCheck = "secvitals:nis2_obligation_check"

	// Queue is the dedicated Asynq queue for Vakt Comply evidence and compliance jobs.
	Queue = "secvitals"
)

// NewEvidenceExpiryAlertTask creates a new evidence expiry alert task.
// The Unique option prevents duplicate tasks within a 23-hour window.
func NewEvidenceExpiryAlertTask() *asynq.Task {
	return asynq.NewTask(TaskEvidenceExpiryAlert, nil, asynq.Unique(23*time.Hour))
}

// NewIncidentDeadlineCheckTask creates a new incident deadline check task.
// The Unique option prevents duplicate tasks within a 23-hour window.
func NewIncidentDeadlineCheckTask() *asynq.Task {
	return asynq.NewTask(TaskIncidentDeadlineCheck, nil, asynq.Unique(23*time.Hour))
}

// NewCertExpiryCheckTask creates a new supplier certificate expiry check task.
// The Unique option prevents duplicate tasks within a 23-hour window.
func NewCertExpiryCheckTask() *asynq.Task {
	return asynq.NewTask(TaskCertExpiryCheck, nil, asynq.Unique(23*time.Hour))
}

// NewCCMRunDueTask creates a new task to run all due CCM checks.
// The Unique option prevents duplicate tasks within a 23-hour window.
func NewCCMRunDueTask() *asynq.Task {
	return asynq.NewTask(TaskCCMRunDue, nil, asynq.Unique(23*time.Hour))
}

// NewScoreSnapshotTask creates a new daily compliance score snapshot task.
// The Unique option prevents duplicate tasks within a 23-hour window.
func NewScoreSnapshotTask() *asynq.Task {
	return asynq.NewTask(TaskScoreSnapshot, nil, asynq.Unique(23*time.Hour))
}

// NewDORADeadlineStatusTask creates a task for computing DORA Ampel-Status (S37-4).
// Unique window of 4 minutes prevents duplicate executions within a 5-minute cron interval.
func NewDORADeadlineStatusTask() *asynq.Task {
	return asynq.NewTask(TaskDORADeadlineStatus, nil, asynq.Unique(4*time.Minute))
}

// NewNIS2ObligationCheckTask creates a task for checking NIS2-classified incident deadlines (S39-2).
// Unique window of 23 hours prevents duplicate tasks within a daily cron window.
func NewNIS2ObligationCheckTask() *asynq.Task {
	return asynq.NewTask(TaskNIS2ObligationCheck, nil, asynq.Unique(23*time.Hour))
}
