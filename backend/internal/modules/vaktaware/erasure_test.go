// Copyright (c) 2026 NorvikOps. All rights reserved.
// SPDX-License-Identifier: Elastic-2.0

package vaktaware

import (
	"context"
	"strings"
	"testing"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/stretchr/testify/require"

	sharedevents "github.com/matharnica/vakt/internal/shared/events"
)

// fakeExecer records the statements issued against it and returns a canned
// affected-row count per call, so the eraser's SQL and ordering can be asserted
// without a database. Query/QueryRow satisfy db.DBTX but are never called by the
// erasers (they only Exec).
type fakeExecer struct {
	stmts   []string
	affects []int64 // one per call, in order
	calls   int
}

func (f *fakeExecer) Exec(_ context.Context, sql string, _ ...any) (pgconn.CommandTag, error) {
	f.stmts = append(f.stmts, sql)
	n := int64(0)
	if f.calls < len(f.affects) {
		n = f.affects[f.calls]
	}
	f.calls++
	return pgconn.NewCommandTag("DELETE " + itoa(n)), nil
}

func (f *fakeExecer) Query(context.Context, string, ...any) (pgx.Rows, error) {
	panic("erasers must not Query")
}
func (f *fakeExecer) QueryRow(context.Context, string, ...any) pgx.Row {
	panic("erasers must not QueryRow")
}

func itoa(n int64) string {
	if n == 0 {
		return "0"
	}
	var b [20]byte
	i := len(b)
	for n > 0 {
		i--
		b[i] = byte('0' + n%10)
		n /= 10
	}
	return string(b[i:])
}

func firstTable(sql, verb string) string {
	norm := strings.Join(strings.Fields(sql), " ")
	idx := strings.Index(strings.ToUpper(norm), verb)
	if idx < 0 {
		return ""
	}
	rest := strings.Fields(norm[idx+len(verb):])
	if len(rest) == 0 {
		return ""
	}
	return rest[0]
}

func subjectRef() sharedevents.SubjectRef {
	return sharedevents.SubjectRef{
		OrgID:       "org-1",
		Email:       "victim@example.com",
		EmployeeIDs: []string{"emp-1", "emp-2"},
	}
}

func TestVaktawareEraser_DeletesOwnPrefixInOrder(t *testing.T) {
	fx := &fakeExecer{affects: []int64{2, 5, 1}}
	counts, err := SubjectEraser{}.EraseSubjectPII(context.Background(), fx, subjectRef())
	require.NoError(t, err)

	// Three statements, all writing the sr_ prefix. sr_events MUST precede
	// sr_targets (FK is ON DELETE SET NULL — deleting targets first would orphan
	// the IP/user-agent telemetry instead of erasing it).
	require.Len(t, fx.stmts, 3)
	require.Equal(t, "sr_campaign_enrollments", firstTable(fx.stmts[0], "DELETE FROM"))
	require.Equal(t, "sr_events", firstTable(fx.stmts[1], "DELETE FROM"))
	require.Equal(t, "sr_targets", firstTable(fx.stmts[2], "DELETE FROM"))

	for _, s := range fx.stmts {
		require.True(t, strings.HasPrefix(firstTable(s, "DELETE FROM"), "sr_"),
			"vaktaware eraser must only DELETE the sr_ prefix, got: %s", s)
	}

	require.Equal(t, int64(2), counts["sr_campaign_enrollments"])
	require.Equal(t, int64(5), counts["sr_events"])
	require.Equal(t, int64(1), counts["sr_targets"])
}

// TestVaktawareEraser_TouchesNoForeignTable pins the property that removed the
// ordering trap: after the SubjectRef refactor the vaktaware eraser must not
// reference hr_employees (or any non-sr_ table) at all — not even as a READ.
// A reintroduced sub-SELECT would silently restore the dependency on the vakthr
// eraser not having run yet, which is invisible to the compiler.
func TestVaktawareEraser_TouchesNoForeignTable(t *testing.T) {
	fx := &fakeExecer{affects: []int64{1, 1, 1}}
	_, err := SubjectEraser{}.EraseSubjectPII(context.Background(), fx, subjectRef())
	require.NoError(t, err)

	for _, s := range fx.stmts {
		require.NotContains(t, strings.ToLower(s), "hr_employees",
			"vaktaware eraser must not read hr_employees — employee ids arrive "+
				"pre-resolved in SubjectRef; re-adding this lookup restores the "+
				"eraser-ordering trap (ADR-0079)")
	}
}

// TestVaktawareEraser_NoEmployeeIDsStillErasesTargets covers the non-employee
// subject: an empty EmployeeIDs slice must delete zero enrollments but MUST NOT
// skip sr_targets/sr_events — the subject may exist as a phishing target
// without ever having been an employee.
func TestVaktawareEraser_NoEmployeeIDsStillErasesTargets(t *testing.T) {
	fx := &fakeExecer{affects: []int64{0, 3, 1}}
	counts, err := SubjectEraser{}.EraseSubjectPII(context.Background(), fx, sharedevents.SubjectRef{
		OrgID: "org-1", Email: "outsider@example.com", EmployeeIDs: nil,
	})
	require.NoError(t, err)
	require.Len(t, fx.stmts, 3, "all three deletes must still run for a non-employee subject")
	require.Equal(t, int64(0), counts["sr_campaign_enrollments"])
	require.Equal(t, int64(3), counts["sr_events"])
	require.Equal(t, int64(1), counts["sr_targets"])
}

func TestVaktawareEraser_ModuleName(t *testing.T) {
	require.Equal(t, "vaktaware", SubjectEraser{}.ModuleName())
}
