// Copyright (c) 2026 NorvikOps. All rights reserved.
// SPDX-License-Identifier: Elastic-2.0

package vaktcomply

import (
	"context"
	"fmt"
	"time"
)

// CalculateAndStoreKPIs computes all ISMS KPIs for the organisation and persists
// them as a daily snapshot (upsert on org_id + snapshot_date).
func (s *Service) CalculateAndStoreKPIs(ctx context.Context, orgID string) error {
	snap := CalculateKPIsForOrg(ctx, s.db, orgID)
	snap.OrgID = orgID
	if err := s.repo.UpsertKPISnapshot(ctx, orgID, snap); err != nil {
		return fmt.Errorf("calculate and store kpis for org %s: %w", orgID, err)
	}
	return nil
}

// GetKPIDashboard returns the latest KPI snapshot and the 90-day history for
// the organisation.
func (s *Service) GetKPIDashboard(ctx context.Context, orgID string) (KPIDashboard, error) {
	current, err := s.repo.GetLatestKPISnapshot(ctx, orgID)
	if err != nil {
		return KPIDashboard{}, fmt.Errorf("get kpi dashboard current: %w", err)
	}
	since := time.Now().AddDate(0, -3, 0) // ~90 days
	history, err := s.repo.ListKPISnapshots(ctx, orgID, since)
	if err != nil {
		return KPIDashboard{}, fmt.Errorf("get kpi dashboard history: %w", err)
	}
	if history == nil {
		history = []KPISnapshot{}
	}
	return KPIDashboard{Current: current, History: history}, nil
}
