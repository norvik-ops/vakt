// Copyright (c) 2026 NorvikOps. All rights reserved.
// SPDX-License-Identifier: Elastic-2.0

package vaktprivacy

import (
	"errors"
	"os"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

// executedNote is what ExecutePPDSRErasure leaves in po_dsr.notes: the caller's
// text, then the marker, then the per-table counts from buildErasureNote.
const executedNote = "Löschung ausgeführt.\n\n" + erasureEvidenceMarker + "\n" +
	"Art. 17 DSGVO erasure executed at 2026-08-06T10:00:00Z\n" +
	"hr_employees deleted: 1\nsr_targets deleted: 2\nusers anonymised: 1"

// TestGuardErasureCompletion_BlocksUnexecutedErasure is the core invariant: an
// erasure request may not be stamped "completed" unless the erasure ran. Both
// closing paths (PATCH /dsr/:id and POST /dsr/:id/resolve) route through this
// guard, so one table covers both.
func TestGuardErasureCompletion_BlocksUnexecutedErasure(t *testing.T) {
	cases := []struct {
		name      string
		dsr       DSR
		newStatus string
		wantErr   bool
	}{
		// ── The defect: closing an erasure that never deleted anything ──
		{
			name:      "erasure open -> completed is refused",
			dsr:       DSR{ID: "d1", Type: "erasure", Status: "open"},
			newStatus: "completed",
			wantErr:   true,
		},
		{
			name:      "erasure in_progress -> completed is refused",
			dsr:       DSR{ID: "d2", Type: "erasure", Status: "in_progress"},
			newStatus: "completed",
			wantErr:   true,
		},
		{
			name:      "erasure overdue -> completed is refused",
			dsr:       DSR{ID: "d3", Type: "erasure", Status: "overdue"},
			newStatus: "completed",
			wantErr:   true,
		},
		{
			// Notes that merely talk about a deletion are not evidence of one.
			// Only ExecutePPDSRErasure writes the marker, and only inside the
			// transaction that did the deleting.
			name:      "erasure with hand-written notes but no marker is refused",
			dsr:       DSR{ID: "d4", Type: "erasure", Status: "open", Notes: "Löschung ausgeführt (Art. 17 DSGVO)."},
			newStatus: "completed",
			wantErr:   true,
		},
		{
			// Status alone is not proof — otherwise the guard would accept the
			// very row a bypass had just written.
			name:      "erasure already completed but WITHOUT evidence marker is refused",
			dsr:       DSR{ID: "d5", Type: "erasure", Status: "completed", Notes: "erledigt"},
			newStatus: "completed",
			wantErr:   true,
		},
		{
			// Marker without completed status: the row was reopened after an
			// erasure, so the request is not discharged in its current state.
			name:      "erasure reopened after execution is refused",
			dsr:       DSR{ID: "d6", Type: "erasure", Status: "in_progress", Notes: executedNote},
			newStatus: "completed",
			wantErr:   true,
		},

		// ── Baseline: the erasure ran, closing it is truthful ──
		{
			name:      "erasure completed WITH evidence marker is allowed",
			dsr:       DSR{ID: "d7", Type: "erasure", Status: "completed", Notes: executedNote},
			newStatus: "completed",
			wantErr:   false,
		},

		// ── Baseline: lawful closures that assert no deletion ──
		{
			// Art. 17 Abs. 3 — lawful grounds to refuse an erasure.
			name:      "erasure -> rejected is allowed",
			dsr:       DSR{ID: "d8", Type: "erasure", Status: "open"},
			newStatus: "rejected",
			wantErr:   false,
		},
		{
			// Art. 12 Abs. 3 — deadline extension.
			name:      "erasure -> extended is allowed",
			dsr:       DSR{ID: "d9", Type: "erasure", Status: "open"},
			newStatus: "extended",
			wantErr:   false,
		},
		{
			name:      "erasure -> in_progress is allowed",
			dsr:       DSR{ID: "d10", Type: "erasure", Status: "open"},
			newStatus: "in_progress",
			wantErr:   false,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			err := guardErasureCompletion(&tc.dsr, tc.newStatus)
			if tc.wantErr {
				require.Error(t, err, "guard must refuse this transition")
				require.ErrorIs(t, err, ErrErasureNotExecuted,
					"refusal must carry the sentinel so handlers can map it to 409")
				return
			}
			require.NoError(t, err)
		})
	}
}

// TestGuardErasureCompletion_OtherTypesUnaffected pins the blast radius: the
// guard is specific to Art. 17. Every other request type keeps closing exactly
// as before, so the fix cannot quietly block ordinary DSR work.
//
// This is also the class-search result made executable — if a future request
// type gains its own mandatory fulfilment action, this table is where the
// deliberate "not guarded" decision becomes visible and has to be revisited.
func TestGuardErasureCompletion_OtherTypesUnaffected(t *testing.T) {
	otherTypes := []string{
		"access",        // Art. 15
		"rectification", // Art. 16
		"restriction",   // Art. 18
		"portability",   // Art. 20
		"objection",     // Art. 21
		"no_profiling",  // Art. 22
	}
	for _, typ := range otherTypes {
		t.Run(typ, func(t *testing.T) {
			dsr := DSR{ID: "x", Type: typ, Status: "open"}
			require.NoError(t, guardErasureCompletion(&dsr, "completed"),
				"only erasure requests carry an execution precondition")
		})
	}
}

// TestErasureExecuted_RequiresBothHalves states the definition of "executed"
// directly, independent of the guard that consumes it.
func TestErasureExecuted_RequiresBothHalves(t *testing.T) {
	require.False(t, erasureExecuted(nil), "nil row is not proof of anything")
	require.False(t, erasureExecuted(&DSR{Status: "completed"}),
		"status alone is inference, not evidence")
	require.False(t, erasureExecuted(&DSR{Status: "open", Notes: executedNote}),
		"marker on a non-completed row means the request was reopened")
	require.True(t, erasureExecuted(&DSR{Status: "completed", Notes: executedNote}))
}

// TestErasureEvidenceMarkerMatchesProducer guards the seam between this file and
// the SQL that writes the marker. erasureExecuted reads a string that is
// produced in db/queries/vaktprivacy.sql (ExecutePPDSRErasure). If someone
// changes the marker on one side only, every executed erasure silently reads as
// "not executed" — fail-closed, but wrong. The generated query text is the
// authority, so assert against it rather than against a second copy.
func TestErasureEvidenceMarkerMatchesProducer(t *testing.T) {
	// The generated query file is the code that actually runs, so read that one
	// rather than db/queries/vaktprivacy.sql.
	const generated = "../../db/vaktprivacy.sql.go"
	src, err := os.ReadFile(generated)
	require.NoError(t, err, "cannot read %s — this test must never pass vacuously", generated)

	idx := strings.Index(string(src), "UPDATE po_dsr SET\n  status       = 'completed'")
	require.GreaterOrEqual(t, idx, 0,
		"ExecutePPDSRErasure not found in %s — did the erasure query move?", generated)

	require.Contains(t, string(src)[idx:], erasureEvidenceMarker,
		"the ExecutePPDSRErasure query must still write the marker erasureExecuted looks for")
}

func TestErrErasureNotExecutedIsSentinel(t *testing.T) {
	err := guardErasureCompletion(&DSR{ID: "d", Type: "erasure", Status: "open"}, "completed")
	require.True(t, errors.Is(err, ErrErasureNotExecuted))
	require.Contains(t, err.Error(), "d", "message should name the offending DSR")
}
