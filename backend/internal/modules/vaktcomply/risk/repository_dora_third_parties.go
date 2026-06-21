// Copyright (c) 2026 NorvikOps. All rights reserved.
// SPDX-License-Identifier: Elastic-2.0

package risk

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"
)

// ListDORAThirdParties returns all DORA third-party entries for an org, optionally
// filtered by criticality. Sorted by name ASC.
func (r *Repository) ListDORAThirdParties(ctx context.Context, orgID, criticality string) ([]DORAThirdParty, error) {
	query := `
		SELECT id, org_id, name, service_type, criticality,
		       contract_start::text, contract_end::text,
		       sla_rto_hours, sla_availability,
		       has_subcontractors, subcontractor_names,
		       data_location, exit_strategy, exit_notes, notes,
		       created_by, created_at, updated_at
		FROM dora_third_parties
		WHERE org_id = $1`
	args := []any{orgID}
	if criticality != "" {
		args = append(args, criticality)
		query += fmt.Sprintf(" AND criticality = $%d", len(args))
	}
	query += " ORDER BY name ASC"

	rows, err := r.db.Query(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("list dora third parties: %w", err)
	}
	defer rows.Close()

	var out []DORAThirdParty
	for rows.Next() {
		tp, err := scanDORAThirdParty(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, tp)
	}
	if out == nil {
		out = []DORAThirdParty{}
	}
	return out, rows.Err()
}

// GetDORAThirdParty returns a single third-party entry including linked control IDs.
func (r *Repository) GetDORAThirdParty(ctx context.Context, orgID, id string) (*DORAThirdParty, error) {
	row := r.db.QueryRow(ctx, `
		SELECT id, org_id, name, service_type, criticality,
		       contract_start::text, contract_end::text,
		       sla_rto_hours, sla_availability,
		       has_subcontractors, subcontractor_names,
		       data_location, exit_strategy, exit_notes, notes,
		       created_by, created_at, updated_at
		FROM dora_third_parties
		WHERE id = $1 AND org_id = $2`, id, orgID)

	tp, err := scanDORAThirdParty(row)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, err
	}

	// Load linked control IDs.
	crows, err := r.db.Query(ctx,
		`SELECT control_id FROM dora_third_party_controls WHERE third_party_id = $1`, id)
	if err != nil {
		return nil, fmt.Errorf("list dora third party controls: %w", err)
	}
	defer crows.Close()
	for crows.Next() {
		var cid string
		if err := crows.Scan(&cid); err != nil {
			return nil, err
		}
		tp.ControlIDs = append(tp.ControlIDs, cid)
	}
	if tp.ControlIDs == nil {
		tp.ControlIDs = []string{}
	}
	return &tp, crows.Err()
}

// CreateDORAThirdParty inserts a new entry and returns it.
func (r *Repository) CreateDORAThirdParty(ctx context.Context, orgID, createdBy string, in CreateDORAThirdPartyInput) (*DORAThirdParty, error) {
	row := r.db.QueryRow(ctx, `
		INSERT INTO dora_third_parties
		  (org_id, name, service_type, criticality,
		   contract_start, contract_end,
		   sla_rto_hours, sla_availability,
		   has_subcontractors, subcontractor_names,
		   data_location, exit_strategy, exit_notes, notes,
		   created_by)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14,$15)
		RETURNING id, org_id, name, service_type, criticality,
		          contract_start::text, contract_end::text,
		          sla_rto_hours, sla_availability,
		          has_subcontractors, subcontractor_names,
		          data_location, exit_strategy, exit_notes, notes,
		          created_by, created_at, updated_at`,
		orgID, in.Name, in.ServiceType, in.Criticality,
		in.ContractStart, in.ContractEnd,
		in.SLARTOHours, in.SLAAvailability,
		in.HasSubcontractors, in.SubcontractorNames,
		in.DataLocation, in.ExitStrategy, in.ExitNotes, in.Notes,
		createdBy,
	)
	tp, err := scanDORAThirdParty(row)
	if err != nil {
		return nil, fmt.Errorf("create dora third party: %w", err)
	}
	tp.ControlIDs = []string{}
	return &tp, nil
}

// UpdateDORAThirdParty replaces all mutable fields on an existing entry.
func (r *Repository) UpdateDORAThirdParty(ctx context.Context, orgID, id string, in UpdateDORAThirdPartyInput) (*DORAThirdParty, error) {
	row := r.db.QueryRow(ctx, `
		UPDATE dora_third_parties SET
		  name = $1, service_type = $2, criticality = $3,
		  contract_start = $4, contract_end = $5,
		  sla_rto_hours = $6, sla_availability = $7,
		  has_subcontractors = $8, subcontractor_names = $9,
		  data_location = $10, exit_strategy = $11,
		  exit_notes = $12, notes = $13,
		  updated_at = NOW()
		WHERE id = $14 AND org_id = $15
		RETURNING id, org_id, name, service_type, criticality,
		          contract_start::text, contract_end::text,
		          sla_rto_hours, sla_availability,
		          has_subcontractors, subcontractor_names,
		          data_location, exit_strategy, exit_notes, notes,
		          created_by, created_at, updated_at`,
		in.Name, in.ServiceType, in.Criticality,
		in.ContractStart, in.ContractEnd,
		in.SLARTOHours, in.SLAAvailability,
		in.HasSubcontractors, in.SubcontractorNames,
		in.DataLocation, in.ExitStrategy,
		in.ExitNotes, in.Notes,
		id, orgID,
	)
	tp, err := scanDORAThirdParty(row)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, fmt.Errorf("update dora third party: %w", err)
	}
	return &tp, nil
}

// DeleteDORAThirdParty removes an entry (cascade deletes control links).
func (r *Repository) DeleteDORAThirdParty(ctx context.Context, orgID, id string) error {
	n, err := r.db.Exec(ctx,
		`DELETE FROM dora_third_parties WHERE id = $1 AND org_id = $2`, id, orgID)
	if err != nil {
		return fmt.Errorf("delete dora third party: %w", err)
	}
	if n.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

// LinkDORAThirdPartyControl adds a control link (idempotent via ON CONFLICT DO NOTHING).
func (r *Repository) LinkDORAThirdPartyControl(ctx context.Context, orgID, thirdPartyID, controlID string) error {
	// Verify ownership first.
	var exists bool
	if err := r.db.QueryRow(ctx,
		`SELECT EXISTS(SELECT 1 FROM dora_third_parties WHERE id=$1 AND org_id=$2)`,
		thirdPartyID, orgID).Scan(&exists); err != nil {
		return err
	}
	if !exists {
		return ErrNotFound
	}
	_, err := r.db.Exec(ctx,
		`INSERT INTO dora_third_party_controls (third_party_id, control_id) VALUES ($1,$2)
		 ON CONFLICT DO NOTHING`,
		thirdPartyID, controlID)
	return err
}

// UnlinkDORAThirdPartyControl removes a control link.
func (r *Repository) UnlinkDORAThirdPartyControl(ctx context.Context, orgID, thirdPartyID, controlID string) error {
	var exists bool
	if err := r.db.QueryRow(ctx,
		`SELECT EXISTS(SELECT 1 FROM dora_third_parties WHERE id=$1 AND org_id=$2)`,
		thirdPartyID, orgID).Scan(&exists); err != nil {
		return err
	}
	if !exists {
		return ErrNotFound
	}
	_, err := r.db.Exec(ctx,
		`DELETE FROM dora_third_party_controls WHERE third_party_id=$1 AND control_id=$2`,
		thirdPartyID, controlID)
	return err
}

// scanDORAThirdParty reads a DORAThirdParty from a pgx Row/Rows.
func scanDORAThirdParty(row interface {
	Scan(...any) error
}) (DORAThirdParty, error) {
	var tp DORAThirdParty
	var contractStart, contractEnd *string
	var slaRTO *int32
	var slaAvail *float64
	var createdBy *string

	err := row.Scan(
		&tp.ID, &tp.OrgID, &tp.Name, &tp.ServiceType, &tp.Criticality,
		&contractStart, &contractEnd,
		&slaRTO, &slaAvail,
		&tp.HasSubcontractors, &tp.SubcontractorNames,
		&tp.DataLocation, &tp.ExitStrategy, &tp.ExitNotes, &tp.Notes,
		&createdBy, &tp.CreatedAt, &tp.UpdatedAt,
	)
	if err != nil {
		return tp, err
	}
	tp.ContractStart = contractStart
	tp.ContractEnd = contractEnd
	if slaRTO != nil {
		v := int(*slaRTO)
		tp.SLARTOHours = &v
	}
	tp.SLAAvailability = slaAvail
	tp.CreatedBy = createdBy
	return tp, nil
}
