// Copyright (c) 2026 NorvikOps. All rights reserved.
// SPDX-License-Identifier: Elastic-2.0

package vaktcomply

import "context"

// bsiSuggestions maps common asset types to recommended BSI Baustein IDs.
var bsiSuggestions = map[string][]string{
	"server":      {"SYS.1.1", "OPS.1.1.3", "ISMS.1"},
	"workstation": {"SYS.2.1", "OPS.1.2.3", "ISMS.1"},
	"network":     {"NET.1.1", "NET.3.1", "ISMS.1"},
	"application": {"APP.1.1", "OPS.1.1.5", "ISMS.1"},
	"database":    {"APP.4.3", "OPS.1.1.3", "ISMS.1"},
}

// GetBSIModelingMatrix returns the full BSI Baustein-to-Asset matrix for an org.
func (s *Service) GetBSIModelingMatrix(ctx context.Context, orgID string) ([]BSIModelingEntry, error) {
	return s.repo.GetBSIModelingMatrix(ctx, orgID)
}

// CreateBSIModeling creates a new BSI modeling entry.
func (s *Service) CreateBSIModeling(ctx context.Context, orgID, userID string, in CreateBSIModelingInput) (BSIModelingEntry, error) {
	return s.repo.CreateBSIModeling(ctx, orgID, userID, in)
}

// UpdateBSIModeling updates an existing BSI modeling entry.
func (s *Service) UpdateBSIModeling(ctx context.Context, orgID, id string, in UpdateBSIModelingInput) (BSIModelingEntry, error) {
	return s.repo.UpdateBSIModeling(ctx, orgID, id, in)
}

// DeleteBSIModeling removes a BSI modeling entry.
func (s *Service) DeleteBSIModeling(ctx context.Context, orgID, id string) error {
	return s.repo.DeleteBSIModeling(ctx, orgID, id)
}

// GetBSIModelingStats returns aggregate check-status counts for an org's matrix.
func (s *Service) GetBSIModelingStats(ctx context.Context, orgID string) (BSIModelingStats, error) {
	return s.repo.GetBSIModelingStats(ctx, orgID)
}

// GetSuggestedBausteine returns a list of suggested BSI Baustein IDs for a given asset type.
// Falls back to ["ISMS.1"] for unknown types.
func (s *Service) GetSuggestedBausteine(assetType string) []string {
	if suggestions, ok := bsiSuggestions[assetType]; ok {
		return suggestions
	}
	return []string{"ISMS.1"}
}
