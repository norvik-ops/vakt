package vaktcomply

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	sharedevents "github.com/matharnica/vakt/internal/shared/events"
	"github.com/matharnica/vakt/internal/shared/platform/events"
	"github.com/rs/zerolog/log"
)

type HRAccessReviewTrigger struct {
	pool *pgxpool.Pool
}

// NewHRAccessReviewTrigger returns an HRAccessReviewTrigger backed by the given pool.
func NewHRAccessReviewTrigger(pool *pgxpool.Pool) sharedevents.AccessReviewTrigger {
	return &HRAccessReviewTrigger{pool: pool}
}

// TriggerOffboardingReview creates an access-review campaign for a completed offboarding run.
func (t *HRAccessReviewTrigger) TriggerOffboardingReview(ctx context.Context, in sharedevents.OffboardingReviewInput) error {
	repo := NewRepository(t.pool)
	due := in.CompletedAt.Add(7 * 24 * time.Hour)
	dueStr := due.UTC().Format(time.RFC3339)

	title := fmt.Sprintf("Offboarding-Zugriffsverifizierung %s", in.CompletedAt.UTC().Format("2006-01-02"))
	if in.Department != "" {
		title = fmt.Sprintf("Offboarding-Zugriffsverifizierung %s — %s", in.CompletedAt.UTC().Format("2006-01-02"), in.Department)
	}

	description := fmt.Sprintf(
		"Automatisch erstellt nach Abschluss des Offboarding-Checkliste-Laufs %s. "+
			"Bitte prüfen und bestätigen Sie, dass alle Zugriffsrechte des ausgeschiedenen Mitarbeiters entzogen wurden. "+
			"Weisen Sie diese Kampagne dem zuständigen IT-Administrator zu.",
		in.RunID,
	)

	_, err := repo.CreateAccessReviewCampaign(ctx, in.OrgID, CreateAccessReviewCampaignInput{
		Title:         title,
		Description:   description,
		ReviewerEmail: "hr-system@offboarding.vakt",
		Scope:         "Offboarding — Zugriffsrechte-Entzug",
		DueDate:       &dueStr,
	})
	if err != nil {
		return fmt.Errorf("hr_access_review: create campaign: %w", err)
	}

	log.Info().
		Str("run_id", in.RunID).
		Str("org_id", in.OrgID).
		Str("department", in.Department).
		Msg("vakthr→vaktcomply: access review campaign created from offboarding")
	return nil
}

type HREvidenceWriter struct {
	pool *pgxpool.Pool
}

// NewHREvidenceWriter creates an HREvidenceWriter backed by the given DB pool.
func NewHREvidenceWriter(pool *pgxpool.Pool) *HREvidenceWriter {
	return &HREvidenceWriter{pool: pool}
}

// WriteChecklistCompletion inserts a row into ck_evidence describing the completed run.
func (w *HREvidenceWriter) WriteChecklistCompletion(ctx context.Context, in events.ChecklistCompletionEvidence) error {
	title := fmt.Sprintf("%s: %s", titleForType(in.ChecklistType), in.EmployeeName)
	description := fmt.Sprintf(
		"Checkliste %q für Mitarbeiter %s (%s) abgeschlossen am %s — %d Schritte erledigt.",
		in.ChecklistName, in.EmployeeName, in.EmployeeEmail,
		in.CompletedAt.Format("02.01.2006 15:04"), in.StepCount,
	)
	data, err := json.Marshal(map[string]any{
		"employee_name":  in.EmployeeName,
		"employee_email": in.EmployeeEmail,
		"checklist_name": in.ChecklistName,
		"checklist_type": in.ChecklistType,
		"run_id":         in.RunID,
		"step_count":     in.StepCount,
		"completed_at":   in.CompletedAt.Format("2006-01-02T15:04:05Z07:00"),
	})
	if err != nil {
		return fmt.Errorf("marshal hr evidence data: %w", err)
	}

	_, err = w.pool.Exec(ctx, `
		INSERT INTO ck_evidence
			(control_id, org_id, title, description, source, collector_data, status,
			 auto_source_type, auto_collected_at)
		VALUES
			(NULL, $1::uuid, $2, $3, 'hr_checklist_completed', $4, 'approved',
			 'hr', NOW())
	`, in.OrgID, title, description, data)
	if err != nil {
		return fmt.Errorf("insert hr evidence: %w", err)
	}
	return nil
}

// WritePersonioOffboardingEvidence inserts a ck_evidence row with approved/pending status
// based on whether the offboarding was completed within 24h of the departure date.
func (w *HREvidenceWriter) WritePersonioOffboardingEvidence(ctx context.Context, in events.PersonioOffboardingEvidence) error {
	dbStatus := "approved"
	title := fmt.Sprintf("Personio Offboarding: Mitarbeiter %d — IT-Zugang gesperrt %.1fh nach Austritt",
		in.PersonioEmployeeID, in.ElapsedHours)
	description := fmt.Sprintf(
		"Offboarding (Run %s) abgeschlossen am %s. Austritt: %s. Verstrichene Zeit: %.1fh. Ziel: <24h.",
		in.RunID,
		in.CompletedAt.Format("02.01.2006 15:04"),
		in.DepartureDate.Format("02.01.2006"),
		in.ElapsedHours,
	)
	if in.ElapsedHours > 24 {
		dbStatus = "pending"
		title += " (Ziel 24h überschritten)"
	}

	data, _ := json.Marshal(map[string]any{
		"personio_employee_id": in.PersonioEmployeeID,
		"run_id":               in.RunID,
		"completed_at":         in.CompletedAt.Format("2006-01-02T15:04:05Z07:00"),
		"departure_date":       in.DepartureDate.Format("2006-01-02"),
		"elapsed_hours":        in.ElapsedHours,
		"within_24h":           in.ElapsedHours <= 24,
	})

	_, err := w.pool.Exec(ctx, `
		INSERT INTO ck_evidence
			(control_id, org_id, title, description, source, collector_data, status,
			 auto_source_type, auto_collected_at)
		VALUES
			(NULL, $1::uuid, $2, $3, 'personio_offboarding', $4, $5,
			 'personio', NOW())`,
		in.OrgID, title, description, data, dbStatus,
	)
	if err != nil {
		return fmt.Errorf("insert personio offboarding evidence: %w", err)
	}
	return nil
}

// WriteEvidence inserts a generic evidence row for events not covered by
// the typed methods (e.g. contractor lifecycle events).
func (w *HREvidenceWriter) WriteEvidence(ctx context.Context, orgID, evidenceType, description, entityID string) error {
	data, _ := json.Marshal(map[string]any{
		"evidence_type": evidenceType,
		"entity_id":     entityID,
	})
	_, err := w.pool.Exec(ctx, `
		INSERT INTO ck_evidence
			(control_id, org_id, title, description, source, collector_data, status,
			 auto_source_type, auto_collected_at)
		VALUES
			(NULL, $1::uuid, $2, $3, $4, $5, 'approved', 'hr', NOW())
	`, orgID, evidenceType, description, "hr_event", data)
	if err != nil {
		return fmt.Errorf("insert hr generic evidence: %w", err)
	}
	return nil
}

func titleForType(t string) string {
	switch t {
	case "onboarding":
		return "Onboarding abgeschlossen"
	case "offboarding":
		return "Offboarding abgeschlossen"
	default:
		return "Checkliste abgeschlossen"
	}
}
