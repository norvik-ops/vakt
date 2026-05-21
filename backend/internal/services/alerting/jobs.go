package alerting

// Asynq task type names for scheduled alerting jobs.
const (
	// TaskSLAOverdueCheck fires daily to detect findings past their SLA deadline.
	TaskSLAOverdueCheck = "alerting:sla_overdue_check"

	// TaskDSROverdueCheck fires daily to detect DSR requests past their due date.
	TaskDSROverdueCheck = "alerting:dsr_overdue_check"
)
