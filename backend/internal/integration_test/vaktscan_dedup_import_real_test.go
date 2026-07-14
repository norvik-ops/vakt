//go:build integration

// Copyright (c) 2026 NorvikOps. All rights reserved.
// SPDX-License-Identifier: Elastic-2.0

package integration_test

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/hibiken/asynq"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/matharnica/vakt/internal/modules/vaktscan"
)

func strptr(s string) *string { return &s }

// TestVaktscan_FindingDedup_RescanAndReopen (S126) drives the deduplication rule
// that decides what a customer sees after the second scan of the same machine.
//
// The rule is the module's core value ("deduplicates findings, prioritises by
// risk") and lives entirely in raw SQL that no test has ever executed:
//
//   - the same CVE on the same asset is ONE finding, seen twice — not two
//     findings. Get this wrong and every nightly scan doubles the backlog;
//   - a finding that was resolved and comes back must REOPEN and count the
//     reopen — that is the difference between "we fixed it" and "we thought we
//     fixed it", and an ISO 27001 auditor asks for exactly that number;
//   - findings for the same CVE reported by two scanners merge into one, keeping
//     both sources.
func TestVaktscan_FindingDedup_RescanAndReopen(t *testing.T) {
	if testing.Short() {
		t.Skip("integration: -short mode")
	}
	pool, orgID, cleanup := bootPostgresWithOrg(t)
	defer cleanup()
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	defer cancel()

	svc := vaktscan.NewService(pool, asynq.RedisClientOpt{})
	repo := vaktscan.NewRepository(pool)

	asset, err := svc.CreateAsset(ctx, orgID, "", vaktscan.CreateAssetInput{
		Name: "web-01", Type: "server", Criticality: "high",
	})
	require.NoError(t, err)

	finding := vaktscan.Finding{
		AssetID:  asset.ID,
		CVEID:    strptr("CVE-2026-1234"),
		Title:    "OpenSSL heap overflow",
		Severity: "critical",
		Status:   "open",
		Scanner:  "trivy",
		Sources:  []string{"trivy"},
	}

	first, err := repo.UpsertFinding(ctx, orgID, finding)
	require.NoError(t, err)
	assert.Equal(t, 1, first.OccurrenceCount)
	assert.Equal(t, 0, first.ReopenCount)

	// Tonight's scan finds the same CVE on the same host again.
	second, err := repo.UpsertFinding(ctx, orgID, finding)
	require.NoError(t, err)
	assert.Equal(t, first.ID, second.ID, "the same CVE on the same asset must not create a second finding")
	assert.Equal(t, 2, second.OccurrenceCount, "it was seen twice")
	assert.Equal(t, 0, second.ReopenCount, "it never went away, so it never came back")

	all, err := repo.ListFindings(ctx, orgID, vaktscan.FindingFilter{})
	require.NoError(t, err)
	require.Len(t, all, 1, "two scans of one vulnerable host produce one finding, not two")

	// The team patches it.
	require.NoError(t, pool.QueryRow(ctx,
		`UPDATE vb_findings SET status = 'resolved' WHERE org_id = $1::uuid AND id = $2::uuid RETURNING id`,
		orgID, first.ID).Scan(new(string)))

	// And a later scan finds it again — the patch did not hold, or the box was
	// rebuilt from a stale image.
	third, err := repo.UpsertFinding(ctx, orgID, finding)
	require.NoError(t, err)
	assert.Equal(t, first.ID, third.ID)
	assert.Equal(t, "open", third.Status, "a resolved finding that reappears must reopen")
	assert.Equal(t, 1, third.ReopenCount, "the reopen has to be counted — it is the evidence that the fix failed")

	// A second scanner reports the same CVE. One finding, two sources.
	fromNuclei := finding
	fromNuclei.Scanner = "nuclei"
	fromNuclei.Sources = []string{"nuclei"}
	merged, err := repo.UpsertFinding(ctx, orgID, fromNuclei)
	require.NoError(t, err)
	assert.Equal(t, first.ID, merged.ID, "two scanners, one vulnerability")
	assert.ElementsMatch(t, []string{"trivy", "nuclei"}, merged.Sources, "both scanners must stay credited")
}

// TestVaktscan_ImportAssetsCSV_RoundTrip (S126) runs the CSV asset import against
// a real database. Until now only its failure paths were covered — the unit test
// constructs a bare &Service{} and asserts on errors that occur before the first
// repository call, because a concrete *Repository leaves no other way in. So the
// half that writes rows has never been executed by a test.
func TestVaktscan_ImportAssetsCSV_RoundTrip(t *testing.T) {
	if testing.Short() {
		t.Skip("integration: -short mode")
	}
	pool, orgID, cleanup := bootPostgresWithOrg(t)
	defer cleanup()
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	defer cancel()

	svc := vaktscan.NewService(pool, asynq.RedisClientOpt{})

	csv := strings.Join([]string{
		"name,type,criticality,tags",
		"prod-web-01,server,critical,\"edge,public\"",
		"prod-db-01,database,high,internal",
		// No criticality: the importer defaults it rather than rejecting the row.
		"build-runner,container,,ci",
		// Invalid type: must be reported, and must not take the good rows with it.
		"mystery-box,teapot,low,",
	}, "\n")

	imported, failed, errs, err := svc.ImportAssetsCSV(ctx, orgID, "", strings.NewReader(csv))
	require.NoError(t, err, "a bad row is a row-level failure, not an import-level one")
	assert.Equal(t, 3, imported)
	assert.Equal(t, 1, failed)
	require.Len(t, errs, 1)
	assert.Contains(t, errs[0], "mystery-box", "the rejected row must be named, or the user cannot fix it")

	assets, total, err := svc.ListAssets(ctx, orgID, 1, 50, "")
	require.NoError(t, err)
	assert.Equal(t, 3, total)

	byName := map[string]vaktscan.Asset{}
	for _, a := range assets {
		byName[a.Name] = a
	}
	require.Contains(t, byName, "prod-web-01")
	assert.Equal(t, "critical", byName["prod-web-01"].Criticality)
	assert.ElementsMatch(t, []string{"edge", "public"}, byName["prod-web-01"].Tags,
		"a quoted, comma-separated tag cell is one column with two tags")
	assert.Equal(t, "medium", byName["build-runner"].Criticality, "a blank criticality defaults to medium")
}
