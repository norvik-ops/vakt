//go:build integration

// Copyright (c) 2026 NorvikOps. All rights reserved.
// SPDX-License-Identifier: Elastic-2.0

package integration_test

import (
	"context"
	"fmt"
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

	"github.com/matharnica/vakt/internal/modules/vaktcomply"
	"github.com/matharnica/vakt/internal/modules/vaktcomply/policy"
	shareddb "github.com/matharnica/vakt/internal/shared/db"
)

// TestSoAStatusFromControl_DraftLiveApprovedFrozen pins the single source for
// the SoA implementation status (UX-Durchgang, Kernweg 6).
//
// Before: ck_soa_entries.implementation_status was a second, hand-kept status.
// A.5.2 read "In Bearbeitung" on the control page (one evidence) and "Nicht
// begonnen" in the SoA. Now:
//
//  1. a draft SoA shows the status and the (German) title of the control, and
//     summary, page and PDF count it with the shared rule (voll = ganz,
//     teilweise = halb);
//  2. the PUT no longer writes a status of its own;
//  3. approval freezes the status: a later change on the control does not
//     move the approved version;
//  4. the next draft (created by an edit) reads the control again.
//
// Non-vacuity: drop resolveSoAEntry from ListSoAEntries and step 1 reads
// "not_started"; drop the freeze in ApproveSoAVersion and step 3 reads the
// stale "not_started" instead of "partial"; skip the approved branch in
// resolveSoAEntry and step 3 reads "implemented".
func TestSoAStatusFromControl_DraftLiveApprovedFrozen(t *testing.T) {
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
		INSERT INTO organizations (name, slug) VALUES ('SoAStatusOrg', 'soastatusorg')
		RETURNING id::text`).Scan(&orgID))
	// Same spelling as the Kernweg instance ("ISO27001", no space).
	fwID := seedFramework(ctx, t, pool, orgID, "ISO27001")

	var a52, a51 string
	require.NoError(t, pool.QueryRow(ctx, `
		INSERT INTO ck_controls (org_id, framework_id, control_id, title, domain)
		VALUES ($1::uuid, $2::uuid, 'A.5.2', 'Rollen und Verantwortlichkeiten', '5')
		RETURNING id::text`, orgID, fwID).Scan(&a52))
	require.NoError(t, pool.QueryRow(ctx, `
		INSERT INTO ck_controls (org_id, framework_id, control_id, title, domain, manual_status)
		VALUES ($1::uuid, $2::uuid, 'A.5.1', 'Richtlinien zur Informationssicherheit', '5', 'implemented')
		RETURNING id::text`, orgID, fwID).Scan(&a51))
	// One pending evidence and no manual status: the control page shows "In Bearbeitung".
	_, err = pool.Exec(ctx, `
		INSERT INTO ck_evidence (control_id, org_id, title) VALUES ($1::uuid, $2::uuid, 'Rollenkonzept')`,
		a52, orgID)
	require.NoError(t, err)

	svc := vaktcomply.NewService(pool)
	h := vaktcomply.NewHandler(svc)
	e := echo.New()

	require.NoError(t, svc.InitDedicatedSoA(ctx, orgID))

	entry := func(t *testing.T, ref string) policy.SoADedicatedEntry {
		t.Helper()
		got, err := svc.GetDedicatedSoAEntry(ctx, orgID, ref)
		require.NoError(t, err)
		return *got
	}
	pdfText := func(t *testing.T) string {
		t.Helper()
		req := httptest.NewRequest(http.MethodGet, "/?format=pdf", nil)
		rec := httptest.NewRecorder()
		c := e.NewContext(req, rec)
		c.Set("org_id", orgID)
		require.NoError(t, h.ExportDedicatedSoA(c))
		require.Equal(t, http.StatusOK, rec.Code)
		return strings.Join(pdfVisibleText(t, rec.Body.Bytes()), "\x1f")
	}

	// 1. Draft: status and title of the control.
	got := entry(t, "A.5.2")
	assert.Equal(t, "partial", got.ImplementationStatus, "Entwurf zeigt den Status der Kontrolle")
	assert.Equal(t, policy.SoAStatusSourceControl, got.StatusSource)
	assert.Equal(t, "Rollen und Verantwortlichkeiten", got.ControlName, "deutscher Titel der Kontrolle statt englischem Seed")
	require.NotNil(t, got.LinkedControlID)
	assert.Equal(t, a52, *got.LinkedControlID)
	require.NotNil(t, got.LinkedFrameworkID)
	assert.Equal(t, fwID, *got.LinkedFrameworkID)
	assert.Equal(t, "implemented", entry(t, "A.5.1").ImplementationStatus)
	// A.8.9 has no control in this org: stored value, marked as such.
	assert.Equal(t, policy.SoAStatusSourceSoA, entry(t, "A.8.9").StatusSource)

	sum, err := svc.GetDedicatedSoASummary(ctx, orgID)
	require.NoError(t, err)
	assert.Equal(t, 1, sum.ImplementedCount)
	assert.Equal(t, 1, sum.PartialCount)
	assert.InDelta(t, policy.ReadinessScore(1, 1, sum.ApplicableCount), sum.ImplementationPct, 0.0001)

	text := pdfText(t)
	assert.Contains(t, text, "In Bearbeitung", "PDF nutzt dasselbe Label wie die Kontrollseite")
	assert.Contains(t, text, fmt.Sprintf("Umsetzungsgrad: %.0f%%", sum.ImplementationPct), "PDF und Seite zählen gleich")

	// 2. The PUT writes no status of its own any more.
	require.NoError(t, svc.UpdateDedicatedSoAEntry(ctx, orgID, "A.5.2", policy.UpdateSoAEntryInput{
		Applicable:           true,
		Justification:        "Rollen sind festgelegt",
		ImplementationStatus: "planned",
		ManuallySet:          true,
	}))
	got = entry(t, "A.5.2")
	assert.Equal(t, "partial", got.ImplementationStatus, "ein Status aus dem SoA-PUT darf die Kontrolle nicht überstimmen")
	assert.Equal(t, "Rollen sind festgelegt", got.Justification, "Begründung bleibt SoA-eigen und editierbar")

	// 3. Approve, then change the control: the approved version stays as it was.
	var approverID string
	require.NoError(t, pool.QueryRow(ctx, `
		INSERT INTO users (email, display_name) VALUES ('approver@soastatus.test', 'Freigeber')
		RETURNING id::text`).Scan(&approverID))
	require.NoError(t, svc.ApproveDedicatedSoA(ctx, orgID, approverID))
	_, err = pool.Exec(ctx, `UPDATE ck_controls SET manual_status = 'implemented', title = 'Umbenannt' WHERE id = $1::uuid AND org_id = $2::uuid`, a52, orgID)
	require.NoError(t, err)

	got = entry(t, "A.5.2")
	assert.Equal(t, 1, got.Version)
	assert.Equal(t, "partial", got.ImplementationStatus, "eingefrorene Version ändert sich nicht nachträglich")
	assert.Equal(t, "Rollen und Verantwortlichkeiten", got.ControlName, "Titel ist mit eingefroren")
	assert.Equal(t, policy.SoAStatusSourceSnapshot, got.StatusSource)
	var stored string
	require.NoError(t, pool.QueryRow(ctx, `
		SELECT implementation_status FROM ck_soa_entries WHERE org_id = $1::uuid AND version = 1 AND control_ref = 'A.5.2'`,
		orgID).Scan(&stored))
	assert.Equal(t, "partial", stored, "die Freigabe schreibt den Stand in ck_soa_entries")
	frozenSum, err := svc.GetDedicatedSoASummary(ctx, orgID)
	require.NoError(t, err)
	assert.Equal(t, "approved", frozenSum.Status)
	assert.Equal(t, 1, frozenSum.PartialCount)
	assert.Contains(t, pdfText(t), "In Bearbeitung", "PDF der freigegebenen Version zeigt den eingefrorenen Stand")

	// 4. An edit opens draft v2, which reads the control again.
	require.NoError(t, svc.UpdateDedicatedSoAEntry(ctx, orgID, "A.5.2", policy.UpdateSoAEntryInput{
		Applicable:    true,
		Justification: "Rollen sind festgelegt (v2)",
	}))
	got = entry(t, "A.5.2")
	assert.Equal(t, 2, got.Version)
	assert.Equal(t, "implemented", got.ImplementationStatus)
	assert.Equal(t, "Umbenannt", got.ControlName)
	assert.Equal(t, policy.SoAStatusSourceControl, got.StatusSource)

	// Version 1 still holds its snapshot.
	require.NoError(t, pool.QueryRow(ctx, `
		SELECT implementation_status FROM ck_soa_entries WHERE org_id = $1::uuid AND version = 1 AND control_ref = 'A.5.2'`,
		orgID).Scan(&stored))
	assert.Equal(t, "partial", stored)
}
