//go:build integration

// Copyright (c) 2026 NorvikOps. All rights reserved.
// SPDX-License-Identifier: Elastic-2.0

package integration_test

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"

	"github.com/matharnica/vakt/internal/modules/vaktcomply"
	sharedevents "github.com/matharnica/vakt/internal/shared/events"
)

// TestVaktcomplyErasure_RedactsEvidencePII proves the ADR-0089 behaviour against
// real Postgres: an Art. 17 erasure must strip the subject's name and email from
// the ck_evidence rows that reference them — in both the free-text description and
// the structured collector_data — while KEEPING the row and its other evidence
// content, and while leaving another person's evidence untouched.
//
//	go test -tags=integration ./internal/integration_test/ -run TestVaktcomplyErasure_RedactsEvidencePII
func TestVaktcomplyErasure_RedactsEvidencePII(t *testing.T) {
	if testing.Short() {
		t.Skip("integration: -short mode")
	}
	ctx := context.Background()
	pool, teardown := bootPostgres(t)
	defer teardown()

	orgID := uuid.New().String()
	_, err := pool.Exec(ctx, `
		INSERT INTO organizations (id, name, slug) VALUES ($1, 'Redact', 'redact')`, orgID)
	require.NoError(t, err)

	const subjName = "Anna Beispiel"
	const subjEmail = "anna.beispiel@kunde.de"

	// Row that references the subject — built exactly as hr_integration.go writes it.
	var subjRow string
	require.NoError(t, pool.QueryRow(ctx, `
		INSERT INTO ck_evidence (control_id, org_id, title, description, source, collector_data, status)
		VALUES (NULL, $1::uuid, $2, $3, 'hr_checklist_completed', $4::jsonb, 'approved')
		RETURNING id::text`,
		orgID,
		"Onboarding abgeschlossen: "+subjName,
		`Checkliste "Onboarding" für Mitarbeiter `+subjName+` (`+subjEmail+`) abgeschlossen am 01.09.2026 10:00 — 7 Schritte erledigt.`,
		`{"employee_name":"`+subjName+`","employee_email":"`+subjEmail+`","checklist_name":"Onboarding","step_count":7}`,
	).Scan(&subjRow))

	// Row for a DIFFERENT person — must survive completely untouched.
	var otherRow string
	require.NoError(t, pool.QueryRow(ctx, `
		INSERT INTO ck_evidence (control_id, org_id, title, description, source, collector_data, status)
		VALUES (NULL, $1::uuid, $2, $3, 'hr_checklist_completed', $4::jsonb, 'approved')
		RETURNING id::text`,
		orgID,
		"Onboarding abgeschlossen: Max Mustermann",
		`Checkliste "Onboarding" für Mitarbeiter Max Mustermann (max@kunde.de) abgeschlossen am 02.09.2026 09:00 — 5 Schritte erledigt.`,
		`{"employee_name":"Max Mustermann","employee_email":"max@kunde.de","checklist_name":"Onboarding","step_count":5}`,
	).Scan(&otherRow))

	// Run the eraser exactly as ExecuteErasure invokes it.
	counts, err := vaktcomply.SubjectEraser{}.EraseSubjectPII(ctx, pool, sharedevents.SubjectRef{
		OrgID: orgID,
		Email: subjEmail,
	})
	require.NoError(t, err)
	require.Equal(t, int64(1), counts["ck_evidence PII fields"],
		"exactly the one row referencing the subject must be redacted")

	// Subject row: PII gone, evidence context kept, row still present.
	var desc, cdName, cdEmail, cdChecklist string
	var stepCount int
	// orgid-lint: global — Test-Assertion, PK-Lookup (id) in isolierter bootPostgres-Test-Org
	require.NoError(t, pool.QueryRow(ctx, `
		SELECT description,
		       collector_data->>'employee_name',
		       collector_data->>'employee_email',
		       collector_data->>'checklist_name',
		       (collector_data->>'step_count')::int
		FROM ck_evidence WHERE id = $1::uuid`, subjRow).
		Scan(&desc, &cdName, &cdEmail, &cdChecklist, &stepCount))

	require.NotContains(t, desc, subjName, "name must be gone from the description")
	require.NotContains(t, desc, subjEmail, "email must be gone from the description")
	require.Contains(t, desc, "[redigiert]")
	require.Contains(t, desc, "Onboarding", "other evidence content must survive")
	require.Contains(t, desc, "7 Schritte", "other evidence content must survive")

	require.Equal(t, "[redigiert]", cdName)
	require.Equal(t, "[redigiert]", cdEmail)
	require.Equal(t, "Onboarding", cdChecklist, "non-PII collector_data keys must survive")
	require.Equal(t, 7, stepCount, "non-PII collector_data keys must survive")

	// Other person's row: entirely untouched.
	var otherDesc, otherName, otherEmail string
	// orgid-lint: global — Test-Assertion, PK-Lookup (id) in isolierter bootPostgres-Test-Org
	require.NoError(t, pool.QueryRow(ctx, `
		SELECT description, collector_data->>'employee_name', collector_data->>'employee_email'
		FROM ck_evidence WHERE id = $1::uuid`, otherRow).
		Scan(&otherDesc, &otherName, &otherEmail))
	require.Contains(t, otherDesc, "Max Mustermann")
	require.Contains(t, otherDesc, "max@kunde.de")
	require.Equal(t, "Max Mustermann", otherName)
	require.Equal(t, "max@kunde.de", otherEmail)
}
