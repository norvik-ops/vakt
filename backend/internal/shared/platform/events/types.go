// Package events defines the typed cross-module event vocabulary for Vakt.
// All modules that emit compliance-relevant events should use these constructors
// rather than building raw EvidencePayload structs with unvalidated string fields.
// See ADR-0023 for the rationale and migration path.
package events

import "time"

// Source identifies which Vakt module emitted the event.
const (
	SourceSecpulse   = "vaktscan"
	SourceSecprivacy = "vaktprivacy"
	SourceSecvault   = "vaktvault"
	SourceSecreflex  = "vaktaware"
	SourceSecvitals  = "vaktcomply"
	SourceHR         = "vakthr"
)

// ResourceType identifies what kind of event occurred.
const (
	ResourceTypeFindingCreated    = "vakt-scan/finding-created"
	ResourceTypeBreachNotified    = "vakt-privacy/breach-notified"
	ResourceTypeDSRCompleted      = "vakt-privacy/dsr-completed"
	ResourceTypeSecretRotated     = "vakt-vault/secret-rotated"
	ResourceTypeTrainingCompleted = "vakt-aware/training-completion"
	ResourceTypeIncidentCreated   = "vakt-comply/incident-created"
	ResourceTypeEvidenceCollected = "vakt-comply/evidence-collected"
	ResourceTypeCertExpiring      = "vakt-scan/cert-expiring"
)

// CrossModuleEvent is the canonical envelope for all cross-module events.
// The fields map 1:1 to crossevidence.EvidencePayload so callers can
// switch between the two without data loss.
type CrossModuleEvent struct {
	OrgID        string    `json:"org_id"`
	Source       string    `json:"source"`
	ResourceType string    `json:"resource_type"`
	ResourceID   string    `json:"resource_id"`
	Title        string    `json:"title"`
	Description  string    `json:"description"`
	OccurredAt   time.Time `json:"occurred_at"`
}

// FindingCreated constructs a CrossModuleEvent for a new scanner finding.
func FindingCreated(orgID, findingID, title, severity string) CrossModuleEvent {
	return CrossModuleEvent{
		OrgID:        orgID,
		Source:       SourceSecpulse,
		ResourceType: ResourceTypeFindingCreated,
		ResourceID:   findingID,
		Title:        title,
		Description:  "Scanner-Finding erstellt. Schweregrad: " + severity,
		OccurredAt:   time.Now().UTC(),
	}
}

// BreachNotified constructs a CrossModuleEvent for a DSGVO breach notification.
func BreachNotified(orgID, breachID, title string) CrossModuleEvent {
	return CrossModuleEvent{
		OrgID:        orgID,
		Source:       SourceSecprivacy,
		ResourceType: ResourceTypeBreachNotified,
		ResourceID:   breachID,
		Title:        title,
		Description:  "Datenpanne gem. Art. 33 DSGVO gemeldet und dokumentiert.",
		OccurredAt:   time.Now().UTC(),
	}
}

// DSRCompleted constructs a CrossModuleEvent when a data subject request is resolved.
func DSRCompleted(orgID, dsrID string) CrossModuleEvent {
	return CrossModuleEvent{
		OrgID:        orgID,
		Source:       SourceSecprivacy,
		ResourceType: ResourceTypeDSRCompleted,
		ResourceID:   dsrID,
		Title:        "Betroffenenanfrage (DSR) abgeschlossen",
		Description:  "Eine Datenschutz-Betroffenenanfrage wurde vollständig bearbeitet und abgeschlossen.",
		OccurredAt:   time.Now().UTC(),
	}
}

// SecretRotated constructs a CrossModuleEvent when a secret is rotated.
func SecretRotated(orgID, secretKey string) CrossModuleEvent {
	return CrossModuleEvent{
		OrgID:        orgID,
		Source:       SourceSecvault,
		ResourceType: ResourceTypeSecretRotated,
		ResourceID:   secretKey,
		Title:        "Secret rotiert: " + secretKey,
		Description:  "Ein Secret wurde gemäß Rotationsrichtlinie aktualisiert.",
		OccurredAt:   time.Now().UTC(),
	}
}

// TrainingCompleted constructs a CrossModuleEvent when an awareness training is passed.
func TrainingCompleted(orgID, assignmentID string) CrossModuleEvent {
	return CrossModuleEvent{
		OrgID:        orgID,
		Source:       SourceSecreflex,
		ResourceType: ResourceTypeTrainingCompleted,
		ResourceID:   assignmentID,
		Title:        "Security Awareness Training abgeschlossen",
		Description:  "Ein Mitarbeiter hat ein Security Awareness Training erfolgreich absolviert.",
		OccurredAt:   time.Now().UTC(),
	}
}

// IncidentCreated constructs a CrossModuleEvent for a new security incident.
func IncidentCreated(orgID, incidentID, title string) CrossModuleEvent {
	return CrossModuleEvent{
		OrgID:        orgID,
		Source:       SourceSecvitals,
		ResourceType: ResourceTypeIncidentCreated,
		ResourceID:   incidentID,
		Title:        title,
		Description:  "Sicherheitsvorfall erfasst und dokumentiert.",
		OccurredAt:   time.Now().UTC(),
	}
}

// CertExpiring constructs a CrossModuleEvent when a tracked TLS certificate
// is about to expire (within 30 days) or has already expired.
func CertExpiring(orgID, certID, domain, status string) CrossModuleEvent {
	return CrossModuleEvent{
		OrgID:        orgID,
		Source:       SourceSecpulse,
		ResourceType: ResourceTypeCertExpiring,
		ResourceID:   certID,
		Title:        "TLS-Zertifikat läuft ab: " + domain,
		Description:  "Das TLS-Zertifikat für " + domain + " hat den Status '" + status + "' und erfordert Erneuerung. (A.10.1 / NIS2-H.5)",
		OccurredAt:   time.Now().UTC(),
	}
}

// ChecklistCompletionEvidence is the payload written to the compliance evidence
// store when an HR checklist run reaches the "completed" state.
// Defined here (shared) so neither vaktcomply nor hr imports the other. See ADR-0004.
type ChecklistCompletionEvidence struct {
	OrgID         string
	EmployeeName  string
	EmployeeEmail string
	ChecklistName string
	ChecklistType string // "onboarding" | "offboarding"
	RunID         string
	CompletedAt   time.Time
	StepCount     int
}

// PersonioOffboardingEvidence is the payload written when a Personio-triggered
// offboarding checklist is completed. Defined here so neither vaktcomply nor
// vakthr imports the other. Status is "ok" (<24h) or "warning" (>24h).
type PersonioOffboardingEvidence struct {
	OrgID              string
	PersonioEmployeeID int
	RunID              string
	CompletedAt        time.Time
	DepartureDate      time.Time
	ElapsedHours       float64
	Status             string // "ok" | "warning"
}
