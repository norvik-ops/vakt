package vaktaware

const (
	TaskSendCampaign     = "vaktaware:send_campaign"
	TaskTrainingReminder = "vaktaware:training_reminder"

	// Queue is the dedicated Asynq queue for Vakt Aware campaign and training jobs.
	Queue = "vaktaware"
)

// AwarenessRiskPayload carries aggregated (never individual) campaign stats to the
// awareness risk sync handler. Individual target data is intentionally omitted to
// comply with Betriebsrat anonymity requirements.
type AwarenessRiskPayload struct {
	OrgID           string  `json:"org_id"`
	CampaignID      string  `json:"campaign_id"`
	CampaignName    string  `json:"campaign_name"`
	ClickRate       float64 `json:"click_rate"` // percentage, e.g. 23.5 for 23.5%
	TotalTargets    int     `json:"total_targets"`
	BetriebsratMode bool    `json:"betriebsrat_mode"` // guard: handler must check and abort if true
}
