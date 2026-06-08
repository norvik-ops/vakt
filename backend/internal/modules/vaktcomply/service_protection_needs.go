// Copyright (c) 2026 NorvikOps. All rights reserved.
// SPDX-License-Identifier: Elastic-2.0

package vaktcomply

import "context"

// CreateProtectionNeedAssessment creates a new Schutzbedarfsfeststellung record.
func (s *Service) CreateProtectionNeedAssessment(ctx context.Context, orgID string, in CreateProtectionNeedInput) (ProtectionNeedAssessment, error) {
	return s.repo.CreateProtectionNeedAssessment(ctx, orgID, in)
}

// ListProtectionNeedAssessments returns all assessments for the organisation.
func (s *Service) ListProtectionNeedAssessments(ctx context.Context, orgID string) ([]ProtectionNeedAssessment, error) {
	return s.repo.ListProtectionNeedAssessments(ctx, orgID)
}

// GetProtectionNeedAssessment returns a single assessment by ID.
func (s *Service) GetProtectionNeedAssessment(ctx context.Context, orgID, id string) (ProtectionNeedAssessment, error) {
	return s.repo.GetProtectionNeedAssessment(ctx, orgID, id)
}

// UpdateProtectionNeedAssessment sets C/I/A ratings on a draft assessment.
func (s *Service) UpdateProtectionNeedAssessment(ctx context.Context, orgID, id string, in UpdateProtectionNeedInput) (ProtectionNeedAssessment, error) {
	return s.repo.UpdateProtectionNeedAssessment(ctx, orgID, id, in)
}

// FinalizeProtectionNeedAssessment transitions an assessment to 'finalized'.
func (s *Service) FinalizeProtectionNeedAssessment(ctx context.Context, orgID, id string) (ProtectionNeedAssessment, error) {
	return s.repo.FinalizeProtectionNeedAssessment(ctx, orgID, id)
}

// DeleteProtectionNeedAssessment removes an assessment record.
func (s *Service) DeleteProtectionNeedAssessment(ctx context.Context, orgID, id string) error {
	return s.repo.DeleteProtectionNeedAssessment(ctx, orgID, id)
}

// CalculateOverallProtectionNeed implements the BSI maximum principle:
// normal < hoch < sehr_hoch — the overall level equals the highest individual rating.
func CalculateOverallProtectionNeed(c, i, a string) string {
	order := map[string]int{"normal": 0, "hoch": 1, "sehr_hoch": 2}
	max, result := 0, "normal"
	for _, v := range []string{c, i, a} {
		if order[v] > max {
			max = order[v]
			result = v
		}
	}
	return result
}
