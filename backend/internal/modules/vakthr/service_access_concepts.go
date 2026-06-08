// Copyright (c) 2026 NorvikOps. All rights reserved.
// SPDX-License-Identifier: Elastic-2.0

package vakthr

import (
	"context"
	"encoding/json"
)

// CreateAccessConcept creates a new Berechtigungskonzept document.
func (s *Service) CreateAccessConcept(ctx context.Context, orgID string, in CreateAccessConceptInput) (AccessConcept, error) {
	return s.repo.CreateAccessConcept(ctx, orgID, in)
}

// ListAccessConcepts returns all access concepts for the organisation.
func (s *Service) ListAccessConcepts(ctx context.Context, orgID string) ([]AccessConcept, error) {
	return s.repo.ListAccessConcepts(ctx, orgID)
}

// GetAccessConcept returns a single access concept by ID.
func (s *Service) GetAccessConcept(ctx context.Context, orgID, id string) (AccessConcept, error) {
	return s.repo.GetAccessConcept(ctx, orgID, id)
}

// UpdateAccessConcept updates the metadata of an access concept.
func (s *Service) UpdateAccessConcept(ctx context.Context, orgID, id string, in UpdateAccessConceptInput) (AccessConcept, error) {
	return s.repo.UpdateAccessConcept(ctx, orgID, id, in)
}

// DeleteAccessConcept removes an access concept.
func (s *Service) DeleteAccessConcept(ctx context.Context, orgID, id string) error {
	return s.repo.DeleteAccessConcept(ctx, orgID, id)
}

// AddAccessRole adds a new role definition to an access concept.
// It verifies the concept belongs to the organisation first.
func (s *Service) AddAccessRole(ctx context.Context, orgID, conceptID string, in CreateAccessRoleInput) (AccessRole, error) {
	if _, err := s.repo.GetAccessConcept(ctx, orgID, conceptID); err != nil {
		return AccessRole{}, err
	}
	return s.repo.AddAccessRole(ctx, orgID, conceptID, in)
}

// ListAccessRoles returns all role definitions for an access concept.
func (s *Service) ListAccessRoles(ctx context.Context, orgID, conceptID string) ([]AccessRole, error) {
	if _, err := s.repo.GetAccessConcept(ctx, orgID, conceptID); err != nil {
		return nil, err
	}
	return s.repo.ListAccessRoles(ctx, orgID, conceptID)
}

// UpdateAccessRole updates an existing role definition.
func (s *Service) UpdateAccessRole(ctx context.Context, orgID, roleID string, in UpdateAccessRoleInput) (AccessRole, error) {
	return s.repo.UpdateAccessRole(ctx, orgID, roleID, in)
}

// DeleteAccessRole removes a role definition.
func (s *Service) DeleteAccessRole(ctx context.Context, orgID, roleID string) error {
	return s.repo.DeleteAccessRole(ctx, orgID, roleID)
}

// SnapshotVersion increments the concept version, serialises all current roles
// as JSON, and inserts a new version record.
func (s *Service) SnapshotVersion(ctx context.Context, orgID, conceptID string) (AccessConceptVersionSummary, error) {
	// Verify concept ownership.
	if _, err := s.repo.GetAccessConcept(ctx, orgID, conceptID); err != nil {
		return AccessConceptVersionSummary{}, err
	}

	// Capture all current roles.
	roles, err := s.repo.ListAccessRoles(ctx, orgID, conceptID)
	if err != nil {
		return AccessConceptVersionSummary{}, err
	}

	snap, err := json.Marshal(roles)
	if err != nil {
		return AccessConceptVersionSummary{}, err
	}

	// Increment version counter atomically.
	newVersion, err := s.repo.IncrementConceptVersion(ctx, orgID, conceptID)
	if err != nil {
		return AccessConceptVersionSummary{}, err
	}

	return s.repo.InsertConceptVersion(ctx, orgID, conceptID, newVersion, snap)
}

// ListAccessConceptVersions returns all version summaries for a concept.
func (s *Service) ListAccessConceptVersions(ctx context.Context, orgID, conceptID string) ([]AccessConceptVersionSummary, error) {
	if _, err := s.repo.GetAccessConcept(ctx, orgID, conceptID); err != nil {
		return nil, err
	}
	return s.repo.ListConceptVersions(ctx, orgID, conceptID)
}
