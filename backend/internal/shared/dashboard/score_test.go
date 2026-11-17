// Copyright (c) 2026 NorvikOps. All rights reserved.
// SPDX-License-Identifier: Elastic-2.0

package dashboard

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// UX-37: the dashboard score is the pooled readiness (ADR-0086), not the old
// "70 + 10 per framework" formula. ScoreFromReadiness only rounds and labels
// the tally the ReadinessSource delivers — it computes nothing itself.

func TestScoreFromReadiness_EmptyInstanceIsNotSeventy(t *testing.T) {
	got := ScoreFromReadiness(FrameworkReadiness{}, 0)
	assert.Equal(t, 0, got.Score, "leere Instanz zeigte früher 70/100")
	assert.False(t, got.HasData, "ohne anwendbare Kontrolle gibt es noch keine Daten")
}

func TestScoreFromReadiness_ThreeFrameworksOneOf214(t *testing.T) {
	// The UX-Durchgang state: 3 frameworks, 1 of 214 controls implemented.
	// Old formula: 70 + 3×10 = 100. Readiness: 1/214 = 0.47 % → 0.
	got := ScoreFromReadiness(FrameworkReadiness{Covered: 1, Applicable: 214, Score: 100.0 / 214.0}, 3)
	assert.Equal(t, 0, got.Score)
	assert.True(t, got.HasData)
	assert.InDelta(t, 100.0/214.0, got.Readiness.ScorePct, 1e-9, "ungerundeter Wert bleibt erhalten")
	assert.Equal(t, int64(3), got.Components["active_frameworks"])
}

func TestScoreFromReadiness_OneOfFourIsReadinessValue(t *testing.T) {
	got := ScoreFromReadiness(FrameworkReadiness{Covered: 1, Applicable: 4, Score: 25}, 1)
	assert.Equal(t, 25, got.Score)
	assert.Equal(t, OverallReadiness{Covered: 1, Partial: 0, Applicable: 4, ScorePct: 25}, got.Readiness)
}

func TestScoreFromReadiness_RoundsLikeTheFrontend(t *testing.T) {
	// The "Compliance-Fortschritt" card shows Math.round(score_pct); 12.5 must
	// become 13 here as well (math.Round, not banker's rounding).
	got := ScoreFromReadiness(FrameworkReadiness{Partial: 1, Applicable: 4, Score: 12.5}, 1)
	assert.Equal(t, 13, got.Score)
}

func TestScoreFromReadiness_KeepsLegacyComponents(t *testing.T) {
	got := ScoreFromReadiness(FrameworkReadiness{}, 2)
	for _, k := range []string{"critical_findings", "high_findings", "open_breaches", "active_frameworks"} {
		_, ok := got.Components[k]
		assert.True(t, ok, "Feld %s bleibt für ältere Clients lesbar", k)
	}
}

type failingReadiness struct{}

func (failingReadiness) ReadinessSummary(context.Context, string) (ReadinessSummary, error) {
	return ReadinessSummary{}, errors.New("boom")
}

func TestLoadScore_NoReadinessSourceIsAnError(t *testing.T) {
	// A missing source must not silently show 0 % as if nothing were done.
	_, err := NewService(nil, nil).LoadScore(context.Background(), "org")
	require.Error(t, err)
}

func TestLoadScore_ReadinessErrorPropagates(t *testing.T) {
	_, err := NewService(nil, failingReadiness{}).LoadScore(context.Background(), "org")
	require.Error(t, err)
}

func TestScoreCacheKey_Versioned(t *testing.T) {
	// v2: payload changed meaning (readiness) — an old cached 70/100 must not be served.
	assert.Equal(t, "dashboard:score:v2:org-a", scoreCacheKey("org-a"))
}
