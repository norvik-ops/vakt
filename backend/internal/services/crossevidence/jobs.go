package crossevidence

import (
	"encoding/json"
	"fmt"

	"github.com/hibiken/asynq"

	"github.com/matharnica/vakt/internal/shared/platform/events"
)

// TaskRecordEvidence is the Asynq task type for cross-module evidence recording.
// It is enqueued by SecReflex, SecPrivacy, and SecVault when compliance-relevant
// events occur, and processed by the worker which calls the SecVitals evidence API.
const (
	TaskRecordEvidence = "vaktcomply:record_evidence"

	// Queue is the dedicated Asynq queue for cross-module evidence and compliance jobs.
	Queue = "vaktcomply"
)

// EvidencePayload is an alias for events.CrossModuleEvent.
// Use the typed constructors in the events package (events.DSRCompleted,
// events.SecretRotated, events.TrainingCompleted, etc.) instead of building
// this struct directly. See ADR-0023.
type EvidencePayload = events.CrossModuleEvent

// NewRecordEvidenceTask creates a new Asynq evidence recording task from a typed event.
func NewRecordEvidenceTask(p EvidencePayload) (*asynq.Task, error) {
	payload, err := json.Marshal(p)
	if err != nil {
		return nil, fmt.Errorf("marshal evidence payload: %w", err)
	}
	return asynq.NewTask(TaskRecordEvidence, payload, asynq.Queue(Queue)), nil
}
