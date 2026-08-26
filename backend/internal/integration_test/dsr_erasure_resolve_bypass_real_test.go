//go:build integration

// Copyright (c) 2026 NorvikOps. All rights reserved.
// SPDX-License-Identifier: Elastic-2.0

package integration_test

// R1-36c-01: the Art. 17 DSGVO right to erasure was bypassable with one click.
// POST /dsr/:id/resolve was a second write path into status='completed' that
// never looked at the request type, so an erasure request could be closed as
// fulfilled while the subject's data stayed in the database. What remained was
// a record asserting that a deletion duty had been discharged — plausible,
// auditable-looking, and false.
//
// These tests run against real PostgreSQL because the defect and the fix both
// live in what the row actually holds after the call. Asserting only the
// returned error would pass even if the UPDATE had already committed.
//
// Run with:
//
//	go test -tags=integration ./internal/integration_test/ -run TestDSRErasureResolve

import (
	"context"
	"strings"
	"testing"

	"github.com/google/uuid"
	"github.com/hibiken/asynq"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/require"

	"github.com/matharnica/vakt/internal/modules/vaktaware"
	"github.com/matharnica/vakt/internal/modules/vakthr"
	"github.com/matharnica/vakt/internal/modules/vaktprivacy"
)

// erasureEvidenceMarker mirrors the header that ExecutePPDSRErasure writes into
// po_dsr.notes inside the erasure transaction. It is unexported in vaktprivacy,
// so this test asserts on the same literal from the outside — which is the
// point: it is the durable, on-disk proof an auditor would read.
const erasureEvidenceMarker = "--- Erasure executed ---"

// dsrRow is the persisted state that decides whether a completion claim is true.
type dsrRow struct {
	Status      string
	Notes       string
	CompletedAt *string
	ResolvedBy  *string
}

func readDSR(t *testing.T, pool *pgxpool.Pool, orgID, dsrID string) dsrRow {
	t.Helper()
	var r dsrRow
	err := pool.QueryRow(context.Background(), `
		SELECT status, COALESCE(notes, ''), completed_at::text, resolved_by::text
		FROM po_dsr WHERE id = $1 AND org_id = $2`,
		dsrID, orgID,
	).Scan(&r.Status, &r.Notes, &r.CompletedAt, &r.ResolvedBy)
	require.NoError(t, err)
	return r
}

// seedResolver creates the DPO account that closes the request. po_dsr.resolved_by
// carries a real FK to users(id), so a made-up UUID would fail on the constraint
// and hide whatever the guard actually did.
func seedResolver(t *testing.T, pool *pgxpool.Pool) string {
	t.Helper()
	var id string
	err := pool.QueryRow(context.Background(), `
		INSERT INTO users (email, display_name, password_hash)
		VALUES ($1, 'DPO', 'x') RETURNING id::text`,
		"dpo-"+uuid.New().String()+"@example.com",
	).Scan(&id)
	require.NoError(t, err)
	return id
}

func seedDSR(t *testing.T, pool *pgxpool.Pool, orgID, dsrType, email string) string {
	t.Helper()
	var id string
	err := pool.QueryRow(context.Background(), `
		INSERT INTO po_dsr (org_id, requester_name, requester_email, type, status, due_date)
		VALUES ($1, 'Betroffene Person', $2, $3, 'open', NOW() + INTERVAL '30 days')
		RETURNING id::text`,
		orgID, email, dsrType,
	).Scan(&id)
	require.NoError(t, err)
	return id
}

// newPrivacyService builds the service exactly as cmd/api wires it: every module
// that holds subject PII contributes an eraser, plus the cross-module resolver.
// Without them ExecuteErasure refuses to run at all (ADR-0079).
func newPrivacyService(pool *pgxpool.Pool) *vaktprivacy.Service {
	return vaktprivacy.NewService(pool, asynq.RedisClientOpt{}).
		WithSubjectErasers(vaktaware.NewSubjectEraser(), vakthr.NewSubjectEraser()).
		WithSubjectResolver(vakthr.NewSubjectResolver())
}

// TestDSRErasureResolveBypassIsRefused is the regression test for the defect.
// It fails loudly against the unfixed code: there, ResolveDSR returns no error
// and the row comes back completed, with completed_at and resolved_by stamped —
// a false compliance record.
func TestDSRErasureResolveBypassIsRefused(t *testing.T) {
	if testing.Short() {
		t.Skip("integration: -short mode")
	}
	ctx := context.Background()
	pool, teardown := bootPostgres(t)
	defer teardown()

	orgID := uuid.New().String()
	email := "loeschung@example.com"
	_, err := pool.Exec(ctx,
		`INSERT INTO organizations (id, name, slug) VALUES ($1, 'Test', 'test')`, orgID)
	require.NoError(t, err)

	// The subject's PII, which the erasure is supposed to remove.
	_, err = pool.Exec(ctx, `
		INSERT INTO hr_employees (org_id, email, first_name, last_name)
		VALUES ($1, $2, 'Erika', 'Musterfrau')`, orgID, email)
	require.NoError(t, err)

	dsrID := seedDSR(t, pool, orgID, "erasure", email)
	svc := newPrivacyService(pool)
	resolver := seedResolver(t, pool)

	// ── The bypass attempt ────────────────────────────────────────────────
	_, err = svc.ResolveDSR(ctx, orgID, dsrID, resolver, vaktprivacy.ResolveDSRInput{
		ResolutionType:  "fulfilled",
		ResolutionNotes: "Erledigt.",
	})
	require.Error(t, err,
		"closing an Art. 17 erasure as fulfilled without deleting anything must be refused")
	require.ErrorIs(t, err, vaktprivacy.ErrErasureNotExecuted)

	// ── The part that matters: no completion evidence may exist on disk ──
	// A refusal that still wrote the row would be the same defect with a nicer
	// error message, so the status code alone is not the assertion.
	row := readDSR(t, pool, orgID, dsrID)
	require.Equal(t, "open", row.Status, "DSR must stay open")
	require.Nil(t, row.CompletedAt, "completed_at must not be stamped")
	require.Nil(t, row.ResolvedBy, "resolved_by must not be stamped")
	require.NotContains(t, row.Notes, erasureEvidenceMarker,
		"no erasure evidence may exist — nothing was erased")
	require.NotContains(t, row.Notes, "Erledigt.",
		"the resolution note must not be persisted either")

	// And the subject's data is of course still there — nothing claimed to
	// delete it, so nothing did.
	var employees int
	require.NoError(t, pool.QueryRow(ctx,
		`SELECT count(*) FROM hr_employees WHERE org_id = $1 AND lower(email) = lower($2)`,
		orgID, email).Scan(&employees))
	require.Equal(t, 1, employees)
}

// TestDSRErasureResolveAfterExecutionSucceeds is the baseline in the direction
// that a too-broad guard would break: once the erasure has actually run, the
// request is closable.
func TestDSRErasureResolveAfterExecutionSucceeds(t *testing.T) {
	if testing.Short() {
		t.Skip("integration: -short mode")
	}
	ctx := context.Background()
	pool, teardown := bootPostgres(t)
	defer teardown()

	orgID := uuid.New().String()
	email := "erledigt@example.com"
	_, err := pool.Exec(ctx,
		`INSERT INTO organizations (id, name, slug) VALUES ($1, 'Test', 'test')`, orgID)
	require.NoError(t, err)
	_, err = pool.Exec(ctx, `
		INSERT INTO hr_employees (org_id, email, first_name, last_name)
		VALUES ($1, $2, 'Erika', 'Musterfrau')`, orgID, email)
	require.NoError(t, err)

	dsrID := seedDSR(t, pool, orgID, "erasure", email)
	svc := newPrivacyService(pool)

	// Run the real Art. 17 path — the same one the "Löschung ausführen" action
	// triggers: PATCH status=completed on an erasure DSR routes to ExecuteErasure.
	_, err = svc.UpdateDSR(ctx, orgID, dsrID, vaktprivacy.UpdateDSRInput{Status: "completed"})
	require.NoError(t, err)

	executed := readDSR(t, pool, orgID, dsrID)
	require.Equal(t, "completed", executed.Status)
	require.Contains(t, executed.Notes, erasureEvidenceMarker,
		"the erasure transaction must leave its evidence note")

	// The PII is gone — this is what makes the completion claim true.
	var employees int
	require.NoError(t, pool.QueryRow(ctx,
		`SELECT count(*) FROM hr_employees WHERE org_id = $1 AND lower(email) = lower($2)`,
		orgID, email).Scan(&employees))
	require.Equal(t, 0, employees, "Art. 17 erasure must have removed the employee row")

	// Now closing it is truthful, so it is permitted.
	_, err = svc.ResolveDSR(ctx, orgID, dsrID, seedResolver(t, pool), vaktprivacy.ResolveDSRInput{
		ResolutionType:  "fulfilled",
		ResolutionNotes: "Bestätigt.",
	})
	require.NoError(t, err, "an executed erasure must remain closable")

	after := readDSR(t, pool, orgID, dsrID)
	require.Equal(t, "completed", after.Status)
	require.Contains(t, after.Notes, erasureEvidenceMarker,
		"resolving must not destroy the erasure evidence")
}

// TestDSRAccessResolveStillWorks is the other baseline: the guard is specific to
// Art. 17 and must not touch ordinary DSR work. An access request has no
// deletion to perform, so it closes exactly as before.
func TestDSRAccessResolveStillWorks(t *testing.T) {
	if testing.Short() {
		t.Skip("integration: -short mode")
	}
	ctx := context.Background()
	pool, teardown := bootPostgres(t)
	defer teardown()

	orgID := uuid.New().String()
	_, err := pool.Exec(ctx,
		`INSERT INTO organizations (id, name, slug) VALUES ($1, 'Test', 'test')`, orgID)
	require.NoError(t, err)

	svc := newPrivacyService(pool)

	for _, typ := range []string{"access", "rectification", "restriction", "portability", "objection"} {
		t.Run(typ, func(t *testing.T) {
			dsrID := seedDSR(t, pool, orgID, typ, typ+"@example.com")
			resolver := seedResolver(t, pool)

			_, err := svc.ResolveDSR(ctx, orgID, dsrID, resolver, vaktprivacy.ResolveDSRInput{
				ResolutionType:  "fulfilled",
				ResolutionNotes: "Auskunft erteilt.",
			})
			require.NoError(t, err, "%s requests must still be closable", typ)

			row := readDSR(t, pool, orgID, dsrID)
			require.Equal(t, "completed", row.Status)
			require.NotNil(t, row.CompletedAt)
			require.NotNil(t, row.ResolvedBy)
		})
	}
}

// TestDSRErasureRejectionStillWorks: Art. 17 Abs. 3 lists lawful grounds to
// refuse an erasure. A refusal asserts no deletion, so it needs none — the guard
// must not turn a legitimate rejection into a dead end.
func TestDSRErasureRejectionStillWorks(t *testing.T) {
	if testing.Short() {
		t.Skip("integration: -short mode")
	}
	ctx := context.Background()
	pool, teardown := bootPostgres(t)
	defer teardown()

	orgID := uuid.New().String()
	_, err := pool.Exec(ctx,
		`INSERT INTO organizations (id, name, slug) VALUES ($1, 'Test', 'test')`, orgID)
	require.NoError(t, err)

	svc := newPrivacyService(pool)

	rejected := seedDSR(t, pool, orgID, "erasure", "abgelehnt@example.com")
	_, err = svc.ResolveDSR(ctx, orgID, rejected, seedResolver(t, pool), vaktprivacy.ResolveDSRInput{
		ResolutionType:  "rejected",
		ResolutionNotes: "Gesetzliche Aufbewahrungspflicht (Art. 17 Abs. 3 lit. b DSGVO).",
	})
	require.NoError(t, err, "rejecting an erasure request must stay possible")
	require.Equal(t, "rejected", readDSR(t, pool, orgID, rejected).Status)

	extended := seedDSR(t, pool, orgID, "erasure", "verlaengert@example.com")
	_, err = svc.ResolveDSR(ctx, orgID, extended, seedResolver(t, pool), vaktprivacy.ResolveDSRInput{
		ResolutionType:  "extended",
		ExtensionReason: "Komplexität der Anfrage (Art. 12 Abs. 3 DSGVO).",
	})
	require.NoError(t, err, "extending an erasure deadline must stay possible")
	require.Equal(t, "extended", readDSR(t, pool, orgID, extended).Status)
}

// TestDSRErasureEvidenceSurvivesNoteEdit: the evidence marker is what makes an
// executed erasure readable as executed. UpdatePPDSR overwrites notes wholesale,
// so a plain note edit used to wipe that proof — which would both destroy the
// audit record and silently downgrade the row to "not executed".
func TestDSRErasureEvidenceSurvivesNoteEdit(t *testing.T) {
	if testing.Short() {
		t.Skip("integration: -short mode")
	}
	ctx := context.Background()
	pool, teardown := bootPostgres(t)
	defer teardown()

	orgID := uuid.New().String()
	email := "notiz@example.com"
	_, err := pool.Exec(ctx,
		`INSERT INTO organizations (id, name, slug) VALUES ($1, 'Test', 'test')`, orgID)
	require.NoError(t, err)
	_, err = pool.Exec(ctx, `
		INSERT INTO hr_employees (org_id, email, first_name, last_name)
		VALUES ($1, $2, 'Erika', 'Musterfrau')`, orgID, email)
	require.NoError(t, err)

	dsrID := seedDSR(t, pool, orgID, "erasure", email)
	svc := newPrivacyService(pool)

	_, err = svc.UpdateDSR(ctx, orgID, dsrID, vaktprivacy.UpdateDSRInput{Status: "completed"})
	require.NoError(t, err)
	require.Contains(t, readDSR(t, pool, orgID, dsrID).Notes, erasureEvidenceMarker)

	// An operator reopens the request and replaces the notes — the edit path
	// that actually reaches Repository.UpdateDSR. (A PATCH that keeps the status
	// at "completed" is diverted to ExecuteErasure, which returns early on an
	// already-completed row and never touches the notes at all.)
	_, err = svc.UpdateDSR(ctx, orgID, dsrID, vaktprivacy.UpdateDSRInput{
		Status: "in_progress",
		Notes:  "Rückfrage der Aufsichtsbehörde beantwortet.",
	})
	require.NoError(t, err)

	row := readDSR(t, pool, orgID, dsrID)
	require.Contains(t, row.Notes, "Rückfrage der Aufsichtsbehörde beantwortet.",
		"the edit itself must take effect")
	require.Contains(t, row.Notes, erasureEvidenceMarker,
		"the Art. 17 evidence block must survive a note edit")
	require.True(t, strings.Contains(row.Notes, "users anonymised:"),
		"the per-table counts belong to the evidence block and must survive too")

	// Same for the resolve path, which replaces notes whenever resolution_notes
	// is non-empty.
	_, err = svc.ResolveDSR(ctx, orgID, dsrID, seedResolver(t, pool), vaktprivacy.ResolveDSRInput{
		ResolutionType:  "rejected",
		ResolutionNotes: "Abschließende Stellungnahme.",
	})
	require.NoError(t, err)

	resolved := readDSR(t, pool, orgID, dsrID)
	require.Contains(t, resolved.Notes, "Abschließende Stellungnahme.")
	require.Contains(t, resolved.Notes, erasureEvidenceMarker,
		"resolving must not destroy the Art. 17 evidence block either")
}
