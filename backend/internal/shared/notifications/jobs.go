package notifications

import (
	"time"

	"github.com/hibiken/asynq"
)

// TaskNotifyDeadlines is the Asynq task type for the daily compliance deadline check.
// It runs all four alert functions (breach, DSR, AVV, CCM) in one job.
const TaskNotifyDeadlines = "notifications:check_deadlines"

// NewNotifyDeadlinesTask creates the deadline notification task.
// The Unique option prevents duplicate tasks within a 23-hour window so that
// multiple scheduler instances cannot fire the same job twice on the same day.
func NewNotifyDeadlinesTask() *asynq.Task {
	return asynq.NewTask(TaskNotifyDeadlines, nil, asynq.Unique(23*time.Hour))
}
