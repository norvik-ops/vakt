//go:build integration

// Copyright (c) 2026 NorvikOps. All rights reserved.
// SPDX-License-Identifier: Elastic-2.0

package integration_test

import (
	"context"
	"errors"
	"testing"

	"github.com/google/uuid"
	pgx "github.com/jackc/pgx/v5"
	"github.com/stretchr/testify/require"

	"github.com/matharnica/vakt/internal/modules/vaktaware"
	"github.com/matharnica/vakt/internal/modules/vaktcomply"
	"github.com/matharnica/vakt/internal/modules/vaktcomply/policy"
	"github.com/matharnica/vakt/internal/modules/vaktcomply/risk"
	"github.com/matharnica/vakt/internal/modules/vaktprivacy"
)

// TestMustAffect_SilentFailureHandlersReturnNotFound is the regression guard for the
// R-H18 / S131-A1 SILENT_FAILURE class: eight UPDATE/DELETE-by-id repository methods
// discarded the CommandTag, so writing to a non-existent resource affected zero rows,
// returned no error, and the handler reported 200/204 for a change that never happened
// — worst case a full NIS2 reportability assessment for a phantom incident.
//
// Each method now runs through db.MustAffect, which turns the zero-rows case into
// pgx.ErrNoRows (→ 404 in the handler). This test calls each with random, non-existent
// UUIDs against a real migrated schema and asserts pgx.ErrNoRows.
func TestMustAffect_SilentFailureHandlersReturnNotFound(t *testing.T) {
	if testing.Short() {
		t.Skip("integration: -short mode")
	}
	ctx := context.Background()
	pool, teardown := bootPostgres(t)
	defer teardown()

	org := uuid.NewString()
	id := uuid.NewString()

	awareRepo := vaktaware.NewRepository(pool)
	privacyRepo := vaktprivacy.NewRepository(pool)
	complyRepo := vaktcomply.NewRepository(pool)
	policyRepo := policy.NewRepository(pool)
	riskRepo := risk.NewRepository(pool)

	cases := []struct {
		name string
		call func() error
	}{
		{"vaktaware UpdateEnrollmentRuleActive", func() error {
			return awareRepo.UpdateEnrollmentRuleActive(ctx, org, id, true)
		}},
		{"vaktaware DeleteEnrollmentRule", func() error {
			return awareRepo.DeleteEnrollmentRule(ctx, org, id)
		}},
		{"vaktprivacy AssignDSR", func() error {
			return privacyRepo.AssignDSR(ctx, org, id, "")
		}},
		{"vaktprivacy UpdateRetentionInfo", func() error {
			return privacyRepo.UpdateRetentionInfo(ctx, org, id, vaktprivacy.UpdateRetentionInfoInput{})
		}},
		{"vaktprivacy CompleteDeletionReminder", func() error {
			return privacyRepo.CompleteDeletionReminder(ctx, org, id, "", vaktprivacy.CompleteDeletionReminderInput{})
		}},
		{"vaktcomply UpdateIncidentReportability (assess-reportability phantom)", func() error {
			return complyRepo.UpdateIncidentReportability(ctx, org, id, "not_required", "", false, []byte("{}"))
		}},
		{"vaktcomply/policy UpdateSoAApplicability", func() error {
			return policyRepo.UpdateSoAApplicability(ctx, org, id, true, "", "")
		}},
		{"vaktcomply/risk LinkAssetToPNA", func() error {
			return riskRepo.LinkAssetToPNA(ctx, org, id, nil)
		}},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			err := tc.call()
			require.Error(t, err, "writing to a non-existent resource must not succeed silently")
			require.True(t, errors.Is(err, pgx.ErrNoRows),
				"%s: expected pgx.ErrNoRows (→ 404), got %v", tc.name, err)
		})
	}
}
