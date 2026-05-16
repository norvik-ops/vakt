package retention

import (
	"time"

	"github.com/hibiken/asynq"
)

// TaskRetentionRun is the Asynq task type for the daily data-retention job.
// The handler iterates over all orgs with a retention_config row and prunes
// expired data according to each org's configured retention periods.
const TaskRetentionRun = "retention:run"

// NewRetentionRunTask creates the daily retention job with a 23h uniqueness lock
// to prevent duplicate runs when multiple worker instances are running.
func NewRetentionRunTask() *asynq.Task {
	return asynq.NewTask(TaskRetentionRun, nil, asynq.Unique(23*time.Hour))
}
