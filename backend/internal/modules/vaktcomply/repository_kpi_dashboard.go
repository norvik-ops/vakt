// Copyright (c) 2026 NorvikOps. All rights reserved.
// SPDX-License-Identifier: Elastic-2.0

package vaktcomply

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
)

// UpsertKPISnapshot inserts or updates the KPI snapshot for (org_id, snapshot_date).
func (r *Repository) UpsertKPISnapshot(ctx context.Context, orgID string, snap KPISnapshot) error {
	_, err := r.db.Exec(ctx, `
		INSERT INTO ck_isms_kpi_snapshots (
			org_id,
			snapshot_date,
			kpi_compliance_score,
			kpi_open_critical_controls,
			kpi_open_high_risks,
			kpi_residual_risk_avg,
			kpi_open_incidents,
			kpi_incident_mttr_days,
			kpi_evidence_coverage,
			kpi_expiring_evidence_count,
			kpi_finding_sla_compliance,
			kpi_open_major_ncs,
			kpi_suppliers_overdue_pct,
			kpi_phishing_click_rate
		) VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14)
		ON CONFLICT (org_id, snapshot_date) DO UPDATE SET
			kpi_compliance_score        = EXCLUDED.kpi_compliance_score,
			kpi_open_critical_controls  = EXCLUDED.kpi_open_critical_controls,
			kpi_open_high_risks         = EXCLUDED.kpi_open_high_risks,
			kpi_residual_risk_avg       = EXCLUDED.kpi_residual_risk_avg,
			kpi_open_incidents          = EXCLUDED.kpi_open_incidents,
			kpi_incident_mttr_days      = EXCLUDED.kpi_incident_mttr_days,
			kpi_evidence_coverage       = EXCLUDED.kpi_evidence_coverage,
			kpi_expiring_evidence_count = EXCLUDED.kpi_expiring_evidence_count,
			kpi_finding_sla_compliance  = EXCLUDED.kpi_finding_sla_compliance,
			kpi_open_major_ncs          = EXCLUDED.kpi_open_major_ncs,
			kpi_suppliers_overdue_pct   = EXCLUDED.kpi_suppliers_overdue_pct,
			kpi_phishing_click_rate     = EXCLUDED.kpi_phishing_click_rate`,
		orgID,
		snap.SnapshotDate,
		float64PtrToNumeric(snap.ComplianceScore),
		intPtrToInt4(snap.OpenCriticalControls),
		intPtrToInt4(snap.OpenHighRisks),
		float64PtrToNumeric(snap.ResidualRiskAvg),
		intPtrToInt4(snap.OpenIncidents),
		float64PtrToNumeric(snap.IncidentMTTRDays),
		float64PtrToNumeric(snap.EvidenceCoverage),
		intPtrToInt4(snap.ExpiringEvidenceCount),
		float64PtrToNumeric(snap.FindingSLACompliance),
		intPtrToInt4(snap.OpenMajorNCs),
		float64PtrToNumeric(snap.SuppliersOverduePct),
		float64PtrToNumeric(snap.PhishingClickRate),
	)
	if err != nil {
		return fmt.Errorf("upsert kpi snapshot: %w", err)
	}
	return nil
}

// GetLatestKPISnapshot returns the most recent KPI snapshot for the organisation.
// Returns nil, nil when no snapshot exists yet.
func (r *Repository) GetLatestKPISnapshot(ctx context.Context, orgID string) (*KPISnapshot, error) {
	row := r.db.QueryRow(ctx, `
		SELECT id::text, org_id::text, snapshot_date::text,
		       kpi_compliance_score, kpi_open_critical_controls, kpi_open_high_risks,
		       kpi_residual_risk_avg, kpi_open_incidents, kpi_incident_mttr_days,
		       kpi_evidence_coverage, kpi_expiring_evidence_count, kpi_finding_sla_compliance,
		       kpi_open_major_ncs, kpi_suppliers_overdue_pct, kpi_phishing_click_rate,
		       created_at
		FROM ck_isms_kpi_snapshots
		WHERE org_id = $1
		ORDER BY snapshot_date DESC
		LIMIT 1`, orgID)

	snap, err := scanKPISnapshot(row)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, nil
		}
		return nil, fmt.Errorf("get latest kpi snapshot: %w", err)
	}
	return &snap, nil
}

// ListKPISnapshots returns all snapshots for the organisation since the given time,
// ordered by snapshot_date ascending (oldest first, for chart rendering).
func (r *Repository) ListKPISnapshots(ctx context.Context, orgID string, since time.Time) ([]KPISnapshot, error) {
	rows, err := r.db.Query(ctx, `
		SELECT id::text, org_id::text, snapshot_date::text,
		       kpi_compliance_score, kpi_open_critical_controls, kpi_open_high_risks,
		       kpi_residual_risk_avg, kpi_open_incidents, kpi_incident_mttr_days,
		       kpi_evidence_coverage, kpi_expiring_evidence_count, kpi_finding_sla_compliance,
		       kpi_open_major_ncs, kpi_suppliers_overdue_pct, kpi_phishing_click_rate,
		       created_at
		FROM ck_isms_kpi_snapshots
		WHERE org_id = $1 AND snapshot_date >= $2::date
		ORDER BY snapshot_date ASC`, orgID, since.Format("2006-01-02"))
	if err != nil {
		return nil, fmt.Errorf("list kpi snapshots: %w", err)
	}
	defer rows.Close()

	var out []KPISnapshot
	for rows.Next() {
		snap, err := scanKPISnapshot(rows)
		if err != nil {
			return nil, fmt.Errorf("scan kpi snapshot: %w", err)
		}
		out = append(out, snap)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("list kpi snapshots rows: %w", err)
	}
	return out, nil
}

// ── scan helper ───────────────────────────────────────────────────────────────

// rowScanner is satisfied by both pgx.Row and pgx.Rows.
type rowScanner interface {
	Scan(dest ...any) error
}

func scanKPISnapshot(row rowScanner) (KPISnapshot, error) {
	var (
		snap                                                                                  KPISnapshot
		complianceScore, residualRiskAvg, mttr, evidenceCoverage, slaCmpl, suppliersPct, phish pgtype.Numeric
		openCritical, openHighRisks, openIncidents, expiringEvidence, openMajorNCs           pgtype.Int4
		createdAt                                                                             pgtype.Timestamptz
	)
	err := row.Scan(
		&snap.ID, &snap.OrgID, &snap.SnapshotDate,
		&complianceScore, &openCritical, &openHighRisks,
		&residualRiskAvg, &openIncidents, &mttr,
		&evidenceCoverage, &expiringEvidence, &slaCmpl,
		&openMajorNCs, &suppliersPct, &phish,
		&createdAt,
	)
	if err != nil {
		return KPISnapshot{}, err
	}

	snap.ComplianceScore = numericToFloat64Ptr(complianceScore)
	snap.OpenCriticalControls = int4PtrToIntPtr(openCritical)
	snap.OpenHighRisks = int4PtrToIntPtr(openHighRisks)
	snap.ResidualRiskAvg = numericToFloat64Ptr(residualRiskAvg)
	snap.OpenIncidents = int4PtrToIntPtr(openIncidents)
	snap.IncidentMTTRDays = numericToFloat64Ptr(mttr)
	snap.EvidenceCoverage = numericToFloat64Ptr(evidenceCoverage)
	snap.ExpiringEvidenceCount = int4PtrToIntPtr(expiringEvidence)
	snap.FindingSLACompliance = numericToFloat64Ptr(slaCmpl)
	snap.OpenMajorNCs = int4PtrToIntPtr(openMajorNCs)
	snap.SuppliersOverduePct = numericToFloat64Ptr(suppliersPct)
	snap.PhishingClickRate = numericToFloat64Ptr(phish)
	snap.CreatedAt = ckTsToTime(createdAt)

	return snap, nil
}

// ── conversion helpers ────────────────────────────────────────────────────────

func float64PtrToNumeric(v *float64) pgtype.Numeric {
	if v == nil {
		return pgtype.Numeric{}
	}
	var n pgtype.Numeric
	if err := n.Scan(*v); err != nil {
		return pgtype.Numeric{}
	}
	return n
}

func intPtrToInt4(v *int) pgtype.Int4 {
	if v == nil {
		return pgtype.Int4{}
	}
	return pgtype.Int4{Int32: int32(*v), Valid: true}
}

func int4PtrToIntPtr(v pgtype.Int4) *int {
	if !v.Valid {
		return nil
	}
	i := int(v.Int32)
	return &i
}
