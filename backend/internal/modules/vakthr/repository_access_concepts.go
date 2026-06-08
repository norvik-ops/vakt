// Copyright (c) 2026 NorvikOps. All rights reserved.
// SPDX-License-Identifier: Elastic-2.0

package vakthr

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/matharnica/vakt/internal/db"
)

// --- Access Concepts ---

// CreateAccessConcept inserts a new Berechtigungskonzept for an organisation.
func (r *Repository) CreateAccessConcept(ctx context.Context, orgID string, in CreateAccessConceptInput) (AccessConcept, error) {
	row, err := r.q.CreateHRAccessConcept(ctx, db.CreateHRAccessConceptParams{
		OrgID: orgID,
		Title: in.Title,
		Scope: in.Scope,
		Owner: in.Owner,
	})
	if err != nil {
		return AccessConcept{}, fmt.Errorf("create access concept: %w", err)
	}
	return accessConceptFromRow(row), nil
}

// ListAccessConcepts returns all access concepts for an organisation.
func (r *Repository) ListAccessConcepts(ctx context.Context, orgID string) ([]AccessConcept, error) {
	rows, err := r.q.ListHRAccessConcepts(ctx, orgID)
	if err != nil {
		return nil, fmt.Errorf("list access concepts: %w", err)
	}
	out := make([]AccessConcept, len(rows))
	for i, row := range rows {
		out[i] = accessConceptFromRow(row)
	}
	return out, nil
}

// GetAccessConcept returns a single access concept by ID within an organisation.
func (r *Repository) GetAccessConcept(ctx context.Context, orgID, id string) (AccessConcept, error) {
	row, err := r.q.GetHRAccessConcept(ctx, db.GetHRAccessConceptParams{ID: id, OrgID: orgID})
	if err != nil {
		return AccessConcept{}, fmt.Errorf("get access concept: %w", err)
	}
	return accessConceptFromRow(row), nil
}

// UpdateAccessConcept updates the metadata of an existing access concept.
func (r *Repository) UpdateAccessConcept(ctx context.Context, orgID, id string, in UpdateAccessConceptInput) (AccessConcept, error) {
	row, err := r.q.UpdateHRAccessConcept(ctx, db.UpdateHRAccessConceptParams{
		ID:    id,
		OrgID: orgID,
		Title: in.Title,
		Scope: in.Scope,
		Owner: in.Owner,
	})
	if err != nil {
		return AccessConcept{}, fmt.Errorf("update access concept: %w", err)
	}
	return accessConceptFromRow(row), nil
}

// DeleteAccessConcept removes an access concept and returns an error if not found.
func (r *Repository) DeleteAccessConcept(ctx context.Context, orgID, id string) error {
	n, err := r.q.DeleteHRAccessConcept(ctx, db.DeleteHRAccessConceptParams{ID: id, OrgID: orgID})
	if err != nil {
		return fmt.Errorf("delete access concept: %w", err)
	}
	if n == 0 {
		return fmt.Errorf("access concept not found")
	}
	return nil
}

// --- Access Roles ---

// AddAccessRole inserts a new role definition into an access concept.
func (r *Repository) AddAccessRole(ctx context.Context, orgID, conceptID string, in CreateAccessRoleInput) (AccessRole, error) {
	row, err := r.q.AddHRAccessRole(ctx, db.AddHRAccessRoleParams{
		ConceptID:            conceptID,
		OrgID:                orgID,
		RoleName:             in.RoleName,
		SystemName:           in.SystemName,
		AccessLevel:          in.AccessLevel,
		Justification:        in.Justification,
		ReviewIntervalMonths: in.ReviewIntervalMonths,
	})
	if err != nil {
		return AccessRole{}, fmt.Errorf("add access role: %w", err)
	}
	return accessRoleFromRow(row), nil
}

// ListAccessRoles returns all role definitions for an access concept.
func (r *Repository) ListAccessRoles(ctx context.Context, orgID, conceptID string) ([]AccessRole, error) {
	rows, err := r.q.ListHRAccessRoles(ctx, db.ListHRAccessRolesParams{ConceptID: conceptID, OrgID: orgID})
	if err != nil {
		return nil, fmt.Errorf("list access roles: %w", err)
	}
	out := make([]AccessRole, len(rows))
	for i, row := range rows {
		out[i] = accessRoleFromRow(row)
	}
	return out, nil
}

// UpdateAccessRole updates an existing role definition.
func (r *Repository) UpdateAccessRole(ctx context.Context, orgID, roleID string, in UpdateAccessRoleInput) (AccessRole, error) {
	row, err := r.q.UpdateHRAccessRole(ctx, db.UpdateHRAccessRoleParams{
		ID:                   roleID,
		OrgID:                orgID,
		RoleName:             in.RoleName,
		SystemName:           in.SystemName,
		AccessLevel:          in.AccessLevel,
		Justification:        in.Justification,
		ReviewIntervalMonths: in.ReviewIntervalMonths,
	})
	if err != nil {
		return AccessRole{}, fmt.Errorf("update access role: %w", err)
	}
	return accessRoleFromRow(row), nil
}

// DeleteAccessRole removes a single role definition.
func (r *Repository) DeleteAccessRole(ctx context.Context, orgID, roleID string) error {
	n, err := r.q.DeleteHRAccessRole(ctx, db.DeleteHRAccessRoleParams{ID: roleID, OrgID: orgID})
	if err != nil {
		return fmt.Errorf("delete access role: %w", err)
	}
	if n == 0 {
		return fmt.Errorf("access role not found")
	}
	return nil
}

// --- Versions ---

// IncrementConceptVersion increments current_version and returns the new number.
func (r *Repository) IncrementConceptVersion(ctx context.Context, orgID, conceptID string) (int32, error) {
	v, err := r.q.IncrementHRAccessConceptVersion(ctx, db.IncrementHRAccessConceptVersionParams{
		ID:    conceptID,
		OrgID: orgID,
	})
	if err != nil {
		return 0, fmt.Errorf("increment concept version: %w", err)
	}
	return v, nil
}

// InsertConceptVersion stores a snapshot of the current roles as a version record.
func (r *Repository) InsertConceptVersion(ctx context.Context, orgID, conceptID string, versionNumber int32, snapshot json.RawMessage) (AccessConceptVersionSummary, error) {
	row, err := r.q.InsertHRAccessConceptVersion(ctx, db.InsertHRAccessConceptVersionParams{
		ConceptID:     conceptID,
		OrgID:         orgID,
		VersionNumber: versionNumber,
		Snapshot:      snapshot,
	})
	if err != nil {
		return AccessConceptVersionSummary{}, fmt.Errorf("insert concept version: %w", err)
	}
	return AccessConceptVersionSummary{
		ID:            row.ID,
		ConceptID:     row.ConceptID,
		VersionNumber: row.VersionNumber,
		CreatedAt:     tsToTime(row.CreatedAt),
	}, nil
}

// ListConceptVersions returns all version summaries for a concept (newest first).
func (r *Repository) ListConceptVersions(ctx context.Context, orgID, conceptID string) ([]AccessConceptVersionSummary, error) {
	rows, err := r.q.ListHRAccessConceptVersions(ctx, db.ListHRAccessConceptVersionsParams{
		ConceptID: conceptID,
		OrgID:     orgID,
	})
	if err != nil {
		return nil, fmt.Errorf("list concept versions: %w", err)
	}
	out := make([]AccessConceptVersionSummary, len(rows))
	for i, row := range rows {
		out[i] = AccessConceptVersionSummary{
			ID:            row.ID,
			ConceptID:     row.ConceptID,
			VersionNumber: row.VersionNumber,
			CreatedAt:     tsToTime(row.CreatedAt),
		}
	}
	return out, nil
}

// --- mapping helpers ---

func accessConceptFromRow(row db.HrAccessConcepts) AccessConcept {
	return AccessConcept{
		ID:             row.ID,
		OrgID:          row.OrgID,
		Title:          row.Title,
		Scope:          row.Scope,
		Owner:          row.Owner,
		CurrentVersion: row.CurrentVersion,
		CreatedAt:      tsToTime(row.CreatedAt),
		UpdatedAt:      tsToTime(row.UpdatedAt),
	}
}

func accessRoleFromRow(row db.HrAccessRoles) AccessRole {
	return AccessRole{
		ID:                   row.ID,
		ConceptID:            row.ConceptID,
		OrgID:                row.OrgID,
		RoleName:             row.RoleName,
		SystemName:           row.SystemName,
		AccessLevel:          row.AccessLevel,
		Justification:        row.Justification,
		ReviewIntervalMonths: row.ReviewIntervalMonths,
		CreatedAt:            tsToTime(row.CreatedAt),
		UpdatedAt:            tsToTime(row.UpdatedAt),
	}
}
