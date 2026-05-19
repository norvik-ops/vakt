// Copyright (c) 2026 NorvikOps. All rights reserved.
// SPDX-License-Identifier: Elastic-2.0

package secvitals

import (
	"context"
	"fmt"
	"time"

	"github.com/rs/zerolog/log"
)

// GetEvidenceHistory returns the audit trail for a single evidence item.
func (s *Service) GetEvidenceHistory(ctx context.Context, orgID, evidenceID string) ([]EvidenceHistoryEntry, error) {
	return s.repo.ListEvidenceHistory(ctx, orgID, evidenceID)
}

// recordEvidenceHistory inserts a history row for an evidence change.
// Errors are logged but not returned — history recording is best-effort.
func (s *Service) recordEvidenceHistory(ctx context.Context, orgID, evidenceID, changedByUserID string, e Evidence, note string) {
	var changedBy *string
	if changedByUserID != "" {
		changedBy = &changedByUserID
	}
	_, err := s.db.Exec(ctx, `
		INSERT INTO ck_evidence_history (evidence_id, org_id, changed_by, title, description, status, file_url, change_note)
		VALUES ($1::uuid, $2::uuid, $3::uuid, $4, $5, $6, $7, $8)`,
		evidenceID, orgID, changedBy, e.Title, e.Description, e.Status, e.FilePath, note,
	)
	if err != nil {
		log.Error().Err(err).Str("evidence_id", evidenceID).Msg("evidence history: record failed")
	}
}

// AddEvidence stores a new evidence item for a control.
func (s *Service) AddEvidence(ctx context.Context, orgID, controlID, userID string, input AddEvidenceInput) (*Evidence, error) {
	// Verify the control belongs to this org.
	if _, err := s.repo.GetControl(ctx, orgID, controlID); err != nil {
		return nil, fmt.Errorf("control not found: %w", err)
	}

	ev, err := s.repo.AddEvidence(ctx, orgID, controlID, userID, input)
	if err != nil {
		return nil, fmt.Errorf("add evidence: %w", err)
	}
	return ev, nil
}

// ListEvidence returns all evidence items for a control.
func (s *Service) ListEvidence(ctx context.Context, orgID, controlID string) ([]Evidence, error) {
	return s.repo.ListEvidence(ctx, orgID, controlID)
}

// ReviewEvidence updates the review status of an evidence item.
// status must be one of: "approved", "rejected".
func (s *Service) ReviewEvidence(ctx context.Context, orgID, evidenceID, reviewerID, status, _ string) error {
	if status != "approved" && status != "rejected" {
		return fmt.Errorf("invalid review status: %s (must be approved or rejected)", status)
	}
	return s.repo.ReviewEvidence(ctx, orgID, evidenceID, reviewerID, status)
}

// GetExpiringEvidenceAll returns evidence expiring within the given number of days, across all frameworks.
func (s *Service) GetExpiringEvidenceAll(ctx context.Context, orgID string, days int) ([]Evidence, error) {
	threshold := time.Now().UTC().AddDate(0, 0, days)
	items, err := s.repo.GetExpiringEvidenceAllFrameworks(ctx, orgID, threshold)
	if err != nil {
		return nil, fmt.Errorf("get expiring evidence all: %w", err)
	}
	if items == nil {
		items = []Evidence{}
	}
	return items, nil
}

// CollectEvidence runs the named collector and stores the result as an evidence item.
func (s *Service) CollectEvidence(ctx context.Context, orgID, controlID, userID string, cfg CollectorConfig) (*Evidence, error) {
	collector, err := GetCollector(cfg.Type)
	if err != nil {
		return nil, err
	}

	data, err := collector.Collect(ctx, cfg)
	if err != nil {
		return nil, fmt.Errorf("collector %s: %w", cfg.Type, err)
	}

	title := fmt.Sprintf("Auto-collected: %s (%s)", cfg.Type, time.Now().UTC().Format(time.DateOnly))
	return s.repo.AddCollectorEvidence(ctx, orgID, controlID, userID, cfg.Type, title, data)
}
