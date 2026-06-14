// Copyright (c) 2026 NorvikOps. All rights reserved.
// SPDX-License-Identifier: Elastic-2.0

package vaktcomply

import (
	"context"
	"fmt"
	"time"
)

// OverdueEffectivenessCheck is a lightweight struct returned by ListOverdueEffectivenessChecks.
type OverdueEffectivenessCheck struct {
	OrgID  string
	CAPAID string
}

// UpdateCAPANCFields updates the NC root-cause and effectiveness fields on a CAPA record.
// Uses raw SQL because sqlc generate is broken (pre-existing issue).
func (r *Repository) UpdateCAPANCFields(ctx context.Context, orgID, id string, fields CAPANCFields) error {
	_, err := r.db.Exec(ctx, `
		UPDATE ck_capas
		SET    nc_classification       = $3,
		       immediate_containment   = $4,
		       root_cause              = $5,
		       similar_ncs_assessed    = $6,
		       similar_ncs_notes       = $7,
		       effectiveness_check_date = $8::date,
		       updated_at              = NOW()
		WHERE  id     = $1::uuid
		  AND  org_id = $2::uuid
	`,
		id, orgID,
		fields.NCClassification,
		fields.ImmediateContainment,
		fields.RootCause,
		fields.SimilarNCsAssessed,
		fields.SimilarNCsNotes,
		fields.EffectivenessCheckDate,
	)
	if err != nil {
		return fmt.Errorf("update capa nc fields: %w", err)
	}
	return nil
}

// CompleteEffectivenessCheck records the result of a CAPA effectiveness check.
// If confirmed is true the CAPA status is also set to 'closed'.
func (r *Repository) CompleteEffectivenessCheck(ctx context.Context, orgID, id, userID string, in EffectivenessCheckInput) error {
	now := time.Now().UTC()
	_, err := r.db.Exec(ctx, `
		UPDATE ck_capas
		SET    effectiveness_confirmed   = $3,
		       effectiveness_checked_at  = $4,
		       effectiveness_checked_by  = $5::uuid,
		       effectiveness_evidence    = $6,
		       updated_at               = NOW()
		WHERE  id     = $1::uuid
		  AND  org_id = $2::uuid
	`,
		id, orgID,
		in.Confirmed,
		now,
		userID,
		in.EvidenceNote,
	)
	if err != nil {
		return fmt.Errorf("complete effectiveness check (fields): %w", err)
	}

	if in.Confirmed {
		_, err = r.db.Exec(ctx, `
			UPDATE ck_capas
			SET    status     = 'closed',
			       closed_at  = NOW(),
			       updated_at = NOW()
			WHERE  id     = $1::uuid
			  AND  org_id = $2::uuid
			  AND  status NOT IN ('closed', 'verified')
		`, id, orgID)
		if err != nil {
			return fmt.Errorf("complete effectiveness check (close): %w", err)
		}
	}

	return nil
}

// ListOverdueEffectivenessChecks returns all major_nc CAPAs whose effectiveness_check_date
// has passed and whose effectiveness has not yet been confirmed.
// Used by the daily alert worker job.
func (r *Repository) ListOverdueEffectivenessChecks(ctx context.Context) ([]OverdueEffectivenessCheck, error) {
	rows, err := r.db.Query(ctx, `
		SELECT org_id, id
		FROM   ck_capas
		WHERE  effectiveness_check_date < CURRENT_DATE
		  AND  effectiveness_confirmed IS NULL
		  AND  nc_classification = 'major_nc'
	`)
	if err != nil {
		return nil, fmt.Errorf("list overdue effectiveness checks: %w", err)
	}
	defer rows.Close()

	var out []OverdueEffectivenessCheck
	for rows.Next() {
		var item OverdueEffectivenessCheck
		if err := rows.Scan(&item.OrgID, &item.CAPAID); err != nil {
			return nil, fmt.Errorf("scan overdue effectiveness check: %w", err)
		}
		out = append(out, item)
	}
	return out, rows.Err()
}
