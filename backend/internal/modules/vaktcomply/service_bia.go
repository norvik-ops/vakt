// Copyright (c) 2026 NorvikOps. All rights reserved.
// SPDX-License-Identifier: Elastic-2.0

package vaktcomply

import (
	"context"
	"errors"
	"fmt"
)

// ── BIA ───────────────────────────────────────────────────────────────────────

var (
	ErrRPOExceedsRTO     = errors.New("rpo_hours must be less than or equal to rto_hours")
	ErrMBCOOutOfRange    = errors.New("mbco_percent must be between 0 and 100")
	ErrStepsOrderInvalid = errors.New("steps order must be sequential starting at 1")
	ErrRTORequired       = errors.New("rto_hours must be greater than 0")
)

func (s *Service) CreateBIAProcess(ctx context.Context, orgID string, in CreateBIAProcessInput) (BIAProcess, error) {
	if err := validateBIAProcess(in.RTOHours, in.RPOHours, in.MBCOPercent); err != nil {
		return BIAProcess{}, err
	}
	return s.repo.CreateBIAProcess(ctx, orgID, in)
}

func (s *Service) ListBIAProcesses(ctx context.Context, orgID string) ([]BIAProcess, error) {
	return s.repo.ListBIAProcesses(ctx, orgID)
}

func (s *Service) GetBIAProcess(ctx context.Context, orgID, id string) (BIAProcess, error) {
	return s.repo.GetBIAProcess(ctx, orgID, id)
}

func (s *Service) UpdateBIAProcess(ctx context.Context, orgID, id string, in UpdateBIAProcessInput) (BIAProcess, error) {
	if err := validateBIAProcess(in.RTOHours, in.RPOHours, in.MBCOPercent); err != nil {
		return BIAProcess{}, err
	}
	return s.repo.UpdateBIAProcess(ctx, orgID, id, in)
}

func (s *Service) DeleteBIAProcess(ctx context.Context, orgID, id string) error {
	return s.repo.DeleteBIAProcess(ctx, orgID, id)
}

func (s *Service) GetBIASummary(ctx context.Context, orgID string) (BIASummary, error) {
	return s.repo.GetBIASummary(ctx, orgID)
}

func validateBIAProcess(rtoHours, rpoHours, mbcoPercent int) error {
	if rpoHours > rtoHours {
		return ErrRPOExceedsRTO
	}
	if mbcoPercent < 0 || mbcoPercent > 100 {
		return ErrMBCOOutOfRange
	}
	return nil
}

// ── Recovery Plans ────────────────────────────────────────────────────────────

func (s *Service) CreateRecoveryPlan(ctx context.Context, orgID string, in CreateRecoveryPlanInput) (RecoveryPlan, error) {
	if err := validateRecoveryPlan(in.RTOHours, in.Steps); err != nil {
		return RecoveryPlan{}, err
	}
	return s.repo.CreateRecoveryPlan(ctx, orgID, in)
}

func (s *Service) ListRecoveryPlans(ctx context.Context, orgID string) ([]RecoveryPlan, error) {
	return s.repo.ListRecoveryPlans(ctx, orgID)
}

func (s *Service) ListRecoveryPlansByBIAProcess(ctx context.Context, orgID, biaProcessID string) ([]RecoveryPlan, error) {
	return s.repo.ListRecoveryPlansByBIAProcess(ctx, orgID, biaProcessID)
}

func (s *Service) GetRecoveryPlan(ctx context.Context, orgID, id string) (RecoveryPlan, error) {
	return s.repo.GetRecoveryPlan(ctx, orgID, id)
}

func (s *Service) UpdateRecoveryPlan(ctx context.Context, orgID, id string, in UpdateRecoveryPlanInput) (RecoveryPlan, error) {
	if err := validateRecoveryPlan(in.RTOHours, in.Steps); err != nil {
		return RecoveryPlan{}, err
	}
	return s.repo.UpdateRecoveryPlan(ctx, orgID, id, in)
}

func (s *Service) DeleteRecoveryPlan(ctx context.Context, orgID, id string) error {
	return s.repo.DeleteRecoveryPlan(ctx, orgID, id)
}

func validateRecoveryPlan(rtoHours int, steps []RecoveryStep) error {
	if rtoHours <= 0 {
		return ErrRTORequired
	}
	for i, step := range steps {
		if step.Order != i+1 {
			return fmt.Errorf("%w: got %d at index %d", ErrStepsOrderInvalid, step.Order, i)
		}
	}
	return nil
}

// ── Emergency Contacts ────────────────────────────────────────────────────────

func (s *Service) CreateEmergencyContact(ctx context.Context, orgID string, in CreateEmergencyContactInput) (EmergencyContact, error) {
	return s.repo.CreateEmergencyContact(ctx, orgID, in)
}

func (s *Service) ListEmergencyContacts(ctx context.Context, orgID string) ([]EmergencyContact, error) {
	return s.repo.ListEmergencyContacts(ctx, orgID)
}

func (s *Service) UpdateEmergencyContact(ctx context.Context, orgID, id string, in UpdateEmergencyContactInput) (EmergencyContact, error) {
	return s.repo.UpdateEmergencyContact(ctx, orgID, id, in)
}

func (s *Service) DeleteEmergencyContact(ctx context.Context, orgID, id string) error {
	return s.repo.DeleteEmergencyContact(ctx, orgID, id)
}

// ── BCM Readiness Score ───────────────────────────────────────────────────────

// GetBCMReadinessScore computes a 0–100 readiness score based on 5 criteria (20pts each):
// 1. ≥1 active BCP plan
// 2. BIA with ≥1 critical process
// 3. Every critical process has a WAP
// 4. ≥1 tested WAP in last 12 months
// 5. Alarmierungsplan ≥3 contacts
func (s *Service) GetBCMReadinessScore(ctx context.Context, orgID string) (BCMReadinessScore, error) {
	criteria := []BCMCriterion{
		{Key: "active_bcp_plan", Points: 20},
		{Key: "bia_critical_processes", Points: 20},
		{Key: "critical_processes_have_wap", Points: 20},
		{Key: "wap_tested_12m", Points: 20},
		{Key: "alarmierungsplan_contacts", Points: 20},
	}

	// Criterion 1: ≥1 active BCP plan
	bcpPlans, err := s.repo.ListBCPPlans(ctx, orgID)
	if err == nil {
		for _, p := range bcpPlans {
			if p.Status == "active" {
				criteria[0].Met = true
				break
			}
		}
	}

	// Criterion 2: BIA with ≥1 critical process
	highCount, err := s.repo.CountHighCriticalityBIAProcesses(ctx, orgID)
	if err == nil && highCount > 0 {
		criteria[1].Met = true
	}

	// Criterion 3: Every critical process has a WAP (at least 1 WAP linked to a high process)
	if criteria[1].Met {
		coveredCount, err := s.repo.CountRecoveryPlansForHighCriticality(ctx, orgID)
		if err == nil && coveredCount >= highCount {
			criteria[2].Met = true
		}
	}

	// Criterion 4: ≥1 tested WAP in last 12 months
	testedCount, err := s.repo.CountRecoveryPlansTested(ctx, orgID)
	if err == nil && testedCount > 0 {
		criteria[3].Met = true
	}

	// Criterion 5: ≥3 emergency contacts
	contactCount, err := s.repo.CountEmergencyContacts(ctx, orgID)
	if err == nil && contactCount >= 3 {
		criteria[4].Met = true
	}

	score := 0
	for _, c := range criteria {
		if c.Met {
			score += c.Points
		}
	}

	return BCMReadinessScore{Score: score, Criteria: criteria}, nil
}
