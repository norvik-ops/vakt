package siem

import (
	"time"

	"github.com/hibiken/asynq"
)

// TaskSIEMForward is the Asynq task name for the periodic SIEM forward job.
const TaskSIEMForward = "siem:forward_pending"

// NewSIEMForwardTask creates an Asynq task for the SIEM forward job.
// Unique(4m) prevents duplicate enqueues when two workers run concurrently.
func NewSIEMForwardTask() *asynq.Task {
	return asynq.NewTask(TaskSIEMForward, nil, asynq.Unique(4*time.Minute))
}
