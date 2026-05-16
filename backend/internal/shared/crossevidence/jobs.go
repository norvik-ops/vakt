package crossevidence

import (
	"encoding/json"
	"fmt"
	"time"

	"github.com/hibiken/asynq"
)

// TaskRecordEvidence is the Asynq task type for cross-module evidence recording.
// It is enqueued by SecReflex, SecPrivacy, and SecVault when compliance-relevant
// events occur, and processed by the worker which calls the SecVitals evidence API.
const TaskRecordEvidence = "secvitals:record_evidence"

// EvidencePayload is the task payload for cross-module evidence recording.
type EvidencePayload struct {
	OrgID        string    `json:"org_id"`
	Source       string    `json:"source"`        // "secreflex" | "secprivacy" | "secvault"
	ResourceType string    `json:"resource_type"` // "training_completion" | "dsr_completed" | "secret_rotated"
	ResourceID   string    `json:"resource_id"`
	Title        string    `json:"title"`
	Description  string    `json:"description"`
	OccurredAt   time.Time `json:"occurred_at"`
}

// NewRecordEvidenceTask creates a new evidence recording task.
func NewRecordEvidenceTask(p EvidencePayload) (*asynq.Task, error) {
	payload, err := json.Marshal(p)
	if err != nil {
		return nil, fmt.Errorf("marshal evidence payload: %w", err)
	}
	return asynq.NewTask(TaskRecordEvidence, payload, asynq.Queue("low")), nil
}
