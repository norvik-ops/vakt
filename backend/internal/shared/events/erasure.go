// Copyright (c) 2026 NorvikOps. All rights reserved.
// SPDX-License-Identifier: Elastic-2.0

package events

import (
	"context"

	"github.com/matharnica/vakt/internal/db"
)

// Execer is the query surface a SubjectErasure runs on. It aliases the
// sqlc-generated db.DBTX, which both *pgx.Tx and *pgxpool.Pool satisfy — so
// vaktprivacy passes its open erasure transaction in and the whole Art. 17
// DSGVO erasure stays atomic across module boundaries. (Aliased rather than
// re-declared so no new untyped-`any` variadic enters the interface ratchet.
// Erasers call Exec; a SubjectResolver also calls Query.)
type Execer = db.DBTX

// ErasureCounts maps a table name to the number of rows an eraser deleted, for
// the DSGVO evidence note. A nil/empty map is a valid "nothing to erase" result.
type ErasureCounts map[string]int64

// SubjectRef identifies the data subject of an Art. 17 erasure, with every
// cross-module identifier ALREADY RESOLVED.
//
// This type is why erasers are order-independent. The naive design has the
// vaktaware eraser resolve `employee_id` by reading hr_employees at delete
// time — which silently depends on the vakthr eraser not having run yet. That
// dependency is invisible to the compiler and survives any refactor that
// reorders the wiring: vakthr deletes hr_employees, vaktaware's sub-SELECT then
// matches nothing, sr_campaign_enrollments KEEPS the PII, and the DSR is still
// stamped "completed" with a plausible-looking `deleted: 0`. No error, no log —
// exactly the silent-failure class ADR-0079 calls a compliance violation.
//
// Resolving up front (see SubjectResolver) removes the ordering constraint
// entirely rather than documenting it: by the time any eraser runs, every
// identifier it needs is already in hand.
type SubjectRef struct {
	// OrgID scopes the erasure to one organisation.
	OrgID string
	// Email identifies the data subject.
	Email string
	// EmployeeIDs are the hr_employees ids matching Email, resolved BEFORE any
	// eraser runs. Empty when the subject is not an employee — a valid state,
	// not an error.
	EmployeeIDs []string
}

// SubjectResolver resolves the module-owned identifiers that OTHER modules need
// in order to find their rows for a data subject. It runs once, before any
// eraser, inside the same transaction.
//
// It exists so a module never reads across a prefix boundary at delete time:
// the module that OWNS the table does the lookup and hands the result over.
type SubjectResolver interface {
	// ResolveEmployeeIDs returns the ids of the subject's employee rows, or an
	// empty slice when the subject is not an employee.
	ResolveEmployeeIDs(ctx context.Context, tx Execer, orgID, email string) ([]string, error)
}

// SubjectErasure erases all personal data ONE module stores for a data subject
// as part of an Art. 17 DSGVO request. Each module implements it over its OWN
// table prefix only; vaktprivacy orchestrates the calls inside its erasure
// transaction so no module writes across a prefix boundary (module isolation,
// [[ADR-0079]] / ADR-0004).
//
// Erasers are ORDER-INDEPENDENT by construction — every cross-module identifier
// arrives pre-resolved in SubjectRef. Do not reintroduce a read of another
// module's tables here; that would restore the ordering trap this interface was
// shaped to remove.
type SubjectErasure interface {
	EraseSubjectPII(ctx context.Context, tx Execer, subj SubjectRef) (ErasureCounts, error)
	// ModuleName reports which module this eraser covers (e.g. "vakthr"). It
	// lets the orchestrator verify that EVERY module known to hold subject PII
	// is actually wired, instead of only checking that the list is non-empty —
	// a PARTIALLY wired erasure is the dangerous case, not an unwired one.
	ModuleName() string
}
