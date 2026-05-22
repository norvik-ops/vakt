package siem

import "github.com/hibiken/asynq"

// TaskSIEMForward is the Asynq task name for the periodic SIEM forward job.
const TaskSIEMForward = "siem:forward_pending"

// NewSIEMForwardTask creates an Asynq task for the SIEM forward job.
func NewSIEMForwardTask() *asynq.Task {
	return asynq.NewTask(TaskSIEMForward, nil)
}
