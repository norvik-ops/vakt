// Copyright (c) 2026 NorvikOps. All rights reserved.
// SPDX-License-Identifier: Elastic-2.0

package vaktcomply

import (
	"context"
	"fmt"
)

// CreateOrVersionISMSScope creates a new ISMS scope version for the org.
func (s *Service) CreateOrVersionISMSScope(ctx context.Context, orgID, userID string, in CreateISMSScopeInput) (ISMSScope, error) {
	return s.repo.CreateOrVersionISMSScope(ctx, orgID, userID, in)
}

// GetCurrentISMSScope returns the latest ISMS scope version for the org.
func (s *Service) GetCurrentISMSScope(ctx context.Context, orgID string) (ISMSScope, error) {
	return s.repo.GetCurrentISMSScope(ctx, orgID)
}

// ListISMSScopeVersions returns all ISMS scope versions for the org.
func (s *Service) ListISMSScopeVersions(ctx context.Context, orgID string) ([]ISMSScope, error) {
	return s.repo.ListISMSScopeVersions(ctx, orgID)
}

// ApproveISMSScope approves the specified ISMS scope version.
// Only users with the "admin" role may approve.
func (s *Service) ApproveISMSScope(ctx context.Context, orgID, id, approverID, userRole string) (ISMSScope, error) {
	if userRole != "admin" {
		return ISMSScope{}, fmt.Errorf("only admins may approve the ISMS scope")
	}
	return s.repo.ApproveISMSScope(ctx, orgID, id, approverID)
}
