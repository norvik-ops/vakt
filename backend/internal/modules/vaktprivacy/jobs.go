package vaktprivacy

import "time"

// Job type constants for SecPrivacy Asynq tasks.
const (
	// TaskAVVExpiryCheck runs daily to mark expired AVVs and send expiry alerts.
	TaskAVVExpiryCheck = "vaktprivacy:avv_expiry_check"

	// TaskBreachIncidentCreate creates a linked SecVitals incident when a breach is recorded.
	TaskBreachIncidentCreate = "vaktprivacy:breach_incident_create"

	// Queue is the dedicated Asynq queue for Vakt Privacy jobs (breach notifications, AVV checks).
	Queue = "vaktprivacy"
)

// BreachIncidentPayload is the Asynq payload for TaskBreachIncidentCreate.
type BreachIncidentPayload struct {
	OrgID        string    `json:"org_id"`
	BreachID     string    `json:"breach_id"`
	Title        string    `json:"title"`
	Description  string    `json:"description"`
	DiscoveredAt time.Time `json:"discovered_at"`
}
