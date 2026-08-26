//go:build integration

// Copyright (c) 2026 NorvikOps. All rights reserved.
// SPDX-License-Identifier: Elastic-2.0

package integration_test

import (
	"context"
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/testcontainers/testcontainers-go/modules/postgres"

	"github.com/matharnica/vakt/internal/shared/dataexport"
	shareddb "github.com/matharnica/vakt/internal/shared/db"
)

// exportTypesFixture spins up a migrated PostgreSQL, seeds one org with a
// framework → control chain plus a dated and a binary column, and returns the
// pool together with the ids as the database itself reports them.
type exportTypesFixture struct {
	pool        *pgxpool.Pool
	orgID       string
	frameworkID string
	controlID   string
	dueDate     string
	entryHash   string
}

func setupExportTypes(t *testing.T) *exportTypesFixture {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	t.Cleanup(cancel)

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
	t.Cleanup(func() { _ = pgC.Terminate(context.Background()) })

	dsn, err := pgC.ConnectionString(ctx, "sslmode=disable")
	require.NoError(t, err)
	require.NoError(t, shareddb.RunMigrations(dsn, migrationsDir(t)))

	pool, err := pgxpool.New(ctx, dsn)
	require.NoError(t, err)
	t.Cleanup(pool.Close)

	f := &exportTypesFixture{pool: pool, dueDate: "2026-11-03"}

	require.NoError(t, pool.QueryRow(ctx,
		`INSERT INTO organizations (name, slug) VALUES ('Acme','acme') RETURNING id::text`).Scan(&f.orgID))

	require.NoError(t, pool.QueryRow(ctx,
		`INSERT INTO ck_frameworks (org_id, name) VALUES ($1::uuid,'ISO 27001') RETURNING id::text`,
		f.orgID).Scan(&f.frameworkID))

	// The control points at the framework — this is the relation the archive
	// has to keep resolvable.
	// status and soa_applicable are GENERATED ALWAYS columns — not insertable.
	require.NoError(t, pool.QueryRow(ctx, `
		INSERT INTO ck_controls (org_id, framework_id, control_id, title, domain, due_date)
		VALUES ($1::uuid, $2::uuid, 'A.5.1', 'Policies', 'Organizational', $3::date)
		RETURNING id::text`,
		f.orgID, f.frameworkID, f.dueDate).Scan(&f.controlID))

	// A bytea column (the audit hash chain) so the binary mapping is exercised.
	f.entryHash = `\x00ff41`
	_, err = pool.Exec(ctx, `
		INSERT INTO audit_log (org_id, action, resource_type, entry_hash)
		VALUES ($1::uuid, 'export.test', 'control', '\x00ff41'::bytea)`, f.orgID)
	require.NoError(t, err)

	return f
}

// decodeRows parses one archive file into a slice of generic rows.
func decodeRows(t *testing.T, files map[string]string, name string) []map[string]any {
	t.Helper()
	raw, ok := files[name]
	require.True(t, ok, "archive is missing %s", name)
	var rows []map[string]any
	require.NoError(t, json.Unmarshal([]byte(raw), &rows), "%s is not a JSON array of objects", name)
	return rows
}

// TestDataExport_ForeignKeysAreResolvable is the acceptance test for R1-20-08.
//
// It deliberately does NOT merely assert that the files parse as JSON — the
// broken version produced valid JSON too. It takes the foreign key out of the
// archive and holds it against the value the database reports for the same row,
// then joins the two archive files on it. If uuid columns regress to
// [238,190,84,…] the comparison fails and the join finds nothing.
func TestDataExport_ForeignKeysAreResolvable(t *testing.T) {
	if testing.Short() {
		t.Skip("integration: -short mode")
	}
	f := setupExportTypes(t)
	files := runExport(t, f.pool, f.orgID, "vaktcomply,vakthr,vaktaware,vaktprivacy")

	controls := decodeRows(t, files, "controls.json")
	require.Len(t, controls, 1)
	frameworks := decodeRows(t, files, "frameworks.json")
	require.Len(t, frameworks, 1)

	// 1. The primary key is the string the database reports, not a byte array.
	assert.Equal(t, f.controlID, controls[0]["id"],
		"controls.json id must be the canonical UUID string")
	assert.Equal(t, f.frameworkID, frameworks[0]["id"],
		"frameworks.json id must be the canonical UUID string")

	// 2. The foreign key matches the database value.
	assert.Equal(t, f.frameworkID, controls[0]["framework_id"],
		"controls.json framework_id must be the canonical UUID string")

	// 3. The relation is actually resolvable inside the archive: joining
	//    controls.framework_id to frameworks.id finds the framework by name.
	byID := map[string]map[string]any{}
	for _, fw := range frameworks {
		id, ok := fw["id"].(string)
		require.True(t, ok, "frameworks.json id is not a string but %T", fw["id"])
		byID[id] = fw
	}
	fkRaw := controls[0]["framework_id"]
	fk, ok := fkRaw.(string)
	require.True(t, ok, "controls.json framework_id is not a string but %T — the relation cannot be resolved", fkRaw)
	joined, found := byID[fk]
	require.True(t, found, "controls.framework_id %q does not resolve to any row in frameworks.json", fk)
	assert.Equal(t, "ISO 27001", joined["name"])

	// 4. org_id is exported as a string too (every table carries it).
	assert.Equal(t, f.orgID, controls[0]["org_id"])
}

// TestDataExport_ColumnTypeRendering pins the remaining conversions that a
// plain "is it valid JSON" check would wave through.
func TestDataExport_ColumnTypeRendering(t *testing.T) {
	if testing.Short() {
		t.Skip("integration: -short mode")
	}
	f := setupExportTypes(t)
	files := runExport(t, f.pool, f.orgID, "vaktcomply,vakthr,vaktaware,vaktprivacy")

	controls := decodeRows(t, files, "controls.json")
	require.Len(t, controls, 1)

	// date — a calendar day, not a midnight timestamp that can shift by timezone.
	assert.Equal(t, f.dueDate, controls[0]["due_date"], "date must render as YYYY-MM-DD")

	// timestamptz — RFC3339 in UTC (the project-wide convention).
	created, ok := controls[0]["created_at"].(string)
	require.True(t, ok, "created_at is not a string but %T", controls[0]["created_at"])
	assert.True(t, strings.HasSuffix(created, "Z"),
		"timestamps must be normalised to UTC, got %q", created)
	_, err := time.Parse(time.RFC3339Nano, created)
	assert.NoError(t, err, "created_at %q is not RFC3339", created)

	// bytea — a PostgreSQL hex literal, not Base64.
	auditRows := decodeRows(t, files, "audit_log.json")
	require.Len(t, auditRows, 1)
	assert.Equal(t, f.entryHash, auditRows[0]["entry_hash"],
		"bytea must render as a PostgreSQL hex literal")

	// bool / int stay native JSON types.
	assert.Equal(t, true, controls[0]["soa_applicable"])
	assert.Equal(t, float64(1), controls[0]["weight"])

	// NULLs stay null rather than becoming an empty byte array.
	assert.Nil(t, controls[0]["last_tested_at"])
}

// TestDataExport_MetaStatesVersions covers R1-20-10: an archive that does not
// say which release produced it is useless when restoring it later.
func TestDataExport_MetaStatesVersions(t *testing.T) {
	if testing.Short() {
		t.Skip("integration: -short mode")
	}
	f := setupExportTypes(t)
	files := runExport(t, f.pool, f.orgID, "vaktcomply")

	var meta map[string]string
	require.NoError(t, json.Unmarshal([]byte(files["meta.json"]), &meta))

	assert.Equal(t, "0.42.99-test", meta["vakt_version"],
		"meta.json must carry the running version, not a build-time placeholder")
	assert.NotEqual(t, "dev", meta["vakt_version"])
	assert.Equal(t, "2", meta["format_version"],
		"the uuid/bytea/date change is a format change and must be announced in meta.json")
	assert.Equal(t, f.orgID, meta["org_id"])
}

// TestDataExport_AllFilesAreValidJSON is the baseline: nothing dropped out of
// the archive and every file still parses.
func TestDataExport_AllFilesAreValidJSON(t *testing.T) {
	if testing.Short() {
		t.Skip("integration: -short mode")
	}
	f := setupExportTypes(t)
	files := runExport(t, f.pool, f.orgID, "vaktcomply,vakthr,vaktaware,vaktprivacy")

	want := []string{
		"meta.json",
		"frameworks.json", "controls.json", "evidence.json", "risks.json", "incidents.json",
		"policies.json", "capas.json", "tasks.json", "comments.json",
		"vvt.json", "dpias.json", "avv.json", "breaches.json",
		"hr_employees.json", "hr_checklist_runs.json", "hr_contractors.json", "hr_mover_events.json",
		"sr_targets.json", "sr_events.json", "sr_assignments.json", "sr_completions.json",
		"audit_log.json",
	}
	for _, name := range want {
		require.Contains(t, files, name, "archive is missing %s", name)
		var v any
		assert.NoError(t, json.Unmarshal([]byte(files[name]), &v), "%s is not valid JSON", name)
	}
	assert.Len(t, files, len(want), "unexpected extra file in the archive")
}

// TestDataExport_NoUnrenderableColumnTypes is the drift guard.
//
// The export scans generically, so it inherits whatever column types the schema
// grows. dataexport.RenderableOIDs is the reviewed set of types it can put into
// JSON faithfully; `interval` and `time` are deliberately not in it, because pgx
// hands them back as pgtype structs that json.Marshal renders as
// {"Microseconds":…,"Valid":true}. If a migration ever adds such a column to an
// exported table, this test goes red and names it — instead of the archive
// silently gaining that noise.
func TestDataExport_NoUnrenderableColumnTypes(t *testing.T) {
	if testing.Short() {
		t.Skip("integration: -short mode")
	}
	f := setupExportTypes(t)
	ctx := context.Background()

	tables := dataexport.ExportedTables()
	rows, err := f.pool.Query(ctx, `
		SELECT c.relname, a.attname, a.atttypid, format_type(a.atttypid, a.atttypmod)
		FROM pg_attribute a
		JOIN pg_class c ON c.oid = a.attrelid
		JOIN pg_namespace n ON n.oid = c.relnamespace
		WHERE n.nspname = 'public' AND c.relname = ANY($1) AND a.attnum > 0 AND NOT a.attisdropped
		ORDER BY c.relname, a.attnum`, tables)
	require.NoError(t, err)
	defer rows.Close()

	checked := 0
	for rows.Next() {
		var table, column, typeName string
		var oid uint32
		require.NoError(t, rows.Scan(&table, &column, &oid, &typeName))
		checked++
		_, known := dataexport.RenderableOIDs[oid]
		assert.True(t, known,
			"%s.%s is %s (oid %d) — the data export scans generically and has no reviewed "+
				"rendering for this type. Add a case to normalizeValue and an entry to "+
				"RenderableOIDs, or the archive will carry pgx's internal representation.",
			table, column, typeName, oid)
	}
	require.NoError(t, rows.Err())

	// Name the denominator: a guard that checked nothing must not pass.
	require.Greater(t, checked, 200, "expected several hundred columns across %d exported tables, checked %d", len(tables), checked)
	t.Logf("schema guard: %d columns across %d exported tables, all renderable", checked, len(tables))
}
