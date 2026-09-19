//go:build integration

// Copyright (c) 2026 NorvikOps. All rights reserved.
// SPDX-License-Identifier: Elastic-2.0

package integration_test

// MaterializeEnrollmentsAsTargets gegen echtes Postgres — ADR-0088 / R1-36a-D08.
//
// Der Defekt war unsichtbar unterhalb der geschriebenen Zeile: auto-enrollte
// Mitarbeiter standen nur in sr_campaign_enrollments, SendCampaignEmails las
// aber ausschließlich sr_targets — sie bekamen die Kampagne nie. Nachweisbar ist
// die Materialisierung deshalb nur an der sr_targets-Zeile, die danach existiert
// (oder eben nicht doppelt existiert).
//
// Ausführen mit:
//
//	go test -tags=integration ./internal/integration_test/ -run TestMaterializeEnrollments

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/require"

	"github.com/matharnica/vakt/internal/modules/vaktaware"
)

// seedMaterializeFixture legt Organisation, Zielgruppe und eine auf die Gruppe
// zeigende Kampagne an und gibt ihre IDs zurück.
func seedMaterializeFixture(t *testing.T, pool *pgxpool.Pool) (orgID, campaignID, groupID string) {
	t.Helper()
	ctx := context.Background()

	orgID = uuid.New().String()
	campaignID = uuid.New().String()
	groupID = uuid.New().String()

	_, err := pool.Exec(ctx, `
		INSERT INTO organizations (id, name, slug) VALUES ($1, 'MatOrg', $2)`,
		orgID, "matorg-"+uuid.New().String()[:8])
	require.NoError(t, err)

	_, err = pool.Exec(ctx, `
		INSERT INTO sr_target_groups (id, org_id, name) VALUES ($1, $2, 'Onboarding')`,
		groupID, orgID)
	require.NoError(t, err)

	_, err = pool.Exec(ctx, `
		INSERT INTO sr_campaigns (id, org_id, name, status, group_id, from_name, from_email, subject)
		VALUES ($1, $2, 'Onboarding-Schulung', 'running', $3, 'IT Security', 'it@example.com', 'Willkommen')`,
		campaignID, orgID, groupID)
	require.NoError(t, err)

	return orgID, campaignID, groupID
}

func countTargetsInGroup(t *testing.T, pool *pgxpool.Pool, groupID string) int {
	t.Helper()
	var n int
	// orgid-lint: global — Test-Assertion, group_id-scoped in isolierter bootPostgres-Test-Org
	require.NoError(t, pool.QueryRow(context.Background(),
		`SELECT COUNT(*) FROM sr_targets WHERE group_id = $1`, groupID).Scan(&n))
	return n
}

// TestMaterializeEnrollmentsCreatesTargetOnce ist die Kernabnahme: ein
// auto-enrollter Mitarbeiter mit Mail landet als sr_targets-Zeile, und ein
// zweiter Lauf erzeugt keine zweite Zeile (Idempotenz — Schutz gegen
// Doppelversand bei einem erneut zugestellten Asynq-Task).
func TestMaterializeEnrollmentsCreatesTargetOnce(t *testing.T) {
	if testing.Short() {
		t.Skip("integration: -short mode")
	}
	ctx := context.Background()
	pool, teardown := bootPostgres(t)
	defer teardown()

	orgID, campaignID, groupID := seedMaterializeFixture(t, pool)
	repo := vaktaware.NewRepository(pool)

	// Ein adressierbarer Auto-Enrollment (mit PII) …
	require.NoError(t, repo.CreateCampaignEnrollment(ctx, orgID, campaignID,
		uuid.New().String(), vaktaware.SourceAutoNewEmployee, "ada.auto@example.com", "Ada Auto"))
	// … und einer OHNE PII (phishing_click-Pfad): darf NICHT materialisiert werden.
	require.NoError(t, repo.CreateCampaignEnrollment(ctx, orgID, campaignID,
		uuid.New().String(), vaktaware.SourceAutoPhishingClick, "", ""))

	require.NoError(t, repo.MaterializeEnrollmentsAsTargets(ctx, orgID, campaignID, groupID))

	require.Equal(t, 1, countTargetsInGroup(t, pool, groupID),
		"genau der adressierbare Enrollment muss als sr_targets-Zeile erscheinen (NULL-Mail übersprungen)")

	var email, firstName string
	// orgid-lint: global — Test-Assertion, group_id-scoped in isolierter bootPostgres-Test-Org
	require.NoError(t, pool.QueryRow(ctx, `
		SELECT email, first_name FROM sr_targets WHERE group_id = $1`, groupID).Scan(&email, &firstName))
	require.Equal(t, "ada.auto@example.com", email)
	require.Equal(t, "Ada Auto", firstName, "full_name muss in first_name landen (Greeting-Token)")

	// Zweiter Lauf: keine Doppelanlage.
	require.NoError(t, repo.MaterializeEnrollmentsAsTargets(ctx, orgID, campaignID, groupID))
	require.Equal(t, 1, countTargetsInGroup(t, pool, groupID),
		"ein erneuter Lauf darf keine zweite sr_targets-Zeile erzeugen")
}

// TestMaterializeEnrollmentsDedupsAgainstExistingTarget belegt den Dedup gegen
// eine bereits bestehende Zielperson — case-insensitiv über lower(email). Ein
// Mitarbeiter, der bereits von Hand in die Gruppe importiert wurde, darf durch
// den Auto-Enrollment keine zweite (doppelt anzuschreibende) Zeile bekommen.
func TestMaterializeEnrollmentsDedupsAgainstExistingTarget(t *testing.T) {
	if testing.Short() {
		t.Skip("integration: -short mode")
	}
	ctx := context.Background()
	pool, teardown := bootPostgres(t)
	defer teardown()

	orgID, campaignID, groupID := seedMaterializeFixture(t, pool)
	repo := vaktaware.NewRepository(pool)

	// Bestehende Zielperson, Mail in ANDERER Schreibweise.
	_, err := pool.Exec(ctx, `
		INSERT INTO sr_targets (org_id, group_id, email, first_name)
		VALUES ($1, $2, 'Bob.Bestand@Example.com', 'Bob')`, orgID, groupID)
	require.NoError(t, err)

	// Auto-Enrollment mit derselben Mail in Kleinschreibung.
	require.NoError(t, repo.CreateCampaignEnrollment(ctx, orgID, campaignID,
		uuid.New().String(), vaktaware.SourceAutoNewEmployee, "bob.bestand@example.com", "Bob Bestand"))

	require.NoError(t, repo.MaterializeEnrollmentsAsTargets(ctx, orgID, campaignID, groupID))

	require.Equal(t, 1, countTargetsInGroup(t, pool, groupID),
		"eine bereits als Zielperson vorhandene Mail darf nicht ein zweites Mal angelegt werden (lower(email)-Dedup)")
}
