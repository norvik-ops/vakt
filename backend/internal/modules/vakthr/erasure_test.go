// Copyright (c) 2026 NorvikOps. All rights reserved.
// SPDX-License-Identifier: Elastic-2.0

package vakthr

import (
	"context"
	"strings"
	"testing"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/stretchr/testify/require"

	sharedevents "github.com/matharnica/vakt/internal/shared/events"
)

// fakeExecer satisfies db.DBTX. The eraser only calls Exec; the resolver calls
// Query, so Query records its statement instead of panicking.
type fakeExecer struct {
	stmts     []string
	queries   []string
	queryRows pgx.Rows
	queryErr  error
}

func (f *fakeExecer) Exec(_ context.Context, sql string, _ ...any) (pgconn.CommandTag, error) {
	f.stmts = append(f.stmts, sql)
	return pgconn.NewCommandTag("DELETE 3"), nil
}
func (f *fakeExecer) Query(_ context.Context, sql string, _ ...any) (pgx.Rows, error) {
	f.queries = append(f.queries, sql)
	return f.queryRows, f.queryErr
}
func (f *fakeExecer) QueryRow(context.Context, string, ...any) pgx.Row {
	panic("eraser must not QueryRow")
}

func TestVakthrEraser_DeletesOnlyHRPrefix(t *testing.T) {
	fx := &fakeExecer{}
	counts, err := SubjectEraser{}.EraseSubjectPII(context.Background(), fx, sharedevents.SubjectRef{
		OrgID: "org-1", Email: "victim@example.com",
	})
	require.NoError(t, err)

	// Two hr_ tables carry subject PII: hr_employees and hr_contractors.
	require.Len(t, fx.stmts, 2)
	joined := strings.ToUpper(strings.Join(fx.stmts, " | "))
	joined = strings.Join(strings.Fields(joined), " ")
	require.Contains(t, joined, "DELETE FROM HR_EMPLOYEES",
		"vakthr eraser must delete hr_employees (its own prefix)")
	// It must not write any foreign module prefix.
	for _, foreign := range []string{"SR_", "VB_", "CK_", "SO_", "PO_"} {
		require.NotContains(t, joined, "DELETE FROM "+foreign,
			"vakthr eraser must only write the hr_ prefix")
	}

	require.Equal(t, int64(3), counts["hr_employees"])
}

// TestVakthrEraser_DeletesContractors pins R1-36c-02: hr_contractors stores a
// contractor's first_name/last_name/email (Migration 190). Before the fix the
// eraser only touched hr_employees, so a contractor's PII survived an Art. 17
// erasure. Removing the hr_contractors DELETE from EraseSubjectPII turns this
// test red (both the statement assertion and the count).
func TestVakthrEraser_DeletesContractors(t *testing.T) {
	fx := &fakeExecer{}
	counts, err := SubjectEraser{}.EraseSubjectPII(context.Background(), fx, sharedevents.SubjectRef{
		OrgID: "org-1", Email: "victim@example.com",
	})
	require.NoError(t, err)

	joined := strings.ToUpper(strings.Join(fx.stmts, " | "))
	joined = strings.Join(strings.Fields(joined), " ")
	require.Contains(t, joined, "DELETE FROM HR_CONTRACTORS",
		"vakthr eraser must also erase hr_contractors PII (R1-36c-02)")

	// The contractor delete must be keyed on org_id + email, like the employee
	// delete — never an unscoped table wipe.
	var contractorStmt string
	for _, s := range fx.stmts {
		if strings.Contains(strings.ToUpper(s), "HR_CONTRACTORS") {
			contractorStmt = strings.ToUpper(strings.Join(strings.Fields(s), " "))
		}
	}
	require.Contains(t, contractorStmt, "ORG_ID = $1",
		"contractor erasure must be org-scoped")
	require.Contains(t, contractorStmt, "LOWER(EMAIL) = LOWER($2)",
		"contractor erasure must target the subject's email")

	require.Equal(t, int64(3), counts["hr_contractors"])
}

func TestVakthrEraser_ModuleName(t *testing.T) {
	require.Equal(t, "vakthr", SubjectEraser{}.ModuleName())
}

// TestVakthrResolver_ReadsOwnPrefix pins that the employee-id lookup lives with
// the module that OWNS hr_employees. This is the piece that makes eraser order
// irrelevant: it runs before any delete, so vaktaware never has to read a table
// vakthr is about to remove.
func TestVakthrResolver_ReadsOwnPrefix(t *testing.T) {
	fx := &fakeExecer{queryErr: errStub}
	_, err := SubjectEraser{}.ResolveEmployeeIDs(context.Background(), fx, "org-1", "victim@example.com")
	require.Error(t, err, "a failing lookup must surface, never be swallowed into an empty id set")

	require.Len(t, fx.queries, 1)
	norm := strings.ToUpper(strings.Join(strings.Fields(fx.queries[0]), " "))
	require.Contains(t, norm, "FROM HR_EMPLOYEES",
		"the resolver must read hr_employees — the table vakthr owns")
	require.Empty(t, fx.stmts, "resolving must not write anything")
}

var errStub = stubErr("lookup failed")

type stubErr string

func (e stubErr) Error() string { return string(e) }
