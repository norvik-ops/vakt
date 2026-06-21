// Copyright (c) 2026 NorvikOps. All rights reserved.
// SPDX-License-Identifier: Elastic-2.0

package risk

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5/pgtype"
)

// UpdateRiskResidualFields sets the inherent and/or residual likelihood/impact columns for a risk.
// Only columns supplied (non-nil in the input) are written; others are set to NULL.
func (r *Repository) UpdateRiskResidualFields(ctx context.Context, orgID, id string, in UpdateRiskResidualInput) error {
	il := ckOptIntPtr(in.InherentLikelihood)
	ii := ckOptIntPtr(in.InherentImpact)
	rl := ckOptIntPtr(in.ResidualLikelihood)
	ri := ckOptIntPtr(in.ResidualImpact)

	tag, err := r.db.Exec(ctx, `
		UPDATE ck_risks
		SET inherent_likelihood = $3,
		    inherent_impact     = $4,
		    residual_likelihood = $5,
		    residual_impact     = $6,
		    updated_at          = NOW()
		WHERE id = $1 AND org_id = $2`,
		id, orgID, il, ii, rl, ri,
	)
	if err != nil {
		return fmt.Errorf("update risk residual fields: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return fmt.Errorf("risk not found")
	}
	return nil
}

// AcceptRisk records a formal risk acceptance with justification text and the accepting user.
// It requires the risk to already have treatment_status = 'accepted'; otherwise an error is returned.
func (r *Repository) AcceptRisk(ctx context.Context, orgID, id, userID, justification string) error {
	var treatmentStatus pgtype.Text
	err := r.db.QueryRow(ctx,
		`SELECT treatment_status FROM ck_risks WHERE id = $1 AND org_id = $2`,
		id, orgID,
	).Scan(&treatmentStatus)
	if err != nil {
		return fmt.Errorf("get risk for acceptance check: %w", err)
	}
	if !treatmentStatus.Valid || treatmentStatus.String != "accepted" {
		return fmt.Errorf("risk must have treatment_status=accepted before formal acceptance")
	}

	var acceptedByUUID pgtype.UUID
	if userID != "" {
		if err := acceptedByUUID.Scan(userID); err != nil {
			return fmt.Errorf("invalid user id: %w", err)
		}
	}

	tag, err := r.db.Exec(ctx, `
		UPDATE ck_risks
		SET risk_accepted_by              = $3,
		    risk_accepted_at              = NOW(),
		    risk_acceptance_justification = $4,
		    updated_at                    = NOW()
		WHERE id = $1 AND org_id = $2`,
		id, orgID, acceptedByUUID, justification,
	)
	if err != nil {
		return fmt.Errorf("accept risk: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return fmt.Errorf("risk not found")
	}
	return nil
}
