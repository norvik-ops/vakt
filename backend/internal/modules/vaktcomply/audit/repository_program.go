// Copyright (c) 2026 NorvikOps. All rights reserved.
// SPDX-License-Identifier: Elastic-2.0

package audit

import (
	"context"
	"time"

	"github.com/jackc/pgx/v5/pgtype"
)

// ── Audit Plans ──────────────────────────────────────────────────────────────

// ListAuditPlans returns all audit plans for the org.
func (r *Repository) ListAuditPlans(ctx context.Context, orgID string) ([]AuditPlan, error) {
	rows, err := r.db.Query(ctx, `
		SELECT id, org_id, year, COALESCE(scope,''), responsible_id,
		       status, COALESCE(notes,''), created_at, updated_at
		FROM ck_audit_plans WHERE org_id = $1 ORDER BY year DESC`, orgID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var plans []AuditPlan
	for rows.Next() {
		var p AuditPlan
		var responsible pgtype.Text
		var createdAt, updatedAt time.Time
		if err := rows.Scan(&p.ID, &p.OrgID, &p.Year, &p.Scope, &responsible,
			&p.Status, &p.Notes, &createdAt, &updatedAt); err != nil {
			return nil, err
		}
		p.CreatedAt = createdAt.Format(time.RFC3339)
		p.UpdatedAt = updatedAt.Format(time.RFC3339)
		if responsible.Valid {
			p.ResponsibleID = &responsible.String
		}
		plans = append(plans, p)
	}
	return plans, rows.Err()
}

// CreateAuditPlan inserts a new audit plan.
func (r *Repository) CreateAuditPlan(ctx context.Context, orgID string, in CreateAuditPlanInput) (*AuditPlan, error) {
	var p AuditPlan
	var responsible pgtype.Text
	var createdAt, updatedAt time.Time
	err := r.db.QueryRow(ctx, `
		INSERT INTO ck_audit_plans (org_id, year, scope, responsible_id, notes)
		VALUES ($1, $2, NULLIF($3,''), $4, NULLIF($5,''))
		RETURNING id, org_id, year, COALESCE(scope,''), responsible_id, status, COALESCE(notes,''), created_at, updated_at`,
		orgID, in.Year, in.Scope, in.ResponsibleID, in.Notes,
	).Scan(&p.ID, &p.OrgID, &p.Year, &p.Scope, &responsible, &p.Status, &p.Notes, &createdAt, &updatedAt)
	if err != nil {
		return nil, err
	}
	p.CreatedAt = createdAt.Format(time.RFC3339)
	p.UpdatedAt = updatedAt.Format(time.RFC3339)
	if responsible.Valid {
		p.ResponsibleID = &responsible.String
	}
	return &p, nil
}

// UpdateAuditPlan modifies an existing audit plan.
func (r *Repository) UpdateAuditPlan(ctx context.Context, orgID, id string, in CreateAuditPlanInput) (*AuditPlan, error) {
	var p AuditPlan
	var responsible pgtype.Text
	var createdAt, updatedAt time.Time
	err := r.db.QueryRow(ctx, `
		UPDATE ck_audit_plans SET
			year           = $1,
			scope          = NULLIF($2,''),
			responsible_id = $3,
			notes          = NULLIF($4,''),
			updated_at     = NOW()
		WHERE org_id = $5 AND id = $6
		RETURNING id, org_id, year, COALESCE(scope,''), responsible_id, status, COALESCE(notes,''), created_at, updated_at`,
		in.Year, in.Scope, in.ResponsibleID, in.Notes, orgID, id,
	).Scan(&p.ID, &p.OrgID, &p.Year, &p.Scope, &responsible, &p.Status, &p.Notes, &createdAt, &updatedAt)
	if err != nil {
		return nil, err
	}
	p.CreatedAt = createdAt.Format(time.RFC3339)
	p.UpdatedAt = updatedAt.Format(time.RFC3339)
	if responsible.Valid {
		p.ResponsibleID = &responsible.String
	}
	return &p, nil
}

// ── Individual Audits ────────────────────────────────────────────────────────

// ListAuditProgramAudits returns all audits for the org.
func (r *Repository) ListAuditProgramAudits(ctx context.Context, orgID string) ([]AuditProgramAudit, error) {
	rows, err := r.db.Query(ctx, `
		SELECT a.id, a.org_id, a.audit_plan_id, a.title, a.audit_type, a.scope,
		       COALESCE(a.methodology,'combined'), a.planned_date::text,
		       a.actual_date::text, a.lead_auditor_id, COALESCE(a.auditor_ids,'{}'),
		       a.supplier_id, a.status, COALESCE(a.audit_report,''),
		       COUNT(f.id) AS findings_count,
		       a.created_at, a.updated_at
		FROM ck_audit_program_audits a
		LEFT JOIN ck_audit_program_findings f ON f.audit_id = a.id
		WHERE a.org_id = $1
		GROUP BY a.id
		ORDER BY a.planned_date DESC`, orgID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var audits []AuditProgramAudit
	for rows.Next() {
		a, err := scanAuditProgramAudit(rows)
		if err != nil {
			return nil, err
		}
		audits = append(audits, *a)
	}
	return audits, rows.Err()
}

// GetAuditProgramAudit returns a single audit by ID.
func (r *Repository) GetAuditProgramAudit(ctx context.Context, orgID, id string) (*AuditProgramAudit, error) {
	row := r.db.QueryRow(ctx, `
		SELECT a.id, a.org_id, a.audit_plan_id, a.title, a.audit_type, a.scope,
		       COALESCE(a.methodology,'combined'), a.planned_date::text,
		       a.actual_date::text, a.lead_auditor_id, COALESCE(a.auditor_ids,'{}'),
		       a.supplier_id, a.status, COALESCE(a.audit_report,''),
		       COUNT(f.id) AS findings_count,
		       a.created_at, a.updated_at
		FROM ck_audit_program_audits a
		LEFT JOIN ck_audit_program_findings f ON f.audit_id = a.id
		WHERE a.org_id = $1 AND a.id = $2
		GROUP BY a.id`, orgID, id)
	return scanAuditProgramAudit(row)
}

type auditScanner interface {
	Scan(dest ...any) error
}

func scanAuditProgramAudit(row auditScanner) (*AuditProgramAudit, error) {
	var a AuditProgramAudit
	var planID, leadAuditor, supplierID pgtype.Text
	var actualDate pgtype.Text
	var createdAt, updatedAt time.Time
	if err := row.Scan(
		&a.ID, &a.OrgID, &planID, &a.Title, &a.AuditType, &a.Scope,
		&a.Methodology, &a.PlannedDate, &actualDate, &leadAuditor, &a.AuditorIDs,
		&supplierID, &a.Status, &a.AuditReport, &a.FindingsCount,
		&createdAt, &updatedAt,
	); err != nil {
		return nil, err
	}
	a.CreatedAt = createdAt.Format(time.RFC3339)
	a.UpdatedAt = updatedAt.Format(time.RFC3339)
	if planID.Valid {
		a.AuditPlanID = &planID.String
	}
	if actualDate.Valid && actualDate.String != "" {
		a.ActualDate = &actualDate.String
	}
	if leadAuditor.Valid {
		a.LeadAuditorID = &leadAuditor.String
	}
	if supplierID.Valid {
		a.SupplierID = &supplierID.String
	}
	if a.AuditorIDs == nil {
		a.AuditorIDs = []string{}
	}
	return &a, nil
}

// CreateAuditProgramAudit inserts a new audit.
func (r *Repository) CreateAuditProgramAudit(ctx context.Context, orgID string, in CreateAuditProgramAuditInput) (*AuditProgramAudit, error) {
	methodology := in.Methodology
	if methodology == "" {
		methodology = "combined"
	}
	auditorIDs := in.AuditorIDs
	if auditorIDs == nil {
		auditorIDs = []string{}
	}
	var id string
	err := r.db.QueryRow(ctx, `
		INSERT INTO ck_audit_program_audits
			(org_id, audit_plan_id, title, audit_type, scope, methodology, planned_date, lead_auditor_id, auditor_ids, supplier_id)
		VALUES ($1, $2, $3, $4, $5, $6, $7::date, $8, $9, $10)
		RETURNING id`,
		orgID, in.AuditPlanID, in.Title, in.AuditType, in.Scope, methodology, in.PlannedDate,
		in.LeadAuditorID, auditorIDs, in.SupplierID,
	).Scan(&id)
	if err != nil {
		return nil, err
	}
	return r.GetAuditProgramAudit(ctx, orgID, id)
}

// UpdateAuditProgramAudit modifies an existing audit.
func (r *Repository) UpdateAuditProgramAudit(ctx context.Context, orgID, id string, in CreateAuditProgramAuditInput) (*AuditProgramAudit, error) {
	methodology := in.Methodology
	if methodology == "" {
		methodology = "combined"
	}
	auditorIDs := in.AuditorIDs
	if auditorIDs == nil {
		auditorIDs = []string{}
	}
	_, err := r.db.Exec(ctx, `
		UPDATE ck_audit_program_audits SET
			audit_plan_id   = $1,
			title           = $2,
			audit_type      = $3,
			scope           = $4,
			methodology     = $5,
			planned_date    = $6::date,
			lead_auditor_id = $7,
			auditor_ids     = $8,
			supplier_id     = $9,
			updated_at      = NOW()
		WHERE org_id = $10 AND id = $11`,
		in.AuditPlanID, in.Title, in.AuditType, in.Scope, methodology, in.PlannedDate,
		in.LeadAuditorID, auditorIDs, in.SupplierID, orgID, id,
	)
	if err != nil {
		return nil, err
	}
	return r.GetAuditProgramAudit(ctx, orgID, id)
}

// CompleteAuditProgramAudit sets status=completed with report and actual date.
func (r *Repository) CompleteAuditProgramAudit(ctx context.Context, orgID, id string, in CompleteAuditInput) error {
	_, err := r.db.Exec(ctx, `
		UPDATE ck_audit_program_audits SET
			status       = 'completed',
			audit_report = $1,
			actual_date  = $2::date,
			updated_at   = NOW()
		WHERE org_id = $3 AND id = $4`,
		in.AuditReport, in.ActualDate, orgID, id,
	)
	return err
}

// ── Findings ─────────────────────────────────────────────────────────────────

// ListAuditFindings returns all findings for a given audit.
func (r *Repository) ListAuditFindings(ctx context.Context, orgID, auditID string) ([]AuditFinding, error) {
	rows, err := r.db.Query(ctx, `
		SELECT id, org_id, audit_id, title, description, severity,
		       affected_control_id, capa_id, created_at
		FROM ck_audit_program_findings
		WHERE org_id = $1 AND audit_id = $2
		ORDER BY created_at`, orgID, auditID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var findings []AuditFinding
	for rows.Next() {
		var f AuditFinding
		var controlID, capaID pgtype.Text
		var createdAt time.Time
		if err := rows.Scan(&f.ID, &f.OrgID, &f.AuditID, &f.Title, &f.Description,
			&f.Severity, &controlID, &capaID, &createdAt); err != nil {
			return nil, err
		}
		f.CreatedAt = createdAt.Format(time.RFC3339)
		if controlID.Valid {
			f.AffectedControlID = &controlID.String
		}
		if capaID.Valid {
			f.CAPAid = &capaID.String
		}
		findings = append(findings, f)
	}
	return findings, rows.Err()
}

// CreateAuditFinding inserts a new finding.
func (r *Repository) CreateAuditFinding(ctx context.Context, orgID, auditID string, in CreateAuditFindingInput) (*AuditFinding, error) {
	var f AuditFinding
	var controlID, capaID pgtype.Text
	var createdAt time.Time
	err := r.db.QueryRow(ctx, `
		INSERT INTO ck_audit_program_findings (org_id, audit_id, title, description, severity, affected_control_id)
		VALUES ($1, $2, $3, $4, $5, $6)
		RETURNING id, org_id, audit_id, title, description, severity, affected_control_id, capa_id, created_at`,
		orgID, auditID, in.Title, in.Description, in.Severity, in.AffectedControlID,
	).Scan(&f.ID, &f.OrgID, &f.AuditID, &f.Title, &f.Description, &f.Severity, &controlID, &capaID, &createdAt)
	if err != nil {
		return nil, err
	}
	f.CreatedAt = createdAt.Format(time.RFC3339)
	if controlID.Valid {
		f.AffectedControlID = &controlID.String
	}
	if capaID.Valid {
		f.CAPAid = &capaID.String
	}
	return &f, nil
}

// CreateCAPAFromAuditFinding creates a CAPA entry linked to an audit finding.
// Returns the new CAPA ID.
func (r *Repository) CreateCAPAFromAuditFinding(ctx context.Context, orgID, findingID, title, severity string) (string, error) {
	var capaID string
	err := r.db.QueryRow(ctx, `
		INSERT INTO ck_capas (org_id, title, nc_classification, source, source_ref, status)
		VALUES ($1, $2, $3, 'internal_audit', $4, 'open')
		RETURNING id`,
		orgID, "Audit-Befund: "+title, severity, findingID,
	).Scan(&capaID)
	return capaID, err
}

// SetAuditFindingCAPAID links a finding to its auto-created CAPA.
func (r *Repository) SetAuditFindingCAPAID(ctx context.Context, findingID, capaID string) error {
	// orgid-lint: global — UPDATE by PK; ck_audit_program_findings has org_id, caller verifies org ownership before this call
	_, err := r.db.Exec(ctx, `UPDATE ck_audit_program_findings SET capa_id = $1 WHERE id = $2`, capaID, findingID)
	return err
}

// ── Summary & Evidence ───────────────────────────────────────────────────────

// GetAuditProgramSummary returns aggregate stats for the current year.
func (r *Repository) GetAuditProgramSummary(ctx context.Context, orgID string) (*AuditProgramSummary, error) {
	year := time.Now().Year()
	var s AuditProgramSummary
	err := r.db.QueryRow(ctx, `
		SELECT
			COUNT(*) FILTER (WHERE EXTRACT(YEAR FROM planned_date) = $2)             AS planned_this_year,
			COUNT(*) FILTER (WHERE status = 'completed' AND EXTRACT(YEAR FROM planned_date) = $2) AS completed
		FROM ck_audit_program_audits WHERE org_id = $1`, orgID, year,
	).Scan(&s.AuditsPlannedThisYear, &s.AuditsCompleted)
	if err != nil {
		return nil, err
	}
	r.db.QueryRow(ctx, `
		SELECT COUNT(*) FROM ck_audit_program_findings f
		JOIN ck_audit_program_audits a ON a.id = f.audit_id
		WHERE f.org_id = $1 AND f.capa_id IS NULL`, orgID,
	).Scan(&s.OpenFindings) //nolint:errcheck
	return &s, nil
}

// CountCompletedAuditsLastYear returns how many audits were completed in the last 12 months.
func (r *Repository) CountCompletedAuditsLastYear(ctx context.Context, orgID string) (int, error) {
	var count int
	err := r.db.QueryRow(ctx, `
		SELECT COUNT(*) FROM ck_audit_program_audits
		WHERE org_id = $1 AND status = 'completed' AND actual_date > CURRENT_DATE - INTERVAL '365 days'`, orgID,
	).Scan(&count)
	return count, err
}

// CountOpenAuditFindings returns how many audit findings have no linked CAPA yet.
func (r *Repository) CountOpenAuditFindings(ctx context.Context, orgID string) (int, error) {
	var count int
	err := r.db.QueryRow(ctx, `
		SELECT COUNT(*) FROM ck_audit_program_findings
		WHERE org_id = $1 AND capa_id IS NULL`, orgID,
	).Scan(&count)
	return count, err
}
