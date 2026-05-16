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
