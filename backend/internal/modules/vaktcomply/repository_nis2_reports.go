// Copyright (c) 2026 NorvikOps. All rights reserved.
// SPDX-License-Identifier: Elastic-2.0

package vaktcomply

import (
	"context"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
)

// --- NIS2 Art.23 Stage Reports (Migration 175) ---

// SetNIS2Reportable marks an incident as NIS2-reportable and sets the three stage deadlines.
func (r *Repository) SetNIS2Reportable(ctx context.Context, orgID, incidentID string, detectedAt, earlyWarningDue, fullReportDue, finalReportDue time.Time, reportable bool) error {
	tag, err := r.db.Exec(ctx,
		`UPDATE ck_incidents
		    SET nis2_reportable              = $3,
		        nis2_reporting_stage         = 'none',
		        nis2_detected_at             = $4,
		        nis2_early_warning_due        = $5,
		        nis2_full_report_due          = $6,
		        nis2_final_report_due         = $7,
		        updated_at                   = NOW()
		  WHERE id = $1::uuid AND org_id = $2::uuid`,
		incidentID, orgID, reportable, detectedAt, earlyWarningDue, fullReportDue, finalReportDue,
	)
	if err != nil {
		return fmt.Errorf("set nis2 reportable: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return pgx.ErrNoRows
	}
	return nil
}

// UpdateNIS2Stage advances the incident's nis2_reporting_stage and sets the submitted_at timestamp.
func (r *Repository) UpdateNIS2Stage(ctx context.Context, orgID, incidentID, stage string) error {
	var col string
	switch stage {
	case "early_warning":
		col = "nis2_early_warning_submitted_at"
	case "full_report":
		col = "nis2_full_report_submitted_at"
	case "final_report":
		col = "nis2_final_report_submitted_at"
	default:
		return fmt.Errorf("invalid stage: %s", stage)
	}
	// orgid-lint: global — WHERE clause includes org_id = $2::uuid; scanner misses it due to string concatenation
	_, err := r.db.Exec(ctx,
		`UPDATE ck_incidents
		    SET `+col+` = NOW(),
		        nis2_reporting_stage = $3,
		        updated_at = NOW()
		  WHERE id = $1::uuid AND org_id = $2::uuid`,
		incidentID, orgID, stage,
	)
	return err
}

// UpsertNIS2Report saves or updates report content for a stage.
func (r *Repository) UpsertNIS2Report(ctx context.Context, orgID, incidentID, userID, stage string, in NIS2ReportInput) (*NIS2StageReport, error) {
	var id string
	var submittedAt pgtype.Timestamptz
	err := r.db.QueryRow(ctx,
		`INSERT INTO ck_nis2_reports
		    (org_id, incident_id, stage,
		     affected_services, initial_assessment, root_cause,
		     affected_users_estimate, measures_taken, estimated_recovery,
		     full_root_cause_analysis, permanent_measures, effectiveness_evidence,
		     submitted_by, submitted_at, updated_at)
		 VALUES ($1::uuid, $2::uuid, $3,
		         $4, $5, $6,
		         $7, $8, $9,
		         $10, $11, $12,
		         $13::uuid, NOW(), NOW())
		 ON CONFLICT (incident_id, stage) DO UPDATE
		    SET affected_services        = EXCLUDED.affected_services,
		        initial_assessment       = EXCLUDED.initial_assessment,
		        root_cause               = EXCLUDED.root_cause,
		        affected_users_estimate  = EXCLUDED.affected_users_estimate,
		        measures_taken           = EXCLUDED.measures_taken,
		        estimated_recovery       = EXCLUDED.estimated_recovery,
		        full_root_cause_analysis = EXCLUDED.full_root_cause_analysis,
		        permanent_measures       = EXCLUDED.permanent_measures,
		        effectiveness_evidence   = EXCLUDED.effectiveness_evidence,
		        submitted_by             = EXCLUDED.submitted_by,
		        submitted_at             = NOW(),
		        updated_at               = NOW()
		 RETURNING id::text, submitted_at`,
		orgID, incidentID, stage,
		in.AffectedServices, in.InitialAssessment, in.RootCause,
		in.AffectedUsersEstimate, in.MeasuresTaken, in.EstimatedRecovery,
		in.FullRootCauseAnalysis, in.PermanentMeasures, in.EffectivenessEvidence,
		userID,
	).Scan(&id, &submittedAt)
	if err != nil {
		return nil, fmt.Errorf("upsert nis2 report: %w", err)
	}
	t := ckTsToTimePtr(submittedAt)
	return &NIS2StageReport{ID: id, Stage: stage, SubmittedAt: t}, nil
}

// ListNIS2Reports returns all stage reports for an incident.
func (r *Repository) ListNIS2Reports(ctx context.Context, orgID, incidentID string) ([]NIS2StageReport, error) {
	rows, err := r.db.Query(ctx,
		`SELECT id::text, stage, submitted_at, COALESCE(pdf_path, '')
		   FROM ck_nis2_reports
		  WHERE org_id = $1::uuid AND incident_id = $2::uuid
		  ORDER BY CASE stage
		    WHEN 'early_warning' THEN 1
		    WHEN 'full_report'   THEN 2
		    WHEN 'final_report'  THEN 3
		  END`,
		orgID, incidentID,
	)
	if err != nil {
		return nil, fmt.Errorf("list nis2 reports: %w", err)
	}
	defer rows.Close()

	var out []NIS2StageReport
	for rows.Next() {
		var rep NIS2StageReport
		var submittedAt pgtype.Timestamptz
		if err := rows.Scan(&rep.ID, &rep.Stage, &submittedAt, &rep.PDFPath); err != nil {
			return nil, fmt.Errorf("scan nis2 report: %w", err)
		}
		rep.SubmittedAt = ckTsToTimePtr(submittedAt)
		out = append(out, rep)
	}
	return out, rows.Err()
}

// ListNIS2OpenIncidents returns all open NIS2-reportable incidents for deadline checking.
func (r *Repository) ListNIS2OpenIncidents(ctx context.Context, orgID string) ([]Incident, error) {
	rows, err := r.db.Query(ctx,
		`SELECT id::text, org_id::text, title, status,
		        nis2_early_warning_due, nis2_full_report_due, nis2_final_report_due,
		        nis2_early_warning_submitted_at, nis2_full_report_submitted_at, nis2_final_report_submitted_at
		   FROM ck_incidents
		  WHERE org_id = $1::uuid
		    AND nis2_reportable = true
		    AND status NOT IN ('resolved', 'closed')`,
		orgID,
	)
	if err != nil {
		return nil, fmt.Errorf("list nis2 open incidents: %w", err)
	}
	defer rows.Close()

	var out []Incident
	for rows.Next() {
		var inc Incident
		var ewd, frd, fnd pgtype.Timestamptz
		var ewsa, frsa, fnsa pgtype.Timestamptz
		if err := rows.Scan(
			&inc.ID, &inc.OrgID, &inc.Title, &inc.Status,
			&ewd, &frd, &fnd,
			&ewsa, &frsa, &fnsa,
		); err != nil {
			return nil, fmt.Errorf("scan nis2 open incident: %w", err)
		}
		reportable := true
		inc.NIS2Reportable = &reportable
		inc.NIS2EarlyWarningDue = ckTsToTimePtr(ewd)
		inc.NIS2FullReportDue = ckTsToTimePtr(frd)
		inc.NIS2FinalReportDue = ckTsToTimePtr(fnd)
		inc.NIS2EarlyWarningSubmittedAt = ckTsToTimePtr(ewsa)
		inc.NIS2FullReportSubmittedAt = ckTsToTimePtr(frsa)
		inc.NIS2FinalReportSubmittedAt = ckTsToTimePtr(fnsa)
		out = append(out, inc)
	}
	return out, rows.Err()
}

// --- Authority Contacts (Migration 175) ---

// ListAuthorityContacts returns org-specific plus built-in authority contacts.
func (r *Repository) ListAuthorityContacts(ctx context.Context, orgID string) ([]AuthorityContact, error) {
	rows, err := r.db.Query(ctx,
		`SELECT id::text, org_id::text, country, COALESCE(sector,''),
		        authority_name, COALESCE(report_url,''), COALESCE(email,''), COALESCE(phone,''),
		        COALESCE(notes,''), is_primary, is_builtin, created_at::text
		   FROM ck_authority_contacts
		  WHERE org_id = $1::uuid OR (is_builtin = true AND org_id IS NULL)
		  ORDER BY is_builtin DESC, country, authority_name`,
		orgID,
	)
	if err != nil {
		return nil, fmt.Errorf("list authority contacts: %w", err)
	}
	defer rows.Close()

	var out []AuthorityContact
	for rows.Next() {
		var c AuthorityContact
		var oid pgtype.Text
		if err := rows.Scan(
			&c.ID, &oid, &c.Country, &c.Sector,
			&c.AuthorityName, &c.ReportURL, &c.Email, &c.Phone,
			&c.Notes, &c.IsPrimary, &c.IsBuiltin, &c.CreatedAt,
		); err != nil {
			return nil, fmt.Errorf("scan authority contact: %w", err)
		}
		if oid.Valid {
			s := oid.String
			c.OrgID = &s
		}
		out = append(out, c)
	}
	return out, rows.Err()
}

// CreateAuthorityContact inserts a custom org-scoped authority contact.
func (r *Repository) CreateAuthorityContact(ctx context.Context, orgID string, in AuthorityContact) (*AuthorityContact, error) {
	var id, createdAt string
	err := r.db.QueryRow(ctx,
		`INSERT INTO ck_authority_contacts
		    (org_id, country, sector, authority_name, report_url, email, phone, notes, is_primary, is_builtin)
		 VALUES ($1::uuid, $2, $3, $4, $5, $6, $7, $8, $9, false)
		 RETURNING id::text, created_at::text`,
		orgID, in.Country, in.Sector, in.AuthorityName, in.ReportURL, in.Email, in.Phone, in.Notes, in.IsPrimary,
	).Scan(&id, &createdAt)
	if err != nil {
		return nil, fmt.Errorf("create authority contact: %w", err)
	}
	in.ID = id
	in.CreatedAt = createdAt
	in.OrgID = &orgID
	return &in, nil
}
