// Copyright (c) 2026 NorvikOps. All rights reserved.
// SPDX-License-Identifier: Elastic-2.0

package vaktcomply

import (
	"context"
	"fmt"
)

// --- Risk ↔ Control Links ---
// The risk register itself now lives in the risk/ sub-package (accessed via
// s.Risk). The methods below stay in root because they return/use the shared
// Control type.

func (s *Service) LinkRiskControl(ctx context.Context, orgID, riskID, controlID string) error {
	return s.repo.LinkRiskControl(ctx, orgID, riskID, controlID)
}

func (s *Service) UnlinkRiskControl(ctx context.Context, orgID, riskID, controlID string) error {
	return s.repo.UnlinkRiskControl(ctx, orgID, riskID, controlID)
}

func (s *Service) ListRiskControls(ctx context.Context, orgID, riskID string) ([]Control, error) {
	controls, err := s.repo.ListRiskControls(ctx, orgID, riskID)
	if err != nil {
		return nil, fmt.Errorf("list risk controls: %w", err)
	}
	if controls == nil {
		controls = []Control{}
	}
	return controls, nil
}
