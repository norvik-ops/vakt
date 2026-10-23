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
// The mover-event, transfer and TIA/adequacy cases left with the archived HR and
// Privacy modules (docs/stories/module-archivieren.md); the Comply sources stay.
func TestR1_18_EmptyListsSerializeAsArrayNotNull(t *testing.T) {
	pool, orgID, cleanup := bootPostgresWithOrg(t)
	defer cleanup()
	ctx := context.Background()

	ckRepo := vaktcomply.NewRepository(pool)

	// ── Confirmed live (SA-18) ────────────────────────────────────────────────

	keys, err := ckRepo.ListCryptoKeys(ctx, orgID)
	require.NoError(t, err)
	assert.NotNil(t, keys, "ListCryptoKeys must return [] not nil on the empty state")
	assert.Len(t, keys, 0)

	parties, err := ckRepo.ListInterestedParties(ctx, orgID)
	require.NoError(t, err)
	assert.NotNil(t, parties, "ListInterestedParties must return [] not nil on the empty state")
	assert.Len(t, parties, 0)
}
