// Copyright (c) 2026 NorvikOps. All rights reserved.
// SPDX-License-Identifier: Elastic-2.0

package vaktcomply

import (
	"context"
	"fmt"
	"time"

	"github.com/matharnica/vakt/internal/shared/notify"
	"github.com/matharnica/vakt/internal/shared/safego"
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
	return s.repo.GetRiskFull(ctx, orgID, id)
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
		title := risk.Title
		safego.Run(ctx, "vaktcomply.risk.notify_owner", func(ctx context.Context) error {
			notifyCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
			defer cancel()
			notify.Send(notifyCtx, s.db, orgID,
				"Risiko zugewiesen",
				fmt.Sprintf("Das Risiko '%s' wurde Ihnen zugewiesen.", title),
				"info", "vaktcomply")
			return nil
		})
	}
	return risk, nil
}

// UpdateRiskTreatment patches the treatment workflow fields of a risk (ISO 27001 Clause 6).
func (s *Service) UpdateRiskTreatment(ctx context.Context, orgID, id string, in UpdateRiskTreatmentInput) (*Risk, error) {
	return s.repo.UpdateRiskTreatment(ctx, orgID, id, in)
}

func (s *Service) DeleteRisk(ctx context.Context, orgID, id string) error {
	if err := s.repo.DeleteRisk(ctx, orgID, id); err != nil {
		return err
	}
	s.invalidateDashboardCache(ctx, orgID)
	return nil
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

// UpdateRiskResidualFields patches the inherent/residual likelihood+impact columns (S61-4).
func (s *Service) UpdateRiskResidualFields(ctx context.Context, orgID, id string, in UpdateRiskResidualInput) error {
	return s.repo.UpdateRiskResidualFields(ctx, orgID, id, in)
}

// AcceptRisk records a formal risk acceptance for a risk with treatment_status=accepted (S61-4).
func (s *Service) AcceptRisk(ctx context.Context, orgID, id, userID string, in AcceptRiskInput) error {
	return s.repo.AcceptRisk(ctx, orgID, id, userID, in.Justification)
}
