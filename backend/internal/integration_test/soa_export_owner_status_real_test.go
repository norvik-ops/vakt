//go:build integration

// Copyright (c) 2026 NorvikOps. All rights reserved.
// SPDX-License-Identifier: Elastic-2.0

package integration_test

import (
	"bytes"
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/labstack/echo/v4"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/testcontainers/testcontainers-go/modules/postgres"
	"github.com/xuri/excelize/v2"

	"github.com/matharnica/vakt/internal/license"
	"github.com/matharnica/vakt/internal/modules/vaktcomply"
	"github.com/matharnica/vakt/internal/modules/vaktcomply/policy"
	shareddb "github.com/matharnica/vakt/internal/shared/db"
)

// TestSoAXLSXExport_OwnerFromControlAndGermanStatus pins R1-20-06 and R1-20-07.
//
// R1-20-06: the dedicated SoA XLSX/DOCX export put ck_soa_entries.approved_by —
// the UUID of whoever approved the version — in the "Verantwortlicher" column.
// That is both a raw UUID and the wrong person. The responsible party lives on
// the linked ck_control (owner, a human-readable string), which is what the
// auditor cross-checks.
//
// R1-20-07: the same export leaked the raw English implementation_status enum
// ("implemented") into a German product; the PDF right next to it says
// "Implementiert".
//
// The test drives the real write path (seed control with owner → InitDedicatedSoA
// → link the entry to that control → approve, which sets approved_by to a real
// user), then pulls the XLSX through its handler and reads the cells back.
//
// Non-vacuity: revert owner to *e.ApprovedBy and the owner assertion flips to the
// approver UUID; revert the status to e.ImplementationStatus and it reads
// "implemented" instead of "Implementiert".
func TestSoAXLSXExport_OwnerFromControlAndGermanStatus(t *testing.T) {
	if testing.Short() {
		t.Skip("integration: -short mode")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Minute)
	defer cancel()

	pgC, err := postgres.Run(ctx,
		imagePostgres,
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

	var orgID string
	require.NoError(t, pool.QueryRow(ctx, `
		INSERT INTO organizations (name, slug) VALUES ('OwnerOrg', 'ownerorg')
		RETURNING id::text`).Scan(&orgID))
	fwID := seedFramework(ctx, t, pool, orgID, "ISO 27001")

	// A control with a distinctive, human-readable owner — the correct source.
	const controlOwner = "Frau Dr. Verantwortlich (IT-Leitung)"
	var ctrlID string
	require.NoError(t, pool.QueryRow(ctx, `
		INSERT INTO ck_controls (org_id, framework_id, control_id, title, domain, owner)
		VALUES ($1::uuid, $2::uuid, 'A.5.7', 'Threat intelligence', '5', $3)
		RETURNING id::text`, orgID, fwID, controlOwner).Scan(&ctrlID))

	// A real approver user — approved_by has a FK to users(id). Its UUID is what
	// the buggy export used to show in the "Verantwortlicher" column.
	var approverID string
	require.NoError(t, pool.QueryRow(ctx, `
		INSERT INTO users (email, display_name) VALUES ('approver@ownerorg.test', 'Herr Genehmiger')
		RETURNING id::text`).Scan(&approverID))

	svc := vaktcomply.NewService(pool)
	h := vaktcomply.NewHandler(svc)
	e := echo.New()

	require.NoError(t, svc.InitDedicatedSoA(ctx, orgID))
	// Link the A.5.7 SoA entry to the ck_control and mark it implemented.
	require.NoError(t, svc.UpdateDedicatedSoAEntry(ctx, orgID, "A.5.7", policy.UpdateSoAEntryInput{
		Applicable:           true,
		Justification:        "Bedrohungslage wird ausgewertet",
		ImplementationStatus: "implemented",
		CKControlID:          &ctrlID,
	}))
	// Approve → sets approved_by = approverID on the entries (the wrong person).
	require.NoError(t, svc.ApproveDedicatedSoA(ctx, orgID, approverID))

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)
	c.Set("org_id", orgID)
	c.Set("license", &license.License{Tier: "pro", Features: []string{license.FeatureAuditPDF}})
	require.NoError(t, h.ExportDedicatedSoAXLSX(c))
	require.Equal(t, http.StatusOK, rec.Code)

	f, err := excelize.OpenReader(bytes.NewReader(rec.Body.Bytes()))
	require.NoError(t, err)
	defer func() { _ = f.Close() }()

	rows, err := f.GetRows("SoA")
	require.NoError(t, err)
	require.NotEmpty(t, rows, "SoA sheet must have rows")

	header := rows[0]
	ownerCol := colIndex(t, header, "Verantwortlicher")
	statusCol := colIndex(t, header, "Implementierungsstatus")
	refCol := colIndex(t, header, "Control ID")

	var found bool
	for _, r := range rows[1:] {
		if refCol >= len(r) || r[refCol] != "A.5.7" {
			continue
		}
		found = true
		require.Greater(t, len(r), ownerCol)
		require.Greater(t, len(r), statusCol)

		// R1-20-06: the owner is the control owner, not the approver UUID.
		assert.Equal(t, controlOwner, r[ownerCol], "owner must come from ck_control.owner")
		assert.NotEqual(t, approverID, r[ownerCol], "owner must not be the approver UUID")
		require.NotEmpty(t, approverID)

		// R1-20-07: the status is German, not the raw English enum.
		assert.Equal(t, "Implementiert", r[statusCol], "status must be the German label")
		assert.NotEqual(t, "implemented", r[statusCol])
	}
	require.True(t, found, "row A.5.7 must be present in the export")
}

func colIndex(t *testing.T, header []string, name string) int {
	t.Helper()
	for i, h := range header {
		if h == name {
			return i
		}
	}
	t.Fatalf("column %q not found in header %v", name, header)
	return -1
}
