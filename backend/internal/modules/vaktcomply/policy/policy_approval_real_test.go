// Copyright (c) 2026 NorvikOps. All rights reserved.
// SPDX-License-Identifier: Elastic-2.0

package policy

import (
	"context"
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Service.UpdatePolicy gegen die echte Datenbank (Migration 272): Die Freigabe
// trägt die angemeldete Identität, nie den Freitext last_updated_by; ein Edit an
// einer aktiven Richtlinie lässt sie stehen; das Verlassen von active leert sie;
// eine fremde Organisation erreicht die Richtlinie nicht.
func TestPolicyApproval_ServiceGegenEchteDB(t *testing.T) {
	url := os.Getenv("VAKT_DB_URL")
	if url == "" {
		t.Skip("VAKT_DB_URL not set — dieser Test braucht eine migrierte Postgres (CI setzt sie)")
	}
	ctx := context.Background()
	pool, err := pgxpool.New(ctx, url)
	require.NoError(t, err)
	t.Cleanup(pool.Close)

	suffix := fmt.Sprintf("%d-%d", os.Getpid(), time.Now().UnixNano())
	newOrg := func(name string) string {
		var id string
		require.NoError(t, pool.QueryRow(ctx,
			`INSERT INTO organizations (name, slug) VALUES ($1, $1) RETURNING id::text`, name+"-"+suffix).Scan(&id))
		return id
	}
	newUser := func(email, name string) string {
		var id string
		require.NoError(t, pool.QueryRow(ctx,
			`INSERT INTO users (email, display_name) VALUES ($1, $2) RETURNING id::text`,
			suffix+"-"+email, name).Scan(&id))
		return id
	}
	orgID := newOrg("freigabe")
	otherOrg := newOrg("fremd")
	alice := newUser("alice@example.test", "Alice Freigabe")
	bob := newUser("bob@example.test", "")
	t.Cleanup(func() {
		bg := context.Background()
		for _, o := range []string{orgID, otherOrg} {
			_, _ = pool.Exec(bg, `DELETE FROM ck_policy_versions WHERE org_id = $1::uuid`, o)
			_, _ = pool.Exec(bg, `DELETE FROM ck_policies WHERE org_id = $1::uuid`, o)
			_, _ = pool.Exec(bg, `DELETE FROM organizations WHERE id = $1::uuid`, o)
		}
		_, _ = pool.Exec(bg, `DELETE FROM users WHERE id = ANY($1::uuid[])`, []string{alice, bob})
	})

	svc := NewService(pool)
	created, err := svc.CreatePolicy(ctx, orgID, CreatePolicyInput{Title: "Passwortrichtlinie"})
	require.NoError(t, err)
	require.Equal(t, "draft", created.Status)
	assert.Nil(t, created.ApprovedBy)

	fremder := "Fremder"
	in := UpdatePolicyInput{Title: "Passwortrichtlinie", Status: "active", UpdatedBy: &fremder}

	before := time.Now().Add(-time.Minute)
	p, event, err := svc.UpdatePolicy(ctx, orgID, created.ID, alice, in)
	require.NoError(t, err)
	assert.Equal(t, ApprovalGranted, event)
	require.NotNil(t, p.ApprovedBy)
	assert.Equal(t, alice, *p.ApprovedBy, "Freigeber ist die angemeldete Identität")
	require.NotNil(t, p.ApprovedByName)
	assert.Equal(t, "Alice Freigabe", *p.ApprovedByName, "Name kommt aus users, nicht aus dem Body")
	require.NotNil(t, p.ApprovedAt)
	assert.True(t, p.ApprovedAt.After(before))
	assert.Equal(t, "Fremder", p.LastUpdatedBy, "Freitext bleibt „Bearbeitet von“")

	t.Run("Edit_an_aktiver_Richtlinie_laesst_Freigabe_stehen", func(t *testing.T) {
		edit := in
		edit.Description = "neuer Text"
		got, event, err := svc.UpdatePolicy(ctx, orgID, created.ID, bob, edit)
		require.NoError(t, err)
		assert.Equal(t, ApprovalUnchanged, event)
		require.NotNil(t, got.ApprovedBy)
		assert.Equal(t, alice, *got.ApprovedBy, "Bob hat bearbeitet, nicht freigegeben")
		assert.True(t, got.ApprovedAt.Equal(*p.ApprovedAt))
	})

	t.Run("Fremde_Org_sieht_und_aendert_nichts", func(t *testing.T) {
		_, err := svc.GetPolicy(ctx, otherOrg, created.ID)
		assert.ErrorIs(t, err, ErrNotFound)
		_, _, err = svc.UpdatePolicy(ctx, otherOrg, created.ID, bob, UpdatePolicyInput{Title: "x", Status: "draft"})
		assert.ErrorIs(t, err, ErrNotFound)
		got, err := svc.GetPolicy(ctx, orgID, created.ID)
		require.NoError(t, err)
		assert.Equal(t, "active", got.Status)
	})

	t.Run("Zurueck_auf_Entwurf_leert_Freigabe", func(t *testing.T) {
		back := in
		back.Status = "draft"
		got, event, err := svc.UpdatePolicy(ctx, orgID, created.ID, bob, back)
		require.NoError(t, err)
		assert.Equal(t, ApprovalRevoked, event)
		assert.Nil(t, got.ApprovedBy)
		assert.Nil(t, got.ApprovedByName)
		assert.Nil(t, got.ApprovedAt)
	})

	t.Run("Neue_Freigabe_ohne_Anzeigename_zeigt_E-Mail", func(t *testing.T) {
		got, event, err := svc.UpdatePolicy(ctx, orgID, created.ID, bob, in)
		require.NoError(t, err)
		assert.Equal(t, ApprovalGranted, event)
		require.NotNil(t, got.ApprovedByName)
		assert.Equal(t, suffix+"-bob@example.test", *got.ApprovedByName)

		list, err := svc.ListPolicies(ctx, orgID)
		require.NoError(t, err)
		require.Len(t, list, 1)
		require.NotNil(t, list[0].ApprovedBy)
		assert.Equal(t, bob, *list[0].ApprovedBy, "die Liste trägt dieselbe Quelle")
	})
}
