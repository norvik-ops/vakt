// Copyright (c) 2026 NorvikOps. All rights reserved.
// SPDX-License-Identifier: Elastic-2.0

package main

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5/pgxpool"
)

// nonDemoOrgIDs returns the IDs of all real (non-ephemeral) organisations.
// Ephemeral demo orgs (slug LIKE 'demo-%') are excluded because:
//   - They are hard-deleted after 4 h by the hourly demo cleanup cron.
//   - All batch snapshot/evidence jobs that iterate orgs write to tables with
//     org_id FK on organizations(id). A race at the :00-minute mark between a
//     batch job and the cleanup produces SQLSTATE 23503 FK violations.
//   - Demo orgs have no need for persistent KPI snapshots, score history,
//     evidence staleness updates, or notification emails.
func nonDemoOrgIDs(ctx context.Context, pool *pgxpool.Pool) ([]string, error) {
	rows, err := pool.Query(ctx, `SELECT id::text FROM organizations WHERE slug NOT LIKE 'demo-%'`)
	if err != nil {
		return nil, fmt.Errorf("list non-demo org ids: %w", err)
	}
	defer rows.Close()

	var ids []string
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		ids = append(ids, id)
	}
	return ids, rows.Err()
}

// orgRow holds org_id + name for handlers that need both.
type orgRow struct {
	id   string
	name string
}

// nonDemoOrgs returns id + name for all real (non-ephemeral) organisations.
// See nonDemoOrgIDs for the rationale behind the demo exclusion.
func nonDemoOrgs(ctx context.Context, pool *pgxpool.Pool) ([]orgRow, error) {
	rows, err := pool.Query(ctx, `SELECT id::text, name FROM organizations WHERE slug NOT LIKE 'demo-%'`)
	if err != nil {
		return nil, fmt.Errorf("list non-demo orgs: %w", err)
	}
	defer rows.Close()

	var orgs []orgRow
	for rows.Next() {
		var o orgRow
		if err := rows.Scan(&o.id, &o.name); err != nil {
			return nil, err
		}
		orgs = append(orgs, o)
	}
	return orgs, rows.Err()
}
