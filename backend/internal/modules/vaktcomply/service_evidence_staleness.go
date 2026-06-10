// Copyright (c) 2026 NorvikOps. All rights reserved.
// SPDX-License-Identifier: Elastic-2.0

package vaktcomply

import (
	"context"
	"fmt"
	"time"

	"github.com/rs/zerolog/log"
)

const TaskEvidenceStalenessCheck = "comply:evidence_staleness_check"

// RunStalenessCheck updates evidence_status for every control in the org
// based on evidence age vs evidence_max_age_days.
func (s *Service) RunStalenessCheck(ctx context.Context, orgID string) error {
	n, err := s.repo.UpdateEvidenceStaleness(ctx, orgID)
	if err != nil {
		return fmt.Errorf("evidence staleness check: %w", err)
	}
	log.Info().Str("org_id", orgID).Int("updated", n).Msg("evidence staleness check complete")
	return nil
}

// GetComplianceScore returns the compliance score for an org, counting stale evidence as not-ok.
func (s *Service) GetComplianceScore(ctx context.Context, orgID string) (*ComplianceScore, error) {
	return s.repo.GetComplianceScore(ctx, orgID)
}

// SetControlMaxAge sets the evidence_max_age_days for a control (org override).
func (s *Service) SetControlMaxAge(ctx context.Context, orgID, controlID string, maxAgeDays *int) error {
	return s.repo.SetControlMaxAge(ctx, orgID, controlID, maxAgeDays)
}

// ComplianceScore holds the aggregated compliance score for an org.
type ComplianceScore struct {
	TotalControls int     `json:"total_controls"`
	OkCount       int     `json:"ok_count"`
	StaleCount    int     `json:"stale_count"`
	MissingCount  int     `json:"missing_count"`
	NACount       int     `json:"na_count"`
	ScorePct      float64 `json:"score_pct"`
	AsOf          string  `json:"as_of"`
}

// DefaultMaxAgeDays returns the recommended evidence max age for a given evidence type.
func DefaultMaxAgeDays(evidenceType string) int {
	defaults := map[string]int{
		"scanner":           7,
		"cloud":             2,
		"identity":          7,
		"phishing":          90,
		"policy":            365,
		"pentest":           365,
		"manual":            180,
		"bcp_test":          365,
		"management_review": 365,
	}
	if d, ok := defaults[evidenceType]; ok {
		return d
	}
	return 180
}

// ListStaleControls returns controls with evidence_status = 'stale'.
func (s *Service) ListStaleControls(ctx context.Context, orgID string) ([]Control, error) {
	return s.repo.ListStaleControls(ctx, orgID)
}

// repository methods referenced from this service are defined in repository_evidence_staleness.go.
var _ = time.Now // keep time import
