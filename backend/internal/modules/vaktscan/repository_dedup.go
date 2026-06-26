// Copyright (c) 2026 NorvikOps. All rights reserved.
// SPDX-License-Identifier: Elastic-2.0

package vaktscan

import (
	"context"
	"fmt"
	"time"
)

// --- S69-3: SLA Policies ---

// SLASummaryRow is a raw aggregate row from GetSLASummaryRows.
type SLASummaryRow struct {
	Severity  string
	SLAStatus string
	Count     int
}

// ListSLAPolicies returns all SLA policies for an org from vb_sla_policies.
func (r *Repository) ListSLAPolicies(ctx context.Context, orgID string) ([]SLAPolicy, error) {
	rows, err := r.db.Query(ctx, `
		SELECT id::text, org_id::text, severity, remediation_days,
		       notification_advance_days, is_default, created_at, updated_at
		FROM vb_sla_policies
		WHERE org_id = $1::uuid
		ORDER BY remediation_days ASC`,
		orgID,
	)
	if err != nil {
		return nil, fmt.Errorf("list sla policies: %w", err)
	}
	defer rows.Close()

	var out []SLAPolicy
	for rows.Next() {
		var p SLAPolicy
		if err := rows.Scan(&p.ID, &p.OrgID, &p.Severity, &p.RemediationDays,
			&p.NotificationAdvanceDays, &p.IsDefault, &p.CreatedAt, &p.UpdatedAt); err != nil {
			return nil, fmt.Errorf("scan sla policy: %w", err)
		}
		out = append(out, p)
	}
	return out, rows.Err()
}

// CreateSLAPolicy inserts a new SLA policy row.
func (r *Repository) CreateSLAPolicy(ctx context.Context, orgID, severity string, remDays, advDays int, isDefault bool) error {
	_, err := r.db.Exec(ctx, `
		INSERT INTO vb_sla_policies (org_id, severity, remediation_days, notification_advance_days, is_default)
		VALUES ($1::uuid, $2, $3, $4, $5)
		ON CONFLICT (org_id, severity) DO NOTHING`,
		orgID, severity, remDays, advDays, isDefault,
	)
	return err
}

// UpsertSLAPolicy inserts or updates an SLA policy for an org+severity.
func (r *Repository) UpsertSLAPolicy(ctx context.Context, orgID, severity string, remDays, advDays int) error {
	_, err := r.db.Exec(ctx, `
		INSERT INTO vb_sla_policies (org_id, severity, remediation_days, notification_advance_days, is_default)
		VALUES ($1::uuid, $2, $3, $4, false)
		ON CONFLICT (org_id, severity)
		DO UPDATE SET remediation_days = EXCLUDED.remediation_days,
		              notification_advance_days = EXCLUDED.notification_advance_days,
		              is_default = false,
		              updated_at = NOW()`,
		orgID, severity, remDays, advDays,
	)
	return err
}

// DeleteSLAPolicies removes all SLA policies for an org (used by the reset flow).
func (r *Repository) DeleteSLAPolicies(ctx context.Context, orgID string) error {
	_, err := r.db.Exec(ctx, `DELETE FROM vb_sla_policies WHERE org_id = $1::uuid`, orgID)
	return err
}

// GetSLASummaryRows returns aggregate counts of open findings by severity + sla_status.
func (r *Repository) GetSLASummaryRows(ctx context.Context, orgID string) ([]SLASummaryRow, error) {
	rows, err := r.db.Query(ctx, `
		SELECT severity, COALESCE(sla_status, 'on_track'), COUNT(*)
		FROM vb_findings
		WHERE org_id = $1::uuid AND status NOT IN ('resolved', 'wont_fix', 'false_positive')
		GROUP BY severity, sla_status`,
		orgID,
	)
	if err != nil {
		return nil, fmt.Errorf("sla summary rows: %w", err)
	}
	defer rows.Close()

	var out []SLASummaryRow
	for rows.Next() {
		var row SLASummaryRow
		if err := rows.Scan(&row.Severity, &row.SLAStatus, &row.Count); err != nil {
			return nil, fmt.Errorf("scan sla summary row: %w", err)
		}
		out = append(out, row)
	}
	return out, rows.Err()
}

// SLAFinding is a lightweight finding row for SLA processing.
type SLAFinding struct {
	ID        string
	Severity  string
	SLADueAt  *time.Time
	CreatedAt time.Time
}

// ListOpenFindingsWithSLA returns minimal finding data for SLA processing.
func (r *Repository) ListOpenFindingsWithSLA(ctx context.Context, orgID string) ([]SLAFinding, error) {
	rows, err := r.db.Query(ctx, `
		SELECT id::text, severity, sla_due_at, created_at
		FROM vb_findings
		WHERE org_id = $1::uuid
		  AND status NOT IN ('resolved', 'wont_fix', 'false_positive')
		ORDER BY severity, created_at`,
		orgID,
	)
	if err != nil {
		return nil, fmt.Errorf("list open findings with sla: %w", err)
	}
	defer rows.Close()

	var out []SLAFinding
	for rows.Next() {
		var f SLAFinding
		if err := rows.Scan(&f.ID, &f.Severity, &f.SLADueAt, &f.CreatedAt); err != nil {
			return nil, fmt.Errorf("scan sla finding: %w", err)
		}
		out = append(out, f)
	}
	return out, rows.Err()
}

// SetFindingSLADue sets the sla_due_at column for a finding.
func (r *Repository) SetFindingSLADue(ctx context.Context, orgID, findingID string, due time.Time) error {
	_, err := r.db.Exec(ctx, `
		UPDATE vb_findings SET sla_due_at = $1, updated_at = NOW()
		WHERE id = $2::uuid AND org_id = $3::uuid`,
		due, findingID, orgID,
	)
	return err
}

// UpdateFindingSLAStatus sets the sla_status column for a finding.
func (r *Repository) UpdateFindingSLAStatus(ctx context.Context, orgID, findingID, status string) error {
	_, err := r.db.Exec(ctx, `
		UPDATE vb_findings SET sla_status = $1, updated_at = NOW()
		WHERE id = $2::uuid AND org_id = $3::uuid`,
		status, findingID, orgID,
	)
	return err
}
