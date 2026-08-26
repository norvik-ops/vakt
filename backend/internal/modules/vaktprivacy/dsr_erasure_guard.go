// Copyright (c) 2026 NorvikOps. All rights reserved.
// SPDX-License-Identifier: Elastic-2.0

package vaktprivacy

import (
	"errors"
	"fmt"
	"strings"
)

// ── Art. 17 DSGVO: a completed erasure must have an executed erasure behind it ──
//
// po_dsr has two independent write paths that can move a request into a closed
// state: Service.UpdateDSR (PATCH /dsr/:id) and Service.ResolveDSR
// (POST /dsr/:id/resolve). Only the first routed erasure-type requests through
// Repository.ExecuteErasure; the second wrote `status = 'completed'` with a raw
// UPDATE that never looked at `type`. One request closed an Art. 17 erasure
// without deleting anything, and left behind a record that claims a deletion
// duty was discharged. A supervisory authority reads such a record as evidence.
//
// The enforcement lives on the server, at both write paths, keyed on the stored
// row — not on the route and not on which button the frontend rendered. Hiding
// the button is not enforcement; this project has paid for that lesson twice
// (feature gates on literal sibling routes, `// public` comments on protected
// mounts).

// ErrErasureNotExecuted is returned when a caller tries to close an Art. 17
// erasure request as completed/fulfilled while the erasure transaction has not
// committed. Handlers map it to 409 Conflict: the request is well-formed, the
// row is simply not in a state where completion is truthful.
var ErrErasureNotExecuted = errors.New(
	"erasure request cannot be marked completed before the Art. 17 erasure has been executed")

// erasureEvidenceMarker is the header that the ExecutePPDSRErasure query writes
// into po_dsr.notes, inside the erasure transaction, right before it stamps
// status = 'completed'. It is the only positive, durable proof carried by the
// row that the deletion actually committed.
//
// It is deliberately preferred over inferring "executed" from status alone:
// status is a field many code paths write, so reading it as proof of deletion
// would be circular. The marker has exactly one producer, and that producer
// cannot write it without the deletes having committed in the same transaction.
const erasureEvidenceMarker = "--- Erasure executed ---"

// erasureExecuted reports whether the stored DSR carries proof of a committed
// Art. 17 erasure.
//
// Both halves are required. status alone is inference; the marker alone could
// survive on a row that was later reopened, and a reopened erasure request is
// by definition not discharged. Anything else — including a row whose evidence
// note was overwritten — counts as "not executed" and fails closed: the worst
// outcome of a false negative is that an operator must run the erasure action,
// while the worst outcome of a false positive is a false compliance record.
func erasureExecuted(d *DSR) bool {
	if d == nil {
		return false
	}
	return d.Status == "completed" && strings.Contains(d.Notes, erasureEvidenceMarker)
}

// guardErasureCompletion refuses to move an erasure-type DSR into the completed
// state unless the erasure has already been executed.
//
// Scope — deliberately only "completed":
//   - rejected  stays allowed. Art. 17 Abs. 3 lists lawful grounds to refuse an
//     erasure (legal retention duties, freedom of expression, legal claims). A
//     refusal claims no deletion, so it needs none.
//   - extended  stays allowed (Art. 12 Abs. 3, deadline extension).
//   - open / in_progress / overdue are not closing states.
//
// Only "completed" asserts "we did what you asked", and that is the only
// assertion that has to be backed by a committed transaction.
func guardErasureCompletion(existing *DSR, newStatus string) error {
	if existing == nil || existing.Type != "erasure" || newStatus != "completed" {
		return nil
	}
	if erasureExecuted(existing) {
		// Already discharged: re-closing is a no-op on the deletion itself and
		// stays permitted so an operator can still record resolution notes.
		return nil
	}
	return fmt.Errorf("%w (dsr %s, status %q)", ErrErasureNotExecuted, existing.ID, existing.Status)
}

// preserveErasureEvidence returns the notes value to store, carrying the Art. 17
// evidence block over from the stored row when the caller's new notes would drop
// it.
//
// Both closing paths overwrite po_dsr.notes wholesale — UpdatePPDSR with a plain
// assignment, ResolveDSR with a COALESCE over the incoming value. So an operator
// who edits the notes of an executed erasure used to delete the only durable
// proof that the deletion ran. That is bad twice over: the audit record is gone,
// and erasureExecuted silently downgrades the row to "not executed".
//
// Callers pass the notes they are about to write; an empty newNotes yields the
// evidence block alone, which is still strictly better than NULL.
func preserveErasureEvidence(existing *DSR, newNotes string) string {
	if existing == nil || existing.Type != "erasure" {
		return newNotes
	}
	idx := strings.Index(existing.Notes, erasureEvidenceMarker)
	if idx < 0 || strings.Contains(newNotes, erasureEvidenceMarker) {
		return newNotes
	}
	return strings.TrimSpace(strings.TrimSpace(newNotes) + "\n\n" + existing.Notes[idx:])
}
