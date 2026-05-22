// Copyright (c) 2026 NorvikOps. All rights reserved.
// SPDX-License-Identifier: Elastic-2.0

package cloud

import "context"

// EvidenceWriter allows the cloud collectors to write compliance evidence
// into a compliance module without depending on it directly.
type EvidenceWriter interface {
	// FindControlsByKeywords returns controls whose tags or keywords match
	// the given keywords for the given org.
	FindControlsByKeywords(ctx context.Context, orgID string, keywords []string) ([]ControlMatch, error)
	// AddCollectorEvidence records an evidence item collected from a cloud provider.
	AddCollectorEvidence(ctx context.Context, orgID, controlID, userID, source, title string, data []byte) error
}

// ControlMatch is a minimal representation of a matched control.
type ControlMatch struct {
	ID    string
	Title string
}

// noopEvidenceWriter is a no-op implementation used when the secvitals module is disabled.
type noopEvidenceWriter struct{}

func (noopEvidenceWriter) FindControlsByKeywords(_ context.Context, _ string, _ []string) ([]ControlMatch, error) {
	return nil, nil
}

func (noopEvidenceWriter) AddCollectorEvidence(_ context.Context, _, _, _, _, _ string, _ []byte) error {
	return nil
}

// NoopEvidenceWriter returns an EvidenceWriter that silently discards all calls.
// Use this when the secvitals module is disabled.
func NoopEvidenceWriter() EvidenceWriter {
	return noopEvidenceWriter{}
}
