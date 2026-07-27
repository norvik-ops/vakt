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

	"github.com/matharnica/vakt/internal/modules/vaktaware"
	"github.com/matharnica/vakt/internal/modules/vakthr"
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

	// Wire the module-owned erasers (module isolation, ADR-0079). Order is
	// deliberately IRRELEVANT — see TestExecuteErasure_OrderIndependent below.
	repo := vaktprivacy.NewRepository(pool).
		WithSubjectErasers(vaktaware.NewSubjectEraser(), vakthr.NewSubjectEraser()).
		WithSubjectResolver(vakthr.NewSubjectResolver())

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

// TestExecuteErasure_OrderIndependent is the regression guard for the review
// finding on S8: before the SubjectRef refactor, the erasure only worked when
// vaktaware ran BEFORE vakthr, and that constraint was held up by a comment.
// Reversed, vakthr deleted hr_employees first, vaktaware's sub-SELECT matched
// nothing, sr_campaign_enrollments KEPT the PII — and the DSR was still stamped
// "completed" with a plausible `deleted: 0`. No error, no log.
//
// This test wires the erasers in the WORST order on purpose. It must still
// erase everything. Run it against the pre-fix code and it fails.
func TestExecuteErasure_OrderIndependent(t *testing.T) {
	if testing.Short() {
		t.Skip("integration: -short mode")
	}
	ctx := context.Background()
	pool, teardown := bootPostgres(t)
	defer teardown()

	orgID := uuid.New().String()
	requesterEmail := "reversed@example.com"
	campaignID := uuid.New().String()

	_, err := pool.Exec(ctx, `
		INSERT INTO organizations (id, name, slug) VALUES ($1, 'Rev', 'rev')`, orgID)
	require.NoError(t, err)

	var empID string
	require.NoError(t, pool.QueryRow(ctx, `
		INSERT INTO hr_employees (org_id, email, first_name, last_name)
		VALUES ($1, $2, 'Rev', 'User')
		RETURNING id::text`, orgID, requesterEmail).Scan(&empID))

	_, err = pool.Exec(ctx, `
		INSERT INTO sr_campaigns (id, org_id, name, status, from_name, from_email, subject)
		VALUES ($1, $2, 'Rev Campaign', 'running', 'IT', 'it@example.com', 'Test')`,
		campaignID, orgID)
	require.NoError(t, err)

	_, err = pool.Exec(ctx, `
		INSERT INTO sr_campaign_enrollments (org_id, campaign_id, employee_id)
		VALUES ($1, $2, $3)`, orgID, campaignID, empID)
	require.NoError(t, err)

	var dsrID string
	require.NoError(t, pool.QueryRow(ctx, `
		INSERT INTO po_dsr (org_id, requester_name, requester_email, type, status, due_date)
		VALUES ($1, 'Rev User', $2, 'erasure', 'open', NOW() + INTERVAL '30 days')
		RETURNING id::text`, orgID, requesterEmail).Scan(&dsrID))

	// REVERSED on purpose: vakthr (deletes hr_employees) wired FIRST.
	repo := vaktprivacy.NewRepository(pool).
		WithSubjectErasers(vakthr.NewSubjectEraser(), vaktaware.NewSubjectEraser()).
		WithSubjectResolver(vakthr.NewSubjectResolver())

	_, err = repo.ExecuteErasure(ctx, orgID, dsrID)
	require.NoError(t, err)

	var enrollments, employees int
	require.NoError(t, pool.QueryRow(ctx,
		`SELECT COUNT(*) FROM sr_campaign_enrollments WHERE org_id = $1`, orgID).Scan(&enrollments))
	require.NoError(t, pool.QueryRow(ctx,
		`SELECT COUNT(*) FROM hr_employees WHERE org_id = $1`, orgID).Scan(&employees))

	require.Equal(t, 0, enrollments,
		"PII must be erased regardless of eraser wiring order — a reversed wiring "+
			"must not leave sr_campaign_enrollments behind (ADR-0079)")
	require.Equal(t, 0, employees, "hr_employees must be erased too")
}

// TestExecuteErasure_RefusesPartialWiring pins the second half of the finding:
// wiring only SOME of the modules that hold subject PII must fail loudly. The
// old guard only caught "none wired", so a single missing eraser produced a
// silent partial erasure stamped "completed".
func TestExecuteErasure_RefusesPartialWiring(t *testing.T) {
	if testing.Short() {
		t.Skip("integration: -short mode")
	}
	ctx := context.Background()
	pool, teardown := bootPostgres(t)
	defer teardown()

	orgID := uuid.New().String()
	_, err := pool.Exec(ctx, `
		INSERT INTO organizations (id, name, slug) VALUES ($1, 'Part', 'part')`, orgID)
	require.NoError(t, err)

	var dsrID string
	require.NoError(t, pool.QueryRow(ctx, `
		INSERT INTO po_dsr (org_id, requester_name, requester_email, type, status, due_date)
		VALUES ($1, 'Part User', 'part@example.com', 'erasure', 'open', NOW() + INTERVAL '30 days')
		RETURNING id::text`, orgID).Scan(&dsrID))

	// Only vaktaware wired — vakthr's hr_employees PII would be left behind.
	repo := vaktprivacy.NewRepository(pool).
		WithSubjectErasers(vaktaware.NewSubjectEraser()).
		WithSubjectResolver(vakthr.NewSubjectResolver())

	_, err = repo.ExecuteErasure(ctx, orgID, dsrID)
	require.Error(t, err, "a partially wired erasure must fail loudly, not report completed")
	require.Contains(t, err.Error(), "vakthr",
		"the error must name the missing module so the operator knows what is unwired")

	// And the DSR must NOT have been stamped completed.
	var status string
	// orgid-lint: global — Testabfrage auf einen in diesem Test erzeugten DSR,
	// adressiert per Primaerschluessel; keine mandantenuebergreifende Leseflaeche.
	require.NoError(t, pool.QueryRow(ctx,
		`SELECT status FROM po_dsr WHERE id = $1::uuid`, dsrID).Scan(&status))
	require.NotEqual(t, "completed", status,
		"a refused erasure must leave the DSR open — never claim completion")
}
