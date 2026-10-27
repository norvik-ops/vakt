// Copyright (c) 2026 NorvikOps. All rights reserved.
// SPDX-License-Identifier: Elastic-2.0

package vaktcomply

import (
	"context"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/matharnica/vakt/internal/modules/vaktcomply/policy"
	"github.com/matharnica/vakt/internal/shared/dashboard"
)

// DashboardReadiness implements dashboard.ReadinessSource with the canonical
// readiness formula of this module (ADR-0086). The dependency points module →
// shared, as ADR-0004/ADR-0066 require: the dashboard only knows the
// interface, cmd/api wires this implementation (R1-KW-6, Subreview PR #182).
type DashboardReadiness struct {
	repo *policy.Repository
}

// NewDashboardReadiness returns the readiness source for the dashboard.
func NewDashboardReadiness(pool *pgxpool.Pool) *DashboardReadiness {
	return &DashboardReadiness{repo: policy.NewRepository(pool)}
}

// ReadinessSummary returns the tally per framework and the org-wide value,
// pooled over all frameworks (sums first, then one ReadinessScore).
func (d *DashboardReadiness) ReadinessSummary(ctx context.Context, orgID string) (dashboard.ReadinessSummary, error) {
	byFW, err := d.repo.ReadinessByFramework(ctx, orgID)
	if err != nil {
		return dashboard.ReadinessSummary{}, err
	}
	out := dashboard.ReadinessSummary{ByFramework: make(map[string]dashboard.FrameworkReadiness, len(byFW))}
	for id, t := range byFW {
		out.ByFramework[id] = toDashboardReadiness(t)
	}
	out.Overall = toDashboardReadiness(policy.PoolReadiness(byFW))
	return out, nil
}

func toDashboardReadiness(t policy.FrameworkReadiness) dashboard.FrameworkReadiness {
	return dashboard.FrameworkReadiness{Covered: t.Covered, Partial: t.Partial, Applicable: t.Applicable, Score: t.Score}
}
