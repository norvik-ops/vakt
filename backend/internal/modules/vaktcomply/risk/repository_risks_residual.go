// Copyright (c) 2026 NorvikOps. All rights reserved.
// SPDX-License-Identifier: Elastic-2.0

package risk

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/matharnica/vakt/internal/shared/apperr"
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
		return fmt.Errorf("risk %w", apperr.ErrNotFound)
	}
	return nil
}

// AcceptRisk records a formal risk acceptance with justification text and the accepting user.
// It requires the risk register status to already be 'accepted'; otherwise ErrRiskNotMarkedAccepted
// is returned and the caller answers 409.
//
// R1-14c-12: this precondition used to read treatment_status and demand the value
// 'accepted'. That value exists in neither the input validation (oneof=pending
// in_progress implemented verified) nor the CHECK constraint on the column, so the
// precondition was unreachable by every path including a manual UPDATE — the fully
// wired acceptance dialog answered 409 forever and formal risk acceptance, a record
// an auditor asks for under ISO 27001 6.1.3/8.3 and BSI-Grundschutz, could not be
// produced at all. treatment_status tracks how far the treatment plan got
// (pending → in_progress → implemented → verified); it is a progress axis with no
// room for a decision. The decision lives in the register status
// (open | mitigated | accepted | closed), which does allow 'accepted' and which the
// frontend already uses to gate this very dialog.
func (r *Repository) AcceptRisk(ctx context.Context, orgID, id, userID, justification string) error {
	var status pgtype.Text
	err := r.db.QueryRow(ctx,
		`SELECT status FROM ck_risks WHERE id = $1 AND org_id = $2`,
		id, orgID,
	).Scan(&status)
	if errors.Is(err, pgx.ErrNoRows) {
		// Without this the caller cannot tell "no such risk" from a real failure and
		// answers 500 for a mistyped id.
		return fmt.Errorf("risk %w", ErrNotFound)
	}
	if err != nil {
		return fmt.Errorf("get risk for acceptance check: %w", err)
	}
	if !status.Valid || status.String != "accepted" {
		return fmt.Errorf("%w (current status: %q)", ErrRiskNotMarkedAccepted, status.String)
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
		return fmt.Errorf("risk %w", apperr.ErrNotFound)
	}
	return nil
}
