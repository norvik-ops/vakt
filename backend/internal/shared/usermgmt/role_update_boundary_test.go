// Copyright (c) 2026 NorvikOps. All rights reserved.
// SPDX-License-Identifier: Elastic-2.0

package usermgmt

import (
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// R1-21-A02 — the UPDATE boundary, as a gate instead of a comment.
//
// ADR-0077 and scripts/check_user_role_insert.py pin the INSERT boundary: every
// site that inserts an org_members row must set users.role to match. Nobody had
// written the rule for the UPDATE side, and the result was
// PATCH /admin/users/:id/role writing only the cache: "UPDATE org_members"
// existed nowhere in the repository, so a demoted Admin kept the Admin claim on
// every route that reads org_members — 97 of them — while the UI reported
// success. ESK-13 fixed the endpoint (measured: reverting the org_members write
// turns TestUpdateUserRole_writesTheAuthoritativeRole red) and left the rule as
// a comment, naming this gate as the missing counterpart.
//
// The rule: org_members.role_id is the role, users.role is its lossy cache, and
// UpdateUserRole is the only production site that may change either after the
// insert — it changes both in one transaction. A second UPDATE site writing just
// one of them re-creates the drift migration 253 had to clean up.
//
// Scope, stated rather than implied (a gate has to say what it does NOT look at):
//   - Only production Go under backend/internal, backend/cmd, backend/pkg.
//   - *_test.go is excluded and counted: fixtures seed drifted members on
//     purpose (seedDriftedMember), which is the opposite of a production write.
//   - db/migrations/**.sql is NOT scanned. Migrations 249 and 253 write
//     users.role by design, as an operator-run backfill, and are reviewed as
//     migrations.
//   - Only string literals are read, via go/parser — prose in a comment that
//     mentions UPDATE org_members neither fails the gate nor inflates its count.
//   - A write through a view or through dynamically assembled SQL is not
//     visible here. Eine solche Stelle existiert: cmd/seed/main.go schreibt role_id per Upsert. Sie ist
//     seit der Nachbesserung abgedeckt; die Aufzaehlung unten nennt die Schreibweisen,
//     die das Tor kennt — und nur die
//     caught.
func TestRoleUpdateBoundary_onlyOneSiteMayWriteEitherRoleColumn(t *testing.T) {
	backendRoot := backendRoot(t)
	allowed := filepath.Join("internal", "shared", "usermgmt", "service.go")

	// Begruendete Ausnahmen. Jede steht hier, WEIL sie beide Rollenspalten in
	// derselben Transaktion setzt — genau die Invariante, die dieses Tor schuetzt.
	// Eine Ausnahme ohne diese Eigenschaft gehoert nicht in die Liste, sondern gefixt.
	exempt := map[string]string{
		filepath.Join("cmd", "seed", "main.go"): "Entwickler-Seed: setzt users.role='admin' UND " +
			"org_members.role_id=Admin in derselben Transaktion (main.go:133 und :156). Kein " +
			"Produktionspfad, und die Invariante 'beide Spalten bewegen sich gemeinsam' haelt.",
	}

	var (
		checked    int
		inAllowed  int
		skippedGo  int
		exemptHits int
		violations []string
	)

	for _, root := range []string{"internal", "cmd", "pkg"} {
		dir := filepath.Join(backendRoot, root)
		if _, err := os.Stat(dir); err != nil {
			continue // pkg/ does not exist in every layout — not a silent pass, see the count below
		}
		require.NoError(t, filepath.Walk(dir, func(path string, info os.FileInfo, err error) error {
			if err != nil || info.IsDir() || !strings.HasSuffix(path, ".go") {
				return err
			}
			if strings.HasSuffix(path, "_test.go") {
				skippedGo++
				return nil
			}
			rel, relErr := filepath.Rel(backendRoot, path)
			require.NoError(t, relErr)

			for _, lit := range stringLiterals(t, path) {
				if !writesARoleColumn(lit) {
					continue
				}
				checked++
				if rel == allowed {
					inAllowed++
					continue
				}
				if reason, ok := exempt[rel]; ok {
					exemptHits++
					t.Logf("Ausnahme: %s — %s", rel, reason)
					continue
				}
				violations = append(violations, rel)
			}
			return nil
		}))
	}

	t.Logf("UPDATE-Grenze: %d Schreibstellen auf org_members/users.role gefunden, "+
		"%d davon in %s, %d begruendete Ausnahmen, %d Testdateien uebersprungen",
		checked, inAllowed, allowed, exemptHits, skippedGo)

	// A gate that finds nothing has not proven anything — it has lost its target.
	require.NotZero(t, checked,
		"the gate found no role UPDATE at all; the scan roots or the pattern are wrong, not the code")
	require.NotZero(t, skippedGo, "no test files were skipped — the walk is not reaching the repository")

	assert.Empty(t, violations,
		"these files write org_members.role_id or users.role outside UpdateUserRole. "+
			"Both columns move together in one transaction or they drift apart — that drift is R1-21-A02 "+
			"(a demoted Admin keeping their claims) and migration 253 exists to clean up its remains.")
}

// The gate's own acceptance test: it has to go red over a real violation, in
// every spelling the SQL allows, and stay quiet over what is not one.
func TestRoleUpdateBoundary_detectorCatchesTheSpellingsItClaims(t *testing.T) {
	// The strings below are probe text for the detector and are never executed —
	// hence the per-line orgid-lint opt-outs.
	violating := []struct {
		name string
		sql  string
	}{
		{"bare table", `UPDATE org_members SET role_id = $1 WHERE user_id = $2`}, // orgid-lint: global — probe text, never executed
		{"schema qualified", `UPDATE public.org_members SET role_id = $1`},       // orgid-lint: global — probe text, never executed
		{"upsert", `INSERT INTO org_members (org_id, user_id, role_id) VALUES ($1,$2,$3)
			ON CONFLICT (org_id, user_id) DO UPDATE SET role_id = EXCLUDED.role_id`}, // orgid-lint: global — probe text, never executed
		{"quoted", `UPDATE "org_members" SET role_id = $1`},   // orgid-lint: global — probe text, never executed
		{"lower case", `update org_members set role_id = $1`}, // orgid-lint: global — probe text, never executed
		{"cache only", `UPDATE users SET role = $1 WHERE id = $2::uuid`},
		{"cache, schema qualified", `UPDATE public.users SET role = $1, updated_at = NOW()`},
		{"cache, quoted column", `UPDATE users SET "role" = $1`},
		{"cache, multi-line", "UPDATE users\n\t\t\tSET display_name = $1,\n\t\t\t    role = $2\n\t\t\tWHERE id = $3"},
	}
	for _, tc := range violating {
		t.Run(tc.name, func(t *testing.T) {
			assert.True(t, writesARoleColumn(tc.sql), "not detected: %s", tc.sql)
		})
	}

	harmless := []struct {
		name string
		sql  string
	}{
		{"other column on users", `UPDATE users SET last_login_at = NOW() WHERE id = $1::uuid`},
		{"deactivation", `UPDATE users SET is_active = FALSE, updated_at = NOW() WHERE id = $1::uuid`},
		{"pw_version bump", `UPDATE users SET pw_version = pw_version + 1 WHERE id = $1::uuid RETURNING pw_version`},
		{"role_id on another table", `UPDATE auditor_invites SET role_id = $1 WHERE id = $2`}, // orgid-lint: global — probe text, never executed
		{"reading org_members", `SELECT role_id FROM org_members WHERE org_id = $1`},
		{"inserting a member", `INSERT INTO org_members (org_id, user_id, role_id) VALUES ($1, $2, $3)`},
		{"a column that merely starts with role", `UPDATE users SET role_synced_at = NOW() WHERE id = $1`},
	}
	for _, tc := range harmless {
		t.Run(tc.name, func(t *testing.T) {
			assert.False(t, writesARoleColumn(tc.sql), "false positive: %s", tc.sql)
		})
	}
}

// ─── the detector ────────────────────────────────────────────────────────────

var (
	// UPDATE against org_members, optionally schema-qualified and/or quoted —
	// the K2-06 lesson from check_user_role_insert.py, where the bare-identifier
	// pattern silently missed public.users and "users".
	updateOrgMembersRe = regexp.MustCompile(`(?is)\bupdate\s+(?:only\s+)?(?:"?[a-z_][a-z_0-9]*"?\.)?"?org_members"?\b`)
	updateUsersRe      = regexp.MustCompile(`(?is)\bupdate\s+(?:only\s+)?(?:"?[a-z_][a-z_0-9]*"?\.)?"?users"?\b`)
	// An assignment to the role column itself. role_id, role_synced_at and the
	// like do not match: the name has to end right there.
	assignsRoleRe = regexp.MustCompile(`(?is)(^|[\s,(])"?role"?\s*=`)

	// Ein Upsert schreibt dieselbe Spalte, ohne das Wort UPDATE vor den Tabellennamen
	// zu stellen: `INSERT INTO org_members ... ON CONFLICT ... DO UPDATE SET role_id`.
	// Die erste Fassung dieses Tors sah genau diese Schreibweise nicht, obwohl eine
	// solche Stelle in seinem eigenen Suchbereich liegt (cmd/seed/main.go). Ein Tor,
	// das eine gaengige Schreibweise nicht kennt, meldet Abdeckung, die es nicht hat.
	upsertRoleRe = regexp.MustCompile(`(?is)\bon\s+conflict\b[^;]*?\bdo\s+update\s+set\b[^;]*?"?role(_id)?"?\s*=`)
)

// writesARoleColumn reports whether the SQL changes either role column:
// org_members.role_id (the authoritative role) or users.role (its cache).
func writesARoleColumn(sql string) bool {
	if updateOrgMembersRe.MatchString(sql) {
		return true
	}
	// Upsert-Schreibweise: schreibt role_id, ohne UPDATE vor den Tabellennamen zu setzen.
	if upsertRoleRe.MatchString(sql) {
		return true
	}
	loc := updateUsersRe.FindStringIndex(sql)
	if loc == nil {
		return false
	}
	return assignsRoleRe.MatchString(sql[loc[1]:])
}

// stringLiterals returns the unquoted string literals of a Go file. Reading
// literals instead of the raw text keeps comments out of the gate — a sentence
// about UPDATE org_members is prose, not a write.
func stringLiterals(t *testing.T, path string) []string {
	t.Helper()
	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, path, nil, 0)
	require.NoError(t, err, "parse %s", path)

	var out []string
	ast.Inspect(file, func(n ast.Node) bool {
		lit, ok := n.(*ast.BasicLit)
		if !ok || lit.Kind != token.STRING {
			return true
		}
		if s, uErr := strconv.Unquote(lit.Value); uErr == nil {
			out = append(out, s)
		}
		return true
	})
	return out
}

// backendRoot walks up from this package to backend/.
func backendRoot(t *testing.T) string {
	t.Helper()
	wd, err := os.Getwd() // backend/internal/shared/usermgmt
	require.NoError(t, err)
	root := filepath.Join(wd, "..", "..", "..")
	abs, err := filepath.Abs(root)
	require.NoError(t, err)
	require.DirExists(t, filepath.Join(abs, "internal"), "backend root not found at %s", abs)
	return abs
}
