// Copyright (c) 2026 NorvikOps. All rights reserved.
// SPDX-License-Identifier: Elastic-2.0

package vaktcomply

import (
	"context"

	"github.com/jackc/pgx/v5/pgtype"

	"github.com/matharnica/vakt/internal/db"
)

// S121-F6 (A3): the export/calendar handlers used to call h.q.* (sqlc) directly,
// breaking the "handlers call the service layer only" rule (CLAUDE.md). These
// service methods own the queries; the handler now calls them and only shapes
// the HTTP/iCal/DTO output.

// ListPolicyTemplates returns DB-backed compliance templates, optionally filtered
// by category ("policy" | "dpia" | "avv"; empty = all).
func (s *Service) ListPolicyTemplates(ctx context.Context, category string) ([]DBPolicyTemplate, error) {
	arg := db.ListCKPolicyTemplatesParams{}
	if category != "" {
		arg.Category = pgtype.Text{String: category, Valid: true}
	}
	rows, err := s.q.ListCKPolicyTemplates(ctx, arg)
	if err != nil {
		return nil, err
	}
	out := make([]DBPolicyTemplate, 0, len(rows))
	for _, r := range rows {
		out = append(out, templateListRowToDTO(r))
	}
	return out, nil
}

// GetPolicyTemplate returns a single DB-backed template by UUID. A missing row
// surfaces as the ErrNotFound sentinel so the handler can answer 404.
func (s *Service) GetPolicyTemplate(ctx context.Context, id string) (*DBPolicyTemplate, error) {
	r, err := s.q.GetCKPolicyTemplateByID(ctx, id)
	if err != nil {
		if isNotFound(err) {
			return nil, ErrNotFound
		}
		return nil, err
	}
	dto := templateGetRowToDTO(r)
	return &dto, nil
}

// ICalDeadlines bundles the three deadline sources that feed the compliance
// calendar (iCal) export.
type ICalDeadlines struct {
	Milestones []db.ListCKICalMilestonesRow
	CAPAs      []db.ListCKICalCAPAsRow
	Evidence   []db.ListCKICalExpiringEvidenceRow
}

// ListICalDeadlines fetches all deadline sources for an org's compliance calendar.
func (s *Service) ListICalDeadlines(ctx context.Context, orgID string) (ICalDeadlines, error) {
	milestones, err := s.q.ListCKICalMilestones(ctx, orgID)
	if err != nil {
		return ICalDeadlines{}, err
	}
	capas, err := s.q.ListCKICalCAPAs(ctx, orgID)
	if err != nil {
		return ICalDeadlines{}, err
	}
	evidence, err := s.q.ListCKICalExpiringEvidence(ctx, orgID)
	if err != nil {
		return ICalDeadlines{}, err
	}
	return ICalDeadlines{Milestones: milestones, CAPAs: capas, Evidence: evidence}, nil
}
