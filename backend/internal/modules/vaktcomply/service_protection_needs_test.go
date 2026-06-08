package vaktcomply

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestCalculateOverallProtectionNeed(t *testing.T) {
	cases := []struct{ c, i, a, want string }{
		{"normal", "normal", "normal", "normal"},
		{"normal", "hoch", "normal", "hoch"},
		{"sehr_hoch", "normal", "normal", "sehr_hoch"},
		{"hoch", "sehr_hoch", "hoch", "sehr_hoch"},
		{"hoch", "hoch", "normal", "hoch"},
		{"normal", "normal", "sehr_hoch", "sehr_hoch"},
	}
	for _, tc := range cases {
		got := CalculateOverallProtectionNeed(tc.c, tc.i, tc.a)
		assert.Equal(t, tc.want, got, "c=%s i=%s a=%s", tc.c, tc.i, tc.a)
	}
}
