//go:build integration

// Copyright (c) 2026 NorvikOps. All rights reserved.
// SPDX-License-Identifier: Elastic-2.0

package integration_test

import (
	"context"
	"encoding/json"
	"net/http"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Eine Aufgabenliste je Kontrolle (Migration 273). Früher gab es auf der
// Kontrollseite zwei Datenmodelle: die Checkliste ck_control_tasks und die
// zugewiesenen Aufgaben ck_tasks. Migration 273 übernimmt die Checkliste nach
// ck_tasks. Geprüft wird hier gegen eine echte Datenbank:
//   - die Übernahme ist verlustfrei (Zählung vorher = nachher, Felder 1:1),
//   - ein zweiter Lauf übernimmt nichts doppelt,
//   - die Down-Migration entfernt genau die übernommenen Zeilen und schreibt
//     den Erledigt-Stand zurück,
//   - die Liste über die echte Route: Reihenfolge, Anlegen ohne Person/Frist,
//     Abhaken, Leser 403 beim Schreiben, fremde Org 404, alte Route weg.

func readMigration(t *testing.T, name string) string {
	t.Helper()
	b, err := os.ReadFile(filepath.Join(migrationsDir(t), name))
	require.NoError(t, err)
	return string(b)
}

type migratedTask struct {
	Title     string
	Status    string
	OrgID     string
	EntityID  string
	CreatedAt time.Time
	UpdatedAt time.Time
}

func migratedTasks(t *testing.T, pool *pgxpool.Pool) map[string]migratedTask {
	t.Helper()
	rows, err := pool.Query(context.Background(), `
		SELECT source_control_task_id::text, title, status, org_id::text, entity_id::text,
		       created_at, updated_at
		FROM ck_tasks WHERE source_control_task_id IS NOT NULL AND entity_type = 'control'`)
	require.NoError(t, err)
	defer rows.Close()
	out := map[string]migratedTask{}
	for rows.Next() {
		var src string
		var m migratedTask
		require.NoError(t, rows.Scan(&src, &m.Title, &m.Status, &m.OrgID, &m.EntityID, &m.CreatedAt, &m.UpdatedAt))
		out[src] = m
	}
	require.NoError(t, rows.Err())
	return out
}

func countRows(t *testing.T, pool *pgxpool.Pool, sql string) int {
	t.Helper()
	var n int
	require.NoError(t, pool.QueryRow(context.Background(), sql).Scan(&n))
	return n
}

func TestMigration273TakesOverControlChecklist(t *testing.T) {
	pool, orgID, cleanup := bootPostgresWithOrg(t)
	defer cleanup()
	ctx := context.Background()

	var otherOrg string
	require.NoError(t, pool.QueryRow(ctx,
		`INSERT INTO organizations (name, slug) VALUES ('Fremd', 'fremd-273') RETURNING id::text`).Scan(&otherOrg))
	fw := seedFramework(ctx, t, pool, orgID, "ISO 27001")
	ctl := seedControl(ctx, t, pool, orgID, fw, "A.5.1", "Org", "")
	fwOther := seedFramework(ctx, t, pool, otherOrg, "ISO 27001")
	ctlOther := seedControl(ctx, t, pool, otherOrg, fwOther, "A.5.1", "Org", "")

	// Bestand, wie ihn die alte Checkliste geschrieben hat: zwei offene, ein
	// erledigter Schritt, deutlich getrennte Zeitpunkte, dazu eine fremde Org.
	base := time.Date(2026, 3, 1, 9, 0, 0, 0, time.UTC)
	type seed struct {
		org, control, text string
		completed          bool
		created            time.Time
	}
	seeds := []seed{
		{orgID, ctl, "Richtlinie entwerfen", true, base},
		{orgID, ctl, "Freigabe einholen", false, base.Add(time.Hour)},
		{orgID, ctl, "S", false, base.Add(2 * time.Hour)}, // ein Zeichen: alte Checkliste erlaubte das
		{otherOrg, ctlOther, "Fremder Schritt", false, base.Add(3 * time.Hour)},
	}
	ids := make([]string, len(seeds))
	for i, s := range seeds {
		require.NoError(t, pool.QueryRow(ctx, `
			INSERT INTO ck_control_tasks (control_id, org_id, text, completed, created_at, updated_at)
			VALUES ($1::uuid, $2::uuid, $3, $4, $5, $6) RETURNING id::text`,
			s.control, s.org, s.text, s.completed, s.created, s.created.Add(30*time.Minute)).Scan(&ids[i]))
	}

	up := readMigration(t, "273_control_tasks_into_ck_tasks.up.sql")
	down := readMigration(t, "273_control_tasks_into_ck_tasks.down.sql")

	// Die Migration lief beim Booten auf leerer Tabelle; jetzt mit Bestand.
	before := countRows(t, pool, `SELECT COUNT(*) FROM ck_control_tasks`) // orgid-lint: global — Testprüfung über die ganze Test-DB
	_, err := pool.Exec(ctx, up)
	require.NoError(t, err)
	got := migratedTasks(t, pool)
	require.Len(t, got, before, "Zählung vorher (ck_control_tasks) = nachher (übernommene ck_tasks)")

	for i, s := range seeds {
		m, ok := got[ids[i]]
		require.True(t, ok, "Zeile %q fehlt", s.text)
		assert.Equal(t, s.text, m.Title)
		assert.Equal(t, map[bool]string{true: "done", false: "open"}[s.completed], m.Status)
		assert.Equal(t, s.org, m.OrgID)
		assert.Equal(t, s.control, m.EntityID)
		assert.True(t, s.created.Equal(m.CreatedAt), "created_at %q", s.text)
		assert.True(t, s.created.Add(30*time.Minute).Equal(m.UpdatedAt), "updated_at %q", s.text)
	}

	// Idempotent: ein zweiter Lauf übernimmt nichts doppelt.
	_, err = pool.Exec(ctx, up)
	require.NoError(t, err)
	assert.Len(t, migratedTasks(t, pool), before)

	// Eine von Hand angelegte Aufgabe (keine Herkunft) überlebt die Down-Migration.
	_, err = pool.Exec(ctx, `
		INSERT INTO ck_tasks (org_id, entity_type, entity_id, title) VALUES ($1::uuid, 'control', $2::uuid, 'Neu nach Upgrade')`,
		orgID, ctl)
	require.NoError(t, err)
	// Nach dem Upgrade abgehakt …
	_, err = pool.Exec(ctx, `UPDATE ck_tasks SET status = 'done' WHERE source_control_task_id = $1::uuid`, ids[1]) // orgid-lint: global — Testprüfung über die ganze Test-DB
	require.NoError(t, err)

	_, err = pool.Exec(ctx, down)
	require.NoError(t, err)
	assert.Equal(t, 0, countRows(t, pool,
		`SELECT COUNT(*) FROM information_schema.columns WHERE table_name = 'ck_tasks' AND column_name = 'source_control_task_id'`)) // orgid-lint: global — Testprüfung über die ganze Test-DB
	assert.Equal(t, 1, countRows(t, pool, `SELECT COUNT(*) FROM ck_tasks`), "nur die neu angelegte Aufgabe bleibt")               // orgid-lint: global — Testprüfung über die ganze Test-DB
	assert.Equal(t, before, countRows(t, pool, `SELECT COUNT(*) FROM ck_control_tasks`), "alte Tabelle unangetastet vollständig") // orgid-lint: global — Testprüfung über die ganze Test-DB
	var completed bool
	require.NoError(t, pool.QueryRow(ctx, `SELECT completed FROM ck_control_tasks WHERE id = $1::uuid`, ids[1]).Scan(&completed)) // orgid-lint: global — Testprüfung über die ganze Test-DB
	assert.True(t, completed, "… der Haken überlebt das Downgrade")

	// Und wieder hoch: dieselbe Zählung.
	_, err = pool.Exec(ctx, up)
	require.NoError(t, err)
	assert.Len(t, migratedTasks(t, pool), before)
}

type apiTask struct {
	ID            string  `json:"id"`
	EntityID      string  `json:"entity_id"`
	Title         string  `json:"title"`
	Status        string  `json:"status"`
	AssigneeEmail string  `json:"assignee_email"`
	DueDate       *string `json:"due_date"`
}

func TestControlTaskListOverHTTP(t *testing.T) {
	pool, orgID, cleanup := bootPostgresWithOrg(t)
	defer cleanup()
	ctx := context.Background()

	var otherOrg string
	require.NoError(t, pool.QueryRow(ctx,
		`INSERT INTO organizations (name, slug) VALUES ('Fremd', 'fremd-http') RETURNING id::text`).Scan(&otherOrg))
	fw := seedFramework(ctx, t, pool, orgID, "ISO 27001")
	ctl := seedControl(ctx, t, pool, orgID, fw, "A.5.1", "Org", "")
	fwOther := seedFramework(ctx, t, pool, otherOrg, "ISO 27001")
	ctlOther := seedControl(ctx, t, pool, otherOrg, fwOther, "A.5.1", "Org", "")

	adminID := seedUser(t, pool, "aufgaben-admin@example.org")
	viewerID := seedUser(t, pool, "aufgaben-viewer@example.org")
	admin := complyHTTP(t, pool, orgID, adminID, []string{"Admin"})
	viewer := complyHTTP(t, pool, orgID, viewerID, []string{"Viewer"})
	foreign := complyHTTP(t, pool, otherOrg, adminID, []string{"Admin"})

	base := "/vaktcomply/controls/" + ctl + "/collab-tasks"

	// Anlegen nur mit Titel (wie die frühere Checkliste) …
	rec := doJSON(t, admin, http.MethodPost, base, `{"title":"Schritt eins"}`)
	require.Equal(t, http.StatusCreated, rec.Code, rec.Body.String())
	var first apiTask
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &first))
	assert.Equal(t, "open", first.Status)
	assert.Empty(t, first.AssigneeEmail)
	assert.Nil(t, first.DueDate)

	// … und mit Person und Frist.
	rec = doJSON(t, admin, http.MethodPost, base,
		`{"title":"Schritt zwei","assignee_email":"it@example.org","due_date":"2026-12-31"}`)
	require.Equal(t, http.StatusCreated, rec.Code, rec.Body.String())
	var second apiTask
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &second))
	assert.Equal(t, "it@example.org", second.AssigneeEmail)
	require.NotNil(t, second.DueDate)

	// Abhaken.
	rec = doJSON(t, admin, http.MethodPatch, "/vaktcomply/collab-tasks/"+first.ID, `{"status":"done"}`)
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())

	// Leser sieht die Liste, in Anlege-Reihenfolge.
	rec = doJSON(t, viewer, http.MethodGet, base, "")
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	var list []apiTask
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &list))
	require.Len(t, list, 2)
	assert.Equal(t, first.ID, list[0].ID, "älteste zuerst — die Checkliste behält ihre Reihenfolge")
	assert.Equal(t, "done", list[0].Status)

	// Leser darf nicht schreiben — und die Datenbank bleibt unverändert.
	for _, c := range []struct{ method, path, body string }{
		{http.MethodPost, base, `{"title":"Leser"}`},
		{http.MethodPatch, "/vaktcomply/collab-tasks/" + second.ID, `{"status":"done"}`},
		{http.MethodDelete, "/vaktcomply/collab-tasks/" + second.ID, ""},
	} {
		rec = doJSON(t, viewer, c.method, c.path, c.body)
		assert.Equal(t, http.StatusForbidden, rec.Code, "%s %s", c.method, c.path)
	}
	assert.Equal(t, 2, countRows(t, pool, `SELECT COUNT(*) FROM ck_tasks`)) // orgid-lint: global — Testprüfung über die ganze Test-DB
	var st string
	require.NoError(t, pool.QueryRow(ctx, `SELECT status FROM ck_tasks WHERE id = $1::uuid`, second.ID).Scan(&st)) // orgid-lint: global — Testprüfung über die ganze Test-DB
	assert.Equal(t, "open", st)

	// Fremde Org: Kontrolle und Aufgaben existieren für sie nicht (404, nicht 403).
	rec = doJSON(t, foreign, http.MethodGet, base, "")
	assert.Equal(t, http.StatusNotFound, rec.Code, rec.Body.String())
	rec = doJSON(t, foreign, http.MethodPost, base, `{"title":"Fremd"}`)
	assert.Equal(t, http.StatusNotFound, rec.Code, rec.Body.String())
	rec = doJSON(t, foreign, http.MethodPatch, "/vaktcomply/collab-tasks/"+second.ID, `{"status":"done"}`)
	assert.Equal(t, http.StatusNotFound, rec.Code, rec.Body.String())
	rec = doJSON(t, foreign, http.MethodDelete, "/vaktcomply/collab-tasks/"+second.ID, "")
	assert.Equal(t, http.StatusNotFound, rec.Code, rec.Body.String())
	// Umgekehrt: eigene Org auf die fremde Kontrolle.
	rec = doJSON(t, admin, http.MethodGet, "/vaktcomply/controls/"+ctlOther+"/collab-tasks", "")
	assert.Equal(t, http.StatusNotFound, rec.Code, rec.Body.String())
	assert.Equal(t, 2, countRows(t, pool, `SELECT COUNT(*) FROM ck_tasks`), "fremde Org hat nichts geschrieben") // orgid-lint: global — Testprüfung über die ganze Test-DB

	// Die frühere Checklisten-Route gibt es nicht mehr.
	rec = doJSON(t, admin, http.MethodGet, "/vaktcomply/controls/"+ctl+"/tasks", "")
	assert.Contains(t, []int{http.StatusNotFound, http.StatusMethodNotAllowed}, rec.Code)

	// Löschen durch Admin.
	rec = doJSON(t, admin, http.MethodDelete, "/vaktcomply/collab-tasks/"+second.ID, "")
	assert.Equal(t, http.StatusNoContent, rec.Code, rec.Body.String())
	assert.Equal(t, 1, countRows(t, pool, `SELECT COUNT(*) FROM ck_tasks`)) // orgid-lint: global — Testprüfung über die ganze Test-DB
}
