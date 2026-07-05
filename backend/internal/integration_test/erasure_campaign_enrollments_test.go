//go:build integration

// Copyright (c) 2026 NorvikOps. All rights reserved.
// SPDX-License-Identifier: Elastic-2.0

package integration_test

// TestExecuteErasureDeletesCampaignEnrollments verifies that Art.17 DSGVO
// erasure removes sr_campaign_enrollments rows for the affected employee.
// sr_campaign_enrollments.employee_id is TEXT (no FK cascade on hr_employees),
// so the delete must be explicit.
//
// Run with:
//
//	go test -tags=integration ./internal/integration_test/ -run TestExecuteErasure

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"

	"github.com/matharnica/vakt/internal/modules/vaktprivacy"
)

func TestExecuteErasureDeletesCampaignEnrollments(t *testing.T) {
	if testing.Short() {
		t.Skip("integration: -short mode")
	}
	ctx := context.Background()
	pool, teardown := bootPostgres(t)
	defer teardown()

	orgID := uuid.New().String()
	requesterEmail := "victim@example.com"
	campaignID := uuid.New().String()

	// Seed org, employee, campaign, and enrollment.
	_, err := pool.Exec(ctx, `
		INSERT INTO organizations (id, name, slug) VALUES ($1, 'Test', 'test')`,
		orgID,
	)
	require.NoError(t, err)

	var empID string
	err = pool.QueryRow(ctx, `
		INSERT INTO hr_employees (org_id, email, first_name, last_name)
		VALUES ($1, $2, 'Victim', 'User')
		RETURNING id::text`,
		orgID, requesterEmail,
	).Scan(&empID)
	require.NoError(t, err)

	_, err = pool.Exec(ctx, `
		INSERT INTO sr_campaigns (id, org_id, name, status, from_name, from_email, subject)
		VALUES ($1, $2, 'Test Campaign', 'running', 'IT Security', 'it@example.com', 'Awareness Test')`,
		campaignID, orgID,
	)
	require.NoError(t, err)

	_, err = pool.Exec(ctx, `
		INSERT INTO sr_campaign_enrollments (org_id, campaign_id, employee_id)
		VALUES ($1, $2, $3)`,
		orgID, campaignID, empID,
	)
	require.NoError(t, err)

	// Seed DSR erasure request.
	var dsrID string
	err = pool.QueryRow(ctx, `
		INSERT INTO po_dsr (org_id, requester_name, requester_email, type, status, due_date)
		VALUES ($1, 'Victim User', $2, 'erasure', 'open', NOW() + INTERVAL '30 days')
		RETURNING id::text`,
		orgID, requesterEmail,
	).Scan(&dsrID)
	require.NoError(t, err)

	repo := vaktprivacy.NewRepository(pool)

	_, err = repo.ExecuteErasure(ctx, orgID, dsrID)
	require.NoError(t, err)

	// Verify enrollment is gone.
	var count int
	err = pool.QueryRow(ctx, `
		SELECT COUNT(*) FROM sr_campaign_enrollments WHERE org_id = $1`, orgID,
	).Scan(&count)
	require.NoError(t, err)
	require.Equal(t, 0, count, "sr_campaign_enrollments must be deleted by erasure")
}
