// Copyright (c) 2026 NorvikOps. All rights reserved.
// SPDX-License-Identifier: Elastic-2.0

package vaktcomply

import (
	"context"
	"time"

	"github.com/rs/zerolog/log"

	"github.com/matharnica/vakt/internal/db"
)

// ChangeLogEntry represents one field change on a control.
type ChangeLogEntry struct {
	ID        string    `json:"id"`
	ControlID string    `json:"control_id"`
	UserEmail string    `json:"user_email"`
	Field     string    `json:"field"`
	OldValue  string    `json:"old_value"`
	NewValue  string    `json:"new_value"`
	ChangedAt time.Time `json:"changed_at"`
}

// AppendControlChange inserts a change log entry into ck_control_changelog.
// Errors are logged but not returned — a changelog write failure must never
// abort the primary update operation.
func (r *Repository) AppendControlChange(ctx context.Context, orgID, controlID, userID, userEmail, field, oldVal, newVal string) {
	err := r.q.AppendCKControlChange(ctx, db.AppendCKControlChangeParams{
		ControlID: controlID,
		OrgID:     orgID,
		UserID:    ckOptUUIDFromStr(userID),
		UserEmail: ckOptText(userEmail),
		Field:     field,
		OldValue:  ckOptText(oldVal),
		NewValue:  ckOptText(newVal),
	})
	if err != nil {
		log.Error().
			Err(err).
			Str("control_id", controlID).
			Str("field", field).
			Msg("changelog: failed to append control change")
	}
}

// ListControlChanges returns the last 50 field-level changes for a control,
// ordered newest first.
func (r *Repository) ListControlChanges(ctx context.Context, orgID, controlID string) ([]ChangeLogEntry, error) {
	rows, err := r.q.ListCKControlChanges(ctx, db.ListCKControlChangesParams{OrgID: orgID, ControlID: controlID})
	if err != nil {
		return nil, err
	}
	out := make([]ChangeLogEntry, 0, len(rows))
	for _, row := range rows {
		out = append(out, ChangeLogEntry{
			ID:        row.ID,
			ControlID: row.ControlID,
			UserEmail: row.UserEmail.String,
			Field:     row.Field,
			OldValue:  row.OldValue.String,
			NewValue:  row.NewValue.String,
			ChangedAt: ckTsToTime(row.ChangedAt),
		})
	}
	return out, nil
}
