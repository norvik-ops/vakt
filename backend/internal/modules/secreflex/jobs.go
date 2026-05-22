package secreflex

const (
	TaskSendCampaign     = "secreflex:send_campaign"
	TaskTrainingReminder = "secreflex:training_reminder"

	// Queue is the dedicated Asynq queue for Vakt Aware campaign and training jobs.
	Queue = "secreflex"
)
