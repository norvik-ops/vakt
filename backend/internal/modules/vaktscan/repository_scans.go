// Copyright (c) 2026 NorvikOps. All rights reserved.
// SPDX-License-Identifier: Elastic-2.0

package vaktscan

import (
	"context"
	"fmt"

	"github.com/matharnica/vakt/internal/db"
)

// ---------------------------------------------------------------------------
// Scans
// ---------------------------------------------------------------------------

// CreateScan inserts a new scan record and returns it.
func (r *Repository) CreateScan(ctx context.Context, orgID string, input CreateScanInput, assetID string) (*Scan, error) {
	row, err := r.q.CreateSPScan(ctx, db.CreateSPScanParams{
		OrgID:     orgID,
		AssetID:   assetID,
		Scanner:   input.Scanner,
		TargetUrl: spOptText(input.TargetURL),
		TargetIp:  spOptText(input.TargetIP),
	})
	if err != nil {
		return nil, fmt.Errorf("insert scan: %w", err)
	}
	s := scanFromVbScans(row)
	return &s, nil
}

// GetScan fetches a scan by ID within the org.
func (r *Repository) GetScan(ctx context.Context, orgID, scanID string) (*Scan, error) {
	row, err := r.q.GetSPScan(ctx, db.GetSPScanParams{ID: scanID, OrgID: orgID})
	if err != nil {
		return nil, fmt.Errorf("get scan: %w", err)
	}
	s := scanFromVbScans(row)
	return &s, nil
}

// UpdateScanStatus updates scan status and optional fields.
func (r *Repository) UpdateScanStatus(ctx context.Context, scanID, status string, opts ...ScanUpdateOpt) error {
	o := &scanUpdateOptions{}
	for _, opt := range opts {
		opt(o)
	}
	err := r.q.UpdateSPScanStatus(ctx, db.UpdateSPScanStatusParams{
		ID:           scanID,
		Status:       status,
		ErrorMessage: spOptText(derefStrPtr(o.errorMessage)),
		FindingCount: spOptInt4(o.findingCount),
		DurationMs:   spOptInt8(o.durationMs),
		StartedAt:    spOptTs(o.startedAt),
		CompletedAt:  spOptTs(o.completedAt),
	})
	if err != nil {
		return fmt.Errorf("update scan status: %w", err)
	}
	return nil
}

// ---------------------------------------------------------------------------
// Scan Schedules
// ---------------------------------------------------------------------------

// CreateScanSchedule inserts a new scan schedule for an asset.
func (r *Repository) CreateScanSchedule(ctx context.Context, orgID, assetID string, input CreateScanScheduleInput) (*ScanSchedule, error) {
	row, err := r.q.CreateSPScanSchedule(ctx, db.CreateSPScanScheduleParams{
		OrgID:    orgID,
		AssetID:  assetID,
		Scanner:  input.Scanner,
		CronExpr: input.CronExpr,
	})
	if err != nil {
		return nil, fmt.Errorf("insert scan schedule: %w", err)
	}
	s := scheduleFromVbScanSchedule(row)
	return &s, nil
}

// ListScanSchedules returns all scan schedules for an asset.
func (r *Repository) ListScanSchedules(ctx context.Context, orgID, assetID string) ([]ScanSchedule, error) {
	rows, err := r.q.ListSPScanSchedules(ctx, db.ListSPScanSchedulesParams{OrgID: orgID, AssetID: assetID})
	if err != nil {
		return nil, fmt.Errorf("list scan schedules: %w", err)
	}
	out := make([]ScanSchedule, 0, len(rows))
	for _, row := range rows {
		out = append(out, scheduleFromVbScanSchedule(row))
	}
	return out, nil
}

// DeleteScanSchedule removes a scan schedule by ID within the org.
func (r *Repository) DeleteScanSchedule(ctx context.Context, orgID, scheduleID string) error {
	n, err := r.q.DeleteSPScanSchedule(ctx, db.DeleteSPScanScheduleParams{ID: scheduleID, OrgID: orgID})
	if err != nil {
		return fmt.Errorf("delete scan schedule: %w", err)
	}
	if n == 0 {
		return fmt.Errorf("scan schedule not found")
	}
	return nil
}

// ---------------------------------------------------------------------------
// Risk Trend
// ---------------------------------------------------------------------------

// GetRiskTrend returns daily aggregated risk data over the last N days.
// It reads from vb_risk_trend_snapshots when pre-computed data is available,
// falling back to the live generate_series query for orgs without snapshots yet.
func (r *Repository) GetRiskTrend(ctx context.Context, orgID string, days int) ([]RiskTrendPoint, error) {
	if days <= 0 {
		days = 30
	}

	// Prefer snapshot table — O(days) index scan instead of cartesian join.
	const snapshotSQL = `
		SELECT
			TO_CHAR(d::date, 'YYYY-MM-DD')         AS date,
			COALESCE(s.total_risk_score, 0)::float8 AS total_risk_score,
			COALESCE(s.open_count, 0)::int          AS open_count,
			COALESCE(s.critical_count, 0)::int      AS critical_count
		FROM generate_series(
			(CURRENT_DATE - make_interval(days => $2::int))::date,
			CURRENT_DATE,
			'1 day'::interval
		) AS d
		LEFT JOIN vb_risk_trend_snapshots s
			ON s.org_id = $1::uuid
		   AND s.snapshot_date = d::date
		ORDER BY d`

	snapRows, err := r.db.Query(ctx, snapshotSQL, orgID, days)
	if err == nil {
		defer snapRows.Close()
		var out []RiskTrendPoint
		for snapRows.Next() {
			var p RiskTrendPoint
			var openC, critC int32
			if scanErr := snapRows.Scan(&p.Date, &p.TotalRiskScore, &openC, &critC); scanErr != nil {
				continue
			}
			p.OpenCount = int(openC)
			p.CriticalCount = int(critC)
			out = append(out, p)
		}
		if snapRows.Err() == nil && len(out) > 0 {
			// At least one snapshot row exists — return snapshot data.
			return out, nil
		}
	}

	// No snapshots yet (fresh install, job hasn't run). Fall back to live query.
	liveRows, err := r.q.GetSPRiskTrend(ctx, db.GetSPRiskTrendParams{OrgID: orgID, Column2: int32(days)})
	if err != nil {
		return nil, fmt.Errorf("get risk trend: %w", err)
	}
	out := make([]RiskTrendPoint, 0, len(liveRows))
	for _, row := range liveRows {
		out = append(out, RiskTrendPoint{
			Date:           row.Date,
			TotalRiskScore: row.TotalRiskScore,
			OpenCount:      int(row.OpenCount),
			CriticalCount:  int(row.CriticalCount),
		})
	}
	return out, nil
}
