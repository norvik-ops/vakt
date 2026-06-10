// Copyright (c) 2026 NorvikOps. All rights reserved.
// SPDX-License-Identifier: Elastic-2.0

package vakthr

import (
	"context"

	"github.com/matharnica/vakt/internal/shared/platform/events"
)

// EvidenceWriter abstracts writing compliance evidence so the HR module does not
// depend directly on the vaktcomply module. When vaktcomply is disabled, a noop
// writer is used.
type EvidenceWriter interface {
	WriteChecklistCompletion(ctx context.Context, in events.ChecklistCompletionEvidence) error
	WritePersonioOffboardingEvidence(ctx context.Context, in events.PersonioOffboardingEvidence) error
	WriteEvidence(ctx context.Context, orgID, evidenceType, description, entityID string) error
}

type noopEvidenceWriter struct{}

func (noopEvidenceWriter) WriteChecklistCompletion(_ context.Context, _ events.ChecklistCompletionEvidence) error {
	return nil
}

func (noopEvidenceWriter) WritePersonioOffboardingEvidence(_ context.Context, _ events.PersonioOffboardingEvidence) error {
	return nil
}

func (noopEvidenceWriter) WriteEvidence(_ context.Context, _, _, _, _ string) error {
	return nil
}

// NoopEvidenceWriter returns an EvidenceWriter that silently discards all calls.
// Use this when the vaktcomply module is disabled.
func NoopEvidenceWriter() EvidenceWriter {
	return noopEvidenceWriter{}
}
