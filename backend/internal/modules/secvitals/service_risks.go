// Copyright (c) 2026 NorvikOps. All rights reserved.
// SPDX-License-Identifier: Elastic-2.0

package secvitals

import (
	"context"
	"fmt"
	"time"

	"github.com/matharnica/vakt/internal/shared/notify"
)

// --- Risk Assessment (FR-CK12) ---

func (s *Service) ListRisks(ctx context.Context, orgID string) ([]Risk, error) {
	risks, err := s.repo.ListRisks(ctx, orgID)
	if err != nil {
		return nil, fmt.Errorf("list risks: %w", err)
	}
	if risks == nil {
		risks = []Risk{}
	}
	return risks, nil
}

func (s *Service) GetRisk(ctx context.Context, orgID, id string) (*Risk, error) {
	return s.repo.GetRisk(ctx, orgID, id)
}

func (s *Service) CreateRisk(ctx context.Context, orgID string, in CreateRiskInput) (*Risk, error) {
	risk, err := s.repo.CreateRisk(ctx, orgID, in)
	if err != nil {
		return nil, err
	}
	s.invalidateDashboardCache(ctx, orgID)
	return risk, nil
}

func (s *Service) UpdateRisk(ctx context.Context, orgID, id string, in UpdateRiskInput) (*Risk, error) {
	risk, err := s.repo.UpdateRisk(ctx, orgID, id, in)
	if err != nil {
		return nil, err
	}
	s.invalidateDashboardCache(ctx, orgID)
	if in.Owner != "" && risk != nil {
		go func() {
			notifyCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()
			notify.Send(notifyCtx, s.db, orgID,
				"Risiko zugewiesen",
				fmt.Sprintf("Das Risiko '%s' wurde Ihnen zugewiesen.", risk.Title),
				"info", "secvitals")
		}()
	}
	return risk, nil
}

// UpdateRiskTreatment patches the treatment workflow fields of a risk (ISO 27001 Clause 6).
func (s *Service) UpdateRiskTreatment(ctx context.Context, orgID, id string, in UpdateRiskTreatmentInput) (*Risk, error) {
	return s.repo.UpdateRiskTreatment(ctx, orgID, id, in)
}

// --- Risk ↔ Control Links ---

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
