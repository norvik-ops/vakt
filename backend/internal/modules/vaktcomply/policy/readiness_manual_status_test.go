// Copyright (c) 2026 NorvikOps. All rights reserved.
// SPDX-License-Identifier: Elastic-2.0

package policy_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/matharnica/vakt/internal/modules/vaktcomply/policy"
)

// TestComputeReadinessReport_HonoursManualStatus pins R1-14b-A7.
//
// ComputeReadinessReport bucketed every control purely by its evidence count
// (>=2 covered, ==1 partial, else missing). ck_controls.manual_status and
// not_applicable — the two fields the user actually maintains in the UI — were
// never read, although policy.ResolveStatus with exactly the right precedence
// sat two functions further down in the same file and was used by
// checkFrameworkMilestone.
//
// Setting a control to "implemented" therefore moved the readiness report, the
// score snapshot (service_reporting.go) and the framework PDF (pdf.go) by zero,
// while attaching evidence to the same control did move them — two inputs, one
// of which silently did nothing.
//
// Reverting service_helpers.go to the evidence-only switch makes every subtest
// below fail.
func TestComputeReadinessReport_HonoursManualStatus(t *testing.T) {
	fw := &policy.Framework{ID: "fw-1", Name: "ISO27001"}

	t.Run("manual implemented counts without any evidence", func(t *testing.T) {
		controls := []policy.Control{
			{ID: "c-1", Domain: "Access", ManualStatus: "implemented"},
			{ID: "c-2", Domain: "Access"},
		}
		report := policy.ComputeReadinessReport(fw, controls, map[string]int{})

		require.NotNil(t, report)
		assert.Equal(t, 2, report.TotalControls)
		assert.Equal(t, 1, report.Covered, "a control set to implemented must count as covered")
		assert.Equal(t, 0, report.Partial)
		assert.Equal(t, 1, report.Missing)
		assert.InDelta(t, 50.0, report.ReadinessScore, 0.001)
	})

	t.Run("manual in_progress counts as partial", func(t *testing.T) {
		controls := []policy.Control{
			{ID: "c-1", Domain: "Access", ManualStatus: "in_progress"},
			{ID: "c-2", Domain: "Access"},
		}
		report := policy.ComputeReadinessReport(fw, controls, map[string]int{})

		assert.Equal(t, 0, report.Covered)
		assert.Equal(t, 1, report.Partial)
		assert.Equal(t, 1, report.Missing)
		assert.InDelta(t, 25.0, report.ReadinessScore, 0.001)
	})

	t.Run("manual status wins over the evidence count", func(t *testing.T) {
		// Three evidences would be "covered" on the evidence axis, but the user
		// says the control is only in progress. ResolveStatus gives the human
		// the last word — the report must do the same.
		controls := []policy.Control{
			{ID: "c-1", Domain: "Access", ManualStatus: "in_progress"},
		}
		report := policy.ComputeReadinessReport(fw, controls, map[string]int{"c-1": 3})

		assert.Equal(t, 0, report.Covered)
		assert.Equal(t, 1, report.Partial)
		assert.InDelta(t, 50.0, report.ReadinessScore, 0.001)
	})

	t.Run("not_applicable leaves the denominator", func(t *testing.T) {
		// One applicable control, implemented → 100 %, not 50 %. The N/A
		// control stays visible in TotalControls so the four buckets still
		// add up to the total shown as "Gesamt: N Controls" in the PDF.
		controls := []policy.Control{
			{ID: "c-1", Domain: "Access", ManualStatus: "implemented"},
			{ID: "c-2", Domain: "Access", NotApplicable: true},
		}
		report := policy.ComputeReadinessReport(fw, controls, map[string]int{})

		assert.Equal(t, 2, report.TotalControls)
		assert.Equal(t, 1, report.Covered)
		assert.Equal(t, 0, report.Missing)
		assert.Equal(t, 1, report.NotApplicable)
		assert.Equal(t, report.TotalControls,
			report.Covered+report.Partial+report.Missing+report.NotApplicable,
			"buckets must add up to the total")
		assert.InDelta(t, 100.0, report.ReadinessScore, 0.001)
	})

	t.Run("not_applicable wins over manual status and evidence", func(t *testing.T) {
		controls := []policy.Control{
			{ID: "c-1", Domain: "Access", ManualStatus: "implemented", NotApplicable: true},
		}
		report := policy.ComputeReadinessReport(fw, controls, map[string]int{"c-1": 5})

		assert.Equal(t, 0, report.Covered)
		assert.Equal(t, 1, report.NotApplicable)
		assert.InDelta(t, 0.0, report.ReadinessScore, 0.001,
			"a framework with only N/A controls has no applicable controls to score")
	})

	t.Run("per-domain score follows the same rule", func(t *testing.T) {
		controls := []policy.Control{
			{ID: "c-1", Domain: "Access", ManualStatus: "implemented"},
			{ID: "c-2", Domain: "Crypto"},
		}
		report := policy.ComputeReadinessReport(fw, controls, map[string]int{})

		byDomain := map[string]policy.DomainScore{}
		for _, d := range report.ByDomain {
			byDomain[d.Domain] = d
		}
		require.Contains(t, byDomain, "Access")
		assert.InDelta(t, 100.0, byDomain["Access"].Score, 0.001,
			"the implemented control must lift its own domain too")
		assert.InDelta(t, 0.0, byDomain["Crypto"].Score, 0.001)
	})
}
