//go:build integration

// Copyright (c) 2026 NorvikOps. All rights reserved.
// SPDX-License-Identifier: Elastic-2.0

package integration_test

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/matharnica/vakt/internal/modules/vaktaware"
)

// TestVaktaware_OverdueAssignments (L1-05).
//
// ListSROverdueAssignments filterte auf `is_overdue = TRUE`. Diese Spalte schrieb
// kein Codepfad je — kein UPDATE, kein Job, keine Ableitung aus due_date. Damit
// war „Ueberfaellige Schulungen" dauerhaft leer, auch bei 60 Tagen
// Ueberschreitung, und jede Zuweisung trug in ihrer Antwort is_overdue:false.
//
// Der Fix leitet den Wert in der Abfrage ab. Ein abgeleiteter Wert kann nicht
// veralten — ein Feld, das ein Job pflegen muesste, kann es immer.
func TestVaktaware_OverdueAssignments(t *testing.T) {
	if testing.Short() {
		t.Skip("integration: -short mode")
	}
	pool, orgID, cleanup := bootPostgresWithOrg(t)
	defer cleanup()
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	defer cancel()

	userID := awareTestUser(t, ctx, pool, "aware-overdue@acme.test")
	svc := vaktaware.NewService(pool, vaktaware.SMTPConfig{Host: "smtp.test", Port: "25"})
	repo := vaktaware.NewRepository(pool)

	module, err := repo.CreateModule(ctx, orgID, userID, vaktaware.CreateModuleInput{
		Title: "Phishing erkennen", Type: "video", AttackType: "phishing",
		ContentURL: "https://kunde.test/video", DurationSeconds: 600, PassingScore: 80,
	})
	require.NoError(t, err)

	group, err := repo.CreateTargetGroup(ctx, orgID, "Alle", "manual")
	require.NoError(t, err)
	spaet, err := repo.CreateTarget(ctx, orgID, group.ID, "spaet@kunde.test", "Spaet", "Dran", "IT")
	require.NoError(t, err)
	fertig, err := repo.CreateTarget(ctx, orgID, group.ID, "fertig@kunde.test", "Schon", "Fertig", "IT")
	require.NoError(t, err)
	frisch, err := repo.CreateTarget(ctx, orgID, group.ID, "frisch@kunde.test", "Noch", "Zeit", "IT")
	require.NoError(t, err)

	vorbei := time.Now().Add(-60 * 24 * time.Hour)
	kuenftig := time.Now().Add(30 * 24 * time.Hour)

	aSpaet, err := repo.UpsertAssignment(ctx, orgID, module.ID, &spaet.ID, "IT", vorbei)
	require.NoError(t, err)
	aFertig, err := repo.UpsertAssignment(ctx, orgID, module.ID, &fertig.ID, "IT", vorbei)
	require.NoError(t, err)
	_, err = repo.UpsertAssignment(ctx, orgID, module.ID, &frisch.ID, "IT", kuenftig)
	require.NoError(t, err)

	// UpsertAssignment setzt due_date = GREATEST(neu, alt); die Vergangenheit
	// muss deshalb direkt gestellt werden — das simuliert nur den Zeitablauf.
	_, err = pool.Exec(ctx,
		`UPDATE sr_assignments SET due_date = $3 WHERE org_id = $1::uuid AND id = $2::uuid`,
		orgID, aSpaet.ID, vorbei)
	require.NoError(t, err)
	_, err = pool.Exec(ctx,
		`UPDATE sr_assignments SET due_date = $3 WHERE org_id = $1::uuid AND id = $2::uuid`,
		orgID, aFertig.ID, vorbei)
	require.NoError(t, err)

	// Eine der beiden ueberfaelligen Zuweisungen wurde inzwischen erledigt — sie
	// darf nicht mehr als ueberfaellig gelten, sonst waere die Liste nur laut
	// statt richtig.
	_, err = repo.CreateCompletion(ctx, orgID, aFertig.ID, nil, true)
	require.NoError(t, err)

	overdue, err := svc.ListAssignments(ctx, orgID, "overdue")
	require.NoError(t, err)
	require.Len(t, overdue, 1,
		"genau eine Zuweisung ist ueberfaellig: 60 Tage vorbei und nicht erledigt (gefunden: %d)", len(overdue))
	assert.Equal(t, aSpaet.ID, overdue[0].ID)
	assert.True(t, overdue[0].IsOverdue, "die Antwort muss das Merkmal auch tragen, nicht nur die Liste filtern")

	// Und in der ungefilterten Liste muss das Merkmal je Zeile stimmen.
	alle, err := svc.ListAssignments(ctx, orgID, "")
	require.NoError(t, err)
	require.Len(t, alle, 3)
	byID := map[string]bool{}
	for _, a := range alle {
		byID[a.ID] = a.IsOverdue
	}
	assert.True(t, byID[aSpaet.ID], "60 Tage vorbei, nicht erledigt")
	assert.False(t, byID[aFertig.ID], "vorbei, aber erledigt")

	completed, err := svc.ListAssignments(ctx, orgID, "completed")
	require.NoError(t, err)
	require.Len(t, completed, 1)
	assert.False(t, completed[0].IsOverdue)
}
