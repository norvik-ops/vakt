// Copyright (c) 2026 NorvikOps. All rights reserved.
// SPDX-License-Identifier: Elastic-2.0

// ESK-12 — Regressionstest fuer den Schreibpfad der vier BSI-200-4-Felder von
// ck_bcp_plans.
//
// DER DEFEKT, DEN DIESE DATEI FAENGT
// ---------------------------------
// PR #75 hat rto_hours/rpo_hours/schutzbedarfsklasse/last_tested_at in die vier
// ck_bcp_plans-Queries aufgenommen — gelesen wurden sie damit korrekt. Geschrieben
// hat sie niemand: das INSERT nannte 6 Spalten, das UPDATE 5, die Input-Structs
// fuehrten die Felder nicht. Jeder BCP-Plan jeder Organisation lieferte dauerhaft
// die Migrations-Defaults 72/24/2 und last_tested_at=null — Werte, die wie
// kuratierte BSI-200-4-Angaben aussehen und von echten nicht zu unterscheiden
// sind. Gemessen vor dem Fix gegen eine echte Postgres:
//
//	{"rto_hours":72,"rpo_hours":24,"schutzbedarfsklasse":2,"last_tested_at":null}
//
// WARUM DIESE TESTS GEGEN EINE ECHTE DB LAUFEN
// --------------------------------------------
// Der Defekt lebte in der Spaltenliste einer Query. Ein Test mit einem Fake-
// Querier haette ihn nicht gesehen: er haette geprueft, dass das Repository die
// Params fuellt, waehrend die Query sie ignoriert. Genau diese Naht ist die
// Fundstelle. VAKT_DB_URL setzt CI fuer `go test ./...` (ci.yml); lokal ohne
// gesetzte Variable ueberspringen die Tests, wie bei G5/rawsqlcov auch.
package bcm

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	db "github.com/matharnica/vakt/internal/db"
)

func intPtr(v int) *int { return &v }

// bcpTestOrg legt eine Wegwerf-Organisation an und gibt ihre ID zurueck. Der
// Slug traegt den Testnamen, damit parallele Laeufe sich nicht ueberschreiben;
// ON DELETE CASCADE (Migration 213) raeumt die Plaene mit ab.
func bcpTestOrg(ctx context.Context, t *testing.T, pool *pgxpool.Pool) string {
	t.Helper()
	slug := fmt.Sprintf("esk12-%s", t.Name())
	var id string
	require.NoError(t, pool.QueryRow(ctx,
		`INSERT INTO organizations (name, slug) VALUES ($1, $1) RETURNING id::text`, slug,
	).Scan(&id))
	t.Cleanup(func() {
		_, _ = pool.Exec(context.Background(), `DELETE FROM organizations WHERE id = $1::uuid`, id)
	})
	return id
}

func bcpTestPool(t *testing.T) *pgxpool.Pool {
	t.Helper()
	url := os.Getenv("VAKT_DB_URL")
	if url == "" {
		t.Skip("VAKT_DB_URL not set — dieser Test braucht eine migrierte Postgres (CI setzt sie)")
	}
	pool, err := pgxpool.New(context.Background(), url)
	require.NoError(t, err)
	t.Cleanup(pool.Close)
	return pool
}

// TestBCPPlanWritePath_CreateRoundTrip: gesetzte Werte kommen als DIESE Werte
// zurueck — aus dem CREATE-Ergebnis UND aus einem frischen GET, denn ein INSERT
// ohne die Spalten haette trotzdem ein plausibles RETURNING geliefert.
func TestBCPPlanWritePath_CreateRoundTrip(t *testing.T) {
	pool := bcpTestPool(t)
	ctx := context.Background()
	orgID := bcpTestOrg(ctx, t, pool)
	svc := NewService(pool)

	created, err := svc.CreateBCPPlan(ctx, orgID, CreateBCPPlanInput{
		Title:               "RZ-Ausfall",
		RTOHours:            intPtr(4),
		RPOHours:            intPtr(1),
		Schutzbedarfsklasse: intPtr(3),
	})
	require.NoError(t, err)

	for _, tc := range []struct {
		label string
		plan  BCPPlan
	}{{"create-Antwort", created}, {"frischer GET", mustGet(ctx, t, svc, orgID, created.ID)}} {
		require.NotNil(t, tc.plan.RTOHours, "%s: rto_hours ist null, obwohl 4 gesetzt wurde", tc.label)
		require.NotNil(t, tc.plan.RPOHours, "%s: rpo_hours ist null, obwohl 1 gesetzt wurde", tc.label)
		require.NotNil(t, tc.plan.Schutzbedarfsklasse, "%s: schutzbedarfsklasse ist null, obwohl 3 gesetzt wurde", tc.label)
		// Die drei Defaults aus Migration 216 stehen hier ausdruecklich als
		// Gegenprobe: 72/24/2 waere der Wert, den der Defekt geliefert hat.
		assert.Equal(t, 4, *tc.plan.RTOHours, "%s: rto_hours (72 = Migrations-Default, also kein Schreibpfad)", tc.label)
		assert.Equal(t, 1, *tc.plan.RPOHours, "%s: rpo_hours (24 = Migrations-Default)", tc.label)
		assert.Equal(t, 3, *tc.plan.Schutzbedarfsklasse, "%s: schutzbedarfsklasse (2 = Migrations-Default)", tc.label)
	}
}

// TestBCPPlanWritePath_UpdateRoundTrip: auch das UPDATE traegt die Felder.
func TestBCPPlanWritePath_UpdateRoundTrip(t *testing.T) {
	pool := bcpTestPool(t)
	ctx := context.Background()
	orgID := bcpTestOrg(ctx, t, pool)
	svc := NewService(pool)

	created, err := svc.CreateBCPPlan(ctx, orgID, CreateBCPPlanInput{Title: "Erst leer"})
	require.NoError(t, err)
	require.Nil(t, created.RTOHours)

	updated, err := svc.UpdateBCPPlan(ctx, orgID, created.ID, UpdateBCPPlanInput{
		Title:               "Erst leer",
		Status:              "active",
		RTOHours:            SetInt(48),
		RPOHours:            SetInt(12),
		Schutzbedarfsklasse: SetInt(1),
	})
	require.NoError(t, err)
	require.NotNil(t, updated.RTOHours)
	assert.Equal(t, 48, *updated.RTOHours)

	got := mustGet(ctx, t, svc, orgID, created.ID)
	require.NotNil(t, got.RTOHours)
	require.NotNil(t, got.RPOHours)
	require.NotNil(t, got.Schutzbedarfsklasse)
	assert.Equal(t, 48, *got.RTOHours)
	assert.Equal(t, 12, *got.RPOHours)
	assert.Equal(t, 1, *got.Schutzbedarfsklasse)
}

// TestBCPPlanPatch_OmittedFieldsSurvive ist der Rundlauf, der in der ersten
// Fassung dieses Commits gefehlt hat — und genau der, der ihren Datenverlust
// gefangen haette (REV-ESK12 B1).
//
// GEMESSEN VOR DER NACHBESSERUNG, gegen dieselbe Postgres:
//
//	PATCH {"title":…,"status":"active","rto_hours":8,"rpo_hours":2,"schutzbedarfsklasse":3}
//	  -> 200  rto_hours=8, rpo_hours=2, schutzbedarfsklasse=3
//	PATCH {"title":…,"status":"active"}
//	  -> 200  rto_hours=null, rpo_hours=null, schutzbedarfsklasse=null
//
// Der zweite Aufruf ist der vollstaendige Body, den openapi.yaml vor diesem
// Commit auswies. Wer den Status eines Plans umstellte, loeschte seine
// BSI-200-4-Angaben — mit 200 und ohne Meldung.
//
// setzen -> PATCH OHNE die Felder -> lesen -> Werte noch da.
func TestBCPPlanPatch_OmittedFieldsSurvive(t *testing.T) {
	pool := bcpTestPool(t)
	ctx := context.Background()
	orgID := bcpTestOrg(ctx, t, pool)
	svc := NewService(pool)

	created, err := svc.CreateBCPPlan(ctx, orgID, CreateBCPPlanInput{
		Title:               "Notfallhandbuch RZ",
		Scope:               "Rechenzentrum Nord",
		Version:             "2.1",
		Owner:               "Leitung IT",
		RTOHours:            intPtr(8),
		RPOHours:            intPtr(2),
		Schutzbedarfsklasse: intPtr(3),
	})
	require.NoError(t, err)

	// Der Body, den ein bestehender Integrator schickt, um NUR den Status zu
	// setzen: title und status (beide Pflicht), sonst nichts.
	updated, err := svc.UpdateBCPPlan(ctx, orgID, created.ID, UpdateBCPPlanInput{
		Title:  "Notfallhandbuch RZ",
		Status: "active",
	})
	require.NoError(t, err)

	for _, tc := range []struct {
		label string
		plan  BCPPlan
	}{{"PATCH-Antwort", updated}, {"frischer GET", mustGet(ctx, t, svc, orgID, created.ID)}} {
		require.NotNil(t, tc.plan.RTOHours, "%s: rto_hours wurde vom PATCH still geloescht", tc.label)
		require.NotNil(t, tc.plan.RPOHours, "%s: rpo_hours wurde vom PATCH still geloescht", tc.label)
		require.NotNil(t, tc.plan.Schutzbedarfsklasse, "%s: schutzbedarfsklasse wurde vom PATCH still geloescht", tc.label)
		assert.Equal(t, 8, *tc.plan.RTOHours, "%s", tc.label)
		assert.Equal(t, 2, *tc.plan.RPOHours, "%s", tc.label)
		assert.Equal(t, 3, *tc.plan.Schutzbedarfsklasse, "%s", tc.label)
		// Dieselbe Klasse, nur ohne eigenen Befund: scope/version/owner gingen
		// bei einem PATCH ohne sie ebenfalls verloren (auf "").
		assert.Equal(t, "Rechenzentrum Nord", tc.plan.Scope, "%s: scope wurde still geleert", tc.label)
		assert.Equal(t, "2.1", tc.plan.Version, "%s: version wurde still geleert", tc.label)
		assert.Equal(t, "Leitung IT", tc.plan.Owner, "%s: owner wurde still geleert", tc.label)
		// Was das PATCH mitgeschickt hat, gilt.
		assert.Equal(t, "active", tc.plan.Status, "%s", tc.label)
	}
}

// TestBCPPlanPatch_ExplicitNullClears ist die Gegenprobe: mergende Semantik darf
// den Weg zum Loeschen nicht verschliessen. `null` heisst loeschen, und der
// Unterschied zu "weggelassen" muss durch die volle Naht (JSON -> Struct -> SQL)
// ueberleben — deshalb geht dieser Test durch json.Unmarshal und nicht durch ein
// Struct-Literal: ein `*int` haette beides als nil dekodiert.
func TestBCPPlanPatch_ExplicitNullClears(t *testing.T) {
	pool := bcpTestPool(t)
	ctx := context.Background()
	orgID := bcpTestOrg(ctx, t, pool)
	svc := NewService(pool)

	created, err := svc.CreateBCPPlan(ctx, orgID, CreateBCPPlanInput{
		Title:               "Zu loeschen",
		RTOHours:            intPtr(8),
		RPOHours:            intPtr(2),
		Schutzbedarfsklasse: intPtr(3),
	})
	require.NoError(t, err)

	var in UpdateBCPPlanInput
	require.NoError(t, json.Unmarshal([]byte(
		`{"title":"Zu loeschen","status":"draft","rto_hours":null,"schutzbedarfsklasse":null}`), &in))
	require.True(t, in.RTOHours.Set, "rto_hours stand im Body")
	require.Nil(t, in.RTOHours.Value, "…und zwar als null")
	require.False(t, in.RPOHours.Set, "rpo_hours stand NICHT im Body")

	updated, err := svc.UpdateBCPPlan(ctx, orgID, created.ID, in)
	require.NoError(t, err)

	got := mustGet(ctx, t, svc, orgID, created.ID)
	for _, tc := range []struct {
		label string
		plan  BCPPlan
	}{{"PATCH-Antwort", updated}, {"frischer GET", got}} {
		assert.Nil(t, tc.plan.RTOHours, "%s: explizites null muss loeschen", tc.label)
		assert.Nil(t, tc.plan.Schutzbedarfsklasse, "%s: explizites null muss loeschen", tc.label)
		require.NotNil(t, tc.plan.RPOHours, "%s: rpo_hours war nicht im Body und muss bleiben", tc.label)
		assert.Equal(t, 2, *tc.plan.RPOHours, "%s", tc.label)
	}
}

// TestBCPPlanPatch_MergedStateIsValidated: die Invariante rpo <= rto gilt fuer
// den ZIELZUSTAND, nicht fuer den Requestbody. Bestand rto=4; ein PATCH, das nur
// rpo=8 schickt, sieht fuer sich genommen unauffaellig aus.
func TestBCPPlanPatch_MergedStateIsValidated(t *testing.T) {
	pool := bcpTestPool(t)
	ctx := context.Background()
	orgID := bcpTestOrg(ctx, t, pool)
	svc := NewService(pool)

	created, err := svc.CreateBCPPlan(ctx, orgID, CreateBCPPlanInput{
		Title: "Invariante", RTOHours: intPtr(4), RPOHours: intPtr(1),
	})
	require.NoError(t, err)

	_, err = svc.UpdateBCPPlan(ctx, orgID, created.ID, UpdateBCPPlanInput{
		Title: "Invariante", Status: "draft", RPOHours: SetInt(8),
	})
	require.Error(t, err, "rpo=8 gegen den Bestand rto=4 muss abgelehnt werden")
	assert.ErrorIs(t, err, ErrRPOExceedsRTO)

	// Und die Ablehnung darf nichts halb geschrieben haben.
	got := mustGet(ctx, t, svc, orgID, created.ID)
	require.NotNil(t, got.RPOHours)
	assert.Equal(t, 1, *got.RPOHours, "der abgelehnte PATCH darf den Bestand nicht veraendern")
}

// TestBCPPlanWritePath_UnsetStaysNull ist die Kern-Assertion des Befundes: ein
// Plan, fuer den niemand RTO/RPO/Schutzbedarf festgelegt hat, darf keine Zahl
// behaupten. Dreht man den Fix zurueck (NOT NULL DEFAULT 72/24/2 oder int statt
// *int), liefert diese Stelle wieder 72/24/2 und der Test wird rot.
func TestBCPPlanWritePath_UnsetStaysNull(t *testing.T) {
	pool := bcpTestPool(t)
	ctx := context.Background()
	orgID := bcpTestOrg(ctx, t, pool)
	svc := NewService(pool)

	created, err := svc.CreateBCPPlan(ctx, orgID, CreateBCPPlanInput{Title: "Nichts festgelegt"})
	require.NoError(t, err)

	got := mustGet(ctx, t, svc, orgID, created.ID)
	assert.Nil(t, got.RTOHours, "rto_hours muss null sein, nicht der Migrations-Default 72")
	assert.Nil(t, got.RPOHours, "rpo_hours muss null sein, nicht der Migrations-Default 24")
	assert.Nil(t, got.Schutzbedarfsklasse, "schutzbedarfsklasse muss null sein, nicht der Migrations-Default 2")
	assert.Nil(t, got.LastTestedAt, "ohne Testeintrag darf kein Testdatum behauptet werden")

	// Dieselbe Aussage ueber die Liste — sie hat eine eigene Query, und der
	// Ausgangsdefekt lag in genau so einer Spaltenliste.
	plans, err := svc.ListBCPPlans(ctx, orgID)
	require.NoError(t, err)
	require.Len(t, plans, 1)
	assert.Nil(t, plans[0].RTOHours)
	assert.Nil(t, plans[0].Schutzbedarfsklasse)
}

// TestBCPPlanLastTestedAt_SetByRealTest deckt (d): nach einem echt angelegten
// Test traegt der Plan das Datum dieses Tests.
func TestBCPPlanLastTestedAt_SetByRealTest(t *testing.T) {
	pool := bcpTestPool(t)
	ctx := context.Background()
	orgID := bcpTestOrg(ctx, t, pool)
	svc := NewService(pool)

	plan, err := svc.CreateBCPPlan(ctx, orgID, CreateBCPPlanInput{Title: "Mit Test"})
	require.NoError(t, err)
	require.Nil(t, mustGet(ctx, t, svc, orgID, plan.ID).LastTestedAt)

	_, err = svc.AddBCPTest(ctx, orgID, plan.ID, CreateBCPTestInput{
		TestDate: "2026-05-04", TestType: "tabletop", Outcome: "passed",
	})
	require.NoError(t, err)

	got := mustGet(ctx, t, svc, orgID, plan.ID)
	require.NotNil(t, got.LastTestedAt, "last_tested_at ist nach einem echten Test immer noch null")
	assert.Equal(t, "2026-05-04", *got.LastTestedAt)

	// Ein spaeterer Test schiebt das Datum vor.
	_, err = svc.AddBCPTest(ctx, orgID, plan.ID, CreateBCPTestInput{
		TestDate: "2026-06-30", TestType: "fulltest", Outcome: "failed",
	})
	require.NoError(t, err)
	got = mustGet(ctx, t, svc, orgID, plan.ID)
	require.NotNil(t, got.LastTestedAt)
	assert.Equal(t, "2026-06-30", *got.LastTestedAt,
		"ein fehlgeschlagener Test IST ein durchgefuehrter Test und zaehlt fuer last_tested_at")

	// Ein NACHGETRAGENER aelterer Test darf das Datum nicht zurueckdrehen — die
	// Ableitung ist ein MAX, keine Zuweisung des zuletzt Eingegebenen.
	_, err = svc.AddBCPTest(ctx, orgID, plan.ID, CreateBCPTestInput{
		TestDate: "2026-01-02", TestType: "walkthrough", Outcome: "partial",
	})
	require.NoError(t, err)
	got = mustGet(ctx, t, svc, orgID, plan.ID)
	require.NotNil(t, got.LastTestedAt)
	assert.Equal(t, "2026-06-30", *got.LastTestedAt)
}

// TestBCPPlanLastTestedAt_IsolatedPerOrg: die Ableitung darf nicht ueber
// Organisationsgrenzen greifen.
//
// REV-ESK12 B5: Der Test bestand vorher nur aus dem Service-Aufruf unten — und
// der kommt gar nicht bis zum Refresh, weil Service.AddBCPTest vorher
// GetBCPPlan prueft (ein BESTEHENDER Check, aelter als dieser Diff). Er sicherte
// also den Vorabcheck, nicht die neue Ableitung, obwohl sein Kommentar das
// Gegenteil behauptete.
//
// Traegt die Isolation der Ableitung selbst, wird das WO wichtig, und die erste
// Nachbesserung hier hatte es falsch: nicht das `org_id = $2` im aeusseren WHERE
// schuetzt, sondern das `t.org_id = ck_bcp_plans.org_id` in der Unterabfrage.
// Der aeussere Filter entscheidet nur, WELCHE Zeile neu berechnet wird — und ein
// Neuberechnen ist idempotent, richtet also auch aus einer fremden Organisation
// heraus nichts an. Gemessen: mit entferntem aeusserem org_id-Filter bleibt
// dieser Test gruen. Was er PRUEFEN muss, ist die Unterabfrage, und dafuer
// braucht es eine Testzeile mit fremder org_id auf demselben plan_id. Die ist
// anlegbar, weil ck_bcp_tests keinen FK auf org_id und keinen
// zusammengesetzten auf (plan_id, org_id) hat — nachgesehen im Schema.
func TestBCPPlanLastTestedAt_IsolatedPerOrg(t *testing.T) {
	pool := bcpTestPool(t)
	ctx := context.Background()
	orgA := bcpTestOrg(ctx, t, pool)
	svc := NewService(pool)

	plan, err := svc.CreateBCPPlan(ctx, orgA, CreateBCPPlanInput{Title: "Org A"})
	require.NoError(t, err)
	_, err = svc.AddBCPTest(ctx, orgA, plan.ID, CreateBCPTestInput{
		TestDate: "2026-03-03", TestType: "tabletop", Outcome: "passed",
	})
	require.NoError(t, err)

	var orgB string
	require.NoError(t, pool.QueryRow(ctx,
		`INSERT INTO organizations (name, slug) VALUES ($1,$1) RETURNING id::text`,
		"esk12-"+t.Name()+"-b").Scan(&orgB))
	t.Cleanup(func() { _, _ = pool.Exec(context.Background(), `DELETE FROM organizations WHERE id = $1::uuid`, orgB) })

	_, err = svc.AddBCPTest(ctx, orgB, plan.ID, CreateBCPTestInput{
		TestDate: "2026-04-04", TestType: "tabletop", Outcome: "passed",
	})
	assert.Error(t, err, "ein Test gegen einen fremden Plan darf nicht durchgehen")

	got := mustGet(ctx, t, svc, orgA, plan.ID)
	require.NotNil(t, got.LastTestedAt)
	assert.Equal(t, "2026-03-03", *got.LastTestedAt)

	// Eine Testzeile mit FREMDER org_id auf demselben plan_id — der Zustand,
	// den kein Endpunkt herstellt, den das Schema aber zulaesst. Ihr Datum liegt
	// spaeter als das echte; zaehlte die Ableitung sie mit, schoebe sie vor.
	_, err = pool.Exec(ctx,
		`INSERT INTO ck_bcp_tests (org_id, plan_id, test_date, test_type, outcome)
		 VALUES ($1::uuid, $2::uuid, '2026-12-31', 'tabletop', 'passed')`, orgB, plan.ID)
	require.NoError(t, err)

	// Der Refresh DIREKT, am Vorabcheck des Service vorbei.
	require.NoError(t, db.New(pool).RefreshCKBCPPlanLastTested(ctx,
		db.RefreshCKBCPPlanLastTestedParams{ID: plan.ID, OrgID: orgA}))

	got = mustGet(ctx, t, svc, orgA, plan.ID)
	require.NotNil(t, got.LastTestedAt)
	assert.Equal(t, "2026-03-03", *got.LastTestedAt,
		"die Ableitung hat einen Testeintrag einer FREMDEN Organisation mitgezaehlt")

	// Und aus der fremden Organisation heraus angestossen ebenfalls nicht.
	require.NoError(t, db.New(pool).RefreshCKBCPPlanLastTested(ctx,
		db.RefreshCKBCPPlanLastTestedParams{ID: plan.ID, OrgID: orgB}))
	got = mustGet(ctx, t, svc, orgA, plan.ID)
	require.NotNil(t, got.LastTestedAt, "ein Refresh aus einer fremden Organisation hat das Datum geleert")
	assert.Equal(t, "2026-03-03", *got.LastTestedAt)
}

// TestBCPPlanValidation prueft die Grenzen gegen den BESTEHENDEN CHECK der
// Tabelle (schutzbedarfsklasse IN (1,2,3), Migration 216) und die
// Plausibilitaetsgrenzen fuer RTO/RPO. Ohne die Service-Pruefung wuerde eine 0
// erst der DB-CHECK fangen — als 500 statt als benannter Eingabefehler.
func TestBCPPlanValidation(t *testing.T) {
	for _, tc := range []struct {
		name    string
		in      CreateBCPPlanInput
		wantErr error
	}{
		{"schutzbedarfsklasse 0 (der alte Nullwert)", CreateBCPPlanInput{Title: "x", Schutzbedarfsklasse: intPtr(0)}, ErrSchutzbedarfsklasseInvalid},
		{"schutzbedarfsklasse 4", CreateBCPPlanInput{Title: "x", Schutzbedarfsklasse: intPtr(4)}, ErrSchutzbedarfsklasseInvalid},
		{"rto 0", CreateBCPPlanInput{Title: "x", RTOHours: intPtr(0)}, ErrRTOOutOfRange},
		{"rto negativ", CreateBCPPlanInput{Title: "x", RTOHours: intPtr(-5)}, ErrRTOOutOfRange},
		{"rto ueber einem Jahr", CreateBCPPlanInput{Title: "x", RTOHours: intPtr(BCPPlanMaxRTOHours + 1)}, ErrRTOOutOfRange},
		{"rpo 0", CreateBCPPlanInput{Title: "x", RPOHours: intPtr(0)}, ErrRPOOutOfRange},
		{"rpo groesser als rto", CreateBCPPlanInput{Title: "x", RTOHours: intPtr(4), RPOHours: intPtr(8)}, ErrRPOExceedsRTO},
	} {
		t.Run(tc.name, func(t *testing.T) {
			err := validateBCPPlanTargets(tc.in.RTOHours, tc.in.RPOHours, tc.in.Schutzbedarfsklasse)
			assert.ErrorIs(t, err, tc.wantErr)
		})
	}

	// Erlaubte Faelle, inklusive der Grenzen und des "nichts gesetzt"-Zustands.
	for _, tc := range []struct {
		name string
		in   CreateBCPPlanInput
	}{
		{"nichts gesetzt", CreateBCPPlanInput{Title: "x"}},
		{"nur rto", CreateBCPPlanInput{Title: "x", RTOHours: intPtr(72)}},
		{"rpo == rto", CreateBCPPlanInput{Title: "x", RTOHours: intPtr(4), RPOHours: intPtr(4)}},
		{"schutzbedarfsklasse 1/2/3 obere Grenze", CreateBCPPlanInput{Title: "x", Schutzbedarfsklasse: intPtr(3)}},
		{"rto genau ein Jahr", CreateBCPPlanInput{Title: "x", RTOHours: intPtr(BCPPlanMaxRTOHours)}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			assert.NoError(t, validateBCPPlanTargets(tc.in.RTOHours, tc.in.RPOHours, tc.in.Schutzbedarfsklasse))
		})
	}
}

func mustGet(ctx context.Context, t *testing.T, svc *Service, orgID, id string) BCPPlan {
	t.Helper()
	p, err := svc.GetBCPPlan(ctx, orgID, id)
	require.NoError(t, err)
	return p
}
