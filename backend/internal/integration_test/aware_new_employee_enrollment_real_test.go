//go:build integration

// Copyright (c) 2026 NorvikOps. All rights reserved.
// SPDX-License-Identifier: Elastic-2.0

package integration_test

// Die Einschreibungskette gegen echtes Postgres — R1-35-01 und R1-SA25-01.
//
// Warum dieser Test bis in die Datenbank geht und nicht bis zur Warteschlange:
// Beide Defekte waren für jede Stufe darunter unsichtbar. Der Code übersetzte
// und baute fehlerfrei (`go build`), die Wertemengen kollidierten erst im
// CHECK-Constraint, und der abgesetzte Auftrag sah in jeder Prüfung bis zur
// Warteschlange nach Erfolg aus. Nachweisbar ist die Kette deshalb nur an der
// geschriebenen Zeile.
//
// Ausführen mit:
//
//	go test -tags=integration ./internal/integration_test/ -run TestNewEmployee

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/require"

	"github.com/matharnica/vakt/internal/modules/vaktaware"
	"github.com/matharnica/vakt/internal/modules/vakthr"
)

// seedEnrollmentFixture legt Organisation, Kampagne und — auf Wunsch — eine
// aktive new_employee-Regel an, die auf die Kampagne zeigt.
func seedEnrollmentFixture(t *testing.T, pool *pgxpool.Pool, withRule bool) (orgID, campaignID string) {
	t.Helper()
	ctx := context.Background()

	orgID = uuid.New().String()
	campaignID = uuid.New().String()

	_, err := pool.Exec(ctx, `
		INSERT INTO organizations (id, name, slug) VALUES ($1, 'EnrollOrg', $2)`,
		orgID, "enrollorg-"+uuid.New().String()[:8])
	require.NoError(t, err)

	_, err = pool.Exec(ctx, `
		INSERT INTO sr_campaigns (id, org_id, name, status, from_name, from_email, subject)
		VALUES ($1, $2, 'Onboarding-Schulung', 'running', 'IT Security', 'it@example.com', 'Willkommen')`,
		campaignID, orgID)
	require.NoError(t, err)

	if withRule {
		_, err = pool.Exec(ctx, `
			INSERT INTO sr_enrollment_rules (org_id, name, trigger_type, target_campaign_id, is_active)
			VALUES ($1, 'Neueintritte', 'new_employee', $2, true)`,
			orgID, campaignID)
		require.NoError(t, err)
	}
	return orgID, campaignID
}

// newWiredHRService baut die Kette so, wie cmd/api/routes.go und
// cmd/worker/services.go sie bauen: vaktaware als Abonnent des
// Eintritts-Ereignisses von vakthr.
func newWiredHRService(pool *pgxpool.Pool) *vakthr.Service {
	aware := vaktaware.NewService(pool, vaktaware.SMTPConfig{})
	return vakthr.NewServiceFromPool(pool).
		WithEmployeeOnboardingTrigger(vaktaware.NewEnrollmentTrigger(aware))
}

// TestNewEmployeeEntryWritesEnrollmentRow ist die ROT-bei-Regression-Abnahme.
//
// Sie löst einen echten Eintritt über vakthr.CreateEmployee aus — denselben
// Weg, den der HTTP-Handler nimmt — und liest die Zeile danach aus
// sr_campaign_enrollments zurück. Sie schlägt fehl, sobald einer der beiden
// Defekte zurückgedreht wird:
//
//   - ohne Abbildung der Wertemengen: INSERT wird mit 23514 abgewiesen,
//     0 Zeilen;
//   - ohne Abonnent (WithEmployeeOnboardingTrigger weggelassen oder Noop):
//     der Auslöser feuert nie, 0 Zeilen.
func TestNewEmployeeEntryWritesEnrollmentRow(t *testing.T) {
	if testing.Short() {
		t.Skip("integration: -short mode")
	}
	ctx := context.Background()
	pool, teardown := bootPostgres(t)
	defer teardown()

	orgID, campaignID := seedEnrollmentFixture(t, pool, true)
	hr := newWiredHRService(pool)

	emp, err := hr.CreateEmployee(ctx, vakthr.Actor{OrgID: orgID, UserID: uuid.New().String()},
		vakthr.CreateEmployeeInput{
			FirstName: "Nina", LastName: "Neu",
			Email:      "nina.neu@example.com",
			Department: "Vertrieb",
		})
	require.NoError(t, err)
	require.NotEmpty(t, emp.ID)

	// Die eigentliche Aussage: die Zeile steht in der Datenbank, mit dem
	// richtigen Wert aus der Ziel-Wertemenge.
	var source string
	var count int
	require.NoError(t, pool.QueryRow(ctx, `
		SELECT COUNT(*) FROM sr_campaign_enrollments
		WHERE org_id = $1 AND campaign_id = $2 AND employee_id = $3`,
		orgID, campaignID, emp.ID).Scan(&count))
	require.Equal(t, 1, count,
		"Eintritt hat keine Einschreibung erzeugt — Auslöser nicht verdrahtet oder INSERT abgewiesen")

	require.NoError(t, pool.QueryRow(ctx, `
		SELECT source FROM sr_campaign_enrollments
		WHERE org_id = $1 AND campaign_id = $2 AND employee_id = $3`,
		orgID, campaignID, emp.ID).Scan(&source))
	require.Equal(t, vaktaware.SourceAutoNewEmployee, source,
		"source muss der Ziel-Wertemenge entstammen, nicht dem Auslösertyp")
}

// TestManualEnrollmentUnchanged ist die GRÜN-auf-der-Baseline-Abnahme.
//
// Eine manuelle Einschreibung (source='manual') muss unverändert funktionieren,
// und der automatische Weg darf sie nicht überschreiben: ein Mitarbeiter, der
// bereits von Hand eingeschrieben ist, behält beim Eintritt seine Herkunft.
// Genau diese Unterscheidung ist der Grund, die Wertemengen NICHT per Migration
// zusammenzulegen.
func TestManualEnrollmentUnchanged(t *testing.T) {
	if testing.Short() {
		t.Skip("integration: -short mode")
	}
	ctx := context.Background()
	pool, teardown := bootPostgres(t)
	defer teardown()

	orgID, campaignID := seedEnrollmentFixture(t, pool, true)
	repo := vaktaware.NewRepository(pool)

	empID := uuid.New().String()
	require.NoError(t, repo.CreateCampaignEnrollment(ctx, orgID, campaignID, empID, vaktaware.SourceManual),
		"manuelle Einschreibung muss unverändert schreiben")

	var source string
	require.NoError(t, pool.QueryRow(ctx, `
		SELECT source FROM sr_campaign_enrollments
		WHERE org_id = $1 AND campaign_id = $2 AND employee_id = $3`,
		orgID, campaignID, empID).Scan(&source))
	require.Equal(t, vaktaware.SourceManual, source)

	// Derselbe Mitarbeiter durchläuft nun den automatischen Weg. Er ist schon
	// eingeschrieben, also darf keine zweite Zeile entstehen und die Herkunft
	// muss 'manual' bleiben.
	aware := vaktaware.NewService(pool, vaktaware.SMTPConfig{})
	require.NoError(t, aware.HandleAutoEnrollment(ctx, vaktaware.AutoEnrollmentPayload{
		OrgID:       orgID,
		TriggerType: vaktaware.TriggerNewEmployee,
		EmployeeID:  empID,
	}))

	var count int
	require.NoError(t, pool.QueryRow(ctx, `
		SELECT COUNT(*) FROM sr_campaign_enrollments
		WHERE org_id = $1 AND campaign_id = $2 AND employee_id = $3`,
		orgID, campaignID, empID).Scan(&count))
	require.Equal(t, 1, count, "keine Doppel-Einschreibung")

	require.NoError(t, pool.QueryRow(ctx, `
		SELECT source FROM sr_campaign_enrollments
		WHERE org_id = $1 AND campaign_id = $2 AND employee_id = $3`,
		orgID, campaignID, empID).Scan(&source))
	require.Equal(t, vaktaware.SourceManual, source,
		"der automatische Weg darf eine manuelle Herkunft nicht überschreiben")
}

// Die I4-Sperrklinke — „jeder abgebildete Wert wird vom CHECK-Constraint
// angenommen" — liegt bewusst NICHT hier, sondern in
// internal/modules/vaktaware/enrollment_source_real_test.go: sie muss die
// unexportierte Abbildung selbst befragen, statt deren Ergebnis nachzubauen.
// Ein Test, der die Werteliste im Testcode wiederholt, prüft seine eigene
// Kopie und nicht den Code.

// TestORP3InductionNeedsRealEnrollments belegt, dass der Prüfnachweis nicht
// mehr allein auf der Existenz einer Regel steht.
//
// Vor dem Fix meldete ORP.3.A3 „erfüllt", sobald eine aktive Regel existierte —
// auch wenn sie nie eine Zeile erzeugt hatte, was wegen R1-35-01 nie der Fall
// sein konnte. Der Bericht behauptete also eine Einweisung, die nie stattfand,
// und floss so als Nachweis nach Vakt Comply.
func TestORP3InductionNeedsRealEnrollments(t *testing.T) {
	if testing.Short() {
		t.Skip("integration: -short mode")
	}
	ctx := context.Background()
	pool, teardown := bootPostgres(t)
	defer teardown()

	orgID, _ := seedEnrollmentFixture(t, pool, true)
	aware := vaktaware.NewService(pool, vaktaware.SMTPConfig{})

	// Regel vorhanden, aber noch nie gewirkt.
	before, err := aware.GetORP3Status(ctx, orgID)
	require.NoError(t, err)
	require.False(t, orp3Fulfilled(t, before, "ORP.3.A3"),
		"A3 darf ohne eine einzige Einschreibung nicht erfüllt sein")
	require.True(t, orp3Fulfilled(t, before, "ORP.3.A2"),
		"A2 (Schulungsplan) bleibt an der Existenz der Regel — unverändert")

	// Echter Eintritt.
	hr := newWiredHRService(pool)
	_, err = hr.CreateEmployee(ctx, vakthr.Actor{OrgID: orgID, UserID: uuid.New().String()},
		vakthr.CreateEmployeeInput{FirstName: "Ola", LastName: "Ny", Email: "ola.ny@example.com"})
	require.NoError(t, err)

	after, err := aware.GetORP3Status(ctx, orgID)
	require.NoError(t, err)
	require.True(t, orp3Fulfilled(t, after, "ORP.3.A3"),
		"nach einer echten Einschreibung muss A3 erfüllt sein")
}

func orp3Fulfilled(t *testing.T, c *vaktaware.BSIOrp3Compliance, id string) bool {
	t.Helper()
	for _, r := range c.Requirements {
		if r.ID == id {
			return r.Fulfilled
		}
	}
	t.Fatalf("Anforderung %s nicht im Bericht — Nenner falsch", id)
	return false
}
