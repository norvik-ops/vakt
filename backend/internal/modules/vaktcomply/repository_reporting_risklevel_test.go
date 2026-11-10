// Copyright (c) 2026 NorvikOps. All rights reserved.
// SPDX-License-Identifier: Elastic-2.0

package vaktcomply

import (
	"testing"

	"github.com/matharnica/vakt/internal/db"
)

// R1-KW-17: the executive summary PDF labels its top 5 with the risk register
// levels. Before the fix the SQL cut at 15/9/4 — score 9 was printed „Hoch“.
func TestToExecutiveTopRisks_UsesRiskRegisterLevels(t *testing.T) {
	rows := []db.GetExecutiveTopRisksRow{
		{Title: "a", Score: 15}, {Title: "b", Score: 10}, {Title: "c", Score: 9},
		{Title: "d", Score: 5}, {Title: "e", Score: 4},
	}
	want := []string{"critical", "high", "medium", "medium", "low"}
	got := toExecutiveTopRisks(rows)
	if len(got) != len(want) {
		t.Fatalf("len = %d, want %d", len(got), len(want))
	}
	for i, w := range want {
		if got[i].Severity != w {
			t.Errorf("score %d: severity %q, want %q", got[i].Score, got[i].Severity, w)
		}
	}
}
