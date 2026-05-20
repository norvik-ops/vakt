// Copyright (c) 2026 NorvikOps. All rights reserved.
// SPDX-License-Identifier: Elastic-2.0

package secvitals

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/matharnica/vakt/internal/modules/hr"
)

// HREvidenceWriter persists HR checklist completions as compliance evidence in ck_evidence.
// The control_id is left NULL — the compliance manager links the evidence to a specific
// control (typically ISO 27001 A.6 Personnel Security or BSI ORP.2) via the SecVitals UI.
type HREvidenceWriter struct {
	pool *pgxpool.Pool
}

// NewHREvidenceWriter creates an HREvidenceWriter backed by the given DB pool.
func NewHREvidenceWriter(pool *pgxpool.Pool) *HREvidenceWriter {
	return &HREvidenceWriter{pool: pool}
}

// WriteChecklistCompletion inserts a row into ck_evidence describing the completed run.
func (w *HREvidenceWriter) WriteChecklistCompletion(ctx context.Context, in hr.ChecklistCompletionEvidence) error {
	title := fmt.Sprintf("%s: %s", titleForType(in.ChecklistType), in.EmployeeName)
	description := fmt.Sprintf(
		"Checkliste %q für Mitarbeiter %s (%s) abgeschlossen am %s — %d Schritte erledigt.",
		in.ChecklistName, in.EmployeeName, in.EmployeeEmail,
		in.CompletedAt.Format("02.01.2006 15:04"), in.StepCount,
	)
	data, err := json.Marshal(map[string]any{
		"employee_name":  in.EmployeeName,
		"employee_email": in.EmployeeEmail,
		"checklist_name": in.ChecklistName,
		"checklist_type": in.ChecklistType,
		"run_id":         in.RunID,
		"step_count":     in.StepCount,
		"completed_at":   in.CompletedAt.Format("2006-01-02T15:04:05Z07:00"),
	})
	if err != nil {
		return fmt.Errorf("marshal hr evidence data: %w", err)
	}

	_, err = w.pool.Exec(ctx, `
		INSERT INTO ck_evidence
			(control_id, org_id, title, description, source, collector_data, status)
		VALUES
			(NULL, $1::uuid, $2, $3, 'hr_checklist_completed', $4, 'approved')
	`, in.OrgID, title, description, data)
	if err != nil {
		return fmt.Errorf("insert hr evidence: %w", err)
	}
	return nil
}

func titleForType(t string) string {
	switch t {
	case "onboarding":
		return "Onboarding abgeschlossen"
	case "offboarding":
		return "Offboarding abgeschlossen"
	default:
		return "Checkliste abgeschlossen"
	}
}
