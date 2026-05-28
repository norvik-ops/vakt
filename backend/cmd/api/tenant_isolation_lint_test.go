// Copyright (c) 2026 NorvikOps. All rights reserved.
// SPDX-License-Identifier: Elastic-2.0

package main

import (
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"testing"
)

// TestTenantIsolation_OrgIDInQueries scans db/queries/*.sql for sqlc queries
// against org-scoped tables (any table with an org_id column referencing
// organizations) and fails if a SELECT, UPDATE, or DELETE on such a table
// is missing an org_id filter. INSERTs are checked for an explicit org_id
// column in the column list.
//
// Why this exists: ADR-0042 rolled back PostgreSQL Row-Level Security
// (migration 150). Tenant isolation now lives 100% in the application
// layer — a single forgotten WHERE org_id = $N clause is a cross-tenant
// data leak with no second line of defence. This linter is that second
// line: it runs in CI and blocks merges that introduce drift.
//
// Caveats:
//   - Detects the obvious case (org-scoped table mentioned, "org_id" token
//     absent in the same block). It does NOT prove the org_id filter is
//     wired to the caller's org context — that's reviewer + integration
//     test territory.
//   - JOINs that inherit isolation from the driving table are reported as
//     OK as long as the block contains the org_id token somewhere.
func TestTenantIsolation_OrgIDInQueries(t *testing.T) {
	repoRoot, err := findRepoRoot()
	if err != nil {
		t.Fatalf("locate repo root: %v", err)
	}

	orgScoped, err := loadOrgScopedTables(filepath.Join(repoRoot, "db/migrations"))
	if err != nil {
		t.Fatalf("load org-scoped tables: %v", err)
	}
	if len(orgScoped) < 30 {
		t.Fatalf("only %d org-scoped tables discovered — migration glob likely broken", len(orgScoped))
	}

	violations, err := scanQueries(filepath.Join(repoRoot, "db/queries"), orgScoped)
	if err != nil {
		t.Fatalf("scan queries: %v", err)
	}

	// Allow-list: pre-existing queries that lack an org_id filter today.
	// Each one is either (a) a pure primary-key lookup where the PK already
	// scopes the row uniquely, or (b) a known follow-up tracked in the
	// post-marketreife backlog. Adding entries here REQUIRES updating the
	// matching audit ticket in docs/post-rls-tenant-lint-backlog.md.
	allowList := map[string]bool{
		"MarkExpiredPPAVVs":                       true, // bulk status update; PK-bound elsewhere
		"BatchUpdateSPComponentEOL":               true, // FK chain via vb_sboms
		"ListSPComponentsBySBOM":                  true, // FK chain via vb_sboms
		"ListSPComponentsBySBOMFull":              true, // FK chain via vb_sboms
		"StoreSPReportContent":                    true, // PK update by id
		"UpdateSPComponentEOL":                    true, // PK update by id
		"UpdateSPReport":                          true, // PK update by id
		"UpdateSPScanStatus":                      true, // PK update by id
		"GetSVSecretProjectID":                    true, // PK lookup by id
		"GetSVShareLink":                          true, // PK lookup by share-token
		"UpdateSVSecretAccess":                    true, // PK update by id
		"GetCKPolicyAcceptanceCampaignStats":      true, // aggregate via campaign_id FK
		"IncrementCKAuditorLinkUsage":             true, // PK update by id
		"ListCKPolicyAcceptanceRequests":          true, // FK chain via campaign_id
		"MarkCKEvidenceExpiryNotified":            true, // PK update by id
		"MarkCKPolicyAcceptanceRequestSent":       true, // PK update by id
		"RecordCKPolicyAcceptance":                true, // PK update by request_id
		"UpdateCKAssessmentStatus":                true, // PK update by id
		"UpdateCKAuditorLinkAccess":               true, // PK update by id
		"UpdateCKCCMCheckEnabled":                 true, // PK update by id
		"UpdateCKCCMCheckLastRun":                 true, // PK update by id
	}

	var unexpected []string
	for _, v := range violations {
		queryName := strings.SplitN(strings.SplitN(v, ":", 2)[1], " ", 2)[0]
		if !allowList[queryName] {
			unexpected = append(unexpected, v)
		}
	}

	if len(unexpected) > 0 {
		sort.Strings(unexpected)
		t.Errorf("tenant-isolation: %d NEW sqlc queries on org-scoped tables lack org_id (not in allow-list):\n  - %s\n\nADD AN org_id FILTER (preferred) OR justify in the allow-list with a // comment.",
			len(unexpected), strings.Join(unexpected, "\n  - "))
	}
}

func findRepoRoot() (string, error) {
	wd, err := os.Getwd()
	if err != nil {
		return "", err
	}
	dir := wd
	for {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			return dir, nil
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return "", os.ErrNotExist
		}
		dir = parent
	}
}

var createTableRE = regexp.MustCompile(`(?is)CREATE\s+TABLE\s+(?:IF\s+NOT\s+EXISTS\s+)?([a-z_][a-z0-9_]*)\s*\((.*?)\);`)

func loadOrgScopedTables(migrationsDir string) (map[string]bool, error) {
	out := map[string]bool{}
	entries, err := os.ReadDir(migrationsDir)
	if err != nil {
		return nil, err
	}
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".up.sql") {
			continue
		}
		body, err := os.ReadFile(filepath.Join(migrationsDir, e.Name()))
		if err != nil {
			return nil, err
		}
		text := string(body)
		for _, m := range createTableRE.FindAllStringSubmatch(text, -1) {
			table, body := strings.ToLower(m[1]), strings.ToLower(m[2])
			// An org-scoped table has an org_id column that references the
			// organizations table OR is typed as UUID NOT NULL (matches the
			// canonical pattern used throughout the schema).
			if !strings.Contains(body, "org_id") {
				continue
			}
			if strings.Contains(body, "references organizations") ||
				regexpMatch(`\borg_id\s+uuid`, body) {
				out[table] = true
			}
		}
	}
	// Some tables are intentionally NOT org-scoped (organizations, users,
	// roles, permissions, login_history aggregates). They never trigger the
	// linter regardless of what the heuristic says.
	delete(out, "organizations")
	delete(out, "users")
	delete(out, "roles")
	delete(out, "permissions")
	return out, nil
}

func regexpMatch(pattern, text string) bool {
	return regexp.MustCompile(pattern).MatchString(text)
}

// sqlcQueryHeader matches "-- name: QueryName :many|:one|:exec|:execrows..."
var sqlcQueryHeader = regexp.MustCompile(`(?m)^--\s*name:\s*(\S+)\s*:\s*(\S+)\s*$`)

// blockTables extracts referenced tables from a single sqlc block.
var blockTableRE = regexp.MustCompile(`(?is)(?:FROM|JOIN|INTO|UPDATE|DELETE\s+FROM)\s+([a-z_][a-z0-9_]*)`)

func scanQueries(queriesDir string, orgScoped map[string]bool) ([]string, error) {
	var violations []string
	entries, err := os.ReadDir(queriesDir)
	if err != nil {
		return nil, err
	}
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".sql") {
			continue
		}
		body, err := os.ReadFile(filepath.Join(queriesDir, e.Name()))
		if err != nil {
			return nil, err
		}
		blocks := splitSqlcBlocks(string(body))
		for _, b := range blocks {
			if v := checkBlock(b, orgScoped, e.Name()); v != "" {
				violations = append(violations, v)
			}
		}
	}
	return violations, nil
}

type sqlcBlock struct {
	Name string
	Op   string // many / one / exec / execrows
	SQL  string
}

func splitSqlcBlocks(content string) []sqlcBlock {
	var out []sqlcBlock
	headers := sqlcQueryHeader.FindAllStringSubmatchIndex(content, -1)
	for i, idx := range headers {
		nameStart, nameEnd := idx[2], idx[3]
		opStart, opEnd := idx[4], idx[5]
		var sqlStart, sqlEnd int
		sqlStart = idx[1] // end of header line
		if i+1 < len(headers) {
			sqlEnd = headers[i+1][0]
		} else {
			sqlEnd = len(content)
		}
		out = append(out, sqlcBlock{
			Name: content[nameStart:nameEnd],
			Op:   content[opStart:opEnd],
			SQL:  content[sqlStart:sqlEnd],
		})
	}
	return out
}

func checkBlock(b sqlcBlock, orgScoped map[string]bool, file string) string {
	lower := strings.ToLower(b.SQL)
	// Skip exec/execrows that don't actually touch a table (calls / refresh views).
	for _, m := range blockTableRE.FindAllStringSubmatch(lower, -1) {
		tbl := m[1]
		if !orgScoped[tbl] {
			continue
		}
		// Block touches an org-scoped table. Does it mention org_id anywhere?
		if !strings.Contains(lower, "org_id") {
			return file + ":" + b.Name + " — touches " + tbl + ", no org_id reference"
		}
	}
	return ""
}
