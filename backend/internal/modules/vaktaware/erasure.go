// Copyright (c) 2026 NorvikOps. All rights reserved.
// SPDX-License-Identifier: Elastic-2.0

package vaktaware

import (
	"context"
	"fmt"

	sharedevents "github.com/matharnica/vakt/internal/shared/events"
)

// SubjectEraser erases the personal data vaktaware stores for a data subject —
// phishing-simulation targets, their tracking telemetry (IPs, user agents) and
// campaign enrollments — as part of an Art. 17 DSGVO erasure. It writes ONLY the
// sr_ prefix; vaktprivacy invokes it inside its erasure transaction so no module
// writes across a prefix boundary (module isolation, ADR-0079).
type SubjectEraser struct{}

// NewSubjectEraser returns a stateless vaktaware eraser typed as the shared
// interface. It carries no state — all writes run on the transaction passed in.
func NewSubjectEraser() sharedevents.SubjectErasure { return SubjectEraser{} }

// ModuleName identifies this eraser for the orchestrator's completeness check.
func (SubjectEraser) ModuleName() string { return "vaktaware" }

// EraseSubjectPII deletes sr_campaign_enrollments, sr_events and sr_targets for
// the subject, on the passed transaction.
//
// Order-independent: it touches ONLY the sr_ prefix. The employee ids needed for
// the enrollment delete arrive pre-resolved in subj.EmployeeIDs (resolved by
// vakthr before any eraser ran), so this no longer depends on hr_employees still
// existing — the trap that previously made wiring order load-bearing.
func (SubjectEraser) EraseSubjectPII(ctx context.Context, tx sharedevents.Execer, subj sharedevents.SubjectRef) (sharedevents.ErasureCounts, error) {
	counts := sharedevents.ErasureCounts{}
	orgID, email := subj.OrgID, subj.Email

	// sr_campaign_enrollments.employee_id is TEXT with no FK cascade on
	// hr_employees, so these rows survive the hr_employees delete and would keep
	// PII if we skipped them. `= ANY($2::text[])` on an empty array deletes
	// nothing, which is the correct outcome for a non-employee subject.
	enrollTag, err := tx.Exec(ctx, `
		DELETE FROM sr_campaign_enrollments
		WHERE org_id = $1 AND employee_id = ANY($2::text[])`,
		orgID, subj.EmployeeIDs,
	)
	if err != nil {
		return nil, fmt.Errorf("vaktaware erasure: delete sr_campaign_enrollments: %w", err)
	}
	counts["sr_campaign_enrollments"] = enrollTag.RowsAffected()

	// Tracking events BEFORE targets — the FK is ON DELETE SET NULL, so deleting
	// targets first would null target_id and orphan IP addresses / user-agents in
	// sr_events. Art. 17 requires erasing that telemetry too.
	evTag, err := tx.Exec(ctx, `
		DELETE FROM sr_events
		WHERE target_id IN (
			SELECT id FROM sr_targets
			WHERE org_id = $1::uuid AND lower(email) = lower($2)
		)`, orgID, email,
	)
	if err != nil {
		return nil, fmt.Errorf("vaktaware erasure: delete sr_events: %w", err)
	}
	counts["sr_events"] = evTag.RowsAffected()

	tgtTag, err := tx.Exec(ctx,
		`DELETE FROM sr_targets WHERE org_id = $1 AND lower(email) = lower($2)`,
		orgID, email,
	)
	if err != nil {
		return nil, fmt.Errorf("vaktaware erasure: delete sr_targets: %w", err)
	}
	counts["sr_targets"] = tgtTag.RowsAffected()

	return counts, nil
}
