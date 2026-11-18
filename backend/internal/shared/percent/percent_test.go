// Copyright (c) 2026 NorvikOps. All rights reserved.
// SPDX-License-Identifier: Elastic-2.0

package percent

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestRound_HalbRundetAuf(t *testing.T) {
	cases := []struct {
		in   float64
		want int
	}{
		{0, 0},
		{0.49, 0},
		{0.5, 1},
		{12.5, 13},  // %.0f druckte hier 12 (round half to even)
		{2.5, 3},    // %.0f: 2
		{18.75, 19}, // 1 umgesetzt + 1 teilweise von 8
		{5.113636, 5},
		{99.5, 100},
		{100, 100},
	}
	for _, c := range cases {
		require.Equal(t, c.want, Round(c.in), "Round(%v)", c.in)
	}
}

// 14 umgesetzt + 1 teilweise von 100 sind genau 14,5 %; als float64 kommt
// 14.499999999999998 heraus. Ohne Ausgleich würde das abgerundet.
func TestRound_FloatRauschenBeiHalb(t *testing.T) {
	// Wie policy.ReadinessScore, mit Variablen statt Konstanten (Go rechnet
	// konstante Ausdrücke exakt).
	covered, partial, total := 14, 1, 100
	weighted := float64(covered) + float64(partial)*0.5
	pct := (weighted / float64(total)) * 100
	require.Less(t, pct, 14.5, "Vorbedingung: float64 liegt knapp unter 14,5")
	require.Equal(t, 15, Round(pct))
}

func TestFormat_WieRoundMitProzentzeichen(t *testing.T) {
	require.Equal(t, "13%", Format(12.5))
	require.Equal(t, "0%", Format(0))
	// Gegenprobe: genau der Unterschied, den das Paket beseitigt.
	require.Equal(t, "12%", fmt.Sprintf("%.0f%%", 12.5))
}
