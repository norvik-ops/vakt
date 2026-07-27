// Copyright (c) 2026 NorvikOps. All rights reserved.
// SPDX-License-Identifier: Elastic-2.0

package vaktprivacy

import (
	"context"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"

	sharedevents "github.com/matharnica/vakt/internal/shared/events"
)

// buildErasureNote must render a deterministic, complete Art. 17 evidence note
// regardless of the map iteration order of the counts it receives — the note is
// audit evidence, so its content may not depend on eraser wiring order.
func TestBuildErasureNote_DeterministicAndComplete(t *testing.T) {
	counts := sharedevents.ErasureCounts{
		"sr_targets":              1,
		"hr_employees":            2,
		"sr_events":               5,
		"sr_campaign_enrollments": 0,
	}

	note := buildErasureNote(counts, 1)

	require.True(t, strings.HasPrefix(note, "Art. 17 DSGVO erasure executed at "))
	// Every reported table appears with its count.
	require.Contains(t, note, "hr_employees deleted: 2")
	require.Contains(t, note, "sr_campaign_enrollments deleted: 0")
	require.Contains(t, note, "sr_events deleted: 5")
	require.Contains(t, note, "sr_targets deleted: 1")
	require.Contains(t, note, "users anonymised: 1")

	// Tables are listed in sorted order → stable output for two identical inputs.
	require.Equal(t, note, buildErasureNote(counts, 1))
	// hr_ sorts before sr_.
	require.Less(t, strings.Index(note, "hr_employees"), strings.Index(note, "sr_events"))
}

func TestBuildErasureNote_NoTables(t *testing.T) {
	note := buildErasureNote(sharedevents.ErasureCounts{}, 0)
	require.Contains(t, note, "Art. 17 DSGVO erasure executed at ")
	require.Contains(t, note, "users anonymised: 0")
}

// ── Wiring completeness (ADR-0079) ──────────────────────────────────────────

type stubEraser struct{ module string }

func (s stubEraser) ModuleName() string { return s.module }
func (s stubEraser) EraseSubjectPII(_ context.Context, _ sharedevents.Execer, _ sharedevents.SubjectRef) (sharedevents.ErasureCounts, error) {
	return sharedevents.ErasureCounts{}, nil
}

// TestWithSubjectErasers_AppendsAndDeduplicates pins two properties that a
// partial-erasure bug would otherwise hide: a second call must not DISCARD the
// erasers from the first (it used to overwrite the slice), and wiring the same
// module twice must not make it count twice.
func TestWithSubjectErasers_AppendsAndDeduplicates(t *testing.T) {
	r := &Repository{}
	r.WithSubjectErasers(stubEraser{"vaktaware"})
	r.WithSubjectErasers(stubEraser{"vakthr"})
	require.Len(t, r.subjectErasers, 2, "a second call must append, not replace")

	r.WithSubjectErasers(stubEraser{"vakthr"})
	require.Len(t, r.subjectErasers, 2, "the same module must not be wired twice")
}

// TestRequiredEraserModules_CoversEveryPIIModule is the completeness contract.
// If a module starts storing data-subject PII and is not added to
// requiredEraserModules, ExecuteErasure would happily run without its eraser
// and still stamp the DSR "completed" — a silent partial Art. 17 erasure.
//
// The list is asserted explicitly rather than derived, so ADDING a module is a
// deliberate act that shows up in review instead of being inferred.
func TestRequiredEraserModules_CoversEveryPIIModule(t *testing.T) {
	require.ElementsMatch(t, []string{"vaktaware", "vakthr"}, requiredEraserModules,
		"modules holding subject PII changed — wire an eraser for the new module "+
			"and update requiredEraserModules, or Art. 17 erasure silently skips it")
}
