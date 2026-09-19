// Copyright (c) 2026 NorvikOps. All rights reserved.
// SPDX-License-Identifier: Elastic-2.0

package vaktcomply

import (
	"context"
	"regexp"
	"strings"
	"testing"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/stretchr/testify/require"

	sharedevents "github.com/matharnica/vakt/internal/shared/events"
)

// captureExecer records the single UPDATE the eraser issues so the test can pin
// its shape (which columns it touches, which params it binds) without a database.
// The row-level redaction semantics are proven against real Postgres in
// internal/integration_test/vaktcomply_erasure_redact_test.go.
type captureExecer struct {
	sql  string
	args []any
	tag  pgconn.CommandTag
}

func (c *captureExecer) Exec(_ context.Context, sql string, args ...any) (pgconn.CommandTag, error) {
	c.sql = sql
	c.args = args
	return c.tag, nil
}

func (c *captureExecer) Query(_ context.Context, _ string, _ ...any) (pgx.Rows, error) {
	panic("eraser must not Query")
}

func (c *captureExecer) QueryRow(_ context.Context, _ string, _ ...any) pgx.Row {
	panic("eraser must not QueryRow")
}

func TestSubjectEraser_ModuleName(t *testing.T) {
	require.Equal(t, "vaktcomply", SubjectEraser{}.ModuleName(),
		"ModuleName must equal the entry added to requiredEraserModules, or the "+
			"completeness contract skips ck_ redaction")
}

// TestEraseSubjectPII_RedactsFieldsNeverDeletesRows pins the contract that makes
// this eraser different from the hr_/sr_ ones: it must UPDATE ck_evidence (redact
// the PII columns) and must never DELETE a row — the evidence must survive under
// the retention obligation (ADR-0089).
func TestEraseSubjectPII_RedactsFieldsNeverDeletesRows(t *testing.T) {
	exec := &captureExecer{tag: pgconn.NewCommandTag("UPDATE 2")}

	counts, err := SubjectEraser{}.EraseSubjectPII(context.Background(), exec, sharedevents.SubjectRef{
		OrgID: "11111111-1111-1111-1111-111111111111",
		Email: "victim+tag@example.com",
	})
	require.NoError(t, err)

	// It redacts, it does not delete.
	require.Contains(t, exec.sql, "UPDATE ck_evidence")
	require.NotContains(t, exec.sql, "DELETE")
	// Both PII columns are targeted.
	require.Contains(t, exec.sql, "employee_email")
	require.Contains(t, exec.sql, "employee_name")
	require.Contains(t, exec.sql, "[redigiert]")

	// Params: org, regexp-escaped email, plain email. No NUL-byte sentinel — a
	// text parameter containing 0x00 is rejected by Postgres (SQLSTATE 22021) and
	// would abort the whole Art. 17 run; the name replace is skipped via CASE when
	// employee_name is empty instead.
	require.Len(t, exec.args, 3)
	require.Equal(t, "11111111-1111-1111-1111-111111111111", exec.args[0])
	require.Equal(t, regexp.QuoteMeta("victim+tag@example.com"), exec.args[1],
		"the email fed to regexp_replace must be escaped so '+' and '.' match literally")
	require.Equal(t, "victim+tag@example.com", exec.args[2])

	// The escaped form must actually differ from the raw one for this address —
	// otherwise the escaping is untested.
	require.NotEqual(t, exec.args[1], exec.args[2])

	// No argument may carry a NUL byte — that is the bug this guards against.
	for _, a := range exec.args {
		if s, ok := a.(string); ok {
			require.NotContains(t, s, "\x00", "no query parameter may contain a NUL byte")
		}
	}

	// Count is reported under a key that reads truthfully in the Art. 17 note.
	require.Equal(t, int64(2), counts["ck_evidence PII fields"])
	for k := range counts {
		require.False(t, strings.Contains(k, "deleted"),
			"the ck_ count key must not claim deletion — rows are kept")
	}
}
