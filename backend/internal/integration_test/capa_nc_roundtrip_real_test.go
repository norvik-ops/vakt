//go:build integration

// Copyright (c) 2026 NorvikOps. All rights reserved.
// SPDX-License-Identifier: Elastic-2.0

package integration_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/matharnica/vakt/internal/modules/vaktcomply"
)

// TestCAPANCFieldsRoundTrip is the regression guard for S131-G3/D27-02: the
// NC/effectiveness workflow (nc_classification, immediate_containment,
// similar_ncs_*, effectiveness_*) was written by a dedicated endpoint but NO
// read selected those columns — the local sqlc CkCapas model never regenerated
// after Migration 163. The FE badges + edit-form (CAPAsPage.tsx) therefore
// silently rendered empty after every save.
//
// The fix reads CAPAs via a raw NC-aware projection (capaCols). This test writes
// the NC fields directly, then asserts BOTH read projections (single GetCAPA and
// list ListCAPAsPaged) return them — which also guards the hand-written scan-arg
// order against silent column/field misalignment (a wrong order would map data
// into the wrong field, worse than a missing value).
func TestCAPANCFieldsRoundTrip(t *testing.T) {
	pool, orgID, cleanup := bootPostgresWithOrg(t)
	defer cleanup()
	ctx := context.Background()

	repo := vaktcomply.NewRepository(pool)

	created, err := repo.CreateCAPA(ctx, orgID, vaktcomply.CreateCAPAInput{
		SourceType: "manual",
		Title:      "NC round-trip guard",
	})
	require.NoError(t, err)
	require.NotEmpty(t, created.ID)
	// Fresh CAPA has no NC fields yet.
	assert.Nil(t, created.NCClassification)
	assert.Nil(t, created.EffectivenessConfirmed)

	// Write the NC/effectiveness fields the way the NC-workflow endpoint does.
	_, err = pool.Exec(ctx, `
		UPDATE ck_capas
		SET nc_classification        = 'major_nc',
		    immediate_containment    = 'isolated the affected system',
		    similar_ncs_assessed     = true,
		    similar_ncs_notes        = 'checked two adjacent controls',
		    effectiveness_check_date = '2026-08-01',
		    effectiveness_confirmed  = true,
		    effectiveness_evidence   = 'follow-up audit passed'
		WHERE id = $1 AND org_id = $2`, created.ID, orgID)
	require.NoError(t, err)

	assertNC := func(t *testing.T, c vaktcomply.CAPA, where string) {
		t.Helper()
		require.NotNil(t, c.NCClassification, "%s: nc_classification", where)
		assert.Equal(t, "major_nc", *c.NCClassification, where)
		assert.Equal(t, "isolated the affected system", c.ImmediateContainment, where)
		require.NotNil(t, c.SimilarNCsAssessed, "%s: similar_ncs_assessed", where)
		assert.True(t, *c.SimilarNCsAssessed, where)
		assert.Equal(t, "checked two adjacent controls", c.SimilarNCsNotes, where)
		require.NotNil(t, c.EffectivenessCheckDate, "%s: effectiveness_check_date", where)
		assert.Equal(t, "2026-08-01", *c.EffectivenessCheckDate, where)
		require.NotNil(t, c.EffectivenessConfirmed, "%s: effectiveness_confirmed", where)
		assert.True(t, *c.EffectivenessConfirmed, where)
		assert.Equal(t, "follow-up audit passed", c.EffectivenessEvidence, where)
	}

	// Single read.
	got, err := repo.GetCAPA(ctx, orgID, created.ID)
	require.NoError(t, err)
	assertNC(t, got, "GetCAPA")

	// List read (independent scan path).
	page, total, err := repo.ListCAPAsPaged(ctx, orgID, "", 0, 50)
	require.NoError(t, err)
	require.GreaterOrEqual(t, total, 1)
	var found *vaktcomply.CAPA
	for i := range page {
		if page[i].ID == created.ID {
			found = &page[i]
			break
		}
	}
	require.NotNil(t, found, "created CAPA must appear in ListCAPAsPaged")
	assertNC(t, *found, "ListCAPAsPaged")
}
