// Copyright (c) 2026 NorvikOps. All rights reserved.
// SPDX-License-Identifier: Elastic-2.0

package secvitals

import (
	"context"

	cloud "github.com/matharnica/vakt/internal/shared/integrations/cloud"
)

// CloudEvidenceWriter adapts secvitals.Repository to implement cloud.EvidenceWriter.
// This keeps the shared/integrations/cloud package free of any secvitals import.
type CloudEvidenceWriter struct {
	repo *Repository
}

// NewCloudEvidenceWriter creates a CloudEvidenceWriter backed by the given Repository.
func NewCloudEvidenceWriter(repo *Repository) *CloudEvidenceWriter {
	return &CloudEvidenceWriter{repo: repo}
}

// FindControlsByKeywords delegates to the repository and maps []Control → []cloud.ControlMatch.
func (w *CloudEvidenceWriter) FindControlsByKeywords(ctx context.Context, orgID string, keywords []string) ([]cloud.ControlMatch, error) {
	controls, err := w.repo.FindControlsByKeywords(ctx, orgID, keywords)
	if err != nil {
		return nil, err
	}
	out := make([]cloud.ControlMatch, 0, len(controls))
	for _, c := range controls {
		out = append(out, cloud.ControlMatch{
			ID:    c.ID,
			Title: c.Title,
		})
	}
	return out, nil
}

// AddCollectorEvidence delegates to the repository, discarding the returned evidence record.
func (w *CloudEvidenceWriter) AddCollectorEvidence(ctx context.Context, orgID, controlID, userID, source, title string, data []byte) error {
	_, err := w.repo.AddCollectorEvidence(ctx, orgID, controlID, userID, source, title, data)
	return err
}
