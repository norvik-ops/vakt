// Copyright (c) 2026 NorvikOps. All rights reserved.
// SPDX-License-Identifier: Elastic-2.0

package policy

import (
	"context"
	"fmt"
	"os"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/require"
)

// R1-KW-16: an accepted policy must be filed as evidence on ISO 27001 A.5.1.
// FindCKControlsByKeywords returned the first 10 keyword hits in physical row
// order and never looked at control_id. With every framework of a real org
// matching "policy"/"richtlinie" (NIS2, BSI, CIS, …) more than 10 controls hit,
// and A.5.1 fell off the end — the employee confirmed, the evidence went to ten
// unrelated controls (Kernweg KW7).
//
// The fixture reproduces that shape on purpose: 12 keyword hits in other
// frameworks are inserted BEFORE the ISO controls, plus A.5.10–A.5.12, whose
// control_id merely contains "a.5.1". A.5.1 must still come back, and first.
func TestFindControlsByKeywords_exactControlIDWinsOverKeywordHits(t *testing.T) {
	url := os.Getenv("VAKT_DB_URL")
	if url == "" {
		t.Skip("VAKT_DB_URL not set — dieser Test braucht eine migrierte Postgres (CI setzt sie)")
	}
	ctx := context.Background()
	pool, err := pgxpool.New(ctx, url)
	require.NoError(t, err)
	t.Cleanup(pool.Close)

	var orgID string
	slug := fmt.Sprintf("r1-kw-16-%d", os.Getpid())
	require.NoError(t, pool.QueryRow(ctx,
		`INSERT INTO organizations (name, slug) VALUES ($1, $1) RETURNING id::text`, slug).Scan(&orgID))
	t.Cleanup(func() {
		_, _ = pool.Exec(context.Background(), `DELETE FROM ck_controls WHERE org_id = $1::uuid`, orgID)
		_, _ = pool.Exec(context.Background(), `DELETE FROM ck_frameworks WHERE org_id = $1::uuid`, orgID)
		_, _ = pool.Exec(context.Background(), `DELETE FROM organizations WHERE id = $1::uuid`, orgID)
	})

	framework := func(name string) string {
		var id string
		require.NoError(t, pool.QueryRow(ctx,
			`INSERT INTO ck_frameworks (org_id, name) VALUES ($1::uuid, $2) RETURNING id::text`,
			orgID, name).Scan(&id))
		return id
	}
	control := func(fwID, controlID, title string) {
		_, err := pool.Exec(ctx,
			`INSERT INTO ck_controls (framework_id, org_id, control_id, title, domain)
			 VALUES ($1::uuid, $2::uuid, $3, $4, 'Governance')`,
			fwID, orgID, controlID, title)
		require.NoError(t, err)
	}

	for _, fw := range []string{"NIS2", "BSI IT-Grundschutz", "CIS v8"} {
		fwID := framework(fw)
		for i := 1; i <= 4; i++ {
			control(fwID, fmt.Sprintf("%s-%d", fw, i), fmt.Sprintf("Sicherheitsrichtlinie %d (%s)", i, fw))
		}
	}
	iso := framework("ISO27001")
	for _, id := range []string{"A.5.10", "A.5.11", "A.5.12"} {
		control(iso, id, "Zulässiger Gebrauch von Informationen")
	}
	control(iso, "A.5.1", "Richtlinien zur Informationssicherheit")

	got, err := NewRepository(pool).FindControlsByKeywords(ctx, orgID, policyAcceptanceKeywords)
	require.NoError(t, err)
	require.NotEmpty(t, got)

	ids := make([]string, len(got))
	for i, c := range got {
		ids[i] = c.ControlID
	}
	require.Equal(t, "A.5.1", got[0].ControlID,
		"A.5.1 must be the first hit for the policy-acceptance keywords, got %v", ids)
	require.LessOrEqual(t, len(got), 10, "the limit stays at 10")
}
