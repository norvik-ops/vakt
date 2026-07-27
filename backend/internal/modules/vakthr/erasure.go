// Copyright (c) 2026 NorvikOps. All rights reserved.
// SPDX-License-Identifier: Elastic-2.0

package vakthr

import (
	"context"
	"fmt"

	sharedevents "github.com/matharnica/vakt/internal/shared/events"
)

// SubjectEraser erases the personal data Vakt HR stores for a data subject —
// the hr_employees directory row — as part of an Art. 17 DSGVO erasure. It
// writes ONLY the hr_ prefix; vaktprivacy invokes it inside its erasure
// transaction so no module writes across a prefix boundary (module isolation,
// ADR-0079).
//
// It also implements events.SubjectResolver: hr_employees belongs to this
// module, so this module — not vaktaware, and not vaktprivacy — performs the
// employee-id lookup. That resolve runs before ANY eraser, which is what makes
// eraser order irrelevant.
type SubjectEraser struct{}

// NewSubjectEraser returns a stateless vakthr eraser typed as the shared
// interface. All writes run on the transaction passed in.
func NewSubjectEraser() sharedevents.SubjectErasure { return SubjectEraser{} }

// NewSubjectResolver returns the vakthr resolver for hr_employees ids.
func NewSubjectResolver() sharedevents.SubjectResolver { return SubjectEraser{} }

// ModuleName identifies this eraser for the orchestrator's completeness check.
func (SubjectEraser) ModuleName() string { return "vakthr" }

// ResolveEmployeeIDs returns the hr_employees ids for the subject. It runs
// BEFORE any eraser deletes anything, so callers receive the ids while the rows
// still exist. An empty result means "not an employee" and is not an error.
func (SubjectEraser) ResolveEmployeeIDs(ctx context.Context, tx sharedevents.Execer, orgID, email string) ([]string, error) {
	rows, err := tx.Query(ctx,
		`SELECT id::text FROM hr_employees WHERE org_id = $1 AND lower(email) = lower($2)`,
		orgID, email,
	)
	if err != nil {
		return nil, fmt.Errorf("vakthr resolve employee ids: %w", err)
	}
	defer rows.Close()

	var ids []string
	for rows.Next() {
		var id string
		if scanErr := rows.Scan(&id); scanErr != nil {
			return nil, fmt.Errorf("vakthr resolve employee ids: scan: %w", scanErr)
		}
		ids = append(ids, id)
	}
	if rowsErr := rows.Err(); rowsErr != nil {
		return nil, fmt.Errorf("vakthr resolve employee ids: %w", rowsErr)
	}
	return ids, nil
}

// EraseSubjectPII deletes the hr_employees row(s) for the subject, on the
// passed transaction. Order-independent: it neither reads nor writes any other
// module's prefix.
func (SubjectEraser) EraseSubjectPII(ctx context.Context, tx sharedevents.Execer, subj sharedevents.SubjectRef) (sharedevents.ErasureCounts, error) {
	tag, err := tx.Exec(ctx,
		`DELETE FROM hr_employees WHERE org_id = $1 AND lower(email) = lower($2)`,
		subj.OrgID, subj.Email,
	)
	if err != nil {
		return nil, fmt.Errorf("vakthr erasure: delete hr_employees: %w", err)
	}
	return sharedevents.ErasureCounts{"hr_employees": tag.RowsAffected()}, nil
}
