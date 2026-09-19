// Copyright (c) 2026 NorvikOps. All rights reserved.
// SPDX-License-Identifier: Elastic-2.0

package vaktcomply

import (
	"context"
	"fmt"
	"regexp"

	sharedevents "github.com/matharnica/vakt/internal/shared/events"
)

// SubjectEraser redacts the personal data Vakt Comply stores for a data subject
// inside ck_evidence — the employee name and email that hr_integration.go writes
// into ck_evidence.description and ck_evidence.collector_data when an HR
// checklist run or a contractor lifecycle event produces compliance evidence.
//
// Unlike the hr_/sr_ erasers it does NOT delete rows. Compliance evidence is
// subject to a retention obligation (Art. 17 Abs. 3 lit. b DSGVO / ISO 27001
// Nachweispflicht), so the evidence ROW must survive an Art. 17 erasure; only
// the personal-data FIELDS are replaced with "[redigiert]". This satisfies
// Art. 17 (the PII is gone) while keeping the audit trail intact (ADR-0089).
//
// It writes ONLY the ck_ prefix and reads no other module's tables — the name it
// redacts from the free-text description is taken from that same evidence row's
// own collector_data, not from a cross-module lookup into hr_employees. So module
// isolation (ADR-0079) holds: vaktprivacy never touches ck_, and this eraser
// never touches hr_/sr_.
type SubjectEraser struct{}

// NewSubjectEraser returns a stateless vaktcomply eraser typed as the shared
// interface. It carries no state — all writes run on the transaction passed in.
func NewSubjectEraser() sharedevents.SubjectErasure { return SubjectEraser{} }

// ModuleName identifies this eraser for the orchestrator's completeness check
// (requiredEraserModules in vaktprivacy). Adding this module to that list is what
// makes ExecuteErasure refuse to run a partial Art. 17 erasure that skips ck_.
func (SubjectEraser) ModuleName() string { return "vaktcomply" }

// EraseSubjectPII redacts the subject's name and email from every ck_evidence row
// that references the subject, on the passed transaction. It never deletes a row.
//
// A row references the subject when the subject's email appears either in the
// structured collector_data.employee_email field or literally in the free-text
// description. For each such row:
//   - collector_data.employee_email / .employee_name are overwritten with
//     "[redigiert]" (only the keys that exist; other keys are untouched),
//   - the email is stripped from the description case-insensitively,
//   - the name is stripped from the description using that row's own
//     collector_data.employee_name value (an exact match, since hr_integration
//     built the description from exactly that value).
//
// The rest of the description (checklist name, dates, step count) survives, so
// the evidence stays meaningful without carrying the person's identity.
func (SubjectEraser) EraseSubjectPII(ctx context.Context, tx sharedevents.Execer, subj sharedevents.SubjectRef) (sharedevents.ErasureCounts, error) {
	// regexp_replace needs the email as a literal pattern; QuoteMeta escapes the
	// '.', '+' etc. an address may contain so they match literally, not as
	// regex metacharacters.
	emailPattern := regexp.QuoteMeta(subj.Email)

	// The name is stripped only when this row actually carries one — replacing by
	// a "sentinel that cannot appear" is impossible with a plain Postgres text
	// parameter: any NUL byte (0x00) in the search term is rejected with SQLSTATE
	// 22021 and would abort the whole Art. 17 run. A CASE that skips the name
	// replace when employee_name is empty is the correct no-op.
	tag, err := tx.Exec(ctx, `
		UPDATE ck_evidence
		SET
			description = regexp_replace(
				CASE
					WHEN NULLIF(collector_data->>'employee_name', '') IS NOT NULL
					THEN replace(description, collector_data->>'employee_name', '[redigiert]')
					ELSE description
				END,
				$2, '[redigiert]', 'gi'
			),
			collector_data = CASE
				WHEN collector_data IS NULL THEN NULL
				ELSE collector_data
					|| (CASE WHEN jsonb_exists(collector_data, 'employee_email')
						 THEN jsonb_build_object('employee_email', '[redigiert]') ELSE '{}'::jsonb END)
					|| (CASE WHEN jsonb_exists(collector_data, 'employee_name')
						 THEN jsonb_build_object('employee_name', '[redigiert]') ELSE '{}'::jsonb END)
			END
		WHERE org_id = $1::uuid
		  AND (
				lower(collector_data->>'employee_email') = lower($3)
			 OR position(lower($3) in lower(COALESCE(description, ''))) > 0
		  )`,
		subj.OrgID, emailPattern, subj.Email,
	)
	if err != nil {
		return nil, fmt.Errorf("vaktcomply erasure: redact ck_evidence: %w", err)
	}

	// The row stays; only its PII fields are cleared. The count is reported as
	// "PII fields" so the Art. 17 evidence note reads truthfully — these rows were
	// redacted, not deleted.
	return sharedevents.ErasureCounts{
		"ck_evidence PII fields": tag.RowsAffected(),
	}, nil
}
