// Copyright (c) 2026 NorvikOps. All rights reserved.
// SPDX-License-Identifier: Elastic-2.0

package policy

import (
	"context"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5/pgtype"

	"github.com/matharnica/vakt/internal/db"
)

// evidenceFields is the union of columns returned by all Evidence-returning
// sqlc queries (Add/List/GetExpiring). Identical shape, so one container.
// Duplicated from the parent vaktcomply package for the policy-domain methods
// (evidence counts, collector upsert) that compliance readiness depends on.
type evidenceFields struct {
	ID               string
	ControlID        pgtype.UUID
	OrgID            string
	Title            string
	Description      pgtype.Text
	Source           string
	FilePath         pgtype.Text
	FileSize         pgtype.Int8
	Status           string
	Version          int32
	ExpiresAt        pgtype.Timestamptz
	ExpiryNotifiedAt pgtype.Timestamptz
	CreatedAt        pgtype.Timestamptz
	UpdatedAt        pgtype.Timestamptz
}

func evidenceFromFields(f evidenceFields) Evidence {
	var controlID string
	if f.ControlID.Valid {
		controlID = f.ControlID.String()
	}
	return Evidence{
		ID:               f.ID,
		ControlID:        controlID,
		OrgID:            f.OrgID,
		Title:            f.Title,
		Description:      f.Description.String,
		Source:           f.Source,
		FilePath:         f.FilePath.String,
		FileSize:         f.FileSize.Int64,
		Status:           f.Status,
		Version:          int(f.Version),
		ExpiresAt:        ckTsToTimePtr(f.ExpiresAt),
		ExpiryNotifiedAt: ckTsToTimePtr(f.ExpiryNotifiedAt),
		CreatedAt:        ckTsToTime(f.CreatedAt),
		UpdatedAt:        ckTsToTime(f.UpdatedAt),
	}
}

// CountEvidenceByControl returns the number of approved evidence items per control for a framework.
// Result: map[controlUUID]count.
func (r *Repository) CountEvidenceByControl(ctx context.Context, orgID, frameworkID string) (map[string]int, error) {
	rows, err := r.q.CountCKEvidenceByControl(ctx, db.CountCKEvidenceByControlParams{OrgID: orgID, FrameworkID: frameworkID})
	if err != nil {
		return nil, fmt.Errorf("count evidence by control: %w", err)
	}
	counts := make(map[string]int, len(rows))
	for _, row := range rows {
		counts[row.ControlID] = int(row.EvidenceCount)
	}
	return counts, nil
}

// GetExpiringEvidence returns evidence items expiring within the given threshold for a framework.
func (r *Repository) GetExpiringEvidence(ctx context.Context, orgID, frameworkID string, threshold time.Time) ([]Evidence, error) {
	rows, err := r.q.GetCKExpiringEvidence(ctx, db.GetCKExpiringEvidenceParams{
		OrgID:       orgID,
		FrameworkID: frameworkID,
		ExpiresAt:   pgtype.Timestamptz{Time: threshold, Valid: true},
	})
	if err != nil {
		return nil, fmt.Errorf("get expiring evidence: %w", err)
	}
	out := make([]Evidence, 0, len(rows))
	for _, row := range rows {
		out = append(out, evidenceFromFields(evidenceFields{
			ID: row.ID, ControlID: row.ControlID, OrgID: row.OrgID, Title: row.Title,
			Description: row.Description, Source: row.Source, FilePath: row.FilePath,
			FileSize: row.FileSize, Status: row.Status, Version: row.Version,
			ExpiresAt: row.ExpiresAt, ExpiryNotifiedAt: row.ExpiryNotifiedAt,
			CreatedAt: row.CreatedAt, UpdatedAt: row.UpdatedAt,
		}))
	}
	return out, nil
}

// AddCollectorEvidence upserts an automated/collector evidence item for a control.
func (r *Repository) AddCollectorEvidence(ctx context.Context, orgID, controlID, userID, source, title string, data []byte) (*Evidence, error) {
	row, err := r.q.AddCKCollectorEvidence(ctx, db.AddCKCollectorEvidenceParams{
		ControlID:     ckOptUUIDFromStr(controlID),
		OrgID:         orgID,
		Title:         title,
		Source:        source,
		CollectorData: data,
		UploadedBy:    ckOptUUIDFromStr(userID),
	})
	if err != nil {
		return nil, fmt.Errorf("add collector evidence: %w", err)
	}
	ev := evidenceFromFields(evidenceFields{
		ID: row.ID, ControlID: row.ControlID, OrgID: row.OrgID, Title: row.Title,
		Description: row.Description, Source: row.Source, FilePath: row.FilePath,
		FileSize: row.FileSize, Status: row.Status, Version: row.Version,
		ExpiresAt: row.ExpiresAt, ExpiryNotifiedAt: row.ExpiryNotifiedAt,
		CreatedAt: row.CreatedAt, UpdatedAt: row.UpdatedAt,
	})
	return &ev, nil
}
