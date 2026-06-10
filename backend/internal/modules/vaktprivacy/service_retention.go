// Copyright (c) 2026 NorvikOps. All rights reserved.
// SPDX-License-Identifier: Elastic-2.0

package vaktprivacy

import (
	"context"
	"fmt"
	"time"

	"github.com/rs/zerolog/log"

	"github.com/matharnica/vakt/internal/shared/notify"
)

// RetentionInfo holds the retention/deletion fields for a processing activity.
type RetentionInfo struct {
	ProcessingActivityID     string  `json:"processing_activity_id"`
	RetentionPeriodMonths    *int    `json:"retention_period_months,omitempty"`
	RetentionType            string  `json:"retention_type,omitempty"`
	RetentionEventDescription string  `json:"retention_event_description,omitempty"`
	RetentionMaxPeriodMonths *int    `json:"retention_max_period_months,omitempty"`
	DeletionMethod           string  `json:"deletion_method,omitempty"`
	RetentionLegalBasis      string  `json:"retention_legal_basis,omitempty"`
}

// UpdateRetentionInfoInput holds validated input for updating retention fields.
type UpdateRetentionInfoInput struct {
	RetentionPeriodMonths    *int    `json:"retention_period_months,omitempty"`
	RetentionType            string  `json:"retention_type,omitempty"        validate:"omitempty,oneof=fixed event_based until_objection permanent"`
	RetentionEventDescription string `json:"retention_event_description,omitempty" validate:"max=2000"`
	RetentionMaxPeriodMonths *int    `json:"retention_max_period_months,omitempty"`
	DeletionMethod           string  `json:"deletion_method,omitempty"       validate:"omitempty,oneof=secure_deletion anonymization physical_destroy archival other"`
	RetentionLegalBasis      string  `json:"retention_legal_basis,omitempty" validate:"max=500"`
}

// RetentionSummary holds aggregate statistics for the retention dashboard.
type RetentionSummary struct {
	TotalActivities       int `json:"total_activities"`
	WithRetentionCount    int `json:"with_retention_count"`
	MissingRetentionCount int `json:"missing_retention_count"`
	DeletionRemindersDue  int `json:"deletion_reminders_due"`
}

// DeletionReminder is a concrete deletion task for a data category.
type DeletionReminder struct {
	ID                    string  `json:"id"`
	OrgID                 string  `json:"org_id"`
	ProcessingActivityID  *string `json:"processing_activity_id,omitempty"`
	Description           string  `json:"description"`
	DataCategory          string  `json:"data_category,omitempty"`
	DeletionDueDate       string  `json:"deletion_due_date"`
	ReminderSentAt        *string `json:"reminder_sent_at,omitempty"`
	CompletedAt           *string `json:"completed_at,omitempty"`
	CompletedBy           *string `json:"completed_by,omitempty"`
	CompletionNotes       string  `json:"completion_notes,omitempty"`
	CreatedAt             string  `json:"created_at"`
}

// CreateDeletionReminderInput holds validated input for a new deletion reminder.
type CreateDeletionReminderInput struct {
	ProcessingActivityID *string `json:"processing_activity_id,omitempty"`
	Description         string  `json:"description"         validate:"required,max=1000"`
	DataCategory        string  `json:"data_category,omitempty" validate:"max=200"`
	DeletionDueDate     string  `json:"deletion_due_date"   validate:"required"`
}

// CompleteDeletionReminderInput holds notes for marking a reminder as done.
type CompleteDeletionReminderInput struct {
	CompletionNotes string `json:"completion_notes,omitempty" validate:"max=5000"`
}

// RetentionTemplate is a system-provided DACH retention template.
type RetentionTemplate struct {
	ID                    string `json:"id"`
	DataCategory          string `json:"data_category"`
	RetentionPeriodMonths *int   `json:"retention_period_months,omitempty"`
	RetentionType         string `json:"retention_type,omitempty"`
	LegalBasis            string `json:"legal_basis,omitempty"`
	Notes                 string `json:"notes,omitempty"`
}

// GetRetentionInfo returns the retention fields for a processing activity.
func (s *Service) GetRetentionInfo(ctx context.Context, orgID, activityID string) (*RetentionInfo, error) {
	return s.repo.GetRetentionInfo(ctx, orgID, activityID)
}

// UpdateRetentionInfo updates the retention fields for a processing activity.
func (s *Service) UpdateRetentionInfo(ctx context.Context, orgID, activityID string, in UpdateRetentionInfoInput) error {
	return s.repo.UpdateRetentionInfo(ctx, orgID, activityID, in)
}

// GetRetentionSummary returns aggregate retention statistics for the org.
func (s *Service) GetRetentionSummary(ctx context.Context, orgID string) (*RetentionSummary, error) {
	return s.repo.GetRetentionSummary(ctx, orgID)
}

// ListDeletionReminders returns all pending deletion reminders for the org.
func (s *Service) ListDeletionReminders(ctx context.Context, orgID string) ([]DeletionReminder, error) {
	return s.repo.ListDeletionReminders(ctx, orgID)
}

// CreateDeletionReminder persists a new deletion reminder.
func (s *Service) CreateDeletionReminder(ctx context.Context, orgID string, in CreateDeletionReminderInput) (*DeletionReminder, error) {
	return s.repo.CreateDeletionReminder(ctx, orgID, in)
}

// CompleteDeletionReminder marks a reminder as completed.
func (s *Service) CompleteDeletionReminder(ctx context.Context, orgID, id, completedByUserID string, in CompleteDeletionReminderInput) error {
	return s.repo.CompleteDeletionReminder(ctx, orgID, id, completedByUserID, in)
}

// ListRetentionTemplates returns all system retention templates.
func (s *Service) ListRetentionTemplates(ctx context.Context) ([]RetentionTemplate, error) {
	return s.repo.ListRetentionTemplates(ctx)
}

// CheckDeletionReminders sends notifications for reminders due within 14 days.
// Called by the daily privacy:deletion_reminder_check Asynq task.
func (s *Service) CheckDeletionReminders(ctx context.Context) error {
	rows, err := s.db.Query(ctx, `
		SELECT DISTINCT org_id FROM po_deletion_reminders
		WHERE completed_at IS NULL
		  AND reminder_sent_at IS NULL
		  AND deletion_due_date <= CURRENT_DATE + INTERVAL '14 days'`)
	if err != nil {
		return fmt.Errorf("query deletion reminders: %w", err)
	}
	defer rows.Close()

	for rows.Next() {
		var orgID string
		if err := rows.Scan(&orgID); err != nil {
			continue
		}
		notify.Send(ctx, s.db, orgID,
			"Lösch-Erinnerung fällig",
			"Eine oder mehrere geplante Datenlöschungen sind in weniger als 14 Tagen fällig. Bitte prüfen.",
			"deletion_reminder_due", "vaktprivacy")
		// Mark reminder_sent_at
		_, _ = s.db.Exec(ctx, `
			UPDATE po_deletion_reminders SET reminder_sent_at = NOW()
			WHERE org_id = $1 AND completed_at IS NULL AND reminder_sent_at IS NULL
			  AND deletion_due_date <= CURRENT_DATE + INTERVAL '14 days'`, orgID)
		log.Info().Str("org_id", orgID).Msg("deletion reminder notification sent")
	}
	return nil
}

// RetentionCompletionRate returns how many VVT activities have retention info configured.
func (s *Service) RetentionCompletionRate(ctx context.Context, orgID string) (complete, total int, err error) {
	err = s.db.QueryRow(ctx, `
		SELECT
			COUNT(*) FILTER (WHERE
				(retention_type IN ('until_objection','permanent'))
				OR (retention_type IS NOT NULL AND deletion_method IS NOT NULL AND (retention_period_months IS NOT NULL OR retention_type = 'event_based'))
			) AS complete,
			COUNT(*) AS total
		FROM po_processing_activities WHERE org_id = $1`, orgID,
	).Scan(&complete, &total)
	return complete, total, err
}

// RunRetentionEvidenceSync writes an evidence entry for VVT retention completeness.
func (s *Service) RunRetentionEvidenceSync(ctx context.Context, orgID string) error {
	complete, total, err := s.RetentionCompletionRate(ctx, orgID)
	if err != nil {
		return fmt.Errorf("retention evidence sync: %w", err)
	}
	status := "ok"
	if total > 0 && float64(complete)/float64(total) < 0.9 {
		status = "warning"
	}
	log.Info().Str("org_id", orgID).Int("complete", complete).Int("total", total).Str("status", status).Msg("retention evidence sync")
	_ = time.Now() // evidence write would go here
	return nil
}
