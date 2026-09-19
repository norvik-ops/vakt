// Copyright (c) 2026 NorvikOps. All rights reserved.
// SPDX-License-Identifier: Elastic-2.0

package retention

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/jackc/pgx/v5/pgconn"
)

// fakeExecer records the SQL statements it is asked to run and can be told to
// fail one specific category, so we can prove a delete error is propagated
// rather than swallowed.
type fakeExecer struct {
	seen     []string
	failWhen func(sql string) bool
}

func (f *fakeExecer) Exec(_ context.Context, sql string, _ ...any) (pgconn.CommandTag, error) {
	f.seen = append(f.seen, sql)
	if f.failWhen != nil && f.failWhen(sql) {
		return pgconn.CommandTag{}, errors.New("boom: delete failed")
	}
	return pgconn.CommandTag{}, nil
}

func allCategoriesEnabled() *RetentionConfig {
	return &RetentionConfig{
		AuditLogDays:         365,
		FindingsResolvedDays: 180,
		NotificationsDays:    90,
	}
}

// TestPruneRetention_DeleteErrorPropagates is the core R1-W8C-N1(a) guard: a
// failing delete must surface as a returned error, not a silent success.
func TestPruneRetention_DeleteErrorPropagates(t *testing.T) {
	db := &fakeExecer{
		failWhen: func(sql string) bool { return strings.Contains(sql, "vb_findings") },
	}

	err := pruneRetention(context.Background(), db, allCategoriesEnabled(), "org-1")
	if err == nil {
		t.Fatal("expected the vb_findings delete failure to propagate, got nil")
	}
	if !strings.Contains(err.Error(), "vb_findings") {
		t.Fatalf("error should name the failing category, got: %v", err)
	}

	// Every enabled category must still have been attempted despite the failure.
	if len(db.seen) != 3 {
		t.Fatalf("expected all 3 categories attempted, got %d: %v", len(db.seen), db.seen)
	}
}

// TestPruneRetention_AllOK verifies the normal path is unchanged: no error when
// every delete succeeds.
func TestPruneRetention_AllOK(t *testing.T) {
	db := &fakeExecer{}
	if err := pruneRetention(context.Background(), db, allCategoriesEnabled(), "org-1"); err != nil {
		t.Fatalf("expected nil error on success, got: %v", err)
	}
	if len(db.seen) != 3 {
		t.Fatalf("expected 3 statements, got %d", len(db.seen))
	}
}

// TestPruneRetention_DisabledCategoriesSkipped verifies a retention of 0 means
// "disabled" — the category is neither run nor counted as an error.
func TestPruneRetention_DisabledCategoriesSkipped(t *testing.T) {
	db := &fakeExecer{
		// Would fail if run, but the only enabled category is audit_log.
		failWhen: func(sql string) bool { return strings.Contains(sql, "vb_findings") },
	}
	cfg := &RetentionConfig{AuditLogDays: 30} // others left at 0 = disabled

	if err := pruneRetention(context.Background(), db, cfg, "org-1"); err != nil {
		t.Fatalf("disabled categories must be skipped, got: %v", err)
	}
	if len(db.seen) != 1 {
		t.Fatalf("expected only the audit_log statement, got %d: %v", len(db.seen), db.seen)
	}
}
