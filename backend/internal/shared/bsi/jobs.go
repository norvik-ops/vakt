package bsi

import (
	"time"

	"github.com/hibiken/asynq"
)

// TaskBSIFeedSync is the Asynq task type for the daily BSI CERT-Bund feed sync.
// The job fetches new advisories and creates SecPulse findings for affected assets.
// It is scheduled to run daily at 06:00 UTC.
const TaskBSIFeedSync = "bsi:feed_sync"

// NewBSIFeedSyncTask creates the daily BSI feed sync task with a 23h uniqueness lock
// to prevent duplicate runs when multiple worker instances are running.
func NewBSIFeedSyncTask() *asynq.Task {
	return asynq.NewTask(TaskBSIFeedSync, nil, asynq.Unique(23*time.Hour))
}
