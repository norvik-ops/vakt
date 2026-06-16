//go:build integration

// Copyright (c) 2026 NorvikOps. All rights reserved.
// SPDX-License-Identifier: Elastic-2.0

package integration_test

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/testcontainers/testcontainers-go/modules/postgres"

	"github.com/matharnica/vakt/internal/modules/vaktcomply"
	shareddb "github.com/matharnica/vakt/internal/shared/db"
)

// TestVVTControlLinks_CRUDAndOrgScoping is the S88-9 acceptance test: link a VVT
// entry to a control, read it from both directions, ensure idempotency, org
// scoping, and unlink.
func TestVVTControlLinks_CRUDAndOrgScoping(t *testing.T) {
	if testing.Short() {
		t.Skip("integration: -short mode")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()

	pgC, err := postgres.Run(ctx,
		"postgres:16-alpine",
		postgres.WithDatabase("vakt_test"),
		postgres.WithUsername("vakt"),
		postgres.WithPassword("vakt"),
		postgres.WithSQLDriver("pgx"),
		postgres.BasicWaitStrategies(),
	)
	if err != nil {
		if strings.Contains(err.Error(), "permission denied") ||
			strings.Contains(err.Error(), "Cannot connect to the Docker daemon") {
			t.Skipf("integration: Docker unavailable (%v)", err)
		}
		t.Fatalf("postgres container: %v", err)
	}
	defer func() { _ = pgC.Terminate(ctx) }()

	dsn, err := pgC.ConnectionString(ctx, "sslmode=disable")
	require.NoError(t, err)
	require.NoError(t, shareddb.RunMigrations(dsn, migrationsDir(t)))

	pool, err := pgxpool.New(ctx, dsn)
	require.NoError(t, err)
	defer pool.Close()

	mkOrgControl := func(slug string) (string, string) {
		var orgID, fwID, ctrlID string
		require.NoError(t, pool.QueryRow(ctx, `INSERT INTO organizations (name, slug) VALUES ($1,$1) RETURNING id::text`, slug).Scan(&orgID))
		require.NoError(t, pool.QueryRow(ctx, `INSERT INTO ck_frameworks (org_id, name) VALUES ($1,'ISO 27001') RETURNING id::text`, orgID).Scan(&fwID))
		require.NoError(t, pool.QueryRow(ctx, `
			INSERT INTO ck_controls (framework_id, org_id, control_id, title, description, domain)
			VALUES ($1::uuid,$2::uuid,'A.5.34','Privacy and PII protection','','Organizational') RETURNING id::text`,
			fwID, orgID).Scan(&ctrlID))
		return orgID, ctrlID
	}

	orgA, ctrlA := mkOrgControl("orga")
	orgB, ctrlB := mkOrgControl("orgb")

	svc := vaktcomply.NewService(pool)

	// Link VVT → control in org A.
	link, err := svc.LinkVVTToControl(ctx, orgA, vaktcomply.LinkVVTToControlInput{
		VVTID: "vvt-1", VVTName: "Bewerbermanagement", ControlID: ctrlA,
	})
	require.NoError(t, err)
	assert.Equal(t, "vvt-1", link.VVTID)

	// Idempotent re-link: same tuple → still one row.
	_, err = svc.LinkVVTToControl(ctx, orgA, vaktcomply.LinkVVTToControlInput{
		VVTID: "vvt-1", VVTName: "Bewerbermanagement", ControlID: ctrlA,
	})
	require.NoError(t, err)

	// Reverse views.
	fromControl, err := svc.ListLinksForControl(ctx, orgA, ctrlA)
	require.NoError(t, err)
	assert.Len(t, fromControl, 1, "control shows exactly one linked VVT")

	fromVVT, err := svc.ListLinksForVVT(ctx, orgA, "vvt-1")
	require.NoError(t, err)
	assert.Len(t, fromVVT, 1, "VVT shows exactly one linked control")

	// Org scoping: org B sees nothing, and cannot link to org A's control.
	bView, err := svc.ListLinksForControl(ctx, orgB, ctrlA)
	require.NoError(t, err)
	assert.Len(t, bView, 0, "cross-org control link list must be empty")

	_, err = svc.LinkVVTToControl(ctx, orgB, vaktcomply.LinkVVTToControlInput{
		VVTID: "vvt-x", VVTName: "x", ControlID: ctrlA, // ctrlA belongs to org A
	})
	require.Error(t, err, "must not link to a control from another org")

	// org B can link its own control.
	_, err = svc.LinkVVTToControl(ctx, orgB, vaktcomply.LinkVVTToControlInput{
		VVTID: "vvt-2", VVTName: "y", ControlID: ctrlB,
	})
	require.NoError(t, err)

	// Unlink in org A.
	require.NoError(t, svc.UnlinkVVTFromControl(ctx, orgA, link.ID))
	after, err := svc.ListLinksForControl(ctx, orgA, ctrlA)
	require.NoError(t, err)
	assert.Len(t, after, 0, "link removed")
}
