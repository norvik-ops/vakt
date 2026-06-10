// Copyright (c) 2026 NorvikOps. All rights reserved.
// SPDX-License-Identifier: Elastic-2.0

package vaktcomply

import (
	"context"
	"time"

	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"
)

// CalculateKPIsForOrg computes all 12 ISMS KPIs for a single organisation
// from raw DB queries. Each sub-function is best-effort: DB errors produce nil
// (unknown) values rather than failing the whole snapshot.
func CalculateKPIsForOrg(ctx context.Context, db *pgxpool.Pool, orgID string) KPISnapshot {
	return KPISnapshot{
		SnapshotDate:          time.Now().Format("2006-01-02"),
		ComplianceScore:       calcComplianceScore(ctx, db, orgID),
		OpenCriticalControls:  calcOpenCriticalControls(ctx, db, orgID),
		OpenHighRisks:         calcOpenHighRisks(ctx, db, orgID),
		ResidualRiskAvg:       calcResidualRiskAvg(ctx, db, orgID),
		OpenIncidents:         calcOpenIncidents(ctx, db, orgID),
		IncidentMTTRDays:      calcIncidentMTTR(ctx, db, orgID),
		EvidenceCoverage:      calcEvidenceCoverage(ctx, db, orgID),
		ExpiringEvidenceCount: calcExpiringEvidence(ctx, db, orgID),
		FindingSLACompliance:  nil, // deferred — requires vb_findings schema analysis
		OpenMajorNCs:          nil, // deferred — ck_capas lacks nc_classification column
		SuppliersOverduePct:   nil, // deferred — complex supplier assessment logic
		PhishingClickRate:     nil, // deferred — awareness module data not yet linked
	}
}

// ── helpers ───────────────────────────────────────────────────────────────────

func numericToFloat64Ptr(n pgtype.Numeric) *float64 {
	if !n.Valid {
		return nil
	}
	f, err := n.Float64Value()
	if err != nil || !f.Valid {
		return nil
	}
	v := f.Float64
	return &v
}

// ── KPI sub-calculators ───────────────────────────────────────────────────────

// calcComplianceScore returns the percentage of controls with status = 'implemented'.
func calcComplianceScore(ctx context.Context, db *pgxpool.Pool, orgID string) *float64 {
	if db == nil {
		return nil
	}
	var val pgtype.Numeric
	_ = db.QueryRow(ctx, `
		SELECT CASE WHEN COUNT(*) > 0
			THEN ROUND(100.0 * COUNT(CASE WHEN status = 'implemented' THEN 1 END)::numeric / COUNT(*), 2)
			ELSE NULL END
		FROM ck_controls WHERE org_id = $1`, orgID).Scan(&val)
	return numericToFloat64Ptr(val)
}

// calcOpenCriticalControls returns the count of non-implemented controls with low maturity.
func calcOpenCriticalControls(ctx context.Context, db *pgxpool.Pool, orgID string) *int {
	if db == nil {
		return nil
	}
	var val pgtype.Int4
	_ = db.QueryRow(ctx, `
		SELECT COUNT(*)::int FROM ck_controls
		WHERE org_id = $1 AND status != 'implemented' AND maturity_score < 2`, orgID).Scan(&val)
	if !val.Valid {
		return nil
	}
	v := int(val.Int32)
	return &v
}

// calcOpenHighRisks returns the count of high-severity risks (risk_score >= 15)
// that have not been accepted or closed.
func calcOpenHighRisks(ctx context.Context, db *pgxpool.Pool, orgID string) *int {
	if db == nil {
		return nil
	}
	var val pgtype.Int4
	_ = db.QueryRow(ctx, `
		SELECT COUNT(*)::int FROM ck_risks
		WHERE org_id = $1
		  AND risk_score >= 15
		  AND treatment != 'accept'
		  AND status NOT IN ('accepted','closed')`, orgID).Scan(&val)
	if !val.Valid {
		return nil
	}
	v := int(val.Int32)
	return &v
}

// calcResidualRiskAvg returns the average residual risk score (impact × likelihood).
func calcResidualRiskAvg(ctx context.Context, db *pgxpool.Pool, orgID string) *float64 {
	if db == nil {
		return nil
	}
	var val pgtype.Numeric
	_ = db.QueryRow(ctx, `
		SELECT AVG(residual_impact * residual_likelihood)::numeric
		FROM ck_risks
		WHERE org_id = $1
		  AND residual_impact IS NOT NULL
		  AND residual_likelihood IS NOT NULL`, orgID).Scan(&val)
	return numericToFloat64Ptr(val)
}

// calcOpenIncidents returns the count of incidents not yet resolved or closed.
func calcOpenIncidents(ctx context.Context, db *pgxpool.Pool, orgID string) *int {
	if db == nil {
		return nil
	}
	var val pgtype.Int4
	_ = db.QueryRow(ctx, `
		SELECT COUNT(*)::int FROM ck_incidents
		WHERE org_id = $1
		  AND status NOT IN ('resolved','closed')`, orgID).Scan(&val)
	if !val.Valid {
		return nil
	}
	v := int(val.Int32)
	return &v
}

// calcIncidentMTTR returns average mean-time-to-resolve in days for incidents
// resolved or closed within the last 90 days (uses resolved_at column).
func calcIncidentMTTR(ctx context.Context, db *pgxpool.Pool, orgID string) *float64 {
	if db == nil {
		return nil
	}
	var val pgtype.Numeric
	_ = db.QueryRow(ctx, `
		SELECT AVG(EXTRACT(EPOCH FROM (resolved_at - created_at)) / 86400.0)::numeric
		FROM ck_incidents
		WHERE org_id = $1
		  AND status IN ('resolved','closed')
		  AND resolved_at IS NOT NULL
		  AND created_at >= NOW() - INTERVAL '90 days'`, orgID).Scan(&val)
	return numericToFloat64Ptr(val)
}

// calcEvidenceCoverage returns the percentage of controls that have at least one
// evidence record.
func calcEvidenceCoverage(ctx context.Context, db *pgxpool.Pool, orgID string) *float64 {
	if db == nil {
		return nil
	}
	var val pgtype.Numeric
	_ = db.QueryRow(ctx, `
		SELECT CASE WHEN total > 0 THEN ROUND(100.0 * with_evidence / total, 2) ELSE NULL END
		FROM (
			SELECT COUNT(*) AS total,
			       COUNT(DISTINCT e.control_id) AS with_evidence
			FROM ck_controls c
			LEFT JOIN ck_evidence e
			       ON e.control_id = c.id AND e.org_id = c.org_id
			WHERE c.org_id = $1
		) sub`, orgID).Scan(&val)
	return numericToFloat64Ptr(val)
}

// calcExpiringEvidence returns the count of evidence items expiring within the
// next 30 days (but not already expired).
func calcExpiringEvidence(ctx context.Context, db *pgxpool.Pool, orgID string) *int {
	if db == nil {
		return nil
	}
	var val pgtype.Int4
	_ = db.QueryRow(ctx, `
		SELECT COUNT(*)::int FROM ck_evidence
		WHERE org_id = $1
		  AND expires_at IS NOT NULL
		  AND expires_at < NOW() + INTERVAL '30 days'
		  AND expires_at > NOW()`, orgID).Scan(&val)
	if !val.Valid {
		return nil
	}
	v := int(val.Int32)
	return &v
}
