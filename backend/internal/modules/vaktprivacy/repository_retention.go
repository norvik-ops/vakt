// Copyright (c) 2026 NorvikOps. All rights reserved.
// SPDX-License-Identifier: Elastic-2.0

package vaktprivacy

import (
	"context"
	"time"

	"github.com/jackc/pgx/v5/pgtype"
)

// GetRetentionInfo returns the retention columns for a processing activity.
func (r *Repository) GetRetentionInfo(ctx context.Context, orgID, activityID string) (*RetentionInfo, error) {
	var info RetentionInfo
	info.ProcessingActivityID = activityID
	var retType, eventDesc, deletionMethod, legalBasis pgtype.Text
	var periodMonths, maxPeriodMonths pgtype.Int4
	err := r.db.QueryRow(ctx, `
		SELECT retention_period_months, COALESCE(retention_type,''), COALESCE(retention_event_description,''),
		       retention_max_period_months, COALESCE(deletion_method,''), COALESCE(retention_legal_basis,'')
		FROM po_processing_activities WHERE org_id = $1 AND id = $2`, orgID, activityID,
	).Scan(&periodMonths, &retType, &eventDesc, &maxPeriodMonths, &deletionMethod, &legalBasis)
	if err != nil {
		return nil, err
	}
	if periodMonths.Valid {
		v := int(periodMonths.Int32)
		info.RetentionPeriodMonths = &v
	}
	if maxPeriodMonths.Valid {
		v := int(maxPeriodMonths.Int32)
		info.RetentionMaxPeriodMonths = &v
	}
	info.RetentionType = retType.String
	info.RetentionEventDescription = eventDesc.String
	info.DeletionMethod = deletionMethod.String
	info.RetentionLegalBasis = legalBasis.String
	return &info, nil
}

// UpdateRetentionInfo updates the retention columns for a processing activity.
func (r *Repository) UpdateRetentionInfo(ctx context.Context, orgID, activityID string, in UpdateRetentionInfoInput) error {
	_, err := r.db.Exec(ctx, `
		UPDATE po_processing_activities SET
			retention_period_months     = $1,
			retention_type              = NULLIF($2,''),
			retention_event_description = NULLIF($3,''),
			retention_max_period_months = $4,
			deletion_method             = NULLIF($5,''),
			retention_legal_basis       = NULLIF($6,''),
			updated_at                  = NOW()
		WHERE org_id = $7 AND id = $8`,
		in.RetentionPeriodMonths, in.RetentionType, in.RetentionEventDescription,
		in.RetentionMaxPeriodMonths, in.DeletionMethod, in.RetentionLegalBasis,
		orgID, activityID,
	)
	return err
}

// GetRetentionSummary returns aggregate stats.
func (r *Repository) GetRetentionSummary(ctx context.Context, orgID string) (*RetentionSummary, error) {
	var s RetentionSummary
	err := r.db.QueryRow(ctx, `
		SELECT
			COUNT(*) AS total,
			COUNT(*) FILTER (WHERE
				(retention_type IN ('until_objection','permanent'))
				OR (retention_type IS NOT NULL AND deletion_method IS NOT NULL AND
				    (retention_period_months IS NOT NULL OR retention_type = 'event_based'))
			) AS complete
		FROM po_processing_activities WHERE org_id = $1`, orgID,
	).Scan(&s.TotalActivities, &s.WithRetentionCount)
	if err != nil {
		return nil, err
	}
	s.MissingRetentionCount = s.TotalActivities - s.WithRetentionCount

	r.db.QueryRow(ctx, `
		SELECT COUNT(*) FROM po_deletion_reminders
		WHERE org_id = $1 AND completed_at IS NULL
		  AND deletion_due_date <= CURRENT_DATE + INTERVAL '14 days'`, orgID,
	).Scan(&s.DeletionRemindersDue) //nolint:errcheck

	return &s, nil
}

// ListDeletionReminders returns all open reminders for the org.
func (r *Repository) ListDeletionReminders(ctx context.Context, orgID string) ([]DeletionReminder, error) {
	rows, err := r.db.Query(ctx, `
		SELECT id, org_id, processing_activity_id, description, COALESCE(data_category,''),
		       deletion_due_date::text, reminder_sent_at::text, completed_at::text,
		       completed_by, COALESCE(completion_notes,''), created_at
		FROM po_deletion_reminders WHERE org_id = $1 ORDER BY deletion_due_date`, orgID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var reminders []DeletionReminder
	for rows.Next() {
		var d DeletionReminder
		var actID, reminderSent, completedAt, completedBy pgtype.Text
		var createdAt time.Time
		if err := rows.Scan(&d.ID, &d.OrgID, &actID, &d.Description, &d.DataCategory,
			&d.DeletionDueDate, &reminderSent, &completedAt, &completedBy, &d.CompletionNotes, &createdAt); err != nil {
			return nil, err
		}
		d.CreatedAt = createdAt.Format(time.RFC3339)
		if actID.Valid {
			d.ProcessingActivityID = &actID.String
		}
		if reminderSent.Valid {
			d.ReminderSentAt = &reminderSent.String
		}
		if completedAt.Valid {
			d.CompletedAt = &completedAt.String
		}
		if completedBy.Valid {
			d.CompletedBy = &completedBy.String
		}
		reminders = append(reminders, d)
	}
	if reminders == nil {
		reminders = []DeletionReminder{}
	}
	return reminders, rows.Err()
}

// CreateDeletionReminder inserts a new deletion reminder.
func (r *Repository) CreateDeletionReminder(ctx context.Context, orgID string, in CreateDeletionReminderInput) (*DeletionReminder, error) {
	var d DeletionReminder
	var createdAt time.Time
	err := r.db.QueryRow(ctx, `
		INSERT INTO po_deletion_reminders (org_id, processing_activity_id, description, data_category, deletion_due_date)
		VALUES ($1, $2, $3, NULLIF($4,''), $5::date)
		RETURNING id, org_id, processing_activity_id, description, COALESCE(data_category,''),
		          deletion_due_date::text, NULL::text, NULL::text, NULL::text, '', created_at`,
		orgID, in.ProcessingActivityID, in.Description, in.DataCategory, in.DeletionDueDate,
	).Scan(&d.ID, &d.OrgID, &d.ProcessingActivityID, &d.Description, &d.DataCategory,
		&d.DeletionDueDate, &d.ReminderSentAt, &d.CompletedAt, &d.CompletedBy, &d.CompletionNotes, &createdAt)
	if err != nil {
		return nil, err
	}
	d.CreatedAt = createdAt.Format(time.RFC3339)
	return &d, nil
}

// CompleteDeletionReminder marks a reminder as done.
func (r *Repository) CompleteDeletionReminder(ctx context.Context, orgID, id, completedByUserID string, in CompleteDeletionReminderInput) error {
	_, err := r.db.Exec(ctx, `
		UPDATE po_deletion_reminders SET
			completed_at     = NOW(),
			completed_by     = NULLIF($1,'')::uuid,
			completion_notes = NULLIF($2,'')
		WHERE org_id = $3 AND id = $4`,
		completedByUserID, in.CompletionNotes, orgID, id,
	)
	return err
}

// ListRetentionTemplates returns all system retention templates.
func (r *Repository) ListRetentionTemplates(ctx context.Context) ([]RetentionTemplate, error) {
	// orgid-lint: global — po_retention_templates with is_system_template=true is a shared catalogue, not per-org
	rows, err := r.db.Query(ctx, `
		SELECT id, data_category, retention_period_months, COALESCE(retention_type,''),
		       COALESCE(legal_basis,''), COALESCE(notes,'')
		FROM po_retention_templates WHERE is_system_template = true ORDER BY data_category`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var templates []RetentionTemplate
	for rows.Next() {
		var t RetentionTemplate
		var periodMonths pgtype.Int4
		if err := rows.Scan(&t.ID, &t.DataCategory, &periodMonths, &t.RetentionType, &t.LegalBasis, &t.Notes); err != nil {
			return nil, err
		}
		if periodMonths.Valid {
			v := int(periodMonths.Int32)
			t.RetentionPeriodMonths = &v
		}
		templates = append(templates, t)
	}
	if templates == nil {
		templates = []RetentionTemplate{}
	}
	return templates, rows.Err()
}
