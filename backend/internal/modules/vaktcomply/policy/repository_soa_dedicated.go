// Copyright (c) 2026 NorvikOps. All rights reserved.
// SPDX-License-Identifier: Elastic-2.0

package policy

import (
	"context"
	"errors"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
)

// SoADedicatedEntry is a row in ck_soa_entries.
type SoADedicatedEntry struct {
	ID                   string `json:"id"`
	OrgID                string `json:"org_id"`
	Version              int    `json:"version"`
	ControlRef           string `json:"control_ref"`
	ControlName          string `json:"control_name"`
	ControlGroup         string `json:"control_group"`
	Applicable           bool   `json:"applicable"`
	Justification        string `json:"justification,omitempty"`
	ExclusionReason      string `json:"exclusion_reason,omitempty"`
	ImplementationStatus string `json:"implementation_status"`
	// StatusSource says where ImplementationStatus (and ControlName) came from:
	// "control" (live from the linked ck_control, draft version), "snapshot"
	// (frozen at approval, approved version) or "soa" (no control linked, the
	// stored ck_soa_entries value). See resolveSoAEntry.
	StatusSource string `json:"status_source"`
	// LinkedControlID / LinkedFrameworkID name the ck_control the status is read
	// from, so the SoA page can send the user to the control page, the one
	// place the status is edited.
	LinkedControlID   *string    `json:"linked_control_id,omitempty"`
	LinkedFrameworkID *string    `json:"linked_framework_id,omitempty"`
	ManuallySet       bool       `json:"manually_set"`
	CKControlID       *string    `json:"ck_control_id,omitempty"`
	EvidenceReference string     `json:"evidence_reference,omitempty"`
	IsApproved        bool       `json:"is_approved"`
	ApprovedBy        *string    `json:"approved_by,omitempty"`
	ApprovedAt        *time.Time `json:"approved_at,omitempty"`
	Notes             string     `json:"notes,omitempty"`
	CreatedAt         time.Time  `json:"created_at"`
	UpdatedAt         time.Time  `json:"updated_at"`
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
//
// ImplementationStatus and ManuallySet are still accepted for backward
// compatibility but no longer written: the implementation status of a SoA entry
// is the status of its control (one source). It is edited on the control.
type UpdateSoAEntryInput struct {
	Applicable           bool    `json:"applicable"`
	Justification        string  `json:"justification,omitempty"`
	ExclusionReason      string  `json:"exclusion_reason,omitempty"`
	ImplementationStatus string  `json:"implementation_status,omitempty" validate:"omitempty,oneof=not_started planned partial implemented"`
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

// soaEntrySelect reads the entries of one version together with the control
// each entry takes its status from. That is the control named by ck_control_id,
// otherwise the control with the same control_id (e.g. "A.5.2") in an ISO 27001
// framework of the org (the name heuristic of isISO27001Framework). The evidence
// count uses the filter of CountCKEvidenceByControl, so ResolveStatus sees the
// same input as on the control page.
const soaEntrySelect = `
	SELECT e.id, e.org_id, e.version, e.control_ref, e.control_name, e.control_group,
	       e.applicable, COALESCE(e.justification,''), COALESCE(e.exclusion_reason,''),
	       e.implementation_status, e.manually_set, e.ck_control_id, COALESCE(e.evidence_reference,''),
	       e.is_approved, e.approved_by, e.approved_at, COALESCE(e.notes,''), e.created_at, e.updated_at,
	       lc.id::text, lc.framework_id::text, COALESCE(lc.title,''), COALESCE(lc.not_applicable,false),
	       COALESCE(lc.manual_status,''),
	       COALESCE((SELECT COUNT(*) FROM ck_evidence ev
	                 WHERE ev.control_id = lc.id AND ev.org_id = e.org_id
	                   AND ev.status IN ('approved','pending')), 0)::int
	FROM ck_soa_entries e
	LEFT JOIN LATERAL (
		SELECT c.id, c.framework_id, c.title, c.not_applicable, c.manual_status
		FROM ck_controls c
		JOIN ck_frameworks f ON f.id = c.framework_id AND f.org_id = c.org_id
		WHERE c.org_id = e.org_id
		  AND (c.id = e.ck_control_id
		       OR (c.control_id = e.control_ref
		           AND (LOWER(f.name) LIKE '%iso 27001%' OR LOWER(f.name) LIKE '%iso27001%')))
		ORDER BY (c.id = e.ck_control_id) DESC NULLS LAST, f.created_at, c.id
		LIMIT 1
	) lc ON true
	WHERE e.org_id = $1 AND e.version = $2`

// ListSoAEntries returns all entries for the org at the given version, with the
// implementation status resolved by resolveSoAEntry: a draft shows the status of
// the linked control, an approved version its frozen snapshot.
func (r *Repository) ListSoAEntries(ctx context.Context, orgID string, version int) ([]SoADedicatedEntry, error) {
	approved, err := r.isSoAVersionApproved(ctx, orgID, version)
	if err != nil {
		return nil, err
	}
	rows, err := r.db.Query(ctx, soaEntrySelect+`
		ORDER BY e.control_group, e.control_ref`,
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
		var linkID, linkFW pgtype.Text
		var link soaControlLink
		if err := rows.Scan(
			&e.ID, &e.OrgID, &e.Version, &e.ControlRef, &e.ControlName, &e.ControlGroup,
			&e.Applicable, &e.Justification, &e.ExclusionReason,
			&e.ImplementationStatus, &e.ManuallySet, &ckCtrl, &e.EvidenceReference,
			&e.IsApproved, &approvedBy, &approvedAt, &e.Notes, &e.CreatedAt, &e.UpdatedAt,
			&linkID, &linkFW, &link.Title, &link.NotApplicable, &link.ManualStatus, &link.EvidenceCount,
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
		var linkPtr *soaControlLink
		if linkID.Valid {
			link.ID = linkID.String
			link.FrameworkID = linkFW.String
			linkPtr = &link
		}
		resolveSoAEntry(&e, approved, linkPtr)
		entries = append(entries, e)
	}
	return entries, rows.Err()
}

// isSoAVersionApproved reports whether the given SoA version is approved
// (frozen). A missing version row counts as draft.
func (r *Repository) isSoAVersionApproved(ctx context.Context, orgID string, version int) (bool, error) {
	var status string
	err := r.db.QueryRow(ctx,
		`SELECT COALESCE(status,'draft') FROM ck_soa_versions WHERE org_id = $1 AND version = $2`,
		orgID, version,
	).Scan(&status)
	if errors.Is(err, pgx.ErrNoRows) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	return status == "approved", nil
}

// GetSoAEntry returns a single entry by control_ref, resolved like ListSoAEntries.
func (r *Repository) GetSoAEntry(ctx context.Context, orgID, controlRef string, version int) (*SoADedicatedEntry, error) {
	entries, err := r.ListSoAEntries(ctx, orgID, version)
	if err != nil {
		return nil, err
	}
	for i := range entries {
		if entries[i].ControlRef == controlRef {
			return &entries[i], nil
		}
	}
	return nil, pgx.ErrNoRows
}

// UpdateSoAEntry updates the SoA-owned fields of a single entry by control_ref.
// implementation_status and manually_set are deliberately not written: the
// status comes from the control (resolveSoAEntry) and is frozen only on approval.
func (r *Repository) UpdateSoAEntry(ctx context.Context, orgID, controlRef string, version int, in UpdateSoAEntryInput) error {
	_, err := r.db.Exec(ctx, `
		UPDATE ck_soa_entries SET
			applicable         = $1,
			justification      = NULLIF($2, ''),
			exclusion_reason   = NULLIF($3, ''),
			ck_control_id      = $4,
			evidence_reference = NULLIF($5, ''),
			notes              = NULLIF($6, ''),
			updated_at         = NOW()
		WHERE org_id = $7 AND control_ref = $8 AND version = $9`,
		in.Applicable, in.Justification, in.ExclusionReason,
		in.CKControlID, in.EvidenceReference,
		in.Notes, orgID, controlRef, version,
	)
	return err
}

// ApproveSoAVersion marks all entries for the version as approved and sets version status.
//
// freeze carries the entries as the draft showed them (resolved from their
// controls). Their status and title are written into ck_soa_entries in the same
// transaction, so the approved version is a snapshot: later changes on the
// control no longer move it (an audit document must not change afterwards).
func (r *Repository) ApproveSoAVersion(ctx context.Context, orgID string, version int, approverID string, freeze []SoADedicatedEntry) error {
	now := time.Now().UTC()
	tx, err := r.db.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx) //nolint:errcheck

	for _, e := range freeze {
		if e.StatusSource != SoAStatusSourceControl {
			continue
		}
		// The rows come from ListSoAEntries of this very version; writing the same
		// snapshot twice yields the same state, so the CommandTag is not checked.
		//idempotent: freeze of rows just read from this version, re-run yields the same snapshot
		if _, err := tx.Exec(ctx, `
			UPDATE ck_soa_entries SET implementation_status = $1, control_name = $2, updated_at = NOW()
			WHERE org_id = $3 AND version = $4 AND control_ref = $5`,
			e.ImplementationStatus, e.ControlName, orgID, version, e.ControlRef,
		); err != nil {
			return err
		}
	}

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

// GetSoASummary computes aggregate statistics for the version from the resolved
// entries (summarizeSoA), so the summary, the page and every export count the
// same status.
func (r *Repository) GetSoASummary(ctx context.Context, orgID string, version int) (*SoASummary, error) {
	entries, err := r.ListSoAEntries(ctx, orgID, version)
	if err != nil {
		return nil, err
	}
	approved, err := r.isSoAVersionApproved(ctx, orgID, version)
	if err != nil {
		return nil, err
	}
	s := summarizeSoA(entries)
	s.Version = version
	s.Status = "draft"
	if approved {
		s.Status = "approved"
	}
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
