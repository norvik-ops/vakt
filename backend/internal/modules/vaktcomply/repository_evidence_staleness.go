// Copyright (c) 2026 NorvikOps. All rights reserved.
// SPDX-License-Identifier: Elastic-2.0

package vaktcomply

import (
	"context"
	"fmt"
	"time"
)

// UpdateEvidenceStaleness refreshes evidence_status on all controls for an org.
// Returns the number of controls whose status changed.
func (r *Repository) UpdateEvidenceStaleness(ctx context.Context, orgID string) (int, error) {
	tag, err := r.db.Exec(ctx,
		`UPDATE ck_controls c
		    SET evidence_status = CASE
		        WHEN c.not_applicable = true THEN 'na'
		        WHEN c.evidence_max_age_days IS NULL THEN
		            CASE WHEN e.newest IS NULL THEN 'missing' ELSE 'ok' END
		        WHEN e.newest IS NULL THEN 'missing'
		        WHEN NOW() - e.newest > (c.evidence_max_age_days * INTERVAL '1 day') THEN 'stale'
		        ELSE 'ok'
		    END,
		    evidence_last_updated = e.newest,
		    evidence_expires_at = CASE
		        WHEN c.evidence_max_age_days IS NOT NULL AND e.newest IS NOT NULL
		        THEN e.newest + (c.evidence_max_age_days * INTERVAL '1 day')
		        ELSE NULL
		    END
		   FROM (
		        SELECT control_id, MAX(created_at) AS newest
		          FROM ck_evidence
		         WHERE org_id = $1::uuid
		         GROUP BY control_id
		   ) e
		  WHERE c.id = e.control_id
		    AND c.org_id = $1::uuid`,
		orgID,
	)
	if err != nil {
		return 0, fmt.Errorf("update evidence staleness: %w", err)
	}

	// Also mark controls with no evidence as 'missing'
	if _, err := r.db.Exec(ctx,
		`UPDATE ck_controls
		    SET evidence_status = CASE WHEN not_applicable = true THEN 'na' ELSE 'missing' END,
		        evidence_last_updated = NULL,
		        evidence_expires_at = NULL
		  WHERE org_id = $1::uuid
		    AND id NOT IN (
		        SELECT DISTINCT control_id FROM ck_evidence WHERE org_id = $1::uuid
		    )`,
		orgID,
	); err != nil {
		return int(tag.RowsAffected()), fmt.Errorf("mark missing controls: %w", err)
	}

	return int(tag.RowsAffected()), nil
}

// GetComplianceScore returns aggregated counts for the compliance score.
func (r *Repository) GetComplianceScore(ctx context.Context, orgID string) (*ComplianceScore, error) {
	row := r.db.QueryRow(ctx,
		`SELECT
		    COUNT(*)                                     AS total,
		    COUNT(*) FILTER (WHERE evidence_status = 'ok')      AS ok_count,
		    COUNT(*) FILTER (WHERE evidence_status = 'stale')   AS stale_count,
		    COUNT(*) FILTER (WHERE evidence_status = 'missing') AS missing_count,
		    COUNT(*) FILTER (WHERE evidence_status = 'na')      AS na_count
		   FROM ck_controls
		  WHERE org_id = $1::uuid`,
		orgID,
	)

	var s ComplianceScore
	if err := row.Scan(&s.TotalControls, &s.OkCount, &s.StaleCount, &s.MissingCount, &s.NACount); err != nil {
		return nil, fmt.Errorf("get compliance score: %w", err)
	}

	denominator := s.TotalControls - s.NACount
	if denominator > 0 {
		s.ScorePct = float64(s.OkCount) / float64(denominator) * 100
	}
	s.AsOf = time.Now().UTC().Format(time.RFC3339)
	return &s, nil
}

// SetControlMaxAge sets the evidence_max_age_days for a specific control.
func (r *Repository) SetControlMaxAge(ctx context.Context, orgID, controlID string, maxAgeDays *int) error {
	_, err := r.db.Exec(ctx,
		`UPDATE ck_controls SET evidence_max_age_days = $3 WHERE id = $1::uuid AND org_id = $2::uuid`,
		controlID, orgID, maxAgeDays,
	)
	return err
}

// ListStaleControls returns all controls with evidence_status = 'stale', oldest evidence first.
func (r *Repository) ListStaleControls(ctx context.Context, orgID string) ([]Control, error) {
	rows, err := r.db.Query(ctx,
		`SELECT id::text, framework_id::text, org_id::text, control_id, title,
		        COALESCE(description,''), domain, evidence_type, weight,
		        COALESCE(evidence_status,'missing'),
		        evidence_max_age_days, evidence_last_updated, evidence_expires_at
		   FROM ck_controls
		  WHERE org_id = $1::uuid AND evidence_status = 'stale'
		  ORDER BY evidence_expires_at ASC NULLS LAST`,
		orgID,
	)
	if err != nil {
		return nil, fmt.Errorf("list stale controls: %w", err)
	}
	defer rows.Close()

	var out []Control
	for rows.Next() {
		var c Control
		var expiresAt *time.Time
		var maxAge *int
		var lastUpdated *time.Time
		if err := rows.Scan(
			&c.ID, &c.FrameworkID, &c.OrgID, &c.ControlID, &c.Title,
			&c.Description, &c.Domain, &c.EvidenceType, &c.Weight,
			&c.EvidenceStatus,
			&maxAge, &lastUpdated, &expiresAt,
		); err != nil {
			return nil, fmt.Errorf("scan stale control: %w", err)
		}
		c.EvidenceMaxAgeDays = maxAge
		c.EvidenceExpiresAt = expiresAt
		out = append(out, c)
	}
	return out, rows.Err()
}
