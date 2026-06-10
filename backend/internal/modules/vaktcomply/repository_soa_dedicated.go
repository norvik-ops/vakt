// Copyright (c) 2026 NorvikOps. All rights reserved.
// SPDX-License-Identifier: Elastic-2.0

package vaktcomply

import (
	"context"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
)

// SoADedicatedEntry is a row in ck_soa_entries.
type SoADedicatedEntry struct {
	ID                   string     `json:"id"`
	OrgID                string     `json:"org_id"`
	Version              int        `json:"version"`
	ControlRef           string     `json:"control_ref"`
	ControlName          string     `json:"control_name"`
	ControlGroup         string     `json:"control_group"`
	Applicable           bool       `json:"applicable"`
	Justification        string     `json:"justification,omitempty"`
	ExclusionReason      string     `json:"exclusion_reason,omitempty"`
	ImplementationStatus string     `json:"implementation_status"`
	ManuallySet          bool       `json:"manually_set"`
	CKControlID          *string    `json:"ck_control_id,omitempty"`
	EvidenceReference    string     `json:"evidence_reference,omitempty"`
	IsApproved           bool       `json:"is_approved"`
	ApprovedBy           *string    `json:"approved_by,omitempty"`
	ApprovedAt           *time.Time `json:"approved_at,omitempty"`
	Notes                string     `json:"notes,omitempty"`
	CreatedAt            time.Time  `json:"created_at"`
	UpdatedAt            time.Time  `json:"updated_at"`
}

// SoAVersion is a row in ck_soa_versions.
type SoAVersion struct {
	ID         string     `json:"id"`
	OrgID      string     `json:"org_id"`
	Version    int        `json:"version"`
	Status     string     `json:"status"`
	ApprovedBy *string    `json:"approved_by,omitempty"`
	ApprovedAt *time.Time `json:"approved_at,omitempty"`
	Notes      string     `json:"notes,omitempty"`
	CreatedAt  time.Time  `json:"created_at"`
}

// SoASummary holds aggregate statistics for the Statement of Applicability.
type SoASummary struct {
	Version           int     `json:"version"`
	Status            string  `json:"status"`
	ApplicableCount   int     `json:"applicable_count"`
	ExcludedCount     int     `json:"excluded_count"`
	ImplementedCount  int     `json:"implemented_count"`
	PartialCount      int     `json:"partial_count"`
	PlannedCount      int     `json:"planned_count"`
	NotStartedCount   int     `json:"not_started_count"`
	ImplementationPct float64 `json:"implementation_pct"`
}

// UpdateSoAEntryInput holds validated input for updating a single SoA entry.
type UpdateSoAEntryInput struct {
	Applicable           bool    `json:"applicable"`
	Justification        string  `json:"justification,omitempty"`
	ExclusionReason      string  `json:"exclusion_reason,omitempty"`
	ImplementationStatus string  `json:"implementation_status" validate:"required,oneof=not_started planned partial implemented"`
	ManuallySet          bool    `json:"manually_set"`
	CKControlID          *string `json:"ck_control_id,omitempty"`
	EvidenceReference    string  `json:"evidence_reference,omitempty"`
	Notes                string  `json:"notes,omitempty"`
}

// HasDedicatedSoA returns true if the org has any ck_soa_entries (idempotency check).
func (r *Repository) HasDedicatedSoA(ctx context.Context, orgID string) (bool, error) {
	var count int
	err := r.db.QueryRow(ctx,
		`SELECT COUNT(*) FROM ck_soa_entries WHERE org_id = $1`, orgID,
	).Scan(&count)
	return count > 0, err
}

// GetCurrentSoAVersion returns the highest version number for the org, or 0 if none.
func (r *Repository) GetCurrentSoAVersion(ctx context.Context, orgID string) (int, error) {
	var version int
	err := r.db.QueryRow(ctx,
		`SELECT COALESCE(MAX(version), 0) FROM ck_soa_versions WHERE org_id = $1`, orgID,
	).Scan(&version)
	return version, err
}

// CreateSoAVersion inserts a new ck_soa_versions row.
func (r *Repository) CreateSoAVersion(ctx context.Context, orgID string, version int) error {
	_, err := r.db.Exec(ctx,
		`INSERT INTO ck_soa_versions (org_id, version, status) VALUES ($1, $2, 'draft')`,
		orgID, version,
	)
	return err
}

// InitSoAEntries inserts one entry per ISO 27001:2022 control for the org at the given version.
func (r *Repository) InitSoAEntries(ctx context.Context, orgID string, version int, controls []soaControlTemplate) error {
	batch := &pgx.Batch{}
	for _, c := range controls {
		batch.Queue(
			`INSERT INTO ck_soa_entries
				(org_id, version, control_ref, control_name, control_group, applicable, implementation_status)
			 VALUES ($1, $2, $3, $4, $5, true, 'not_started')
			 ON CONFLICT (org_id, version, control_ref) DO NOTHING`,
			orgID, version, c.Ref, c.Name, c.Group,
		)
	}
	br := r.db.SendBatch(ctx, batch)
	defer br.Close()
	for range controls {
		if _, err := br.Exec(); err != nil {
			return err
		}
	}
	return nil
}

// ListSoAEntries returns all entries for the org at the given version.
func (r *Repository) ListSoAEntries(ctx context.Context, orgID string, version int) ([]SoADedicatedEntry, error) {
	rows, err := r.db.Query(ctx, `
		SELECT id, org_id, version, control_ref, control_name, control_group,
		       applicable, COALESCE(justification,''), COALESCE(exclusion_reason,''),
		       implementation_status, manually_set, ck_control_id, COALESCE(evidence_reference,''),
		       is_approved, approved_by, approved_at, COALESCE(notes,''), created_at, updated_at
		FROM ck_soa_entries
		WHERE org_id = $1 AND version = $2
		ORDER BY control_group, control_ref`,
		orgID, version,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var entries []SoADedicatedEntry
	for rows.Next() {
		var e SoADedicatedEntry
		var ckCtrl pgtype.Text
		var approvedBy pgtype.Text
		var approvedAt pgtype.Timestamptz
		if err := rows.Scan(
			&e.ID, &e.OrgID, &e.Version, &e.ControlRef, &e.ControlName, &e.ControlGroup,
			&e.Applicable, &e.Justification, &e.ExclusionReason,
			&e.ImplementationStatus, &e.ManuallySet, &ckCtrl, &e.EvidenceReference,
			&e.IsApproved, &approvedBy, &approvedAt, &e.Notes, &e.CreatedAt, &e.UpdatedAt,
		); err != nil {
			return nil, err
		}
		if ckCtrl.Valid {
			e.CKControlID = &ckCtrl.String
		}
		if approvedBy.Valid {
			e.ApprovedBy = &approvedBy.String
		}
		if approvedAt.Valid {
			t := approvedAt.Time
			e.ApprovedAt = &t
		}
		entries = append(entries, e)
	}
	return entries, rows.Err()
}

// GetSoAEntry returns a single entry by control_ref.
func (r *Repository) GetSoAEntry(ctx context.Context, orgID, controlRef string, version int) (*SoADedicatedEntry, error) {
	var e SoADedicatedEntry
	var ckCtrl, approvedBy pgtype.Text
	var approvedAt pgtype.Timestamptz
	err := r.db.QueryRow(ctx, `
		SELECT id, org_id, version, control_ref, control_name, control_group,
		       applicable, COALESCE(justification,''), COALESCE(exclusion_reason,''),
		       implementation_status, manually_set, ck_control_id, COALESCE(evidence_reference,''),
		       is_approved, approved_by, approved_at, COALESCE(notes,''), created_at, updated_at
		FROM ck_soa_entries
		WHERE org_id = $1 AND control_ref = $2 AND version = $3`,
		orgID, controlRef, version,
	).Scan(
		&e.ID, &e.OrgID, &e.Version, &e.ControlRef, &e.ControlName, &e.ControlGroup,
		&e.Applicable, &e.Justification, &e.ExclusionReason,
		&e.ImplementationStatus, &e.ManuallySet, &ckCtrl, &e.EvidenceReference,
		&e.IsApproved, &approvedBy, &approvedAt, &e.Notes, &e.CreatedAt, &e.UpdatedAt,
	)
	if err != nil {
		return nil, err
	}
	if ckCtrl.Valid {
		e.CKControlID = &ckCtrl.String
	}
	if approvedBy.Valid {
		e.ApprovedBy = &approvedBy.String
	}
	if approvedAt.Valid {
		t := approvedAt.Time
		e.ApprovedAt = &t
	}
	return &e, nil
}

// UpdateSoAEntry updates a single entry by control_ref.
func (r *Repository) UpdateSoAEntry(ctx context.Context, orgID, controlRef string, version int, in UpdateSoAEntryInput) error {
	_, err := r.db.Exec(ctx, `
		UPDATE ck_soa_entries SET
			applicable            = $1,
			justification         = NULLIF($2, ''),
			exclusion_reason      = NULLIF($3, ''),
			implementation_status = $4,
			manually_set          = $5,
			ck_control_id         = $6,
			evidence_reference    = NULLIF($7, ''),
			notes                 = NULLIF($8, ''),
			updated_at            = NOW()
		WHERE org_id = $9 AND control_ref = $10 AND version = $11`,
		in.Applicable, in.Justification, in.ExclusionReason,
		in.ImplementationStatus, in.ManuallySet, in.CKControlID, in.EvidenceReference,
		in.Notes, orgID, controlRef, version,
	)
	return err
}

// ApproveSoAVersion marks all entries for the version as approved and sets version status.
func (r *Repository) ApproveSoAVersion(ctx context.Context, orgID string, version int, approverID string) error {
	now := time.Now().UTC()
	tx, err := r.db.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx) //nolint:errcheck

	_, err = tx.Exec(ctx, `
		UPDATE ck_soa_entries SET is_approved = true, approved_by = $1, approved_at = $2
		WHERE org_id = $3 AND version = $4`,
		approverID, now, orgID, version,
	)
	if err != nil {
		return err
	}
	_, err = tx.Exec(ctx, `
		UPDATE ck_soa_versions SET status = 'approved', approved_by = $1, approved_at = $2
		WHERE org_id = $3 AND version = $4`,
		approverID, now, orgID, version,
	)
	if err != nil {
		return err
	}
	return tx.Commit(ctx)
}

// CopyVersionEntries copies all entries from srcVersion to dstVersion (for new draft after approve).
func (r *Repository) CopyVersionEntries(ctx context.Context, orgID string, srcVersion, dstVersion int) error {
	_, err := r.db.Exec(ctx, `
		INSERT INTO ck_soa_entries
			(org_id, version, control_ref, control_name, control_group, applicable,
			 justification, exclusion_reason, implementation_status, manually_set,
			 ck_control_id, evidence_reference, notes)
		SELECT org_id, $1, control_ref, control_name, control_group, applicable,
			   justification, exclusion_reason, implementation_status, manually_set,
			   ck_control_id, evidence_reference, notes
		FROM ck_soa_entries
		WHERE org_id = $2 AND version = $3
		ON CONFLICT DO NOTHING`,
		dstVersion, orgID, srcVersion,
	)
	return err
}

// ListSoAVersions returns all versions for the org.
func (r *Repository) ListSoAVersions(ctx context.Context, orgID string) ([]SoAVersion, error) {
	rows, err := r.db.Query(ctx, `
		SELECT id, org_id, version, status, approved_by, approved_at, COALESCE(notes,''), created_at
		FROM ck_soa_versions WHERE org_id = $1 ORDER BY version DESC`,
		orgID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var versions []SoAVersion
	for rows.Next() {
		var v SoAVersion
		var approvedBy pgtype.Text
		var approvedAt pgtype.Timestamptz
		if err := rows.Scan(&v.ID, &v.OrgID, &v.Version, &v.Status, &approvedBy, &approvedAt, &v.Notes, &v.CreatedAt); err != nil {
			return nil, err
		}
		if approvedBy.Valid {
			v.ApprovedBy = &approvedBy.String
		}
		if approvedAt.Valid {
			t := approvedAt.Time
			v.ApprovedAt = &t
		}
		versions = append(versions, v)
	}
	return versions, rows.Err()
}

// GetSoASummary computes aggregate statistics for the current version.
func (r *Repository) GetSoASummary(ctx context.Context, orgID string, version int) (*SoASummary, error) {
	var s SoASummary
	s.Version = version
	err := r.db.QueryRow(ctx, `
		SELECT
			COUNT(*) FILTER (WHERE applicable = true)  AS applicable_count,
			COUNT(*) FILTER (WHERE applicable = false) AS excluded_count,
			COUNT(*) FILTER (WHERE applicable = true AND implementation_status = 'implemented') AS implemented_count,
			COUNT(*) FILTER (WHERE applicable = true AND implementation_status = 'partial')     AS partial_count,
			COUNT(*) FILTER (WHERE applicable = true AND implementation_status = 'planned')     AS planned_count,
			COUNT(*) FILTER (WHERE applicable = true AND implementation_status = 'not_started') AS not_started_count
		FROM ck_soa_entries WHERE org_id = $1 AND version = $2`,
		orgID, version,
	).Scan(&s.ApplicableCount, &s.ExcludedCount, &s.ImplementedCount, &s.PartialCount, &s.PlannedCount, &s.NotStartedCount)
	if err != nil {
		return nil, err
	}
	if s.ApplicableCount > 0 {
		s.ImplementationPct = float64(s.ImplementedCount) / float64(s.ApplicableCount) * 100
	}
	// Get version status
	r.db.QueryRow(ctx, `SELECT COALESCE(status,'draft') FROM ck_soa_versions WHERE org_id = $1 AND version = $2`, orgID, version).Scan(&s.Status) //nolint:errcheck
	return &s, nil
}

// CountExcludedWithoutReason returns how many excluded entries lack an exclusion_reason.
func (r *Repository) CountExcludedWithoutReason(ctx context.Context, orgID string, version int) (int, error) {
	var count int
	err := r.db.QueryRow(ctx, `
		SELECT COUNT(*) FROM ck_soa_entries
		WHERE org_id = $1 AND version = $2 AND applicable = false AND (exclusion_reason IS NULL OR exclusion_reason = '')`,
		orgID, version,
	).Scan(&count)
	return count, err
}

// SyncSoAImplementationStatus updates implementation_status based on evidence count,
// but only if manually_set = false for the entry.
func (r *Repository) SyncSoAImplementationStatus(ctx context.Context, orgID, controlID string) error {
	// Count non-stale evidence for this control
	var evidenceCount int
	r.db.QueryRow(ctx, `
		SELECT COUNT(*) FROM ck_evidence
		WHERE org_id = $1 AND control_id = $2
		  AND (evidence_status IS NULL OR evidence_status != 'stale')`,
		orgID, controlID,
	).Scan(&evidenceCount) //nolint:errcheck

	status := "not_started"
	switch {
	case evidenceCount >= 3:
		status = "implemented"
	case evidenceCount >= 1:
		status = "partial"
	}

	// Get current version
	version, err := r.GetCurrentSoAVersion(ctx, orgID)
	if err != nil || version == 0 {
		return err
	}

	// Update only if not manually_set
	_, err = r.db.Exec(ctx, `
		UPDATE ck_soa_entries SET implementation_status = $1, updated_at = NOW()
		WHERE org_id = $2 AND version = $3 AND ck_control_id = $4 AND manually_set = false`,
		status, orgID, version, controlID,
	)
	return err
}
