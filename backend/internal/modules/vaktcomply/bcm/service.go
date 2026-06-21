// Copyright (c) 2026 NorvikOps. All rights reserved.
// SPDX-License-Identifier: Elastic-2.0

package bcm

import (
	"context"

	"github.com/jackc/pgx/v5/pgxpool"
)

// Service provides BCM business logic (BCP, BIA, Recovery Plans, Emergency Contacts).
type Service struct {
	repo *Repository
}

// NewService creates a new BCM service.
func NewService(pool *pgxpool.Pool) *Service {
	return &Service{repo: NewRepository(pool)}
}

// Delegation wrappers for count methods — used by root service's SyncBCMEvidence.

func (s *Service) CountHighCriticalityBIAProcesses(ctx context.Context, orgID string) (int, error) {
	return s.repo.CountHighCriticalityBIAProcesses(ctx, orgID)
}

func (s *Service) CountRecoveryPlansActive(ctx context.Context, orgID string) (int, error) {
	return s.repo.CountRecoveryPlansActive(ctx, orgID)
}

func (s *Service) CountRecoveryPlansTested(ctx context.Context, orgID string) (int, error) {
	return s.repo.CountRecoveryPlansTested(ctx, orgID)
}

func (s *Service) CountEmergencyContacts(ctx context.Context, orgID string) (int, error) {
	return s.repo.CountEmergencyContacts(ctx, orgID)
}
