//go:build integration

// Copyright (c) 2026 NorvikOps. All rights reserved.
// SPDX-License-Identifier: Elastic-2.0

package integration_test

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/matharnica/vakt/internal/modules/vaktcomply"
	"github.com/matharnica/vakt/internal/modules/vakthr"
	"github.com/matharnica/vakt/internal/modules/vaktprivacy"
)

// TestR1_18_EmptyListsSerializeAsArrayNotNull is the regression guard for
// R1-18-D1 (CRITICAL, SHAPE_MISMATCH). Four list repositories/services used the
// nil-able idiom `var out []T` and returned a nil slice on the empty state, which
// serialises to JSON `null`, not `[]`. The handlers pass the slice straight to
// c.JSON with no guard, so an empty org received `200 null`. The frontend's
// `const { data = [] }` destructuring default only fires on `undefined`, not
// `null` → `data.map(...)` throws → the ErrorBoundary tears the whole module to
// its error page. SA-18 confirmed this live for crypto-keys, interested-parties,
// mover-events and transfers (4 of 56 array endpoints).
//
// The fix normalises each source to `make([]T, 0)` at the repository/service
// boundary — the same class fix S131-D3 applied (see empty_list_shape_real_test).
// This test drives each source function against a fresh, empty org and asserts a
// non-nil empty slice. Reverting any source to `var out []T` makes that
// assertion red (a nil slice on zero rows).
//
// The variant sweep (I4) also normalised three same-idiom siblings whose leak was
// masked live — adequacy-decisions (seeded global table), TIAs-for-transfer
// (nested under a transfer id) and mover-templates (seed-on-first-access). They
// are pinned here too; adequacy is force-emptied first so its assertion is a real
// regression guard rather than passing on seed rows.
func TestR1_18_EmptyListsSerializeAsArrayNotNull(t *testing.T) {
	pool, orgID, cleanup := bootPostgresWithOrg(t)
	defer cleanup()
	ctx := context.Background()

	ckRepo := vaktcomply.NewRepository(pool)
	hrRepo := vakthr.NewRepository(pool)
	tia := vaktprivacy.NewTIAService(pool)

	// ── Confirmed live (SA-18) ────────────────────────────────────────────────

	keys, err := ckRepo.ListCryptoKeys(ctx, orgID)
	require.NoError(t, err)
	assert.NotNil(t, keys, "ListCryptoKeys must return [] not nil on the empty state")
	assert.Len(t, keys, 0)

	parties, err := ckRepo.ListInterestedParties(ctx, orgID)
	require.NoError(t, err)
	assert.NotNil(t, parties, "ListInterestedParties must return [] not nil on the empty state")
	assert.Len(t, parties, 0)

	movers, err := hrRepo.ListMoverEvents(ctx, orgID)
	require.NoError(t, err)
	assert.NotNil(t, movers, "ListMoverEvents must return [] not nil on the empty state")
	assert.Len(t, movers, 0)

	transfers, err := tia.ListTransfers(ctx, orgID)
	require.NoError(t, err)
	assert.NotNil(t, transfers, "ListTransfers must return [] not nil on the empty state")
	assert.Len(t, transfers, 0)

	// ── Same-class variants (I4) ──────────────────────────────────────────────

	templates, err := hrRepo.ListMoverTemplates(ctx, orgID)
	require.NoError(t, err)
	assert.NotNil(t, templates, "ListMoverTemplates must return [] not nil on the empty state")
	assert.Len(t, templates, 0)

	tias, err := tia.ListTIAsForTransfer(ctx, orgID, uuid.NewString())
	require.NoError(t, err)
	assert.NotNil(t, tias, "ListTIAsForTransfer must return [] not nil on the empty state")
	assert.Len(t, tias, 0)

	// Adequacy decisions live in a seeded global table (migration 189), so force
	// the empty state to make this a genuine red-on-revert guard.
	_, err = pool.Exec(ctx, `DELETE FROM po_adequacy_decisions`)
	require.NoError(t, err)
	decisions, err := tia.ListAdequacyDecisions(ctx)
	require.NoError(t, err)
	assert.NotNil(t, decisions, "ListAdequacyDecisions must return [] not nil on the empty state")
	assert.Len(t, decisions, 0)
}
