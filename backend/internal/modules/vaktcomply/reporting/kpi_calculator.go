// Copyright (c) 2026 NorvikOps. All rights reserved.
// SPDX-License-Identifier: Elastic-2.0

package reporting

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
		FindingSLACompliance:  calcFindingSLACompliance(ctx, db, orgID),
		OpenMajorNCs:          calcOpenMajorNCs(ctx, db, orgID),
		// S121-F5 (T5): both previously-nil KPIs are now computed —
		// SuppliersOverduePct from ck_suppliers.next_assessment_due (migration 176)
		// and PhishingClickRate from sr_events over completed campaigns.
		SuppliersOverduePct: calcSuppliersOverduePct(ctx, db, orgID),
		PhishingClickRate:   calcPhishingClickRate(ctx, db, orgID),
	}
}

// phishingClickWindowDays bounds the phishing KPI to a rolling period so a
// long-past campaign cannot dominate the current figure. This is the
// "campaign-period scoping" the KPI was previously blocked on.
const phishingClickWindowDays = 365

// calcPhishingClickRate returns the share of phished recipients who clicked,
// across the org's COMPLETED campaigns in the last phishingClickWindowDays.
//
// Denominator: the recipients of those campaigns (targets of the campaign's
// group — the same derivation vaktaware uses for its per-campaign rate).
// Numerator: DISTINCT (campaign, target) click events, so a recipient clicking
// twice cannot push the rate above 100 %. Returns nil (unknown) when the org ran
// no completed campaign with recipients in the window — the dashboard then hides
// the KPI rather than showing a misleading 0 %.
func calcPhishingClickRate(ctx context.Context, db *pgxpool.Pool, orgID string) *float64 {
	if db == nil {
		return nil
	}
	var val pgtype.Numeric
	_ = db.QueryRow(ctx, `
		WITH camp AS (
			SELECT c.id,
			       COALESCE((SELECT COUNT(*) FROM sr_targets t
			                  WHERE t.group_id = c.group_id
			                    AND t.org_id = $1::uuid), 0) AS recipients
			FROM sr_campaigns c
			WHERE c.org_id = $1::uuid
			  AND c.status = 'completed'
			  AND c.completed_at >= NOW() - make_interval(days => $2::int)
		)
		SELECT CASE WHEN COALESCE(SUM(camp.recipients), 0) > 0
			THEN ROUND(100.0 * (
				SELECT COUNT(DISTINCT (e.campaign_id, e.target_id))
				FROM sr_events e
				JOIN camp ON camp.id = e.campaign_id
				WHERE e.org_id = $1::uuid
				  AND e.type = 'click'
				  AND e.target_id IS NOT NULL
			)::numeric / SUM(camp.recipients), 2)
			ELSE NULL END
		FROM camp`, orgID, phishingClickWindowDays).Scan(&val)
	return numericToFloat64Ptr(val)
}

// calcSuppliersOverduePct returns the percentage of assessable suppliers whose
// next assessment is past due. The denominator is suppliers that have a due date
// set (next_assessment_due IS NOT NULL); if none do, the KPI is unknown (nil).
func calcSuppliersOverduePct(ctx context.Context, db *pgxpool.Pool, orgID string) *float64 {
	if db == nil {
		return nil
	}
	var val pgtype.Numeric
	_ = db.QueryRow(ctx, `
		SELECT CASE WHEN COUNT(*) FILTER (WHERE next_assessment_due IS NOT NULL) > 0
			THEN ROUND(
				100.0 * COUNT(*) FILTER (WHERE next_assessment_due < CURRENT_DATE)::numeric
				/ COUNT(*) FILTER (WHERE next_assessment_due IS NOT NULL), 2)
			ELSE NULL END
		FROM ck_suppliers WHERE org_id = $1`, orgID).Scan(&val)
	return numericToFloat64Ptr(val)
}

// numericToFloat64Ptr converts a pgtype.Numeric to *float64 (nil on NULL or error).
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

// calcFindingSLACompliance returns the percentage of SLA-tracked findings
// (sla_status IS NOT NULL) that have not breached their SLA.
// Compliant = on_track | at_risk | resolved_on_time; non-compliant = overdue | resolved_late.
// NIS2 Art. 21f: organisations must measure remediation effectiveness.
func calcFindingSLACompliance(ctx context.Context, db *pgxpool.Pool, orgID string) *float64 {
	if db == nil {
		return nil
	}
	var val pgtype.Numeric
	_ = db.QueryRow(ctx, `
		SELECT CASE WHEN COUNT(*) > 0
			THEN ROUND(
				100.0 * COUNT(CASE WHEN sla_status NOT IN ('overdue','resolved_late') THEN 1 END)::numeric
				/ COUNT(*), 2)
			ELSE NULL END
		FROM vb_findings
		WHERE org_id = $1
		  AND sla_status IS NOT NULL`, orgID).Scan(&val)
	return numericToFloat64Ptr(val)
}

// calcOpenMajorNCs returns the count of open major non-conformities from internal
// or external audits — CAPAs with nc_classification = 'major_nc' that are not
// yet verified or closed. ISO 27001:2022 clause 10.1 requires tracking and
// closure of major NCs within agreed timescales.
func calcOpenMajorNCs(ctx context.Context, db *pgxpool.Pool, orgID string) *int {
	if db == nil {
		return nil
	}
	var val pgtype.Int4
	_ = db.QueryRow(ctx, `
		SELECT COUNT(*)::int FROM ck_capas
		WHERE org_id = $1
		  AND nc_classification = 'major_nc'
		  AND status NOT IN ('verified','closed')`, orgID).Scan(&val)
	if !val.Valid {
		return nil
	}
	v := int(val.Int32)
	return &v
}
