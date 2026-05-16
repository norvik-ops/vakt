package emaildigest

import (
	"time"

	"github.com/hibiken/asynq"
)

// TaskWeeklyDigest is the Asynq task type for the weekly e-mail digest job.
// The job runs every Monday at 08:00 UTC via the Asynq scheduler.
const TaskWeeklyDigest = "digest:weekly"

// NewWeeklyDigestTask creates the weekly digest task with a 7-day uniqueness lock
// to prevent duplicate runs when multiple worker instances are running.
func NewWeeklyDigestTask() *asynq.Task {
	return asynq.NewTask(TaskWeeklyDigest, nil, asynq.Unique(7*24*time.Hour))
}
