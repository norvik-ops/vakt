// Copyright (c) 2026 NorvikOps. All rights reserved.
// SPDX-License-Identifier: Elastic-2.0

package vaktcomply

import (
	"context"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/rs/zerolog/log"

	"github.com/matharnica/vakt/internal/modules/vakthr"
)

// HRAccessReviewTrigger implements vakthr.AccessReviewTrigger.
// It creates an access-review campaign in vaktcomply whenever an offboarding
// checklist run completes, ensuring that access revocation is verified by a
// reviewer within 7 days.
type HRAccessReviewTrigger struct {
	pool *pgxpool.Pool
}

// NewHRAccessReviewTrigger returns an HRAccessReviewTrigger backed by the given pool.
func NewHRAccessReviewTrigger(pool *pgxpool.Pool) vakthr.AccessReviewTrigger {
	return &HRAccessReviewTrigger{pool: pool}
}

// TriggerOffboardingReview creates an access-review campaign for a completed offboarding run.
func (t *HRAccessReviewTrigger) TriggerOffboardingReview(ctx context.Context, in vakthr.OffboardingReviewInput) error {
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
