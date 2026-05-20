// Copyright (c) 2026 NorvikOps. All rights reserved.
// SPDX-License-Identifier: Elastic-2.0

package hr

import (
	"context"
	"time"
)

// ChecklistCompletionEvidence is the payload written to the compliance evidence
// store when a checklist run reaches the "completed" state.
type ChecklistCompletionEvidence struct {
	OrgID         string
	EmployeeName  string
	EmployeeEmail string
	ChecklistName string
	ChecklistType string // "onboarding" | "offboarding"
	RunID         string
	CompletedAt   time.Time
	StepCount     int
}

// EvidenceWriter abstracts writing compliance evidence so the HR module does not
// depend directly on the secvitals module. When secvitals is disabled, a noop
// writer is used.
type EvidenceWriter interface {
	WriteChecklistCompletion(ctx context.Context, in ChecklistCompletionEvidence) error
}

type noopEvidenceWriter struct{}

func (noopEvidenceWriter) WriteChecklistCompletion(_ context.Context, _ ChecklistCompletionEvidence) error {
	return nil
}

// NoopEvidenceWriter returns an EvidenceWriter that silently discards all calls.
// Use this when the secvitals module is disabled.
func NoopEvidenceWriter() EvidenceWriter {
	return noopEvidenceWriter{}
}
