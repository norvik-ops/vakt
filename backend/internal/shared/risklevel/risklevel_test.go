// Copyright (c) 2026 NorvikOps. All rights reserved.
// SPDX-License-Identifier: Elastic-2.0

package risklevel

import (
	"encoding/json"
	"os"
	"testing"
)

type fixture struct {
	Cases []struct {
		Score int    `json:"score"`
		Level string `json:"level"`
	} `json:"cases"`
}

// R1-KW-17: Go and TypeScript read the same fixture (score 0–25 → level).
// frontend/src/shared/utils/riskLevel.test.ts checks riskLevel() against it,
// so a boundary that moves on one side only turns one of the two tests red.
func TestOf_SharedFixture(t *testing.T) {
	raw, err := os.ReadFile("testdata/risk-levels.json")
	if err != nil {
		t.Fatal(err)
	}
	var f fixture
	if err := json.Unmarshal(raw, &f); err != nil {
		t.Fatal(err)
	}
	if len(f.Cases) != 26 {
		t.Fatalf("fixture has %d cases, want 26 (score 0–25)", len(f.Cases))
	}
	for _, c := range f.Cases {
		if got := Of(c.Score); string(got) != c.Level {
			t.Errorf("Of(%d) = %q, fixture says %q", c.Score, got, c.Level)
		}
	}
}

// The grid points are the boundaries of the agreed 15/10/5 scheme, written
// out so the fixture itself cannot drift unnoticed.
func TestOf_Boundaries(t *testing.T) {
	cases := []struct {
		score int
		want  Level
	}{
		{1, Low}, {4, Low},
		{5, Medium}, {9, Medium},
		{10, High}, {14, High},
		{15, Critical}, {25, Critical},
		// residual risk uses the same cut
		{12, High}, {6, Medium},
	}
	for _, c := range cases {
		if got := Of(c.score); got != c.want {
			t.Errorf("Of(%d) = %q, want %q", c.score, got, c.want)
		}
	}
}
