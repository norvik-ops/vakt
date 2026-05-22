// Copyright (c) 2026 NorvikOps. All rights reserved.
// SPDX-License-Identifier: Elastic-2.0

package secvitals

import "context"

// GetSoAEntries returns all controls for the org's frameworks with SoA metadata.
func (s *Service) GetSoAEntries(ctx context.Context, orgID string) ([]SoAEntry, error) {
	return s.repo.GetSoAEntries(ctx, orgID)
}

// UpdateSoAApplicability sets the applicability and justification for a control.
func (s *Service) UpdateSoAApplicability(ctx context.Context, orgID, controlID string, applicable bool, justYes, justNo string) error {
	return s.repo.UpdateSoAApplicability(ctx, orgID, controlID, applicable, justYes, justNo)
}
