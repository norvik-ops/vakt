// Copyright (c) 2026 NorvikOps. All rights reserved.
// SPDX-License-Identifier: Elastic-2.0

package secvitals

import "context"

// ListDORAThirdParties returns the DORA IKT-Drittanbieter for an org.
func (s *Service) ListDORAThirdParties(ctx context.Context, orgID, criticality string) ([]DORAThirdParty, error) {
	return s.repo.ListDORAThirdParties(ctx, orgID, criticality)
}

// GetDORAThirdParty returns a single entry including linked control IDs.
func (s *Service) GetDORAThirdParty(ctx context.Context, orgID, id string) (*DORAThirdParty, error) {
	return s.repo.GetDORAThirdParty(ctx, orgID, id)
}

// CreateDORAThirdParty creates a new entry.
func (s *Service) CreateDORAThirdParty(ctx context.Context, orgID, createdBy string, in CreateDORAThirdPartyInput) (*DORAThirdParty, error) {
	return s.repo.CreateDORAThirdParty(ctx, orgID, createdBy, in)
}

// UpdateDORAThirdParty updates an existing entry.
func (s *Service) UpdateDORAThirdParty(ctx context.Context, orgID, id string, in UpdateDORAThirdPartyInput) (*DORAThirdParty, error) {
	return s.repo.UpdateDORAThirdParty(ctx, orgID, id, in)
}

// DeleteDORAThirdParty removes an entry.
func (s *Service) DeleteDORAThirdParty(ctx context.Context, orgID, id string) error {
	return s.repo.DeleteDORAThirdParty(ctx, orgID, id)
}

// LinkDORAThirdPartyControl adds a control link.
func (s *Service) LinkDORAThirdPartyControl(ctx context.Context, orgID, thirdPartyID, controlID string) error {
	return s.repo.LinkDORAThirdPartyControl(ctx, orgID, thirdPartyID, controlID)
}

// UnlinkDORAThirdPartyControl removes a control link.
func (s *Service) UnlinkDORAThirdPartyControl(ctx context.Context, orgID, thirdPartyID, controlID string) error {
	return s.repo.UnlinkDORAThirdPartyControl(ctx, orgID, thirdPartyID, controlID)
}
