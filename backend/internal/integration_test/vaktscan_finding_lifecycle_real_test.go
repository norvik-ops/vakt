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

	"github.com/matharnica/vakt/internal/modules/vaktscan"

	"github.com/hibiken/asynq"
)

// TestVaktscan_FindingLifecycle_AndClassification is the S126 (A17-01) regression
// guard for vaktscan — the other module that historically produced born-broken
// bugs (DeleteFinding was UI-wired with no repository method; raw-SQL 500s). It
// runs the real repository against real Postgres:
//   - create an asset with a classification and confirm it ROUND-TRIPS on read
//     (S124-3/DB-01: classification was written but never selected);
//   - upsert a finding on it, list it, then DELETE it and confirm it is gone
//     (DeleteFinding — a real CRUD path that only ever existed after S121).
func TestVaktscan_FindingLifecycle_AndClassification(t *testing.T) {
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
		Name:           "prod-db-01",
		Type:           "database",
		Criticality:    "high",
		Classification: "confidential",
	})
	require.NoError(t, err)

	// DB-01: classification must round-trip on read (was always "" before S124-3).
	got, err := repo.GetAsset(ctx, orgID, asset.ID)
	require.NoError(t, err)
	assert.Equal(t, "confidential", got.Classification,
		"asset classification must survive a read (DB-01)")

	// Create a finding on the asset.
	tmpl := "cwe-89-sqli"
	finding, err := repo.UpsertFinding(ctx, orgID, vaktscan.Finding{
		AssetID:    asset.ID,
		Title:      "SQL injection in login",
		Severity:   "high",
		Status:     "open",
		Scanner:    "nuclei",
		TemplateID: tmpl,
	})
	require.NoError(t, err)
	require.NotEmpty(t, finding.ID)

	list, err := repo.ListFindings(ctx, orgID, vaktscan.FindingFilter{})
	require.NoError(t, err)
	assert.Len(t, list, 1, "one finding should be listed after create")

	// DeleteFinding — the born-broken CRUD path. Must actually remove the row.
	require.NoError(t, svc.DeleteFinding(ctx, orgID, finding.ID))

	_, err = repo.GetFinding(ctx, orgID, finding.ID)
	assert.Error(t, err, "GetFinding must fail after delete")

	list2, err := repo.ListFindings(ctx, orgID, vaktscan.FindingFilter{})
	require.NoError(t, err)
	assert.Empty(t, list2, "no findings should remain after delete")
}
